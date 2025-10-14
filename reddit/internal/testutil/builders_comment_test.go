package testutil

import (
	"encoding/json"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestCommentBuilder_Defaults(t *testing.T) {
	comment := NewCommentBuilder().Build()

	if comment.ID != "comment123" {
		t.Errorf("Expected ID 'comment123', got '%s'", comment.ID)
	}

	if comment.Name != "t1_comment123" {
		t.Errorf("Expected Name 't1_comment123', got '%s'", comment.Name)
	}

	if comment.Body != "Test comment" {
		t.Errorf("Expected Body 'Test comment', got '%s'", comment.Body)
	}

	if comment.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", comment.Author)
	}

	if comment.Score != 10 {
		t.Errorf("Expected Score 10, got %d", comment.Score)
	}

	if comment.Ups != 10 {
		t.Errorf("Expected Ups 10, got %d", comment.Ups)
	}

	if comment.Downs != 0 {
		t.Errorf("Expected Downs 0, got %d", comment.Downs)
	}

	if comment.LinkID != "t3_post123" {
		t.Errorf("Expected LinkID 't3_post123', got '%s'", comment.LinkID)
	}

	if comment.ParentID != "t3_post123" {
		t.Errorf("Expected ParentID 't3_post123', got '%s'", comment.ParentID)
	}

	if comment.Subreddit != "test" {
		t.Errorf("Expected Subreddit 'test', got '%s'", comment.Subreddit)
	}

	if comment.SubredditID != "t5_test" {
		t.Errorf("Expected SubredditID 't5_test', got '%s'", comment.SubredditID)
	}

	if comment.Created.Created == 0 {
		t.Error("Expected Created timestamp to be set")
	}

	if comment.Created.CreatedUTC == 0 {
		t.Error("Expected CreatedUTC timestamp to be set")
	}

	if len(comment.Replies) != 0 {
		t.Errorf("Expected 0 replies, got %d", len(comment.Replies))
	}
}

func TestCommentBuilder_WithID(t *testing.T) {
	comment := NewCommentBuilder().WithID("abc123").Build()

	if comment.ID != "abc123" {
		t.Errorf("Expected ID 'abc123', got '%s'", comment.ID)
	}

	if comment.Name != "t1_abc123" {
		t.Errorf("Expected Name 't1_abc123', got '%s'", comment.Name)
	}
}

func TestCommentBuilder_FluentChaining(t *testing.T) {
	comment := NewCommentBuilder().
		WithID("xyz789").
		WithBody("Custom comment body").
		WithAuthor("cooluser").
		WithScore(50).
		WithSubreddit("golang").
		WithLinkID("t3_abc123").
		WithParentID("t3_abc123").
		Build()

	if comment.ID != "xyz789" {
		t.Errorf("Expected ID 'xyz789', got '%s'", comment.ID)
	}

	if comment.Body != "Custom comment body" {
		t.Errorf("Expected Body 'Custom comment body', got '%s'", comment.Body)
	}

	if comment.Author != "cooluser" {
		t.Errorf("Expected Author 'cooluser', got '%s'", comment.Author)
	}

	if comment.Score != 50 {
		t.Errorf("Expected Score 50, got %d", comment.Score)
	}

	if comment.Ups != 50 {
		t.Errorf("Expected Ups 50, got %d", comment.Ups)
	}

	if comment.Subreddit != "golang" {
		t.Errorf("Expected Subreddit 'golang', got '%s'", comment.Subreddit)
	}

	if comment.SubredditID != "t5_golang" {
		t.Errorf("Expected SubredditID 't5_golang', got '%s'", comment.SubredditID)
	}

	if comment.LinkID != "t3_abc123" {
		t.Errorf("Expected LinkID 't3_abc123', got '%s'", comment.LinkID)
	}

	if comment.ParentID != "t3_abc123" {
		t.Errorf("Expected ParentID 't3_abc123', got '%s'", comment.ParentID)
	}
}

func TestCommentBuilder_WithParentPost(t *testing.T) {
	comment := NewCommentBuilder().WithParentPost("xyz789").Build()

	if comment.LinkID != "t3_xyz789" {
		t.Errorf("Expected LinkID 't3_xyz789', got '%s'", comment.LinkID)
	}

	if comment.ParentID != "t3_xyz789" {
		t.Errorf("Expected ParentID 't3_xyz789', got '%s'", comment.ParentID)
	}
}

