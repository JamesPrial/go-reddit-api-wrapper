package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
)

// TestMemoryUsageEfficiency tests memory usage patterns and efficiency
func TestMemoryUsageEfficiency(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return moderate-sized response
		posts := make([]map[string]interface{}, 50)
		for i := 0; i < 50; i++ {
			posts[i] = map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           fmt.Sprintf("post_%d", i),
					"title":        fmt.Sprintf("Test Post %d with some content", i),
					"score":        100 + i,
					"author":       fmt.Sprintf("user_%d", i),
					"selftext":     fmt.Sprintf("This is test content for post %d. ", i),
					"created_utc":  1609459200.0 + float64(i),
					"num_comments": i + 1,
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
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Measure memory before operations
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Make multiple requests
	const iterations = 10
	for i := 0; i < iterations; i++ {
		resp, err := client.GetHot(ctx, nil)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if len(resp.Posts) != 50 {
			t.Errorf("Expected 50 posts, got %d", len(resp.Posts))
		}
	}

	// Measure memory after operations
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Calculate memory usage
	memUsed := m2.Alloc - m1.Alloc
	memPerRequest := memUsed / iterations

	t.Logf("Memory efficiency test:")
	t.Logf("  Total iterations: %d", iterations)
	t.Logf("  Memory used: %d bytes", memUsed)
	t.Logf("  Memory per request: %d bytes", memPerRequest)
	t.Logf("  Total requests made: %d", requestCount)

	// Memory usage should be reasonable (less than 1MB per request for this data size)
	if memPerRequest > 1024*1024 {
		t.Errorf("Memory usage per request too high: %d bytes", memPerRequest)
	}

	if requestCount != iterations {
		t.Errorf("Expected %d requests, got %d", iterations, requestCount)
	}
}

// TestConcurrentPerformance tests performance under concurrent load
func TestConcurrentPerformance(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		postData := map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":      "test_post",
				"title":   "Test Post",
				"score":   100,
				"author":  "testuser",
				"created_utc": 1609459200.0,
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{postData},
				"after":    "",
				"before":   "",
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test concurrent performance
	const numGoroutines = 10
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := client.GetHot(ctx, nil)
				if err != nil {
					t.Errorf("Goroutine %d, request %d failed: %v", goroutineID, j+1, err)
					return
				}

				if len(resp.Posts) == 0 {
					t.Errorf("Goroutine %d, request %d: expected posts, got empty", goroutineID, j+1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := numGoroutines * requestsPerGoroutine
	requestsPerSecond := float64(totalRequests) / duration.Seconds()

	t.Logf("Concurrent performance test:")
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Requests per goroutine: %d", requestsPerGoroutine)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Total time: %v", duration)
	t.Logf("  Requests per second: %.2f", requestsPerSecond)
	t.Logf("  Average request time: %v", duration/time.Duration(totalRequests))

	if requestCount != totalRequests {
		t.Errorf("Expected %d requests, got %d", totalRequests, requestCount)
	}

	// Performance should be reasonable (more than 50 requests/second for this simple test)
	if requestsPerSecond < 50 {
		t.Errorf("Performance too low: %.2f requests/second", requestsPerSecond)
	}
}

// TestParsingPerformance tests JSON parsing performance
func TestParsingPerformance(t *testing.T) {
	// Create a large JSON response for testing
	posts := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		posts[i] = map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":           fmt.Sprintf("post_%d", i),
				"title":        fmt.Sprintf("Test Post %d with a reasonably long title", i),
				"score":        100 + i,
				"author":       fmt.Sprintf("user_%d", i),
				"selftext":     fmt.Sprintf("This is test content for post %d. It has some length to make parsing more realistic. ", i),
				"created_utc":  1609459200.0 + float64(i),
				"num_comments": i + 1,
				"over_18":      false,
				"stickied":     false,
			},
		}
	}

	listingData := map[string]interface{}{
		"kind": "Listing",
		"data": map[string]interface{}{
			"children": posts,
			"after":    "t3_next",
			"before":   "",
		},
	}

	jsonData, err := json.Marshal(listingData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test parsing performance
	const iterations = 5
	var totalParseTime time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := client.GetHot(ctx, nil)
		parseTime := time.Since(start)
		totalParseTime += parseTime

		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if len(resp.Posts) != 1000 {
			t.Errorf("Expected 1000 posts, got %d", len(resp.Posts))
		}
	}

	avgParseTime := totalParseTime / iterations
	bytesPerSecond := float64(len(jsonData)) / avgParseTime.Seconds()

	t.Logf("Parsing performance test:")
	t.Logf("  JSON size: %d bytes", len(jsonData))
	t.Logf("  Posts per response: %d", len(posts))
	t.Logf("  Iterations: %d", iterations)
	t.Logf("  Average parse time: %v", avgParseTime)
	t.Logf("  Parsing speed: %.2f bytes/second", bytesPerSecond)
	t.Logf("  Posts per second: %.2f", float64(len(posts))/avgParseTime.Seconds())

	// Parsing should be reasonably fast (less than 100ms for 1000 posts)
	if avgParseTime > 100*time.Millisecond {
		t.Errorf("Parsing too slow: %v for %d posts", avgParseTime, len(posts))
	}
}

