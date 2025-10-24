package e2e

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// E2E (End-to-End) benchmarks for Reddit API authentication flows.
//
// These benchmarks measure real-world authentication performance against Reddit's live OAuth2 API,
// testing scenarios from cold start authentication to token refresh and concurrent access patterns.
// They require valid Reddit OAuth2 credentials set in environment variables.
//
// Prerequisites:
//   - REDDIT_CLIENT_ID: OAuth2 client ID from Reddit app registration
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret from Reddit app registration
//
// Optional (for user authentication):
//   - REDDIT_USERNAME: Reddit username
//   - REDDIT_PASSWORD: Reddit password
//
// Run with:
//
//	go test -bench=BenchmarkE2E_Auth ./benchmarks/e2e -benchmem
//
// Note: These benchmarks make real API calls and respect Reddit's rate limits.
// They will be skipped if credentials are not available.

// BenchmarkE2E_TokenFetch_ColdStart measures OAuth2 token acquisition from scratch
// with no cached token. This tests the initial authentication flow performance including:
//   - HTTP request to Reddit's OAuth2 token endpoint
//   - Credential validation
//   - Token parsing and cache population
//   - Full authentication round-trip time
//
// This benchmark simulates the worst-case scenario: a completely fresh client that must
// authenticate before making any API calls. Each iteration creates a new client to ensure
// we're measuring true cold-start performance without any cached state.
//
// Typical use cases:
//   - Application startup
//   - First request after cache invalidation
//   - Lambda/serverless cold starts
func BenchmarkE2E_TokenFetch_ColdStart(b *testing.B) {
	skipIfNoCredentials(b)

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a fresh client for each iteration to simulate cold start
		// This ensures we measure the full authentication overhead without any cached tokens
		client := newE2EClient(b)

		// Make a minimal API request to trigger authentication
		// Using GetHot with limit 1 to minimize API overhead and focus on auth
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 1,
			},
		})
		cancel()

		if err != nil {
			b.Fatalf("Cold start authentication failed: %v", err)
		}
	}
}

// BenchmarkE2E_TokenRefresh simulates token expiry and refresh scenarios.
//
// NOTE: This benchmark has limitations with the current client implementation.
// The real Reddit client caches tokens internally, and we cannot directly control
// token expiry from the E2E level. This benchmark measures the time to create a new
// client and make a request, which implicitly tests the auth flow, but does not
// directly test token refresh logic.
//
// What this measures:
//   - Creating a new authenticated client
//   - Making an API request with fresh authentication
//
// What this does NOT measure:
//   - Actual token refresh when a cached token expires
//   - Token refresh retry logic on 401 responses
//
// For true token refresh benchmarks, see reddit/internal/auth/auth_bench_test.go
// which tests the internal auth.Authenticator with controlled token expiry.
func BenchmarkE2E_TokenRefresh(b *testing.B) {
	skipIfNoCredentials(b)

	b.Logf("Note: E2E token refresh is limited by lack of direct cache control. " +
		"See reddit/internal/auth/auth_bench_test.go for comprehensive token refresh benchmarks.")

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a client and make a request
		// This simulates the scenario where we need fresh authentication
		client := newE2EClient(b)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 1,
			},
		})
		cancel()

		if err != nil {
			b.Fatalf("Token refresh scenario failed: %v", err)
		}

		// Validate response
		if len(resp.Posts) == 0 {
			b.Error("Expected posts in response")
		}
	}
}

// BenchmarkE2E_ConcurrentAuth tests concurrent authentication requests in a thundering herd scenario.
// This measures how well the client handles multiple goroutines trying to authenticate simultaneously,
// which can occur in:
//   - Web servers handling concurrent requests at startup
//   - Worker pools starting simultaneously
//   - Burst traffic after idle periods
//
// The client should protect against thundering herd by ensuring only one goroutine fetches a token
// while others wait, rather than making N concurrent token requests.
//
// This benchmark uses b.RunParallel() to simulate realistic concurrent load and measures:
//   - Contention handling in the authentication layer
//   - Performance degradation under concurrent access
//   - Proper synchronization and mutex usage
func BenchmarkE2E_ConcurrentAuth(b *testing.B) {
	skipIfNoCredentials(b)

	// Create a single client that will be shared across all parallel goroutines
	// This tests the client's ability to handle concurrent authentication requests
	client := newE2EClient(b)

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()

	// RunParallel spawns multiple goroutines (typically GOMAXPROCS)
	// All goroutines will attempt to make API calls concurrently
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Create a new context with timeout for each iteration to prevent context leaks
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// Each goroutine makes an API request
			// The client's internal auth layer should handle concurrent token access efficiently
			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 1,
				},
			})
			if err != nil {
				b.Errorf("Concurrent request failed: %v", err)
			}

			// Cancel context at end of iteration
			cancel()
		}
	})
}

