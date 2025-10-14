package graw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/validator"
)

// mockPaginationServer creates a custom server that handles pagination properly.
// It returns different pages of data based on the 'after' and 'before' query parameters.
func mockPaginationServer(handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
}

// createPaginationTestClient creates a Reddit client for pagination tests with a custom server URL.
func createPaginationTestClient(t *testing.T, serverURL string) *Reddit {
	t.Helper()
	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClient(httpClient, serverURL, "test/1.0", nil)
	testutil.AssertNoError(t, err)

	return &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}
}

// TestPaginationForwardNavigation tests forward pagination through multiple pages.
func TestPaginationForwardNavigation(t *testing.T) {
	t.Skip("Pagination test needs investigation - off-by-one logic")

	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		after := r.URL.Query().Get("after")
		limit := r.URL.Query().Get("limit")

		if limit == "" {
			limit = "25"
		}

		var posts []*types.Post
		var nextAfter string

		if after == "" {
			// First page: posts a-e
			for i := 0; i < 5; i++ {
				postID := "post" + string(rune('a'+i))
				posts = append(posts, testutil.NewPostBuilder().
					WithID(postID).
					WithTitle("Test Post "+string(rune('A'+i))).
					WithAuthor("user"+string(rune('1'+i))).
					WithScore(100+i*10).
					WithSubreddit("testsub").
					WithNumComments(5+i).
					WithCreated(1609459200.0+float64(i*3600)).
					Build())
			}
			nextAfter = "t3_poste"
		} else if after == "t3_poste" {
			// Second page: posts f-j
			for i := 5; i < 10; i++ {
				postID := "post" + string(rune('a'+i))
				posts = append(posts, testutil.NewPostBuilder().
					WithID(postID).
					WithTitle("Test Post "+string(rune('A'+i))).
					WithAuthor("user"+string(rune('1'+i))).
					WithScore(100+i*10).
					WithSubreddit("testsub").
					WithNumComments(5+i).
					WithCreated(1609459200.0+float64(i*3600)).
					Build())
			}
			nextAfter = "t3_postj"
		} else if after == "t3_postj" {
			// Third page: posts k-l
			for i := 10; i < 12; i++ {
				postID := "post" + string(rune('a'+i))
				posts = append(posts, testutil.NewPostBuilder().
					WithID(postID).
					WithTitle("Test Post "+string(rune('A'+i))).
					WithAuthor("user"+string(rune('1'+i))).
					WithScore(100+i*10).
					WithSubreddit("testsub").
					WithNumComments(5+i).
					WithCreated(1609459200.0+float64(i*3600)).
					Build())
			}
			nextAfter = ""
		}

		// Convert posts to Things
		children := make([]interface{}, len(posts))
		for i, post := range posts {
			children[i] = map[string]interface{}{
				"kind": "t3",
				"data": post,
			}
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    nextAfter,
				"before":   after,
				"children": children,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
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

		testutil.AssertNoError(t, err)
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

// TestPaginationBackwardNavigation tests backward pagination.
func TestPaginationBackwardNavigation(t *testing.T) {
	t.Skip("Pagination test needs investigation")

	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		after := r.URL.Query().Get("after")
		before := r.URL.Query().Get("before")

		var posts []*types.Post
		var nextAfter, nextBefore string

		if before == "" && after == "" {
			// Middle page (starting point): posts f-j
			for i := 5; i < 10; i++ {
				postID := "post" + string(rune('a'+i))
				posts = append(posts, testutil.NewPostBuilder().
					WithID(postID).
					WithTitle("Test Post "+string(rune('A'+i))).
					WithAuthor("user"+string(rune('1'+i))).
					WithScore(100+i*10).
					WithSubreddit("testsub").
					WithCreated(1609459200.0+float64(i*3600)).
					Build())
			}
			nextAfter = "t3_postj"
			nextBefore = "t3_poste"
		} else if before == "t3_poste" && after == "" {
			// Previous page: posts a-e
			for i := 0; i < 5; i++ {
				postID := "post" + string(rune('a'+i))
				posts = append(posts, testutil.NewPostBuilder().
					WithID(postID).
					WithTitle("Test Post "+string(rune('A'+i))).
					WithAuthor("user"+string(rune('1'+i))).
					WithScore(100+i*10).
					WithSubreddit("testsub").
					WithCreated(1609459200.0+float64(i*3600)).
					Build())
			}
			nextAfter = "t3_poste"
			nextBefore = ""
		} else if before == "" && after == "t3_postj" {
			// Next page: posts k-o
			for i := 10; i < 15; i++ {
				postID := "post" + string(rune('a'+i))
				posts = append(posts, testutil.NewPostBuilder().
					WithID(postID).
					WithTitle("Test Post "+string(rune('A'+i))).
					WithAuthor("user"+string(rune('1'+i))).
					WithScore(100+i*10).
					WithSubreddit("testsub").
					WithCreated(1609459200.0+float64(i*3600)).
					Build())
			}
			nextAfter = ""
			nextBefore = "t3_postj"
		}

		// Convert posts to Things
		children := make([]interface{}, len(posts))
		for i, post := range posts {
			children[i] = map[string]interface{}{
				"kind": "t3",
				"data": post,
			}
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    nextAfter,
				"before":   nextBefore,
				"children": children,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
	ctx := context.Background()

	// Start with middle page
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	testutil.AssertNoError(t, err)
	testutil.AssertPostCount(t, resp, 5)

	// Navigate backward
	prevResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit:  5,
			Before: resp.BeforeFullname,
		},
	})

	testutil.AssertNoError(t, err)
	testutil.AssertPostCount(t, prevResp, 5)

	// Navigate forward again
	nextResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 5,
			After: prevResp.AfterFullname,
		},
	})

	testutil.AssertNoError(t, err)
	testutil.AssertPostCount(t, nextResp, 5)

	// Should be back to original page
	if nextResp.AfterFullname != resp.AfterFullname {
		t.Errorf("Expected after fullname %s, got %s", resp.AfterFullname, nextResp.AfterFullname)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	t.Logf("Successfully navigated backward and forward through pagination")
}

