package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestMalformedJSONResponse tests handling of malformed JSON responses
func TestMalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return malformed JSON
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test that malformed JSON is handled gracefully
	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected error for malformed JSON, but got none")
	}

	// Check if the error is a parse error
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "JSON") {
		t.Errorf("Expected parse error, got: %v", err)
	}

	t.Logf("Successfully handled malformed JSON response: %v", err)
}

// TestEmptyResponse tests handling of completely empty responses
func TestEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return completely empty response
		w.Write([]byte(""))
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected error for empty response, but got none")
	}

	t.Logf("Successfully handled empty response: %v", err)
}

// TestUnexpectedResponseStructure tests handling of unexpected JSON structures
func TestUnexpectedResponseStructure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return valid JSON but unexpected structure
		unexpectedStruct := map[string]interface{}{
			"unexpected_field": "value",
			"nested": map[string]interface{}{
				"wrong_structure": []string{"item1", "item2"},
			},
		}
		json.NewEncoder(w).Encode(unexpectedStruct)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetSubreddit(ctx, "testsub")
	if err == nil {
		t.Error("Expected error for unexpected response structure, but got none")
	}

	t.Logf("Successfully handled unexpected response structure: %v", err)
}

// TestNullFieldsInResponse tests handling of null fields in otherwise valid responses
func TestNullFieldsInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return response with null fields
		responseWithNulls := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name":       nil,
				"subscribers":        nil,
				"created_utc":        nil,
				"public_description": "valid description",
				"over18":             false,
			},
		}
		json.NewEncoder(w).Encode(responseWithNulls)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Unexpected error handling null fields: %v", err)
	}

	// Verify that null fields are handled gracefully
	if subreddit.DisplayName != "" {
		t.Errorf("Expected empty display name for null field, got: %s", subreddit.DisplayName)
	}

	if subreddit.Subscribers != 0 {
		t.Errorf("Expected 0 subscribers for null field, got: %d", subreddit.Subscribers)
	}

	// Verify that valid fields are still parsed correctly
	if subreddit.PublicDescription != "valid description" {
		t.Errorf("Expected 'valid description', got: %s", subreddit.PublicDescription)
	}

	t.Logf("Successfully handled null fields in response")
}

// TestVeryLargeResponse tests handling of very large responses
func TestVeryLargeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Create a very large response
		posts := make([]map[string]interface{}, 1000)
		for i := 0; i < 1000; i++ {
			posts[i] = map[string]interface{}{
				"kind": "t3",
				"data": map[string]interface{}{
					"id":           fmt.Sprintf("post_%d", i),
					"title":        fmt.Sprintf("Very Long Title With Lots of Text to Make the Response Bigger %d", i),
					"score":        i,
					"author":       fmt.Sprintf("user_%d", i),
					"selftext":     strings.Repeat("This is a very long selftext to make the response larger. ", 100),
					"created_utc":  1609459200.0 + float64(i),
					"num_comments": i,
				},
			}
		}

		largeResponse := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": posts,
				"after":    "",
				"before":   "",
			},
		}
		json.NewEncoder(w).Encode(largeResponse)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Measure parsing time
	start := time.Now()
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "largesub",
		Pagination: types.Pagination{
			Limit: 1000,
		},
	})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to handle large response: %v", err)
	}

	if len(resp.Posts) != 1000 {
		t.Errorf("Expected 1000 posts, got %d", len(resp.Posts))
	}

	// Verify some data was parsed correctly
	if resp.Posts[0].Title == "" {
		t.Error("Expected post title to be parsed, but got empty string")
	}

	t.Logf("Successfully handled large response with %d posts in %v", len(resp.Posts), duration)
}

// TestUnicodeAndSpecialCharacters tests handling of unicode and special characters
func TestUnicodeAndSpecialCharacters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		// Return response with unicode and special characters
		unicodeResponse := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name":       "测试🚀",
				"title":              "Tëst wïth üñïçødé ñð spëçïål chäräçtërs 🌟",
				"description":        "描述 avec des caractères spéciaux: éàèùçñëüöäß",
				"public_description": "Test with emojis: 🎉🎊🎈🎁 and math: ∑∏∫∆∇∂",
				"subscribers":        100000,
				"over18":             false,
			},
		}
		json.NewEncoder(w).Encode(unicodeResponse)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "unicode_test")
	if err != nil {
		t.Fatalf("Failed to handle unicode characters: %v", err)
	}

	// Verify unicode characters are preserved
	if subreddit.DisplayName != "测试🚀" {
		t.Errorf("Expected '测试🚀', got: %s", subreddit.DisplayName)
	}

	if !strings.Contains(subreddit.Title, "üñïçødé") {
		t.Errorf("Expected unicode characters in title, got: %s", subreddit.Title)
	}

	if !strings.Contains(subreddit.PublicDescription, "🎉") {
		t.Errorf("Expected emojis in description, got: %s", subreddit.PublicDescription)
	}

	t.Logf("Successfully handled unicode and special characters")
}

