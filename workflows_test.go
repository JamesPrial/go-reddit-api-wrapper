package graw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestCompletePostBrowsingWorkflow tests the complete flow from subreddit discovery to post browsing
func TestCompletePostBrowsingWorkflow(t *testing.T) {
	// Mock server that simulates a complete subreddit browsing experience
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Set rate limit headers
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/r/golang/about.json"):
			// Subreddit info
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name":        "golang",
					"title":               "The Go Programming Language",
					"public_description":  "Go discussions",
					"subscribers":         500000,
					"active_user_count":   2500,
					"over18":              false,
					"user_is_banned":      false,
					"user_is_moderator":   false,
					"user_is_contributor": false,
					"user_is_muted":       false,
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(r.URL.Path, "/r/golang/hot.json"):
			// Hot posts with pagination
			after := r.URL.Query().Get("after")
			limit := r.URL.Query().Get("limit")

			if limit == "" {
				limit = "25"
			}

			posts := make([]map[string]interface{}, 0)
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
							"subreddit":    "golang",
							"created_utc":  1609459200.0 + float64(i*3600),
							"num_comments": 5 + i,
							"permalink":    "/r/golang/comments/post" + string(rune('a'+i)) + "/test_post_" + string(rune('A'+i)) + "/",
						},
					})
				}
			} else {
				// Second page
				for i := 6; i <= 8; i++ {
					posts = append(posts, map[string]interface{}{
						"kind": "t3",
						"data": map[string]interface{}{
							"id":           "post" + string(rune('a'+i)),
							"title":        "Test Post " + string(rune('A'+i)),
							"score":        100 + i*10,
							"author":       "user" + string(rune('1'+i)),
							"subreddit":    "golang",
							"created_utc":  1609459200.0 + float64(i*3600),
							"num_comments": 5 + i,
							"permalink":    "/r/golang/comments/post" + string(rune('a'+i)) + "/test_post_" + string(rune('A'+i)) + "/",
						},
					})
				}
			}

			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"after": func() string {
						if after == "" {
							return "t3_poste"
						} else {
							return ""
						}
					}(),
					"before":   after,
					"children": posts,
				},
			}
			json.NewEncoder(w).Encode(listingData)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	// Create client
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

	// Step 1: Get subreddit info
	t.Run("GetSubredditInfo", func(t *testing.T) {
		subreddit, err := client.GetSubreddit(ctx, "golang")
		if err != nil {
			t.Fatalf("Failed to get subreddit info: %v", err)
		}

		if subreddit.DisplayName != "golang" {
			t.Errorf("Expected display name 'golang', got '%s'", subreddit.DisplayName)
		}

		if subreddit.Subscribers != 500000 {
			t.Errorf("Expected 500000 subscribers, got %d", subreddit.Subscribers)
		}

		t.Logf("Successfully retrieved subreddit: %s (%d subscribers)", subreddit.DisplayName, subreddit.Subscribers)
	})

	// Step 2: Get first page of hot posts
	t.Run("GetFirstPage", func(t *testing.T) {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 5,
			},
		})

		if err != nil {
			t.Fatalf("Failed to get hot posts: %v", err)
		}

		if len(resp.Posts) != 5 {
			t.Errorf("Expected 5 posts, got %d", len(resp.Posts))
		}

		if resp.AfterFullname != "t3_poste" {
			t.Errorf("Expected after fullname 't3_poste', got '%s'", resp.AfterFullname)
		}

		// Verify post structure
		for i, post := range resp.Posts {
			expectedTitle := "Test Post " + string(rune('A'+i))
			if post.Title != expectedTitle {
				t.Errorf("Post %d: expected title '%s', got '%s'", i, expectedTitle, post.Title)
			}

			if post.Subreddit != "golang" {
				t.Errorf("Post %d: expected subreddit 'golang', got '%s'", i, post.Subreddit)
			}
		}

		t.Logf("Successfully retrieved first page: %d posts", len(resp.Posts))
	})

	// Step 3: Get second page using pagination
	t.Run("GetSecondPage", func(t *testing.T) {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 5,
				After: "t3_poste",
			},
		})

		if err != nil {
			t.Fatalf("Failed to get second page: %v", err)
		}

		if len(resp.Posts) != 3 {
			t.Errorf("Expected 3 posts on second page, got %d", len(resp.Posts))
		}

		if resp.AfterFullname != "" {
			t.Errorf("Expected empty after fullname on last page, got '%s'", resp.AfterFullname)
		}

		t.Logf("Successfully retrieved second page: %d posts", len(resp.Posts))
	})

	// Step 4: Verify workflow completion
	t.Run("WorkflowCompletion", func(t *testing.T) {
		if requestCount < 4 {
			t.Errorf("Expected at least 4 requests (subreddit + 3 post pages), got %d", requestCount)
		}

		t.Logf("Workflow completed successfully with %d requests", requestCount)
	})
}

