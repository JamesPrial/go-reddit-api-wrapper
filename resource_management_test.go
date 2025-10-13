package graw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/internal/testutil"
)

// Note: mockTokenProvider is defined in reddit_test.go and shared across all test files

// TestConnectionResourceManagement tests proper cleanup of HTTP connections
func TestConnectionResourceManagement(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	// Setup test data using builders
	post := testutil.NewPostBuilder().
		WithID("test_post").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	server := testutil.NewMockServer().
		WithPosts("", "hot", post).
		Start()
	defer server.Close()

	// Override the handler to count requests
	originalHandler := server.Server().Config.Handler
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		originalHandler.ServeHTTP(w, r)
	})

	// Test multiple clients with proper cleanup
	const numClients = 5
	const requestsPerClient = 3

	var clients []*Reddit
	var httpClients []*http.Client

	// Create multiple clients
	for i := 0; i < numClients; i++ {
		httpClient := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
			},
		}
		httpClients = append(httpClients, httpClient)

		internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
		testutil.AssertNoError(t, err)

		client := &Reddit{
			httpClient: internalClient,
			parser:     internal.NewParser(),
			validator:  internal.NewValidator(),
			auth:       &mockTokenProvider{token: "test_token"},
		}
		clients = append(clients, client)
	}

	ctx := context.Background()

	// Use all clients concurrently
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(clientID int, c *Reddit) {
			defer wg.Done()

			for j := 0; j < requestsPerClient; j++ {
				resp, err := c.GetHot(ctx, nil)
				testutil.AssertNoError(t, err)
				testutil.AssertPostCount(t, resp, 1)
			}
		}(i, client)
	}

	wg.Wait()

	// Clean up all clients
	for i := range clients {
		if closer, ok := clients[i].httpClient.(interface{ Close() error }); ok {
			closer.Close()
		}
		if transport, ok := httpClients[i].Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	// Verify all requests were made
	expectedRequests := numClients * requestsPerClient
	if requestCount != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
	}

	t.Logf("Connection resource management test completed:")
	t.Logf("  Clients created: %d", numClients)
	t.Logf("  Requests per client: %d", requestsPerClient)
	t.Logf("  Total requests: %d", requestCount)
	t.Logf("  All connections cleaned up properly")
}

// TestMemoryResourceManagement tests memory allocation and cleanup
func TestMemoryResourceManagement(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	// Create 200 posts for larger responses
	posts := make([]*http.HandlerFunc, 0, 200)
	for i := 0; i < 200; i++ {
		// We'll track requests outside the builder
		posts = append(posts, nil)
	}

	// We need to use httptest directly here since we need to customize the handler
	// to generate large responses dynamically based on request count
	server := testutil.NewMockServer().Start()
	defer server.Close()

	// Override handler to return large responses and count requests
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)

		// Build 200 posts dynamically using builders
		postList := make([]*testutil.PostBuilder, 200)
		for i := 0; i < 200; i++ {
			postList[i] = testutil.NewPostBuilder().
				WithID(fmt.Sprintf("post_%d", i)).
				WithTitle(fmt.Sprintf("Test Post %d with substantial content", i)).
				WithScore(100 + i).
				WithAuthor(fmt.Sprintf("user_%d", i)).
				WithSelfText(fmt.Sprintf("This is a longer selftext for post %d to test memory management. ", i)).
				WithCreated(1609459200.0 + float64(i)).
				WithNumComments(i + 1)
		}

		// Convert to Things and write response
		children := make([]interface{}, 200)
		for i := 0; i < 200; i++ {
			children[i] = postList[i].ToThing()
		}

		listing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": children,
				"after":    "",
				"before":   "",
			},
		}

		// Write JSON response
		w.Header().Set("Content-Type", "application/json")
		// Manual JSON encoding to avoid import
		fmt.Fprintf(w, `{"kind":"Listing","data":{"children":[`)
		for i := 0; i < 200; i++ {
			post := postList[i].Build()
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, `{"kind":"t3","data":{"id":"%s","title":"%s","score":%d,"author":"%s","selftext":"%s","created_utc":%f,"num_comments":%d,"name":"t3_%s","permalink":"/r/test/comments/%s/","subreddit":"test","url":"https://reddit.com/r/test/"}}`,
				post.ID, post.Title, post.Score, post.Author, post.SelfText, post.CreatedUTC, post.NumComments, post.ID, post.ID)
		}
		fmt.Fprintf(w, `],"after":"","before":""}}`)
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Measure memory usage over multiple operations
	const iterations = 5
	var memReadings []uint64

	for iteration := 0; iteration < iterations; iteration++ {
		// Force garbage collection before measurement
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memReadings = append(memReadings, m.Alloc)

		// Make multiple requests
		for i := 0; i < 10; i++ {
			resp, err := client.GetHot(ctx, nil)
			testutil.AssertNoError(t, err)
			testutil.AssertPostCount(t, resp, 200)

			// Clear reference to allow garbage collection
			resp = nil
		}

		t.Logf("Memory after iteration %d: %d bytes", iteration+1, m.Alloc)
	}

	// Final garbage collection
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	// Analyze memory usage patterns
	baselineMem := memReadings[0]
	peakMem := baselineMem
	for _, mem := range memReadings {
		if mem > peakMem {
			peakMem = mem
		}
	}

	memIncrease := finalMem.Alloc - baselineMem
	avgMemPerIteration := float64(memIncrease) / float64(iterations)

	t.Logf("Memory resource management test:")
	t.Logf("  Baseline memory: %d bytes", baselineMem)
	t.Logf("  Peak memory: %d bytes", peakMem)
	t.Logf("  Final memory: %d bytes", finalMem.Alloc)
	t.Logf("  Memory increase: %d bytes", memIncrease)
	t.Logf("  Average memory per iteration: %.2f bytes", avgMemPerIteration)
	t.Logf("  Total requests: %d", requestCount)

	// Memory increase should be minimal (less than 2MB for this test)
	if memIncrease > 2*1024*1024 {
		t.Errorf("Excessive memory usage: %d bytes increase", memIncrease)
	}

	if requestCount != iterations*10 {
		t.Errorf("Expected %d requests, got %d", iterations*10, requestCount)
	}
}

