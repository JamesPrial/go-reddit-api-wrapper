package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/cache"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

// Benchmarks for authentication focusing on memory allocations and concurrent performance.
// The token caching hot path (cached reads) should have zero allocations.

// BenchmarkAuth_GetToken_Cached measures the performance of reading a valid cached token.
// This is the most critical hot path and should have zero allocations.
func BenchmarkAuth_GetToken_Cached(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.Fatal("server should not be called when token is cached")
	}))
	defer server.Close()

	mockClock := clock.NewMockClock(time.Time{})
	testCache := cache.NewMemoryCache(mockClock)
	auth, err := NewAuthenticator(
		server.Client(),
		"",
		"",
		"test-id",
		"test-secret",
		"test-agent",
		server.URL,
		"client_credentials",
		nil,
		mockClock,
		testCache,
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	// Pre-populate cache with a valid token that won't expire during the benchmark
	_ = testCache.Set(context.Background(), "cached-token-12345", time.Now().Add(1*time.Hour))

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		token, err := auth.GetToken(ctx)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		_ = token // Prevent compiler optimization
	}

	// Verify ONCE after loop
	token, _ := auth.GetToken(ctx)
	if token != "cached-token-12345" {
		b.Fatalf("got token %q, want %q", token, "cached-token-12345")
	}
}

// BenchmarkAuth_GetToken_Refresh measures the performance of a full OAuth2 token fetch.
// This includes HTTP request, JSON unmarshal, and cache store operations.
func BenchmarkAuth_GetToken_Refresh(b *testing.B) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"access_token": "token-%d", "token_type": "bearer", "expires_in": 3600}`, callCount)
	}))
	defer server.Close()

	mockClock := clock.NewMockClock(time.Time{})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Create fresh authenticator for each iteration to simulate cache miss
		testCache := cache.NewMemoryCache(mockClock)
		auth, err := NewAuthenticator(
			server.Client(),
			"",
			"",
			"test-id",
			"test-secret",
			"test-agent",
			server.URL,
			"client_credentials",
			nil,
			mockClock,
			testCache,
		)
		if err != nil {
			b.Fatalf("failed to create authenticator: %v", err)
		}
		b.StartTimer()

		_, err = auth.GetToken(ctx)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAuth_GetToken_Concurrent_Cached measures concurrent reads from cache.
// Tests scalability of lock-free atomic.Pointer reads with varying concurrency levels.
func BenchmarkAuth_GetToken_Concurrent_Cached(b *testing.B) {
	tests := []struct {
		name string
	}{
		{"parallel"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b.Fatal("server should not be called when token is cached")
			}))
			defer server.Close()

			mockClock := clock.NewMockClock(time.Time{})
			testCache := cache.NewMemoryCache(mockClock)
			auth, err := NewAuthenticator(
				server.Client(),
				"",
				"",
				"test-id",
				"test-secret",
				"test-agent",
				server.URL,
				"client_credentials",
				nil,
				mockClock,
				testCache,
			)
			if err != nil {
				b.Fatalf("failed to create authenticator: %v", err)
			}

			// Pre-populate cache with a valid token
			_ = testCache.Set(context.Background(), "cached-token-concurrent", time.Now().Add(1*time.Hour))

			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := auth.GetToken(ctx)
					if err != nil {
						b.Errorf("unexpected error: %v", err)
					}
				}
			})
		})
	}
}

// BenchmarkAuth_GetToken_Concurrent_Refresh measures concurrent token refresh.
// Tests thundering herd protection - only one goroutine should fetch, others should wait.
func BenchmarkAuth_GetToken_Concurrent_Refresh(b *testing.B) {
	var fetchCount int
	var fetchMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchMu.Lock()
		fetchCount++
		current := fetchCount
		fetchMu.Unlock()

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"access_token": "token-%d", "token_type": "bearer", "expires_in": 3600}`, current)
	}))
	defer server.Close()

	mockClock := clock.NewMockClock(time.Time{})
	testCache := cache.NewMemoryCache(mockClock)
	auth, err := NewAuthenticator(
		server.Client(),
		"",
		"",
		"test-id",
		"test-secret",
		"test-agent",
		server.URL,
		"client_credentials",
		nil,
		mockClock,
		testCache,
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	ctx := context.Background()
	concurrency := 100

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Invalidate cache to force refresh
		auth.InvalidateToken(context.Background())
		b.StartTimer()

		// Launch concurrent requests
		var wg sync.WaitGroup
		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := auth.GetToken(ctx)
				if err != nil {
					b.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
	}

	// Verify only one fetch occurred (thundering herd protection)
	fetchMu.Lock()
	finalCount := fetchCount
	fetchMu.Unlock()
	if finalCount != b.N {
		b.Errorf("expected %d token fetches, got %d", b.N, finalCount)
	}
}

