package testutil

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// CommentBuilder provides a fluent API for building Comment test data with sensible defaults.
// It simplifies creating test comments with nested reply structures, automatically handling
// Reddit-specific fields like fullnames (name with "t1_" prefix) and timestamps.
//
// Example usage:
//
//	// Simple comment
//	comment := NewCommentBuilder().
//		WithID("abc123").
//		WithBody("This is a test comment").
//		WithAuthor("testuser").
//		Build()
//
//	// Comment with nested replies
//	reply1 := NewCommentBuilder().
//		WithID("def456").
//		WithBody("First reply").
//		WithParentID("t1_abc123").
//		Build()
//
//	reply2 := NewCommentBuilder().
//		WithID("ghi789").
//		WithBody("Second reply").
//		WithParentID("t1_abc123").
//		Build()
//
//	parent := NewCommentBuilder().
//		WithID("abc123").
//		WithBody("Parent comment").
//		WithReplies(reply1, reply2).
//		Build()
//
//	// Convert to Thing for API responses
//	thing := NewCommentBuilder().WithID("test123").ToThing()
//
//	// Get raw JSON for mock responses
//	jsonData := NewCommentBuilder().WithID("test456").ToJSON()
type CommentBuilder struct {
	comment *types.Comment
}

// NewCommentBuilder creates a new CommentBuilder with sensible defaults.
// Defaults include:
//   - ID: "comment123"
//   - Name: "t1_comment123" (auto-generated from ID)
//   - Body: "Test comment"
//   - Author: "testuser"
//   - Score: 10
//   - Ups: 10
//   - Downs: 0
//   - LinkID: "t3_post123"
//   - ParentID: "t3_post123"
//   - Subreddit: "test"
//   - SubredditID: "t5_test"
//   - Created/CreatedUTC: current Unix timestamp as float64
//   - Replies: empty slice
func NewCommentBuilder() *CommentBuilder {
	now := float64(time.Now().Unix())
	return &CommentBuilder{
		comment: &types.Comment{
			ThingData: types.ThingData{
				ID:   "comment123",
				Name: "t1_comment123",
			},
			Votable: types.Votable{
				Score: 10,
				Ups:   10,
				Downs: 0,
				Likes: nil,
			},
			Created: types.Created{
				Created:    now,
				CreatedUTC: now,
			},
			Body:        "Test comment",
			BodyHTML:    "&lt;div class=\"md\"&gt;&lt;p&gt;Test comment&lt;/p&gt;\n&lt;/div&gt;",
			Author:      "testuser",
			LinkID:      "t3_post123",
			ParentID:    "t3_post123",
			Subreddit:   "test",
			SubredditID: "t5_test",
			Replies:     []*types.Comment{},
			Edited:      types.Edited{IsEdited: false, Timestamp: 0},
			Gilded:      0,
			Saved:       false,
			ScoreHidden: false,
		},
	}
}

// WithID sets the comment ID and automatically generates the Name field as "t1_" + ID.
func (b *CommentBuilder) WithID(id string) *CommentBuilder {
	b.comment.ID = id
	b.comment.Name = "t1_" + id
	return b
}

// WithBody sets the comment body text.
func (b *CommentBuilder) WithBody(body string) *CommentBuilder {
	b.comment.Body = body
	// Update HTML version as well
	b.comment.BodyHTML = fmt.Sprintf("&lt;div class=\"md\"&gt;&lt;p&gt;%s&lt;/p&gt;\n&lt;/div&gt;", body)
	return b
}

// WithAuthor sets the comment author username.
func (b *CommentBuilder) WithAuthor(author string) *CommentBuilder {
	b.comment.Author = author
	return b
}

// WithScore sets the comment score (upvotes minus downvotes).
// Also sets Ups to the same value since Reddit returns them as equal.
func (b *CommentBuilder) WithScore(score int) *CommentBuilder {
	b.comment.Score = score
	b.comment.Ups = score
	return b
}

// WithLinkID sets the LinkID field (the post this comment belongs to).
// Should be in format "t3_postid".
func (b *CommentBuilder) WithLinkID(linkID string) *CommentBuilder {
	b.comment.LinkID = linkID
	return b
}

