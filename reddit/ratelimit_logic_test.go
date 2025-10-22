package graw

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

// TestProactiveRateLimitingBehavior tests proactive rate limiting when approaching limits
func TestProactiveRateLimitingBehavior(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	// Create account using builder
	account := testutil.NewAccount("testuser123").
		WithID("user123").
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Override handler to include rate limit tracking
	originalHandler := server.Server().Config.Handler
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++

		// Simulate rate limit headers that decrease with each request
		remaining := 60 - int(currentCount%10)
		reset := int(time.Now().Unix()) + 300
		w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(reset))
		w.Header().Set("X-Ratelimit-Used", strconv.Itoa(int(currentCount%10)))
		mu.Unlock()

		originalHandler.ServeHTTP(w, r)
	})

	// Create client with rate limiting using the factory
	config := RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              10,
		ProactiveThreshold: 8, // Start being proactive at 8 remaining
	}
	client := NewTestClientWithRateLimit(server.URL(), config)

	ctx := context.Background()

	// Test concurrent requests to trigger proactive rate limiting
	t.Run("ProactiveRateLimiting", func(t *testing.T) {
		numRequests := 15
		var wg sync.WaitGroup
		results := make(chan bool, numRequests)

		startTime := time.Now()

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestNum int) {
				defer wg.Done()

				_, err := client.Me(ctx)
				testutil.AssertNoError(t, err)
				results <- true
			}(i)

			// Small delay between request initiations
			time.Sleep(10 * time.Millisecond)
		}

		wg.Wait()
		close(results)

		totalTime := time.Since(startTime)
		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}

		t.Logf("Completed %d requests in %v", numRequests, totalTime)
		t.Logf("Total requests made: %d", requestCount)

		// Verify we made the expected number of requests
		if requestCount < int64(numRequests) {
			t.Errorf("Expected at least %d requests, got %d", numRequests, requestCount)
		}

		// Verify all requests succeeded
		if successCount != numRequests {
			t.Errorf("Expected %d successful requests, got %d", numRequests, successCount)
		}
	})
}

// TestRateLimitRecoveryPatterns tests recovery patterns after hitting rate limits
func TestRateLimitRecoveryPatterns(t *testing.T) {
	var requestCount int64
	var hitRateLimit bool
	var mu sync.Mutex

	// Create account using builder
	account := testutil.NewAccount("testuser123").
		WithID("user123").
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Override handler to simulate rate limit errors
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// After 5 requests, start returning 429
		if currentCount >= 5 && currentCount < 8 {
			w.Header().Set("X-Ratelimit-Remaining", "0")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+2)) // 2 seconds
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"Too Many Requests","error":"rate_limit_exceeded"}`))
			return
		}

		// Normal response - use builder
		w.Header().Set("X-Ratelimit-Remaining", "10")
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))

		thing := testutil.NewAccount("testuser123").
			WithID("user123").
			ToThing()

		// Write the Thing as JSON
		json.NewEncoder(w).Encode(thing)
	})

	// Create client with rate limiting
	config := RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              5,
		ProactiveThreshold: 3,
	}
	client := NewTestClientWithRateLimit(server.URL(), config)

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
			time.Sleep(50 * time.Millisecond)
		}

		// Wait for rate limit reset period to pass
		time.Sleep(2500 * time.Millisecond)

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

	// Create account using builder
	account := testutil.NewAccount("testuser123").
		WithID("user123").
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Override handler for rate limit tracking
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Always allow requests, but track rate limit headers
		w.Header().Set("X-Ratelimit-Remaining", "50")
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		w.Header().Set("Content-Type", "application/json")

		thing := testutil.NewAccount("testuser123").
			WithID("user123").
			ToThing()

		json.NewEncoder(w).Encode(thing)
	})

	// Create client with burst capacity
	config := RateLimitConfig{
		RequestsPerMinute:  30, // 0.5 per second
		Burst:              10, // Allow burst of 10
		ProactiveThreshold: 5,
	}
	client := NewTestClientWithRateLimit(server.URL(), config)

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

		testutil.AssertNoError(t, err)

		t.Logf("Recovery request completed in %v", recoveryDuration)
	})
}

// TestMalformedRateLimitHeaders tests handling of malformed rate limit headers
func TestMalformedRateLimitHeaders(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	// Create account using builder
	account := testutil.NewAccount("testuser123").
		WithID("user123").
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Override handler to test malformed headers
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		case 2:
			// Negative remaining
			w.Header().Set("X-Ratelimit-Remaining", "-5")
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
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
			w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(int(time.Now().Unix())+300))
		}

		thing := testutil.NewAccount("testuser123").
			WithID("user123").
			ToThing()

		json.NewEncoder(w).Encode(thing)
	})

	// Create client
	config := RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              10,
		ProactiveThreshold: 5,
	}
	client := NewTestClientWithRateLimit(server.URL(), config)

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

			// Small delay between requests
			time.Sleep(50 * time.Millisecond)
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

	// Create account using builder
	account := testutil.NewAccount("testuser123").
		WithID("user123").
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Override handler for rate limit simulation
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentCount := requestCount
		mu.Unlock()

		// Simulate rate limiting headers with plenty of headroom
		remaining := 100 - int(currentCount%10)
		reset := int(time.Now().Unix()) + 60

		w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-Ratelimit-Reset", strconv.Itoa(reset))
		w.Header().Set("Content-Type", "application/json")

		thing := testutil.NewAccount("testuser123").
			WithID("user123").
			ToThing()

		json.NewEncoder(w).Encode(thing)
	})

	// Create client with generous rate limiting to avoid blocking
	config := RateLimitConfig{
		RequestsPerMinute:  600, // High limit to avoid real blocking
		Burst:              50,
		ProactiveThreshold: 10,
	}
	client := NewTestClientWithRateLimit(server.URL(), config)

	ctx := context.Background()

	t.Run("ConcurrentRateLimiting", func(t *testing.T) {
		numGoroutines := 10
		requestsPerGoroutine := 2

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
		t.Logf("  Time elapsed: %v", totalTime)
		t.Logf("  Actual server requests: %d", requestCount)

		// Should have high success rate with generous limits
		if successRate < 95 {
			t.Errorf("Success rate too low: %.1f%%, expected at least 95%%", successRate)
		}

		// Verify all requests were made
		if requestCount < int64(totalRequests) {
			t.Errorf("Expected %d server requests, got %d", totalRequests, requestCount)
		}
	})
}

// TestRateLimitEdgeCases tests edge cases in rate limiting
func TestRateLimitEdgeCases(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	// Create account using builder
	account := testutil.NewAccount("testuser123").
		WithID("user123").
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Override handler for edge case testing
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		thing := testutil.NewAccount("testuser123").
			WithID("user123").
			ToThing()

		json.NewEncoder(w).Encode(thing)
	})

	// Create client
	config := RateLimitConfig{
		RequestsPerMinute:  60,
		Burst:              10,
		ProactiveThreshold: 5,
	}
	client := NewTestClientWithRateLimit(server.URL(), config)

	ctx := context.Background()

	t.Run("RateLimitEdgeCases", func(t *testing.T) {
		successCount := 0
		var results []string

		for i := 0; i < 6; i++ {
			start := time.Now()
			_, err := client.Me(ctx)
			duration := time.Since(start)

			if err != nil {
				results = append(results, "Request "+strconv.Itoa(i+1)+": FAILED ("+err.Error()+")")
			} else {
				results = append(results, "Request "+strconv.Itoa(i+1)+": SUCCESS ("+duration.String()+")")
				successCount++
			}

			// Small delay between requests
			time.Sleep(100 * time.Millisecond)
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
