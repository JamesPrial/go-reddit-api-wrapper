package graw

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/validator"
)

// TestRedditAPIClientUsage tests real-world Reddit API client usage patterns
func TestRedditAPIClientUsage(t *testing.T) {
	// Create test data using builders
	subreddit := testutil.NewSubreddit("testsub").
		WithTitle("Test Subreddit").
		WithDescription("A test subreddit for real-world scenarios").
		WithSubscribers(100000).
		Build()

	hotPosts := make([]*types.Post, 10)
	for i := 0; i < 10; i++ {
		hotPosts[i] = testutil.NewPostBuilder().
			WithID(fmt.Sprintf("abc%d", i)).
			WithTitle(fmt.Sprintf("Real World Test Post %d", i)).
			WithScore(100 + i*10).
			WithAuthor(fmt.Sprintf("user_%d", i)).
			WithSubreddit("testsub").
			WithNumComments(10 + i).
			Build()
	}

	newPosts := make([]*types.Post, 10)
	for i := 0; i < 10; i++ {
		newPosts[i] = testutil.NewPostBuilder().
			WithID(fmt.Sprintf("new%d", i)).
			WithTitle(fmt.Sprintf("New Post %d", i)).
			WithScore(50 + i*5).
			WithAuthor(fmt.Sprintf("user_%d", i)).
			WithSubreddit("testsub").
			Build()
	}

	commentsPost := testutil.NewPostBuilder().
		WithID("abc123").
		WithTitle("Main Post for Comments").
		WithScore(1000).
		WithAuthor("mainuser").
		WithSubreddit("testsub").
		Build()

	comments := make([]*types.Comment, 25)
	for i := 0; i < 25; i++ {
		comments[i] = testutil.NewCommentBuilder().
			WithID(fmt.Sprintf("comment%d", i)).
			WithBody(fmt.Sprintf("This is comment %d with some realistic content that would appear in a real Reddit thread.", i)).
			WithAuthor(fmt.Sprintf("commenter_%d", i)).
			WithScore(5 + i).
			WithParentPost("abc123").
			WithSubreddit("testsub").
			Build()
	}

	// Setup mock server
	server := testutil.NewMockServer().
		WithSubreddit("testsub", subreddit).
		WithPosts("testsub", "hot", hotPosts...).
		WithPosts("testsub", "new", newPosts...).
		WithComments("testsub", "abc123", commentsPost, comments...).
		Start()
	defer server.Close()

	httpClient := server.Server().Client()
	httpClient.Timeout = 30 * time.Second
	internalClient, err := client.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Scenario 1: Subreddit analysis workflow
	t.Run("SubredditAnalysis", func(t *testing.T) {
		// Get subreddit information
		sub, err := client.GetSubreddit(ctx, "testsub")
		testutil.AssertNoError(t, err)

		if sub.DisplayName != "testsub" {
			t.Errorf("Expected 'testsub', got: %s", sub.DisplayName)
		}

		if sub.Subscribers != 100000 {
			t.Errorf("Expected 100000 subscribers, got: %d", sub.Subscribers)
		}

		// Get hot posts
		hotResp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "testsub",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})
		testutil.AssertNoError(t, err)
		testutil.AssertPostCount(t, hotResp, 10)

		// Get new posts
		newResp, err := client.GetNew(ctx, &types.PostsRequest{
			Subreddit: "testsub",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})
		testutil.AssertNoError(t, err)
		testutil.AssertPostCount(t, newResp, 10)

		// Analyze posts
		totalScore := 0
		for _, post := range hotResp.Posts {
			totalScore += post.Score
		}

		avgScore := float64(totalScore) / float64(len(hotResp.Posts))
		t.Logf("Subreddit analysis completed:")
		t.Logf("  Subreddit: %s", sub.DisplayName)
		t.Logf("  Subscribers: %d", sub.Subscribers)
		t.Logf("  Hot posts analyzed: %d", len(hotResp.Posts))
		t.Logf("  Average score: %.2f", avgScore)
	})

	// Scenario 2: Post and comment thread analysis
	t.Run("PostCommentAnalysis", func(t *testing.T) {
		// Get comments for a post
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "testsub",
			PostID:    "abc123",
			Pagination: types.Pagination{
				Limit: 25,
			},
		})
		testutil.AssertNoError(t, err)

		if commentsResp.Post == nil {
			t.Error("Expected post in comments response, got nil")
		}

		testutil.AssertCommentCount(t, commentsResp, 25)

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
			testutil.AssertNoError(t, err)

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
	t.Logf("  All scenarios executed successfully")
}