// TestCommentTreeNavigationWorkflow tests the complete flow from post to comments to more comments
func TestCommentTreeNavigationWorkflow(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/r/golang/comments/post1.json"):
			// Post and initial comments
			postData := map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           "post1",
					"title":        "Test Post for Comments",
					"author":       "testuser",
					"subreddit":    "golang",
					"num_comments": 10,
					"permalink":    "/r/golang/comments/post1/test_post/",
				},
			}

			// Create a comment tree with some "more" comments
			comments := []map[string]interface{}{
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        "comment1",
						"author":    "user1",
						"body":      "This is a top-level comment",
						"score":     10,
						"link_id":   "t3_post1",
						"parent_id": "t3_post1",
						"replies": map[string]interface{}{
							"kind": "Listing",
							"data": map[string]interface{}{
								"children": []map[string]interface{}{
									{
										"kind": "t1",
										"data": map[string]interface{}{
											"id":        "comment2",
											"author":    "user2",
											"body":      "This is a reply",
											"score":     5,
											"link_id":   "t3_post1",
											"parent_id": "t1_comment1",
											"replies":   map[string]interface{}{"kind": "Listing", "data": map[string]interface{}{"children": []interface{}{}}},
										},
									},
								},
							},
						},
					},
				},
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        "comment3",
						"author":    "user3",
						"body":      "Another top-level comment",
						"score":     8,
						"link_id":   "t3_post1",
						"parent_id": "t3_post1",
						"replies": map[string]interface{}{
							"kind": "Listing",
							"data": map[string]interface{}{
								"children": []map[string]interface{}{
									{
										"kind": "more",
										"data": map[string]interface{}{
											"count":    5,
											"children": []string{"comment4", "comment5", "comment6", "comment7", "comment8"},
										},
									},
								},
							},
						},
					},
				},
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
					"children": comments,
				},
			}

			response := []interface{}{postListing, commentsListing}
			json.NewEncoder(w).Encode(response)

		case strings.Contains(r.URL.Path, "/api/morechildren.json"):
			// More comments endpoint
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			commentIDs := r.Form["children"]
			linkID := r.Form.Get("link_id")

			if linkID != "t3_post1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Return the requested more comments
			things := make([]map[string]interface{}, 0)
			for _, id := range commentIDs {
				things = append(things, map[string]interface{}{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        id,
						"author":    "user" + id[len(id)-1:],
						"body":      "This is a more comment: " + id,
						"score":     3,
						"link_id":   "t3_post1",
						"parent_id": "t1_comment3",
						"replies":   map[string]interface{}{"kind": "Listing", "data": map[string]interface{}{"children": []interface{}{}}},
					},
				})
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"json": map[string]interface{}{
					"data": map[string]interface{}{
						"things": things,
					},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	// Create client
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

	// Step 1: Get initial comments
	t.Run("GetInitialComments", func(t *testing.T) {
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    "post1",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		if err != nil {
			t.Fatalf("Failed to get comments: %v", err)
		}

		if commentsResp.Post == nil {
			t.Fatal("Expected post in response, got nil")
		}

		if commentsResp.Post.Title != "Test Post for Comments" {
			t.Errorf("Expected post title 'Test Post for Comments', got '%s'", commentsResp.Post.Title)
		}

		if len(commentsResp.Comments) != 2 {
			t.Errorf("Expected 2 top-level comments, got %d", len(commentsResp.Comments))
		}

		if len(commentsResp.MoreIDs) != 5 {
			t.Errorf("Expected 5 more comment IDs, got %d", len(commentsResp.MoreIDs))
		}

		// Verify comment tree structure
		if len(commentsResp.Comments[0].Replies) != 1 {
			t.Errorf("Expected 1 reply to first comment, got %d", len(commentsResp.Comments[0].Replies))
		}

		t.Logf("Successfully retrieved %d comments with %d more IDs", len(commentsResp.Comments), len(commentsResp.MoreIDs))
	})

	// Step 2: Get more comments
	t.Run("GetMoreComments", func(t *testing.T) {
		moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
			LinkID:     "post1",
			CommentIDs: []string{"comment4", "comment5", "comment6"},
			Sort:       "confidence",
		})

		if err != nil {
			t.Fatalf("Failed to get more comments: %v", err)
		}

		if len(moreComments) != 3 {
			t.Errorf("Expected 3 more comments, got %d", len(moreComments))
		}

		// Verify more comment structure
		for i, comment := range moreComments {
			expectedID := []string{"comment4", "comment5", "comment6"}[i]
			if comment.ID != expectedID {
				t.Errorf("Comment %d: expected ID '%s', got '%s'", i, expectedID, comment.ID)
			}

			if comment.ParentID != "t1_comment3" {
				t.Errorf("Comment %d: expected parent ID 't1_comment3', got '%s'", i, comment.ParentID)
			}
		}

		t.Logf("Successfully retrieved %d more comments", len(moreComments))
	})

	// Step 3: Verify workflow completion
	t.Run("WorkflowCompletion", func(t *testing.T) {
		if requestCount < 2 {
			t.Errorf("Expected at least 2 requests (initial comments + more comments), got %d", requestCount)
		}

		t.Logf("Comment navigation workflow completed successfully with %d requests", requestCount)
	})
}