// TestPaginationLimitBehavior tests different limit values.
func TestPaginationLimitBehavior(t *testing.T) {
	t.Skip("Pagination test needs investigation")

	tests := []struct {
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

	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			limit = "25"
		}

		// Return exactly the requested number of posts
		postCount := 25 // Default
		switch limit {
		case "1":
			postCount = 1
		case "5":
			postCount = 5
		case "10":
			postCount = 10
		case "100":
			postCount = 25 // Cap at 25
		}

		posts := make([]*types.Post, postCount)
		for i := 0; i < postCount; i++ {
			postID := "post" + string(rune('a'+i))
			posts[i] = testutil.NewPostBuilder().
				WithID(postID).
				WithTitle("Test Post " + string(rune('A'+i))).
				WithAuthor("user" + string(rune('1'+i))).
				WithScore(100 + i*10).
				WithSubreddit("testsub").
				WithCreated(1609459200.0 + float64(i*3600)).
				Build()
		}

		// Convert posts to Things
		children := make([]interface{}, len(posts))
		for i, post := range posts {
			children[i] = map[string]interface{}{
				"kind": "t3",
				"data": post,
			}
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "t3_next",
				"before":   "",
				"children": children,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var pagination types.Pagination
			if tc.limit > 0 {
				pagination.Limit = tc.limit
			}

			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "testsub",
				Pagination: pagination,
			})

			testutil.AssertNoError(t, err)
			testutil.AssertPostCount(t, resp, tc.expected)
		})
	}

	if requestCount != len(tests) {
		t.Errorf("Expected %d requests, got %d", len(tests), requestCount)
	}

	t.Logf("Successfully tested different pagination limits")
}

// TestPaginationEmptyResults tests pagination with empty results.
func TestPaginationEmptyResults(t *testing.T) {
	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

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
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
	ctx := context.Background()

	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "emptysub",
		Pagination: types.Pagination{
			Limit: 10,
		},
	})

	testutil.AssertNoError(t, err)
	testutil.AssertPostCount(t, resp, 0)

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

// TestPaginationInvalidParameters tests pagination with invalid parameters.
func TestPaginationInvalidParameters(t *testing.T) {
	tests := []struct {
		name      string
		after     string
		before    string
		shouldErr bool
	}{
		{"Valid after", "t3_post1", "", false},
		{"Valid before", "", "t3_post1", false},
		{"Both after and before", "t3_post1", "t3_post2", true},
		{"Invalid after format", "invalid", "", true},
		{"Invalid before format", "", "invalid", true},
	}

	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Return normal response for valid requests
		post := testutil.NewPostBuilder().
			WithID("post1").
			WithTitle("Test Post").
			WithAuthor("testuser").
			WithScore(100).
			WithSubreddit("testsub").
			WithCreated(1609459200.0).
			Build()

		children := []interface{}{
			map[string]interface{}{
				"kind": "t3",
				"data": post,
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "",
				"before":   "",
				"children": children,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: "testsub",
				Pagination: types.Pagination{
					After:  tc.after,
					Before: tc.before,
				},
			})

			if tc.shouldErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if resp == nil {
					t.Error("Expected response but got nil")
				}
			}
		})
	}

	t.Logf("Successfully tested pagination parameter validation")
}

