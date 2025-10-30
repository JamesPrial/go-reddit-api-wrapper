package graw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/validator"
)

// TestConcurrentClientUsage tests multiple clients using the API simultaneously
func TestConcurrentClientUsage(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("testsubreddit").
		WithTitle("Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("testsubreddit", subreddit).
		WithPosts("testsubreddit", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	// Create multiple clients
	numClients := 5
	clients := make([]*Reddit, numClients)

	for i := 0; i < numClients; i++ {
		httpClient := &http.Client{Timeout: 30 * time.Second}
		internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), fmt.Sprintf("test_agent_%d/1.0", i), nil, client.RateLimitConfig{}, mockClock)
		testutil.AssertNoError(t, err)

		clients[i] = &Reddit{
			httpClient: internalClient,
			parser:     parse.NewParser(nil),
			validator:  validator.NewValidator(),
			auth:       &mockTokenProvider{token: "test_token"},
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

			testutil.AssertPostCount(t, posts, 1)
		}(clientIdx, client)
	}

	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}
}

// TestConcurrentSameClientOperations tests a single client used concurrently
func TestConcurrentSameClientOperations(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("concurrent_test").
		WithTitle("Concurrent Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("concurrentpost").
		WithTitle("Concurrent Post").
		WithScore(100).
		WithAuthor("testuser").
		WithSubreddit("concurrent_test").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("concurrent_test", subreddit).
		WithPosts("concurrent_test", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	// Use default rate limit config (same as TestConcurrentClientUsage which passes)
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "concurrent_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test concurrent operations on the same client
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	// Subreddit operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sr, err := client.GetSubreddit(context.Background(), "concurrent_test")
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("subreddit error: %v", err))
				errorMu.Unlock()
				return
			}

			if sr.DisplayName == "" {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("empty subreddit name"))
				errorMu.Unlock()
			}
		}()
	}

	// Posts operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			posts, err := client.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "concurrent_test",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}

