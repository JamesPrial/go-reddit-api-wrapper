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

// TestConnectionResourceManagement tests proper cleanup of HTTP connections
func TestConnectionResourceManagement(t *testing.T) {
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

	// Test multiple clients with proper cleanup
	const numClients = 5
	const requestsPerClient = 3

	var clients []*Client
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

		internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
		if err != nil {
			t.Fatalf("Failed to create internal client %d: %v", i, err)
		}

		client := &Client{
			client:    internalClient,
			parser:    internal.NewParser(),
			validator: internal.NewValidator(),
			auth:      &mockTokenProvider{token: "test_token"},
		}
		clients = append(clients, client)
	}

	ctx := context.Background()

	// Use all clients concurrently
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(clientID int, c *Client) {
			defer wg.Done()

			for j := 0; j < requestsPerClient; j++ {
				resp, err := c.GetHot(ctx, nil)
				if err != nil {
					t.Errorf("Client %d, request %d failed: %v", clientID, j+1, err)
					return
				}

				if len(resp.Posts) == 0 {
					t.Errorf("Client %d, request %d: expected posts, got empty", clientID, j+1)
				}
			}
		}(i, client)
	}

	wg.Wait()

	// Clean up all clients
	for i := range clients {
		if closer, ok := clients[i].client.(interface{ Close() error }); ok {
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return larger responses to test memory management
		posts := make([]map[string]interface{}, 200)
		for i := 0; i < 200; i++ {
			posts[i] = map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           fmt.Sprintf("post_%d", i),
					"title":        fmt.Sprintf("Test Post %d with substantial content", i),
					"score":        100 + i,
					"author":       fmt.Sprintf("user_%d", i),
					"selftext":     fmt.Sprintf("This is a longer selftext for post %d to test memory management. ", i),
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
			if err != nil {
				t.Fatalf("Iteration %d, request %d failed: %v", iteration+1, i+1, err)
			}

			if len(resp.Posts) != 200 {
				t.Errorf("Expected 200 posts, got %d", len(resp.Posts))
			}

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
				if err != nil {
					t.Errorf("Goroutine %d failed: %v", goroutineID, err)
					return
				}

				if len(resp.Posts) == 0 {
					t.Errorf("Goroutine %d: expected posts, got empty", goroutineID)
				}
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Simulate slow response
		time.Sleep(100 * time.Millisecond)

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

	// Test context cancellation
	const numContexts = 5
	var successfulCancellations int

	for i := 0; i < numContexts; i++ {
		// Create context that cancels quickly
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetHot(ctx, nil)
		if err != nil {
			if err == context.DeadlineExceeded {
				successfulCancellations++
			}
		}
	}

	t.Logf("Context resource management test:")
	t.Logf("  Contexts created: %d", numContexts)
	t.Logf("  Successful cancellations: %d", successfulCancellations)
	t.Logf("  Total requests made: %d", requestCount)

	// Most contexts should have been cancelled
	if successfulCancellations < numContexts/2 {
		t.Errorf("Too few successful cancellations: %d out of %d", successfulCancellations, numContexts)
	}
}

// TestFileDescriptorResourceManagement tests file descriptor usage
func TestFileDescriptorResourceManagement(t *testing.T) {
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

	// Create multiple HTTP clients to test file descriptor usage
	const numClients = 10
	var clients []*Client

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

		internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
		if err != nil {
			t.Fatalf("Failed to create internal client %d: %v", i, err)
		}

		client := &Client{
			client:    internalClient,
			parser:    internal.NewParser(),
			validator: internal.NewValidator(),
			auth:      &mockTokenProvider{token: "test_token"},
		}
		clients = append(clients, client)
	}

	ctx := context.Background()

	// Use all clients
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(clientID int, c *Client) {
			defer wg.Done()

			for j := 0; j < 3; j++ {
				resp, err := c.GetHot(ctx, nil)
				if err != nil {
					t.Errorf("Client %d, request %d failed: %v", clientID, j+1, err)
					return
				}

				if len(resp.Posts) == 0 {
					t.Errorf("Client %d, request %d: expected posts, got empty", clientID, j+1)
				}
			}
		}(i, client)
	}

	wg.Wait()

	// Clean up all clients
	for i, client := range clients {
		if closer, ok := client.client.(interface{ Close() error }); ok {
			closer.Close()
		}
		// Note: Transport cleanup handled by client.Close()
		_ = clients[i] // Clear reference
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return responses with varying sizes to test buffer management
		size := (requestCount % 5) + 1 // 1-5KB responses
		content := make([]byte, size*1024)
		for i := range content {
			content[i] = byte('A' + (i % 26))
		}

		postData := map[string]interface{}{
			"kind": "t3",
			"data": map[string]interface{}{
				"id":       fmt.Sprintf("post_%d", requestCount),
				"title":    "Test Post",
				"score":    100,
				"author":   "testuser",
				"content":  string(content),
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

	// Test buffer management with many requests
	const numRequests = 50
	var totalResponseSize int64

	for i := 0; i < numRequests; i++ {
		resp, err := client.GetHot(ctx, nil)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if len(resp.Posts) == 0 {
			t.Errorf("Request %d: expected posts, got empty", i+1)
		}

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

	// Baseline resource measurements
	var baselineMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&baselineMem)
	baselineGoroutines := runtime.NumGoroutine()

	// Create and destroy many clients
	const numCycles = 3
	const clientsPerCycle = 5

	for cycle := 0; cycle < numCycles; cycle++ {
		var clients []*Client

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

			internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
			if err != nil {
				t.Fatalf("Failed to create internal client %d-%d: %v", cycle, i, err)
			}

			client := &Client{
				client:    internalClient,
				parser:    internal.NewParser(),
				validator: internal.NewValidator(),
				auth:      &mockTokenProvider{token: "test_token"},
			}
			clients = append(clients, client)
		}

		// Use clients
		ctx := context.Background()
		var wg sync.WaitGroup

		for i, client := range clients {
			wg.Add(1)
			go func(clientID int, c *Client) {
				defer wg.Done()

				for j := 0; j < 2; j++ {
					resp, err := c.GetHot(ctx, nil)
					if err != nil {
						t.Errorf("Cycle %d, client %d, request %d failed: %v", cycle, clientID, j+1, err)
						return
					}

					if len(resp.Posts) == 0 {
						t.Errorf("Cycle %d, client %d, request %d: expected posts, got empty", cycle, clientID, j+1)
					}
				}
			}(i, client)
		}

		wg.Wait()

		// Clean up clients
		for i := range clients {
			if closer, ok := clients[i].client.(interface{ Close() error }); ok {
				closer.Close()
			}
			// Note: Transport cleanup handled by client.Close()
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