// TestResponseWithExtraFields tests handling of responses with extra/unknown fields
func TestResponseWithExtraFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return response with extra fields that shouldn't break parsing
		responseWithExtras := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name":       "testsub",
				"subscribers":        100000,
				"created_utc":        1234567890.0,
				"public_description": "A test subreddit",
				"unknown_field1":     "should be ignored",
				"unknown_field2":     42,
				"nested_unknown": map[string]interface{}{
					"field1": "value1",
					"field2": []string{"a", "b", "c"},
				},
				"over18": false,
			},
		}
		json.NewEncoder(w).Encode(responseWithExtras)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Failed to handle response with extra fields: %v", err)
	}

	// Verify known fields are parsed correctly
	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if subreddit.Subscribers != 100000 {
		t.Errorf("Expected 100000 subscribers, got: %d", subreddit.Subscribers)
	}

	t.Logf("Successfully handled response with extra fields")
}

// TestResponseWithWrongTypes tests handling of responses with wrong data types
func TestResponseWithWrongTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return response with wrong data types
		responseWithWrongTypes := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name":       123,            // Should be string
				"subscribers":        "100000",       // Should be number
				"created_utc":        "1234567890.0", // Should be number
				"public_description": "A test subreddit",
				"over18":             "false", // Should be boolean
			},
		}
		json.NewEncoder(w).Encode(responseWithWrongTypes)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Failed to handle response with wrong types: %v", err)
	}

	// Verify that type conversion was attempted or handled gracefully
	// The exact behavior depends on the parser implementation
	if subreddit.PublicDescription != "A test subreddit" {
		t.Errorf("Expected 'A test subreddit', got: %s", subreddit.PublicDescription)
	}

	t.Logf("Successfully handled response with wrong data types")
}

// TestPartialResponse tests handling of partial/incomplete responses
func TestPartialResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return partial response with missing required fields
		partialResponse := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"display_name": "testsub",
				// Missing subscribers, created_utc, etc.
				"public_description": "A test subreddit",
			},
		}
		json.NewEncoder(w).Encode(partialResponse)
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Failed to handle partial response: %v", err)
	}

	// Verify that available fields are parsed
	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if subreddit.PublicDescription != "A test subreddit" {
		t.Errorf("Expected 'A test subreddit', got: %s", subreddit.PublicDescription)
	}

	// Missing fields should have default/zero values
	if subreddit.Subscribers != 0 {
		t.Errorf("Expected 0 subscribers for missing field, got: %d", subreddit.Subscribers)
	}

	t.Logf("Successfully handled partial response")
}

// TestResponseWithNewlinesAndWhitespace tests handling of responses with unusual whitespace
func TestResponseWithNewlinesAndWhitespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return JSON with unusual whitespace formatting
		whitespaceResponse := `{
			"kind": "t5",
			"data": {
				"display_name": "testsub",
				"subscribers": 100000,
				"public_description": "A test subreddit\nwith newlines\tand\ttabs",
				"created_utc": 1234567890.0,
				"over18": false
			}
		}`
		w.Write([]byte(whitespaceResponse))
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		t.Fatalf("Failed to handle response with unusual whitespace: %v", err)
	}

	// Verify whitespace is handled correctly
	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if !strings.Contains(subreddit.PublicDescription, "with newlines") {
		t.Errorf("Expected newlines in description, got: %s", subreddit.PublicDescription)
	}

	t.Logf("Successfully handled response with unusual whitespace")
}

// TestResponseStreamError tests handling of response stream errors
func TestResponseStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Start writing JSON but cut off mid-stream
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [{"kind": "t3", "data": {"id": "post1"`))
		// Close connection abruptly
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
	})
	if err == nil {
		t.Error("Expected error for stream interruption, but got none")
	}

	// Check if it's a connection/read error
	if !strings.Contains(err.Error(), "connection") &&
		!strings.Contains(err.Error(), "read") &&
		!strings.Contains(err.Error(), "parse") {
		t.Errorf("Expected connection/read/parse error, got: %v", err)
	}

	t.Logf("Successfully handled response stream error: %v", err)
}

// TestResponseWithInvalidContentType tests handling of responses with invalid content types
func TestResponseWithInvalidContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain") // Wrong content type
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind": "t5", "data": {"display_name": "testsub"}}`))
	}))
	defer server.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create internal httpClient: %v", err)
	}

	client := &Reddit{
		httpClient:    internalClient,
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
		auth:      &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// The client should still attempt to parse the response
	subreddit, err := client.GetSubreddit(ctx, "testsub")
	if err != nil {
		// Some parsers might reject wrong content type, which is acceptable
		t.Logf("Parser rejected wrong content type (acceptable): %v", err)
	} else {
		// If it succeeds, verify the data was parsed
		if subreddit.DisplayName != "testsub" {
			t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
		}
		t.Logf("Parser handled wrong content type gracefully")
	}
}
