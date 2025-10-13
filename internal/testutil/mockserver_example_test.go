package testutil_test

import (
	"fmt"
	"net/http"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/testutil"
)

// This example demonstrates how to use MockServer with the fluent builder API
// to create a comprehensive test environment for Reddit API client testing.
func ExampleMockServer() {
	// Create test data using the fluent builders
	sub := testutil.NewSubreddit("golang").
		WithSubscribers(500000).
		WithTitle("The Go Programming Language").
		Build()

	post1 := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Introduction to Go").
		WithScore(1500).
		Build()

	post2 := testutil.NewPostBuilder().
		WithID("post2").
		WithTitle("Go 1.21 Released").
		WithScore(2500).
		Build()

	// Configure and start the mock server
	server := testutil.NewMockServer().
		WithSubreddit("golang", sub).
		WithPosts("golang", "hot", post1, post2).
		Start()
	defer server.Close()

	// Make HTTP requests to the mock server
	resp, _ := http.Get(server.URL() + "/r/golang/about")
	fmt.Printf("Subreddit status: %d\n", resp.StatusCode)
	resp.Body.Close()

	resp, _ = http.Get(server.URL() + "/r/golang/hot")
	fmt.Printf("Posts status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Output:
	// Subreddit status: 200
	// Posts status: 200
}

// This example shows how to test error handling with MockServer.
func ExampleMockServer_errorHandling() {
	// Configure a server that returns errors for certain paths
	server := testutil.NewMockServer().
		WithError("/r/private", http.StatusForbidden, "This subreddit is private").
		WithError("/r/banned", http.StatusForbidden, "You are banned from this subreddit").
		Start()
	defer server.Close()

	// Try to access the private subreddit
	resp, _ := http.Get(server.URL() + "/r/private/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Try to access the banned subreddit
	resp, _ = http.Get(server.URL() + "/r/banned/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Try to access a non-configured subreddit (returns empty listing)
	resp, _ = http.Get(server.URL() + "/r/unconfigured/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Output:
	// Status: 403
	// Status: 403
	// Status: 200
}

// This example demonstrates testing different sort orders.
func ExampleMockServer_sortOrders() {
	// Create different posts for different sort orders
	hotPost1 := testutil.NewPostBuilder().
		WithID("hot1").
		WithTitle("Trending Now").
		WithScore(1000).
		Build()

	newPost1 := testutil.NewPostBuilder().
		WithID("new1").
		WithTitle("Just Posted").
		WithScore(10).
		Build()

	topPost1 := testutil.NewPostBuilder().
		WithID("top1").
		WithTitle("All Time Best").
		WithScore(10000).
		Build()

	// Configure different posts for each sort order
	server := testutil.NewMockServer().
		WithPosts("golang", "hot", hotPost1).
		WithPosts("golang", "new", newPost1).
		WithPosts("golang", "top", topPost1).
		Start()
	defer server.Close()

	// Test each sort order
	endpoints := []string{"/hot", "/new", "/top"}
	for _, endpoint := range endpoints {
		resp, _ := http.Get(server.URL() + "/r/golang" + endpoint)
		fmt.Printf("%s - Status: %d\n", endpoint, resp.StatusCode)
		resp.Body.Close()
	}

	// Output:
	// /hot - Status: 200
	// /new - Status: 200
	// /top - Status: 200
}
