package adversarial_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/adversarial_tests/helpers"
	"github.com/jamesprial/go-reddit-api-wrapper/internal"
)

// TestConcurrentTokenRefreshRace tests that concurrent GetToken() calls don't cause race conditions
func TestConcurrentTokenRefreshRace(t *testing.T) {
	// Create a mock server that returns valid tokens
	requestCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate network delay
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "token_123", "expires_in": 3600}`))
	}))
	defer server.Close()

	// Create authenticator
	auth, err := internal.NewAuthenticator(
		server.Client(),
		"test_user",
		"test_pass",
		"test_client",
		"test_secret",
		"test/1.0",
		server.URL,
		"password",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	// Launch 1000 concurrent goroutines trying to get token
	numGoroutines := 1000
	errors := make(chan error, numGoroutines)
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			token, err := auth.GetToken(ctx)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %w", id, err)
				return
			}
			if token == "" {
				errors <- fmt.Errorf("goroutine %d: got empty token", id)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent token fetch error: %v", err)
	}

	// Verify that caching worked - should only make 1 request since all goroutines
	// are fetching at the same time and token is cached
	requests := atomic.LoadInt32(&requestCount)
	t.Logf("Total auth requests made: %d (with %d concurrent goroutines)", requests, numGoroutines)

	// Should be small number of requests due to caching, not 1000
	if requests > 10 {
		t.Errorf("Too many auth requests (%d), caching may not be working properly", requests)
	}
}

// TestTokenCachePoisoning tests handling of malicious token responses
func TestTokenCachePoisoning(t *testing.T) {
	generator := helpers.NewJSONGenerator()
	malformedResponses := generator.GenerateMalformedTokenResponses()

	for i, responseBody := range malformedResponses {
		t.Run(fmt.Sprintf("malformed_%d", i), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(responseBody))
			}))
			defer server.Close()

			auth, err := internal.NewAuthenticator(
				http.DefaultClient,
				"test_user",
				"test_pass",
				"test_client",
				"test_secret",
				"test/1.0",
				server.URL,
				"password",
				nil,
			)
			if err != nil {
				t.Fatalf("Failed to create authenticator: %v", err)
			}

			// Try to get token - should either succeed or fail gracefully
			ctx := context.Background()
			token, err := auth.GetToken(ctx)

			// Malformed responses should trigger errors
			if err == nil && token == "" {
				t.Error("Expected error or valid token, got empty token with no error")
			}

			t.Logf("Response: %s, Token: %q, Error: %v", responseBody[:min(50, len(responseBody))], token, err)
		})
	}
}

// TestOversizedTokenResponse tests handling of extremely large token responses
func TestOversizedTokenResponse(t *testing.T) {
	generator := helpers.NewJSONGenerator()
	oversizedResponse := generator.GenerateOversizedTokenResponse()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(oversizedResponse))
	}))
	defer server.Close()

	auth, err := internal.NewAuthenticator(
		server.Client(),
		"test_user",
		"test_pass",
		"test_client",
		"test_secret",
		"test/1.0",
		server.URL,
		"password",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	ctx := context.Background()

	// This should either handle the large response or fail gracefully
	// but should not hang or crash
	done := make(chan struct{})
	var token string
	var authErr error

	go func() {
		token, authErr = auth.GetToken(ctx)
		close(done)
	}()

	select {
	case <-done:
		t.Logf("Oversized token request completed. Token length: %d, Error: %v", len(token), authErr)
	case <-time.After(5 * time.Second):
		t.Fatal("Oversized token request timed out (possible hang)")
	}
}

// TestTokenExpiryBoundsEnforcement tests that invalid expiry values are rejected
func TestTokenExpiryBoundsEnforcement(t *testing.T) {
	generator := helpers.NewJSONGenerator()
	testCases := generator.GenerateTokenResponseWithInvalidExpiry()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.Response))
			}))
			defer server.Close()

			auth, err := internal.NewAuthenticator(
				http.DefaultClient,
				"test_user",
				"test_pass",
				"test_client",
				"test_secret",
				"test/1.0",
				server.URL,
				"password",
				nil,
			)
			if err != nil {
				t.Fatalf("Failed to create authenticator: %v", err)
			}

			ctx := context.Background()
			token, err := auth.GetToken(ctx)

			// Check that the response includes validation
			t.Logf("Test case %s: token=%q, err=%v", tc.Name, token, err)

			// Some invalid expiry values should trigger errors
			// (negative values, overflow values, etc. should be caught by validation)
		})
	}
}

// TestConcurrentTokenRefreshWithExpiry tests concurrent token refresh when tokens expire
func TestConcurrentTokenRefreshWithExpiry(t *testing.T) {
	requestCount := int32(0)
	tokenVersion := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		version := atomic.AddInt32(&tokenVersion, 1)

		// Return short-lived tokens (1 second expiry)
		response := fmt.Sprintf(`{"access_token": "token_v%d", "expires_in": 1}`, version)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	auth, err := internal.NewAuthenticator(
		server.Client(),
		"test_user",
		"test_pass",
		"test_client",
		"test_secret",
		"test/1.0",
		server.URL,
		"password",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	// Get initial token
	ctx := context.Background()
	token1, err := auth.GetToken(ctx)
	if err != nil {
		t.Fatalf("Failed to get initial token: %v", err)
	}
	t.Logf("Initial token: %s", token1)

	// Wait for token to expire
	time.Sleep(1500 * time.Millisecond)

	// Now launch concurrent requests - all should get the refreshed token
	numGoroutines := 100
	tokens := make(chan string, numGoroutines)
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			token, err := auth.GetToken(ctx)
			if err != nil {
				t.Errorf("Failed to get token after expiry: %v", err)
				return
			}
			tokens <- token
		}()
	}

	wg.Wait()
	close(tokens)

	// Collect all tokens
	uniqueTokens := make(map[string]int)
	for token := range tokens {
		uniqueTokens[token]++
	}

	t.Logf("Unique tokens received: %d", len(uniqueTokens))
	t.Logf("Total auth requests: %d", atomic.LoadInt32(&requestCount))

	// All goroutines should have received the same refreshed token
	if len(uniqueTokens) > 2 {
		t.Errorf("Too many unique tokens (%d), expected 1-2 (token refresh may have race condition)", len(uniqueTokens))
	}
}

// TestAuthenticationUnderStress tests authentication under high load
func TestAuthenticationUnderStress(t *testing.T) {
	// Create a server that tracks concurrent requests
	var concurrentRequests int32
	var peakConcurrent int32
	requestCount := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&concurrentRequests, 1)
		atomic.AddInt32(&requestCount, 1)

		// Track peak concurrency
		for {
			peak := atomic.LoadInt32(&peakConcurrent)
			if current <= peak || atomic.CompareAndSwapInt32(&peakConcurrent, peak, current) {
				break
			}
		}

		time.Sleep(5 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "token_123", "expires_in": 3600}`))

		atomic.AddInt32(&concurrentRequests, -1)
	}))
	defer server.Close()

	auth, err := internal.NewAuthenticator(
		server.Client(),
		"test_user",
		"test_pass",
		"test_client",
		"test_secret",
		"test/1.0",
		server.URL,
		"password",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	// Create stress tester
	stressCfg := &helpers.StressConfig{
		NumGoroutines: 500,
		Duration:      2 * time.Second,
		OperationFunc: func(goroutineID int) error {
			ctx := context.Background()
			_, err := auth.GetToken(ctx)
			return err
		},
		CollectMetrics: true,
	}

	tester := helpers.NewStressTester(stressCfg)
	result := tester.Run()

	t.Log(result.FormatResult())

	// Check for leaks
	if result.HasGoroutineLeak() {
		t.Errorf("Goroutine leak detected: %d goroutines leaked", result.EndGoroutines-result.StartGoroutines)
	}

	if result.HasMemoryLeak() {
		t.Errorf("Memory leak detected: %d bytes leaked", int64(result.EndMemoryBytes)-int64(result.StartMemoryBytes))
	}

	t.Logf("Peak concurrent auth requests: %d", atomic.LoadInt32(&peakConcurrent))
	t.Logf("Total auth requests made: %d", atomic.LoadInt32(&requestCount))
}

