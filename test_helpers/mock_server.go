package test_helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// MockServer provides a configurable mock Reddit API server for testing
type MockServer struct {
	server     *httptest.Server
	handler    *MockHandler
	baseURL    string
	requestLog []RequestEntry
	logMutex   sync.Mutex
	callCount  map[string]int
	countMutex sync.Mutex
}

// RequestEntry logs incoming requests for debugging
type RequestEntry struct {
	Method       string
	Path         string
	Headers      http.Header
	Body         string
	Timestamp    time.Time
	ResponseCode int
}

// MockHandler handles mock API responses
type MockHandler struct {
	responses   map[string]*MockResponse
	defaultResp *MockResponse
	delay       time.Duration
	errorRate   float64
	callCount   map[string]int
	mutex       sync.RWMutex
}

// MockResponse defines a mock API response
type MockResponse struct {
	Status    int
	Body      string
	Headers   map[string]string
	Delay     time.Duration
	Error     error
	CallCount int
	MaxCalls  int // 0 = unlimited
}

// NewMockServer creates a new mock server instance
func NewMockServer() *MockServer {
	handler := &MockHandler{
		responses: make(map[string]*MockResponse),
		callCount: make(map[string]int),
		defaultResp: &MockResponse{
			Status: http.StatusOK,
			Body:   `{"message": "mock response"}`,
		},
	}

	server := httptest.NewServer(handler)

	return &MockServer{
		server:     server,
		handler:    handler,
		baseURL:    server.URL,
		requestLog: make([]RequestEntry, 0),
		callCount:  make(map[string]int),
	}
}

// URL returns the base URL of the mock server
func (ms *MockServer) URL() string {
	return ms.baseURL
}

// Close shuts down the mock server
func (ms *MockServer) Close() {
	ms.server.Close()
}

// SetResponse configures a response for a specific path
func (ms *MockServer) SetResponse(path string, response *MockResponse) {
	ms.handler.mutex.Lock()
	defer ms.handler.mutex.Unlock()
	ms.handler.responses[path] = response
}

// SetDefaultResponse configures the default response
func (ms *MockServer) SetDefaultResponse(response *MockResponse) {
	ms.handler.mutex.Lock()
	defer ms.handler.mutex.Unlock()
	ms.handler.defaultResp = response
}

// SetDelay adds delay to all responses
func (ms *MockServer) SetDelay(delay time.Duration) {
	ms.handler.mutex.Lock()
	defer ms.handler.mutex.Unlock()
	ms.handler.delay = delay
}

// SetErrorRate sets the error rate for responses (0.0 to 1.0)
func (ms *MockServer) SetErrorRate(rate float64) {
	ms.handler.mutex.Lock()
	defer ms.handler.mutex.Unlock()
	ms.handler.errorRate = rate
}

// GetRequestLog returns the request log
func (ms *MockServer) GetRequestLog() []RequestEntry {
	ms.logMutex.Lock()
	defer ms.logMutex.Unlock()
	return append([]RequestEntry{}, ms.requestLog...)
}

// GetCallCount returns the call count for a path
func (ms *MockServer) GetCallCount(path string) int {
	ms.countMutex.Lock()
	defer ms.countMutex.Unlock()
	return ms.callCount[path]
}

// ClearLog clears the request log
func (ms *MockServer) ClearLog() {
	ms.logMutex.Lock()
	defer ms.logMutex.Unlock()
	ms.requestLog = ms.requestLog[:0]

	ms.countMutex.Lock()
	defer ms.countMutex.Unlock()
	ms.callCount = make(map[string]int)
}

