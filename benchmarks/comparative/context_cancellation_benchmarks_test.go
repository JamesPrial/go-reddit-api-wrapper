package comparative

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"golang.org/x/time/rate"
)

// =============================================================================
// COMPARISON 6: Context Cancellation Benchmarks
// =============================================================================

// BenchmarkComparison_ContextCancellation_Immediate measures the overhead of
// context checking when context is cancelled before the first request.
//
// This benchmark demonstrates:
// - Overhead of context.Err() checks at the start of operations
// - How quickly cancellation is detected before any network I/O
// - Baseline cost of context propagation through the call stack
//
// Expected: Both our client and raw HTTP should complete very quickly since
// no network I/O happens. Our client may have slightly more overhead due to
// additional layers (auth, validation, rate limiting) that check context.
// Should complete in nanoseconds, not microseconds.
func BenchmarkComparison_ContextCancellation_Immediate(b *testing.B) {
	b.Run("our_client/with_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		server := setupMockServer(fixture)
		defer server.Close()

		redditClient := createTestRedditClient(b, server.URL)

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := redditClient.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 100,
				},
			})
			// Expect context.Canceled error
			if err == nil {
				b.Fatal("expected error from cancelled context, got nil")
			}
			if !errors.Is(err, context.Canceled) {
				b.Fatalf("expected context.Canceled, got: %v", err)
			}
		}
	})

	b.Run("our_client/without_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		b.SetBytes(int64(len(fixture)))
		server := setupMockServer(fixture)
		defer server.Close()

		redditClient := createTestRedditClient(b, server.URL)
		ctx := context.Background()

		// Warmup to cache auth token
		_, err := redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 100,
			},
		})
		if err != nil {
			b.Fatalf("warmup GetHot failed: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := redditClient.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 100,
				},
			})
			if err != nil {
				b.Fatalf("GetHot failed: %v", err)
			}
		}
	})

	b.Run("raw_http/with_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		server := setupMockServer(fixture)
		defer server.Close()

		rawClient := &rawHTTPClient{
			client:  &http.Client{Timeout: 30 * time.Second},
			baseURL: server.URL,
		}

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/r/golang/hot.json", nil)
			if err != nil {
				b.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("User-Agent", "test/1.0")
			req.Header.Set("Authorization", "Bearer test-token")

			_, err = rawClient.client.Do(req)
			// Expect context.Canceled error
			if err == nil {
				b.Fatal("expected error from cancelled context, got nil")
			}
			if !errors.Is(err, context.Canceled) {
				b.Fatalf("expected context.Canceled, got: %v", err)
			}
		}
	})

	b.Run("raw_http/without_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		b.SetBytes(int64(len(fixture)))
		server := setupMockServer(fixture)
		defer server.Close()

		rawClient := &rawHTTPClient{
			client:  &http.Client{Timeout: 30 * time.Second},
			baseURL: server.URL,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := rawClient.GetPosts("golang")
			if err != nil {
				b.Fatalf("GetPosts failed: %v", err)
			}
			_ = result
		}
	})
}

