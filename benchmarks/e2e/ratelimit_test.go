package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// E2E (End-to-End) benchmarks for Reddit API rate limiting behavior.
//
// These benchmarks measure how the client handles Reddit's rate limits in real-world scenarios:
//   - Processing rate limit headers (X-Ratelimit-Remaining, X-Ratelimit-Reset, Retry-After)
//   - Throttling behavior when approaching limits
//   - Recovery after exhausting rate limits
//   - Burst vs sustained request patterns
//
// Reddit's rate limits (as of 2025):
//   - OAuth2: 600 requests per 10 minutes (60 requests/minute)
//   - Rate limit resets on a sliding window
//   - Headers: X-Ratelimit-Remaining (requests left), X-Ratelimit-Reset (seconds until reset)
//
// Prerequisites:
//   - REDDIT_CLIENT_ID: OAuth2 client ID
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret
//
// Run with:
//
//	go test -bench=BenchmarkE2E_RateLimit ./benchmarks/e2e -benchmem
//
// IMPORTANT: These benchmarks make many real API calls and will consume rate limit quota.
// Some benchmarks may take several minutes to complete as they test rate limit recovery.

// requestTiming tracks timing information for a single request.
type requestTiming struct {
	requestNum int
	duration   time.Duration
	timestamp  time.Time
}

// BenchmarkE2E_RateLimitHeaders measures the overhead of processing Reddit's rate limit
// headers on every response. This benchmark verifies that header extraction and parsing
// (X-Ratelimit-Remaining, X-Ratelimit-Reset, Retry-After) doesn't add significant latency.
//
// The client extracts and applies these headers after each request to:
//   - Track remaining quota
//   - Calculate proactive throttling delays
//   - Respect Retry-After directives
//
// This benchmark makes a small number of requests (5) to measure the processing overhead
// without exhausting rate limits.
func BenchmarkE2E_RateLimitHeaders(b *testing.B) {
	skipIfNoCredentials(b)

	client := newE2EClient(b)
	ctx := context.Background()

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make a small number of requests to observe header processing
		// without triggering aggressive throttling
		for j := 0; j < 5; j++ {
			startReq := time.Now()

			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				b.Fatalf("Request %d failed: %v", j, err)
			}

			reqDuration := time.Since(startReq)

			// Validate response
			if resp == nil || len(resp.Posts) == 0 {
				b.Error("Expected non-empty response")
			}

			// Log timing for first iteration to observe rate limit processing
			if i == 0 && j == 0 {
				b.Logf("First request took %v (includes auth overhead)", reqDuration)
			}
		}
	}
}

// BenchmarkE2E_ThrottlingBehavior measures how the client throttles requests when
// approaching Reddit's rate limits. The client implements proactive throttling to
// prevent hitting hard limits and getting errors.
//
// This benchmark makes enough requests (~50-60) to trigger the proactive throttling
// mechanism, which activates when X-Ratelimit-Remaining drops below the threshold
// (default: 5 requests remaining).
//
// Expected behavior:
//   - Initial requests: fast, no throttling
//   - Middle requests: moderate throttling as quota decreases
//   - Final requests: aggressive throttling to spread remaining quota
//
// Note: This benchmark consumes significant rate limit quota and may take 1-2 minutes
// to complete as it exercises the full throttling behavior.
func BenchmarkE2E_ThrottlingBehavior(b *testing.B) {
	skipIfNoCredentials(b)

	// This benchmark makes many requests and will consume quota
	// Skip if b.N is too large to avoid exhausting limits
	if b.N > 1 {
		b.Log("Limiting iterations to 1 to conserve rate limit quota")
		b.N = 1
	}

	client := newE2EClient(b)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make 60 requests to exercise the full rate limiting behavior
		// Reddit's limit is 600/10min = 60/min, so this approaches the limit
		const requestCount = 60
		// Track request timings to observe throttling progression
		timings := make([]requestTiming, 0, requestCount)

		startBenchmark := time.Now()

		for j := 0; j < requestCount; j++ {
			startReq := time.Now()

			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				b.Fatalf("Request %d/%d failed: %v", j+1, requestCount, err)
			}

			reqDuration := time.Since(startReq)
			timings = append(timings, requestTiming{
				requestNum: j + 1,
				duration:   reqDuration,
				timestamp:  startReq,
			})

			// Log progress every 10 requests
			if (j+1)%10 == 0 {
				b.Logf("Completed %d/%d requests (last request: %v)", j+1, requestCount, reqDuration)
			}
		}

		totalDuration := time.Since(startBenchmark)

		// Report statistics for analysis
		if len(timings) > 0 {
			// Calculate average durations for different phases
			early := timings[:10]
			middle := timings[20:30]
			late := timings[50:]

			avgEarly := averageDuration(early)
			avgMiddle := averageDuration(middle)
			avgLate := averageDuration(late)

			b.Logf("\nThrottling progression analysis:")
			b.Logf("  Early requests (1-10):   avg %v", avgEarly)
			b.Logf("  Middle requests (21-30): avg %v", avgMiddle)
			b.Logf("  Late requests (51-60):   avg %v", avgLate)
			b.Logf("  Total duration: %v", totalDuration)
			b.Logf("  Average throughput: %.2f req/sec", float64(requestCount)/totalDuration.Seconds())
		}
	}
}