// TestConnectionPooling tests HTTP connection pooling efficiency
func TestConnectionPooling(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		postData := map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":      "test_post",
				"title":   "Test Post",
				"score":   100,
				"author":  "testuser",
				"created_utc": 1609459200.0,
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{postData},
				"after":    "",
				"before":   "",
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	// Create client with connection pooling
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test connection pooling with sequential requests
	const numRequests = 20
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		resp, err := client.GetHot(ctx, nil)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if len(resp.Posts) == 0 {
			t.Errorf("Request %d: expected posts, got empty", i+1)
		}
	}

	totalTime := time.Since(start)
	avgTimePerRequest := totalTime / numRequests

	t.Logf("Connection pooling test:")
	t.Logf("  Number of requests: %d", numRequests)
	t.Logf("  Total time: %v", totalTime)
	t.Logf("  Average time per request: %v", avgTimePerRequest)

	if requestCount != numRequests {
		t.Errorf("Expected %d requests, got %d", numRequests, requestCount)
	}

	// With connection pooling, subsequent requests should be faster
	if avgTimePerRequest > 50*time.Millisecond {
		t.Errorf("Connection pooling ineffective: average request time %v", avgTimePerRequest)
	}
}

// TestGoroutineScalability tests scalability with increasing goroutine count
func TestGoroutineScalability(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Simulate minimal processing time
		time.Sleep(1 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		postData := map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":      "test_post",
				"title":   "Test Post",
				"score":   100,
				"author":  "testuser",
				"created_utc": 1609459200.0,
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{postData},
				"after":    "",
				"before":   "",
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test scalability with different goroutine counts
	goroutineCounts := []int{1, 5, 10, 20}
	const requestsPerGoroutine = 3

	for _, numGoroutines := range goroutineCounts {
		// Reset counters
		requestCount = 0
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				for j := 0; j < requestsPerGoroutine; j++ {
					resp, err := client.GetHot(ctx, nil)
					if err != nil {
						t.Errorf("Request failed: %v", err)
						return
					}

					if len(resp.Posts) == 0 {
						t.Error("Expected posts, got empty")
					}
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		totalRequests := numGoroutines * requestsPerGoroutine
		requestsPerSecond := float64(totalRequests) / duration.Seconds()

		t.Logf("Scalability test with %d goroutines:", numGoroutines)
		t.Logf("  Total requests: %d", totalRequests)
		t.Logf("  Duration: %v", duration)
		t.Logf("  Requests per second: %.2f", requestsPerSecond)

		if requestCount != totalRequests {
			t.Errorf("Expected %d requests, got %d", totalRequests, requestCount)
		}
	}
}

// TestMemoryLeakDetection tests for memory leaks over time
func TestMemoryLeakDetection(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		postData := map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":      "test_post",
				"title":   "Test Post",
				"score":   100,
				"author":  "testuser",
				"created_utc": 1609459200.0,
			},
		}

		listingData := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{postData},
				"after":    "",
				"before":   "",
			},
		}
		json.NewEncoder(w).Encode(listingData)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Measure memory over multiple iterations to detect leaks
	const iterations = 5
	const requestsPerIteration = 20

	var baselineMem uint64
	var memReadings []uint64

	for iteration := 0; iteration < iterations; iteration++ {
		// Force garbage collection before measurement
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		if iteration == 0 {
			baselineMem = m.Alloc
		}
		memReadings = append(memReadings, m.Alloc)

		// Make requests
		for i := 0; i < requestsPerIteration; i++ {
			resp, err := client.GetHot(ctx, nil)
			if err != nil {
				t.Fatalf("Iteration %d, request %d failed: %v", iteration+1, i+1, err)
			}

			if len(resp.Posts) == 0 {
				t.Errorf("Iteration %d, request %d: expected posts, got empty", iteration+1, i+1)
			}
		}

		t.Logf("Memory after iteration %d: %d bytes", iteration+1, m.Alloc)
	}

	// Check for memory leaks
	finalMem := memReadings[len(memReadings)-1]
	memIncrease := finalMem - baselineMem
	avgMemPerRequest := float64(memIncrease) / float64(iterations*requestsPerIteration)

	t.Logf("Memory leak detection:")
	t.Logf("  Baseline memory: %d bytes", baselineMem)
	t.Logf("  Final memory: %d bytes", finalMem)
	t.Logf("  Memory increase: %d bytes", memIncrease)
	t.Logf("  Average memory per request: %.2f bytes", avgMemPerRequest)
	t.Logf("  Total requests: %d", requestCount)

	// Memory increase should be minimal (less than 1KB per request)
	if avgMemPerRequest > 1024 {
		t.Errorf("Potential memory leak detected: %.2f bytes per request", avgMemPerRequest)
	}

	if requestCount != iterations*requestsPerIteration {
		t.Errorf("Expected %d requests, got %d", iterations*requestsPerIteration, requestCount)
	}
}