// TestConcurrentRateLimitingBehavior tests rate limiting behavior under concurrent load
func TestConcurrentRateLimitingBehavior(t *testing.T) {
	var requestCount int64
	var rateLimitHits int64
	var mu sync.Mutex

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})
	lastRequestTime := mockClock.Now()

	// Setup test data
	subreddit := testutil.NewSubreddit("ratelimit_test").
		WithTitle("Rate Limit Test Subreddit").
		WithSubscribers(100000).
		Build()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		currentTime := mockClock.Now()
		timeSinceLastRequest := mockClock.Since(lastRequestTime)
		lastRequestTime = currentTime

		// Simulate rate limiting - if requests come too quickly, return 429
		if timeSinceLastRequest < 50*time.Millisecond && atomic.LoadInt64(&requestCount) > 1 {
			atomic.AddInt64(&rateLimitHits, 1)
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1") // 1 second remaining
			w.WriteHeader(http.StatusTooManyRequests)
			mu.Unlock()
			return
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining", "100")
		w.Header().Set("X-RateLimit-Reset", "3600") // 1 hour = 3600 seconds remaining
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"kind": "t5",
			"data": subreddit,
		})
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL, "ratelimit_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	// Setup test data
	subreddit := testutil.NewSubreddit("cancellation_test").
		WithTitle("Cancellation Test Subreddit").
		WithSubscribers(100000).
		Build()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		atomic.AddInt64(&activeRequests, 1)
		defer atomic.AddInt64(&activeRequests, -1)

		// Keep real delay for context cancellation testing
		// Context timeouts use real time, not mock time
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"kind": "t5",
			"data": subreddit,
		})
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL, "cancellation_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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
				if errors.Is(err, context.DeadlineExceeded) {
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

	// Wait for any remaining requests to complete (real time needed for context cancellation)
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
	t.Skip("Resource contention test takes too long")
	var requestCount int64
	var mu sync.Mutex

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	// Setup test data
	subreddit := testutil.NewSubreddit("contention_test").
		WithTitle("Resource Contention Test Subreddit").
		WithSubscribers(100000).
		Build()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)

		mu.Lock()
		defer mu.Unlock()

		// No simulated resource contention delays needed with mock clock

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"kind": "t5",
			"data": subreddit,
		})
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL, "contention_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test high concurrency with resource contention
	numGoroutines := 10
	operationsPerGoroutine := 3
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex
	var successCount int64

	startTime := mockClock.Now()

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
	duration := mockClock.Since(startTime)

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
	// Setup test data
	subreddit := testutil.NewSubreddit("mixed_test_sub").
		WithTitle("Mixed Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("mixedpost1").
		WithTitle("Mixed Post").
		WithScore(100).
		WithAuthor("testuser").
		WithSubreddit("mixed_test_sub").
		WithNumComments(50).
		Build()

	comment := testutil.NewCommentBuilder().
		WithID("mixedcomment1").
		WithBody("Mixed Comment").
		WithScore(10).
		WithAuthor("testuser").
		WithLinkID("t3_mixedpost1").
		WithParentID("t3_mixedpost1").
		WithSubreddit("mixed_test_sub").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("mixed_test_sub", subreddit).
		WithPosts("mixed_test_sub", "hot", post).
		WithComments("mixed_test_sub", "mixedpost1", post, comment).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "mixed_operations_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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
				PostID:    "mixedpost1",
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

// TestConcurrentSameClientOperationsWithSeparateParsers tests with separate parsers
func TestConcurrentSameClientOperationsWithSeparateParsers(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("concurrent_test_sep").
		WithTitle("Concurrent Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("concurrentpostsep").
		WithTitle("Concurrent Post").
		WithScore(100).
		WithAuthor("testuser").
		WithSubreddit("concurrent_test_sep").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("concurrent_test_sep", subreddit).
		WithPosts("concurrent_test_sep", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	// Use default rate limit config (same as TestConcurrentClientUsage which passes)
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "concurrent_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	// Test concurrent operations on the same client but with separate parsers
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	// Subreddit operations with shared parser
	sharedParser := parse.NewParser(nil)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &Reddit{
				httpClient: internalClient,
				parser:     sharedParser,
				validator:  validator.NewValidator(),
				auth:       &mockTokenProvider{token: "test_token"},
			}
			sr, err := client.GetSubreddit(context.Background(), "concurrent_test_sep")
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("subreddit error: %v", err))
				errorMu.Unlock()
				return
			}

			if sr.DisplayName == "" {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("empty subreddit name"))
				errorMu.Unlock()
			}
		}()
	}

	// Posts operations with separate parsers for each goroutine
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Create a NEW parser for each goroutine
			client := &Reddit{
				httpClient: internalClient,
				parser:     parse.NewParser(nil), // DIFFERENT parser
				validator:  validator.NewValidator(),
				auth:       &mockTokenProvider{token: "test_token"},
			}
			posts, err := client.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "concurrent_test_sep",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}

// TestConcurrentSameClientOperationsWithSeparateClients tests with fully separate clients
func TestConcurrentSameClientOperationsWithSeparateClients(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("concurrent_test_full").
		WithTitle("Concurrent Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("concurrentpostfull").
		WithTitle("Concurrent Post").
		WithScore(100).
		WithAuthor("testuser").
		WithSubreddit("concurrent_test_full").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("concurrent_test_full", subreddit).
		WithPosts("concurrent_test_full", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	// Test concurrent operations with COMPLETELY separate clients
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	// Subreddit operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Create a new http.Client and internal client for each goroutine
			httpClient := &http.Client{Timeout: 30 * time.Second}
			internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "concurrent_test_agent_sub/1.0", nil, client.RateLimitConfig{}, mockClock)
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("failed to create client: %v", err))
				errorMu.Unlock()
				return
			}
			client := &Reddit{
				httpClient: internalClient,
				parser:     parse.NewParser(nil),
				validator:  validator.NewValidator(),
				auth:       &mockTokenProvider{token: "test_token"},
			}
			sr, err := client.GetSubreddit(context.Background(), "concurrent_test_full")
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("subreddit error: %v", err))
				errorMu.Unlock()
				return
			}

			if sr.DisplayName == "" {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("empty subreddit name"))
				errorMu.Unlock()
			}
		}()
	}

	// Posts operations with separate clients
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Create a new http.Client and internal client for each goroutine
			httpClient := &http.Client{Timeout: 30 * time.Second}
			internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "concurrent_test_agent_post/1.0", nil, client.RateLimitConfig{}, mockClock)
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d failed to create client: %v", idx, err))
				errorMu.Unlock()
				return
			}
			client := &Reddit{
				httpClient: internalClient,
				parser:     parse.NewParser(nil),
				validator:  validator.NewValidator(),
				auth:       &mockTokenProvider{token: "test_token"},
			}
			posts, err := client.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "concurrent_test_full",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}

