package graw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestPaginationForwardNavigation tests forward pagination through multiple pages
func TestPaginationForwardNavigation(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		after := r.URL.Query().Get("after")
		limit := r.URL.Query().Get("limit")

		if limit == "" {
			limit = "25"
		}

		posts := make([]map[string]interface{}, 0)
		var nextAfter string

		if after == "" {
			// First page
			for i := 1; i <= 5; i++ {
				posts = append(posts, map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":           "post" + string(rune('a'+i)),
						"title":        "Test Post " + string(rune('A'+i)),
						"score":        100 + i*10,
						"author":       "user" + string(rune('1'+i)),
						"subreddit":    "testsub",
						"created_utc":  1609459200.0 + float64(i*3600),
						"num_comments": 5 + i,
					},
				})
			}
			nextAfter = "t3_poste"
		} else if after == "t3_poste" {
			// Second page
			for i := 6; i <= 10; i++ {
				posts = append(posts, map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":           "post" + string(rune('a'+i)),
						"title":        "Test Post " + string(rune('A'+i)),
						"score":        100 + i*10,
						"author":       "user" + string(rune('1'+i)),
						"subreddit":    "testsub",
						"created_utc":  1609459200.0 + float64(i*3600),
						"num_comments": 5 + i,
					},
				})
			}
			nextAfter = "t3_postj"
		} else if after == "t3_postj" {
			// Third page (last)
			for i := 11; i <= 12; i++ {
				posts = append(posts, map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":           "post" + string(rune('a'+i)),
						"title":        "Test Post " + string(rune('A'+i)),
						"score":        100 + i*10,
						"author":       "user" + string(rune('1'+i)),
						"subreddit":    "testsub",
						"created_utc":  1609459200.0 + float64(i*3600),
						"num_comments": 5 + i,
					},
				})
			}
			nextAfter = ""
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    nextAfter,
				"before":   after,
				"children": posts,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()
	var allPosts []map[string]interface{}
	currentAfter := ""

	// Navigate through all pages
	for {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "testsub",
			Pagination: types.Pagination{
				Limit: 5,
				After: currentAfter,
			},
		})

		if err != nil {
			t.Fatalf("Failed to get posts: %v", err)
		}

		if len(resp.Posts) == 0 {
			t.Error("Expected posts but got empty response")
			break
		}

		// Collect posts
		for _, post := range resp.Posts {
			allPosts = append(allPosts, map[string]interface{}{
				"id":    post.ID,
				"title": post.Title,
				"score": post.Score,
			})
		}

		// Check if we've reached the end
		if resp.AfterFullname == "" {
			break
		}

		currentAfter = resp.AfterFullname
	}

	// Verify we got all expected posts
	if len(allPosts) != 12 {
		t.Errorf("Expected 12 posts total, got %d", len(allPosts))
	}

	// Verify pagination order
	expectedOrder := []string{"posta", "postb", "postc", "postd", "poste", "postf", "postg", "posth", "posti", "postj", "postk", "postl"}
	for i, post := range allPosts {
		if post["id"] != expectedOrder[i] {
			t.Errorf("Post %d: expected ID %s, got %s", i, expectedOrder[i], post["id"])
		}
	}

	// Verify request count (3 pages)
	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	t.Logf("Successfully paginated through %d posts in %d requests", len(allPosts), requestCount)
}

