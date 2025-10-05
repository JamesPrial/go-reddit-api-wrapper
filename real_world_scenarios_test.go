package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestRedditAPIClientUsage tests real-world Reddit API client usage patterns
func TestRedditAPIClientUsage(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "59")
		w.Header().Set("X-Ratelimit-Reset", "60")

		path := r.URL.Path
		switch {
		case strings.Contains(path, "/r/"):
			// Subreddit endpoint
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name":       "testsub",
					"title":              "Test Subreddit",
					"public_description": "A test subreddit for real-world scenarios",
					"subscribers":        100000,
					"active_users":       5000,
					"created_utc":        1234567890.0,
					"over18":             false,
					"subscriber_growth":  []float64{1000, 2000, 3000, 4000, 5000},
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(path, "/hot/") || strings.Contains(path, "/new/") || strings.Contains(path, "/top/"):
			// Posts listing endpoint
			posts := make([]map[string]interface{}, 25)
			for i := 0; i < 25; i++ {
				posts[i] = map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":           fmt.Sprintf("t3_%d", i),
						"title":        fmt.Sprintf("Real World Test Post %d", i),
						"score":        100 + i*10,
						"author":       fmt.Sprintf("user_%d", i),
						"selftext":     fmt.Sprintf("This is test content for post %d with some realistic text to simulate real Reddit posts.", i),
						"url":          fmt.Sprintf("https://example.com/post_%d", i),
						"permalink":    fmt.Sprintf("/r/testsub/comments/%d/real_world_test_post_%d/", i, i),
						"created_utc":  1609459200.0 + float64(i*3600),
						"num_comments": 10 + i,
						"over_18":      false,
						"stickied":     i == 0,
						"gilded":       i%3 == 0,
						"thumbnail":    "https://example.com/thumb.jpg",
					},
				}
			}

			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": posts,
					"after":    "t3_next_page",
					"before":   "",
				},
			}
			json.NewEncoder(w).Encode(listingData)

		case strings.Contains(path, "/comments/"):
			// Comments endpoint
			postData := map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":       "t3_main_post",
					"title":    "Main Post for Comments",
					"score":    1000,
					"author":   "main_user",
					"selftext": "This is the main post content",
				},
			}

			comments := make([]map[string]interface{}, 50)
			for i := 0; i < 50; i++ {
				comments[i] = map[string]interface{}{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":          fmt.Sprintf("t1_%d", i),
						"author":      fmt.Sprintf("commenter_%d", i),
						"body":        fmt.Sprintf("This is comment %d with some realistic content that would appear in a real Reddit thread.", i),
						"score":       5 + i,
						"created_utc": 1609459200.0 + float64(i*60),
						"replies": map[string]interface{}{
							"kind": "Listing",
							"data": map[string]interface{}{
								"children": []interface{}{},
							},
						},
					},
				}
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
					"after":    "t1_next_page",
					"before":   "",
				},
			}

			response := []interface{}{postListing, commentsListing}
			json.NewEncoder(w).Encode(response)

		default:
			// Default response
			w.WriteHeader(http.StatusNotFound)
			errorData := map[string]interface{}{
				"error":   "Not Found",
				"message": "The requested resource was not found",
			}
			json.NewEncoder(w).Encode(errorData)
		}
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

	// Scenario 1: Subreddit analysis workflow
	t.Run("SubredditAnalysis", func(t *testing.T) {
		// Get subreddit information
		subreddit, err := client.GetSubreddit(ctx, "testsub")
		if err != nil {
			t.Fatalf("Failed to get subreddit: %v", err)
		}

		if subreddit.DisplayName != "testsub" {
			t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
		}

		if subreddit.Subscribers != 100000 {
			t.Errorf("Expected 100000 subscribers, got: %d", subreddit.Subscribers)
		}

		// Get hot posts
		hotResp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "testsub",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})
		if err != nil {
			t.Fatalf("Failed to get hot posts: %v", err)
		}

		if len(hotResp.Posts) != 10 {
			t.Errorf("Expected 10 hot posts, got: %d", len(hotResp.Posts))
		}

		// Get new posts
		newResp, err := client.GetNew(ctx, &types.PostsRequest{
			Subreddit: "testsub",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})
		if err != nil {
			t.Fatalf("Failed to get new posts: %v", err)
		}

		if len(newResp.Posts) != 10 {
			t.Errorf("Expected 10 new posts, got: %d", len(newResp.Posts))
		}

		// Analyze posts
		totalScore := 0
		for _, post := range hotResp.Posts {
			totalScore += post.Score
		}

		avgScore := float64(totalScore) / float64(len(hotResp.Posts))
		t.Logf("Subreddit analysis completed:")
		t.Logf("  Subreddit: %s", subreddit.DisplayName)
		t.Logf("  Subscribers: %d", subreddit.Subscribers)
		t.Logf("  Hot posts analyzed: %d", len(hotResp.Posts))
		t.Logf("  Average score: %.2f", avgScore)
	})

	// Scenario 2: Post and comment thread analysis
	t.Run("PostCommentAnalysis", func(t *testing.T) {
		// Get comments for a post
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "testsub",
			PostID:    "main_post",
			Pagination: types.Pagination{
				Limit: 25,
			},
		})
		if err != nil {
			t.Fatalf("Failed to get comments: %v", err)
		}

		if commentsResp.Post == nil {
			t.Error("Expected post in comments response, got nil")
		}

		if len(commentsResp.Comments) != 25 {
			t.Errorf("Expected 25 comments, got: %d", len(commentsResp.Comments))
		}

		// Analyze comments
		totalComments := len(commentsResp.Comments)
		totalCommentScore := 0
		uniqueAuthors := make(map[string]bool)

		for _, comment := range commentsResp.Comments {
			totalCommentScore += comment.Score
			uniqueAuthors[comment.Author] = true
		}

		avgCommentScore := float64(totalCommentScore) / float64(totalComments)
		t.Logf("Post and comment analysis completed:")
		t.Logf("  Post title: %s", commentsResp.Post.Title)
		t.Logf("  Total comments: %d", totalComments)
		t.Logf("  Average comment score: %.2f", avgCommentScore)
		t.Logf("  Unique commenters: %d", len(uniqueAuthors))
	})

	// Scenario 3: Pagination workflow
	t.Run("PaginationWorkflow", func(t *testing.T) {
		allPosts := make([]*types.Post, 0)
		currentAfter := ""
		pageCount := 0

		for {
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: "testsub",
				Pagination: types.Pagination{
					Limit: 10,
					After: currentAfter,
				},
			})
			if err != nil {
				t.Fatalf("Failed to get posts page %d: %v", pageCount+1, err)
			}

			if len(resp.Posts) == 0 {
				break
			}

			allPosts = append(allPosts, resp.Posts...)
			currentAfter = resp.AfterFullname
			pageCount++

			if currentAfter == "" {
				break
			}

			if pageCount >= 3 { // Limit for test
				break
			}
		}

		t.Logf("Pagination workflow completed:")
		t.Logf("  Pages fetched: %d", pageCount)
		t.Logf("  Total posts collected: %d", len(allPosts))
		t.Logf("  First post: %s", allPosts[0].Title)
		if len(allPosts) > 0 {
			t.Logf("  Last post: %s", allPosts[len(allPosts)-1].Title)
		}
	})

	t.Logf("Real-world scenarios test completed:")
	t.Logf("  Total requests made: %d", requestCount)
	t.Logf("  All scenarios executed successfully")
}

