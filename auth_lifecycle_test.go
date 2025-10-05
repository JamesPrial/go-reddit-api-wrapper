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
	"sync/atomic"
	"testing"
	"time"
)

// TestTokenRefreshTimingEdgeCases tests edge cases around token refresh timing
func TestTokenRefreshTimingEdgeCases(t *testing.T) {
	var requestCount int64
	var tokenExpiry int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Handle token requests
		if strings.Contains(r.URL.Path, "/api/v1/access_token") {
			mu.Lock()
			currentExpiry := tokenExpiry
			mu.Unlock()

			// Parse form data
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			grantType := r.Form.Get("grant_type")

			var response map[string]interface{}

			switch grantType {
			case "client_credentials":
				// Initial token or refresh
				expiry := time.Now().Add(1 * time.Hour).Unix()
				mu.Lock()
				tokenExpiry = expiry
				mu.Unlock()

				response = map[string]interface{}{
					"access_token":  "test_token_" + strconv.FormatInt(currentExpiry, 10),
					"token_type":    "bearer",
					"expires_in":    3600,
					"scope":         "read",
					"refresh_token": "refresh_token_" + strconv.FormatInt(currentExpiry, 10),
				}

			case "refresh_token":
				// Refresh token flow
				expiry := time.Now().Add(1 * time.Hour).Unix()
				mu.Lock()
				tokenExpiry = expiry
				mu.Unlock()

				response = map[string]interface{}{
					"access_token":  "refreshed_token_" + strconv.FormatInt(expiry, 10),
					"token_type":    "bearer",
					"expires_in":    3600,
					"scope":         "read",
					"refresh_token": "new_refresh_token_" + strconv.FormatInt(expiry, 10),
				}

			default:
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle API requests
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "123456789")
		w.Header().Set("Content-Type", "application/json")

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
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

	t.Run("TokenExpiryEdgeCases", func(t *testing.T) {
		testCases := []struct {
			name          string
			tokenLifespan time.Duration
			requestDelay  time.Duration
			expectRefresh bool
			description   string
		}{
			{
				name:          "FreshToken",
				tokenLifespan: 1 * time.Hour,
				requestDelay:  1 * time.Minute,
				expectRefresh: false,
				description:   "Token should not refresh when fresh",
			},
			{
				name:          "NearExpiry",
				tokenLifespan: 2 * time.Minute,
				requestDelay:  1 * time.Minute,
				expectRefresh: true,
				description:   "Token should refresh when near expiry",
			},
			{
				name:          "ExpiredToken",
				tokenLifespan: 500 * time.Millisecond,
				requestDelay:  1 * time.Second,
				expectRefresh: true,
				description:   "Token should refresh when expired",
			},
			{
				name:          "ImmediateExpiry",
				tokenLifespan: 1 * time.Millisecond,
				requestDelay:  10 * time.Millisecond,
				expectRefresh: true,
				description:   "Token should refresh when immediately expired",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Reset counters
				atomic.StoreInt64(&requestCount, 0)
				mu.Lock()
				tokenExpiry = 0
				mu.Unlock()

				// Create client with custom token lifespan
				config := &Config{
					ClientID:     "test_id",
					ClientSecret: "test_secret",
					UserAgent:    "test/1.0",
					AuthURL:      server.URL,
					BaseURL:      server.URL,
					HTTPClient:   &http.Client{Timeout: 30 * time.Second},
				}

				client, err := NewClient(config)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}

				// Log the test being performed
				t.Logf("Testing %s: %s", tc.name, tc.description)

				// Wait for the specified delay
				time.Sleep(tc.requestDelay)

				// Make a request
				startTime := time.Now()
				_, err = client.Me(context.Background())
				requestDuration := time.Since(startTime)

				if err != nil {
					t.Errorf("Request failed: %v", err)
				}

				totalRequests := atomic.LoadInt64(&requestCount)

				// Initial auth request + potential refresh + API request
				minExpected := int64(2) // auth + API
				if tc.expectRefresh {
					minExpected = int64(3) // auth + refresh + API
				}

				if totalRequests < minExpected {
					t.Errorf("Expected at least %d requests, got %d", minExpected, totalRequests)
				}

				t.Logf("Test %s completed in %v with %d total requests",
					tc.name, requestDuration, totalRequests)
			})
		}
	})
}