// BenchmarkE2E_ConcurrentAuth_ThunderingHerd specifically tests the thundering herd scenario
// where many goroutines simultaneously attempt to authenticate with an empty cache.
// This is more aggressive than BenchmarkE2E_ConcurrentAuth as it uses a fixed high concurrency
// level to stress-test the authentication layer.
//
// This simulates scenarios like:
//   - Cold start with immediate high load
//   - Cache invalidation followed by burst traffic
//   - Multiple workers starting simultaneously in a distributed system
//
// The benchmark measures whether the client properly coalesces concurrent token requests
// to avoid overwhelming Reddit's OAuth endpoint.
func BenchmarkE2E_ConcurrentAuth_ThunderingHerd(b *testing.B) {
	skipIfNoCredentials(b)

	// Use a fixed high concurrency level to stress test
	const concurrency = 50

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create a fresh client for each iteration to simulate thundering herd on cold cache
		client := newE2EClient(b)
		b.StartTimer()

		// Launch many concurrent goroutines all trying to authenticate at once
		var wg sync.WaitGroup
		errCh := make(chan error, concurrency)

		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				_, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit: 1,
					},
				})
				if err != nil {
					errCh <- err
				}
			}()
		}

		wg.Wait()
		close(errCh)

		// Check for any errors
		for err := range errCh {
			b.Errorf("Thundering herd request failed: %v", err)
		}
	}
}

// BenchmarkE2E_AuthenticatedRequest measures the complete performance of making an authenticated
// API request, including all authentication overhead. This benchmark provides two sub-benchmarks:
//
// 1. FirstRequest: Measures cold start + auth + API call (worst case)
// 2. SubsequentRequest: Measures cached token + API call (typical case)
//
// This helps understand the performance difference between:
//   - Initial requests that must authenticate from scratch
//   - Follow-up requests that use cached credentials
//
// Typical performance expectations:
//   - FirstRequest: ~500ms-2s depending on network latency to Reddit
//   - SubsequentRequest: ~100ms-500ms (just the API call, no auth overhead)
func BenchmarkE2E_AuthenticatedRequest(b *testing.B) {
	skipIfNoCredentials(b)

	b.Run("FirstRequest", func(b *testing.B) {
		// Report allocations for memory profiling
		b.ReportAllocs()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create fresh client for each iteration - measures auth + API call
			client := newE2EClient(b)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 10,
				},
			})
			cancel()

			if err != nil {
				b.Fatalf("First request failed: %v", err)
			}

			// Save first response as fixture for inspection
			if i == 0 {
				b.StopTimer()
				saveFixture(b, "auth_first_request", resp)
				b.StartTimer()
			}

			// Validate response
			if len(resp.Posts) == 0 {
				b.Error("Expected posts in response")
			}
		}
	})

	b.Run("SubsequentRequest", func(b *testing.B) {
		// Create client once, reuse for all iterations - measures just API call overhead
		client := newE2EClient(b)
		ctx := context.Background()

		// Warm up: make one request to populate token cache
		_, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})
		if err != nil {
			b.Fatalf("Warmup request failed: %v", err)
		}

		// Report allocations for memory profiling
		b.ReportAllocs()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 10,
				},
			})
			if err != nil {
				b.Fatalf("Subsequent request failed: %v", err)
			}

			// Save first response as fixture for inspection
			if i == 0 {
				b.StopTimer()
				saveFixture(b, "auth_subsequent_request", resp)
				b.StartTimer()
			}

			// Validate response
			if len(resp.Posts) == 0 {
				b.Error("Expected posts in response")
			}
		}
	})
}

// BenchmarkE2E_AuthOverhead_Comparison provides a direct comparison between authentication
// scenarios by testing the same API operation with different authentication states.
// This helps quantify the actual overhead of authentication in real-world usage.
//
// Sub-benchmarks:
//   - ColdClient: New client, no cached token (full auth overhead)
//   - WarmClient: Existing client, cached token (minimal auth overhead)
//
// The difference between these benchmarks represents the authentication cost that can
// be amortized across multiple requests by reusing a client instance.
func BenchmarkE2E_AuthOverhead_Comparison(b *testing.B) {
	skipIfNoCredentials(b)

	b.Run("ColdClient", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			client := newE2EClient(b)

			// Create context with timeout for each iteration to prevent context leaks
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 1},
			})
			if err != nil {
				b.Fatalf("Cold client request failed: %v", err)
			}

			// Cancel context at end of iteration
			cancel()
		}
	})

	b.Run("WarmClient", func(b *testing.B) {
		// Create client once and reuse
		client := newE2EClient(b)

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			// Create context with timeout for each iteration to prevent context leaks
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 1},
			})
			if err != nil {
				b.Fatalf("Warm client request failed: %v", err)
			}

			// Cancel context at end of iteration
			cancel()
		}
	})
}

// BenchmarkE2E_MultipleClients_ConcurrentAuth tests the scenario where multiple independent
// clients authenticate concurrently. This simulates multi-tenant systems or microservices
// where different components might create their own Reddit clients simultaneously.
//
// Unlike BenchmarkE2E_ConcurrentAuth which tests concurrent access to a single client,
// this benchmark creates multiple clients in parallel, each with independent authentication state.
//
// This measures:
//   - Independent authentication flows without shared state
//   - Memory usage when multiple clients exist simultaneously
//   - Potential rate limit impacts from multiple concurrent auth requests
func BenchmarkE2E_MultipleClients_ConcurrentAuth(b *testing.B) {
	skipIfNoCredentials(b)

	const numClients = 10

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		errCh := make(chan error, numClients)

		// Create multiple clients concurrently
		for j := 0; j < numClients; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				client := newE2EClient(b)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				_, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: 1},
				})
				if err != nil {
					errCh <- err
				}
			}()
		}

		wg.Wait()
		close(errCh)

		// Check for errors
		for err := range errCh {
			b.Errorf("Multi-client request failed: %v", err)
		}
	}
}
