package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
)

// TestNetworkTimeoutRecovery tests recovery from network timeouts
func TestNetworkTimeoutRecovery(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		// First request times out, subsequent requests succeed
		if currentRequest == 1 {
			// Simulate timeout by delaying response
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		} else {
			// Normal response for subsequent requests
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		}
	}))
	defer server.Close()

	// Create client with short timeout to trigger timeout on first request
	httpClient := &http.Client{Timeout: 500 * time.Millisecond}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// First request should timeout
	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Create new client with longer timeout for recovery
	httpClient2 := &http.Client{Timeout: 5 * time.Second}
	internalClient2, err := internal.NewClient(httpClient2, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create recovery client: %v", err)
	}

	recoveryClient := &Client{
		client:    internalClient2,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Second request should succeed
	subreddit, err := recoveryClient.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if requestCount < 2 {
		t.Errorf("Expected at least 2 requests, got: %d", requestCount)
	}

	t.Logf("Successfully recovered from network timeout after %d requests", requestCount)
}

// TestConnectionRefusedRecovery tests recovery from connection refused errors
func TestConnectionRefusedRecovery(t *testing.T) {
	// Start with a server that will be closed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		subredditData := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name": "testsub",
				"subscribers":  100000,
			},
		}
		json.NewEncoder(w).Encode(subredditData)
	}))

	// Create client
	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Close the server to simulate connection refused
	server.Close()

	// Request should fail with connection refused error
	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected connection refused error, but got none")
	}

	if !strings.Contains(err.Error(), "connection refused") &&
		!strings.Contains(err.Error(), "connect") {
		t.Errorf("Expected connection refused error, got: %v", err)
	}

	// Start a new server for recovery
	recoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		subredditData := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name": "testsub",
				"subscribers":  100000,
			},
		}
		json.NewEncoder(w).Encode(subredditData)
	}))
	defer recoveryServer.Close()

	// Create new client pointing to recovery server
	recoveryHttpClient := &http.Client{Timeout: 5 * time.Second}
	recoveryInternalClient, err := internal.NewClient(recoveryHttpClient, recoveryServer.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create recovery internal client: %v", err)
	}

	recoveryClient := &Client{
		client:    recoveryInternalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Recovery request should succeed
	subreddit, err := recoveryClient.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	t.Logf("Successfully recovered from connection refused error")
}

// TestDNSFailureRecovery tests recovery from DNS resolution failures
func TestDNSFailureRecovery(t *testing.T) {
	// Create client pointing to non-existent domain
	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, "http://non-existent-domain-for-testing.invalid", "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Request should fail with DNS resolution error
	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected DNS resolution error, but got none")
	}

	if !strings.Contains(err.Error(), "no such host") &&
		!strings.Contains(err.Error(), "dns") &&
		!strings.Contains(err.Error(), "lookup") {
		t.Errorf("Expected DNS resolution error, got: %v", err)
	}

	// Create working server for recovery
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		subredditData := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name": "testsub",
				"subscribers":  100000,
			},
		}
		json.NewEncoder(w).Encode(subredditData)
	}))
	defer server.Close()

	// Create new client pointing to working server
	recoveryHttpClient := &http.Client{Timeout: 5 * time.Second}
	recoveryInternalClient, err := internal.NewClient(recoveryHttpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create recovery internal client: %v", err)
	}

	recoveryClient := &Client{
		client:    recoveryInternalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Recovery request should succeed
	subreddit, err := recoveryClient.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	t.Logf("Successfully recovered from DNS resolution failure")
}

// TestHTTP5xxErrorRecovery tests recovery from server errors (5xx)
func TestHTTP5xxErrorRecovery(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// First few requests return 500, then succeed
		if currentRequest <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			errorResponse := map[string]interface{}{
				"error":   "Internal Server Error",
				"message": "Simulated server error",
			}
			json.NewEncoder(w).Encode(errorResponse)
		} else {
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// First few requests should fail with 500 errors
	for i := 0; i < 3; i++ {
		_, err = client.GetSubreddit(ctx, "testsub")
		if err == nil {
			t.Errorf("Expected server error for request %d, but got none", i+1)
		}

		if !strings.Contains(err.Error(), "500") &&
			!strings.Contains(err.Error(), "Internal Server Error") {
			t.Errorf("Expected 500 error for request %d, got: %v", i+1, err)
		}
	}

	// Fourth request should succeed
	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if requestCount != 4 {
		t.Errorf("Expected 4 requests, got: %d", requestCount)
	}

	t.Logf("Successfully recovered from HTTP 500 errors after %d attempts", requestCount)
}

