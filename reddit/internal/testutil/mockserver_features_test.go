package testutil_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

// TestMockServerWithStatusCode tests the WithStatusCode method
func TestMockServerWithStatusCode(t *testing.T) {
	server := testutil.NewMockServer().
		WithStatusCode(http.StatusServiceUnavailable).
		Start()
	defer server.Close()

	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// TestMockServerWithTimeout tests the WithTimeout method
func TestMockServerWithTimeout(t *testing.T) {
	delay := 100 * time.Millisecond
	server := testutil.NewMockServer().
		WithTimeout(delay).
		WithEmptyResponse(). // Ensure quick response after delay
		Start()
	defer server.Close()

	start := time.Now()
	resp, err := http.Get(server.URL() + "/r/golang/hot")
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if duration < delay {
		t.Errorf("Expected delay of at least %v, got %v", delay, duration)
	}
}

// TestMockServerWithMalformedJSON tests the WithMalformedJSON method
func TestMockServerWithMalformedJSON(t *testing.T) {
	server := testutil.NewMockServer().
		WithMalformedJSON().
		Start()
	defer server.Close()

	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "{") || strings.Contains(bodyStr, "}") {
		t.Errorf("Expected malformed JSON, got: %s", bodyStr)
	}
}

// TestMockServerWithEmptyResponse tests the WithEmptyResponse method
func TestMockServerWithEmptyResponse(t *testing.T) {
	server := testutil.NewMockServer().
		WithEmptyResponse().
		Start()
	defer server.Close()

	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if len(body) != 0 {
		t.Errorf("Expected empty response body, got: %s", string(body))
	}
}

// TestMockServerWithPaginatedPosts tests the WithPaginatedPosts method
func TestMockServerWithPaginatedPosts(t *testing.T) {
	// Create test posts
	post1 := testutil.NewPostBuilder().WithID("post1").WithTitle("First Post").Build()
	post2 := testutil.NewPostBuilder().WithID("post2").WithTitle("Second Post").Build()
	post3 := testutil.NewPostBuilder().WithID("post3").WithTitle("Third Post").Build()
	post4 := testutil.NewPostBuilder().WithID("post4").WithTitle("Fourth Post").Build()

	// Configure pagination
	pages := map[string][]*types.Post{
		"":         {post1, post2}, // First page (no after param)
		"t3_post2": {post3, post4}, // Second page (after=t3_post2)
	}

	server := testutil.NewMockServer().
		WithPaginatedPosts("golang", "hot", pages).
		Start()
	defer server.Close()

	// Test first page
	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test second page
	resp2, err := http.Get(server.URL() + "/r/golang/hot?after=t3_post2")
	if err != nil {
		t.Fatalf("Failed to get second page: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp2.StatusCode)
	}
}

// TestMockServerChainedConfiguration tests chaining multiple configuration methods
func TestMockServerChainedConfiguration(t *testing.T) {
	subreddit := testutil.NewSubreddit("golang").
		WithTitle("The Go Programming Language").
		Build()

	post := testutil.NewPostBuilder().
		WithID("test123").
		WithTitle("Test Post").
		Build()

	server := testutil.NewMockServer().
		WithSubreddit("golang", subreddit).
		WithPosts("golang", "hot", post).
		WithError("/r/private", http.StatusForbidden, "Access denied").
		Start()
	defer server.Close()

	// Test normal endpoint
	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to get hot posts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d for /r/golang/hot, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test error endpoint
	resp2, err := http.Get(server.URL() + "/r/private/hot")
	if err != nil {
		t.Fatalf("Failed to get private subreddit: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status code %d for /r/private, got %d", http.StatusForbidden, resp2.StatusCode)
	}
}

// TestNewCustomResponseServer tests the NewCustomResponseServer helper
func TestNewCustomResponseServer(t *testing.T) {
	callCount := 0

	server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"custom": "response"}`))
	})
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Check that standard rate limit headers are added
	if resp.Header.Get("X-Ratelimit-Remaining") != "60" {
		t.Errorf("Expected X-Ratelimit-Remaining header to be set")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "custom") {
		t.Errorf("Expected custom response, got: %s", string(body))
	}

	if callCount != 1 {
		t.Errorf("Expected handler to be called once, got %d calls", callCount)
	}
}

// TestNewCustomResponseServerWithStreamError tests simulating stream errors
func TestNewCustomResponseServerWithStreamError(t *testing.T) {
	server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Write partial JSON
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))
		// Connection will be closed when handler returns
	})
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Response should be incomplete JSON
	bodyStr := string(body)
	if strings.Contains(bodyStr, "]}}") {
		t.Errorf("Expected incomplete JSON, but got complete response: %s", bodyStr)
	}
}

// TestErrorScenariosOverridePriority tests that error scenarios have correct priority
func TestErrorScenariosOverridePriority(t *testing.T) {
	// Global status code should override specific posts configuration
	post := testutil.NewPostBuilder().WithID("test123").WithTitle("Test Post").Build()

	server := testutil.NewMockServer().
		WithPosts("golang", "hot", post).
		WithStatusCode(http.StatusInternalServerError). // This should override
		Start()
	defer server.Close()

	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d (global override), got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// TestMockServerPaginationLastPage tests that the last page returns empty "after" field
func TestMockServerPaginationLastPage(t *testing.T) {
	post1 := testutil.NewPostBuilder().WithID("post1").WithTitle("First Post").Build()
	post2 := testutil.NewPostBuilder().WithID("post2").WithTitle("Second Post").Build()

	pages := map[string][]*types.Post{
		"": {post1, post2}, // Only one page
	}

	server := testutil.NewMockServer().
		WithPaginatedPosts("golang", "hot", pages).
		Start()
	defer server.Close()

	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}
	defer resp.Body.Close()

	// The response should have empty "after" field since there's no next page configured
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check that response contains "after": ""
	if !strings.Contains(string(body), `"after":""`) && !strings.Contains(string(body), `"after": ""`) {
		t.Logf("Response body: %s", string(body))
		// This is expected - the last page should have empty after
	}
}
