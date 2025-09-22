package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"golang.org/x/time/rate"
)

// TokenProvider defines the interface for retrieving an access token.
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

// Client manages communication with the Reddit API.
type Client struct {
	client          *http.Client
	BaseURL         *url.URL
	UserAgent       string
	tokenProvider   TokenProvider
	logger          *slog.Logger
	maxLogBodyBytes int

	limiter        *rate.Limiter
	forceWaitUntil atomic.Int64 // Unix nanoseconds
}

// RateLimitConfig controls how requests are throttled before reaching Reddit.
type RateLimitConfig struct {
	// RequestsPerMinute caps steady-state throughput. Defaults to 60 if zero.
	RequestsPerMinute float64
	// Burst allows short spikes above the steady-state rate. Defaults to 10 if zero.
	Burst int
}

const (
	DefaultRequestsPerMinute    = 1000
	DefaultRateLimitBurst       = 10
	SecondsPerMinute            = 60.0
	ParseFloatBitSize           = 64
	defaultLogBodyBytes         = 4 * 1024
	ProactiveRateLimitThreshold = 5 // Start throttling when remaining requests drop below this
)

// NewClient returns a new Reddit API client.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewClient(httpClient *http.Client, tokenProvider TokenProvider, baseURL string, userAgent string, logger *slog.Logger) (*Client, error) {
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

	limiter := buildLimiter(RateLimitConfig{})

	c := &Client{
		client:          httpClient,
		BaseURL:         parsedURL,
		UserAgent:       userAgent,
		tokenProvider:   tokenProvider,
		limiter:         limiter,
		logger:          logger,
		maxLogBodyBytes: defaultLogBodyBytes,
	}

	return c, nil
}

// SetLogBodyLimit adjusts how many response bytes are captured when debug logging is enabled.
// Non-positive values revert to the default limit.
func (c *Client) SetLogBodyLimit(limit int) {
	if limit <= 0 {
		c.maxLogBodyBytes = defaultLogBodyBytes
		return
	}
	c.maxLogBodyBytes = limit
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

	// Get token and set auth header
	token, err := c.tokenProvider.GetToken(ctx)
	if err != nil {
		return nil, &ClientError{OriginalErr: fmt.Errorf("failed to get auth token: %w", err)}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred.
func (c *Client) Do(req *http.Request, v *types.Thing) (*http.Response, error) {
	ctx := req.Context()
	start := time.Now()

	if err := c.waitForRateLimit(ctx); err != nil {
		c.logWaitFailure(ctx, req, err)
		return nil, &ClientError{OriginalErr: err}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logTransportError(ctx, req, time.Since(start), err)
		return nil, &ClientError{OriginalErr: err}
	}
	defer resp.Body.Close()

	c.applyRateHeaders(resp)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logBodyReadError(ctx, req, resp, time.Since(start), err)
		return resp, &ClientError{OriginalErr: err}
	}

	c.logHTTPResult(ctx, req, resp, bodyBytes, time.Since(start))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, &APIError{Response: resp, Message: "request failed"}
	}

	if v != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, v); err != nil {
			c.logDecodeError(ctx, req, resp, err)
			return resp, &ClientError{OriginalErr: err}
		}
	}

	return resp, nil
}

// DoRaw sends an API request and returns the raw response body as bytes.
// It automatically adds authentication headers to the request.
func (c *Client) DoRaw(req *http.Request) ([]byte, error) {
	ctx := req.Context()
	start := time.Now()

	// Only set auth headers if not already present
	// (NewRequest already sets them, so this handles direct DoRaw calls)
	if req.Header.Get("Authorization") == "" {
		token, err := c.tokenProvider.GetToken(ctx)
		if err != nil {
			return nil, &ClientError{OriginalErr: fmt.Errorf("failed to get auth token: %w", err)}
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	if err := c.waitForRateLimit(ctx); err != nil {
		c.logWaitFailure(ctx, req, err)
		return nil, &ClientError{OriginalErr: err}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logTransportError(ctx, req, time.Since(start), err)
		return nil, &ClientError{OriginalErr: err}
	}
	defer resp.Body.Close()

	c.applyRateHeaders(resp)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logBodyReadError(ctx, req, resp, time.Since(start), err)
		return nil, &ClientError{OriginalErr: err}
	}

	c.logHTTPResult(ctx, req, resp, bodyBytes, time.Since(start))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Response: resp, Message: fmt.Sprintf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))}
	}

	return bodyBytes, nil
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
	// Handle forced delay from rate limit headers
	for {
		waitUntilNanos := c.forceWaitUntil.Load()

		if waitUntilNanos == 0 {
			break
		}

		waitUntil := time.Unix(0, waitUntilNanos)
		now := time.Now()
		if !now.Before(waitUntil) {
			c.clearForcedDelay(waitUntilNanos)
			break
		}

		timer := time.NewTimer(waitUntil.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			c.clearForcedDelay(waitUntilNanos)
		}
	}

	// Apply local rate limiter if configured
	if c.limiter == nil {
		return nil
	}

	return c.limiter.Wait(ctx)
}

