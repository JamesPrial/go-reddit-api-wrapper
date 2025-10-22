package testutil_test

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

// ExampleMockServer_withStatusCode demonstrates how to test error handling
// with global status code overrides.
func ExampleMockServer_withStatusCode() {
	// Create a server that returns 503 Service Unavailable for all requests
	server := testutil.NewMockServer().
		WithStatusCode(http.StatusServiceUnavailable).
		Start()
	defer server.Close()

	resp, _ := http.Get(server.URL() + "/r/golang/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Output:
	// Status: 503
}

// ExampleMockServer_withMalformedJSON demonstrates testing JSON parsing errors.
func ExampleMockServer_withMalformedJSON() {
	// Create a server that returns malformed JSON
	server := testutil.NewMockServer().
		WithMalformedJSON().
		Start()
	defer server.Close()

	resp, _ := http.Get(server.URL() + "/r/golang/hot")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// The response will be incomplete JSON
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Is valid JSON: %t\n", string(body)[len(body)-1] == '}')

	// Output:
	// Status: 200
	// Is valid JSON: false
}

// ExampleMockServer_withEmptyResponse demonstrates testing empty response handling.
func ExampleMockServer_withEmptyResponse() {
	// Create a server that returns empty responses
	server := testutil.NewMockServer().
		WithEmptyResponse().
		Start()
	defer server.Close()

	resp, _ := http.Get(server.URL() + "/r/golang/hot")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Body length: %d\n", len(body))

	// Output:
	// Status: 200
	// Body length: 0
}

// ExampleMockServer_withTimeout demonstrates testing timeout scenarios.
func ExampleMockServer_withTimeout() {
	// Create a server with a 100ms delay
	server := testutil.NewMockServer().
		WithTimeout(100 * time.Millisecond).
		WithEmptyResponse().
		Start()
	defer server.Close()

	start := time.Now()
	resp, _ := http.Get(server.URL() + "/r/golang/hot")
	duration := time.Since(start)
	resp.Body.Close()

	fmt.Printf("Request took at least 100ms: %t\n", duration >= 100*time.Millisecond)

	// Output:
	// Request took at least 100ms: true
}

// ExampleMockServer_withPaginatedPosts demonstrates setting up multi-page responses.
func ExampleMockServer_withPaginatedPosts() {
	// Create posts for different pages
	post1 := testutil.NewPostBuilder().WithID("post1").WithTitle("First").Build()
	post2 := testutil.NewPostBuilder().WithID("post2").WithTitle("Second").Build()
	post3 := testutil.NewPostBuilder().WithID("post3").WithTitle("Third").Build()
	post4 := testutil.NewPostBuilder().WithID("post4").WithTitle("Fourth").Build()

	// Configure pagination - the map key is the "after" parameter value
	pages := map[string][]*types.Post{
		"":         {post1, post2}, // First page (no after param)
		"t3_post2": {post3, post4}, // Second page (after=t3_post2)
	}

	server := testutil.NewMockServer().
		WithPaginatedPosts("golang", "hot", pages).
		Start()
	defer server.Close()

	// Get first page
	resp1, _ := http.Get(server.URL() + "/r/golang/hot")
	fmt.Printf("First page status: %d\n", resp1.StatusCode)
	resp1.Body.Close()

	// Get second page
	resp2, _ := http.Get(server.URL() + "/r/golang/hot?after=t3_post2")
	fmt.Printf("Second page status: %d\n", resp2.StatusCode)
	resp2.Body.Close()

	// Output:
	// First page status: 200
	// Second page status: 200
}

// ExampleMockServer_chainedConfiguration demonstrates combining multiple configuration methods.
func ExampleMockServer_chainedConfiguration() {
	// Create test data
	subreddit := testutil.NewSubreddit("golang").
		WithTitle("The Go Programming Language").
		Build()

	post := testutil.NewPostBuilder().
		WithID("test123").
		WithTitle("Test Post").
		Build()

	// Combine multiple configurations
	server := testutil.NewMockServer().
		WithSubreddit("golang", subreddit).
		WithPosts("golang", "hot", post).
		WithError("/r/private", http.StatusForbidden, "Access denied").
		Start()
	defer server.Close()

	// Test normal endpoint
	resp1, _ := http.Get(server.URL() + "/r/golang/hot")
	fmt.Printf("Public subreddit: %d\n", resp1.StatusCode)
	resp1.Body.Close()

	// Test error endpoint
	resp2, _ := http.Get(server.URL() + "/r/private/hot")
	fmt.Printf("Private subreddit: %d\n", resp2.StatusCode)
	resp2.Body.Close()

	// Output:
	// Public subreddit: 200
	// Private subreddit: 403
}

// ExampleNewCustomResponseServer demonstrates creating a server with custom response logic.
func ExampleNewCustomResponseServer() {
	requestCount := 0

	// Create a server with completely custom response logic
	server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		// First request fails, subsequent requests succeed
		if requestCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Internal Server Error"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success"}`))
		}
	})
	defer server.Close()

	// First request fails
	resp1, _ := http.Get(server.URL)
	fmt.Printf("First request: %d\n", resp1.StatusCode)
	resp1.Body.Close()

	// Second request succeeds
	resp2, _ := http.Get(server.URL)
	fmt.Printf("Second request: %d\n", resp2.StatusCode)
	resp2.Body.Close()

	// Output:
	// First request: 500
	// Second request: 200
}

// ExampleNewCustomResponseServer_networkError demonstrates simulating network errors.
func ExampleNewCustomResponseServer_networkError() {
	// Create a server that simulates incomplete responses
	server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write partial JSON to simulate network interruption
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))
		// Response ends abruptly when handler returns
	})
	defer server.Close()

	resp, _ := http.Get(server.URL)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response incomplete: %t\n", string(body)[len(body)-1] != '}')

	// Output:
	// Status: 200
	// Response incomplete: true
}

// ExampleMockServer_errorPriority demonstrates how error configurations are prioritized.
func ExampleMockServer_errorPriority() {
	post := testutil.NewPostBuilder().WithID("test").Build()

	// Global status code overrides specific configurations
	server := testutil.NewMockServer().
		WithPosts("golang", "hot", post).      // Configure posts
		WithStatusCode(http.StatusBadGateway). // But override with 502
		Start()
	defer server.Close()

	resp, _ := http.Get(server.URL() + "/r/golang/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Output:
	// Status: 502
}

// ExampleMockServer_multiplePaginatedSubreddits demonstrates pagination across multiple subreddits.
func ExampleMockServer_multiplePaginatedSubreddits() {
	// Create posts for golang subreddit
	golangPost1 := testutil.NewPostBuilder().WithID("go1").WithTitle("Go 1").Build()
	golangPost2 := testutil.NewPostBuilder().WithID("go2").WithTitle("Go 2").Build()

	golangPages := map[string][]*types.Post{
		"": {golangPost1, golangPost2},
	}

	// Create posts for rust subreddit
	rustPost1 := testutil.NewPostBuilder().WithID("rust1").WithTitle("Rust 1").Build()
	rustPost2 := testutil.NewPostBuilder().WithID("rust2").WithTitle("Rust 2").Build()

	rustPages := map[string][]*types.Post{
		"": {rustPost1, rustPost2},
	}

	server := testutil.NewMockServer().
		WithPaginatedPosts("golang", "hot", golangPages).
		WithPaginatedPosts("rust", "hot", rustPages).
		Start()
	defer server.Close()

	// Get golang posts
	resp1, _ := http.Get(server.URL() + "/r/golang/hot")
	fmt.Printf("Golang status: %d\n", resp1.StatusCode)
	resp1.Body.Close()

	// Get rust posts
	resp2, _ := http.Get(server.URL() + "/r/rust/hot")
	fmt.Printf("Rust status: %d\n", resp2.StatusCode)
	resp2.Body.Close()

	// Output:
	// Golang status: 200
	// Rust status: 200
}
