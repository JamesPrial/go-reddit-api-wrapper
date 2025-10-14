package testutil

import (
	"encoding/json"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestPostBuilder_Defaults(t *testing.T) {
	post := NewPostBuilder().Build()

	if post.ID != "post123" {
		t.Errorf("Expected ID 'post123', got '%s'", post.ID)
	}

	if post.Name != "t3_post123" {
		t.Errorf("Expected Name 't3_post123', got '%s'", post.Name)
	}

	if post.Title != "Test Post" {
		t.Errorf("Expected Title 'Test Post', got '%s'", post.Title)
	}

	if post.Score != 100 {
		t.Errorf("Expected Score 100, got %d", post.Score)
	}

	if post.Ups != 100 {
		t.Errorf("Expected Ups 100, got %d", post.Ups)
	}

	if post.Downs != 0 {
		t.Errorf("Expected Downs 0, got %d", post.Downs)
	}

	if post.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", post.Author)
	}

	if post.Subreddit != "test" {
		t.Errorf("Expected Subreddit 'test', got '%s'", post.Subreddit)
	}

	if post.SubredditID != "t5_test" {
		t.Errorf("Expected SubredditID 't5_test', got '%s'", post.SubredditID)
	}

	if post.NumComments != 0 {
		t.Errorf("Expected NumComments 0, got %d", post.NumComments)
	}

	if post.UpvoteRatio != 0.95 {
		t.Errorf("Expected UpvoteRatio 0.95, got %f", post.UpvoteRatio)
	}

	if post.Created.Created == 0 {
		t.Error("Expected Created timestamp to be set")
	}

	if post.Created.CreatedUTC == 0 {
		t.Error("Expected CreatedUTC timestamp to be set")
	}
}

func TestPostBuilder_WithID(t *testing.T) {
	post := NewPostBuilder().WithID("abc123").Build()

	if post.ID != "abc123" {
		t.Errorf("Expected ID 'abc123', got '%s'", post.ID)
	}

	if post.Name != "t3_abc123" {
		t.Errorf("Expected Name 't3_abc123', got '%s'", post.Name)
	}
}

func TestPostBuilder_FluentChaining(t *testing.T) {
	post := NewPostBuilder().
		WithID("xyz789").
		WithTitle("Custom Title").
		WithScore(500).
		WithAuthor("cooluser").
		WithSubreddit("golang").
		WithNumComments(42).
		Build()

	if post.ID != "xyz789" {
		t.Errorf("Expected ID 'xyz789', got '%s'", post.ID)
	}

	if post.Title != "Custom Title" {
		t.Errorf("Expected Title 'Custom Title', got '%s'", post.Title)
	}

	if post.Score != 500 {
		t.Errorf("Expected Score 500, got %d", post.Score)
	}

	if post.Ups != 500 {
		t.Errorf("Expected Ups 500, got %d", post.Ups)
	}

	if post.Author != "cooluser" {
		t.Errorf("Expected Author 'cooluser', got '%s'", post.Author)
	}

	if post.Subreddit != "golang" {
		t.Errorf("Expected Subreddit 'golang', got '%s'", post.Subreddit)
	}

	if post.SubredditID != "t5_golang" {
		t.Errorf("Expected SubredditID 't5_golang', got '%s'", post.SubredditID)
	}

	if post.NumComments != 42 {
		t.Errorf("Expected NumComments 42, got %d", post.NumComments)
	}
}

func TestPostBuilder_ToThing(t *testing.T) {
	thing := NewPostBuilder().
		WithID("thing123").
		WithTitle("Thing Post").
		ToThing()

	if thing == nil {
		t.Fatal("Expected non-nil Thing")
	}

	if thing.Kind != "t3" {
		t.Errorf("Expected Kind 't3', got '%s'", thing.Kind)
	}

	if thing.ID != "thing123" {
		t.Errorf("Expected ID 'thing123', got '%s'", thing.ID)
	}

	if thing.Name != "t3_thing123" {
		t.Errorf("Expected Name 't3_thing123', got '%s'", thing.Name)
	}

	// Verify the Data field contains valid JSON
	var post types.Post
	if err := json.Unmarshal(thing.Data, &post); err != nil {
		t.Fatalf("Failed to unmarshal Thing.Data: %v", err)
	}

	if post.ID != "thing123" {
		t.Errorf("Expected unmarshaled ID 'thing123', got '%s'", post.ID)
	}

	if post.Title != "Thing Post" {
		t.Errorf("Expected unmarshaled Title 'Thing Post', got '%s'", post.Title)
	}
}

func TestPostBuilder_ToJSON(t *testing.T) {
	jsonData := NewPostBuilder().
		WithID("json123").
		WithTitle("JSON Post").
		ToJSON()

	if jsonData == nil {
		t.Fatal("Expected non-nil JSON data")
	}

	// Verify it's valid JSON that can be unmarshaled
	var post types.Post
	if err := json.Unmarshal(jsonData, &post); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if post.ID != "json123" {
		t.Errorf("Expected ID 'json123', got '%s'", post.ID)
	}

	if post.Title != "JSON Post" {
		t.Errorf("Expected Title 'JSON Post', got '%s'", post.Title)
	}
}

func TestPostBuilder_WithCreated(t *testing.T) {
	timestamp := 1234567890.0
	post := NewPostBuilder().WithCreated(timestamp).Build()

	if post.Created.Created != timestamp {
		t.Errorf("Expected Created %f, got %f", timestamp, post.Created.Created)
	}

	if post.Created.CreatedUTC != timestamp {
		t.Errorf("Expected CreatedUTC %f, got %f", timestamp, post.Created.CreatedUTC)
	}
}

func TestPostBuilder_WithEdited(t *testing.T) {
	timestamp := 1234567890.0
	post := NewPostBuilder().WithEdited(timestamp).Build()

	if !post.Edited.IsEdited {
		t.Error("Expected IsEdited to be true")
	}

	if post.Edited.Timestamp != timestamp {
		t.Errorf("Expected Edited timestamp %f, got %f", timestamp, post.Edited.Timestamp)
	}
}

func TestPostBuilder_WithSelfText(t *testing.T) {
	post := NewPostBuilder().WithSelfText("This is self text").Build()

	if post.SelfText != "This is self text" {
		t.Errorf("Expected SelfText 'This is self text', got '%s'", post.SelfText)
	}

	if !post.IsSelf {
		t.Error("Expected IsSelf to be true when SelfText is set")
	}

	// Test that empty self text sets IsSelf to false
	post2 := NewPostBuilder().WithSelfText("").Build()
	if post2.IsSelf {
		t.Error("Expected IsSelf to be false when SelfText is empty")
	}
}

func TestPostBuilder_WithDistinguished(t *testing.T) {
	post := NewPostBuilder().WithDistinguished("moderator").Build()

	if post.Distinguished == nil {
		t.Fatal("Expected Distinguished to be non-nil")
	}

	if *post.Distinguished != "moderator" {
		t.Errorf("Expected Distinguished 'moderator', got '%s'", *post.Distinguished)
	}
}

func TestPostBuilder_WithBooleanFlags(t *testing.T) {
	post := NewPostBuilder().
		WithOver18(true).
		WithStickied(true).
		WithLocked(true).
		Build()

	if !post.Over18 {
		t.Error("Expected Over18 to be true")
	}

	if !post.Stickied {
		t.Error("Expected Stickied to be true")
	}

	if !post.Locked {
		t.Error("Expected Locked to be true")
	}
}