// WithParentID sets the ParentID field (the comment or post this is replying to).
// Should be in format "t1_commentid" for comment replies or "t3_postid" for top-level comments.
func (b *CommentBuilder) WithParentID(parentID string) *CommentBuilder {
	b.comment.ParentID = parentID
	return b
}

// WithParentPost sets both LinkID and ParentID to "t3_" + postID.
// Use this for top-level comments that reply directly to a post.
func (b *CommentBuilder) WithParentPost(postID string) *CommentBuilder {
	fullname := "t3_" + postID
	b.comment.LinkID = fullname
	b.comment.ParentID = fullname
	return b
}

// WithSubreddit sets the subreddit name (without "r/" prefix).
// Also updates SubredditID to "t5_" + sub.
func (b *CommentBuilder) WithSubreddit(sub string) *CommentBuilder {
	b.comment.Subreddit = sub
	b.comment.SubredditID = "t5_" + sub
	return b
}

// WithCreated sets both Created and CreatedUTC to the given Unix timestamp.
func (b *CommentBuilder) WithCreated(timestamp float64) *CommentBuilder {
	b.comment.Created.Created = timestamp
	b.comment.Created.CreatedUTC = timestamp
	return b
}

// WithReply adds a single nested reply to this comment.
// Can be called multiple times to add multiple replies.
func (b *CommentBuilder) WithReply(reply *types.Comment) *CommentBuilder {
	b.comment.Replies = append(b.comment.Replies, reply)
	return b
}

// WithReplies sets the replies to the given slice of comments.
// This replaces any previously set replies.
func (b *CommentBuilder) WithReplies(replies ...*types.Comment) *CommentBuilder {
	b.comment.Replies = replies
	return b
}

// WithEdited sets the edited status and optional timestamp.
func (b *CommentBuilder) WithEdited(isEdited bool, timestamp float64) *CommentBuilder {
	b.comment.Edited = types.Edited{
		IsEdited:  isEdited,
		Timestamp: timestamp,
	}
	return b
}

// WithGilded sets the gilded count (number of awards/gold).
func (b *CommentBuilder) WithGilded(gilded int) *CommentBuilder {
	b.comment.Gilded = gilded
	return b
}

// WithScoreHidden sets whether the comment score is hidden.
func (b *CommentBuilder) WithScoreHidden(hidden bool) *CommentBuilder {
	b.comment.ScoreHidden = hidden
	return b
}

// WithLinkTitle sets the title of the post this comment belongs to.
func (b *CommentBuilder) WithLinkTitle(title string) *CommentBuilder {
	b.comment.LinkTitle = title
	return b
}

// WithLinkAuthor sets the author of the post this comment belongs to.
func (b *CommentBuilder) WithLinkAuthor(author string) *CommentBuilder {
	b.comment.LinkAuthor = author
	return b
}

// WithDistinguished sets the distinguished status (e.g., "moderator", "admin").
func (b *CommentBuilder) WithDistinguished(distinguished string) *CommentBuilder {
	b.comment.Distinguished = &distinguished
	return b
}

// Build returns the constructed Comment struct.
func (b *CommentBuilder) Build() *types.Comment {
	return b.comment
}

// ToThing wraps the comment in a Thing structure with kind "t1" and the comment data as JSON.
// Returns a Thing{Kind: "t1", Data: json.RawMessage} suitable for API responses.
func (b *CommentBuilder) ToThing() *types.Thing {
	data, err := json.Marshal(b.comment)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal comment to JSON: %v", err))
	}

	return &types.Thing{
		ThingData: types.ThingData{
			ID:   b.comment.ID,
			Name: b.comment.Name,
		},
		Kind: "t1",
		Data: json.RawMessage(data),
	}
}

// ToJSON returns the comment as a json.RawMessage.
// Useful for creating mock API responses.
func (b *CommentBuilder) ToJSON() json.RawMessage {
	data, err := json.Marshal(b.comment)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal comment to JSON: %v", err))
	}
	return json.RawMessage(data)
}