func TestCommentBuilder_WithReplies(t *testing.T) {
	reply1 := NewCommentBuilder().
		WithID("reply1").
		WithBody("First reply").
		Build()

	reply2 := NewCommentBuilder().
		WithID("reply2").
		WithBody("Second reply").
		Build()

	parent := NewCommentBuilder().
		WithID("parent").
		WithReplies(reply1, reply2).
		Build()

	if len(parent.Replies) != 2 {
		t.Fatalf("Expected 2 replies, got %d", len(parent.Replies))
	}

	if parent.Replies[0].ID != "reply1" {
		t.Errorf("Expected first reply ID 'reply1', got '%s'", parent.Replies[0].ID)
	}

	if parent.Replies[1].ID != "reply2" {
		t.Errorf("Expected second reply ID 'reply2', got '%s'", parent.Replies[1].ID)
	}
}

func TestCommentBuilder_WithReply(t *testing.T) {
	parent := NewCommentBuilder().WithID("parent").Build()

	// Should start with no replies
	if len(parent.Replies) != 0 {
		t.Errorf("Expected 0 initial replies, got %d", len(parent.Replies))
	}

	reply1 := NewCommentBuilder().WithID("reply1").Build()
	reply2 := NewCommentBuilder().WithID("reply2").Build()

	// Build a new parent with replies added one at a time
	parent = NewCommentBuilder().
		WithID("parent").
		WithReply(reply1).
		WithReply(reply2).
		Build()

	if len(parent.Replies) != 2 {
		t.Fatalf("Expected 2 replies, got %d", len(parent.Replies))
	}

	if parent.Replies[0].ID != "reply1" {
		t.Errorf("Expected first reply ID 'reply1', got '%s'", parent.Replies[0].ID)
	}

	if parent.Replies[1].ID != "reply2" {
		t.Errorf("Expected second reply ID 'reply2', got '%s'", parent.Replies[1].ID)
	}
}

func TestCommentBuilder_ToThing(t *testing.T) {
	thing := NewCommentBuilder().
		WithID("thing123").
		WithBody("Thing comment").
		ToThing()

	if thing == nil {
		t.Fatal("Expected non-nil Thing")
	}

	if thing.Kind != "t1" {
		t.Errorf("Expected Kind 't1', got '%s'", thing.Kind)
	}

	if thing.ID != "thing123" {
		t.Errorf("Expected ID 'thing123', got '%s'", thing.ID)
	}

	if thing.Name != "t1_thing123" {
		t.Errorf("Expected Name 't1_thing123', got '%s'", thing.Name)
	}

	// Verify the Data field contains valid JSON
	var comment types.Comment
	if err := json.Unmarshal(thing.Data, &comment); err != nil {
		t.Fatalf("Failed to unmarshal Thing.Data: %v", err)
	}

	if comment.ID != "thing123" {
		t.Errorf("Expected unmarshaled ID 'thing123', got '%s'", comment.ID)
	}

	if comment.Body != "Thing comment" {
		t.Errorf("Expected unmarshaled Body 'Thing comment', got '%s'", comment.Body)
	}
}

func TestCommentBuilder_ToJSON(t *testing.T) {
	jsonData := NewCommentBuilder().
		WithID("json123").
		WithBody("JSON comment").
		ToJSON()

	if jsonData == nil {
		t.Fatal("Expected non-nil JSON data")
	}

	// Verify it's valid JSON that can be unmarshaled
	var comment types.Comment
	if err := json.Unmarshal(jsonData, &comment); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if comment.ID != "json123" {
		t.Errorf("Expected ID 'json123', got '%s'", comment.ID)
	}

	if comment.Body != "JSON comment" {
		t.Errorf("Expected Body 'JSON comment', got '%s'", comment.Body)
	}
}

func TestCommentBuilder_WithCreated(t *testing.T) {
	timestamp := 1234567890.0
	comment := NewCommentBuilder().WithCreated(timestamp).Build()

	if comment.Created.Created != timestamp {
		t.Errorf("Expected Created %f, got %f", timestamp, comment.Created.Created)
	}

	if comment.Created.CreatedUTC != timestamp {
		t.Errorf("Expected CreatedUTC %f, got %f", timestamp, comment.Created.CreatedUTC)
	}
}