// BenchmarkComparison_ContextCancellation_InFlight measures cancellation
// detection when context is cancelled while a request is in-flight.
//
// This benchmark demonstrates:
// - How quickly cancellation propagates to active HTTP requests
// - Network I/O interruption behavior
// - Resource cleanup when requests are aborted mid-stream
//
// Expected: Cancellation should be detected within milliseconds of the cancel()
// call, not waiting for full response. Our client and raw HTTP should have similar
// cancellation latency since both rely on http.Request context propagation.
func BenchmarkComparison_ContextCancellation_InFlight(b *testing.B) {
	b.Run("our_client/with_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		// Create slow server that delays response
		server := createSlowServer(fixture, 500*time.Millisecond)
		defer server.Close()

		redditClient := createTestRedditClient(b, server.URL)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create context that will be cancelled mid-request
			ctx, cancel := context.WithCancel(context.Background())

			// Start request in goroutine
			errCh := make(chan error, 1)
			go func() {
				_, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				errCh <- err
			}()

			// Cancel after a short delay (simulating in-flight cancellation)
			time.Sleep(50 * time.Millisecond)
			cancel()

			// Wait for error with timeout protection
			select {
			case err := <-errCh:
				if err == nil {
					b.Fatal("expected error from cancelled context, got nil")
				}
				if !errors.Is(err, context.Canceled) {
					b.Fatalf("expected context.Canceled, got: %v", err)
				}
			case <-time.After(5 * time.Second):
				b.Fatal("timeout waiting for context cancellation to be detected")
			}
		}
	})

	b.Run("our_client/without_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		b.SetBytes(int64(len(fixture)))
		// Use normal (fast) server for baseline
		server := setupMockServer(fixture)
		defer server.Close()

		redditClient := createTestRedditClient(b, server.URL)
		ctx := context.Background()

		// Warmup to cache auth token
		_, err := redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 100,
			},
		})
		if err != nil {
			b.Fatalf("warmup GetHot failed: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := redditClient.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 100,
				},
			})
			if err != nil {
				b.Fatalf("GetHot failed: %v", err)
			}
		}
	})

	b.Run("raw_http/with_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		// Create slow server that delays response
		server := createSlowServer(fixture, 500*time.Millisecond)
		defer server.Close()

		rawClient := &rawHTTPClient{
			client:  &http.Client{Timeout: 30 * time.Second},
			baseURL: server.URL,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create context that will be cancelled mid-request
			ctx, cancel := context.WithCancel(context.Background())

			// Start request in goroutine
			errCh := make(chan error, 1)
			go func() {
				req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/r/golang/hot.json", nil)
				if err != nil {
					errCh <- err
					return
				}
				req.Header.Set("User-Agent", "test/1.0")
				req.Header.Set("Authorization", "Bearer test-token")

				resp, err := rawClient.client.Do(req)
				if err == nil {
					resp.Body.Close()
				}
				errCh <- err
			}()

			// Cancel after a short delay (simulating in-flight cancellation)
			time.Sleep(50 * time.Millisecond)
			cancel()

			// Wait for error with timeout protection
			select {
			case err := <-errCh:
				if err == nil {
					b.Fatal("expected error from cancelled context, got nil")
				}
				if !errors.Is(err, context.Canceled) {
					b.Fatalf("expected context.Canceled, got: %v", err)
				}
			case <-time.After(5 * time.Second):
				b.Fatal("timeout waiting for context cancellation to be detected")
			}
		}
	})

	b.Run("raw_http/without_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		b.SetBytes(int64(len(fixture)))
		server := setupMockServer(fixture)
		defer server.Close()

		rawClient := &rawHTTPClient{
			client:  &http.Client{Timeout: 30 * time.Second},
			baseURL: server.URL,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := rawClient.GetPosts("golang")
			if err != nil {
				b.Fatalf("GetPosts failed: %v", err)
			}
			_ = result
		}
	})
}

