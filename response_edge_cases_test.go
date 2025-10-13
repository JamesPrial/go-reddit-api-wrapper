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
	"github.com/jamesprial/go-reddit-api-wrapper/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// customResponseServer is a helper for creating test servers with custom response handlers.
// This allows full control over the HTTP response for testing edge cases.
type customResponseServer struct {
	server  *httptest.Server
	handler http.HandlerFunc
}

// newCustomResponseServer creates a new test server with a custom handler function.
// The handler receives the ResponseWriter and Request and can write any response.
func newCustomResponseServer(handler http.HandlerFunc) *customResponseServer {
	srv := &customResponseServer{
		handler: handler,
	}
	srv.server = httptest.NewServer(handler)
	return srv
}

// Close shuts down the test server.
func (s *customResponseServer) Close() {
	if s.server != nil {
		s.server.Close()
	}
}

// URL returns the base URL of the test server.
func (s *customResponseServer) URL() string {
	if s.server != nil {
		return s.server.URL
	}
	return ""
}

// TestMalformedJSONResponse tests handling of malformed JSON responses
func TestMalformedJSONResponse(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return malformed JSON (unterminated array)
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	// Test that malformed JSON is handled gracefully
	_, err = client.GetSubreddit(ctx, "testsub")
	testutil.AssertError(t, err)

	// Check if the error mentions parsing or JSON issues
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "JSON") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

// TestEmptyResponse tests handling of completely empty responses
func TestEmptyResponse(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return completely empty response
		w.Write([]byte(""))
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetSubreddit(ctx, "testsub")
	testutil.AssertError(t, err)
}

// TestUnexpectedResponseStructure tests handling of unexpected JSON structures
func TestUnexpectedResponseStructure(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
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
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetSubreddit(ctx, "testsub")
	testutil.AssertError(t, err)
}

// TestNullFieldsInResponse tests handling of null fields in otherwise valid responses
func TestNullFieldsInResponse(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return response with null fields
		responseWithNulls := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"id":                 "test123",
				"display_name":       "testsub",
				"subscribers":        nil,
				"created_utc":        nil,
				"public_description": "valid description",
				"over18":             false,
			},
		}
		json.NewEncoder(w).Encode(responseWithNulls)
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	testutil.AssertNoError(t, err)

	// Verify that null fields are handled gracefully (become zero values)
	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub' for display_name, got: %s", subreddit.DisplayName)
	}

	if subreddit.Subscribers != 0 {
		t.Errorf("Expected 0 subscribers for null field, got: %d", subreddit.Subscribers)
	}

	// Verify that valid fields are still parsed correctly
	if subreddit.PublicDescription != "valid description" {
		t.Errorf("Expected 'valid description', got: %s", subreddit.PublicDescription)
	}
}

// TestVeryLargeResponse tests handling of very large responses
func TestVeryLargeResponse(t *testing.T) {
	t.Parallel()

	// Create a very large response using builders
	posts := make([]*types.Post, 1000)
	for i := 0; i < 1000; i++ {
		posts[i] = testutil.NewPostBuilder().
			WithID(fmt.Sprintf("post_%d", i)).
			WithTitle(fmt.Sprintf("Very Long Title With Lots of Text to Make the Response Bigger %d", i)).
			WithScore(i).
			WithAuthor(fmt.Sprintf("user_%d", i)).
			WithSelfText(strings.Repeat("This is a very long selftext to make the response larger. ", 100)).
			WithCreated(1609459200.0 + float64(i)).
			WithNumComments(i).
			Build()
	}

	server := testutil.NewMockServer().
		WithPosts("largesub", "hot", posts...).
		Start()
	defer server.Close()

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

	// Measure parsing time
	start := time.Now()
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "largesub",
		Pagination: types.Pagination{
			Limit: 100,
		},
	})
	duration := time.Since(start)

	testutil.AssertNoError(t, err)
	testutil.AssertPostCount(t, resp, 1000)

	// Verify some data was parsed correctly
	if resp.Posts[0].Title == "" {
		t.Error("Expected post title to be parsed, but got empty string")
	}

	t.Logf("Successfully handled large response with %d posts in %v", len(resp.Posts), duration)
}