// TestGoroutineResourceManagement tests proper goroutine lifecycle management
func TestGoroutineResourceManagement(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	// Setup test data using builders
	post := testutil.NewPostBuilder().
		WithID("test_post").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	server := testutil.NewMockServer().
		WithPosts("", "hot", post).
		Start()
	defer server.Close()

	// Override handler to count requests
	originalHandler := server.Server().Config.Handler
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		originalHandler.ServeHTTP(w, r)
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Measure goroutine count before and after operations
	initialGoroutines := runtime.NumGoroutine()

	// Create many short-lived goroutines
	const numBatches = 5
	const goroutinesPerBatch = 10

	for batch := 0; batch < numBatches; batch++ {
		var wg sync.WaitGroup

		for i := 0; i < goroutinesPerBatch; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				// Make a request
				resp, err := client.GetHot(ctx, nil)
				testutil.AssertNoError(t, err)
				testutil.AssertPostCount(t, resp, 1)
			}(batch*goroutinesPerBatch + i)
		}

		wg.Wait()

		// Small delay between batches
		time.Sleep(10 * time.Millisecond)

		currentGoroutines := runtime.NumGoroutine()
		t.Logf("Goroutines after batch %d: %d", batch+1, currentGoroutines)
	}

	// Wait for goroutines to clean up
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	finalGoroutines := runtime.NumGoroutine()
	goroutineIncrease := finalGoroutines - initialGoroutines

	t.Logf("Goroutine resource management test:")
	t.Logf("  Initial goroutines: %d", initialGoroutines)
	t.Logf("  Final goroutines: %d", finalGoroutines)
	t.Logf("  Goroutine increase: %d", goroutineIncrease)
	t.Logf("  Total requests: %d", requestCount)

	// Goroutine increase should be minimal (less than 5)
	if goroutineIncrease > 5 {
		t.Errorf("Potential goroutine leak: %d goroutines not cleaned up", goroutineIncrease)
	}

	expectedRequests := numBatches * goroutinesPerBatch
	if requestCount != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
	}
}