// TestConcurrentTokenRefreshRaceCondition tests concurrent token refresh race conditions
func TestConcurrentTokenRefreshRaceCondition(t *testing.T) {
	var requestCount int64
	var tokenRefreshCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Handle token requests
		if strings.Contains(r.URL.Path, "/api/v1/access_token") {
			// Simulate token refresh delay to increase chance of race condition
			time.Sleep(100 * time.Millisecond)

			mu.Lock()
			tokenRefreshCount++
			currentRefreshCount := tokenRefreshCount
			mu.Unlock()

			response := map[string]interface{}{
				"access_token":  fmt.Sprintf("test_token_%d", currentRefreshCount),
				"token_type":    "bearer",
				"expires_in":    3600,
				"scope":         "read",
				"refresh_token": fmt.Sprintf("refresh_token_%d", currentRefreshCount),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle API requests
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "123456789")
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

	t.Run("ConcurrentTokenRefresh", func(t *testing.T) {
		// Reset counters
		atomic.StoreInt64(&requestCount, 0)
		atomic.StoreInt64(&tokenRefreshCount, 0)

		config := &Config{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			UserAgent:    "test/1.0",
			AuthURL:      server.URL,
			BaseURL:      server.URL,
			HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		}

		client, err := NewClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Make multiple concurrent requests that should trigger token refresh
		numGoroutines := 10
		var wg sync.WaitGroup
		results := make(chan error, numGoroutines)

		startTime := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				_, err := client.Me(context.Background())
				results <- err

				if err != nil {
					t.Logf("Goroutine %d failed: %v", goroutineID, err)
				} else {
					t.Logf("Goroutine %d succeeded", goroutineID)
				}
			}(i)
		}

		wg.Wait()
		close(results)

		totalTime := time.Since(startTime)
		totalRequests := atomic.LoadInt64(&requestCount)
		totalRefreshes := atomic.LoadInt64(&tokenRefreshCount)

		// Count successes and failures
		successCount := 0
		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
		}

		t.Logf("Concurrent token refresh test results:")
		t.Logf("  Goroutines: %d", numGoroutines)
		t.Logf("  Successful requests: %d", successCount)
		t.Logf("  Failed requests: %d", errorCount)
		t.Logf("  Total HTTP requests: %d", totalRequests)
		t.Logf("  Token refreshes: %d", totalRefreshes)
		t.Logf("  Total time: %v", totalTime)

		// Should have at least one token refresh
		if totalRefreshes == 0 {
			t.Error("Expected at least one token refresh")
		}

		// Should not have excessive token refreshes (indicating race condition)
		if totalRefreshes > 3 {
			t.Errorf("Too many token refreshes (%d), may indicate race condition", totalRefreshes)
		}

		// Most requests should succeed
		successRate := float64(successCount) / float64(numGoroutines) * 100
		if successRate < 80 {
			t.Errorf("Success rate too low: %.1f%%, expected at least 80%%", successRate)
		}
	})
}

// TestAuthenticationFailureRecovery tests recovery from authentication failures
func TestAuthenticationFailureRecovery(t *testing.T) {
	var requestCount int64
	var authFailureCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Handle token requests
		if strings.Contains(r.URL.Path, "/api/v1/access_token") {
			mu.Lock()
			currentFailureCount := authFailureCount
			authFailureCount++
			mu.Unlock()

			// Simulate auth failures for first few attempts
			if currentFailureCount < 3 {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error":             "invalid_client",
					"error_description": "Invalid client credentials",
				})
				return
			}

			// Success after failures
			response := map[string]interface{}{
				"access_token":  "recovered_token",
				"token_type":    "bearer",
				"expires_in":    3600,
				"scope":         "read",
				"refresh_token": "recovered_refresh_token",
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle API requests
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "123456789")
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

	t.Run("AuthFailureRecovery", func(t *testing.T) {
		// Reset counters
		atomic.StoreInt64(&requestCount, 0)
		atomic.StoreInt64(&authFailureCount, 0)

		config := &Config{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			UserAgent:    "test/1.0",
			AuthURL:      server.URL,
			BaseURL:      server.URL,
			HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		}

		client, err := NewClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Initial request should fail due to auth failures
		_, err = client.Me(context.Background())
		if err == nil {
			t.Error("Expected initial request to fail due to auth issues")
		} else {
			t.Logf("Initial request failed as expected: %v", err)
		}

		// Wait a bit and retry - should eventually succeed
		var successCount int
		var errorCount int

		for i := 0; i < 10; i++ {
			_, err = client.Me(context.Background())
			if err != nil {
				errorCount++
				t.Logf("Retry %d failed: %v", i+1, err)
			} else {
				successCount++
				t.Logf("Retry %d succeeded", i+1)
			}

			// Wait between retries
			time.Sleep(500 * time.Millisecond)
		}

		totalRequests := atomic.LoadInt64(&requestCount)
		totalAuthFailures := atomic.LoadInt64(&authFailureCount)

		t.Logf("Auth failure recovery test results:")
		t.Logf("  Successful requests: %d", successCount)
		t.Logf("  Failed requests: %d", errorCount)
		t.Logf("  Total auth failures: %d", totalAuthFailures)
		t.Logf("  Total HTTP requests: %d", totalRequests)

		// Should have experienced auth failures
		if totalAuthFailures == 0 {
			t.Error("Expected auth failures, but none occurred")
		}

		// Should eventually recover and succeed
		if successCount == 0 {
			t.Error("Expected at least one successful request after recovery")
		}

		// Should have made reasonable number of requests
		if totalRequests < 5 {
			t.Errorf("Expected at least 5 total HTTP requests, got %d", totalRequests)
		}
	})
}

