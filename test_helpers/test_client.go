package test_helpers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
)

// TestClient provides a wrapper around the Reddit client for testing
type TestClient struct {
	*graw.Client
	mockServer *RedditMockServer
	config     MockClientConfig
	mu         sync.RWMutex
}

// NewTestClient creates a new test client with a mock server
func NewTestClient(config *MockClientConfig) *TestClient {
	if config == nil {
		defaultConfig := DefaultMockClientConfig()
		config = &defaultConfig
	}

	mockServer := NewRedditMockServer()

	// Update config with mock server URL
	config.BaseURL = mockServer.URL()

	// Create Reddit client using constructor with mock server URLs
	grawConfig := &graw.Config{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Username:     "test_user",
		Password:     "test_pass",
		UserAgent:    config.UserAgent,
		BaseURL:      config.BaseURL,
		AuthURL:      config.BaseURL, // Use same URL for auth
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
	}

	client, err := graw.NewClient(grawConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to create reddit client: %v", err))
	}

	return &TestClient{
		Client:     client,
		mockServer: mockServer,
		config:     *config,
	}
}

// MockServer returns the underlying mock server
func (tc *TestClient) MockServer() *RedditMockServer {
	return tc.mockServer
}

// Close closes the test client and mock server
func (tc *TestClient) Close() {
	tc.mockServer.Close()
}

// Reset resets the mock server state
func (tc *TestClient) Reset() {
	tc.mockServer.ClearLog()
	tc.mockServer.handler.mutex.Lock()
	tc.mockServer.handler.callCount = make(map[string]int)
	tc.mockServer.handler.mutex.Unlock()
}

// WaitForRequests waits for a specific number of requests
func (tc *TestClient) WaitForRequests(count int, timeout time.Duration) error {
	return tc.mockServer.WaitForRequests(count, timeout)
}

// AssertRequestCount asserts request count for a path
func (tc *TestClient) AssertRequestCount(path string, expectedCount int) error {
	return tc.mockServer.AssertRequestCount(path, expectedCount)
}

// GetRequestLog returns the request log
func (tc *TestClient) GetRequestLog() []RequestEntry {
	return tc.mockServer.GetRequestLog()
}

// SetDelay sets response delay
func (tc *TestClient) SetDelay(delay time.Duration) {
	tc.mockServer.SetDelay(delay)
}

// SetErrorRate sets error rate
func (tc *TestClient) SetErrorRate(rate float64) {
	tc.mockServer.SetErrorRate(rate)
}

// SetupRateLimit configures rate limiting
func (tc *TestClient) SetupRateLimit(remaining, used int, resetTime time.Time) {
	tc.mockServer.SetupRateLimit(remaining, used, resetTime)
}

// SetupError configures error responses
func (tc *TestClient) SetupError(statusCode int, message string) {
	tc.mockServer.SetupError(statusCode, message)
}

// ConcurrentTestHelper helps with concurrent testing
type ConcurrentTestHelper struct {
	clients []*TestClient
	mu      sync.RWMutex
}

// NewConcurrentTestHelper creates a helper for concurrent testing
func NewConcurrentTestHelper(clientCount int) *ConcurrentTestHelper {
	helper := &ConcurrentTestHelper{
		clients: make([]*TestClient, clientCount),
	}

	for i := 0; i < clientCount; i++ {
		helper.clients[i] = NewTestClient(nil)
	}

	return helper
}

// GetClient returns a client by index
func (cth *ConcurrentTestHelper) GetClient(index int) *TestClient {
	cth.mu.RLock()
	defer cth.mu.RUnlock()

	if index < 0 || index >= len(cth.clients) {
		return nil
	}

	return cth.clients[index]
}

// GetAllClients returns all clients
func (cth *ConcurrentTestHelper) GetAllClients() []*TestClient {
	cth.mu.RLock()
	defer cth.mu.RUnlock()

	result := make([]*TestClient, len(cth.clients))
	copy(result, cth.clients)
	return result
}

