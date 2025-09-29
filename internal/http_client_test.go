package internal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"golang.org/x/time/rate"
)

func TestNewClient_DefaultRateLimiter(t *testing.T) {
	client, err := NewClient(nil, "https://example.com/api/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if client.limiter == nil {
		t.Fatalf("expected limiter to be initialized")
	}

	expectedLimit := rate.Limit(1000.0 / 60.0) // 1000 requests per minute
	if got := client.limiter.Limit(); got != expectedLimit {
		t.Errorf("expected default limit %v req/sec, got %v", expectedLimit, got)
	}
	if got := client.limiter.Burst(); got != 10 {
		t.Errorf("expected default burst of 10, got %d", got)
	}
}

func TestNewClient_InvalidBaseURL(t *testing.T) {
	_, err := NewClient(nil, "://bad", "agent", nil)
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestNewClient_BaseURLHandling(t *testing.T) {
	client, err := NewClient(nil, "https://example.com/api", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	if got := client.BaseURL.String(); got != "https://example.com/api/" {
		t.Fatalf("expected base URL to gain trailing slash, got %q", got)
	}

	if client.limiter == nil {
		t.Fatal("expected limiter to be initialized")
	}
}

func TestClient_NewRequestSetsHeaders(t *testing.T) {
	httpClient := &http.Client{}
	c, err := NewClient(httpClient, "https://example.com", "my-agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	// Authorization header is now set by the caller, not by NewRequest
	if got := req.Header.Get("User-Agent"); got != "my-agent" {
		t.Errorf("expected User-Agent 'my-agent', got %q", got)
	}

	if req.URL.String() != "https://example.com/resource" {
		t.Errorf("unexpected request URL: %s", req.URL)
	}
}

func TestClient_NewRequestInvalidPath(t *testing.T) {
	c, err := NewClient(nil, "https://example.com", "agent", nil)
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
	c, err := NewClient(nil, "https://example.com", "agent", nil)
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

func TestClient_NewRequestWithQueryParams(t *testing.T) {
	c, err := NewClient(nil, "https://example.com", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	params := make(map[string][]string)
	params["limit"] = []string{"10"}
	params["sort"] = []string{"hot"}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil, params)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	query := req.URL.Query()
	if query.Get("limit") != "10" {
		t.Errorf("expected limit=10, got %q", query.Get("limit"))
	}
	if query.Get("sort") != "hot" {
		t.Errorf("expected sort=hot, got %q", query.Get("sort"))
	}
}

func TestClient_NewRequestWithMultipleValuesForSameKey(t *testing.T) {
	c, err := NewClient(nil, "https://example.com", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	params := make(map[string][]string)
	params["id"] = []string{"abc", "def", "ghi"}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil, params)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	query := req.URL.Query()
	ids := query["id"]
	if len(ids) != 3 {
		t.Fatalf("expected 3 id values, got %d", len(ids))
	}
	expected := map[string]bool{"abc": true, "def": true, "ghi": true}
	for _, id := range ids {
		if !expected[id] {
			t.Errorf("unexpected id value: %q", id)
		}
	}
}

func TestClient_NewRequestWithEmptyParams(t *testing.T) {
	c, err := NewClient(nil, "https://example.com", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil, make(map[string][]string))
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	if req.URL.RawQuery != "" {
		t.Errorf("expected empty query string, got %q", req.URL.RawQuery)
	}
}

func TestClient_DoDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kind":"t3","data":{"id":"abc123"}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "test", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	var thing types.Thing
	if err := c.Do(req, &thing); err != nil {
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

	c, err := NewClient(httpClient, "https://example.com/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	err = c.Do(req, nil)
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
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "fail", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status on APIError: %d", apiErr.StatusCode)
	}
}

func TestClient_DoJSONDecodeErrorWrapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"bad json"`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "bad-json", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	var thing types.Thing
	err = c.Do(req, &thing)
	if err == nil {
		t.Fatal("expected decode error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

// failingReader simulates a body read error
type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read failure")
}

func (f *failingReader) Close() error {
	return nil
}

func TestClient_DoBodyReadError(t *testing.T) {
	// Create a custom RoundTripper that returns a response with a failing body
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &failingReader{},
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	c, err := NewClient(httpClient, "https://example.com/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	err = c.Do(req, nil)
	if err == nil {
		t.Fatal("expected body read error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if !bytes.Contains([]byte(clientErr.Error()), []byte("simulated read failure")) {
		t.Errorf("expected error to contain 'simulated read failure', got %q", clientErr.Error())
	}
}

func TestClient_DoSkipsDecodeWhenTargetNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"not":"json"`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "skip", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	err = c.Do(req, nil)
	if err != nil {
		t.Fatalf("expected no error when decode target nil, got %v", err)
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
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx := context.Background()
	req1, err := c.NewRequest(ctx, http.MethodGet, "first", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	if err := c.Do(req1, nil); err != nil {
		t.Fatalf("Do on first request returned error: %v", err)
	}

	req2, err := c.NewRequest(ctx, http.MethodGet, "second", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	start := time.Now()
	if err := c.Do(req2, nil); err != nil {
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

	c, err := NewClient(httpClient, "https://example.com/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := c.NewRequest(ctx, http.MethodGet, "resource", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	err = c.Do(req, nil)
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
	c.forceWaitUntil.Store(future.UnixNano())

	start := time.Now()
	if err := c.waitForRateLimit(context.Background()); err != nil {
		t.Fatalf("waitForRateLimit returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 25*time.Millisecond {
		t.Fatalf("expected waitForRateLimit to block, elapsed %v", elapsed)
	}

	cleared := c.forceWaitUntil.Load() == 0
	if !cleared {
		t.Fatal("expected forced delay to be cleared after waiting")
	}
}

func TestClient_WaitForForcedDelayContextCanceled(t *testing.T) {
	c := &Client{}
	c.forceWaitUntil.Store(time.Now().Add(100 * time.Millisecond).UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.waitForRateLimit(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}

	if c.forceWaitUntil.Load() == 0 {
		t.Fatalf("forced delay should remain until cleared on successful wait")
	}
}

func TestClient_DeferRequestsExtendsDelay(t *testing.T) {
	c := &Client{}

	c.deferRequests(context.Background(), -time.Second, "test")
	zero := c.forceWaitUntil.Load() == 0
	if !zero {
		t.Fatal("negative duration should not set forced delay")
	}

	c.deferRequests(context.Background(), 20*time.Millisecond, "test")
	first := c.forceWaitUntil.Load()
	if first == 0 {
		t.Fatal("expected forced delay to be set")
	}

	c.deferRequests(context.Background(), 5*time.Millisecond, "test")
	second := c.forceWaitUntil.Load()
	if second != first {
		t.Fatalf("shorter defer should not reduce wait: first=%v second=%v", first, second)
	}

	c.deferRequests(context.Background(), 40*time.Millisecond, "test")
	third := c.forceWaitUntil.Load()
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
	deferUntilNanos := c.forceWaitUntil.Load()
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
	initial := c.forceWaitUntil.Load()

	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "0.01")
	c.applyRateHeaders(resp)

	final := c.forceWaitUntil.Load()
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
	deferUntilNanos := c.forceWaitUntil.Load()
	if deferUntilNanos == 0 {
		t.Fatal("expected ratelimit headers to schedule delay")
	}
}

func TestClient_ProactiveRateLimiting(t *testing.T) {
	tests := []struct {
		name         string
		remaining    string
		resetSeconds string
		expectDelay  bool
	}{
		{"below threshold - 0 remaining", "0", "10", true},
		{"below threshold - 1 remaining", "1", "10", true},
		{"below threshold - 4 remaining", "4", "10", true},
		{"at threshold - 5 remaining", "5", "10", false},
		{"above threshold - 10 remaining", "10", "10", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{}
			resp := &http.Response{Header: make(http.Header)}
			resp.Header.Set("X-Ratelimit-Remaining", tt.remaining)
			resp.Header.Set("X-Ratelimit-Reset", tt.resetSeconds)

			c.applyRateHeaders(resp)
			deferUntilNanos := c.forceWaitUntil.Load()

			if tt.expectDelay {
				if deferUntilNanos == 0 {
					t.Errorf("expected delay for remaining=%s, but no delay was set", tt.remaining)
				}
			} else {
				if deferUntilNanos != 0 {
					t.Errorf("expected no delay for remaining=%s, but delay was set", tt.remaining)
				}
			}
		})
	}
}

func TestAPIError_ErrorFormatting(t *testing.T) {
	err := &APIError{StatusCode: http.StatusServiceUnavailable, Message: "temporary outage"}

	if got := err.Error(); got != "API request failed with status 503: temporary outage" {
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

func TestClient_DoThingArray_ArrayResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"kind":"Listing","data":{"children":[]}},{"kind":"Listing","data":{"children":[]}}]`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "comments", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoThingArray(req)
	if err != nil {
		t.Fatalf("DoThingArray returned error: %v", err)
	}

	if len(things) != 2 {
		t.Fatalf("expected 2 Things, got %d", len(things))
	}
	if things[0].Kind != "Listing" {
		t.Errorf("expected first Thing kind 'Listing', got %q", things[0].Kind)
	}
	if things[1].Kind != "Listing" {
		t.Errorf("expected second Thing kind 'Listing', got %q", things[1].Kind)
	}
}

func TestClient_DoThingArray_SingleListingWrapped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kind":"Listing","data":{"children":[{"kind":"t1","data":{}}]}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "comments", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoThingArray(req)
	if err != nil {
		t.Fatalf("DoThingArray returned error: %v", err)
	}

	if len(things) != 1 {
		t.Fatalf("expected 1 Thing wrapped in array, got %d", len(things))
	}
	if things[0].Kind != "Listing" {
		t.Errorf("expected Thing kind 'Listing', got %q", things[0].Kind)
	}
}

func TestClient_DoThingArray_APIErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":"USER_REQUIRED","message":"Please log in to do that."}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "restricted", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoThingArray(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if things != nil {
		t.Fatalf("expected nil Things on error, got %v", things)
	}

	// Error objects without "kind" field unmarshal as Thing with empty kind,
	// which triggers "unexpected response kind" error
	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if !bytes.Contains([]byte(clientErr.Error()), []byte("unexpected response kind")) {
		t.Errorf("expected error about unexpected kind, got %q", clientErr.Error())
	}
}

func TestClient_DoThingArray_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(``))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "empty", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoThingArray(req)
	if err == nil {
		t.Fatal("expected error for empty response")
	}
	if things != nil {
		t.Fatalf("expected nil Things on error, got %v", things)
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestClient_DoThingArray_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{bad json}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "bad", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoThingArray(req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if things != nil {
		t.Fatalf("expected nil Things on error, got %v", things)
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestClient_DoThingArray_UnexpectedKind(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kind":"t3","data":{"id":"abc"}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "wrong-kind", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoThingArray(req)
	if err == nil {
		t.Fatal("expected error for unexpected kind")
	}
	if things != nil {
		t.Fatalf("expected nil Things on error, got %v", things)
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
	if !bytes.Contains([]byte(clientErr.Error()), []byte("unexpected response kind")) {
		t.Errorf("expected error about unexpected kind, got %q", clientErr.Error())
	}
}

func TestClient_DoMoreChildren_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"json":{"errors":[],"data":{"things":[{"kind":"t1","data":{"id":"comment1"}},{"kind":"t1","data":{"id":"comment2"}}]}}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "morechildren", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoMoreChildren(req)
	if err != nil {
		t.Fatalf("DoMoreChildren returned error: %v", err)
	}

	if len(things) != 2 {
		t.Fatalf("expected 2 Things, got %d", len(things))
	}
	if things[0].Kind != "t1" {
		t.Errorf("expected first Thing kind 't1', got %q", things[0].Kind)
	}
	if things[1].Kind != "t1" {
		t.Errorf("expected second Thing kind 't1', got %q", things[1].Kind)
	}
}

func TestClient_DoMoreChildren_EmptyThings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"json":{"errors":[],"data":{"things":[]}}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "morechildren", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoMoreChildren(req)
	if err != nil {
		t.Fatalf("DoMoreChildren returned error: %v", err)
	}

	if len(things) != 0 {
		t.Fatalf("expected 0 Things, got %d", len(things))
	}
}

func TestClient_DoMoreChildren_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"json":{"errors":[["THREAD_LOCKED","that comment is archived"]],"data":{"things":[]}}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "morechildren", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoMoreChildren(req)
	if err == nil {
		t.Fatal("expected API error, got nil")
	}
	if things != nil {
		t.Fatalf("expected nil Things on error, got %v", things)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !bytes.Contains([]byte(apiErr.Error()), []byte("THREAD_LOCKED")) {
		t.Errorf("expected error to contain 'THREAD_LOCKED', got %q", apiErr.Error())
	}
}

func TestClient_DoMoreChildren_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad json`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "bad", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoMoreChildren(req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if things != nil {
		t.Fatalf("expected nil Things on error, got %v", things)
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("expected ClientError, got %T", err)
	}
}

func TestClient_DoMoreChildren_MalformedStructure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"different":"structure"}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, server.URL+"/", "agent", nil)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodGet, "malformed", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	things, err := c.DoMoreChildren(req)
	if err != nil {
		t.Fatalf("DoMoreChildren should handle missing nested fields, got error: %v", err)
	}

	if len(things) != 0 {
		t.Fatalf("expected empty Things for missing data.things field, got %d", len(things))
	}
}
