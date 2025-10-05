package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestConcurrentClientUsage tests multiple clients using the API simultaneously
func TestConcurrentClientUsage(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/r/testsubreddit/about") || strings.Contains(r.URL.Path, "r/testsubreddit/about"):
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"id":           "testsub1",
					"name":         "t5_testsub1",
					"display_name": "testsubreddit",
					"subscribers":  100000,
					"created_utc":  1234567890.0,
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(r.URL.Path, "/r/testsubreddit/hot") || strings.Contains(r.URL.Path, "r/testsubreddit/hot"):
			response := map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"after":  "",
					"before": "",
					"children": []map[string]interface{}{
						{
							"kind": "t3",
							"data": map[string]interface{}{
								"id":          "post1",
								"name":        "t3_post1",
								"title":       "Test Post",
								"score":       100,
								"author":      "testuser",
								"subreddit":   "testsubreddit",
								"permalink":   "/r/testsubreddit/comments/post1/test_post/",
								"url":         "https://reddit.com/r/testsubreddit/comments/post1",
								"created_utc": 1234567890.0,
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	// Create multiple clients
	numClients := 5
	clients := make([]*Reddit, numClients)

	for i := 0; i < numClients; i++ {
		httpClient := &http.Client{Timeout: 30 * time.Second}
		internalClient, err := internal.NewClient(httpClient, server.URL, fmt.Sprintf("test_agent_%d/1.0", i), nil)
		if err != nil {
			t.Fatalf("Failed to create internal client %d: %v", i, err)
		}

		clients[i] = &Reddit{
			httpClient:    internalClient,
			parser:    internal.NewParser(),
			validator: internal.NewValidator(),
			auth:      &mockTokenProvider{token: "test_token"},
		}
	}

	// Test concurrent operations
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	// Each client performs multiple operations
	for clientIdx, client := range clients {
		wg.Add(1)
		go func(idx int, c *Reddit) {
			defer wg.Done()

			// Perform subreddit discovery
			sr, err := c.GetSubreddit(context.Background(), "testsubreddit")
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("client %d subreddit error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if sr.DisplayName != "testsubreddit" {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("client %d unexpected subreddit name: %s", idx, sr.DisplayName))
				errorMu.Unlock()
			}

			// Perform post operations
			posts, err := c.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "testsubreddit",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("client %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) != 1 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("client %d expected 1 post, got %d", idx, len(posts.Posts)))
				errorMu.Unlock()
				return
			}
		}(clientIdx, client)
	}

	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}

	// Verify all requests were handled
	if atomic.LoadInt64(&requestCount) < int64(numClients*2) {
		t.Errorf("Expected at least %d requests, got %d", numClients*2, atomic.LoadInt64(&requestCount))
	}
}

// TestConcurrentSameClientOperations tests a single client used concurrently
func TestConcurrentSameClientOperations(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		defer mu.Unlock()

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/r/concurrent_test/about.json"):
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": "concurrent_test",
					"subscribers":  100000,
					"created_utc":  1234567890.0,
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(r.URL.Path, "/r/concurrent_test/hot.json"):
			posts := []map[string]interface{}{
				{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":     "concurrent_post",
						"title":  "Concurrent Post",
						"score":  100,
						"author": "testuser",
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

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "concurrent_test_agent/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Test concurrent operations on the same client
	numGoroutines := 10
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Alternate between subreddit and post operations
			if goroutineID%2 == 0 {
				sr, err := client.GetSubreddit(context.Background(), "concurrent_test")
				if err != nil {
					errorMu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d subreddit error: %v", goroutineID, err))
					errorMu.Unlock()
					return
				}

				if sr.DisplayName == "" {
					errorMu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d empty subreddit name", goroutineID))
					errorMu.Unlock()
				}
			} else {
				posts, err := client.GetHot(context.Background(), &types.PostsRequest{
					Subreddit: "concurrent_test",
					Pagination: types.Pagination{
						Limit: 5,
					},
				})
				if err != nil {
					errorMu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", goroutineID, err))
					errorMu.Unlock()
					return
				}

				if len(posts.Posts) == 0 {
					errorMu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d no posts returned", goroutineID))
					errorMu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}

	// Verify requests were handled
	if atomic.LoadInt64(&requestCount) == 0 {
		t.Error("No requests were processed")
	}
}

// TestConcurrentRateLimitingBehavior tests rate limiting behavior under concurrent load
func TestConcurrentRateLimitingBehavior(t *testing.T) {
	var requestCount int64
	var rateLimitHits int64
	var mu sync.Mutex
	lastRequestTime := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		currentTime := time.Now()
		timeSinceLastRequest := currentTime.Sub(lastRequestTime)
		lastRequestTime = currentTime

		// Simulate rate limiting - if requests come too quickly, return 429
		if timeSinceLastRequest < 50*time.Millisecond && atomic.LoadInt64(&requestCount) > 1 {
			atomic.AddInt64(&rateLimitHits, 1)
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Second).Unix()))
			w.WriteHeader(http.StatusTooManyRequests)
			mu.Unlock()
			return
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining", "100")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()))
		w.WriteHeader(http.StatusOK)

		subredditData := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name": fmt.Sprintf("ratelimit_test_%d", atomic.LoadInt64(&requestCount)),
				"subscribers":  100000,
				"created_utc":  1234567890.0,
			},
		}
		json.NewEncoder(w).Encode(subredditData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "ratelimit_test_agent/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Test rapid concurrent requests
	numGoroutines := 20
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := client.GetSubreddit(context.Background(), "ratelimit_test")
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				// Some errors are expected due to rate limiting
				if !containsRateLimitError(err) {
					t.Errorf("Unexpected non-rate-limit error: %v", err)
				}
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Verify that rate limiting occurred
	if atomic.LoadInt64(&rateLimitHits) == 0 {
		t.Error("Expected some rate limit hits, but got none")
	}

	// Verify that some requests succeeded
	if atomic.LoadInt64(&successCount) == 0 {
		t.Error("Expected some successful requests, but got none")
	}

	t.Logf("Rate limit hits: %d, Successful requests: %d, Failed requests: %d",
		atomic.LoadInt64(&rateLimitHits), atomic.LoadInt64(&successCount), atomic.LoadInt64(&errorCount))
}

// TestConcurrentContextCancellation tests context cancellation in concurrent scenarios
func TestConcurrentContextCancellation(t *testing.T) {
	var requestCount int64
	var activeRequests int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		atomic.AddInt64(&activeRequests, 1)
		defer atomic.AddInt64(&activeRequests, -1)

		// Simulate slow response
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		subredditData := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name": fmt.Sprintf("cancellation_test_%d", atomic.LoadInt64(&requestCount)),
				"subscribers":  100000,
				"created_utc":  1234567890.0,
			},
		}
		json.NewEncoder(w).Encode(subredditData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "cancellation_test_agent/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Test context cancellation
	numGoroutines := 10
	var wg sync.WaitGroup
	var cancelledCount int64
	var completedCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Create a context that cancels after 50ms
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			_, err := client.GetSubreddit(ctx, "cancellation_test")
			if err != nil {
				if err == context.DeadlineExceeded {
					atomic.AddInt64(&cancelledCount, 1)
				} else {
					t.Errorf("Goroutine %d unexpected error: %v", goroutineID, err)
				}
			} else {
				atomic.AddInt64(&completedCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// Most requests should be cancelled due to timeout
	if atomic.LoadInt64(&cancelledCount) == 0 {
		t.Error("Expected some requests to be cancelled")
	}

	// Wait a bit for any remaining requests to complete
	time.Sleep(300 * time.Millisecond)

	// Verify no requests are still active
	if atomic.LoadInt64(&activeRequests) > 0 {
		t.Errorf("Expected no active requests, but %d are still active", atomic.LoadInt64(&activeRequests))
	}

	t.Logf("Cancelled requests: %d, Completed requests: %d",
		atomic.LoadInt64(&cancelledCount), atomic.LoadInt64(&completedCount))
}

// TestConcurrentResourceContention tests resource contention under concurrent load
func TestConcurrentResourceContention(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		defer mu.Unlock()

		// Simulate resource contention with variable response times
		time.Sleep(time.Duration(atomic.LoadInt64(&requestCount)%10) * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		subredditData := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"id":           fmt.Sprintf("sub%d", atomic.LoadInt64(&requestCount)),
				"name":         fmt.Sprintf("t5_sub%d", atomic.LoadInt64(&requestCount)),
				"display_name": fmt.Sprintf("contention_test_%d", atomic.LoadInt64(&requestCount)),
				"subscribers":  100000,
				"created_utc":  1234567890.0,
			},
		}
		json.NewEncoder(w).Encode(subredditData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "contention_test_agent/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Test high concurrency with resource contention
	numGoroutines := 20
	operationsPerGoroutine := 3
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex
	var successCount int64

	startTime := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				_, err := client.GetSubreddit(context.Background(), "contention_test")
				if err != nil {
					errorMu.Lock()
					errors = append(errors, fmt.Errorf("goroutine %d operation %d: %v", goroutineID, j, err))
					errorMu.Unlock()
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}

	expectedOperations := int64(numGoroutines * operationsPerGoroutine)
	if atomic.LoadInt64(&successCount) != expectedOperations {
		t.Errorf("Expected %d successful operations, got %d", expectedOperations, atomic.LoadInt64(&successCount))
	}

	t.Logf("Completed %d operations in %v (%.2f ops/sec)",
		expectedOperations, duration, float64(expectedOperations)/duration.Seconds())

	// Verify all requests were processed
	if atomic.LoadInt64(&requestCount) != expectedOperations {
		t.Errorf("Expected %d requests, got %d", expectedOperations, atomic.LoadInt64(&requestCount))
	}
}

// TestConcurrentMixedOperations tests different types of operations running concurrently
func TestConcurrentMixedOperations(t *testing.T) {
	var requestCount int64
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch {
		case strings.Contains(r.URL.Path, "/r/mixed_test_sub/about.json"):
			subredditData := map[string]interface{}{
				"kind": "t5",
				"data": map[string]interface{}{
					"display_name": fmt.Sprintf("mixed_test_sub_%d", atomic.LoadInt64(&requestCount)),
					"subscribers":  100000,
					"created_utc":  1234567890.0,
				},
			}
			json.NewEncoder(w).Encode(subredditData)

		case strings.Contains(r.URL.Path, "/r/mixed_test_sub/hot.json"):
			posts := []map[string]interface{}{
				{
					"kind": "t3",
					"data": map[string]interface{}{
						"id":           fmt.Sprintf("mixed_post_%d", atomic.LoadInt64(&requestCount)),
						"title":        fmt.Sprintf("Mixed Post %d", atomic.LoadInt64(&requestCount)),
						"score":        100,
						"num_comments": 50,
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

		case strings.Contains(r.URL.Path, "/r/mixed_test_sub/comments/mixed_post_1.json"):
			postData := map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":    "mixed_post_1",
					"title": "Mixed Post 1",
					"score": 100,
				},
			}

			comments := []map[string]interface{}{
				{
					"kind": "t1",
					"data": map[string]interface{}{
						"id":    fmt.Sprintf("mixed_comment_%d", atomic.LoadInt64(&requestCount)),
						"body":  fmt.Sprintf("Mixed Comment %d", atomic.LoadInt64(&requestCount)),
						"score": 10,
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

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "mixed_operations_test_agent/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	// Test mixed operations concurrently
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex
	var operationCounts struct {
		subreddit int64
		posts     int64
		comments  int64
	}

	// Subreddit operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.GetSubreddit(context.Background(), "mixed_test_sub")
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("subreddit error: %v", err))
				errorMu.Unlock()
			} else {
				atomic.AddInt64(&operationCounts.subreddit, 1)
			}
		}()
	}

	// Posts operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "mixed_test_sub",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("posts error: %v", err))
				errorMu.Unlock()
			} else {
				atomic.AddInt64(&operationCounts.posts, 1)
			}
		}()
	}

	// Comments operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.GetComments(context.Background(), &types.CommentsRequest{
				Subreddit: "mixed_test_sub",
				PostID:    "mixed_post_1",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("comments error: %v", err))
				errorMu.Unlock()
			} else {
				atomic.AddInt64(&operationCounts.comments, 1)
			}
		}()
	}

	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}

	// Verify all operation types were successful
	if atomic.LoadInt64(&operationCounts.subreddit) != 5 {
		t.Errorf("Expected 5 subreddit operations, got %d", atomic.LoadInt64(&operationCounts.subreddit))
	}
	if atomic.LoadInt64(&operationCounts.posts) != 5 {
		t.Errorf("Expected 5 posts operations, got %d", atomic.LoadInt64(&operationCounts.posts))
	}
	if atomic.LoadInt64(&operationCounts.comments) != 5 {
		t.Errorf("Expected 5 comments operations, got %d", atomic.LoadInt64(&operationCounts.comments))
	}

	t.Logf("Operation counts - Subreddits: %d, Posts: %d, Comments: %d",
		atomic.LoadInt64(&operationCounts.subreddit),
		atomic.LoadInt64(&operationCounts.posts),
		atomic.LoadInt64(&operationCounts.comments))
}

// Helper function to check if error contains rate limit information
func containsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "429") || contains(errStr, "rate limit") || contains(errStr, "too many requests")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