// Close closes all clients
func (cth *ConcurrentTestHelper) Close() {
	cth.mu.Lock()
	defer cth.mu.Unlock()

	for _, client := range cth.clients {
		client.Close()
	}
}

// Reset resets all clients
func (cth *ConcurrentTestHelper) Reset() {
	cth.mu.Lock()
	defer cth.mu.Unlock()

	for _, client := range cth.clients {
		client.Reset()
	}
}

// RunConcurrentTest runs a test function concurrently with multiple clients
func (cth *ConcurrentTestHelper) RunConcurrentTest(testFunc func(*TestClient) error) []error {
	cth.mu.RLock()
	clients := make([]*TestClient, len(cth.clients))
	copy(clients, cth.clients)
	cth.mu.RUnlock()

	errors := make([]error, len(clients))
	var wg sync.WaitGroup
	var errorMu sync.Mutex

	for i, client := range clients {
		wg.Add(1)
		go func(index int, tc *TestClient) {
			defer wg.Done()
			if err := testFunc(tc); err != nil {
				errorMu.Lock()
				errors[index] = err
				errorMu.Unlock()
			}
		}(i, client)
	}

	wg.Wait()
	return errors
}

// PerformanceTestHelper helps with performance testing
type PerformanceTestHelper struct {
	client    *TestClient
	metrics   *PerformanceMetrics
	mu        sync.RWMutex
	startTime time.Time
	endTime   time.Time
}

// PerformanceMetrics tracks performance metrics
type PerformanceMetrics struct {
	RequestCount     int64
	TotalDuration    time.Duration
	AverageLatency   time.Duration
	MinLatency       time.Duration
	MaxLatency       time.Duration
	ErrorCount       int64
	BytesTransferred int64
	mu               sync.RWMutex
	latencies        []time.Duration
}

// NewPerformanceTestHelper creates a performance testing helper
func NewPerformanceTestHelper() *PerformanceTestHelper {
	client := NewTestClient(nil)

	return &PerformanceTestHelper{
		client: client,
		metrics: &PerformanceMetrics{
			MinLatency: time.Hour, // Initialize to a large value
			latencies:  make([]time.Duration, 0),
		},
	}
}

// Client returns the test client
func (pth *PerformanceTestHelper) Client() *TestClient {
	return pth.client
}

// StartMeasurement starts performance measurement
func (pth *PerformanceTestHelper) StartMeasurement() {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	pth.startTime = time.Now()
	pth.metrics.mu.Lock()
	pth.metrics.latencies = pth.metrics.latencies[:0] // Reset latencies
	pth.metrics.mu.Unlock()
}

// StopMeasurement stops performance measurement
func (pth *PerformanceTestHelper) StopMeasurement() {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	pth.endTime = time.Now()
	pth.metrics.TotalDuration = pth.endTime.Sub(pth.startTime)

	// Calculate average latency
	pth.metrics.mu.Lock()
	if len(pth.metrics.latencies) > 0 {
		var total time.Duration
		for _, latency := range pth.metrics.latencies {
			total += latency
		}
		pth.metrics.AverageLatency = total / time.Duration(len(pth.metrics.latencies))
	}
	pth.metrics.mu.Unlock()
}

// RecordRequest records a request for performance metrics
func (pth *PerformanceTestHelper) RecordRequest(latency time.Duration, bytes int64, err error) {
	pth.metrics.mu.Lock()
	defer pth.metrics.mu.Unlock()

	pth.metrics.RequestCount++
	pth.metrics.BytesTransferred += bytes

	if err != nil {
		pth.metrics.ErrorCount++
	}

	pth.metrics.latencies = append(pth.metrics.latencies, latency)

	if latency < pth.metrics.MinLatency {
		pth.metrics.MinLatency = latency
	}
	if latency > pth.metrics.MaxLatency {
		pth.metrics.MaxLatency = latency
	}
}

