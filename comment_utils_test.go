package graw

import (
	"encoding/json"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Helper function to create a test comment
func createTestComment(id, author string, score int, body string) *types.Comment {
	return &types.Comment{
		ID:   id,
		Name: "t1_" + id,
		Data: &types.CommentData{
			Author: author,
			Body:   body,
			Score:  score,
			Votable: types.Votable{
				Ups:   score,
				Downs: 0,
			},
		},
	}
}

// Helper function to create a comment with replies
func createCommentWithReplies(id, author string, replies ...*types.Comment) *types.Comment {
	comment := createTestComment(id, author, 10, "Test comment "+id)

	if len(replies) > 0 {
		// Create a Thing with Listing kind for replies
		children := make([]*types.Thing, 0, len(replies))
		for _, reply := range replies {
			if reply == nil {
				continue
			}
			// Create a simplified comment data that will marshal/unmarshal correctly
			// We need to avoid the Edited field marshaling issue
			commentDataMap := map[string]interface{}{
				"author": reply.Data.Author,
				"body":   reply.Data.Body,
				"score":  reply.Data.Score,
				"ups":    reply.Data.Ups,
				"downs":  reply.Data.Downs,
				"edited": false, // Use a simple boolean
				"gilded": reply.Data.Gilded,
			}

			// Handle nested replies if they exist
			if reply.Data.Replies.Thing != nil {
				// The reply has its own replies, include them
				commentDataMap["replies"] = reply.Data.Replies.Thing
			} else {
				commentDataMap["replies"] = "" // Empty string for no replies
			}

			replyData, _ := json.Marshal(commentDataMap)

			children = append(children, &types.Thing{
				ID:   reply.ID,
				Name: reply.Name,
				Kind: "t1",
				Data: replyData,
			})
		}

		listingData := types.ListingData{
			Children: children,
		}
		listingDataJSON, _ := json.Marshal(listingData)

		repliesThing := &types.Thing{
			Kind: "Listing",
			Data: listingDataJSON,
		}

		comment.Data.Replies = types.CommentReplies{
			Thing: repliesThing,
		}
	}

	return comment
}

func TestCommentTree_NilHandling(t *testing.T) {
	tests := []struct {
		name     string
		comments []*types.Comment
	}{
		{
			name:     "Nil comments slice",
			comments: nil,
		},
		{
			name:     "Empty comments slice",
			comments: []*types.Comment{},
		},
		{
			name: "Comments with nil elements",
			comments: []*types.Comment{
				nil,
				createTestComment("1", "user1", 5, "test"),
				nil,
			},
		},
		{
			name: "Comment with nil data",
			comments: []*types.Comment{
				{ID: "1", Name: "t1_1", Data: nil},
			},
		},
		{
			name: "Comment with nil replies",
			comments: []*types.Comment{
				createTestComment("1", "user1", 5, "test"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewCommentTree(tt.comments)

			// These should not panic
			_ = tree.Flatten()
			_ = tree.Count()
			_ = tree.GetDepth()
			_ = tree.GetTopLevel()
			_ = tree.GetByAuthor("user1")
			_ = tree.GetByID("1")
			_ = tree.Filter(func(c *types.Comment) bool {
				return c != nil && c.Data != nil && c.Data.Gilded > 0
			})
			_ = tree.Filter(func(c *types.Comment) bool {
				return c != nil && c.Data != nil && c.Data.Score >= 0 && c.Data.Score <= 100
			})
			_ = tree.Find(func(c *types.Comment) bool { return true })
			_ = tree.Filter(func(c *types.Comment) bool { return true })

			tree.Walk(func(c *types.Comment) {})
			// Map is no longer part of the interface
		})
	}
}

func TestCommentTree_Flatten(t *testing.T) {
	// Create nested comment structure
	reply2 := createTestComment("3", "user3", 5, "reply to reply")
	reply1 := createCommentWithReplies("2", "user2", reply2)
	root := createCommentWithReplies("1", "user1", reply1)

	tree := NewCommentTree([]*types.Comment{root})
	flattened := tree.Flatten()

	if len(flattened) != 3 {
		t.Errorf("Expected 3 comments, got %d", len(flattened))
	}
}

func TestCommentTree_Filter(t *testing.T) {
	comments := []*types.Comment{
		createTestComment("1", "user1", 5, "low score"),
		createTestComment("2", "user2", 15, "high score"),
		createTestComment("3", "user1", 20, "high score by user1"),
		nil, // Nil comment should be handled
	}

	tree := NewCommentTree(comments)

	// Filter by score
	highScored := tree.Filter(func(c *types.Comment) bool {
		return c != nil && c.Data != nil && c.Data.Score >= 10
	})

	if len(highScored) != 2 {
		t.Errorf("Expected 2 high-scored comments, got %d", len(highScored))
	}

	// Filter by author
	byUser1 := tree.GetByAuthor("user1")
	if len(byUser1) != 2 {
		t.Errorf("Expected 2 comments by user1, got %d", len(byUser1))
	}
}

func TestCommentTree_GetByID(t *testing.T) {
	comments := []*types.Comment{
		createTestComment("1", "user1", 5, "test"),
		createTestComment("2", "user2", 10, "test"),
		nil,
	}

	tree := NewCommentTree(comments)

	comment := tree.GetByID("1")
	if comment == nil || comment.ID != "1" {
		t.Errorf("Failed to get comment by ID")
	}

	// Non-existent ID
	comment = tree.GetByID("999")
	if comment != nil {
		t.Errorf("Expected nil for non-existent ID")
	}
}

func TestCommentTree_Depth(t *testing.T) {
	// Create deeply nested structure
	deep4 := createTestComment("5", "user5", 5, "depth 4")
	deep3 := createCommentWithReplies("4", "user4", deep4)
	deep2 := createCommentWithReplies("3", "user3", deep3)
	deep1 := createCommentWithReplies("2", "user2", deep2)
	root := createCommentWithReplies("1", "user1", deep1)

	tree := NewCommentTree([]*types.Comment{root})
	depth := tree.GetDepth()

	if depth != 4 {
		t.Errorf("Expected depth of 4, got %d", depth)
	}
}

func TestCommentTree_GetGilded(t *testing.T) {
	comments := []*types.Comment{
		createTestComment("1", "user1", 5, "not gilded"),
	}

	// Set gilded on one comment
	comments[0].Data.Gilded = 2

	tree := NewCommentTree(comments)
	gilded := tree.Filter(func(c *types.Comment) bool {
		return c != nil && c.Data != nil && c.Data.Gilded > 0
	})

	if len(gilded) != 1 {
		t.Errorf("Expected 1 gilded comment, got %d", len(gilded))
	}
}

func TestCommentTree_Walk(t *testing.T) {
	comments := []*types.Comment{
		createTestComment("1", "user1", 5, "test"),
		createTestComment("2", "user2", 10, "test"),
		nil, // Should be handled gracefully
	}

	tree := NewCommentTree(comments)
	count := 0

	tree.Walk(func(c *types.Comment) {
		if c != nil {
			count++
		}
	})

	if count != 2 {
		t.Errorf("Expected to walk 2 comments, got %d", count)
	}
}

func TestExtractReplies_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		comment *types.Comment
		wantLen int
	}{
		{
			name:    "Nil comment",
			comment: nil,
			wantLen: 0,
		},
		{
			name:    "Comment with nil data",
			comment: &types.Comment{ID: "1", Data: nil},
			wantLen: 0,
		},
		{
			name:    "Comment with empty replies",
			comment: createTestComment("1", "user1", 5, "test"),
			wantLen: 0,
		},
		{
			name: "Comment with nil replies Thing",
			comment: &types.Comment{
				ID:   "1",
				Data: &types.CommentData{
					Replies: types.CommentReplies{Thing: nil},
				},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replies := internal.ExtractReplies(tt.comment)
			if len(replies) != tt.wantLen {
				t.Errorf("Expected %d replies, got %d", tt.wantLen, len(replies))
			}
		})
	}
}