package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

// Benchmarks for authentication focusing on memory allocations and concurrent performance.

// BenchmarkAuth_GetToken_Refresh measures the performance of a full OAuth2 token fetch.
// This includes HTTP request and JSON unmarshal operations.
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
		// Create fresh authenticator for each iteration
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
		)
		if err != nil {
			b.Fatalf("failed to create authenticator: %v", err)
		}
		b.StartTimer()

		_, _, err = auth.GetToken(ctx)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAuth_GetToken_Concurrent measures concurrent token fetches.
// Tests scalability of concurrent HTTP requests.
func BenchmarkAuth_GetToken_Concurrent(b *testing.B) {
	callCount := 0
	var callMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMu.Lock()
		callCount++
		current := callCount
		callMu.Unlock()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"access_token": "token-%d", "token_type": "bearer", "expires_in": 3600}`, current)
	}))
	defer server.Close()

	mockClock := clock.NewMockClock(time.Time{})
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
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := auth.GetToken(ctx)
			if err != nil {
				b.Errorf("unexpected error: %v", err)
			}
		}
	})
}

// BenchmarkAuth_ExpiryCalculation measures the performance of token expiry calculation.
// Tests with different token lifetimes: short (<10s), medium (10-60s), long (>60s).
func BenchmarkAuth_ExpiryCalculation(b *testing.B) {
	tests := []struct {
		name      string
		expiresIn int
	}{
		{"long_lived_3600s", 3600},
		{"long_lived_120s", 120},
		{"medium_lived_60s", 60},
		{"medium_lived_30s", 30},
		{"medium_lived_10s", 10},
		{"short_lived_9s", 9},
		{"short_lived_5s", 5},
		{"very_short_lived_1s", 1},
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
				)
				if err != nil {
					b.Fatalf("failed to create authenticator: %v", err)
				}
				b.StartTimer()

				_, _, err = auth.GetToken(ctx)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