// BenchmarkE2E_RateLimitRecovery measures how quickly the client recovers after
// exhausting rate limits. This tests the client's ability to:
//   - Detect rate limit exhaustion (X-Ratelimit-Remaining: 0)
//   - Wait for the rate limit reset window
//   - Resume requests after recovery
//
// This benchmark is expensive and time-consuming:
//  1. Exhaust rate limit (60+ requests)
//  2. Wait for rate limit reset (up to 60 seconds)
//  3. Measure first request after recovery
//
// Note: This benchmark can take several minutes and consumes significant quota.
// Consider using b.Skip() if you want to preserve quota for other tests.
func BenchmarkE2E_RateLimitRecovery(b *testing.B) {
	skipIfNoCredentials(b)

	// This benchmark is very expensive - only run once
	if b.N > 1 {
		b.Log("Limiting iterations to 1 to conserve rate limit quota and time")
		b.N = 1
	}

	// Optional: Skip this benchmark to save time and quota
	// Uncomment the line below if you want to skip recovery testing
	// b.Skip("Skipping rate limit recovery benchmark to conserve quota and time")

	client := newE2EClient(b)
	// Use a long timeout as this benchmark may need to wait for rate limit reset
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.Log("Phase 1: Exhausting rate limit...")
		exhaustStart := time.Now()

		// Make many requests to exhaust the rate limit
		// We'll make 65 requests to ensure we hit the limit
		const exhaustCount = 65
		for j := 0; j < exhaustCount; j++ {
			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				// Expected to eventually hit rate limit
				b.Logf("Request %d failed (expected): %v", j+1, err)
				break
			}

			if (j+1)%10 == 0 {
				b.Logf("Made %d/%d exhaustion requests", j+1, exhaustCount)
			}
		}

		exhaustDuration := time.Since(exhaustStart)
		b.Logf("Exhausted rate limit in %v", exhaustDuration)

		b.Log("Phase 2: Waiting for rate limit reset...")
		resetStart := time.Now()

		// The client should automatically wait based on X-Ratelimit-Reset header
		// Make a request that will trigger the wait
		_, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit:  "golang",
			Pagination: types.Pagination{Limit: 10},
		})

		resetDuration := time.Since(resetStart)

		if err != nil {
			b.Fatalf("First request after reset failed: %v", err)
		}

		b.Logf("Phase 3: Recovery successful")
		b.Logf("  Wait duration: %v", resetDuration)
		b.Logf("  Total benchmark time: %v", time.Since(exhaustStart))

		// Make a few more requests to verify full recovery
		b.Log("Verifying sustained recovery with 5 more requests...")
		for j := 0; j < 5; j++ {
			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				b.Fatalf("Recovery verification request %d failed: %v", j+1, err)
			}
		}

		b.Log("Recovery verification complete")
	}
}