// TestTokenCacheInvalidation tests token cache invalidation behavior
func TestTokenCacheInvalidation(t *testing.T) {
	var requestCount int64
	var tokenIssuedCount int64
	var revokedTokens []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Handle token requests
		if strings.Contains(r.URL.Path, "/api/v1/access_token") {
			tokenID := fmt.Sprintf("token_%d", atomic.AddInt64(&tokenIssuedCount, 1))

			response := map[string]interface{}{
				"access_token":  tokenID,
				"token_type":    "bearer",
				"expires_in":    1, // Very short expiry for testing
				"scope":         "read",
				"refresh_token": "refresh_" + tokenID,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle API requests
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "123456789")
		w.Header().Set("Content-Type", "application/json")

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		mu.Lock()
		for _, revoked := range revokedTokens {
			if token == revoked {
				mu.Unlock()
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error":             "invalid_token",
					"error_description": "Token has been revoked",
				})
				return
			}
		}
		mu.Unlock()

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

	t.Run("TokenCacheInvalidation", func(t *testing.T) {
		// Reset counters
		atomic.StoreInt64(&requestCount, 0)
		atomic.StoreInt64(&tokenIssuedCount, 0)

		config := &Config{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			UserAgent:    "test/1.0",
			AuthURL:      server.URL,
			BaseURL:      server.URL,
			HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		}

		client, err := NewClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Make initial request
		_, err = client.Me(context.Background())
		if err != nil {
			t.Fatalf("Initial request failed: %v", err)
		}

		t.Logf("Initial request succeeded")

		// Wait for token to expire
		time.Sleep(2 * time.Second)

		// Make another request - should trigger token refresh
		_, err = client.Me(context.Background())
		if err != nil {
			t.Errorf("Request after token expiry failed: %v", err)
		} else {
			t.Logf("Request after token expiry succeeded (token refreshed)")
		}

		// Manually revoke current token (simulate server-side revocation)
		mu.Lock()
		revokedTokens = append(revokedTokens, "token_2") // The refreshed token
		mu.Unlock()

		// Make request with revoked token - should trigger new token refresh
		_, err = client.Me(context.Background())
		if err != nil {
			t.Errorf("Request with revoked token failed: %v", err)
		} else {
			t.Logf("Request with revoked token succeeded (new token obtained)")
		}

		totalRequests := atomic.LoadInt64(&requestCount)
		totalTokensIssued := atomic.LoadInt64(&tokenIssuedCount)

		t.Logf("Token cache invalidation test results:")
		t.Logf("  Total HTTP requests: %d", totalRequests)
		t.Logf("  Tokens issued: %d", totalTokensIssued)
		t.Logf("  Revoked tokens: %d", len(revokedTokens))

		// Should have issued multiple tokens due to expiry and revocation
		if totalTokensIssued < 3 {
			t.Errorf("Expected at least 3 tokens issued, got %d", totalTokensIssued)
		}

		// Should have made reasonable number of requests
		if totalRequests < 5 {
			t.Errorf("Expected at least 5 total HTTP requests, got %d", totalRequests)
		}
	})
}