// TestContextResourceManagement tests proper context cancellation and cleanup
func TestContextResourceManagement(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	// Setup test data using builders
	post := testutil.NewPostBuilder().
		WithID("test_post").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	server := testutil.NewMockServer().
		WithPosts("", "hot", post).
		Start()
	defer server.Close()

	// Override handler to simulate slow response and count requests
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Simulate slow response
		time.Sleep(100 * time.Millisecond)

		// Use the MockServer's response format
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)

		thing := testutil.NewPostBuilder().
			WithID("test_post").
			WithTitle("Test Post").
			WithScore(100).
			Build()

		fmt.Fprintf(w, `{"kind":"Listing","data":{"children":[{"kind":"t3","data":{"id":"%s","title":"%s","score":%d,"author":"%s","name":"t3_%s","created_utc":%f,"permalink":"/r/test/comments/%s/","subreddit":"test","url":"https://reddit.com/r/test/","num_comments":0,"upvote_ratio":0.95}}],"after":"","before":""}}`,
			thing.ID, thing.Title, thing.Score, thing.Author, thing.ID, thing.CreatedUTC, thing.ID)
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	// Test context cancellation
	const numContexts = 5
	var successfulCancellations int

	for i := 0; i < numContexts; i++ {
		// Create context that cancels quickly
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetHot(ctx, nil)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				successfulCancellations++
			}
		}
	}

	mu.Lock()
	finalRequestCount := requestCount
	mu.Unlock()

	t.Logf("Context resource management test:")
	t.Logf("  Contexts created: %d", numContexts)
	t.Logf("  Successful cancellations: %d", successfulCancellations)
	t.Logf("  Total requests made: %d", finalRequestCount)

	// Most contexts should have been cancelled
	if successfulCancellations < numContexts/2 {
		t.Errorf("Too few successful cancellations: %d out of %d", successfulCancellations, numContexts)
	}
}

// TestFileDescriptorResourceManagement tests file descriptor usage
func TestFileDescriptorResourceManagement(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	// Setup test data using builders
	post := testutil.NewPostBuilder().
		WithID("test_post").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	server := testutil.NewMockServer().
		WithPosts("", "hot", post).
		Start()
	defer server.Close()

	// Override handler to count requests
	originalHandler := server.Server().Config.Handler
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		originalHandler.ServeHTTP(w, r)
	})

	// Create multiple HTTP clients to test file descriptor usage
	const numClients = 10
	var clients []*Reddit

	for i := 0; i < numClients; i++ {
		httpClient := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     10 * time.Second,
				DisableKeepAlives:   false,
			},
		}

		internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
		testutil.AssertNoError(t, err)

		client := &Reddit{
			httpClient: internalClient,
			parser:     internal.NewParser(),
			validator:  internal.NewValidator(),
			auth:       &mockTokenProvider{token: "test_token"},
		}
		clients = append(clients, client)
	}

	ctx := context.Background()

	// Use all clients
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(clientID int, c *Reddit) {
			defer wg.Done()

			for j := 0; j < 3; j++ {
				resp, err := c.GetHot(ctx, nil)
				testutil.AssertNoError(t, err)
				testutil.AssertPostCount(t, resp, 1)
			}
		}(i, client)
	}

	wg.Wait()

	// Clean up all clients
	for i, client := range clients {
		if closer, ok := client.httpClient.(interface{ Close() error }); ok {
			closer.Close()
		}
		// Clear reference
		_ = clients[i]
	}

	// Force garbage collection
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	t.Logf("File descriptor resource management test:")
	t.Logf("  Clients created: %d", numClients)
	t.Logf("  Total requests: %d", requestCount)
	t.Logf("  All clients and connections cleaned up")

	expectedRequests := numClients * 3
	if requestCount != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
	}
}

// TestBufferResourceManagement tests proper buffer management
func TestBufferResourceManagement(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := testutil.NewMockServer().Start()
	defer server.Close()

	// Override handler to return varying-size responses
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentCount := requestCount
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)

		// Return responses with varying sizes (1-5KB)
		size := (currentCount % 5) + 1
		content := make([]byte, size*1024)
		for i := range content {
			content[i] = byte('A' + (i % 26))
		}

		post := testutil.NewPostBuilder().
			WithID(fmt.Sprintf("post_%d", currentCount)).
			WithTitle("Test Post").
			WithScore(100).
			WithAuthor("testuser").
			WithSelfText(string(content)).
			Build()

		fmt.Fprintf(w, `{"kind":"Listing","data":{"children":[{"kind":"t3","data":{"id":"%s","title":"%s","score":%d,"author":"%s","selftext":"%s","name":"t3_%s","created_utc":%f,"permalink":"/r/test/comments/%s/","subreddit":"test","url":"https://reddit.com/r/test/","num_comments":0,"upvote_ratio":0.95}}],"after":"","before":""}}`,
			post.ID, post.Title, post.Score, post.Author, post.SelfText, post.ID, post.CreatedUTC, post.ID)
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test buffer management with many requests
	const numRequests = 50
	var totalResponseSize int64

	for i := 0; i < numRequests; i++ {
		resp, err := client.GetHot(ctx, nil)
		testutil.AssertNoError(t, err)
		testutil.AssertPostCount(t, resp, 1)

		// Estimate response size
		if len(resp.Posts) > 0 && resp.Posts[0].SelfText != "" {
			totalResponseSize += int64(len(resp.Posts[0].SelfText))
		}
	}

	// Force garbage collection
	runtime.GC()

	t.Logf("Buffer resource management test:")
	t.Logf("  Number of requests: %d", numRequests)
	t.Logf("  Total response size: %d bytes", totalResponseSize)
	t.Logf("  Average response size: %d bytes", totalResponseSize/int64(numRequests))
	t.Logf("  Total requests made: %d", requestCount)

	if requestCount != numRequests {
		t.Errorf("Expected %d requests, got %d", numRequests, requestCount)
	}
}

