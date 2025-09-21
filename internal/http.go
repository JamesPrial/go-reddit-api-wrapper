package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"golang.org/x/time/rate"
)

// Client manages communication with the Reddit API.
type Client struct {
	client    *http.Client
	BaseURL   *url.URL
	UserAgent string
	token     string

	limiter        *rate.Limiter
	mu             sync.Mutex
	forceWaitUntil time.Time
}

// RateLimitConfig controls how requests are throttled before reaching Reddit.
type RateLimitConfig struct {
	// RequestsPerMinute caps steady-state throughput. Defaults to 60 if zero.
	RequestsPerMinute float64
	// Burst allows short spikes above the steady-state rate. Defaults to 10 if zero.
	Burst int
}

const (
	DefaultRequestsPerMinute = 60
	DefaultRateLimitBurst    = 10
	SecondsPerMinute         = 60.0
	ParseFloatBitSize        = 64
)

// NewClient returns a new Reddit API client.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewClient(httpClient *http.Client, authToken string, baseURL string, userAgent string, rateCfg *RateLimitConfig) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, &ClientError{OriginalErr: err}
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}

	if rateCfg == nil {
		rateCfg = &RateLimitConfig{}
	}

	limiter := buildLimiter(*rateCfg)

	c := &Client{
		client:    httpClient,
		BaseURL:   parsedURL,
		UserAgent: userAgent,
		token:     authToken,
		limiter:   limiter,
	}

	return c, nil
}

// NewRequest creates an API request. A relative URL can be provided in path,
// in which case it is resolved relative to the BaseURL of the Client.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u, err := c.BaseURL.Parse(path)
	if err != nil {
		return nil, &ClientError{OriginalErr: err}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, &ClientError{OriginalErr: err}
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred.
func (c *Client) Do(req *http.Request, v *types.Thing) (*http.Response, error) {
	if err := c.waitForRateLimit(req.Context()); err != nil {
		return nil, &ClientError{OriginalErr: err}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &ClientError{OriginalErr: err}
	}
	defer resp.Body.Close()

	c.applyRateHeaders(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, &APIError{Response: resp, Message: "request failed"}
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return resp, &ClientError{OriginalErr: err}
		}
	}

	return resp, nil
}

func buildLimiter(cfg RateLimitConfig) *rate.Limiter {
	requestsPerMinute := cfg.RequestsPerMinute
	if requestsPerMinute <= 0 {
		requestsPerMinute = DefaultRequestsPerMinute
	}

	burst := cfg.Burst
	if burst <= 0 {
		burst = DefaultRateLimitBurst
	}

	limitPerSecond := rate.Limit(requestsPerMinute / SecondsPerMinute)
	if limitPerSecond <= 0 {
		limitPerSecond = rate.Limit(1)
	}

	return rate.NewLimiter(limitPerSecond, burst)
}

func (c *Client) waitForRateLimit(ctx context.Context) error {
	if err := c.waitForForcedDelay(ctx); err != nil {
		return err
	}

	if c.limiter == nil {
		return nil
	}

	return c.limiter.Wait(ctx)
}

func (c *Client) waitForForcedDelay(ctx context.Context) error {
	for {
		c.mu.Lock()
		waitUntil := c.forceWaitUntil
		c.mu.Unlock()

		if waitUntil.IsZero() {
			return nil
		}

		now := time.Now()
		if !now.Before(waitUntil) {
			c.clearForcedDelay(waitUntil)
			return nil
		}

		timer := time.NewTimer(waitUntil.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			c.clearForcedDelay(waitUntil)
		}
	}
}

func (c *Client) clearForcedDelay(previous time.Time) {
	c.mu.Lock()
	if previous.Equal(c.forceWaitUntil) {
		c.forceWaitUntil = time.Time{}
	}
	c.mu.Unlock()
}

func (c *Client) applyRateHeaders(resp *http.Response) {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.ParseFloat(retryAfter, ParseFloatBitSize); err == nil && seconds > 0 {
			c.deferRequests(time.Duration(seconds * float64(time.Second)))
		}
	}

	remainingHeader := resp.Header.Get("X-Ratelimit-Remaining")
	resetHeader := resp.Header.Get("X-Ratelimit-Reset")
	if remainingHeader == "" || resetHeader == "" {
		return
	}

	remaining, errRemaining := strconv.ParseFloat(remainingHeader, ParseFloatBitSize)
	resetSeconds, errReset := strconv.ParseFloat(resetHeader, ParseFloatBitSize)
	if errRemaining != nil || errReset != nil || resetSeconds <= 0 {
		return
	}

	if remaining <= 1 {
		c.deferRequests(time.Duration(resetSeconds * float64(time.Second)))
	}
}

func (c *Client) deferRequests(d time.Duration) {
	if d <= 0 {
		return
	}

	until := time.Now().Add(d)

	c.mu.Lock()
	if until.After(c.forceWaitUntil) {
		c.forceWaitUntil = until
	}
	c.mu.Unlock()
}

// APIError represents an error returned by the Reddit API.
type APIError struct {
	Response *http.Response
	Message  string
}

// Error returns the error message for the APIError.
func (e *APIError) Error() string {
	return fmt.Sprintf("API request failed with status %s: %s", e.Response.Status, e.Message)
}

// ClientError represents an error that occurred within the client.
type ClientError struct {
	OriginalErr error
}

func (e *ClientError) Error() string {
	return e.OriginalErr.Error()
}

func (e *ClientError) Unwrap() error {
	return e.OriginalErr
}