// TestPaginationBackwardNavigation tests backward pagination
func TestPaginationBackwardNavigation(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		after := r.URL.Query().Get("after")
		before := r.URL.Query().Get("before")

		posts := make([]map[string]interface{}, 0)
		var nextAfter, nextBefore string

		if before == "" && after == "" {
			// Middle page (starting point)
			for i := 6; i <= 10; i++ {
				posts = append(posts, map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":     "post" + string(rune('a'+i)),
						"title":  "Test Post " + string(rune('A'+i)),
						"score":  100 + i*10,
						"author": "user" + string(rune('1'+i)),
					},
				})
			}
			nextAfter = "t3_postj"
			nextBefore = "t3_poste"
		} else if before == "t3_poste" && after == "" {
			// Previous page
			for i := 1; i <= 5; i++ {
				posts = append(posts, map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":     "post" + string(rune('a'+i)),
						"title":  "Test Post " + string(rune('A'+i)),
						"score":  100 + i*10,
						"author": "user" + string(rune('1'+i)),
					},
				})
			}
			nextAfter = "t3_poste"
			nextBefore = ""
		} else if before == "" && after == "t3_postj" {
			// Next page
			for i := 11; i <= 15; i++ {
				posts = append(posts, map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":     "post" + string(rune('a'+i)),
						"title":  "Test Post " + string(rune('A'+i)),
						"score":  100 + i*10,
						"author": "user" + string(rune('1'+i)),
					},
				})
			}
			nextAfter = ""
			nextBefore = "t3_postj"
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    nextAfter,
				"before":   nextBefore,
				"children": posts,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Start with middle page
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get initial posts: %v", err)
	}

	if len(resp.Posts) != 5 {
		t.Errorf("Expected 5 posts on initial page, got %d", len(resp.Posts))
	}

	// Navigate backward
	prevResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit:  5,
			Before: resp.BeforeFullname,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get previous posts: %v", err)
	}

	if len(prevResp.Posts) != 5 {
		t.Errorf("Expected 5 posts on previous page, got %d", len(prevResp.Posts))
	}

	// Navigate forward again
	nextResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 5,
			After: prevResp.AfterFullname,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get next posts: %v", err)
	}

	// Should be back to original page
	if len(nextResp.Posts) != 5 {
		t.Errorf("Expected 5 posts on returned page, got %d", len(nextResp.Posts))
	}

	// Verify we got the same posts as the initial request
	if nextResp.AfterFullname != resp.AfterFullname {
		t.Errorf("Expected after fullname %s, got %s", resp.AfterFullname, nextResp.AfterFullname)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	t.Logf("Successfully navigated backward and forward through pagination")
}

// TestPaginationLimitBehavior tests different limit values
func TestPaginationLimitBehavior(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			limit = "25" // Default
		}

		// Return exactly the requested number of posts
		postCount := 0
		if limit == "1" {
			postCount = 1
		} else if limit == "5" {
			postCount = 5
		} else if limit == "10" {
			postCount = 10
		} else if limit == "100" {
			postCount = 25 // Cap at 25 for this test
		} else {
			postCount = 25 // Default
		}

		posts := make([]map[string]interface{}, postCount)
		for i := 0; i < postCount; i++ {
			posts[i] = map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":     "post" + string(rune('a'+i)),
					"title":  "Test Post " + string(rune('A'+i)),
					"score":  100 + i*10,
					"author": "user" + string(rune('1'+i)),
				},
			}
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "t3_next",
				"before":   "",
				"children": posts,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	testCases := []struct {
		name     string
		limit    int
		expected int
	}{
		{"Limit 1", 1, 1},
		{"Limit 5", 5, 5},
		{"Limit 10", 10, 10},
		{"Limit 25", 25, 25},
		{"Limit 100 (capped)", 100, 25},
		{"No limit (default)", 0, 25},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pagination types.Pagination
			if tc.limit > 0 {
				pagination.Limit = tc.limit
			}

			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "testsub",
				Pagination: pagination,
			})

			if err != nil {
				t.Fatalf("Failed to get posts: %v", err)
			}

			if len(resp.Posts) != tc.expected {
				t.Errorf("Expected %d posts, got %d", tc.expected, len(resp.Posts))
			}
		})
	}

	if requestCount != len(testCases) {
		t.Errorf("Expected %d requests, got %d", len(testCases), requestCount)
	}

	t.Logf("Successfully tested different pagination limits")
}

// TestPaginationEmptyResults tests pagination with empty results
func TestPaginationEmptyResults(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		// Return empty listing
		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "",
				"before":   "",
				"children": []interface{}{},
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "emptysub",
		Pagination: types.Pagination{
			Limit: 10,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get posts: %v", err)
	}

	if len(resp.Posts) != 0 {
		t.Errorf("Expected 0 posts, got %d", len(resp.Posts))
	}

	if resp.AfterFullname != "" {
		t.Errorf("Expected empty after fullname, got %s", resp.AfterFullname)
	}

	if resp.BeforeFullname != "" {
		t.Errorf("Expected empty before fullname, got %s", resp.BeforeFullname)
	}

	if requestCount != 1 {
		t.Errorf("Expected 1 request, got %d", requestCount)
	}

	t.Logf("Successfully handled empty pagination results")
}