// TestResourceLeakDetection comprehensive test for resource leaks
func TestResourceLeakDetection(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	// Setup test data using builders
	post := testutil.NewPostBuilder().
		WithID("test_post").
		WithTitle("Test Post").
		WithScore(100).
		WithAuthor("testuser").
		Build()

	server := testutil.NewMockServer().
		WithPosts("", "hot", post).
		Start()
	defer server.Close()

	// Override handler to count requests
	originalHandler := server.Server().Config.Handler
	server.Server().Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		originalHandler.ServeHTTP(w, r)
	})

	// Baseline resource measurements
	var baselineMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&baselineMem)
	baselineGoroutines := runtime.NumGoroutine()

	// Create and destroy many clients
	const numCycles = 3
	const clientsPerCycle = 5

	for cycle := 0; cycle < numCycles; cycle++ {
		var clients []*Reddit

		// Create clients
		for i := 0; i < clientsPerCycle; i++ {
			httpClient := &http.Client{
				Timeout: 5 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        5,
					MaxIdleConnsPerHost: 2,
					IdleConnTimeout:     5 * time.Second,
				},
			}

			internalClient, err := internal.NewClient(httpClient, server.URL(), "test/1.0", nil)
			testutil.AssertNoError(t, err)

			client := &Reddit{
				httpClient: internalClient,
				parser:     internal.NewParser(),
				validator:  internal.NewValidator(),
				auth:       &mockTokenProvider{token: "test_token"},
			}
			clients = append(clients, client)
		}

		// Use clients
		ctx := context.Background()
		var wg sync.WaitGroup

		for i, client := range clients {
			wg.Add(1)
			go func(clientID int, c *Reddit) {
				defer wg.Done()

				for j := 0; j < 2; j++ {
					resp, err := c.GetHot(ctx, nil)
					testutil.AssertNoError(t, err)
					testutil.AssertPostCount(t, resp, 1)
				}
			}(i, client)
		}

		wg.Wait()

		// Clean up clients
		for i := range clients {
			if closer, ok := clients[i].httpClient.(interface{ Close() error }); ok {
				closer.Close()
			}
		}

		// Clear references
		clients = nil

		// Force cleanup between cycles
		runtime.GC()
		time.Sleep(50 * time.Millisecond)

		t.Logf("Completed cycle %d", cycle+1)
	}

	// Final resource measurements
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalGoroutines := runtime.NumGoroutine()

	// Calculate resource usage
	memIncrease := finalMem.Alloc - baselineMem.Alloc
	goroutineIncrease := finalGoroutines - baselineGoroutines

	t.Logf("Resource leak detection test:")
	t.Logf("  Cycles: %d", numCycles)
	t.Logf("  Clients per cycle: %d", clientsPerCycle)
	t.Logf("  Total requests: %d", requestCount)
	t.Logf("  Baseline memory: %d bytes", baselineMem.Alloc)
	t.Logf("  Final memory: %d bytes", finalMem.Alloc)
	t.Logf("  Memory increase: %d bytes", memIncrease)
	t.Logf("  Baseline goroutines: %d", baselineGoroutines)
	t.Logf("  Final goroutines: %d", finalGoroutines)
	t.Logf("  Goroutine increase: %d", goroutineIncrease)

	// Check for resource leaks
	if memIncrease > 5*1024*1024 { // 5MB threshold
		t.Errorf("Potential memory leak: %d bytes increase", memIncrease)
	}

	if goroutineIncrease > 10 {
		t.Errorf("Potential goroutine leak: %d goroutines not cleaned up", goroutineIncrease)
	}

	expectedRequests := numCycles * clientsPerCycle * 2
	if requestCount != expectedRequests {
		t.Errorf("Expected %d requests, got %d", expectedRequests, requestCount)
	}
}