// TestSubredditDiscoveryWorkflow tests discovering and exploring subreddits
func TestSubredditDiscoveryWorkflow(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/r/golang/about.json"):
			// First subreddit
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name":       "golang",
					"title":              "The Go Programming Language",
					"public_description": "Go discussions and news",
					"subscribers":        500000,
					"active_user_count":  2500,
					"over18":             false,
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(r.URL.Path, "/r/golang/hot.json"):
			// Hot posts from golang
			posts := []map[string]interface{}{
				{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":        "post1",
						"title":     "Go 1.20 Released",
						"subreddit": "golang",
						"score":     1500,
					},
				},
			}
			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": posts,
				},
			}
			json.NewEncoder(w).Encode(listingData)

		case strings.Contains(r.URL.Path, "/r/rust/about.json"):
			// Second subreddit
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name":       "rust",
					"title":              "Rust Programming Language",
					"public_description": "Rust discussions and questions",
					"subscribers":        300000,
					"active_user_count":  1800,
					"over18":             false,
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(r.URL.Path, "/r/rust/hot.json"):
			// Hot posts from rust
			posts := []map[string]interface{}{
				{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":        "post2",
						"title":     "Rust 2023 Roadmap",
						"subreddit": "rust",
						"score":     800,
					},
				},
			}
			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": posts,
				},
			}
			json.NewEncoder(w).Encode(listingData)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	// Create client
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
	subreddits := []string{"golang", "rust"}

	// Discover each subreddit
	for _, subredditName := range subreddits {
		t.Run("Discover_"+subredditName, func(t *testing.T) {
			// Step 1: Get subreddit info
			subreddit, err := client.GetSubreddit(ctx, subredditName)
			if err != nil {
				t.Fatalf("Failed to get subreddit info for %s: %v", subredditName, err)
			}

			if subreddit.DisplayName != subredditName {
				t.Errorf("Expected display name '%s', got '%s'", subredditName, subreddit.DisplayName)
			}

			t.Logf("Discovered subreddit: %s (%d subscribers)", subreddit.DisplayName, subreddit.Subscribers)

			// Step 2: Get hot posts to verify it's active
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: subredditName,
				Pagination: types.Pagination{
					Limit: 5,
				},
			})

			if err != nil {
				t.Fatalf("Failed to get hot posts for %s: %v", subredditName, err)
			}

			if len(resp.Posts) == 0 {
				t.Errorf("Expected at least 1 post in %s, got 0", subredditName)
			}

			// Verify posts belong to the correct subreddit
			for _, post := range resp.Posts {
				if post.Subreddit != subredditName {
					t.Errorf("Expected post from %s, got post from %s", subredditName, post.Subreddit)
				}
			}

			t.Logf("Verified %s is active with %d hot posts", subredditName, len(resp.Posts))
		})
	}

	// Verify workflow completion
	t.Run("WorkflowCompletion", func(t *testing.T) {
		expectedRequests := len(subreddits) * 2 // subreddit info + hot posts for each
		if requestCount < expectedRequests {
			t.Errorf("Expected at least %d requests, got %d", expectedRequests, requestCount)
		}

		t.Logf("Subreddit discovery workflow completed successfully with %d requests", requestCount)
	})
}