// BenchmarkComparison_ContextCancellation_RateLimit measures cancellation
// detection while waiting in the rate limiter queue.
//
// This benchmark demonstrates:
// - Rate limiter's responsiveness to context cancellation
// - Whether requests waiting for rate limit tokens can be aborted quickly
// - Cost of context checking in the rate limiting wait path
//
// Expected: Our client should detect cancellation immediately during the
// rate limiter Wait() call without making any HTTP request. This prevents
// wasting resources on requests that the caller has abandoned.
// Raw HTTP client doesn't have rate limiting, so its behavior differs.
func BenchmarkComparison_ContextCancellation_RateLimit(b *testing.B) {
	b.Run("our_client/with_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		server := setupMockServer(fixture)
		defer server.Close()

		// Create client with very low rate limit to force queueing
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		httpClient := &http.Client{Timeout: 30 * time.Second}

		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"access_token": "test-token-12345", "token_type": "bearer", "expires_in": 3600}`))
		}))
		defer authServer.Close()

		config := &graw.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			UserAgent:    "test-agent/1.0",
			BaseURL:      server.URL,
			AuthURL:      authServer.URL,
			HTTPClient:   httpClient,
			Logger:       logger,
			RateLimitConfig: &graw.RateLimitConfig{
				RequestsPerMinute:  1, // Very low rate to force queueing
				Burst:              1,
				ProactiveThreshold: 10,
			},
		}

		redditClient, err := graw.NewClient(config)
		if err != nil {
			b.Fatalf("failed to create Reddit client: %v", err)
		}

		// Warmup to cache auth token and consume initial rate limit tokens
		ctx := context.Background()
		_, err = redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 100,
			},
		})
		if err != nil {
			b.Fatalf("warmup GetHot failed: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create context that will be cancelled while waiting in rate limiter
			ctx, cancel := context.WithCancel(context.Background())

			// Start request in goroutine (will queue in rate limiter)
			errCh := make(chan error, 1)
			go func() {
				_, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				errCh <- err
			}()

			// Cancel immediately (should abort while waiting for rate limit token)
			cancel()

			// Wait for error with timeout protection
			select {
			case err := <-errCh:
				if err == nil {
					b.Fatal("expected error from cancelled context, got nil")
				}
				if !errors.Is(err, context.Canceled) {
					b.Fatalf("expected context.Canceled, got: %v", err)
				}
			case <-time.After(5 * time.Second):
				b.Fatal("timeout waiting for context cancellation to be detected")
			}

			// Small delay to allow rate limiter to reset for next iteration
			time.Sleep(100 * time.Millisecond)
		}
	})

	b.Run("our_client/without_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		b.SetBytes(int64(len(fixture)))
		server := setupMockServer(fixture)
		defer server.Close()

		redditClient := createTestRedditClient(b, server.URL)
		ctx := context.Background()

		// Warmup to cache auth token
		_, err := redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 100,
			},
		})
		if err != nil {
			b.Fatalf("warmup GetHot failed: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := redditClient.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 100,
				},
			})
			if err != nil {
				b.Fatalf("GetHot failed: %v", err)
			}
		}
	})

	b.Run("raw_http/with_rate_limit_and_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		server := setupMockServer(fixture)
		defer server.Close()

		// Create rate limited client to match our client's behavior
		limiter := rate.NewLimiter(rate.Limit(1.0/60.0), 1) // 1 request per minute
		rateLimitClient := &withRateLimitClient{
			client:  &http.Client{Timeout: 30 * time.Second},
			limiter: limiter,
		}

		// Consume initial token
		ctx := context.Background()
		req, err := http.NewRequest("GET", server.URL+"/r/golang/hot.json", nil)
		if err != nil {
			b.Fatalf("failed to create initial request: %v", err)
		}
		resp, err := rateLimitClient.Do(ctx, req)
		if err != nil {
			b.Fatalf("initial request failed: %v", err)
		}
		resp.Body.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create context that will be cancelled while waiting in rate limiter
			ctx, cancel := context.WithCancel(context.Background())

			// Start request in goroutine (will queue in rate limiter)
			errCh := make(chan error, 1)
			go func() {
				req, err := http.NewRequest("GET", server.URL+"/r/golang/hot.json", nil)
				if err != nil {
					errCh <- err
					return
				}
				req.Header.Set("User-Agent", "test/1.0")
				req.Header.Set("Authorization", "Bearer test-token")

				resp, err := rateLimitClient.Do(ctx, req)
				if err == nil {
					resp.Body.Close()
				}
				errCh <- err
			}()

			// Cancel immediately (should abort while waiting for rate limit token)
			cancel()

			// Wait for error with timeout protection
			select {
			case err := <-errCh:
				if err == nil {
					b.Fatal("expected error from cancelled context, got nil")
				}
				if !errors.Is(err, context.Canceled) {
					b.Fatalf("expected context.Canceled, got: %v", err)
				}
			case <-time.After(5 * time.Second):
				b.Fatal("timeout waiting for context cancellation to be detected")
			}

			// Small delay to allow rate limiter to reset for next iteration
			time.Sleep(100 * time.Millisecond)
		}
	})

	b.Run("raw_http/without_rate_limit_or_cancellation", func(b *testing.B) {
		b.ReportAllocs()

		fixture := loadFixture(b, "medium_posts.json")
		b.SetBytes(int64(len(fixture)))
		server := setupMockServer(fixture)
		defer server.Close()

		rawClient := &rawHTTPClient{
			client:  &http.Client{Timeout: 30 * time.Second},
			baseURL: server.URL,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			result, err := rawClient.GetPosts("golang")
			if err != nil {
				b.Fatalf("GetPosts failed: %v", err)
			}
			_ = result
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// createSlowServer creates an httptest server that introduces a delay before
// responding, simulating slow network conditions or long-running requests.
// This is used to test context cancellation while requests are in-flight.
func createSlowServer(fixture []byte, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for context cancellation during the delay
		select {
		case <-time.After(delay):
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Ratelimit-Remaining", "60")
			w.Header().Set("X-Ratelimit-Reset", "60")
			w.WriteHeader(http.StatusOK)
			w.Write(fixture)
		case <-r.Context().Done():
			// Context was cancelled, exit without writing response
			return
		}
	}))
}