// TestCPUUsageEfficiency tests CPU usage patterns
func TestCPUUsageEfficiency(t *testing.T) {
	var requestCount int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return moderately complex data
		posts := make([]map[string]interface{}, 100)
		for i := 0; i < 100; i++ {
			posts[i] = map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           fmt.Sprintf("post_%d", i),
					"title":        fmt.Sprintf("Test Post %d", i),
					"score":        100 + i,
					"author":       fmt.Sprintf("user_%d", i),
					"selftext":     fmt.Sprintf("Content for post %d", i),
					"created_utc":  1609459200.0 + float64(i),
					"num_comments": i + 1,
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
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal client: %v", err)
	}

	client := &Client{
		client:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Measure CPU time
	const numRequests = 10
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		resp, err := client.GetHot(ctx, nil)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if len(resp.Posts) != 100 {
			t.Errorf("Expected 100 posts, got %d", len(resp.Posts))
		}
	}

	totalTime := time.Since(start)
	avgTimePerRequest := totalTime / numRequests

	t.Logf("CPU usage efficiency test:")
	t.Logf("  Number of requests: %d", numRequests)
	t.Logf("  Total time: %v", totalTime)
	t.Logf("  Average time per request: %v", avgTimePerRequest)
	t.Logf("  Posts processed per second: %.2f", float64(numRequests*100)/totalTime.Seconds())

	if requestCount != numRequests {
		t.Errorf("Expected %d requests, got %d", numRequests, requestCount)
	}

	// Processing should be efficient (less than 50ms per request for 100 posts)
	if avgTimePerRequest > 50*time.Millisecond {
		t.Errorf("CPU usage too high: %v per request for 100 posts", avgTimePerRequest)
	}
}
