package graw

import (
	"net/http"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/validator"
)

// NewTestClient creates a test Reddit client connected to a MockServer.
// This is the most common factory for testing as it provides a complete mock server
// with configurable responses.
//
// The client is configured with:
//   - 30 second timeout
//   - "test-client/1.0" user agent
//   - Mock token provider returning "test_token"
//   - Standard parser and validator
//
// Example:
//
//	server := testutil.NewMockServer().
//	    WithPosts("golang", "hot",
//	        testutil.NewPostBuilder().WithTitle("Test Post").Build()).
//	    Start()
//	defer server.Close()
//
//	client := graw.NewTestClient(server)
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang"})
func NewTestClient(mockServer *testutil.MockServer) *Reddit {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClient(httpClient, mockServer.URL(), "test-client/1.0", nil)
	if err != nil {
		// This should never happen in tests with valid inputs
		panic("failed to create test HTTP client: " + err.Error())
	}

	return &Reddit{
		httpClient: internalClient,
		auth:       &testutil.MockTokenProvider{Token: "test_token"},
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
	}
}

// NewTestClientWithMocks creates a test Reddit client with custom mock implementations.
// Use this when you need fine-grained control over authentication or HTTP behavior.
//
// The client is configured with:
//   - The provided auth token provider
//   - The provided HTTP client
//   - Standard parser and validator
//
// This is useful for testing:
//   - Authentication failures (provide a mock auth that returns errors)
//   - Network errors (provide a mock HTTP client that returns errors)
//   - Rate limiting scenarios (provide a custom HTTP client with rate limit simulation)
//
// Example:
//
//	mockAuth := &testutil.MockTokenProvider{Err: errors.New("auth failed")}
//	mockHTTP := &customMockHTTP{} // your custom implementation
//
//	client := graw.NewTestClientWithMocks(mockAuth, mockHTTP)
//	_, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang"})
//	// err will be related to authentication failure
func NewTestClientWithMocks(authProvider TokenProvider, httpClient HTTPClient) *Reddit {
	return &Reddit{
		httpClient: httpClient,
		auth:       authProvider,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
	}
}

// NewTestClientWithURL creates a test Reddit client pointed at a specific base URL.
// Use this when you need to point the client at a custom test server URL.
//
// The client is configured with:
//   - 30 second timeout
//   - "test-client/1.0" user agent
//   - Mock token provider returning "test_token"
//   - Standard parser and validator
//
// This is useful for:
//   - Testing against custom httptest servers
//   - Integration tests against staging environments
//   - Testing URL handling and routing
//
// Example:
//
//	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    // Custom handler logic
//	    w.WriteHeader(http.StatusOK)
//	    json.NewEncoder(w).Encode(customResponse)
//	}))
//	defer server.Close()
//
//	client := graw.NewTestClientWithURL(server.URL)
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "test"})
func NewTestClientWithURL(baseURL string) *Reddit {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClient(httpClient, baseURL, "test-client/1.0", nil)
	if err != nil {
		// This should never happen in tests with valid inputs
		panic("failed to create test HTTP client: " + err.Error())
	}

	return &Reddit{
		httpClient: internalClient,
		auth:       &testutil.MockTokenProvider{Token: "test_token"},
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
	}
}

// NewTestClientWithRateLimit creates a test Reddit client with rate limiting enabled.
// This is useful for testing rate limiting behavior and ensuring the client
// respects Reddit's rate limits.
//
// The client is configured with:
//   - 30 second timeout
//   - "test-client/1.0" user agent
//   - Mock token provider returning "test_token"
//   - Rate limiting with the provided configuration
//   - Standard parser and validator
//
// NOTE: This factory uses real time for rate limiting, so tests may need to use
// time.Sleep() for rate limit recovery testing. For tests that need time control,
// consider using NewTestClientWithMocks and providing a custom HTTP client.
//
// Example:
//
//	server := testutil.NewMockServer().
//	    WithAccount(testutil.NewAccount("testuser").Build()).
//	    Start()
//	defer server.Close()
//
//	config := graw.RateLimitConfig{
//	    RequestsPerMinute:  60,
//	    Burst:              10,
//	    ProactiveThreshold: 5,
//	}
//
//	client := graw.NewTestClientWithRateLimit(server.URL(), config)
//	// Client will enforce rate limiting
func NewTestClientWithRateLimit(baseURL string, rateLimitConfig RateLimitConfig) *Reddit {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Convert public RateLimitConfig to internal client.RateLimitConfig
	// Both types have the same structure, so direct assignment works
	internalRateLimitConfig := client.RateLimitConfig{
		RequestsPerMinute:  rateLimitConfig.RequestsPerMinute,
		Burst:              rateLimitConfig.Burst,
		ProactiveThreshold: rateLimitConfig.ProactiveThreshold,
	}

	// Use real clock for rate limiting (tests will use real time)
	realClock := clock.NewRealClock()

	internalClient, err := client.NewClientWithRateLimit(
		httpClient,
		baseURL,
		"test-client/1.0",
		nil,
		internalRateLimitConfig,
		realClock,
	)
	if err != nil {
		// This should never happen in tests with valid inputs
		panic("failed to create rate-limited test HTTP client: " + err.Error())
	}

	return &Reddit{
		httpClient: internalClient,
		auth:       &testutil.MockTokenProvider{Token: "test_token"},
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
	}
}