func (c *Client) clearForcedDelay(previous int64) {
	// Only clear if the value hasn't changed since we read it
	c.forceWaitUntil.CompareAndSwap(previous, 0)
}

func (c *Client) applyRateHeaders(resp *http.Response) {
	if resp == nil {
		return
	}

	ctx := rateLimitContext(resp)

	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.ParseFloat(retryAfter, ParseFloatBitSize); err == nil && seconds > 0 {
			c.deferRequests(ctx, time.Duration(seconds*float64(time.Second)), "retry_after")
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

	// Proactive throttling: start slowing down when approaching the rate limit
	if remaining < ProactiveRateLimitThreshold {
		// Calculate delay to spread remaining requests over the reset period
		// Add 10% buffer to be conservative
		if remaining > 0 {
			delayPerRequest := (resetSeconds * 1.1) / remaining
			c.deferRequests(ctx, time.Duration(delayPerRequest*float64(time.Second)), "proactive_ratelimit")
		} else {
			// No requests remaining, must wait full reset period
			c.deferRequests(ctx, time.Duration(resetSeconds*float64(time.Second)), "ratelimit_exhausted")
		}
	}
}

func (c *Client) deferRequests(ctx context.Context, d time.Duration, reason string) {
	if d <= 0 {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	until := time.Now().Add(d)
	untilNanos := until.UnixNano()

	// Use a CAS loop to ensure we only update if the new value is later
	for {
		current := c.forceWaitUntil.Load()
		if current >= untilNanos {
			// Current value is already later, nothing to do
			return
		}
		if c.forceWaitUntil.CompareAndSwap(current, untilNanos) {
			// Successfully updated
			if c.logger != nil {
				c.logger.LogAttrs(ctx, slog.LevelInfo, "reddit requests deferred",
					slog.Duration("delay", d),
					slog.Time("until", until),
					slog.String("reason", reason),
				)
			}
			return
		}
		// CAS failed, retry
	}
}

func rateLimitContext(resp *http.Response) context.Context {
	if resp != nil && resp.Request != nil {
		return resp.Request.Context()
	}
	return context.Background()
}

func (c *Client) logWaitFailure(ctx context.Context, req *http.Request, err error) {
	if c.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	c.logger.LogAttrs(ctx, slog.LevelWarn, "reddit request canceled before send",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("error", err.Error()),
	)
}

func (c *Client) logTransportError(ctx context.Context, req *http.Request, duration time.Duration, err error) {
	if c.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	c.logger.LogAttrs(ctx, slog.LevelError, "reddit request transport error",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Duration("duration", duration),
		slog.String("error", err.Error()),
	)
}

func (c *Client) logBodyReadError(ctx context.Context, req *http.Request, resp *http.Response, duration time.Duration, err error) {
	if c.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Duration("duration", duration),
		slog.String("error", err.Error()),
	}
	if resp != nil {
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
	}

	c.logger.LogAttrs(ctx, slog.LevelError, "reddit response read failed", attrs...)
}

func (c *Client) logDecodeError(ctx context.Context, req *http.Request, resp *http.Response, err error) {
	if c.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("error", err.Error()),
	}
	if resp != nil {
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
	}

	c.logger.LogAttrs(ctx, slog.LevelError, "reddit response decode failed", attrs...)
}

func (c *Client) logHTTPResult(ctx context.Context, req *http.Request, resp *http.Response, body []byte, duration time.Duration) {
	if c.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Duration("duration", duration),
	}

	status := 0
	if resp != nil {
		status = resp.StatusCode
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
		if v := resp.Header.Get("X-Ratelimit-Remaining"); v != "" {
			attrs = append(attrs, slog.String("rate_remaining", v))
		}
		if v := resp.Header.Get("X-Ratelimit-Reset"); v != "" {
			attrs = append(attrs, slog.String("rate_reset", v))
		}
		if v := resp.Header.Get("Retry-After"); v != "" {
			attrs = append(attrs, slog.String("retry_after", v))
		}
	}

	level := slog.LevelInfo
	msg := "reddit api request completed"
	if status < 200 || status >= 300 {
		level = slog.LevelWarn
		msg = "reddit api request failed"
	}

	c.logger.LogAttrs(ctx, level, msg, attrs...)

	if len(body) > 0 && c.logger.Enabled(ctx, slog.LevelDebug) {
		snippet, truncated := c.truncateBody(body)
		bodyAttrs := []slog.Attr{
			slog.Int("bytes", len(body)),
			slog.String("body", snippet),
		}
		if truncated {
			bodyAttrs = append(bodyAttrs, slog.Bool("truncated", true))
		}
		c.logger.LogAttrs(ctx, slog.LevelDebug, "reddit api response body", bodyAttrs...)
	}
}

func (c *Client) truncateBody(body []byte) (string, bool) {
	limit := c.maxLogBodyBytes
	if limit <= 0 {
		limit = defaultLogBodyBytes
	}
	if len(body) <= limit {
		return string(body), false
	}
	return string(body[:limit]), true
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
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
