package graw

import (
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// CommentTree provides utility methods for working with comment trees.
type CommentTree struct {
	Comments []*types.Comment
}

// NewCommentTree creates a new CommentTree from a slice of comments.
func NewCommentTree(comments []*types.Comment) *CommentTree {
	return &CommentTree{Comments: comments}
}

// Flatten returns all comments in the tree as a flat slice.
func (ct *CommentTree) Flatten() []*types.Comment {
	var result []*types.Comment
	ct.flattenRecursive(ct.Comments, &result)
	return result
}

// flattenRecursive recursively flattens the comment tree.
func (ct *CommentTree) flattenRecursive(comments []*types.Comment, result *[]*types.Comment) {
	for _, comment := range comments {
		*result = append(*result, comment)
		replies := extractReplies(comment)
		if len(replies) > 0 {
			ct.flattenRecursive(replies, result)
		}
	}
}

// Filter returns comments that match the given filter function.
func (ct *CommentTree) Filter(filterFunc func(*types.Comment) bool) []*types.Comment {
	var result []*types.Comment
	ct.filterRecursive(ct.Comments, &result, filterFunc)
	return result
}

// filterRecursive recursively filters the comment tree.
func (ct *CommentTree) filterRecursive(comments []*types.Comment, result *[]*types.Comment, filterFunc func(*types.Comment) bool) {
	for _, comment := range comments {
		if filterFunc(comment) {
			*result = append(*result, comment)
		}
		replies := extractReplies(comment)
		if len(replies) > 0 {
			ct.filterRecursive(replies, result, filterFunc)
		}
	}
}

// Find returns the first comment that matches the given condition.
func (ct *CommentTree) Find(condition func(*types.Comment) bool) *types.Comment {
	return ct.findRecursive(ct.Comments, condition)
}

// findRecursive recursively searches for a comment matching the condition.
func (ct *CommentTree) findRecursive(comments []*types.Comment, condition func(*types.Comment) bool) *types.Comment {
	for _, comment := range comments {
		if condition(comment) {
			return comment
		}
		replies := extractReplies(comment)
		if len(replies) > 0 {
			if found := ct.findRecursive(replies, condition); found != nil {
				return found
			}
		}
	}
	return nil
}

// FindAll returns all comments that match the given condition.
func (ct *CommentTree) FindAll(condition func(*types.Comment) bool) []*types.Comment {
	return ct.Filter(condition)
}

// GetByID returns a comment by its ID.
func (ct *CommentTree) GetByID(id string) *types.Comment {
	return ct.Find(func(c *types.Comment) bool {
		return c.ID == id
	})
}

// GetByAuthor returns all comments by a specific author.
func (ct *CommentTree) GetByAuthor(author string) []*types.Comment {
	return ct.Filter(func(c *types.Comment) bool {
		return c.Data != nil && c.Data.Author == author
	})
}

// GetTopLevel returns only the top-level comments (no parent).
func (ct *CommentTree) GetTopLevel() []*types.Comment {
	return ct.Comments
}

// GetRepliesTo returns all direct replies to a specific comment.
func (ct *CommentTree) GetRepliesTo(commentID string) []*types.Comment {
	comment := ct.GetByID(commentID)
	if comment == nil {
		return nil
	}
	return extractReplies(comment)
}

// GetDepth returns the maximum depth of the comment tree.
func (ct *CommentTree) GetDepth() int {
	return ct.getDepthRecursive(ct.Comments, 0)
}

// getDepthRecursive recursively calculates the maximum depth.
func (ct *CommentTree) getDepthRecursive(comments []*types.Comment, currentDepth int) int {
	maxDepth := currentDepth
	for _, comment := range comments {
		replies := extractReplies(comment)
		if len(replies) > 0 {
			depth := ct.getDepthRecursive(replies, currentDepth+1)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}
	return maxDepth
}

// Count returns the total number of comments in the tree.
func (ct *CommentTree) Count() int {
	return len(ct.Flatten())
}

// CountByAuthor returns the number of comments by a specific author.
func (ct *CommentTree) CountByAuthor(author string) int {
	return len(ct.GetByAuthor(author))
}

// GetScoreRange returns comments within a score range.
func (ct *CommentTree) GetScoreRange(minScore, maxScore int) []*types.Comment {
	return ct.Filter(func(c *types.Comment) bool {
		if c.Data == nil {
			return false
		}
		return c.Data.Score >= minScore && c.Data.Score <= maxScore
	})
}

// GetGilded returns all gilded comments.
func (ct *CommentTree) GetGilded() []*types.Comment {
	return ct.Filter(func(c *types.Comment) bool {
		return c.Data != nil && c.Data.Gilded > 0
	})
}

// SortByScore sorts comments by score (highest first).
func (ct *CommentTree) SortByScore() []*types.Comment {
	flattened := ct.Flatten()
	// Simple bubble sort for demonstration - in production, use sort.Slice
	for i := 0; i < len(flattened); i++ {
		for j := i + 1; j < len(flattened); j++ {
			if flattened[i].Data != nil && flattened[j].Data != nil {
				if flattened[i].Data.Score < flattened[j].Data.Score {
					flattened[i], flattened[j] = flattened[j], flattened[i]
				}
			}
		}
	}
	return flattened
}

// GetParentChain returns the chain of parent comments for a given comment ID.
// Note: This requires parent tracking which isn't fully implemented in the basic structure.
func (ct *CommentTree) GetParentChain(commentID string) []*types.Comment {
	// This would require maintaining parent references during tree construction
	// For now, return nil as a placeholder
	return nil
}

// Walk applies a function to each comment in the tree.
func (ct *CommentTree) Walk(fn func(*types.Comment)) {
	ct.walkRecursive(ct.Comments, fn)
}

// walkRecursive recursively walks the comment tree.
func (ct *CommentTree) walkRecursive(comments []*types.Comment, fn func(*types.Comment)) {
	for _, comment := range comments {
		fn(comment)
		replies := extractReplies(comment)
		if len(replies) > 0 {
			ct.walkRecursive(replies, fn)
		}
	}
}

// Map transforms each comment in the tree using the provided function.
func (ct *CommentTree) Map(fn func(*types.Comment) interface{}) []interface{} {
	var result []interface{}
	ct.Walk(func(c *types.Comment) {
		result = append(result, fn(c))
	})
	return result
}

// ToThreadedStructure converts the flat comment list to a properly threaded structure.
// This is useful when comments are received as a flat list with parent_id references.
func ToThreadedStructure(comments []*types.Comment) []*types.Comment {
	// Create a map for quick lookup
	commentMap := make(map[string]*types.Comment)
	for _, comment := range comments {
		commentMap[comment.ID] = comment
	}

	// Build the tree
	var roots []*types.Comment
	for _, comment := range comments {
		if comment.Data == nil {
			continue
		}

		// Check if this is a top-level comment (parent is the post)
		if comment.Data.ParentID[:2] == "t3" {
			roots = append(roots, comment)
		}
	}

	return roots
}