// TestPaginationInvalidParameters tests pagination with invalid parameters
func TestPaginationInvalidParameters(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		// Return normal response for valid requests
		posts := []map[string]interface{}{
			{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":     "post1",
					"title":  "Test Post",
					"score":  100,
					"author": "testuser",
				},
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "",
				"before":   "",
				"children": posts,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	testCases := []struct {
		name      string
		after     string
		before    string
		shouldErr bool
	}{
		{"Valid after", "t3_post1", "", false},
		{"Valid before", "", "t3_post1", false},
		{"Both after and before", "t3_post1", "t3_post2", true},
		{"Invalid after format", "invalid", "", false},  // Server should handle this
		{"Invalid before format", "", "invalid", false}, // Server should handle this
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: "testsub",
				Pagination: types.Pagination{
					After:  tc.after,
					Before: tc.before,
				},
			})

			if tc.shouldErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("Expected response but got nil")
				}
			}
		})
	}

	t.Logf("Successfully tested pagination parameter validation")
}

// TestPaginationConsistency tests that pagination tokens remain consistent
func TestPaginationConsistency(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		after := r.URL.Query().Get("after")

		// Always return the same pagination tokens for consistency
		posts := []map[string]interface{}{
			{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":     "post1",
					"title":  "Test Post",
					"score":  100,
					"author": "testuser",
				},
			},
		}

		var nextAfter string
		if after == "" {
			nextAfter = "t3_consistent_token"
		} else if after == "t3_consistent_token" {
			nextAfter = "t3_next_consistent_token"
		} else {
			nextAfter = ""
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    nextAfter,
				"before":   after,
				"children": posts,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Get first page
	resp1, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 1,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}

	firstAfter := resp1.AfterFullname

	// Get second page using the after token
	resp2, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 1,
			After: firstAfter,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get second page: %v", err)
	}

	// Verify the before token matches our after token
	if resp2.BeforeFullname != firstAfter {
		t.Errorf("Expected before token %s, got %s", firstAfter, resp2.BeforeFullname)
	}

	// Get third page
	resp3, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 1,
			After: resp2.AfterFullname,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get third page: %v", err)
	}

	// Verify the before token matches the second page's after token
	if resp3.BeforeFullname != resp2.AfterFullname {
		t.Errorf("Expected before token %s, got %s", resp2.AfterFullname, resp3.BeforeFullname)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	t.Logf("Successfully verified pagination token consistency")
}

// TestPaginationWithComments tests pagination in comment threads
func TestPaginationWithComments(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			limit = "10"
		}

		// Create nested comment structure
		postData := map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":    "post1",
				"title": "Test Post for Comments",
				"score": 100,
			},
		}

		comments := make([]map[string]interface{}, 0)
		commentCount := 0
		if limit == "5" {
			commentCount = 5
		} else {
			commentCount = 10
		}

		for i := 0; i < commentCount; i++ {
			comments = append(comments, map[string]interface{}{
				"kind": "t1",
				"data": map[string]interface{}{
					"id":        "comment" + string(rune('1'+i)),
					"body":      "Test comment " + string(rune('1'+i)),
					"score":     10 + i,
					"author":    "user" + string(rune('1'+i)),
					"link_id":   "t3_post1",
					"parent_id": "t3_post1",
					"replies":   map[string]interface{}{"kind": "Listing", "data": map[string]interface{}{"children": []interface{}{}}},
				},
			})
		}

		postListing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{postData},
			},
		}

		commentsListing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "t1_next",
				"before":   "",
				"children": comments,
			},
		}

		response := []interface{}{postListing, commentsListing}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test comment pagination
	resp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "testsub",
		PostID:    "post1",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get comments: %v", err)
	}

	if resp.Post == nil {
		t.Fatal("Expected post in response, got nil")
	}

	if len(resp.Comments) != 5 {
		t.Errorf("Expected 5 comments, got %d", len(resp.Comments))
	}

	if resp.AfterFullname != "t1_next" {
		t.Errorf("Expected after fullname 't1_next', got %s", resp.AfterFullname)
	}

	// Test with different limit
	resp2, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "testsub",
		PostID:    "post1",
		Pagination: types.Pagination{
			Limit: 10,
		},
	})

	if err != nil {
		t.Fatalf("Failed to get comments with limit 10: %v", err)
	}

	if len(resp2.Comments) != 10 {
		t.Errorf("Expected 10 comments, got %d", len(resp2.Comments))
	}

	if requestCount != 2 {
		t.Errorf("Expected 2 requests, got %d", requestCount)
	}

	t.Logf("Successfully tested comment pagination")
}
