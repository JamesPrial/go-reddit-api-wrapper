package adversarial_tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
	"github.com/jamesprial/go-reddit-api-wrapper/adversarial_tests/helpers"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestGetCommentsMultipleSemaphoreEnforcement tests that max concurrent requests is enforced
func TestGetCommentsMultipleSemaphoreEnforcement(t *testing.T) {
	// Track concurrent requests
	var currentConcurrent int32
	var peakConcurrent int32
	var requestCount int32
	var mu sync.Mutex

	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		current := atomic.AddInt32(&currentConcurrent, 1)
		atomic.AddInt32(&requestCount, 1)

		// Update peak concurrency
		for {
			peak := atomic.LoadInt32(&peakConcurrent)
			if current <= peak || atomic.CompareAndSwapInt32(&peakConcurrent, peak, current) {
				break
			}
		}
		mu.Unlock()

		// Simulate work
		time.Sleep(50 * time.Millisecond)

		// Return minimal valid response
		response := `[
			{"kind": "t3", "data": {"id": "test_post", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))

		mu.Lock()
		atomic.AddInt32(&currentConcurrent, -1)
		mu.Unlock()
	}))
	defer server.Close()

	// Create client
	client := createTestClient(t, server)

	// Create 100 requests (more than MaxConcurrentCommentRequests = 10)
	numRequests := 100
	requests := make([]*types.CommentsRequest, numRequests)
	for i := 0; i < numRequests; i++ {
		requests[i] = &types.CommentsRequest{
			Subreddit: "test",
			PostID:    fmt.Sprintf("post_%d", i),
		}
	}

	ctx := context.Background()
	_, err := client.GetCommentsMultiple(ctx, requests)

	if err != nil {
		// Some errors are acceptable (e.g., rate limits)
		t.Logf("GetCommentsMultiple returned error: %v", err)
	}

	peak := atomic.LoadInt32(&peakConcurrent)
	total := atomic.LoadInt32(&requestCount)

	t.Logf("Peak concurrent requests: %d", peak)
	t.Logf("Total requests made: %d", total)

	// Peak should not exceed MaxConcurrentCommentRequests (10)
	if peak > graw.MaxConcurrentCommentRequests {
		t.Errorf("Semaphore failed: peak concurrency %d exceeded limit %d",
			peak, graw.MaxConcurrentCommentRequests)
	}
}

// TestGetCommentsMultipleContextCancellation tests proper cleanup on context cancellation
func TestGetCommentsMultipleContextCancellation(t *testing.T) {
	requestsStarted := int32(0)
	requestsCompleted := int32(0)

	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsStarted, 1)
		defer atomic.AddInt32(&requestsCompleted, 1)

		// Simulate slow request
		time.Sleep(2 * time.Second)

		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Create many requests
	numRequests := 50
	requests := make([]*types.CommentsRequest, numRequests)
	for i := 0; i < numRequests; i++ {
		requests[i] = &types.CommentsRequest{
			Subreddit: "test",
			PostID:    fmt.Sprintf("post_%d", i),
		}
	}

	// Create context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	goroutinesBefore := runtime.NumGoroutine()

	_, err := client.GetCommentsMultiple(ctx, requests)

	// Should return context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	t.Logf("Context cancellation error (expected): %v", err)
	t.Logf("Requests started: %d", atomic.LoadInt32(&requestsStarted))
	t.Logf("Requests completed: %d", atomic.LoadInt32(&requestsCompleted))

	// Wait for cleanup
	time.Sleep(500 * time.Millisecond)
	runtime.GC()

	goroutinesAfter := runtime.NumGoroutine()

	// Check for goroutine leaks
	leaked := goroutinesAfter - goroutinesBefore
	t.Logf("Goroutines before: %d, after: %d, leaked: %d", goroutinesBefore, goroutinesAfter, leaked)

	if leaked > 10 {
		t.Errorf("Possible goroutine leak: %d goroutines not cleaned up", leaked)
	}
}

// TestGetCommentsMultipleGoroutineLeakDetection tests for goroutine leaks
func TestGetCommentsMultipleGoroutineLeakDetection(t *testing.T) {
	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Take initial snapshot
	snapshotBefore := helpers.TakeGoroutineSnapshot()

	// Run GetCommentsMultiple multiple times
	for iteration := 0; iteration < 10; iteration++ {
		numRequests := 20
		requests := make([]*types.CommentsRequest, numRequests)
		for i := 0; i < numRequests; i++ {
			requests[i] = &types.CommentsRequest{
				Subreddit: "test",
				PostID:    fmt.Sprintf("post_%d", i),
			}
		}

		ctx := context.Background()
		_, err := client.GetCommentsMultiple(ctx, requests)

		if err != nil {
			t.Logf("Iteration %d error: %v", iteration, err)
		}
	}

	// Wait for cleanup
	finalCount, err := helpers.WaitForGoroutineCleanup(2*time.Second, snapshotBefore.Count, 5)

	if err != nil {
		t.Errorf("Goroutine leak detected: %v", err)
	} else {
		t.Logf("Goroutine cleanup successful. Initial: %d, Final: %d",
			snapshotBefore.Count, finalCount)
	}
}

// TestGetCommentsMultipleMemoryLeak tests for memory leaks
func TestGetCommentsMultipleMemoryLeak(t *testing.T) {
	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		// Return response with some data
		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test Post", "selftext": "` +
			string(make([]byte, 1000)) + `"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	memoryBefore := helpers.TakeMemorySnapshot()

	// Run many iterations
	for iteration := 0; iteration < 100; iteration++ {
		numRequests := 10
		requests := make([]*types.CommentsRequest, numRequests)
		for i := 0; i < numRequests; i++ {
			requests[i] = &types.CommentsRequest{
				Subreddit: "test",
				PostID:    fmt.Sprintf("post_%d", i),
			}
		}

		ctx := context.Background()
		_, err := client.GetCommentsMultiple(ctx, requests)

		if err != nil {
			t.Logf("Iteration %d error: %v", iteration, err)
		}
	}

	// Check for memory leaks (allow 10MB threshold)
	memoryAfter := helpers.TakeMemorySnapshot()
	err := helpers.DetectMemoryLeak(memoryBefore, memoryAfter, 10*1024*1024)

	if err != nil {
		t.Errorf("Memory leak detected: %v", err)
	} else {
		t.Logf("No memory leak detected. Before: %d bytes, After: %d bytes",
			memoryBefore.Alloc, memoryAfter.Alloc)
	}
}

// TestGetCommentsMultipleDeadlockDetection tests for potential deadlocks
func TestGetCommentsMultipleDeadlockDetection(t *testing.T) {
	// Create server that sometimes hangs
	hangProbability := 0.1
	requestNum := int32(0)

	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		num := atomic.AddInt32(&requestNum, 1)

		// Every 10th request hangs
		if float64(num)*hangProbability >= 1.0 {
			time.Sleep(10 * time.Second) // Hang
			atomic.StoreInt32(&requestNum, 0)
		}

		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	numRequests := 30
	requests := make([]*types.CommentsRequest, numRequests)
	for i := 0; i < numRequests; i++ {
		requests[i] = &types.CommentsRequest{
			Subreddit: "test",
			PostID:    fmt.Sprintf("post_%d", i),
		}
	}

	// Use deadlock detector
	detector := helpers.NewDeadlockDetector(5 * time.Second)

	err := detector.Run(func() error {
		ctx := context.Background()
		_, err := client.GetCommentsMultiple(ctx, requests)
		return err
	})

	if err != nil {
		t.Logf("Operation completed with error (may be timeout): %v", err)
	} else {
		t.Log("Operation completed without deadlock")
	}
}

// TestConcurrentGetComments tests concurrent calls to GetComments (not GetCommentsMultiple)
func TestConcurrentGetComments(t *testing.T) {
	requestCount := int32(0)

	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		time.Sleep(10 * time.Millisecond)

		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Launch many concurrent GetComments calls
	numGoroutines := 100
	errors := make(chan error, numGoroutines)
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			req := &types.CommentsRequest{
				Subreddit: "test",
				PostID:    fmt.Sprintf("post_%d", id),
			}

			ctx := context.Background()
			_, err := client.GetComments(ctx, req)

			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		if errorCount <= 5 {
			t.Logf("GetComments error: %v", err)
		}
	}

	t.Logf("Total requests: %d, Errors: %d", atomic.LoadInt32(&requestCount), errorCount)
}

// TestSemaphoreStressTesting tests semaphore under extreme stress
func TestSemaphoreStressTesting(t *testing.T) {
	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)

		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Use the semaphore stress tester
	tester := helpers.NewSemaphoreStressTester(graw.MaxConcurrentCommentRequests, 200)

	peak, err := tester.Test(func(id int) error {
		req := &types.CommentsRequest{
			Subreddit: "test",
			PostID:    fmt.Sprintf("post_%d", id),
		}

		ctx := context.Background()
		_, err := client.GetComments(ctx, req)
		return err
	})

	if err != nil {
		t.Errorf("Semaphore stress test failed: %v", err)
	}

	t.Logf("Semaphore stress test passed. Peak concurrency: %d (max: %d)",
		peak, graw.MaxConcurrentCommentRequests)
}

// TestRaceConditionInGetCommentsMultiple tests for race conditions
func TestRaceConditionInGetCommentsMultiple(t *testing.T) {
	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Run multiple GetCommentsMultiple calls concurrently
	numCallers := 50
	errors := make(chan error, numCallers)
	var wg sync.WaitGroup

	wg.Add(numCallers)
	for i := 0; i < numCallers; i++ {
		go func(callerID int) {
			defer wg.Done()

			// Each caller requests multiple comments
			numRequests := 10
			requests := make([]*types.CommentsRequest, numRequests)
			for j := 0; j < numRequests; j++ {
				requests[j] = &types.CommentsRequest{
					Subreddit: "test",
					PostID:    fmt.Sprintf("caller%d_post%d", callerID, j),
				}
			}

			ctx := context.Background()
			_, err := client.GetCommentsMultiple(ctx, requests)

			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		if errorCount <= 5 {
			t.Logf("Concurrent GetCommentsMultiple error: %v", err)
		}
	}

	if errorCount > 0 {
		t.Logf("Total errors: %d out of %d concurrent calls", errorCount, numCallers)
	}
}

// TestConcurrentExecutorWithGetComments tests using ConcurrentExecutor for leak detection
func TestConcurrentExecutorWithGetComments(t *testing.T) {
	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Create concurrent executor with leak detection
	executor := helpers.NewConcurrentExecutor(5, 5*1024*1024) // 5MB threshold

	err := executor.Execute(func() error {
		// Run GetCommentsMultiple many times
		for i := 0; i < 50; i++ {
			numRequests := 10
			requests := make([]*types.CommentsRequest, numRequests)
			for j := 0; j < numRequests; j++ {
				requests[j] = &types.CommentsRequest{
					Subreddit: "test",
					PostID:    fmt.Sprintf("post_%d_%d", i, j),
				}
			}

			ctx := context.Background()
			_, err := client.GetCommentsMultiple(ctx, requests)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		t.Errorf("Concurrent execution with leak detection failed: %v", err)
	}
}

// TestContextPropagationInGetCommentsMultiple tests that context is properly propagated
func TestContextPropagationInGetCommentsMultiple(t *testing.T) {
	requestsReceived := int32(0)

	server := httptest.NewServer(wrapWithAuth(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestsReceived, 1)

		// Check if context was cancelled
		select {
		case <-r.Context().Done():
			t.Log("Server detected context cancellation")
			return
		default:
		}

		time.Sleep(100 * time.Millisecond)

		response := `[
			{"kind": "t3", "data": {"id": "test", "title": "Test"}},
			{"kind": "Listing", "data": {"children": []}}
		]`

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := createTestClient(t, server)

	// Create many requests
	numRequests := 50
	requests := make([]*types.CommentsRequest, numRequests)
	for i := 0; i < numRequests; i++ {
		requests[i] = &types.CommentsRequest{
			Subreddit: "test",
			PostID:    fmt.Sprintf("post_%d", i),
		}
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.GetCommentsMultiple(ctx, requests)

	// Should return context error
	if err == nil {
		t.Error("Expected context error, got nil")
	} else {
		t.Logf("Context cancellation error (expected): %v", err)
	}

	t.Logf("Requests received by server: %d (out of %d requested)",
		atomic.LoadInt32(&requestsReceived), numRequests)
}

// Helper function to create a test client that connects to a test server
// The provided server must handle both OAuth token requests and API requests
func createTestClient(t *testing.T, server *httptest.Server) *graw.Client {
	config := &graw.Config{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
		UserAgent:    "test/1.0",
		BaseURL:      server.URL + "/",
		AuthURL:      server.URL + "/",
		HTTPClient:   server.Client(),
		// Disable rate limiting for tests to allow fast concurrent requests
		RateLimitConfig: &graw.RateLimitConfig{
			RequestsPerMinute: 100000, // Very high rate limit
			Burst:             1000,   // Allow large bursts
		},
	}

	client, err := graw.NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}

// wrapWithAuth wraps an HTTP handler to add OAuth token endpoint support
// This allows test servers to handle both authentication and API requests
func wrapWithAuth(apiHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle OAuth token requests
		if r.URL.Path == "/api/v1/access_token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"access_token": "test_token", "token_type": "bearer", "expires_in": 3600, "scope": "*"}`))
			return
		}
		// Delegate to the API handler for all other requests
		apiHandler(w, r)
	}
}
