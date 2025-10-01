package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"golang.org/x/time/rate"
)

const (
	// maxBufferSize is the maximum size of buffers to keep in the pool.
	// Buffers larger than this will be discarded to prevent excessive memory usage.
	maxBufferSize = 10 * 1024 * 1024 // 10MB
	// initialBufferSize is the initial allocation size for new buffers
	initialBufferSize = 4 * 1024 // 4KB for most API responses
	// maxResponseBodySize limits the size of HTTP response bodies to prevent DoS
	maxResponseBodySize = 10 * 1024 * 1024 // 10MB
)

var bodyBufferPool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate buffers with a reasonable initial size
		buf := new(bytes.Buffer)
		buf.Grow(initialBufferSize)
		return buf
	},
}

func getBuffer() *bytes.Buffer {
	buf := bodyBufferPool.Get().(*bytes.Buffer)
	buf.Reset() // Ensure buffer is clean before use
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	// Safety check - nil buffers shouldn't be returned
	if buf == nil {
		return
	}

	// Don't return oversized buffers to the pool to prevent memory bloat
	// These will be garbage collected instead
	if buf.Cap() > maxBufferSize {
		// Explicitly nil out to help GC (though this local variable will go out of scope anyway)
		// The important thing is we don't keep a reference in the pool
		return
	}

	// Reset the buffer before returning to pool
	buf.Reset()
	bodyBufferPool.Put(buf)
}

const (
	DefaultRequestsPerMinute    = 1000
	DefaultRateLimitBurst       = 10
	SecondsPerMinute            = 60.0
	ParseFloatBitSize           = 64
	defaultLogBodyBytes         = 4 * 1024
	ProactiveRateLimitThreshold = 5 // Start throttling when remaining requests drop below this
	// RateLimitBufferMultiplier adds conservative buffer to rate limit calculations (10% safety margin)
	RateLimitBufferMultiplier = 1.1
	// MinRateLimitPerSecond is the minimum rate limit to prevent division by zero
	MinRateLimitPerSecond = 1.0
)

// Client manages communication with the Reddit API.
type Client struct {
	client          *http.Client
	BaseURL         *url.URL
	UserAgent       string
	logger          *slog.Logger
	maxLogBodyBytes int

	limiter            *rate.Limiter
	forceWaitUntil     atomic.Int64 // Unix nanoseconds
	rateLimitThreshold float64      // When to start proactive throttling
}

// RateLimitConfig controls how requests are throttled before reaching Reddit.
type RateLimitConfig struct {
	// RequestsPerMinute caps steady-state throughput. Defaults to 60 if zero.
	RequestsPerMinute float64
	// Burst allows short spikes above the steady-state rate. Defaults to 10 if zero.
	Burst int
	// ProactiveThreshold is the number of remaining requests at which to start throttling.
	// Defaults to ProactiveRateLimitThreshold if zero.
	ProactiveThreshold float64
}

// NewClient returns a new Reddit API client.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewClient(httpClient *http.Client, baseURL string, userAgent string, logger *slog.Logger) (*Client, error) {
	return NewClientWithRateLimit(httpClient, baseURL, userAgent, logger, RateLimitConfig{})
}

// NewClientWithRateLimit returns a new Reddit API client with custom rate limiting.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewClientWithRateLimit(httpClient *http.Client, baseURL string, userAgent string, logger *slog.Logger, cfg RateLimitConfig) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, &pkgerrs.ClientError{Err: err}
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}

	// Build rate limiter with config
	limiter := buildLimiter(cfg)

	// Set proactive threshold
	threshold := cfg.ProactiveThreshold
	if threshold <= 0 {
		threshold = ProactiveRateLimitThreshold
	}

	c := &Client{
		client:             httpClient,
		BaseURL:            parsedURL,
		UserAgent:          userAgent,
		limiter:            limiter,
		logger:             logger,
		maxLogBodyBytes:    defaultLogBodyBytes,
		rateLimitThreshold: threshold,
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
// Optional query parameters can be provided as url.Values.
// Note: The caller is responsible for setting authentication headers.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
	u, err := c.BaseURL.Parse(path)
	if err != nil {
		return nil, &pkgerrs.ClientError{Err: err}
	}

	// Add query parameters if provided
	if len(params) > 0 && params[0] != nil {
		q := u.Query()
		for key, values := range params[0] {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, &pkgerrs.ClientError{Err: err}
	}

	req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

// doRequest handles the common HTTP request flow and returns raw response body.
// This centralizes rate limiting, logging, and error handling for all HTTP operations.
func (c *Client) doRequest(req *http.Request) ([]byte, *http.Response, error) {
	ctx := req.Context()
	start := time.Now()

	// Rate limiting
	if err := c.waitForRateLimit(ctx); err != nil {
		c.logWaitFailure(ctx, req, err)
		return nil, nil, &pkgerrs.ClientError{Err: err}
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		c.logTransportError(ctx, req, time.Since(start), err)
		return nil, nil, &pkgerrs.ClientError{Err: err}
	}
	defer resp.Body.Close()

	// Apply rate limit headers
	c.applyRateHeaders(resp)

	// Read body using pooled buffer with size limit to prevent DoS
	buf := getBuffer()
	defer putBuffer(buf)

	// Limit response body size
	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	bytesRead, err := io.Copy(buf, limitedReader)
	if err != nil {
		c.logBodyReadError(ctx, req, resp, time.Since(start), err)
		return nil, resp, &pkgerrs.ClientError{Err: err}
	}

	// Check if we hit the size limit
	if bytesRead == maxResponseBodySize {
		// Try reading one more byte to see if there's more data
		var extraByte [1]byte
		if n, _ := resp.Body.Read(extraByte[:]); n > 0 {
			err := fmt.Errorf("response body exceeded max size of %d bytes", maxResponseBodySize)
			c.logBodyReadError(ctx, req, resp, time.Since(start), err)
			return nil, resp, &pkgerrs.ClientError{Err: err}
		}
	}

	// Copy buffer contents to returned byte slice
	bodyBytes := make([]byte, buf.Len())
	copy(bodyBytes, buf.Bytes())

	c.logHTTPResult(ctx, req, resp, bodyBytes, time.Since(start))

	// Check HTTP status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return bodyBytes, resp, &pkgerrs.APIError{StatusCode: resp.StatusCode, Message: "request failed"}
	}

	return bodyBytes, resp, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred.
func (c *Client) Do(req *http.Request, v *types.Thing) error {
	bodyBytes, resp, err := c.doRequest(req)
	if err != nil {
		return err
	}

	if v != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, v); err != nil {
			c.logDecodeError(req.Context(), req, resp, err)
			return &pkgerrs.ClientError{Err: err}
		}
	}

	return nil
}