// TestUserActivityWorkflow tests user-related workflows
func TestUserActivityWorkflow(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/api/v1/me"):
			// Current user info
			userData := map[string]interface{}{
				"id":                 "user123",
				"name":               "testuser",
				"link_karma":         5000,
				"comment_karma":      3000,
				"created_utc":        1609459200.0,
				"verified":           true,
				"has_verified_email": true,
			}
			json.NewEncoder(w).Encode(userData)

		case strings.Contains(r.URL.Path, "/user/testuser/submitted.json"):
			// User's posts
			posts := []map[string]interface{}{
				{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":        "userpost1",
						"title":     "My Go Project",
						"author":    "testuser",
						"subreddit": "golang",
						"score":     50,
					},
				},
				{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":        "userpost2",
						"title":     "Rust vs Go",
						"author":    "testuser",
						"subreddit": "rust",
						"score":     25,
					},
				},
			}
			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": posts,
				},
			}
			json.NewEncoder(w).Encode(listingData)

		case strings.Contains(r.URL.Path, "/user/testuser/comments.json"):
			// User's comments
			comments := []map[string]interface{}{
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        "usercomment1",
						"body":      "Great explanation!",
						"author":    "testuser",
						"subreddit": "golang",
						"score":     10,
					},
				},
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        "usercomment2",
						"body":      "I disagree with this approach",
						"author":    "testuser",
						"subreddit": "programming",
						"score":     5,
					},
				},
			}
			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": comments,
				},
			}
			json.NewEncoder(w).Encode(listingData)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	// Create client
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

	// Step 1: Get current user info
	t.Run("GetUserInfo", func(t *testing.T) {
		account, err := client.Me(ctx)
		if err != nil {
			t.Fatalf("Failed to get user info: %v", err)
		}

		if account.Name != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", account.Name)
		}

		if account.LinkKarma != 5000 {
			t.Errorf("Expected 5000 link karma, got %d", account.LinkKarma)
		}

		t.Logf("Retrieved user info: %s (%d link karma, %d comment karma)",
			account.Name, account.LinkKarma, account.CommentKarma)
	})

	// Step 2: Get user's posts
	t.Run("GetUserPosts", func(t *testing.T) {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "testuser",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		if err != nil {
			t.Fatalf("Failed to get user posts: %v", err)
		}

		if len(resp.Posts) != 2 {
			t.Errorf("Expected 2 user posts, got %d", len(resp.Posts))
		}

		// Verify all posts belong to the user
		for _, post := range resp.Posts {
			if post.Author != "testuser" {
				t.Errorf("Expected post author 'testuser', got '%s'", post.Author)
			}
		}

		t.Logf("Retrieved %d user posts", len(resp.Posts))
	})

	// Step 3: Get user's comments (simulated by getting comments from user's subreddit)
	t.Run("GetUserComments", func(t *testing.T) {
		// Note: In a real implementation, you might have a specific method for user comments
		// For this test, we simulate it by getting comments from the user's "subreddit"
		_, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "testuser",
			PostID:    "userpost1",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		if err != nil {
			t.Fatalf("Failed to get user comments: %v", err)
		}

		// In a real scenario, you'd verify the comments belong to the user
		t.Logf("Retrieved user comments (simulated)")
	})

	// Step 4: Verify workflow completion
	t.Run("WorkflowCompletion", func(t *testing.T) {
		if requestCount < 3 {
			t.Errorf("Expected at least 3 requests (user info + posts + comments), got %d", requestCount)
		}

		t.Logf("User activity workflow completed successfully with %d requests", requestCount)
	})
}