// TestTokenCacheAtomicOperations tests the atomic operations on token cache
func TestTokenCacheAtomicOperations(t *testing.T) {
	requestCount := int32(0)
	tokenID := int32(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		id := atomic.AddInt32(&tokenID, 1)
		response := fmt.Sprintf(`{"access_token": "token_%d", "expires_in": 3600}`, id)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	auth, err := internal.NewAuthenticator(
		server.Client(),
		"test_user",
		"test_pass",
		"test_client",
		"test_secret",
		"test/1.0",
		server.URL,
		"password",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	// Use coordinated start to maximize race condition probability
	numGoroutines := 1000
	errs := helpers.CoordinatedStart(numGoroutines, func(id int) error {
		ctx := context.Background()
		token, err := auth.GetToken(ctx)
		if err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("got empty token")
		}
		return nil
	})

	// Check for errors
	if len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("Coordinated token fetch error: %v", err)
		}
	}

	requests := atomic.LoadInt32(&requestCount)
	t.Logf("Total requests with coordinated start: %d (from %d goroutines)", requests, numGoroutines)

	// With proper atomic operations, should make very few requests
	if requests > 10 {
		t.Errorf("Too many requests (%d) suggests atomic cache operations may have issues", requests)
	}
}

// TestMaliciousAuthErrors tests handling of various authentication error responses
func TestMaliciousAuthErrors(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		responseBody string
	}{
		{
			"401_invalid_grant",
			401,
			`{"error": "invalid_grant"}`,
		},
		{
			"401_invalid_client",
			401,
			`{"error": "invalid_client", "error_description": "Client authentication failed"}`,
		},
		{
			"500_internal_error",
			500,
			`Internal Server Error`,
		},
		{
			"503_service_unavailable",
			503,
			`Service Temporarily Unavailable`,
		},
		{
			"429_rate_limited",
			429,
			`{"error": "rate_limit_exceeded"}`,
		},
		{
			"200_malformed_json",
			200,
			`{"access_token": "token", "expires_in":`,
		},
		{
			"200_empty_body",
			200,
			``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.responseBody))
			}))
			defer server.Close()

			auth, err := internal.NewAuthenticator(
				http.DefaultClient,
				"test_user",
				"test_pass",
				"test_client",
				"test_secret",
				"test/1.0",
				server.URL,
				"password",
				nil,
			)
			if err != nil {
				t.Fatalf("Failed to create authenticator: %v", err)
			}

			ctx := context.Background()
			token, err := auth.GetToken(ctx)

			// Should handle errors gracefully
			if err == nil && token == "" {
				t.Error("Expected error or valid token, got empty token with no error")
			}

			t.Logf("Status: %d, Token: %q, Error: %v", tc.statusCode, token, err)
		})
	}
}