func TestCommentBuilder_WithEdited(t *testing.T) {
	timestamp := 1234567890.0
	comment := NewCommentBuilder().WithEdited(true, timestamp).Build()

	if !comment.Edited.IsEdited {
		t.Error("Expected IsEdited to be true")
	}

	if comment.Edited.Timestamp != timestamp {
		t.Errorf("Expected Edited timestamp %f, got %f", timestamp, comment.Edited.Timestamp)
	}
}

func TestCommentBuilder_WithGilded(t *testing.T) {
	comment := NewCommentBuilder().WithGilded(5).Build()

	if comment.Gilded != 5 {
		t.Errorf("Expected Gilded 5, got %d", comment.Gilded)
	}
}

func TestCommentBuilder_WithScoreHidden(t *testing.T) {
	comment := NewCommentBuilder().WithScoreHidden(true).Build()

	if !comment.ScoreHidden {
		t.Error("Expected ScoreHidden to be true")
	}
}

func TestCommentBuilder_WithLinkMetadata(t *testing.T) {
	comment := NewCommentBuilder().
		WithLinkTitle("Post Title").
		WithLinkAuthor("postauthor").
		Build()

	if comment.LinkTitle != "Post Title" {
		t.Errorf("Expected LinkTitle 'Post Title', got '%s'", comment.LinkTitle)
	}

	if comment.LinkAuthor != "postauthor" {
		t.Errorf("Expected LinkAuthor 'postauthor', got '%s'", comment.LinkAuthor)
	}
}

func TestCommentBuilder_WithDistinguished(t *testing.T) {
	comment := NewCommentBuilder().WithDistinguished("moderator").Build()

	if comment.Distinguished == nil {
		t.Fatal("Expected Distinguished to be non-nil")
	}

	if *comment.Distinguished != "moderator" {
		t.Errorf("Expected Distinguished 'moderator', got '%s'", *comment.Distinguished)
	}
}

func TestCommentBuilder_NestedReplies(t *testing.T) {
	// Build a nested comment structure:
	// parent
	//   -> reply1
	//      -> nestedReply1
	//      -> nestedReply2
	//   -> reply2

	nestedReply1 := NewCommentBuilder().
		WithID("nested1").
		WithBody("Nested reply 1").
		WithParentID("t1_reply1").
		Build()

	nestedReply2 := NewCommentBuilder().
		WithID("nested2").
		WithBody("Nested reply 2").
		WithParentID("t1_reply1").
		Build()

	reply1 := NewCommentBuilder().
		WithID("reply1").
		WithBody("Reply 1").
		WithParentID("t1_parent").
		WithReplies(nestedReply1, nestedReply2).
		Build()

	reply2 := NewCommentBuilder().
		WithID("reply2").
		WithBody("Reply 2").
		WithParentID("t1_parent").
		Build()

	parent := NewCommentBuilder().
		WithID("parent").
		WithBody("Parent comment").
		WithReplies(reply1, reply2).
		Build()

	// Verify parent structure
	if len(parent.Replies) != 2 {
		t.Fatalf("Expected parent to have 2 replies, got %d", len(parent.Replies))
	}

	// Verify first reply has nested replies
	if len(parent.Replies[0].Replies) != 2 {
		t.Fatalf("Expected reply1 to have 2 nested replies, got %d", len(parent.Replies[0].Replies))
	}

	// Verify second reply has no nested replies
	if len(parent.Replies[1].Replies) != 0 {
		t.Fatalf("Expected reply2 to have 0 nested replies, got %d", len(parent.Replies[1].Replies))
	}

	// Verify nested reply IDs
	if parent.Replies[0].Replies[0].ID != "nested1" {
		t.Errorf("Expected nested reply ID 'nested1', got '%s'", parent.Replies[0].Replies[0].ID)
	}

	if parent.Replies[0].Replies[1].ID != "nested2" {
		t.Errorf("Expected nested reply ID 'nested2', got '%s'", parent.Replies[0].Replies[1].ID)
	}
}