// TestMoreCommentsIntegrationWorkflow tests the complete more comments flow
func TestMoreCommentsIntegrationWorkflow(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/r/golang/comments/post1.json"):
			// Post with many comments and "more" placeholders
			postData := map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           "post1",
					"title":        "Post with Many Comments",
					"author":       "testuser",
					"subreddit":    "golang",
					"num_comments": 100,
				},
			}

			// Create comments with multiple "more" placeholders
			comments := []map[string]interface{}{
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        "comment1",
						"author":    "user1",
						"body":      "First comment",
						"score":     10,
						"link_id":   "t3_post1",
						"parent_id": "t3_post1",
						"replies": map[string]interface{}{
							"kind": "Listing",
							"data": map[string]interface{}{
								"children": []map[string]interface{}{
									{
										"kind": "more",
										"data": map[string]interface{}{
											"count": 20,
											"children": func() []string {
												ids := make([]string, 20)
												for i := 0; i < 20; i++ {
													ids[i] = "comment" + string(rune('a'+i+2))
												}
												return ids
											}(),
										},
									},
								},
							},
						},
					},
				},
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        "comment2",
						"author":    "user2",
						"body":      "Second comment",
						"score":     8,
						"link_id":   "t3_post1",
						"parent_id": "t3_post1",
						"replies": map[string]interface{}{
							"kind": "Listing",
							"data": map[string]interface{}{
								"children": []map[string]interface{}{
									{
										"kind": "more",
										"data": map[string]interface{}{
											"count": 30,
											"children": func() []string {
												ids := make([]string, 30)
												for i := 0; i < 30; i++ {
													ids[i] = "comment" + string(rune('a'+i+22))
												}
												return ids
											}(),
										},
									},
								},
							},
						},
					},
				},
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
					"children": comments,
				},
			}

			response := []interface{}{postListing, commentsListing}
			json.NewEncoder(w).Encode(response)

		case strings.Contains(r.URL.Path, "/api/morechildren.json"):
			// More comments endpoint
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			commentIDs := r.Form["children"]
			linkID := r.Form.Get("link_id")

			if linkID != "t3_post1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Return the requested more comments
			things := make([]map[string]interface{}, 0)
			for _, id := range commentIDs {
				things = append(things, map[string]interface{}{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":        id,
						"author":    "user" + id[len(id)-1:],
						"body":      "More comment content: " + id,
						"score":     3,
						"link_id":   "t3_post1",
						"parent_id": "t1_comment1",
						"replies":   map[string]interface{}{"kind": "Listing", "data": map[string]interface{}{"children": []interface{}{}}},
					},
				})
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"json": map[string]interface{}{
					"data": map[string]interface{}{
						"things": things,
					},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	// Create client
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

	var commentsResp *types.CommentsResponse

	// Step 1: Get initial comments with many "more" placeholders
	t.Run("GetInitialCommentsWithManyMore", func(t *testing.T) {
		var err error
		commentsResp, err = client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    "post1",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		if err != nil {
			t.Fatalf("Failed to get comments: %v", err)
		}

		if len(commentsResp.Comments) != 2 {
			t.Errorf("Expected 2 top-level comments, got %d", len(commentsResp.Comments))
		}

		if len(commentsResp.MoreIDs) != 50 {
			t.Errorf("Expected 50 more comment IDs, got %d", len(commentsResp.MoreIDs))
		}

		t.Logf("Retrieved %d comments with %d more comment IDs",
			len(commentsResp.Comments), len(commentsResp.MoreIDs))
	})

	// Step 2: Get first batch of more comments
	t.Run("GetFirstBatchOfMoreComments", func(t *testing.T) {
		// Get first 10 more comments
		firstBatch := commentsResp.MoreIDs[:10]
		moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
			LinkID:     "post1",
			CommentIDs: firstBatch,
			Sort:       "confidence",
		})

		if err != nil {
			t.Fatalf("Failed to get first batch of more comments: %v", err)
		}

		if len(moreComments) != 10 {
			t.Errorf("Expected 10 more comments, got %d", len(moreComments))
		}

		t.Logf("Retrieved first batch: %d more comments", len(moreComments))
	})

	// Step 3: Get second batch of more comments
	t.Run("GetSecondBatchOfMoreComments", func(t *testing.T) {
		// Get next 10 more comments
		secondBatch := commentsResp.MoreIDs[10:20]
		moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
			LinkID:     "post1",
			CommentIDs: secondBatch,
			Sort:       "confidence",
		})

		if err != nil {
			t.Fatalf("Failed to get second batch of more comments: %v", err)
		}

		if len(moreComments) != 10 {
			t.Errorf("Expected 10 more comments, got %d", len(moreComments))
		}

		t.Logf("Retrieved second batch: %d more comments", len(moreComments))
	})

	// Step 4: Test LimitChildren behavior
	t.Run("TestLimitChildrenBehavior", func(t *testing.T) {
		// Get more comments with LimitChildren=true
		remainingBatch := commentsResp.MoreIDs[20:25]
		moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
			LinkID:        "post1",
			CommentIDs:    remainingBatch,
			Sort:          "confidence",
			LimitChildren: true,
		})

		if err != nil {
			t.Fatalf("Failed to get more comments with LimitChildren: %v", err)
		}

		if len(moreComments) != 5 {
			t.Errorf("Expected 5 more comments, got %d", len(moreComments))
		}

		t.Logf("Retrieved with LimitChildren=true: %d more comments", len(moreComments))
	})

	// Step 5: Verify workflow completion
	t.Run("WorkflowCompletion", func(t *testing.T) {
		expectedRequests := 1 + 3 // initial comments + 3 more comments requests
		if requestCount < expectedRequests {
			t.Errorf("Expected at least %d requests, got %d", expectedRequests, requestCount)
		}

		t.Logf("More comments integration workflow completed successfully with %d requests", requestCount)
	})
}
