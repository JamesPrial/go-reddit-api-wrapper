package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
)

func main() {
	// Get credentials from environment variables
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		// Use a simple test to check the parsing logic
		testParsingLogic()
		return
	}

	// Route structured logs to stdout with debug level
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create client configuration
	config := &graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "debug-bot/1.0",
		Logger:       logger,
	}

	// Create the client
	client, err := graw.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to Reddit
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to Reddit: %v", err)
	}

	fmt.Println("Successfully connected to Reddit!")

	// Get hot posts to test with
	hotPosts, err := client.GetHot(ctx, "golang", &graw.ListingOptions{Limit: 3})
	if err != nil || len(hotPosts.Posts) < 3 {
		log.Fatalf("Failed to get enough hot posts: %v", err)
	}

	fmt.Printf("\nTesting with these posts:\n")
	for i, post := range hotPosts.Posts[:3] {
		fmt.Printf("%d. ID=%s, Title=%.50s...\n", i+1, post.ID, post.Data.Title)
	}

	// Test 1: Single sequential requests
	fmt.Printf("\n=== TEST 1: Sequential Requests ===\n")
	for i, post := range hotPosts.Posts[:3] {
		resp, err := client.GetComments(ctx, "golang", post.ID, &graw.ListingOptions{Limit: 5})
		if err != nil {
			fmt.Printf("Post %d: ERROR: %v\n", i+1, err)
		} else {
			if resp.Post != nil {
				fmt.Printf("Post %d: ✓ Has post data: %.50s...\n", i+1, resp.Post.Data.Title)
			} else {
				fmt.Printf("Post %d: ✗ No post data returned\n", i+1)
			}
			fmt.Printf("         Comments: %d loaded\n", len(resp.Comments))
		}
	}

	// Test 2: Batch requests
	fmt.Printf("\n=== TEST 2: Batch/Parallel Requests ===\n")
	requests := []graw.CommentRequest{
		{Subreddit: "golang", PostID: hotPosts.Posts[0].ID, Options: &graw.ListingOptions{Limit: 5}},
		{Subreddit: "golang", PostID: hotPosts.Posts[1].ID, Options: &graw.ListingOptions{Limit: 5}},
		{Subreddit: "golang", PostID: hotPosts.Posts[2].ID, Options: &graw.ListingOptions{Limit: 5}},
	}

	batchResults, err := client.GetCommentsMultiple(ctx, requests)
	if err != nil {
		fmt.Printf("Batch loading error: %v\n", err)
	} else {
		for i, result := range batchResults {
			if result != nil {
				if result.Post != nil {
					fmt.Printf("Post %d: ✓ Has post data: %.50s...\n", i+1, result.Post.Data.Title)
				} else {
					fmt.Printf("Post %d: ✗ No post data returned\n", i+1)
				}
				fmt.Printf("         Comments: %d loaded\n", len(result.Comments))
			} else {
				fmt.Printf("Post %d: nil result\n", i+1)
			}
		}
	}
}

func testParsingLogic() {
	fmt.Println("No Reddit credentials found. Running parsing logic test...")

	// Simulate the response structure that Reddit typically returns
	// This mimics what we see when debugging
	sampleResponse := `[
		{
			"kind": "Listing",
			"data": {
				"children": []
			}
		},
		{
			"kind": "Listing",
			"data": {
				"children": [
					{
						"kind": "t1",
						"data": {
							"author": "testuser",
							"body": "Test comment"
						}
					}
				]
			}
		}
	]`

	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(sampleResponse), &result); err != nil {
		log.Fatalf("Failed to parse sample: %v", err)
	}

	fmt.Printf("Parsed structure:\n")
	fmt.Printf("- Number of listings: %d\n", len(result))
	if len(result) > 0 {
		fmt.Printf("- First listing kind: %v\n", result[0]["kind"])
		if data, ok := result[0]["data"].(map[string]interface{}); ok {
			if children, ok := data["children"].([]interface{}); ok {
				fmt.Printf("- First listing children count: %d\n", len(children))
			}
		}
	}
	if len(result) > 1 {
		fmt.Printf("- Second listing kind: %v\n", result[1]["kind"])
		if data, ok := result[1]["data"].(map[string]interface{}); ok {
			if children, ok := data["children"].([]interface{}); ok {
				fmt.Printf("- Second listing children count: %d\n", len(children))
			}
		}
	}
}