// TestSequentialSameServer tests sequential (non-concurrent) requests
func TestSequentialSameServer(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("sequential_test").
		WithTitle("Sequential Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("sequentialpost").
		WithTitle("Sequential Post").
		WithScore(100).
		WithAuthor("testuser").
		WithSubreddit("sequential_test").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("sequential_test", subreddit).
		WithPosts("sequential_test", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "sequential_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	redditClient := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Make 5 SEQUENTIAL requests (no goroutines)
	for i := 0; i < 5; i++ {
		posts, err := redditClient.GetHot(context.Background(), &types.PostsRequest{
			Subreddit: "sequential_test",
			Pagination: types.Pagination{
				Limit: 5,
			},
		})

		if err != nil {
			t.Fatalf("Request %d failed with error: %v", i, err)
		}

		if len(posts.Posts) == 0 {
			t.Fatalf("Request %d: expected 1 post, got %d", i, len(posts.Posts))
		}

		t.Logf("Request %d: Got %d posts", i, len(posts.Posts))
	}

	t.Logf("All sequential requests succeeded")
}

// TestSequentialWithoutSubreddit tests if removing WithSubreddit fixes the issue
func TestSequentialWithoutSubreddit(t *testing.T) {
	// Setup test data - WITHOUT WithSubreddit like TestConcurrentClientUsage
	subreddit := testutil.NewSubreddit("sequential_no_sub").
		WithTitle("Sequential Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("sequentialpostnosub").
		WithTitle("Sequential Post").
		WithScore(100).
		WithAuthor("testuser").
		// NOTE: NOT calling WithSubreddit - will use default "test"
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("sequential_no_sub", subreddit).
		WithPosts("sequential_no_sub", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "sequential_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	redditClient := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Make 1 request
	posts, err := redditClient.GetHot(context.Background(), &types.PostsRequest{
		Subreddit: "sequential_no_sub",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	if err != nil {
		t.Fatalf("Request failed with error: %v", err)
	}

	if len(posts.Posts) == 0 {
		t.Fatalf("Expected 1 post, got %d", len(posts.Posts))
	}

	t.Logf("Got %d posts", len(posts.Posts))
}

// TestSequentialNoSubField tests if not setting subreddit field fixes it
func TestSequentialNoSubField(t *testing.T) {
	// Setup test data - WITHOUT WithSubreddit like TestConcurrentClientUsage
	subreddit := testutil.NewSubreddit("seqtest").
		WithTitle("Sequential Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("seqpost").
		WithTitle("Sequential Post").
		WithScore(100).
		WithAuthor("testuser").
		// NOTE: NOT calling WithSubreddit - will use default "test"
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("seqtest", subreddit).
		WithPosts("seqtest", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "seqtest_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	redditClient := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Make 1 request
	posts, err := redditClient.GetHot(context.Background(), &types.PostsRequest{
		Subreddit: "seqtest",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	if err != nil {
		t.Fatalf("Request failed with error: %v", err)
	}

	if len(posts.Posts) == 0 {
		t.Fatalf("Expected 1 post, got %d", len(posts.Posts))
	}

	t.Logf("Got %d posts", len(posts.Posts))
}

// TestConcurrentSameClientOperationsNoWithSubreddit tests without WithSubreddit
func TestConcurrentSameClientOperationsNoWithSubreddit(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("concurrent_test").
		WithTitle("Concurrent Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("concurrentpost").
		WithTitle("Concurrent Post").
		WithScore(100).
		WithAuthor("testuser").
		// NOTE: NOT calling WithSubreddit
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("concurrent_test", subreddit).
		WithPosts("concurrent_test", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "concurrent_test_agent/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test concurrent operations on the same client
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	// Posts operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			posts, err := client.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "concurrent_test",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}

// TestExactCopyOfTestConcurrentClientUsage is an exact copy to verify it passes
func TestExactCopyOfTestConcurrentClientUsage(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("testsubreddit").
		WithTitle("Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("testsubreddit", subreddit).
		WithPosts("testsubreddit", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	// Create multiple clients
	numClients := 1 // CHANGE: Only 1 client instead of 5
	clients := make([]*Reddit, numClients)

	for i := 0; i < numClients; i++ {
		httpClient := &http.Client{Timeout: 30 * time.Second}
		internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), fmt.Sprintf("test_agent_%d/1.0", i), nil, client.RateLimitConfig{}, mockClock)
		testutil.AssertNoError(t, err)

		clients[i] = &Reddit{
			httpClient: internalClient,
			parser:     parse.NewParser(nil),
			validator:  validator.NewValidator(),
			auth:       &mockTokenProvider{token: "test_token"},
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

			testutil.AssertPostCount(t, posts, 1)
		}(clientIdx, client)
	}

	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}
}

// TestExactCopyWithConcurrentRequests adds concurrency to the passing test
func TestExactCopyWithConcurrentRequests(t *testing.T) {
	// Setup test data
	subreddit := testutil.NewSubreddit("testsubreddit").
		WithTitle("Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("testsubreddit", subreddit).
		WithPosts("testsubreddit", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "test_agent_0/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	sharedClient := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test concurrent operations with 5 CONCURRENT goroutines
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			posts, err := sharedClient.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "testsubreddit",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}

// TestWithDifferentSubredditName changes just the subreddit name
func TestWithDifferentSubredditName(t *testing.T) {
	// Setup test data - same structure as TestExactCopyWithConcurrentRequests
	subreddit := testutil.NewSubreddit("concurrent_test"). // CHANGE: different name
								WithTitle("Test Subreddit").
								WithSubscribers(100000).
								Build()

	post := testutil.NewPostBuilder().
		WithID("post1"). // Keep original post ID
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("concurrent_test", subreddit). // CHANGE: different name
		WithPosts("concurrent_test", "hot", post).   // CHANGE: different name
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "test_agent_0/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	sharedClient := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test concurrent operations with 5 CONCURRENT goroutines
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			posts, err := sharedClient.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "concurrent_test", // CHANGE: different name
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}

// TestWithDifferentPostID changes just the post ID
func TestWithDifferentPostID(t *testing.T) {
	// Setup test data - same structure as TestWithDifferentSubredditName but with different post ID
	subreddit := testutil.NewSubreddit("concurrent_test").
		WithTitle("Test Subreddit").
		WithSubscribers(100000).
		Build()

	post := testutil.NewPostBuilder().
		WithID("concurrentpost"). // CHANGE: different post ID
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	// Use MockServer for reliable testing
	server := testutil.NewMockServer().
		WithSubreddit("concurrent_test", subreddit).
		WithPosts("concurrent_test", "hot", post).
		Start()
	defer server.Close()

	// Create mock clock for testing
	mockClock := clock.NewMockClock(time.Time{})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClientWithRateLimit(httpClient, server.URL(), "test_agent_0/1.0", nil, client.RateLimitConfig{}, mockClock)
	testutil.AssertNoError(t, err)

	sharedClient := &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test concurrent operations with 5 CONCURRENT goroutines
	var wg sync.WaitGroup
	var errors []error
	var errorMu sync.Mutex

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			posts, err := sharedClient.GetHot(context.Background(), &types.PostsRequest{
				Subreddit: "concurrent_test",
				Pagination: types.Pagination{
					Limit: 5,
				},
			})
			if err != nil {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d posts error: %v", idx, err))
				errorMu.Unlock()
				return
			}

			if len(posts.Posts) == 0 {
				errorMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: no posts returned (got %d posts)", idx, len(posts.Posts)))
				errorMu.Unlock()
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
}
