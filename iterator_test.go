package graw

import (
	"context"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestPostIterator_NilHandling(t *testing.T) {
	// Create a mock client
	client := &Client{}
	ctx := context.Background()

	// Test iterator with nil posts
	mockListFunc := func(ctx context.Context, subreddit string, opts *ListingOptions) (*PostsResponse, error) {
		return &PostsResponse{
			Posts: []*types.Post{
				nil,
				{ID: "1", Data: &types.LinkData{Title: "Test"}},
				nil,
			},
		}, nil
	}

	iterator := &PostIterator{
		client:    client,
		subreddit: "test",
		listFunc:  mockListFunc,
		options:   &ListingOptions{Limit: 10},
		hasMore:   true,
		ctx:       ctx,
	}

	// Should skip nil posts
	post, err := iterator.Next()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if post == nil || post.ID != "1" {
		t.Errorf("Expected post with ID '1', got %v", post)
	}
}

func TestCommentIterator_NilHandling(t *testing.T) {
	tests := []struct {
		name     string
		comments []*types.Comment
		opts     *TraversalOptions
	}{
		{
			name:     "Nil comments",
			comments: nil,
			opts:     nil,
		},
		{
			name:     "Empty comments",
			comments: []*types.Comment{},
			opts:     nil,
		},
		{
			name: "Comments with nil elements",
			comments: []*types.Comment{
				nil,
				createTestComment("1", "user1", 5, "test"),
				nil,
			},
			opts: nil,
		},
		{
			name: "Comments with nil data",
			comments: []*types.Comment{
				{ID: "1", Name: "t1_1", Data: nil},
				createTestComment("2", "user2", 5, "test"),
			},
			opts: nil,
		},
		{
			name: "Depth-first traversal",
			comments: []*types.Comment{
				createTestComment("1", "user1", 5, "test"),
				createTestComment("2", "user2", 10, "test"),
			},
			opts: &TraversalOptions{
				Order:         DepthFirst,
				IterativeMode: true,
			},
		},
		{
			name: "Breadth-first traversal",
			comments: []*types.Comment{
				createTestComment("1", "user1", 5, "test"),
				createTestComment("2", "user2", 10, "test"),
			},
			opts: &TraversalOptions{
				Order:         BreadthFirst,
				IterativeMode: true,
			},
		},
		{
			name: "With min score filter",
			comments: []*types.Comment{
				createTestComment("1", "user1", 5, "low"),
				createTestComment("2", "user2", 15, "high"),
			},
			opts: &TraversalOptions{
				MinScore:      10,
				IterativeMode: true,
			},
		},
		{
			name: "With max depth",
			comments: []*types.Comment{
				createTestComment("1", "user1", 5, "test"),
			},
			opts: &TraversalOptions{
				MaxDepth:      1,
				IterativeMode: true,
			},
		},
		{
			name: "With custom filter",
			comments: []*types.Comment{
				createTestComment("1", "user1", 5, "test"),
				createTestComment("2", "user2", 10, "test"),
			},
			opts: &TraversalOptions{
				FilterFunc: func(c *types.Comment) bool {
					return c != nil && c.Data != nil && c.Data.Author == "user1"
				},
				IterativeMode: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iterator := NewCommentIterator(tt.comments, tt.opts)

			// Should not panic
			count := 0
			maxIterations := 100 // Prevent infinite loops

			for iterator.HasNext() && count < maxIterations {
				comment, err := iterator.Next()
				if err != nil {
					break
				}
				if comment != nil {
					count++
				}
			}
		})
	}
}

func TestCommentIterator_VisitedTracking(t *testing.T) {
	// Create the same comment twice
	comment1 := createTestComment("1", "user1", 5, "test")
	comment2 := comment1 // Same reference

	iterator := NewCommentIterator([]*types.Comment{comment1, comment2}, nil)

	// Should only return the comment once
	count := 0
	for iterator.HasNext() {
		comment, err := iterator.Next()
		if err != nil {
			break
		}
		if comment != nil && comment.ID == "1" {
			count++
		}
		if count > 1 {
			t.Errorf("Comment visited more than once")
			break
		}
	}
}

func TestCommentIterator_NestedReplies(t *testing.T) {
	// Create nested comment structure with potential nil issues
	reply2 := createTestComment("3", "user3", 5, "reply to reply")
	reply1 := createCommentWithReplies("2", "user2", reply2, nil) // Include nil
	root := createCommentWithReplies("1", "user1", reply1)

	iterator := NewCommentIterator([]*types.Comment{root}, &TraversalOptions{
		IterativeMode: true,
		Order:         DepthFirst,
	})

	ids := []string{}
	for iterator.HasNext() {
		comment, err := iterator.Next()
		if err != nil {
			break
		}
		if comment != nil && comment.Data != nil {
			ids = append(ids, comment.ID)
		}
	}

	// Should traverse all non-nil comments
	expectedIDs := map[string]bool{
		"1": true,
		"2": true,
		"3": true,
	}

	for _, id := range ids {
		if !expectedIDs[id] {
			t.Errorf("Unexpected comment ID: %s", id)
		}
		delete(expectedIDs, id)
	}

	if len(expectedIDs) > 0 {
		t.Errorf("Missing comment IDs: %v", expectedIDs)
	}
}

func TestGetCommentDepth_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		comment *types.Comment
		want    int
	}{
		{
			name:    "Nil comment",
			comment: nil,
			want:    0,
		},
		{
			name:    "Comment with nil data",
			comment: &types.Comment{ID: "1", Data: nil},
			want:    0,
		},
		{
			name:    "Regular comment",
			comment: createTestComment("1", "user1", 5, "test"),
			want:    0, // Current implementation always returns 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			depth := getCommentDepth(tt.comment)
			if depth != tt.want {
				t.Errorf("Expected depth %d, got %d", tt.want, depth)
			}
		})
	}
}