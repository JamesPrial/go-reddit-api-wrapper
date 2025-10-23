package graw

import (
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestCalculateCommentDepth verifies that calculateCommentDepth correctly
// traverses parent chains to determine comment nesting depth.
func TestCalculateCommentDepth(t *testing.T) {
	tests := []struct {
		name          string
		comments      []*types.Comment
		targetID      string
		expectedDepth int
		description   string
	}{
		{
			name: "top_level_comment",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
			},
			targetID:      "comment1",
			expectedDepth: 0,
			description:   "Comment replying to post should have depth 0",
		},
		{
			name: "single_level_reply",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
				{ThingData: types.ThingData{ID: "comment2"}, ParentID: "t1_comment1"},
			},
			targetID:      "comment2",
			expectedDepth: 1,
			description:   "Reply to top-level comment should have depth 1",
		},
		{
			name: "deep_nesting_3_levels",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
				{ThingData: types.ThingData{ID: "comment2"}, ParentID: "t1_comment1"},
				{ThingData: types.ThingData{ID: "comment3"}, ParentID: "t1_comment2"},
			},
			targetID:      "comment3",
			expectedDepth: 2,
			description:   "Three-level deep nesting should calculate correctly",
		},
		{
			name: "deep_nesting_5_levels",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
				{ThingData: types.ThingData{ID: "comment2"}, ParentID: "t1_comment1"},
				{ThingData: types.ThingData{ID: "comment3"}, ParentID: "t1_comment2"},
				{ThingData: types.ThingData{ID: "comment4"}, ParentID: "t1_comment3"},
				{ThingData: types.ThingData{ID: "comment5"}, ParentID: "t1_comment4"},
			},
			targetID:      "comment5",
			expectedDepth: 4,
			description:   "Five-level deep nesting should calculate correctly",
		},
		{
			name: "missing_parent_in_collection",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
				{ThingData: types.ThingData{ID: "comment3"}, ParentID: "t1_comment2"}, // comment2 not in collection
			},
			targetID:      "comment3",
			expectedDepth: 1,
			description:   "Missing parent should increment depth once and stop",
		},
		{
			name: "parent_id_without_prefix",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
				{ThingData: types.ThingData{ID: "comment2"}, ParentID: "comment1"}, // No t1_ prefix
			},
			targetID:      "comment2",
			expectedDepth: 1,
			description:   "Should handle both prefixed and unprefixed parent IDs",
		},
		{
			name: "multiple_branches",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t3_post123"},
				{ThingData: types.ThingData{ID: "comment2"}, ParentID: "t1_comment1"},
				{ThingData: types.ThingData{ID: "comment3"}, ParentID: "t1_comment1"},
				{ThingData: types.ThingData{ID: "comment4"}, ParentID: "t1_comment2"},
				{ThingData: types.ThingData{ID: "comment5"}, ParentID: "t1_comment3"},
			},
			targetID:      "comment4",
			expectedDepth: 2,
			description:   "Should correctly calculate depth in branching tree",
		},
		{
			name: "empty_parent_id",
			comments: []*types.Comment{
				{ThingData: types.ThingData{ID: "comment1"}, ParentID: ""},
			},
			targetID:      "comment1",
			expectedDepth: 0,
			description:   "Empty parent ID should be treated as top-level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find target comment
			var targetComment *types.Comment
			for _, c := range tt.comments {
				if c.ID == tt.targetID {
					targetComment = c
					break
				}
			}

			if targetComment == nil {
				t.Fatalf("target comment %q not found in test data", tt.targetID)
			}

			// Calculate depth
			depth := calculateCommentDepth(targetComment, tt.comments)

			// Verify result
			if depth != tt.expectedDepth {
				t.Errorf("%s: got depth %d, want %d", tt.description, depth, tt.expectedDepth)
			}
		})
	}
}

// TestCalculateCommentDepth_CircularReference verifies that circular
// references are handled safely without infinite loops.
func TestCalculateCommentDepth_CircularReference(t *testing.T) {
	// Create circular reference: comment1 -> comment2 -> comment1
	comments := []*types.Comment{
		{ThingData: types.ThingData{ID: "comment1"}, ParentID: "t1_comment2"},
		{ThingData: types.ThingData{ID: "comment2"}, ParentID: "t1_comment1"},
	}

	// Should not panic or loop forever
	depth := calculateCommentDepth(comments[0], comments)

	// Depth should be stopped by cycle detection
	if depth > 2 {
		t.Errorf("circular reference not detected: got depth %d, expected <= 2", depth)
	}
}

// TestCalculateCommentDepth_MaxDepthLimit verifies that the max depth
// safety limit prevents excessive iteration.
func TestCalculateCommentDepth_MaxDepthLimit(t *testing.T) {
	// Create a very deep chain (more than maxDepth=1000)
	const chainLength = 1500
	comments := make([]*types.Comment, chainLength)

	// First comment is top-level
	comments[0] = &types.Comment{
		ThingData: types.ThingData{ID: "comment0"},
		ParentID:  "t3_post123",
	}

	// Build chain
	for i := 1; i < chainLength; i++ {
		comments[i] = &types.Comment{
			ThingData: types.ThingData{ID: "comment" + string(rune('0'+i%10))},
			ParentID:  "t1_comment" + string(rune('0'+(i-1)%10)),
		}
	}

	// Calculate depth for last comment
	depth := calculateCommentDepth(comments[chainLength-1], comments)

	// Should be capped at maxDepth
	if depth > 1000 {
		t.Errorf("max depth limit not enforced: got depth %d, expected <= 1000", depth)
	}
}