// TestErrorHandlingInRealWorld tests error handling in realistic scenarios
func TestErrorHandlingInRealWorld(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentRequest := requestCount
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Simulate various error conditions
		switch {
		case currentRequest <= 2:
			// First 2 requests: rate limit error
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Second).Unix()))
			w.WriteHeader(http.StatusTooManyRequests)
			errorData := map[string]interface{}{
				"error":   "Too Many Requests",
				"message": "Rate limit exceeded",
			}
			json.NewEncoder(w).Encode(errorData)

		case currentRequest <= 4:
			// Next 2 requests: server error
			w.WriteHeader(http.StatusInternalServerError)
			errorData := map[string]interface{}{
				"error":   "Internal Server Error",
				"message": "Simulated server error",
			}
			json.NewEncoder(w).Encode(errorData)

		case currentRequest <= 6:
			// Next 2 requests: timeout simulation
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			successData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(successData)

		default:
			// Remaining requests: success
			w.Header().Set("X-RateLimit-Remaining", "59")
			w.Header().Set("X-RateLimit-Reset", "60")
			w.WriteHeader(http.StatusOK)
			successData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "testsub",
					"subscribers":  100000,
				},
			}
			json.NewEncoder(w).Encode(successData)
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 500 * time.Millisecond}
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

	var successCount, errorCount int
	var errorTypes = make(map[string]int)

	// Make multiple requests to test error handling
	for i := 0; i < 8; i++ {
		_, err := client.GetSubreddit(ctx, "testsub")
		if err != nil {
			errorCount++
			errorMsg := err.Error()
			if strings.Contains(errorMsg, "429") || strings.Contains(errorMsg, "Too Many Requests") {
				errorTypes["rate_limit"]++
			} else if strings.Contains(errorMsg, "500") || strings.Contains(errorMsg, "Internal Server Error") {
				errorTypes["server_error"]++
			} else if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline") {
				errorTypes["timeout"]++
			} else {
				errorTypes["other"]++
			}
		} else {
			successCount++
		}

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Error handling in real world test completed:")
	t.Logf("  Total requests: %d", 8)
	t.Logf("  Successful requests: %d", successCount)
	t.Logf("  Failed requests: %d", errorCount)
	t.Logf("  Error types encountered:")
	for errType, count := range errorTypes {
		t.Logf("    %s: %d", errType, count)
	}

	// Verify we encountered different error types
	if len(errorTypes) < 2 {
		t.Errorf("Expected at least 2 different error types, got: %d", len(errorTypes))
	}

	if successCount == 0 {
		t.Error("Expected at least some successful requests")
	}
}