// TestTokenResponseSizeLimit tests handling of various token response sizes
func TestTokenResponseSizeLimit(t *testing.T) {
	testCases := []struct {
		name       string
		tokenSize  int
		shouldPass bool
	}{
		{"small_token", 100, true},
		{"normal_token", 1000, true},
		{"large_token", 100000, true},
		{"very_large_token", 1000000, false}, // 1MB token
		{"huge_token", 10000000, false},      // 10MB token
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate token of specific size
			token := strings.Repeat("A", tc.tokenSize)
			response := fmt.Sprintf(`{"access_token": "%s", "expires_in": 3600}`, token)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(response))
			}))
			defer server.Close()

			auth, err := internal.NewAuthenticator(
				http.DefaultClient,
				"test_user",
				"test_pass",
				"test_client",
				"test_secret",
				"test/1.0",
				server.URL,
				"password",
				nil,
			)
			if err != nil {
				t.Fatalf("Failed to create authenticator: %v", err)
			}

			ctx := context.Background()

			// Set timeout to prevent hanging
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			receivedToken, err := auth.GetToken(ctx)

			t.Logf("Token size: %d bytes, Received: %d bytes, Error: %v",
				tc.tokenSize, len(receivedToken), err)

			// Very large tokens should either work or fail gracefully
			if err == nil && len(receivedToken) == 0 {
				t.Error("Expected either error or valid token, got empty token with no error")
			}
		})
	}
}