// ServeHTTP implements http.Handler
func (h *MockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Increment call count
	h.mutex.Lock()
	h.callCount[r.URL.Path]++
	h.mutex.Unlock()

	// Log request
	entry := RequestEntry{
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   r.Header,
		Timestamp: time.Now(),
	}

	// Read body if present
	if r.Body != nil {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		entry.Body = string(buf[:n])
	}

	// Find response for this path
	h.mutex.RLock()
	response, exists := h.responses[r.URL.Path]
	if !exists {
		response = h.defaultResp
	}
	h.mutex.RUnlock()

	// Check max calls limit
	if response.MaxCalls > 0 {
		h.mutex.Lock()
		if h.callCount[r.URL.Path] > response.MaxCalls {
			h.mutex.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.mutex.Unlock()
	}

	// Apply delay
	totalDelay := h.delay + response.Delay
	if totalDelay > 0 {
		time.Sleep(totalDelay)
	}

	// Check error rate
	if h.errorRate > 0 {
		if randFloat() < h.errorRate {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal server error"}`))
			entry.ResponseCode = http.StatusInternalServerError
			return
		}
	}

	// Set headers
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}

	// Handle error
	if response.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(response.Error.Error()))
		entry.ResponseCode = http.StatusInternalServerError
		return
	}

	// Send response
	w.WriteHeader(response.Status)
	w.Write([]byte(response.Body))
	entry.ResponseCode = response.Status

	// Log the completed request
	if mockServer := getMockServerForHandler(h); mockServer != nil {
		mockServer.logMutex.Lock()
		mockServer.requestLog = append(mockServer.requestLog, entry)
		mockServer.logMutex.Unlock()

		mockServer.countMutex.Lock()
		mockServer.callCount[r.URL.Path]++
		mockServer.countMutex.Unlock()
	}
}

// Helper function to get the mock server for a handler
var mockServers = make(map[*MockHandler]*MockServer)
var serversMutex sync.RWMutex

func getMockServerForHandler(h *MockHandler) *MockServer {
	serversMutex.RLock()
	defer serversMutex.RUnlock()
	return mockServers[h]
}

func init() {
	// This is a bit of a hack to link handlers back to their servers
	// In practice, we'll set this when creating the server
}

// RedditMockServer provides Reddit-specific mock responses
type RedditMockServer struct {
	*MockServer
}

// NewRedditMockServer creates a mock server pre-configured for Reddit API responses
func NewRedditMockServer() *RedditMockServer {
	baseServer := NewMockServer()

	// Link the handler back to the server for logging
	serversMutex.Lock()
	mockServers[baseServer.handler] = baseServer
	serversMutex.Unlock()

	server := &RedditMockServer{
		MockServer: baseServer,
	}

	// Setup default Reddit responses
	server.setupDefaultResponses()

	return server
}