// GetMetrics returns a pointer to the current performance metrics
func (pth *PerformanceTestHelper) GetMetrics() *PerformanceMetrics {
	return pth.metrics
}

// Reset resets the performance metrics
func (pth *PerformanceTestHelper) Reset() {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	pth.metrics = &PerformanceMetrics{
		MinLatency: time.Hour,
		latencies:  make([]time.Duration, 0),
	}
}

// Close closes the performance test helper
func (pth *PerformanceTestHelper) Close() {
	pth.client.Close()
}

// RunLoadTest runs a load test with the specified configuration
func (pth *PerformanceTestHelper) RunLoadTest(ctx context.Context, concurrency int, duration time.Duration, testFunc func(*TestClient) error) (*PerformanceMetrics, error) {
	pth.Reset()
	pth.StartMeasurement()

	defer pth.StopMeasurement()

	// Create concurrent test helper
	concurrentHelper := NewConcurrentTestHelper(concurrency)
	defer concurrentHelper.Close()

	// Channel to coordinate workers
	workChan := make(chan struct{}, concurrency)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerIndex int) {
			defer wg.Done()
			client := concurrentHelper.GetClient(workerIndex)

			for {
				select {
				case <-ctx.Done():
					return
				case <-workChan:
					start := time.Now()
					err := testFunc(client)
					latency := time.Since(start)

					pth.RecordRequest(latency, 0, err)
				}
			}
		}(i)
	}

	// Send work signals
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	endTime := time.Now().Add(duration)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return pth.GetMetrics(), ctx.Err()
		case <-ticker.C:
			select {
			case workChan <- struct{}{}:
			default:
				// Channel full, skip this iteration
			}
		}
	}

	// Wait for all workers to finish current work
	close(workChan)
	wg.Wait()

	return pth.GetMetrics(), nil
}

// AuthTestHelper helps with authentication testing
type AuthTestHelper struct {
	client *TestClient
}

// NewAuthTestHelper creates an authentication testing helper
func NewAuthTestHelper() *AuthTestHelper {
	return &AuthTestHelper{
		client: NewTestClient(nil),
	}
}

// Client returns the test client
func (ath *AuthTestHelper) Client() *TestClient {
	return ath.client
}

// SetupValidAuth configures the mock server for valid authentication
func (ath *AuthTestHelper) SetupValidAuth() {
	ath.client.SetupRateLimit(100, 1, time.Now().Add(time.Hour))
}

// SetupExpiredToken configures the mock server for expired token scenario
func (ath *AuthTestHelper) SetupExpiredToken() {
	ath.client.SetupError(401, "invalid_grant: The provided authorization grant is invalid, expired, revoked, does not match the redirection URI used in the authorization request, or was issued to another client.")
}

// SetupRateLimited configures the mock server for rate limited scenario
func (ath *AuthTestHelper) SetupRateLimited() {
	ath.client.SetupError(429, "Too Many Requests")
	ath.client.SetupRateLimit(0, 60, time.Now().Add(time.Minute))
}

// Close closes the auth test helper
func (ath *AuthTestHelper) Close() {
	ath.client.Close()
}

// Utility functions for testing

// AssertNoError asserts that an error is nil
func AssertNoError(err error) error {
	if err != nil {
		return fmt.Errorf("expected no error, got: %v", err)
	}
	return nil
}

// AssertError asserts that an error is not nil
func AssertError(err error) error {
	if err == nil {
		return fmt.Errorf("expected error, got nil")
	}
	return nil
}

// AssertErrorContains asserts that an error contains specific text
func AssertErrorContains(err error, expected string) error {
	if err == nil {
		return fmt.Errorf("expected error containing '%s', got nil", expected)
	}
	if !contains(err.Error(), expected) {
		return fmt.Errorf("expected error containing '%s', got '%s'", expected, err.Error())
	}
	return nil
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && indexOf(s, substr) >= 0))
}

// indexOf finds the index of a substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