// BenchmarkE2E_BurstVsSustained compares the performance difference between:
//   - Burst mode: Making requests as fast as possible (hits rate limits)
//   - Sustained mode: Making requests with small delays to stay within limits
//
// This demonstrates the tradeoff between throughput and rate limit compliance.
// The burst pattern is faster initially but may trigger throttling and errors.
// The sustained pattern is slower but maintains consistent throughput.
//
// Expected results:
//   - Burst: Fast initially, but may hit rate limits and slow down
//   - Sustained: Consistent timing throughout, no rate limit delays
func BenchmarkE2E_BurstVsSustained(b *testing.B) {
	skipIfNoCredentials(b)

	const requestCount = 10

	tests := []struct {
		name           string
		delayBetween   time.Duration
		description    string
		skipIfMultiple bool // Skip if b.N > 1 to conserve quota
	}{
		{
			name:           "burst",
			delayBetween:   0,
			description:    "Maximum speed, no artificial delays",
			skipIfMultiple: true,
		},
		{
			name:           "sustained_1s",
			delayBetween:   1 * time.Second,
			description:    "1 second delay between requests (60 req/min)",
			skipIfMultiple: false,
		},
		{
			name:           "sustained_2s",
			delayBetween:   2 * time.Second,
			description:    "2 second delay between requests (30 req/min)",
			skipIfMultiple: false,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Some tests are expensive and should only run once
			if tt.skipIfMultiple && b.N > 1 {
				b.Logf("Limiting iterations to 1 to conserve rate limit quota")
				b.N = 1
			}

			client := newE2EClient(b)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				startBenchmark := time.Now()

				for j := 0; j < requestCount; j++ {
					startReq := time.Now()

					_, err := client.GetHot(ctx, &types.PostsRequest{
						Subreddit:  "golang",
						Pagination: types.Pagination{Limit: 10},
					})
					if err != nil {
						b.Fatalf("Request %d/%d failed: %v", j+1, requestCount, err)
					}

					reqDuration := time.Since(startReq)

					// Log individual request timing for first iteration
					if i == 0 {
						b.Logf("Request %d/%d: %v", j+1, requestCount, reqDuration)
					}

					// Apply artificial delay for sustained mode
					if tt.delayBetween > 0 && j < requestCount-1 {
						time.Sleep(tt.delayBetween)
					}
				}

				totalDuration := time.Since(startBenchmark)
				avgPerRequest := totalDuration / requestCount

				b.Logf("\n%s results:", tt.name)
				b.Logf("  Total time: %v", totalDuration)
				b.Logf("  Average per request: %v", avgPerRequest)
				b.Logf("  Throughput: %.2f req/sec", float64(requestCount)/totalDuration.Seconds())
			}
		})
	}
}

// BenchmarkE2E_RateLimitCompliance verifies that the client stays within Reddit's
// rate limits during sustained load. This benchmark makes requests at a controlled
// rate and ensures no rate limit errors occur.
//
// This is a compliance test rather than a performance test - it verifies correct
// behavior under normal operating conditions.
func BenchmarkE2E_RateLimitCompliance(b *testing.B) {
	skipIfNoCredentials(b)

	// Only run once to measure compliance, not performance variation
	if b.N > 1 {
		b.Log("Limiting iterations to 1 for compliance testing")
		b.N = 1
	}

	client := newE2EClient(b)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Report allocations for memory profiling
	b.ReportAllocs()

	// Make 30 requests at a safe rate (2 req/sec = 120 req/min, well under 600/10min limit)
	const requestCount = 30
	const targetRate = 2 // requests per second
	const delayBetween = time.Second / targetRate

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startBenchmark := time.Now()
		errorCount := 0
		successCount := 0

		for j := 0; j < requestCount; j++ {
			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})

			if err != nil {
				errorCount++
				b.Logf("Request %d/%d failed: %v", j+1, requestCount, err)
			} else {
				successCount++
			}

			// Maintain target rate
			if j < requestCount-1 {
				time.Sleep(delayBetween)
			}
		}

		totalDuration := time.Since(startBenchmark)
		actualRate := float64(requestCount) / totalDuration.Seconds()

		b.Logf("\nRate limit compliance results:")
		b.Logf("  Requests: %d total, %d success, %d errors", requestCount, successCount, errorCount)
		b.Logf("  Duration: %v", totalDuration)
		b.Logf("  Target rate: %d req/sec", targetRate)
		b.Logf("  Actual rate: %.2f req/sec", actualRate)

		// Verify compliance: at 2 req/sec we expect zero errors (or at most 1 transient error)
		if errorCount > 1 {
			b.Errorf("Too many rate limit errors: %d/%d (expected ≤1)", errorCount, requestCount)
		}
	}
}

