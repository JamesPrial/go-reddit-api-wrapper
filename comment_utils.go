package graw

import (
	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// CommentTree provides utility methods for working with comment trees.
type CommentTree interface {
	Flatten() []*types.Comment
	Filter(func(*types.Comment) bool) []*types.Comment
	Find(func(*types.Comment) bool) *types.Comment
	GetByID(string) *types.Comment
	GetByAuthor(string) []*types.Comment
	GetTopLevel() []*types.Comment
	GetDepth() int
	Count() int
	Walk(func(*types.Comment))
}

// NewCommentTree creates a new CommentTree from a slice of comments.
func NewCommentTree(comments []*types.Comment) CommentTree {
	return internal.NewCommentTree(comments)
}