package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"golang.org/x/time/rate"
)

func TestNewClient_DefaultRateLimiter(t *testing.T) {
	client, err := NewClient(nil, "token", "https://example.com/api/", "agent", nil)
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

func TestClient_NewRequestSetsHeaders(t *testing.T) {
	httpClient := &http.Client{}
	c, err := NewClient(httpClient, "token-value", "https://example.com", "my-agent", &RateLimitConfig{RequestsPerMinute: 1000, Burst: 100})
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

func TestClient_DoDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kind":"t3","data":{"id":"abc123"}}`))
	}))
	t.Cleanup(server.Close)

	httpClient := server.Client()
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 1000, Burst: 100})
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
	c, err := NewClient(httpClient, "token", server.URL+"/", "agent", &RateLimitConfig{RequestsPerMinute: 60000, Burst: 1000})
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