// DoThingArray sends an API request and returns either an array of Things or a single Thing wrapped in an array.
// Used for the comments endpoint which can return [post, comments] or a single Listing.
func (c *Client) DoThingArray(req *http.Request) ([]*types.Thing, error) {
	bodyBytes, resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	// Parse the response which can be either an array or a single Thing
	var result []*types.Thing

	if len(bodyBytes) > 0 && bodyBytes[0] == '[' {
		// It's an array response
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return nil, &pkgerrs.ClientError{Err: fmt.Errorf("failed to parse array response: %w", err)}
		}
	} else if len(bodyBytes) > 0 && bodyBytes[0] == '{' {
		// It's a single object - could be a Listing or an error
		var singleThing types.Thing
		if err := json.Unmarshal(bodyBytes, &singleThing); err != nil {
			// Check if it's an error response
			var errObj struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(bodyBytes, &errObj); err == nil && errObj.Error != "" {
				return nil, &pkgerrs.APIError{StatusCode: resp.StatusCode, ErrorCode: errObj.Error, Message: errObj.Message}
			}
			return nil, &pkgerrs.ClientError{Err: fmt.Errorf("failed to parse response: %w", err)}
		}

		// If it's a Listing with comments, wrap it in an array
		if singleThing.Kind == "Listing" {
			result = []*types.Thing{&singleThing}
		} else {
			return nil, &pkgerrs.ClientError{Err: fmt.Errorf("unexpected response kind: %s", singleThing.Kind)}
		}
	} else {
		return nil, &pkgerrs.ClientError{Err: fmt.Errorf("empty or invalid response from Reddit")}
	}

	return result, nil
}

// DoMoreChildren sends an API request to the morechildren endpoint and returns the Things array.
func (c *Client) DoMoreChildren(req *http.Request) ([]*types.Thing, error) {
	bodyBytes, resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	// Parse the morechildren response structure
	var response struct {
		JSON struct {
			Errors [][]string `json:"errors"`
			Data   struct {
				Things []*types.Thing `json:"things"`
			} `json:"data"`
		} `json:"json"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, &pkgerrs.ClientError{Err: fmt.Errorf("failed to parse morechildren response: %w", err)}
	}

	// Check for API errors
	if len(response.JSON.Errors) > 0 {
		return nil, &pkgerrs.APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("API error: %v", response.JSON.Errors[0])}
	}

	return response.JSON.Data.Things, nil
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
		limitPerSecond = rate.Limit(MinRateLimitPerSecond)
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
	if remaining < c.rateLimitThreshold {
		// Calculate delay to spread remaining requests over the reset period
		// Add conservative buffer to be safe
		if remaining > 0 {
			delayPerRequest := (resetSeconds * RateLimitBufferMultiplier) / remaining
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

	// Use background context as fallback if nil (only for logging, not request flow)
	if ctx == nil {
		ctx = context.Background()
	}

	until := time.Now().Add(d)
	untilNanos := until.UnixNano()

	// Use a CAS loop to ensure we only update if the new value is later
	for {
		// Check if context is cancelled before retrying CAS
		select {
		case <-ctx.Done():
			// Context cancelled, stop trying to update rate limit
			return
		default:
		}

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
		// CAS failed, yield to avoid busy-wait before retrying
		time.Sleep(time.Microsecond)
	}
}

// rateLimitContext extracts context from response for rate limit logging.
// Returns Background context as fallback when response/request is nil.
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

	// Only process body if debug logging is enabled (avoid unnecessary allocations)
	if len(body) > 0 && c.logger.Enabled(ctx, slog.LevelDebug) {
		// Truncate body inside debug check to avoid work when debug is disabled
		limit := c.maxLogBodyBytes
		if limit <= 0 {
			limit = defaultLogBodyBytes
		}

		var snippet string
		var truncated bool
		if len(body) <= limit {
			snippet = string(body)
			truncated = false
		} else {
			snippet = string(body[:limit])
			truncated = true
		}

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

// contextOrBackground returns the provided context or Background as fallback.
// Used to ensure logging functions always have a valid context.
func contextOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