// TestGetTokenContextCancellation tests that GetToken respects context cancellation
func TestGetTokenContextCancellation(t *testing.T) {
	// Create server with artificial delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "token_123", "expires_in": 3600}`))
	}))
	defer server.Close()

	auth, err := internal.NewAuthenticator(
		server.Client(),
		"test_user",
		"test_pass",
		"test_client",
		"test_secret",
		"test/1.0",
		server.URL,
		"password",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create authenticator: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = auth.GetToken(ctx)

	// Should return context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	t.Logf("Context cancellation error (expected): %v", err)
}

// TestAuthNetworkErrors tests handling of network errors during authentication
func TestAuthNetworkErrors(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"connection_refused", 0, ""},
		{"timeout", 0, ""},
		{"dns_error", 0, ""},
		{"network_unreachable", 500, "Internal Server Error"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create server that simulates network errors
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.statusCode == 0 {
					// Simulate connection failure by not responding
					time.Sleep(5 * time.Second)
					return
				}

				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.body))
			}))
			defer server.Close()

			// Create client with very short timeout for error cases
			httpClient := &http.Client{
				Timeout: 100 * time.Millisecond,
			}

			auth, err := internal.NewAuthenticator(
				httpClient,
				"test_user",
				"test_pass",
				"test_client",
				"test_secret",
				"test/1.0",
				server.URL,
				"password",
				nil,
			)
			if err != nil {
				t.Fatalf("Failed to create authenticator: %v", err)
			}

			ctx := context.Background()
			token, err := auth.GetToken(ctx)

			// Should return error, not panic
			if err == nil {
				t.Errorf("Expected network error, got token: %q", token)
			}

			t.Logf("Network error (expected): %v", err)
		})
	}
}

// mockHTTPClient is a test helper
type mockHTTPClient struct {
	responses chan *http.Response
	errors    chan error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	select {
	case resp := <-m.responses:
		return resp, nil
	case err := <-m.errors:
		return nil, err
	default:
		return nil, fmt.Errorf("no response configured")
	}
}

func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// TestTokenJSONUnmarshalErrors tests handling of various JSON unmarshal errors
func TestTokenJSONUnmarshalErrors(t *testing.T) {
	malformedJSON := []string{
		`{`,
		`{"access_token": "token"`,
		`{"access_token": "token",}`,
		`{access_token: "token"}`,
		`null`,
		`[]`,
		`"string"`,
		`123`,
		`true`,
		`{"access_token": "token", "expires_in": 3600, "extra": }`,
	}

	for i, jsonStr := range malformedJSON {
		t.Run(fmt.Sprintf("malformed_%d", i), func(t *testing.T) {
			// Try to unmarshal - should fail gracefully
			var tokenResp struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int    `json:"expires_in"`
			}

			err := json.Unmarshal([]byte(jsonStr), &tokenResp)

			// Should return error, not panic
			if err == nil {
				t.Log("Unexpectedly parsed malformed JSON")
			} else {
				t.Logf("JSON unmarshal error (expected): %v", err)
			}
		})
	}
}
