package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
)

// TestProactiveRateLimitingBehavior tests proactive rate limiting when approaching limits
func TestProactiveRateLimitingBehavior(t *testing.T) {
	var requestCount int64
	var lastRequestTime time.Time
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++
		now := time.Now()

		// Track timing between requests
		if !lastRequestTime.IsZero() {
			timeSinceLast := now.Sub(lastRequestTime)
			t.Logf("Request %d came %v after request %d", currentCount, timeSinceLast, currentCount-1)
		}
		lastRequestTime = now
		mu.Unlock()

		// Simulate rate limit headers that decrease with each request
		remaining := 60 - int(currentCount%10)
		reset := int(time.Now().Unix()) + 300

		w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(reset))
		w.Header().Set("X-Ratelimit-Used", strconv.Itoa(int(currentCount%10)))
		w.Header().Set("Content-Type", "application/json")

		// Simple response
		response := map[string]interface{}{
			"kind": "t2",
			"data": map[string]interface{}{
				"id":   "user123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with rate limiting
	httpClient := &http.Client{Timeout: 30 * time.Second}
	rateLimitConfig := internal.RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              10,
		ProactiveThreshold: 8, // Start being proactive at 8 remaining
	}

	internalClient, err := internal.NewClientWithRateLimit(httpClient, server.URL, "test/1.0", nil, rateLimitConfig)
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

	// Test concurrent requests to trigger proactive rate limiting
	t.Run("ProactiveRateLimiting", func(t *testing.T) {
		numRequests := 15
		var wg sync.WaitGroup
		results := make(chan time.Duration, numRequests)

		startTime := time.Now()

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestNum int) {
				defer wg.Done()

				requestStart := time.Now()
				_, err := client.Me(ctx)
				requestDuration := time.Since(requestStart)

				if err != nil {
					t.Errorf("Request %d failed: %v", requestNum, err)
				}

				results <- requestDuration
			}(i)
		}

		wg.Wait()
		close(results)

		totalTime := time.Since(startTime)
		var totalRequestDuration time.Duration
		count := 0
		for duration := range results {
			totalRequestDuration += duration
			count++
		}

		avgRequestDuration := totalRequestDuration / time.Duration(count)

		t.Logf("Completed %d requests in %v", numRequests, totalTime)
		t.Logf("Average request duration: %v", avgRequestDuration)
		t.Logf("Total requests made: %d", requestCount)

		// Verify that rate limiting is working (requests should be spread out)
		if totalTime < time.Duration(numRequests-1)*time.Second {
			t.Errorf("Requests completed too quickly (%v), rate limiting may not be working", totalTime)
		}

		// Verify we made the expected number of requests
		if requestCount < int64(numRequests) {
			t.Errorf("Expected at least %d requests, got %d", numRequests, requestCount)
		}
	})
}

// TestRateLimitRecoveryPatterns tests recovery patterns after hitting rate limits
func TestRateLimitRecoveryPatterns(t *testing.T) {
	var requestCount int64
	var hitRateLimit bool
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++
		mu.Unlock()

		// After 5 requests, start returning 429
		if currentCount >= 5 && currentCount < 8 {
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+2)) // 2 seconds
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Too Many Requests",
				"error":   "rate_limit_exceeded",
			})
			return
		}

		// Normal response
		w.Header().Set("X-Ratelimit-Remaining", "10")
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"kind": "t2",
			"data": map[string]interface{}{
				"id":   "user123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with rate limiting
	httpClient := &http.Client{Timeout: 30 * time.Second}
	rateLimitConfig := internal.RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              5,
		ProactiveThreshold: 3,
	}

	internalClient, err := internal.NewClientWithRateLimit(httpClient, server.URL, "test/1.0", nil, rateLimitConfig)
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

	t.Run("RateLimitRecovery", func(t *testing.T) {
		// Make requests that will hit rate limit
		successCount := 0
		errorCount := 0

		for i := 0; i < 10; i++ {
			_, err := client.Me(ctx)
			if err != nil {
				errorCount++
				t.Logf("Request %d failed: %v", i+1, err)

				// Check if it's a rate limit error
				if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too Many Requests") {
					hitRateLimit = true
					t.Logf("Hit rate limit on request %d", i+1)
				}
			} else {
				successCount++
				t.Logf("Request %d succeeded", i+1)
			}

			// Small delay between requests
			time.Sleep(100 * time.Millisecond)
		}

		t.Logf("Success: %d, Errors: %d", successCount, errorCount)

		// Verify we hit the rate limit
		if !hitRateLimit {
			t.Error("Expected to hit rate limit, but didn't")
		}

		// Verify we eventually recovered
		if successCount < 5 {
			t.Errorf("Expected at least 5 successful requests after recovery, got %d", successCount)
		}

		// Verify total requests made
		if requestCount < 10 {
			t.Errorf("Expected at least 10 total requests, got %d", requestCount)
		}
	})
}

