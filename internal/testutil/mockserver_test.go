package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestMockServer_Posts(t *testing.T) {
	// Create mock posts
	post1 := &types.Post{
		ThingData: types.ThingData{
			ID:   "post1",
			Name: "t3_post1",
		},
		Title:     "Test Post 1",
		Subreddit: "golang",
		Author:    "testuser1",
	}

	post2 := &types.Post{
		ThingData: types.ThingData{
			ID:   "post2",
			Name: "t3_post2",
		},
		Title:     "Test Post 2",
		Subreddit: "golang",
		Author:    "testuser2",
	}

	// Setup mock server
	server := NewMockServer().
		WithPosts("golang", "hot", post1, post2).
		Start()
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL() + "/r/golang/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check headers
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", ct)
	}

	if remaining := resp.Header.Get("X-Ratelimit-Remaining"); remaining != "60" {
		t.Errorf("Expected X-Ratelimit-Remaining '60', got %q", remaining)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var listing map[string]interface{}
	if err := json.Unmarshal(body, &listing); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify listing structure
	if kind, ok := listing["kind"].(string); !ok || kind != "Listing" {
		t.Errorf("Expected kind 'Listing', got %v", listing["kind"])
	}

	data := listing["data"].(map[string]interface{})
	children := data["children"].([]interface{})

	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

func TestMockServer_Subreddit(t *testing.T) {
	// Create mock subreddit
	sub := &types.SubredditData{
		ThingData: types.ThingData{
			ID:   "golang123",
			Name: "t5_golang",
		},
		DisplayName:       "golang",
		Title:             "Go Programming Language",
		PublicDescription: "The Go programming language",
		Subscribers:       100000,
	}

	// Setup mock server
	server := NewMockServer().
		WithSubreddit("golang", sub).
		Start()
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL() + "/r/golang/about")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var thing map[string]interface{}
	if err := json.Unmarshal(body, &thing); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify thing structure
	if kind, ok := thing["kind"].(string); !ok || kind != "t5" {
		t.Errorf("Expected kind 't5', got %v", thing["kind"])
	}

	data := thing["data"].(map[string]interface{})
	if displayName, ok := data["display_name"].(string); !ok || displayName != "golang" {
		t.Errorf("Expected display_name 'golang', got %v", data["display_name"])
	}
}

func TestMockServer_Comments(t *testing.T) {
	// Create mock post
	post := &types.Post{
		ThingData: types.ThingData{
			ID:   "abc123",
			Name: "t3_abc123",
		},
		Title:     "Test Post",
		Subreddit: "golang",
		Author:    "testuser",
	}

	// Create mock comments
	comment1 := &types.Comment{
		ThingData: types.ThingData{
			ID:   "comment1",
			Name: "t1_comment1",
		},
		Body:      "Test comment 1",
		Author:    "commenter1",
		LinkID:    "t3_abc123",
		ParentID:  "t3_abc123",
		Subreddit: "golang",
	}

	comment2 := &types.Comment{
		ThingData: types.ThingData{
			ID:   "comment2",
			Name: "t1_comment2",
		},
		Body:      "Test comment 2",
		Author:    "commenter2",
		LinkID:    "t3_abc123",
		ParentID:  "t3_abc123",
		Subreddit: "golang",
	}

	// Setup mock server
	server := NewMockServer().
		WithComments("golang", "abc123", post, comment1, comment2).
		Start()
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL() + "/r/golang/comments/abc123")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response - should be an array [postListing, commentsListing]
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response []map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(response) != 2 {
		t.Fatalf("Expected 2 listings, got %d", len(response))
	}

	// Verify post listing
	postListing := response[0]
	if kind, ok := postListing["kind"].(string); !ok || kind != "Listing" {
		t.Errorf("Expected post listing kind 'Listing', got %v", postListing["kind"])
	}

	// Verify comments listing
	commentsListing := response[1]
	if kind, ok := commentsListing["kind"].(string); !ok || kind != "Listing" {
		t.Errorf("Expected comments listing kind 'Listing', got %v", commentsListing["kind"])
	}

	commentsData := commentsListing["data"].(map[string]interface{})
	children := commentsData["children"].([]interface{})

	if len(children) != 2 {
		t.Errorf("Expected 2 comments, got %d", len(children))
	}
}

func TestMockServer_Account(t *testing.T) {
	// Create mock account
	account := &types.AccountData{
		ThingData: types.ThingData{
			ID:   "user123",
			Name: "t2_user123",
		},
		CommentKarma: 5000,
		LinkKarma:    10000,
		IsFriend:     false,
		IsGold:       true,
	}

	// Setup mock server
	server := NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL() + "/api/v1/me")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var thing map[string]interface{}
	if err := json.Unmarshal(body, &thing); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify thing structure
	if kind, ok := thing["kind"].(string); !ok || kind != "t2" {
		t.Errorf("Expected kind 't2', got %v", thing["kind"])
	}

	data := thing["data"].(map[string]interface{})
	if commentKarma, ok := data["comment_karma"].(float64); !ok || commentKarma != 5000 {
		t.Errorf("Expected comment_karma 5000, got %v", data["comment_karma"])
	}
}

func TestMockServer_Error(t *testing.T) {
	// Setup mock server with error
	server := NewMockServer().
		WithError("/r/private", http.StatusForbidden, "Private subreddit").
		Start()
	defer server.Close()

	// Make request
	resp, err := http.Get(server.URL() + "/r/private/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify error response
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.StatusCode)
	}

	// Parse error response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var errorData map[string]interface{}
	if err := json.Unmarshal(body, &errorData); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if msg, ok := errorData["message"].(string); !ok || msg != "Private subreddit" {
		t.Errorf("Expected message 'Private subreddit', got %v", errorData["message"])
	}
}

func TestMockServer_EmptyListing(t *testing.T) {
	// Setup mock server with no data
	server := NewMockServer().Start()
	defer server.Close()

	// Make request to unconfigured endpoint
	resp, err := http.Get(server.URL() + "/r/unknown/hot")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var listing map[string]interface{}
	if err := json.Unmarshal(body, &listing); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify empty listing
	data := listing["data"].(map[string]interface{})
	children := data["children"].([]interface{})

	if len(children) != 0 {
		t.Errorf("Expected 0 children, got %d", len(children))
	}
}

func TestMockServer_URL(t *testing.T) {
	server := NewMockServer()

	// URL should be empty before Start()
	if url := server.URL(); url != "" {
		t.Errorf("Expected empty URL before Start(), got %q", url)
	}

	// Start the server
	server.Start()
	defer server.Close()

	// URL should be set after Start()
	if url := server.URL(); url == "" {
		t.Error("Expected non-empty URL after Start()")
	}
}