// TestErrorHandlingInRealWorld tests error handling in realistic scenarios
func TestErrorHandlingInRealWorld(t *testing.T) {
	var requestCount atomic.Int32

	// Create custom handler that simulates various error conditions
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)

		w.Header().Set("Content-Type", "application/json")

		// Simulate various error conditions based on request count
		switch {
		case count <= 2:
			// First 2 requests: rate limit error
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Second).Unix()))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Too Many Requests","message":"Rate limit exceeded"}`))

		case count <= 4:
			// Next 2 requests: server error
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"Internal Server Error","message":"Simulated server error"}`))

		case count <= 6:
			// Next 2 requests: timeout simulation
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"kind":"t5","data":{"id":"testsub123","display_name":"testsub","subscribers":100000,"created_utc":1234567890.0}}`))

		default:
			// Remaining requests: success
			w.Header().Set("X-RateLimit-Remaining", "59")
			w.Header().Set("X-RateLimit-Reset", "60")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"kind":"t5","data":{"id":"testsub123","display_name":"testsub","subscribers":100000,"created_utc":1234567890.0}}`))
		}
	}))
	defer server.Close()

	httpClient := server.Client()
	httpClient.Timeout = 500 * time.Millisecond
	internalClient, err := client.NewClient(httpClient, server.URL, "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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
	// Create test data
	subreddit := testutil.NewSubreddit("testsub").
		WithSubscribers(100000).
		WithDescription("Test subreddit").
		Build()

	posts := make([]*types.Post, 10)
	for i := 0; i < 10; i++ {
		posts[i] = testutil.NewPostBuilder().
			WithID(fmt.Sprintf("def%d", i)).
			WithTitle(fmt.Sprintf("Concurrent Test Post %d", i)).
			WithScore(100 + i).
			WithAuthor(fmt.Sprintf("user_%d", i)).
			WithSubreddit("testsub").
			Build()
	}

	// Setup mock server
	server := testutil.NewMockServer().
		WithSubreddit("testsub", subreddit).
		WithPosts("testsub", "hot", posts...).
		WithPosts("testsub", "new", posts...).
		Start()
	defer server.Close()

	httpClient := server.Server().Client()
	httpClient.Timeout = 30 * time.Second
	internalClient, err := client.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(userID)
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := numUsers * requestsPerUser
	finalSuccessCount := atomic.LoadInt64(&successCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)
	successRate := float64(finalSuccessCount) / float64(totalRequests) * 100
	requestsPerSecond := float64(totalRequests) / duration.Seconds()

	t.Logf("Concurrent real world usage test completed:")
	t.Logf("  Concurrent users: %d", numUsers)
	t.Logf("  Requests per user: %d", requestsPerUser)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful requests: %d", finalSuccessCount)
	t.Logf("  Failed requests: %d", finalErrorCount)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Total duration: %v", duration)
	t.Logf("  Requests per second: %.2f", requestsPerSecond)

	if successRate < 90 {
		t.Errorf("Success rate too low: %.2f%%", successRate)
	}
}

// TestLongRunningOperations tests long-running operations and resource management
func TestLongRunningOperations(t *testing.T) {
	// Create large dataset for long-running operations
	posts := make([]*types.Post, 100)
	for i := 0; i < 100; i++ {
		posts[i] = testutil.NewPostBuilder().
			WithID(fmt.Sprintf("xyz%d", i)).
			WithTitle(fmt.Sprintf("Long Running Test Post %d", i)).
			WithScore(100 + i).
			WithAuthor(fmt.Sprintf("user_%d", i)).
			WithSubreddit("testsub").
			WithNumComments(10 + i).
			Build()
	}

	// Setup mock server
	server := testutil.NewMockServer().
		WithPosts("testsub", "hot", posts...).
		Start()
	defer server.Close()

	httpClient := server.Server().Client()
	httpClient.Timeout = 30 * time.Second
	internalClient, err := client.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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
		testutil.AssertNoError(t, err)
		testutil.AssertPostCount(t, resp, postsPerPage)

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
}