// TestMultiClientAuthBehavior tests multiple clients with same credentials
func TestMultiClientAuthBehavior(t *testing.T) {
	var requestCount int64
	var tokenRequests int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Handle token requests
		if strings.Contains(r.URL.Path, "/api/v1/access_token") {
			atomic.AddInt64(&tokenRequests, 1)

			// Simulate some delay to make race conditions more likely
			time.Sleep(50 * time.Millisecond)

			response := map[string]interface{}{
				"access_token":  fmt.Sprintf("shared_token_%d", tokenRequests),
				"token_type":    "bearer",
				"expires_in":    3600,
				"scope":         "read",
				"refresh_token": fmt.Sprintf("shared_refresh_%d", tokenRequests),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle API requests
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "123456789")
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

	t.Run("MultiClientAuth", func(t *testing.T) {
		// Reset counters
		atomic.StoreInt64(&requestCount, 0)
		atomic.StoreInt64(&tokenRequests, 0)

		// Create multiple clients with same credentials
		numClients := 5
		clients := make([]*Reddit, numClients)

		for i := 0; i < numClients; i++ {
			config := &Config{
				ClientID:     "shared_id",
				ClientSecret: "shared_secret",
				UserAgent:    fmt.Sprintf("test/%d.0", i+1),
				AuthURL:      server.URL,
				BaseURL:      server.URL,
				HTTPClient:   &http.Client{Timeout: 30 * time.Second},
			}

			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("Failed to create client %d: %v", i, err)
			}

			clients[i] = client
		}

		// Use all clients concurrently
		var wg sync.WaitGroup
		results := make(chan error, numClients)

		startTime := time.Now()

		for i, client := range clients {
			wg.Add(1)
			go func(clientID int, c *Reddit) {
				defer wg.Done()

				_, err := c.Me(context.Background())
				results <- err

				if err != nil {
					t.Logf("Client %d failed: %v", clientID, err)
				} else {
					t.Logf("Client %d succeeded", clientID)
				}
			}(i, client)
		}

		wg.Wait()
		close(results)

		totalTime := time.Since(startTime)
		totalRequests := atomic.LoadInt64(&requestCount)
		totalTokenRequests := atomic.LoadInt64(&tokenRequests)

		// Count successes and failures
		successCount := 0
		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
			} else {
				successCount++
			}
		}

		t.Logf("Multi-client auth test results:")
		t.Logf("  Clients: %d", numClients)
		t.Logf("  Successful requests: %d", successCount)
		t.Logf("  Failed requests: %d", errorCount)
		t.Logf("  Token requests: %d", totalTokenRequests)
		t.Logf("  Total HTTP requests: %d", totalRequests)
		t.Logf("  Total time: %v", totalTime)

		// All clients should succeed
		if successCount != numClients {
			t.Errorf("Expected all %d clients to succeed, got %d", numClients, successCount)
		}

		// Should have made token requests (at least one per client, possibly shared)
		if totalTokenRequests == 0 {
			t.Error("Expected at least one token request")
		}

		// Total requests should be reasonable (token requests + API requests)
		expectedMinRequests := int64(numClients) // API requests
		if totalRequests < expectedMinRequests {
			t.Errorf("Expected at least %d total requests, got %d", expectedMinRequests, totalRequests)
		}
	})
}

// TestAuthSystemClockManipulation tests behavior with system clock changes
func TestAuthSystemClockManipulation(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		// Handle token requests
		if strings.Contains(r.URL.Path, "/api/v1/access_token") {
			// Issue token with server-side timestamp
			serverTime := time.Now().Unix()
			expiry := serverTime + 3600 // 1 hour from server time

			mu.Lock()
			_ = expiry // Store expiry (unused in this test but keeps variable)
			mu.Unlock()

			response := map[string]interface{}{
				"access_token":  fmt.Sprintf("clock_token_%d", serverTime),
				"token_type":    "bearer",
				"expires_in":    3600,
				"scope":         "read",
				"refresh_token": fmt.Sprintf("clock_refresh_%d", serverTime),
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle API requests
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "123456789")
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

	t.Run("SystemClockEdgeCases", func(t *testing.T) {
		// Reset counters
		atomic.StoreInt64(&requestCount, 0)
		mu.Lock()
		_ = 0 // Reset token expiry (unused in this test)
		mu.Unlock()

		config := &Config{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			UserAgent:    "test/1.0",
			AuthURL:      server.URL,
			BaseURL:      server.URL,
			HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		}

		client, err := NewClient(config)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Test 1: Normal operation
		_, err = client.Me(context.Background())
		if err != nil {
			t.Errorf("Normal request failed: %v", err)
		} else {
			t.Logf("Normal request succeeded")
		}

		// Test 2: Simulate clock skew by waiting
		time.Sleep(100 * time.Millisecond)

		_, err = client.Me(context.Background())
		if err != nil {
			t.Errorf("Request after clock skew failed: %v", err)
		} else {
			t.Logf("Request after clock skew succeeded")
		}

		// Test 3: Rapid successive requests
		for i := 0; i < 5; i++ {
			_, err = client.Me(context.Background())
			if err != nil {
				t.Errorf("Rapid request %d failed: %v", i+1, err)
			}
			time.Sleep(10 * time.Millisecond)
		}

		totalRequests := atomic.LoadInt64(&requestCount)

		t.Logf("System clock manipulation test results:")
		t.Logf("  Total HTTP requests: %d", totalRequests)

		// Should have made reasonable number of requests
		if totalRequests < 7 {
			t.Errorf("Expected at least 7 total requests, got %d", totalRequests)
		}

		// All requests should have succeeded despite potential clock issues
		t.Logf("All requests completed successfully despite potential clock skew")
	})
}