// setupDefaultResponses configures common Reddit API endpoints
func (rms *RedditMockServer) setupDefaultResponses() {
	// OAuth token endpoint
	rms.SetResponse("/api/v1/access_token", &MockResponse{
		Status: http.StatusOK,
		Body:   `{"access_token":"mock_token","token_type":"bearer","expires_in":3600}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})

	// Subreddit info
	rms.SetResponse("/r/test/about.json", &MockResponse{
		Status: http.StatusOK,
		Body:   `{"data":{"display_name":"test","title":"Test Subreddit","subscribers":1000,"public_description":"A test subreddit"}}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})

	// Posts listing
	rms.SetResponse("/r/test/hot.json", &MockResponse{
		Status: http.StatusOK,
		Body:   `{"data":{"children":[{"data":{"id":"1","title":"Test Post","author":"testuser","score":100,"num_comments":50}}],"after":"t3_2"}}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})

	// Comments
	rms.SetResponse("/r/test/comments/1.json", &MockResponse{
		Status: http.StatusOK,
		Body:   `[{"data":{"children":[{"data":{"id":"1","title":"Test Post","author":"testuser","score":100,"num_comments":50}}]},{"data":{"children":[{"data":{"id":"c1","author":"commenter","body":"Test comment","score":10}}]}]`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})

	// Rate limit headers
	rms.SetDefaultResponse(&MockResponse{
		Status: http.StatusOK,
		Body:   `{"message": "success"}`,
		Headers: map[string]string{
			"X-RateLimit-Remaining": "99",
			"X-RateLimit-Used":      "1",
			"X-RateLimit-Reset":     strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10),
			"Content-Type":          "application/json",
		},
	})
}

// SetupSubreddit configures responses for a specific subreddit
func (rms *RedditMockServer) SetupSubreddit(name string, posts []types.Post, comments []types.Comment) {
	// Subreddit info
	subredditInfo := map[string]interface{}{
		"data": map[string]interface{}{
			"display_name":       name,
			"title":              name + " Subreddit",
			"subscribers":        len(posts) * 100,
			"public_description": "A test subreddit for " + name,
		},
	}

	infoBody, _ := json.Marshal(subredditInfo)
	rms.SetResponse("/r/"+name+"/about.json", &MockResponse{
		Status:  http.StatusOK,
		Body:    string(infoBody),
		Headers: map[string]string{"Content-Type": "application/json"},
	})

	// Posts listing
	postsData := map[string]interface{}{
		"data": map[string]interface{}{
			"children": make([]map[string]interface{}, len(posts)),
			"after":    "t3_next",
		},
	}

	for i, post := range posts {
		postsData["data"].(map[string]interface{})["children"].([]map[string]interface{})[i] = map[string]interface{}{
			"data": post,
		}
	}

	postsBody, _ := json.Marshal(postsData)
	rms.SetResponse("/r/"+name+"/hot.json", &MockResponse{
		Status:  http.StatusOK,
		Body:    string(postsBody),
		Headers: map[string]string{"Content-Type": "application/json"},
	})

	// Comments for first post
	if len(posts) > 0 {
		postID := posts[0].ID
		commentsData := make([]map[string]interface{}, 2)

		// Post data
		commentsData[0] = map[string]interface{}{
			"data": map[string]interface{}{
				"children": []map[string]interface{}{{"data": posts[0]}},
			},
		}

		// Comments data
		commentsList := make([]map[string]interface{}, len(comments))
		for i, comment := range comments {
			commentsList[i] = map[string]interface{}{"data": comment}
		}

		commentsData[1] = map[string]interface{}{
			"data": map[string]interface{}{
				"children": commentsList,
			},
		}

		commentsBody, _ := json.Marshal(commentsData)
		rms.SetResponse("/r/"+name+"/comments/"+postID+".json", &MockResponse{
			Status:  http.StatusOK,
			Body:    string(commentsBody),
			Headers: map[string]string{"Content-Type": "application/json"},
		})
	}
}

// SetupRateLimit configures rate limiting responses
func (rms *RedditMockServer) SetupRateLimit(remaining, used int, resetTime time.Time) {
	headers := map[string]string{
		"X-RateLimit-Remaining": strconv.Itoa(remaining),
		"X-RateLimit-Used":      strconv.Itoa(used),
		"X-RateLimit-Reset":     strconv.FormatInt(resetTime.Unix(), 10),
		"Content-Type":          "application/json",
	}

	rms.SetDefaultResponse(&MockResponse{
		Status:  http.StatusOK,
		Body:    `{"message": "success"}`,
		Headers: headers,
	})
}

// SetupError configures error responses
func (rms *RedditMockServer) SetupError(statusCode int, message string) {
	rms.SetDefaultResponse(&MockResponse{
		Status: statusCode,
		Body:   fmt.Sprintf(`{"error": "%s"}`, message),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	})
}

// Utility functions

func randFloat() float64 {
	return float64(time.Now().UnixNano()%1000) / 1000.0
}

// MockClientConfig provides configuration for mock clients
type MockClientConfig struct {
	BaseURL        string
	UserAgent      string
	Timeout        time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
	RateLimitDelay time.Duration
	AuthHeader     string
}

// DefaultMockClientConfig returns default configuration for mock clients
func DefaultMockClientConfig() MockClientConfig {
	return MockClientConfig{
		UserAgent:      "test-client/1.0",
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     100 * time.Millisecond,
		RateLimitDelay: 100 * time.Millisecond,
		AuthHeader:     "Bearer mock_token",
	}
}

// WaitForRequests waits for a specific number of requests to be made
func (ms *MockServer) WaitForRequests(count int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %d requests", count)
		case <-ticker.C:
			total := 0
			ms.countMutex.Lock()
			for _, c := range ms.callCount {
				total += c
			}
			ms.countMutex.Unlock()

			if total >= count {
				return nil
			}
		}
	}
}

// AssertRequestCount asserts that a specific number of requests were made to a path
func (ms *MockServer) AssertRequestCount(path string, expectedCount int) error {
	actualCount := ms.GetCallCount(path)
	if actualCount != expectedCount {
		return fmt.Errorf("expected %d requests to %s, got %d", expectedCount, path, actualCount)
	}
	return nil
}

// GetLastRequest returns the last request made to a specific path
func (ms *MockServer) GetLastRequest(path string) (*RequestEntry, error) {
	ms.logMutex.Lock()
	defer ms.logMutex.Unlock()

	for i := len(ms.requestLog) - 1; i >= 0; i-- {
		if ms.requestLog[i].Path == path {
			return &ms.requestLog[i], nil
		}
	}

	return nil, fmt.Errorf("no requests found for path: %s", path)
}