// TestBurstCapacityHandling tests burst capacity and recovery
func TestBurstCapacityHandling(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Always allow requests, but track rate limit headers
		w.Header().Set("X-Ratelimit-Remaining", "50")
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"kind": "t2",
			"data": map[string]interface{}{
				"id":   "user123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with burst capacity
	httpClient := &http.Client{Timeout: 30 * time.Second}
	rateLimitConfig := internal.RateLimitConfig{
		RequestsPerMinute:  30, // 0.5 per second
		Burst:              10, // Allow burst of 10
		ProactiveThreshold: 5,
	}

	internalClient, err := internal.NewClientWithRateLimit(httpClient, server.URL, "test/1.0", nil, rateLimitConfig)
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

	t.Run("BurstCapacity", func(t *testing.T) {
		// Test burst capacity - make 8 requests quickly
		burstStart := time.Now()
		var wg sync.WaitGroup
		burstSuccess := make(chan bool, 8)

		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(requestNum int) {
				defer wg.Done()

				_, err := client.Me(ctx)
				success := err == nil
				burstSuccess <- success

				if err != nil {
					t.Logf("Burst request %d failed: %v", requestNum, err)
				} else {
					t.Logf("Burst request %d succeeded", requestNum)
				}
			}(i)
		}

		wg.Wait()
		close(burstSuccess)

		burstDuration := time.Since(burstStart)
		burstSuccessCount := 0
		for success := range burstSuccess {
			if success {
				burstSuccessCount++
			}
		}

		t.Logf("Burst of 8 requests completed in %v with %d successes", burstDuration, burstSuccessCount)

		// Most of the burst should succeed (within burst capacity)
		if burstSuccessCount < 6 {
			t.Errorf("Expected at least 6 successful requests in burst, got %d", burstSuccessCount)
		}

		// Wait for burst to recover
		t.Logf("Waiting for burst recovery...")
		time.Sleep(5 * time.Second)

		// Test that burst has recovered
		recoveryStart := time.Now()
		_, err := client.Me(ctx)
		recoveryDuration := time.Since(recoveryStart)

		if err != nil {
			t.Errorf("Recovery request failed: %v", err)
		}

		t.Logf("Recovery request completed in %v", recoveryDuration)

		// Recovery should be relatively fast
		if recoveryDuration > 2*time.Second {
			t.Errorf("Recovery took too long: %v", recoveryDuration)
		}
	})
}