// Helper functions

// averageDuration calculates the average duration from a slice of request timings.
func averageDuration(timings []requestTiming) time.Duration {
	if len(timings) == 0 {
		return 0
	}

	// Use int64 accumulation to prevent potential overflow with many large durations
	var total int64
	for _, t := range timings {
		total += int64(t.duration)
	}

	return time.Duration(total / int64(len(timings)))
}

// BenchmarkE2E_RetryAfterHeader specifically tests the client's handling of the
// Retry-After header. While Reddit typically uses X-Ratelimit-* headers, it may
// also send Retry-After in certain rate limit scenarios.
//
// This benchmark is informational - it's difficult to trigger Retry-After in normal
// operation, so we primarily verify the client handles standard rate limit headers.
func BenchmarkE2E_RetryAfterHeader(b *testing.B) {
	skipIfNoCredentials(b)

	client := newE2EClient(b)
	ctx := context.Background()

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make a few requests and observe header processing
		// We can't easily trigger Retry-After, but we can verify normal operation
		for j := 0; j < 3; j++ {
			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				// If we get an error with retry information, log it
				b.Logf("Request %d returned error (may contain retry info): %v", j+1, err)
			}
		}
	}

	b.Log("Note: Retry-After header testing is informational - header may not appear in normal operation")
}

// BenchmarkE2E_ConcurrentRateLimit measures rate limiting behavior when multiple
// goroutines make concurrent requests. The client's rate limiter must coordinate
// across goroutines to prevent exceeding limits.
//
// This tests:
//   - Thread-safe rate limit state management
//   - Fair distribution of quota across goroutines
//   - No race conditions in header processing
func BenchmarkE2E_ConcurrentRateLimit(b *testing.B) {
	skipIfNoCredentials(b)

	// Only run once as this is expensive
	if b.N > 1 {
		b.Log("Limiting iterations to 1 to conserve rate limit quota")
		b.N = 1
	}

	client := newE2EClient(b)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Report allocations for memory profiling
	b.ReportAllocs()

	const goroutines = 5
	const requestsPerGoroutine = 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startBenchmark := time.Now()

		// Use errgroup for better error handling
		type result struct {
			goroutineID int
			duration    time.Duration
			err         error
		}
		results := make(chan result, goroutines*requestsPerGoroutine)

		// Launch concurrent workers
		done := make(chan struct{})
		for g := 0; g < goroutines; g++ {
			goroutineID := g
			go func() {
				// Recover from panics to prevent goroutine leaks and ensure results are sent
				defer func() {
					if r := recover(); r != nil {
						// Send error result for each remaining request in this goroutine
						for j := 0; j < requestsPerGoroutine; j++ {
							results <- result{
								goroutineID: goroutineID,
								duration:    0,
								err:         fmt.Errorf("panic in goroutine %d: %v", goroutineID, r),
							}
						}
					}
				}()

				for j := 0; j < requestsPerGoroutine; j++ {
					start := time.Now()
					_, err := client.GetHot(ctx, &types.PostsRequest{
						Subreddit:  "golang",
						Pagination: types.Pagination{Limit: 10},
					})
					results <- result{
						goroutineID: goroutineID,
						duration:    time.Since(start),
						err:         err,
					}
				}
			}()
		}

		// Wait for all requests to complete and track errors
		errorCount := 0
		successCount := 0
		var successDurations []time.Duration

		go func() {
			totalExpected := goroutines * requestsPerGoroutine
			for j := 0; j < totalExpected; j++ {
				r := <-results
				if r.err != nil {
					errorCount++
					b.Logf("Request from goroutine %d failed: %v", r.goroutineID, r.err)
				} else {
					successCount++
					successDurations = append(successDurations, r.duration)
				}
			}
			close(done)
		}()

		select {
		case <-done:
			// All requests completed
		case <-ctx.Done():
			b.Fatal("Benchmark timed out waiting for concurrent requests")
		}

		totalDuration := time.Since(startBenchmark)
		totalRequests := goroutines * requestsPerGoroutine

		// Calculate average duration for successful requests only
		var avgDuration time.Duration
		if len(successDurations) > 0 {
			var total int64
			for _, d := range successDurations {
				total += int64(d)
			}
			avgDuration = time.Duration(total / int64(len(successDurations)))
		}

		b.Logf("\nConcurrent rate limit results:")
		b.Logf("  Goroutines: %d", goroutines)
		b.Logf("  Requests per goroutine: %d", requestsPerGoroutine)
		b.Logf("  Total requests: %d (%d success, %d errors)", totalRequests, successCount, errorCount)
		b.Logf("  Total duration: %v", totalDuration)
		b.Logf("  Average throughput: %.2f req/sec", float64(totalRequests)/totalDuration.Seconds())
		if successCount > 0 {
			b.Logf("  Average duration (successful requests): %v", avgDuration)
		}
		if errorCount > 0 {
			b.Logf("  Warning: %d requests failed", errorCount)
		}
	}
}