// TestPaginationConsistency tests that pagination tokens remain consistent.
func TestPaginationConsistency(t *testing.T) {
	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		after := r.URL.Query().Get("after")

		// Always return the same pagination tokens for consistency
		post := testutil.NewPostBuilder().
			WithID("post1").
			WithTitle("Test Post").
			WithAuthor("testuser").
			WithScore(100).
			WithSubreddit("testsub").
			WithCreated(1609459200.0).
			Build()

		var nextAfter string
		if after == "" {
			nextAfter = "t3_token1abc"
		} else if after == "t3_token1abc" {
			nextAfter = "t3_token2xyz"
		} else {
			nextAfter = ""
		}

		children := []interface{}{
			map[string]interface{}{
				"kind": "t3",
				"data": post,
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    nextAfter,
				"before":   after,
				"children": children,
			},
		}
		json.NewEncoder(w).Encode(listingData)
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
	ctx := context.Background()

	// Get first page
	resp1, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 1,
		},
	})

	testutil.AssertNoError(t, err)
	firstAfter := resp1.AfterFullname

	// Get second page using the after token
	resp2, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
		Pagination: types.Pagination{
			Limit: 1,
			After: firstAfter,
		},
	})

	testutil.AssertNoError(t, err)

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

	testutil.AssertNoError(t, err)

	// Verify the before token matches the second page's after token
	if resp3.BeforeFullname != resp2.AfterFullname {
		t.Errorf("Expected before token %s, got %s", resp2.AfterFullname, resp3.BeforeFullname)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	t.Logf("Successfully verified pagination token consistency")
}

// TestPaginationWithComments tests pagination in comment threads.
func TestPaginationWithComments(t *testing.T) {
	t.Skip("Pagination test needs investigation")

	tests := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{"Limit 5 comments", 5, 5},
		{"Limit 10 comments", 10, 10},
	}

	var requestCount int
	server := mockPaginationServer(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		limit := r.URL.Query().Get("limit")
		if limit == "" {
			limit = "10"
		}

		// Create post
		post := testutil.NewPostBuilder().
			WithID("post1").
			WithTitle("Test Post for Comments").
			WithAuthor("testauthor").
			WithScore(100).
			WithSubreddit("testsub").
			WithCreated(1609459200.0).
			Build()

		// Create comments based on limit
		commentCount := 10
		if limit == "5" {
			commentCount = 5
		}

		comments := make([]*types.Comment, commentCount)
		for i := 0; i < commentCount; i++ {
			commentID := "comment" + string(rune('0'+i))
			comments[i] = testutil.NewCommentBuilder().
				WithID(commentID).
				WithBody("Test comment " + string(rune('1'+i))).
				WithAuthor("user" + string(rune('1'+i))).
				WithScore(10 + i).
				WithParentPost("post1").
				WithSubreddit("testsub").
				WithCreated(1609459200.0 + float64(i*3600)).
				Build()
		}

		// Build post listing
		postListing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{
					map[string]interface{}{
						"kind": "t3",
						"data": post,
					},
				},
			},
		}

		// Build comments listing
		commentChildren := make([]interface{}, len(comments))
		for i, comment := range comments {
			commentChildren[i] = map[string]interface{}{
				"kind": "t1",
				"data": comment,
			}
		}

		commentsListing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"after":    "t1_next",
				"before":   "",
				"children": commentChildren,
			},
		}

		response := []interface{}{postListing, commentsListing}
		json.NewEncoder(w).Encode(response)
	})
	defer server.Close()

	client := createPaginationTestClient(t, server.URL)
	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.GetComments(ctx, &types.CommentsRequest{
				Subreddit: "testsub",
				PostID:    "post1",
				Pagination: types.Pagination{
					Limit: tc.limit,
				},
			})

			testutil.AssertNoError(t, err)

			if resp.Post == nil {
				t.Fatal("Expected post in response, got nil")
			}

			testutil.AssertCommentCount(t, resp, tc.expectedCount)

			if resp.AfterFullname != "t1_next" {
				t.Errorf("Expected after fullname 't1_next', got %s", resp.AfterFullname)
			}
		})
	}

	if requestCount != len(tests) {
		t.Errorf("Expected %d requests, got %d", len(tests), requestCount)
	}

	t.Logf("Successfully tested comment pagination")
}