// TestMalformedRateLimitHeaders tests handling of malformed rate limit headers
func TestMalformedRateLimitHeaders(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Test various malformed headers
		switch currentCount {
		case 1:
			// Non-numeric remaining
			w.Header().Set("X-Ratelimit-Remaining", "invalid")
			w.Header().Set("X-Ratelimit-Reset", "123456789")
		case 2:
			// Negative remaining
			w.Header().Set("X-Ratelimit-Remaining", "-5")
			w.Header().Set("X-Ratelimit-Reset", "123456789")
		case 3:
			// Non-numeric reset
			w.Header().Set("X-Ratelimit-Remaining", "10")
			w.Header().Set("X-Ratelimit-Reset", "invalid")
		case 4:
			// Missing headers
			// Don't set any rate limit headers
		case 5:
			// Extremely large values
			w.Header().Set("X-Ratelimit-Remaining", "999999")
			w.Header().Set("X-Ratelimit-Reset", "999999999")
		default:
			// Normal headers
			w.Header().Set("X-Ratelimit-Remaining", "50")
			w.Header().Set("X-Ratelimit-Reset", "123456789")
		}

		response := map[string]interface{}{
			"kind": "t2",
			"data": map[string]interface{}{
				"id":   "user123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client
	httpClient := &http.Client{Timeout: 30 * time.Second}
	rateLimitConfig := internal.RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              10,
		ProactiveThreshold: 5,
	}

	internalClient, err := internal.NewClientWithRateLimit(httpClient, server.URL, "test/1.0", nil, rateLimitConfig)
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

	t.Run("MalformedHeaders", func(t *testing.T) {
		successCount := 0

		for i := 0; i < 7; i++ {
			_, err := client.Me(ctx)
			if err != nil {
				t.Logf("Request %d failed with malformed headers: %v", i+1, err)
			} else {
				successCount++
				t.Logf("Request %d succeeded despite malformed headers", i+1)
			}

			time.Sleep(100 * time.Millisecond)
		}

		t.Logf("Successfully handled %d/7 requests with malformed headers", successCount)

		// Client should be resilient to malformed headers
		if successCount < 5 {
			t.Errorf("Expected at least 5 successful requests with malformed headers, got %d", successCount)
		}
	})
}

// TestConcurrentRateLimiting tests rate limiting under concurrent load
func TestConcurrentRateLimiting(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentCount := requestCount
		mu.Unlock()

		// Simulate strict rate limiting
		remaining := 60 - int(currentCount%60)
		reset := int(time.Now().Unix()) + 60

		w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(reset))
		w.Header().Set("Content-Type", "application/json")

		response := map[string]interface{}{
			"kind": "t2",
			"data": map[string]interface{}{
				"id":   "user123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with conservative rate limiting
	httpClient := &http.Client{Timeout: 30 * time.Second}
	rateLimitConfig := internal.RateLimitConfig{
		RequestsPerMinute:  30, // 0.5 per second
		Burst:              5,
		ProactiveThreshold: 3,
	}

	internalClient, err := internal.NewClientWithRateLimit(httpClient, server.URL, "test/1.0", nil, rateLimitConfig)
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

	t.Run("ConcurrentRateLimiting", func(t *testing.T) {
		numGoroutines := 20
		requestsPerGoroutine := 3

		var wg sync.WaitGroup
		results := make(chan bool, numGoroutines*requestsPerGoroutine)

		startTime := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for j := 0; j < requestsPerGoroutine; j++ {
					_, err := client.Me(ctx)
					success := err == nil
					results <- success

					if err != nil {
						t.Logf("Goroutine %d, request %d failed: %v", goroutineID, j+1, err)
					}
				}
			}(i)
		}

		wg.Wait()
		close(results)

		totalTime := time.Since(startTime)
		successCount := 0
		totalRequests := 0

		for success := range results {
			totalRequests++
			if success {
				successCount++
			}
		}

		successRate := float64(successCount) / float64(totalRequests) * 100

		t.Logf("Concurrent test completed:")
		t.Logf("  Total requests: %d", totalRequests)
		t.Logf("  Successful: %d (%.1f%%)", successCount, successRate)
		t.Logf("  Total time: %v", totalTime)
		t.Logf("  Actual server requests: %d", requestCount)

		// Verify rate limiting is working (should take some time)
		expectedMinTime := time.Duration(totalRequests/30) * time.Minute
		if totalTime < expectedMinTime/2 { // Allow some tolerance
			t.Errorf("Requests completed too quickly (%v), expected at least %v", totalTime, expectedMinTime/2)
		}

		// Should have reasonable success rate
		if successRate < 80 {
			t.Errorf("Success rate too low: %.1f%%, expected at least 80%%", successRate)
		}
	})
}

// TestRateLimitEdgeCases tests edge cases in rate limiting
func TestRateLimitEdgeCases(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Test edge case values
		switch currentCount {
		case 1:
			// Zero remaining
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+1))
		case 2:
			// One remaining
			w.Header().Set("X-Ratelimit-Remaining", "1")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		case 3:
			// Reset time in the past
			w.Header().Set("X-Ratelimit-Remaining", "10")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())-3600))
		case 4:
			// Reset time far in future
			w.Header().Set("X-Ratelimit-Remaining", "5")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+86400)) // 24 hours
		default:
			// Normal values
			w.Header().Set("X-Ratelimit-Remaining", "30")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		}

		response := map[string]interface{}{
			"kind": "t2",
			"data": map[string]interface{}{
				"id":   "user123",
				"name": "testuser",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client
	httpClient := &http.Client{Timeout: 30 * time.Second}
	rateLimitConfig := internal.RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              10,
		ProactiveThreshold: 5,
	}

	internalClient, err := internal.NewClientWithRateLimit(httpClient, server.URL, "test/1.0", nil, rateLimitConfig)
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

	t.Run("RateLimitEdgeCases", func(t *testing.T) {
		successCount := 0
		var results []string

		for i := 0; i < 6; i++ {
			start := time.Now()
			_, err := client.Me(ctx)
			duration := time.Since(start)

			if err != nil {
				results = append(results, fmt.Sprintf("Request %d: FAILED (%v)", i+1, err))
			} else {
				results = append(results, fmt.Sprintf("Request %d: SUCCESS (%v)", i+1, duration))
				successCount++
			}

			// Small delay between requests
			time.Sleep(200 * time.Millisecond)
		}

		for _, result := range results {
			t.Log(result)
		}

		t.Logf("Edge cases test: %d/6 requests successful", successCount)

		// Should handle edge cases gracefully
		if successCount < 4 {
			t.Errorf("Expected at least 4 successful requests with edge case headers, got %d", successCount)
		}
	})
}