// TestUnicodeAndSpecialCharacters tests handling of unicode and special characters
func TestUnicodeAndSpecialCharacters(t *testing.T) {
	t.Parallel()

	// Create subreddit with unicode and special characters using builder
	subreddit := testutil.NewSubreddit("unicode_test").
		WithTitle("Tëst wïth üñïçødé ñð spëçïål chäräçtërs 🌟").
		WithDescription("描述 avec des caractères spéciaux: éàèùçñëüöäß").
		WithSubscribers(100000).
		Build()
	subreddit.PublicDescription = "Test with emojis: 🎉🎊🎈🎁 and math: ∑∏∫∆∇∂"

	server := testutil.NewMockServer().
		WithSubreddit("unicode_test", subreddit).
		Start()
	defer server.Close()

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

	result, err := client.GetSubreddit(ctx, "unicode_test")
	testutil.AssertNoError(t, err)

	// Verify unicode characters are preserved
	if result.DisplayName != "unicode_test" {
		t.Errorf("Expected 'unicode_test', got: %s", result.DisplayName)
	}

	if !strings.Contains(result.Title, "üñïçødé") {
		t.Errorf("Expected unicode characters in title, got: %s", result.Title)
	}

	if !strings.Contains(result.PublicDescription, "🎉") {
		t.Errorf("Expected emojis in description, got: %s", result.PublicDescription)
	}
}

// TestResponseWithExtraFields tests handling of responses with extra/unknown fields
func TestResponseWithExtraFields(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return response with extra fields that shouldn't break parsing
		responseWithExtras := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"id":                 "test123",
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
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	testutil.AssertNoError(t, err)

	// Verify known fields are parsed correctly
	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if subreddit.Subscribers != 100000 {
		t.Errorf("Expected 100000 subscribers, got: %d", subreddit.Subscribers)
	}
}

// TestResponseWithWrongTypes tests handling of responses with wrong data types
func TestResponseWithWrongTypes(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
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
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetSubreddit(ctx, "testsub")
	testutil.AssertError(t, err)

	// JSON type mismatches should cause parse errors
	// The parser cannot gracefully handle fundamental type mismatches
}

// TestPartialResponse tests handling of partial/incomplete responses
func TestPartialResponse(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return partial response with missing required fields
		partialResponse := map[string]interface{}{
			"kind": "t5",
			"data": map[string]interface{}{
				"id":                 "test123",
				"display_name": "testsub",
				// Missing subscribers, created_utc, etc.
				"public_description": "A test subreddit",
			},
		}
		json.NewEncoder(w).Encode(partialResponse)
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	testutil.AssertNoError(t, err)

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
}

// TestResponseWithNewlinesAndWhitespace tests handling of responses with unusual whitespace
func TestResponseWithNewlinesAndWhitespace(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return JSON with unusual whitespace formatting
		whitespaceResponse := `{
			"kind": "t5",
			"data": {
				"display_name": "testsub",
				"id": "test123",
				"subscribers": 100000,
				"public_description": "A test subreddit\nwith newlines\tand\ttabs",
				"created_utc": 1234567890.0,
				"over18": false
			}
		}`
		w.Write([]byte(whitespaceResponse))
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "testsub")
	testutil.AssertNoError(t, err)

	// Verify whitespace is handled correctly
	if subreddit.DisplayName != "testsub" {
		t.Errorf("Expected 'testsub', got: %s", subreddit.DisplayName)
	}

	if !strings.Contains(subreddit.PublicDescription, "with newlines") {
		t.Errorf("Expected newlines in description, got: %s", subreddit.PublicDescription)
	}
}

// TestResponseStreamError tests handling of response stream errors
func TestResponseStreamError(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Start writing JSON but cut off mid-stream
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [{"kind": "t3", "data": {"id": "post1"`))
		// Close connection abruptly
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}

	ctx := context.Background()

	_, err = client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "testsub",
	})
	testutil.AssertError(t, err)

	// Accept any error from stream interruption (unexpected EOF, connection reset, etc.)
	if !strings.Contains(err.Error(), "EOF") &&
		!strings.Contains(err.Error(), "connection") &&
		!strings.Contains(err.Error(), "read") &&
		!strings.Contains(err.Error(), "parse") {
		t.Errorf("Expected stream error (EOF/connection/read/parse), got: %v", err)
	}
}

// TestResponseWithInvalidContentType tests handling of responses with invalid content types
func TestResponseWithInvalidContentType(t *testing.T) {
	t.Parallel()

	customServer := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain") // Wrong content type
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind": "t5", "data": {"display_name": "testsub"}}`))
	})
	defer customServer.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := internal.NewClient(httpClient, customServer.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	client := &Reddit{
		httpClient: internalClient,
		parser:     internal.NewParser(),
		validator:  internal.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
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