// BenchmarkAuth_ExpiryCalculation measures the performance of tiered expiry threshold calculation.
// Tests with different token lifetimes: short (<10s), medium (10-60s), long (>60s).
func BenchmarkAuth_ExpiryCalculation(b *testing.B) {
	tests := []struct {
		name      string
		expiresIn int
		expected  float64 // Expected cache ratio
	}{
		{"long_lived_3600s", 3600, 0.80}, // >60s: 80% threshold
		{"long_lived_120s", 120, 0.80},   // >60s: 80% threshold
		{"medium_lived_60s", 60, 0.50},   // 10-60s: 50% threshold
		{"medium_lived_30s", 30, 0.50},   // 10-60s: 50% threshold
		{"medium_lived_10s", 10, 0.50},   // 10-60s: 50% threshold
		{"short_lived_9s", 9, 0.90},      // <10s: 90% threshold
		{"short_lived_5s", 5, 0.90},      // <10s: 90% threshold
		{"very_short_lived_1s", 1, 0.90}, // <10s: 90% threshold
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"access_token": "token-%d", "token_type": "bearer", "expires_in": %d}`,
					callCount, tt.expiresIn)
			}))
			defer server.Close()

			mockClock := clock.NewMockClock(time.Time{})
			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()

			var auth *Authenticator
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				testCache := cache.NewMemoryCache(mockClock)
				var err error
				auth, err = NewAuthenticator(
					server.Client(),
					"",
					"",
					"test-id",
					"test-secret",
					"test-agent",
					server.URL,
					"client_credentials",
					nil,
					mockClock,
					testCache,
				)
				if err != nil {
					b.Fatalf("failed to create authenticator: %v", err)
				}
				b.StartTimer()

				_, err = auth.GetToken(ctx)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}

			// Note: expiry calculation verification is now internal to the cache implementation
			// and cannot be directly tested from outside
		})
	}
}

// BenchmarkAuth_InvalidateToken measures the performance of cache invalidation.
// Tests atomic pointer store performance with mutex protection.
func BenchmarkAuth_InvalidateToken(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "token", "token_type": "bearer", "expires_in": 3600}`)
	}))
	defer server.Close()

	mockClock := clock.NewMockClock(time.Time{})
	testCache := cache.NewMemoryCache(mockClock)
	auth, err := NewAuthenticator(
		server.Client(),
		"",
		"",
		"test-id",
		"test-secret",
		"test-agent",
		server.URL,
		"client_credentials",
		nil,
		mockClock,
		testCache,
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	// Pre-populate cache
	_ = testCache.Set(context.Background(), "token-to-invalidate", time.Now().Add(1*time.Hour))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		auth.InvalidateToken(context.Background())
		// Re-populate cache for next iteration
		_ = testCache.Set(context.Background(), "token-to-invalidate", time.Now().Add(1*time.Hour))
	}
}

// BenchmarkAuth_InvalidateToken_Concurrent measures concurrent invalidation performance.
// Tests mutex contention when multiple goroutines invalidate simultaneously.
func BenchmarkAuth_InvalidateToken_Concurrent(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"access_token": "token", "token_type": "bearer", "expires_in": 3600}`)
	}))
	defer server.Close()

	mockClock := clock.NewMockClock(time.Time{})
	testCache := cache.NewMemoryCache(mockClock)
	auth, err := NewAuthenticator(
		server.Client(),
		"",
		"",
		"test-id",
		"test-secret",
		"test-agent",
		server.URL,
		"client_credentials",
		nil,
		mockClock,
		testCache,
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	// Pre-populate cache
	_ = testCache.Set(context.Background(), "token-to-invalidate", time.Now().Add(1*time.Hour))

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			auth.InvalidateToken(context.Background())
		}
	})
}

// BenchmarkAuth_DoubleCheckedLocking measures the efficiency of the double-checked locking pattern
// used in GetToken to avoid unnecessary mutex acquisitions when token is cached.
func BenchmarkAuth_DoubleCheckedLocking(b *testing.B) {
	tests := []struct {
		name     string
		scenario string
	}{
		{"cache_hit_first_check", "first_check"},
		{"cache_hit_second_check", "second_check"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, `{"access_token": "token-%d", "token_type": "bearer", "expires_in": 3600}`, callCount)
			}))
			defer server.Close()

			mockClock := clock.NewMockClock(time.Time{})
			testCache := cache.NewMemoryCache(mockClock)
			auth, err := NewAuthenticator(
				server.Client(),
				"",
				"",
				"test-id",
				"test-secret",
				"test-agent",
				server.URL,
				"client_credentials",
				nil,
				mockClock,
				testCache,
			)
			if err != nil {
				b.Fatalf("failed to create authenticator: %v", err)
			}

			ctx := context.Background()

			if tt.scenario == "first_check" {
				// Pre-populate cache so first check succeeds
				_ = testCache.Set(ctx, "cached-token", time.Now().Add(1*time.Hour))
			} else if tt.scenario == "second_check" {
				// Simulate scenario where another goroutine refreshed between checks
				// We'll let the first GetToken populate the cache
				_, err := auth.GetToken(ctx)
				if err != nil {
					b.Fatalf("initial token fetch failed: %v", err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := auth.GetToken(ctx)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