// TestConcurrentRealWorldUsage tests concurrent usage patterns
func TestConcurrentRealWorldUsage(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Simulate realistic response time
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "59")
		w.Header().Set("X-RateLimit-Reset", "60")

		path := r.URL.Path
		if strings.Contains(path, "/r/") {
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name":       "testsub",
					"subscribers":        100000,
					"public_description": "Test subreddit",
				},
			}
			json.NewEncoder(w).Encode(subredditData)
		} else {
			posts := make([]map[string]interface{}, 10)
			for i := 0; i < 10; i++ {
				posts[i] = map[string]interface{}{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":     fmt.Sprintf("t3_%d", i),
						"title":  fmt.Sprintf("Concurrent Test Post %d", i),
						"score":  100 + i,
						"author": fmt.Sprintf("user_%d", i),
					},
				}
			}

			listingData := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": posts,
					"after":    "",
					"before":   "",
				},
			}
			json.NewEncoder(w).Encode(listingData)
		}
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

	// Simulate multiple concurrent users/workflows
	const numUsers = 5
	const requestsPerUser = 3

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	start := time.Now()

	for userID := 0; userID < numUsers; userID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for req := 0; req < requestsPerUser; req++ {
				// Alternate between different API calls
				var err error
				switch req % 3 {
				case 0:
					_, err = client.GetSubreddit(ctx, "testsub")
				case 1:
					_, err = client.GetHot(ctx, &types.PostsRequest{
						Subreddit:  "testsub",
						Pagination: types.Pagination{Limit: 5},
					})
				case 2:
					_, err = client.GetNew(ctx, &types.PostsRequest{
						Subreddit:  "testsub",
						Pagination: types.Pagination{Limit: 5},
					})
				}

				if err != nil {
					errorCount++
				} else {
					successCount++
				}
			}
		}(userID)
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := numUsers * requestsPerUser
	successRate := float64(successCount) / float64(totalRequests) * 100
	requestsPerSecond := float64(totalRequests) / duration.Seconds()

	t.Logf("Concurrent real world usage test completed:")
	t.Logf("  Concurrent users: %d", numUsers)
	t.Logf("  Requests per user: %d", requestsPerUser)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful requests: %d", successCount)
	t.Logf("  Failed requests: %d", errorCount)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Total duration: %v", duration)
	t.Logf("  Requests per second: %.2f", requestsPerSecond)

	if successRate < 90 {
		t.Errorf("Success rate too low: %.2f%%", successRate)
	}

	if requestCount != totalRequests {
		t.Errorf("Expected %d requests, got %d", totalRequests, requestCount)
	}
}