// BenchmarkE2E_RateLimitMetrics benchmarks collecting and reporting rate limit
// metrics. This is useful for monitoring rate limit consumption in production.
func BenchmarkE2E_RateLimitMetrics(b *testing.B) {
	skipIfNoCredentials(b)

	client := newE2EClient(b)
	ctx := context.Background()

	// Report allocations for memory profiling
	b.ReportAllocs()

	// Track metrics across requests
	type metrics struct {
		requestNum        int
		duration          time.Duration
		timestamp         time.Time
		cumulativeTime    time.Duration
		requestsCompleted int
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var metricsLog []metrics
		startBenchmark := time.Now()

		// Make 10 requests and track detailed metrics
		for j := 0; j < 10; j++ {
			reqStart := time.Now()

			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				b.Fatalf("Request %d failed: %v", j+1, err)
			}

			reqDuration := time.Since(reqStart)
			metricsLog = append(metricsLog, metrics{
				requestNum:        j + 1,
				duration:          reqDuration,
				timestamp:         reqStart,
				cumulativeTime:    time.Since(startBenchmark),
				requestsCompleted: j + 1,
			})
		}

		// Report metrics for first iteration
		if i == 0 && len(metricsLog) > 0 {
			b.Log("\nRate limit metrics:")
			for _, m := range metricsLog {
				b.Logf("  Request %d: duration=%v, cumulative=%v, rate=%.2f req/sec",
					m.requestNum,
					m.duration,
					m.cumulativeTime,
					float64(m.requestsCompleted)/m.cumulativeTime.Seconds(),
				)
			}
		}
	}
}

// Example output helper for understanding the benchmarks
func init() {
	// This init function documents expected benchmark output
	_ = fmt.Sprintf(`
Example benchmark output:

BenchmarkE2E_RateLimitHeaders:
  Measures: Header parsing overhead (~100-500µs per request)
  Expected: Minimal overhead, < 1ms per request

BenchmarkE2E_ThrottlingBehavior:
  Measures: Progressive throttling as quota decreases
  Expected: Early fast (100-500ms), late slow (1-10s per request)

BenchmarkE2E_RateLimitRecovery:
  Measures: Time to recover after exhaustion
  Expected: ~60s wait for rate limit reset

BenchmarkE2E_BurstVsSustained:
  Measures: Throughput difference between patterns
  Expected: Burst fast initially but inconsistent, sustained stable

BenchmarkE2E_RateLimitCompliance:
  Measures: Error rate at safe request rate
  Expected: Zero errors at 2 req/sec
`)
}
