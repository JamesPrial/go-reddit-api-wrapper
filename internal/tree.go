package internal

import (
	"encoding/json"

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

func (ct *CommentTree) flattenRecursive(comments []*types.Comment, result *[]*types.Comment) {
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		*result = append(*result, comment)
		replies := ExtractReplies(comment)
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

func (ct *CommentTree) filterRecursive(comments []*types.Comment, result *[]*types.Comment, filterFunc func(*types.Comment) bool) {
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		if filterFunc(comment) {
			*result = append(*result, comment)
		}
		replies := ExtractReplies(comment)
		if len(replies) > 0 {
			ct.filterRecursive(replies, result, filterFunc)
		}
	}
}

// Find returns the first comment that matches the given condition.
func (ct *CommentTree) Find(condition func(*types.Comment) bool) *types.Comment {
	return ct.findRecursive(ct.Comments, condition)
}

func (ct *CommentTree) findRecursive(comments []*types.Comment, condition func(*types.Comment) bool) *types.Comment {
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		if condition(comment) {
			return comment
		}
		replies := ExtractReplies(comment)
		if len(replies) > 0 {
			if found := ct.findRecursive(replies, condition); found != nil {
				return found
			}
		}
	}
	return nil
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

// GetTopLevel returns only the top-level comments.
func (ct *CommentTree) GetTopLevel() []*types.Comment {
	return ct.Comments
}

// GetDepth returns the maximum depth of the comment tree.
func (ct *CommentTree) GetDepth() int {
	return ct.getDepthRecursive(ct.Comments, 0)
}

func (ct *CommentTree) getDepthRecursive(comments []*types.Comment, currentDepth int) int {
	maxDepth := currentDepth
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		replies := ExtractReplies(comment)
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

// Walk applies a function to each comment in the tree.
func (ct *CommentTree) Walk(fn func(*types.Comment)) {
	ct.walkRecursive(ct.Comments, fn)
}

func (ct *CommentTree) walkRecursive(comments []*types.Comment, fn func(*types.Comment)) {
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		fn(comment)
		replies := ExtractReplies(comment)
		if len(replies) > 0 {
			ct.walkRecursive(replies, fn)
		}
	}
}

// ExtractReplies extracts reply comments from a comment's replies field.
func ExtractReplies(comment *types.Comment) []*types.Comment {
	if comment == nil || comment.Data == nil || comment.Data.Replies.Thing == nil {
		return nil
	}

	repliesThing := comment.Data.Replies.Thing
	if repliesThing.Kind != "Listing" {
		return nil
	}

	var listing types.ListingData
	if err := json.Unmarshal(repliesThing.Data, &listing); err != nil {
		return nil
	}

	var result []*types.Comment
	for _, thing := range listing.Children {
		if thing.Kind == "t1" {
			var commentData types.CommentData
			if err := json.Unmarshal(thing.Data, &commentData); err == nil {
				result = append(result, &types.Comment{
					ID:   thing.ID,
					Name: thing.Name,
					Data: &commentData,
				})
			}
		}
	}

	return result
}