// TestLongRunningOperations tests long-running operations and resource management
func TestLongRunningOperations(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "59")
		w.Header().Set("X-RateLimit-Reset", "60")

		// Return larger datasets for long-running operations
		posts := make([]map[string]interface{}, 100)
		for i := 0; i < 100; i++ {
			posts[i] = map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           fmt.Sprintf("t3_%d", i),
					"title":        fmt.Sprintf("Long Running Test Post %d", i),
					"score":        100 + i,
					"author":       fmt.Sprintf("user_%d", i),
					"selftext":     fmt.Sprintf("This is longer content for post %d to simulate real Reddit posts with substantial text content.", i),
					"created_utc":  1609459200.0 + float64(i*3600),
					"num_comments": 10 + i,
					"url":          fmt.Sprintf("https://reddit.com/r/testsub/comments/%d", i),
					"permalink":    fmt.Sprintf("/r/testsub/comments/%d/long_running_test_post_%d/", i, i),
				},
			}
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": posts,
				"after":    "t3_next_page",
				"before":   "",
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

	// Simulate a long-running data collection operation
	const numPages = 5
	const postsPerPage = 100

	start := time.Now()
	var totalPosts int
	var totalScore int64
	var uniqueAuthors = make(map[string]bool)

	for page := 0; page < numPages; page++ {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "testsub",
			Pagination: types.Pagination{
				Limit: postsPerPage,
			},
		})
		if err != nil {
			t.Fatalf("Failed to get page %d: %v", page+1, err)
		}

		if len(resp.Posts) != postsPerPage {
			t.Errorf("Expected %d posts on page %d, got %d", postsPerPage, page+1, len(resp.Posts))
		}

		// Process posts
		for _, post := range resp.Posts {
			totalPosts++
			totalScore += int64(post.Score)
			uniqueAuthors[post.Author] = true
		}

		// Simulate processing time
		time.Sleep(100 * time.Millisecond)
	}

	duration := time.Since(start)
	avgScore := float64(totalScore) / float64(totalPosts)
	postsPerSecond := float64(totalPosts) / duration.Seconds()

	t.Logf("Long running operations test completed:")
	t.Logf("  Pages processed: %d", numPages)
	t.Logf("  Total posts processed: %d", totalPosts)
	t.Logf("  Unique authors found: %d", len(uniqueAuthors))
	t.Logf("  Average post score: %.2f", avgScore)
	t.Logf("  Total duration: %v", duration)
	t.Logf("  Processing rate: %.2f posts/second", postsPerSecond)

	if totalPosts != numPages*postsPerPage {
		t.Errorf("Expected %d total posts, got %d", numPages*postsPerPage, totalPosts)
	}

	if requestCount != numPages {
		t.Errorf("Expected %d requests, got %d", numPages, requestCount)
	}
}