// TestHTTP429RateLimitRecovery tests recovery from rate limiting (429)
func TestHTTP429RateLimitRecovery(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		// First request returns 429, subsequent requests succeed
		if currentRequest == 1 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Second).Unix()))
			w.WriteHeader(http.StatusTooManyRequests)
			errorResponse := map[string]interface{}{
				"error":   "Too Many Requests",
				"message": "Rate limit exceeded",
			}
			json.NewEncoder(w).Encode(errorResponse)
		} else {
			w.Header().Set("X-RateLimit-Remaining", "60")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// First request should be rate limited
	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected rate limit error, but got none")
	}

	if !strings.Contains(err.Error(), "429") &&
		!strings.Contains(err.Error(), "Too Many Requests") {
		t.Errorf("Expected rate limit error, got: %v", err)
	}

	// Wait for rate limit to reset (in real scenario, this would be longer)
	time.Sleep(100 * time.Millisecond)

	// Second request should succeed
	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests, got: %d", requestCount)
	}

	t.Logf("Successfully recovered from rate limiting after %d requests", requestCount)
}

// TestPartialResponseRecovery tests recovery from partial/incomplete responses
func TestPartialResponseRecovery(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if currentRequest == 1 {
			// Send incomplete JSON
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"kind": "t5", "data": {"display_name":`))
			// Close connection abruptly
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
		} else {
			// Send complete response
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// First request should fail due to incomplete response
	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected parse error, but got none")
	}

	if !strings.Contains(err.Error(), "parse") &&
		!strings.Contains(err.Error(), "JSON") &&
		!strings.Contains(err.Error(), "connection") {
		t.Errorf("Expected parse/connection error, got: %v", err)
	}

	// Second request should succeed
	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests, got: %d", requestCount)
	}

	t.Logf("Successfully recovered from partial response after %d requests", requestCount)
}

// TestIntermittentNetworkFailure tests recovery from intermittent network failures
func TestIntermittentNetworkFailure(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Simulate intermittent failures
		if currentRequest%3 == 0 {
			// Every third request fails
			w.WriteHeader(http.StatusServiceUnavailable)
			errorResponse := map[string]interface{}{
				"error":   "Service Unavailable",
				"message": "Simulated intermittent failure",
			}
			json.NewEncoder(w).Encode(errorResponse)
		} else {
			// Other requests succeed
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	successCount := 0
	failureCount := 0

	// Make multiple requests to test intermittent failures
	for i := 0; i < 6; i++ {
		_, err = client.GetSubreddit(ctx, "testsub")
		if err != nil {
			failureCount++
			// Every third request should fail
			if (i+1)%3 != 0 {
				t.Errorf("Unexpected failure on request %d: %v", i+1, err)
			}
		} else {
			successCount++
			// Requests that aren't multiples of 3 should succeed
			if (i+1)%3 == 0 {
				t.Errorf("Expected failure on request %d, but it succeeded", i+1)
			}
		}
	}

	if successCount != 4 {
		t.Errorf("Expected 4 successful requests, got: %d", successCount)
	}

	if failureCount != 2 {
		t.Errorf("Expected 2 failed requests, got: %d", failureCount)
	}

	if requestCount != 6 {
		t.Errorf("Expected 6 total requests, got: %d", requestCount)
	}

	t.Logf("Successfully handled intermittent failures: %d successes, %d failures", successCount, failureCount)
}

// TestNetworkRecoveryWithRetry tests automatic retry logic for network failures
func TestNetworkRecoveryWithRetry(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Fail first 2 requests, succeed on 3rd
		if currentRequest <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			errorResponse := map[string]interface{}{
				"error":   "Bad Gateway",
				"message": "Simulated temporary failure",
			}
			json.NewEncoder(w).Encode(errorResponse)
		} else {
			w.WriteHeader(http.StatusOK)
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Implement simple retry logic
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry (exponential backoff would be better in production)
			time.Sleep(100 * time.Millisecond * time.Duration(attempt))
		}

		subreddit, err := client.GetSubreddit(ctx, "testsub")
		if err == nil {
			// Success!
			if subreddit.DisplayName != "testsub" {
				t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
			}
			t.Logf("Successfully recovered after %d attempts", attempt+1)
			return
		}

		lastErr = err
		t.Logf("Attempt %d failed: %v", attempt+1, err)
	}

	// If we get here, all retries failed
	t.Fatalf("All %d retry attempts failed, last error: %v", maxRetries, lastErr)
}

// TestContextCancellationDuringRecovery tests context cancellation during recovery attempts
func TestContextCancellationDuringRecovery(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Always fail to test cancellation
		w.WriteHeader(http.StatusInternalServerError)
		errorResponse := map[string]interface{}{
			"error":   "Internal Server Error",
			"message": "Persistent failure for cancellation test",
		}
		json.NewEncoder(w).Encode(errorResponse)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Create context that cancels after 200ms
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Try multiple requests with cancellation
	attempts := 0
	for {
		attempts++
		_, err := client.GetSubreddit(ctx, "testsub")
		if err != nil {
			if err == context.DeadlineExceeded {
				t.Logf("Successfully cancelled after %d attempts", attempts)
				break
			}
			// Continue trying on other errors
		}

		// Small delay between attempts
		time.Sleep(50 * time.Millisecond)
	}

	if attempts == 0 {
		t.Error("Expected at least one attempt before cancellation")
	}

	if requestCount == 0 {
		t.Error("Expected at least one request to be made")
	}

	t.Logf("Context cancellation test completed after %d attempts and %d requests", attempts, requestCount)
}
