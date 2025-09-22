package internal

import (
	"context"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// PostIterator provides an iterator for paginating through posts.
type PostIterator struct {
	ctx       context.Context
	listFunc  func(context.Context, string, map[string]string) ([]*types.Post, string, string, error)
	subreddit string
	limit     int
	buffer    []*types.Post
	bufferIdx int
	after     string
	hasMore   bool
	err       error
}

// NewPostIterator creates a new post iterator.
func NewPostIterator(ctx context.Context, subreddit string, limit int, listFunc func(context.Context, string, map[string]string) ([]*types.Post, string, string, error)) *PostIterator {
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 1
	}
	return &PostIterator{
		ctx:       ctx,
		subreddit: subreddit,
		limit:     limit,
		listFunc:  listFunc,
		hasMore:   true,
	}
}

// HasNext returns true if there are more posts to iterate through.
func (it *PostIterator) HasNext() bool {
	if it.err != nil {
		return false
	}
	return it.bufferIdx < len(it.buffer) || it.hasMore
}

// Next returns the next post in the iteration.
func (it *PostIterator) Next() (*types.Post, error) {
	if it.err != nil {
		return nil, it.err
	}

	if it.bufferIdx >= len(it.buffer) {
		if !it.hasMore {
			return nil, fmt.Errorf("no more posts available")
		}

		params := map[string]string{
			"limit": fmt.Sprintf("%d", it.limit),
		}
		if it.after != "" {
			params["after"] = it.after
		}

		posts, after, _, err := it.listFunc(it.ctx, it.subreddit, params)
		if err != nil {
			it.err = err
			return nil, err
		}

		it.buffer = posts
		it.bufferIdx = 0
		it.after = after

		if len(it.buffer) == 0 || after == "" {
			it.hasMore = false
			if len(it.buffer) == 0 {
				return nil, fmt.Errorf("no more posts available")
			}
		}
	}

	post := it.buffer[it.bufferIdx]
	it.bufferIdx++

	if post == nil {
		return it.Next()
	}

	return post, nil
}

// CommentIterator provides an iterator for traversing comment trees.
type CommentIterator struct {
	stack         []*types.Comment
	visited       map[string]bool
	depthFirst    bool
	filterFunc    func(*types.Comment) bool
	maxDepth      int
	currentDepths map[string]int
}

// CommentIteratorOptions provides options for comment iteration.
type CommentIteratorOptions struct {
	DepthFirst bool
	FilterFunc func(*types.Comment) bool
	MaxDepth   int
}

// NewCommentIterator creates a new iterator for traversing a comment tree.
func NewCommentIterator(comments []*types.Comment, opts *CommentIteratorOptions) *CommentIterator {
	if opts == nil {
		opts = &CommentIteratorOptions{
			DepthFirst: true,
		}
	}

	it := &CommentIterator{
		stack:         make([]*types.Comment, len(comments)),
		visited:       make(map[string]bool),
		depthFirst:    opts.DepthFirst,
		filterFunc:    opts.FilterFunc,
		maxDepth:      opts.MaxDepth,
		currentDepths: make(map[string]int),
	}
	copy(it.stack, comments)

	// Initialize depths for root comments
	for _, c := range it.stack {
		if c != nil {
			it.currentDepths[c.ID] = 0
		}
	}

	if opts.DepthFirst {
		for i, j := 0, len(it.stack)-1; i < j; i, j = i+1, j-1 {
			it.stack[i], it.stack[j] = it.stack[j], it.stack[i]
		}
	}

	return it
}

// HasNext returns true if there are more comments to iterate through.
func (it *CommentIterator) HasNext() bool {
	return len(it.stack) > 0
}

// Next returns the next comment in the iteration.
func (it *CommentIterator) Next() (*types.Comment, error) {
	if len(it.stack) == 0 {
		return nil, fmt.Errorf("no more comments available")
	}

	var comment *types.Comment

	if !it.depthFirst {
		comment = it.stack[0]
		it.stack = it.stack[1:]
	} else {
		comment = it.stack[len(it.stack)-1]
		it.stack = it.stack[:len(it.stack)-1]
	}

	if comment == nil {
		return it.Next()
	}

	if it.visited[comment.ID] {
		return it.Next()
	}
	it.visited[comment.ID] = true

	if it.filterFunc != nil && !it.filterFunc(comment) {
		return it.Next()
	}

	currentDepth := it.currentDepths[comment.ID]
	if it.maxDepth == 0 || currentDepth < it.maxDepth {
		replies := ExtractReplies(comment)
		if len(replies) > 0 {
			for _, reply := range replies {
				if reply != nil {
					it.currentDepths[reply.ID] = currentDepth + 1
				}
			}
			if !it.depthFirst {
				it.stack = append(it.stack, replies...)
			} else {
				for i := len(replies) - 1; i >= 0; i-- {
					it.stack = append(it.stack, replies[i])
				}
			}
		}
	}

	return comment, nil
}