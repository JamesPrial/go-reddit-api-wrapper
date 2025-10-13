package testutil_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// This example demonstrates how to use MockServer with the fluent builder API
// to create a comprehensive test environment for Reddit API client testing.
func ExampleMockServer() {
	// Step 1: Create test data using the fluent builders

	// Create a subreddit
	sub := testutil.NewSubreddit("golang").
		WithSubscribers(500000).
		WithTitle("The Go Programming Language").
		WithDescription("The Go programming language community").
		Build()

	// Create posts for the subreddit
	post1 := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Introduction to Go Concurrency").
		WithAuthor("gopher").
		WithScore(1500).
		WithNumComments(120).
		Build()

	post2 := testutil.NewPostBuilder().
		WithID("post2").
		WithTitle("Go 1.21 Released").
		WithAuthor("golang_team").
		WithScore(2500).
		WithNumComments(350).
		Build()

	// Create a post with comments
	mainPost := testutil.NewPostBuilder().
		WithID("abc123").
		WithTitle("Ask Anything About Go").
		WithAuthor("moderator").
		WithScore(890).
		Build()

	// Create top-level comments
	comment1 := testutil.NewCommentBuilder().
		WithID("c1").
		WithBody("What's the best way to handle errors in Go?").
		WithAuthor("learner").
		WithScore(45).
		WithParentPost("abc123").
		Build()

	// Create a nested reply
	reply := testutil.NewCommentBuilder().
		WithID("r1").
		WithBody("Check out the errors package for wrapping errors!").
		WithAuthor("expert").
		WithScore(78).
		WithLinkID("t3_abc123").
		WithParentID("t1_c1").
		Build()

	// Add the reply to comment1
	comment1.Replies = []*types.Comment{reply}

	comment2 := testutil.NewCommentBuilder().
		WithID("c2").
		WithBody("How do goroutines work under the hood?").
		WithAuthor("curious").
		WithScore(32).
		WithParentPost("abc123").
		Build()

	// Create an account
	account := testutil.NewAccount("testuser").
		WithLinkKarma(10000).
		WithCommentKarma(5000).
		WithGold(true).
		Build()

	// Step 2: Configure and start the mock server
	server := testutil.NewMockServer().
		WithSubreddit("golang", sub).
		WithPosts("golang", "hot", post1, post2).
		WithPosts("golang", "new", post2, post1). // Different order for "new"
		WithComments("golang", "abc123", mainPost, comment1, comment2).
		WithAccount(account).
		WithError("/r/private", http.StatusForbidden, "Private subreddit").
		Start()
	defer server.Close()

	// Step 3: Use the mock server with your Reddit client
	httpClient := &http.Client{Timeout: 10 * time.Second}
	internalClient, _ := internal.NewClient(httpClient, server.URL, "test/1.0", nil)

	// Create a minimal test client structure (normally you'd use the full Reddit client)
	parser := internal.NewParser()
	ctx := context.Background()

	// Test 1: Fetch hot posts
	req, _ := internalClient.NewRequest(ctx, "GET", "/r/golang/hot", nil)
	var hotListing types.Thing
	_ = internalClient.Do(req, &hotListing)
	hotPosts, _ := parser.ExtractPosts(ctx, &hotListing)
	fmt.Printf("Hot posts: %d\n", len(hotPosts))
	fmt.Printf("First post: %s (score: %d)\n", hotPosts[0].Title, hotPosts[0].Score)

	// Test 2: Fetch subreddit info
	req, _ = internalClient.NewRequest(ctx, "GET", "/r/golang/about", nil)
	var subThing types.Thing
	_ = internalClient.Do(req, &subThing)
	subreddit, _ := parser.ParseSubreddit(ctx, &subThing)
	fmt.Printf("Subreddit: %s (%d subscribers)\n", subreddit.DisplayName, subreddit.Subscribers)

	// Test 3: Fetch comments
	req, _ = internalClient.NewRequest(ctx, "GET", "/r/golang/comments/abc123", nil)
	commentThings, _ := internalClient.DoThingArray(req)
	commentsResp, _ := parser.ExtractPostAndComments(ctx, commentThings)
	fmt.Printf("Post: %s\n", commentsResp.Post.Title)
	fmt.Printf("Comments: %d\n", len(commentsResp.Comments))
	fmt.Printf("First comment: %s\n", commentsResp.Comments[0].Body)
	fmt.Printf("Comment has replies: %d\n", len(commentsResp.Comments[0].Replies))

	// Output:
	// Hot posts: 2
	// First post: Introduction to Go Concurrency (score: 1500)
	// Subreddit: golang (500000 subscribers)
	// Post: Ask Anything About Go
	// Comments: 2
	// First comment: What's the best way to handle errors in Go?
	// Comment has replies: 1
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
	resp, _ := http.Get(server.URL + "/r/private/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Try to access the banned subreddit
	resp, _ = http.Get(server.URL + "/r/banned/hot")
	fmt.Printf("Status: %d\n", resp.StatusCode)
	resp.Body.Close()

	// Try to access a non-configured subreddit (returns empty listing)
	resp, _ = http.Get(server.URL + "/r/unconfigured/hot")
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
		resp, _ := http.Get(server.URL + "/r/golang" + endpoint)
		fmt.Printf("%s - Status: %d\n", endpoint, resp.StatusCode)
		resp.Body.Close()
	}

	// Output:
	// /hot - Status: 200
	// /new - Status: 200
	// /top - Status: 200
}
