package internal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"golang.org/x/time/rate"
)

func TestNewClient_DefaultRateLimiter(t *testing.T) {
	client, err := NewClient(nil, "token", "https://example.com/api/", "agent", nil, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if client.limiter == nil {
		t.Fatalf("expected limiter to be initialized")
	}

	if got := client.limiter.Limit(); got != rate.Limit(1) {
		t.Errorf("expected default limit 1 req/sec, got %v", got)
	}
	if got := client.limiter.Burst(); got != 10 {
		t.Errorf("expected default burst of 10, got %d", got)
	}
}

func TestNewClient_InvalidBaseURL(t *testing.T) {
	_, err := NewClient(nil, "token", "://bad", "agent", nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestNewClient_CustomLimiterConfig(t *testing.T) {
	client, err := NewClient(nil, "token", "https://example.com/api", "agent", &RateLimitConfig{RequestsPerMinute: 120, Burst: 5}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if got := client.BaseURL.String(); got != "https://example.com/api/" {
		t.Fatalf("expected base URL to gain trailing slash, got %q", got)
	}

	if client.limiter == nil {
		t.Fatal("expected limiter to be initialized")
	}

	if got := client.limiter.Limit(); got != rate.Limit(2) {
		t.Errorf("expected limit of 2 req/sec, got %v", got)
	}
	if got := client.limiter.Burst(); got != 5 {
		t.Errorf("expected burst of 5, got %d", got)
	}
}

func TestClient_NewRequestSetsHeaders(t *testing.T) {
	httpClient := &http.Client{}
	c, err := NewClient(httpClient, "token-value", "https://example.com", "my-agent", &RateLimitConfig{RequestsPerMinute: 1000, Burst: 100}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer token-value" {
		t.Errorf("expected Authorization header 'Bearer token-value', got %q", got)
	}
	if got := req.Header.Get("User-Agent"); got != "my-agent" {
		t.Errorf("expected User-Agent 'my-agent', got %q", got)
	}

	if req.URL.String() != "https://example.com/resource" {
		t.Errorf("unexpected request URL: %s", req.URL)
	}
}

func TestClient_NewRequestInvalidPath(t *testing.T) {
	c, err := NewClient(nil, "token", "https://example.com", "agent", nil, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, err = c.NewRequest(context.Background(), http.MethodGet, "%zz", nil)
	if err == nil {
		t.Fatal("expected error constructing request with invalid path")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestClient_NewRequestPreservesBody(t *testing.T) {
	c, err := NewClient(nil, "token", "https://example.com", "agent", nil, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	body := []byte("payload")
	req, err := c.NewRequest(context.Background(), http.MethodPost, "resource", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	defer req.Body.Close()

	got, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed reading request body: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("expected body %q, got %q", body, got)
	}
}

func TestClient_DoDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kind":"t3","data":{"id":"abc123"}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 1000, Burst: 100}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "test", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	var thing types.Thing
	if _, err := c.Do(req, &thing); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}

	if thing.Kind != "t3" {
		t.Errorf("expected kind 't3', got %q", thing.Kind)
	}
	if len(thing.Data) == 0 {
		t.Errorf("expected data to be populated")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClient_DoTransportErrorWrapped(t *testing.T) {
	expectedErr := errors.New("boom")
	httpClient := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, expectedErr
	})}

	c, err := NewClient(httpClient, "token", "https://example.com/", "agent", nil, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	_, err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected transport error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if !errors.Is(clientErr, expectedErr) {
		t.Fatalf("expected wrapped error %v, got %v", expectedErr, clientErr)
	}
}

func TestClient_DoNonSuccessStatusReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"temporary"}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 60000, Burst: 1000}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "fail", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	resp, err := c.Do(req, nil)
	if err == nil {
		t.Fatal("expected API error")
	}
	if resp == nil {
		t.Fatal("expected response to be returned alongside error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status on APIError: %d", apiErr.Response.StatusCode)
	}
}

func TestClient_DoJSONDecodeErrorWrapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"bad json"`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 60000, Burst: 1000}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "bad-json", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	var thing types.Thing
	_, err = c.Do(req, &thing)
	if err == nil {
		t.Fatal("expected decode error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestClient_DoSkipsDecodeWhenTargetNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"not":"json"`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 60000, Burst: 1000}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "skip", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	resp, err := c.Do(req, nil)
	if err != nil {
		t.Fatalf("expected no error when decode target nil, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestClient_DoEnforcesRetryAfter(t *testing.T) {
	var (
		mu        sync.Mutex
		callCount int
		firstHit  time.Time
		secondHit time.Time
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		if callCount == 1 {
			firstHit = time.Now()
			w.Header().Set("Retry-After", "0.1")
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", "0.1")
		} else {
			secondHit = time.Now()
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 60000, Burst: 1000}, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx := context.Background()
	req1, err := c.NewRequest(ctx, http.MethodGet, "first", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	if _, err := c.Do(req1, nil); err != nil {
		t.Fatalf("Do on first request returned error: %v", err)
	}

	req2, err := c.NewRequest(ctx, http.MethodGet, "second", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	start := time.Now()
	if _, err := c.Do(req2, nil); err != nil {
		t.Fatalf("Do on second request returned error: %v", err)
	}
	elapsed := time.Since(start)

	mu.Lock()
	s := secondHit
	f := firstHit
	n := callCount
	mu.Unlock()

	if n != 2 {
		t.Fatalf("expected 2 calls to server, got %d", n)
	}
	if s.IsZero() {
		t.Fatal("second request timestamp was not recorded")
	}

	if diff := s.Sub(f); diff < 90*time.Millisecond {
		t.Fatalf("expected at least 90ms between requests, got %v", diff)
	}
	if elapsed < 90*time.Millisecond {
		t.Fatalf("expected Do call to take at least 90ms due to rate limit, took %v", elapsed)
	}
}

func TestClient_DoHonorsCanceledContextBeforeSend(t *testing.T) {
	transportCalled := false
	httpClient := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		transportCalled = true
		return nil, errors.New("unexpected transport call")
	})}

	c, err := NewClient(httpClient, "token", "https://example.com/", "agent", nil, nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := c.NewRequest(ctx, http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	_, err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected error due to canceled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if transportCalled {
		t.Fatal("transport should not be invoked when context already canceled")
	}
}

func TestClient_WaitForForcedDelayBlocksAndClears(t *testing.T) {
	c := &Client{}
	future := time.Now().Add(30 * time.Millisecond)
	atomic.StoreInt64(&c.forceWaitUntil, future.UnixNano())

	start := time.Now()
	if err := c.waitForForcedDelay(context.Background()); err != nil {
		t.Fatalf("waitForForcedDelay returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 25*time.Millisecond {
		t.Fatalf("expected waitForForcedDelay to block, elapsed %v", elapsed)
	}

	cleared := atomic.LoadInt64(&c.forceWaitUntil) == 0
	if !cleared {
		t.Fatal("expected forced delay to be cleared after waiting")
	}
}

func TestClient_WaitForForcedDelayContextCanceled(t *testing.T) {
	c := &Client{}
	atomic.StoreInt64(&c.forceWaitUntil, time.Now().Add(100*time.Millisecond).UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.waitForForcedDelay(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}

	if atomic.LoadInt64(&c.forceWaitUntil) == 0 {
		t.Fatalf("forced delay should remain until cleared on successful wait")
	}
}

func TestClient_DeferRequestsExtendsDelay(t *testing.T) {
	c := &Client{}

	c.deferRequests(context.Background(), -time.Second, "test")
	zero := atomic.LoadInt64(&c.forceWaitUntil) == 0
	if !zero {
		t.Fatal("negative duration should not set forced delay")
	}

	c.deferRequests(context.Background(), 20*time.Millisecond, "test")
	first := atomic.LoadInt64(&c.forceWaitUntil)
	if first == 0 {
		t.Fatal("expected forced delay to be set")
	}

	c.deferRequests(context.Background(), 5*time.Millisecond, "test")
	second := atomic.LoadInt64(&c.forceWaitUntil)
	if second != first {
		t.Fatalf("shorter defer should not reduce wait: first=%v second=%v", first, second)
	}

	c.deferRequests(context.Background(), 40*time.Millisecond, "test")
	third := atomic.LoadInt64(&c.forceWaitUntil)
	if third <= first {
		t.Fatalf("longer defer should extend wait: first=%v third=%v", first, third)
	}
}

func TestClient_SetLogBodyLimit(t *testing.T) {
	c := &Client{maxLogBodyBytes: defaultLogBodyBytes}

	c.SetLogBodyLimit(2048)
	if c.maxLogBodyBytes != 2048 {
		t.Fatalf("expected maxLogBodyBytes to be 2048, got %d", c.maxLogBodyBytes)
	}

	c.SetLogBodyLimit(0)
	if c.maxLogBodyBytes != defaultLogBodyBytes {
		t.Fatalf("expected reset to defaultLogBodyBytes, got %d", c.maxLogBodyBytes)
	}
}

func TestClient_ApplyRateHeadersSetsForcedDelay(t *testing.T) {
	c := &Client{}
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "0.05")

	c.applyRateHeaders(resp)
	deferUntilNanos := atomic.LoadInt64(&c.forceWaitUntil)
	if deferUntilNanos == 0 {
		t.Fatal("expected Retry-After to set forced delay")
	}
	deferUntil := time.Unix(0, deferUntilNanos)
	if time.Until(deferUntil) <= 0 {
		t.Fatalf("expected forced delay to be in the future, got %v", deferUntil)
	}
}

func TestClient_ApplyRateHeadersDoesNotShortenDelay(t *testing.T) {
	c := &Client{}
	c.deferRequests(context.Background(), 60*time.Millisecond, "test")
	initial := atomic.LoadInt64(&c.forceWaitUntil)

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "0.01")
	c.applyRateHeaders(resp)

	final := atomic.LoadInt64(&c.forceWaitUntil)
	if final != initial {
		t.Fatalf("expected shorter retry-after to be ignored: initial=%v final=%v", initial, final)
	}
}

func TestClient_ApplyRateHeadersUsesRatelimitRemaining(t *testing.T) {
	c := &Client{}
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("X-Ratelimit-Remaining", "1")
	resp.Header.Set("X-Ratelimit-Reset", "0.05")

	c.applyRateHeaders(resp)
	deferUntilNanos := atomic.LoadInt64(&c.forceWaitUntil)
	if deferUntilNanos == 0 {
		t.Fatal("expected ratelimit headers to schedule delay")
	}
}

func TestAPIError_ErrorFormatting(t *testing.T) {
	resp := &http.Response{Status: "503 Service Unavailable", StatusCode: http.StatusServiceUnavailable}
	err := &APIError{Response: resp, Message: "temporary outage"}

	if got := err.Error(); got != "API request failed with status 503 Service Unavailable: temporary outage" {
		t.Fatalf("unexpected error string: %q", got)
	}
}

func TestClientError_Unwrap(t *testing.T) {
	inner := errors.New("boom")
	err := &ClientError{OriginalErr: inner}

	if !errors.Is(err, inner) {
		t.Fatalf("expected errors.Is to unwrap inner error")
	}
	if err.Error() != inner.Error() {
		t.Fatalf("expected Error to match inner error message")
	}
}
