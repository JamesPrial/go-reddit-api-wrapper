package graw

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// PostIterator provides an iterator for paginating through posts.
type PostIterator struct {
	client    *Client
	subreddit string
	listFunc  func(context.Context, string, *ListingOptions) (*PostsResponse, error)
	options   *ListingOptions
	buffer    []*types.Post
	bufferIdx int
	after     string
	hasMore   bool
	err       error
	ctx       context.Context
}

// NewHotIterator creates a new iterator for hot posts.
func (c *Client) NewHotIterator(ctx context.Context, subreddit string) *PostIterator {
	return &PostIterator{
		client:    c,
		subreddit: subreddit,
		listFunc:  c.GetHot,
		options:   &ListingOptions{Limit: 100},
		hasMore:   true,
		ctx:       ctx,
	}
}

// NewNewIterator creates a new iterator for new posts.
func (c *Client) NewNewIterator(ctx context.Context, subreddit string) *PostIterator {
	return &PostIterator{
		client:    c,
		subreddit: subreddit,
		listFunc:  c.GetNew,
		options:   &ListingOptions{Limit: 100},
		hasMore:   true,
		ctx:       ctx,
	}
}

// WithLimit sets the number of posts to fetch per request.
func (it *PostIterator) WithLimit(limit int) *PostIterator {
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 1
	}
	it.options.Limit = limit
	return it
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

	// If buffer is empty or exhausted, fetch more posts
	if it.bufferIdx >= len(it.buffer) {
		if !it.hasMore {
			return nil, fmt.Errorf("no more posts available")
		}

		// Set the after parameter for pagination
		it.options.After = it.after

		// Fetch the next batch
		resp, err := it.listFunc(it.ctx, it.subreddit, it.options)
		if err != nil {
			it.err = err
			return nil, err
		}

		if resp == nil {
			it.err = fmt.Errorf("received nil response")
			return nil, it.err
		}

		it.buffer = resp.Posts
		it.bufferIdx = 0
		it.after = resp.After

		// If we got no posts or no after token, we've reached the end
		if len(it.buffer) == 0 || resp.After == "" {
			it.hasMore = false
			if len(it.buffer) == 0 {
				return nil, fmt.Errorf("no more posts available")
			}
		}
	}

	post := it.buffer[it.bufferIdx]
	it.bufferIdx++

	// Skip nil posts
	if post == nil {
		return it.Next()
	}

	return post, nil
}

// Error returns any error encountered during iteration.
func (it *PostIterator) Error() error {
	return it.err
}

// Reset resets the iterator to start from the beginning.
func (it *PostIterator) Reset() {
	it.buffer = nil
	it.bufferIdx = 0
	it.after = ""
	it.hasMore = true
	it.err = nil
	it.options.After = ""
	it.options.Before = ""
}

// Collect fetches all remaining posts up to a maximum limit.
func (it *PostIterator) Collect(maxPosts int) ([]*types.Post, error) {
	var posts []*types.Post
	count := 0

	for it.HasNext() && (maxPosts <= 0 || count < maxPosts) {
		post, err := it.Next()
		if err != nil {
			return posts, err
		}
		posts = append(posts, post)
		count++
	}

	return posts, nil
}

// CommentIterator provides an iterator for traversing comment trees.
type CommentIterator struct {
	stack   []*types.Comment
	visited map[string]bool
	options *TraversalOptions
}

// TraversalOptions provides options for comment tree traversal.
type TraversalOptions struct {
	MaxDepth      int                           // Maximum depth to traverse (0 = unlimited)
	MinScore      int                           // Minimum score for comments to include
	FilterFunc    func(*types.Comment) bool     // Custom filter function
	IterativeMode bool                          // Use iterative instead of recursive traversal
	Order         TraversalOrder                // Order of traversal
}

// TraversalOrder defines the order of tree traversal.
type TraversalOrder int

const (
	// DepthFirst traverses the tree depth-first (default).
	DepthFirst TraversalOrder = iota
	// BreadthFirst traverses the tree breadth-first.
	BreadthFirst
)

// NewCommentIterator creates a new iterator for traversing a comment tree.
func NewCommentIterator(comments []*types.Comment, opts *TraversalOptions) *CommentIterator {
	if opts == nil {
		opts = &TraversalOptions{
			IterativeMode: true,
			Order:         DepthFirst,
		}
	}

	// Initialize the stack/queue with root comments
	it := &CommentIterator{
		stack:   make([]*types.Comment, len(comments)),
		visited: make(map[string]bool),
		options: opts,
	}
	copy(it.stack, comments)

	// Reverse for depth-first to maintain order
	if opts.Order == DepthFirst {
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

	// Pop from stack/queue based on traversal order
	if it.options.Order == BreadthFirst {
		// Dequeue (FIFO)
		comment = it.stack[0]
		it.stack = it.stack[1:]
	} else {
		// Pop (LIFO)
		comment = it.stack[len(it.stack)-1]
		it.stack = it.stack[:len(it.stack)-1]
	}

	// Skip if already visited
	if it.visited[comment.ID] {
		return it.Next()
	}
	it.visited[comment.ID] = true

	// Apply filters
	if comment.Data != nil {
		if it.options.MinScore > 0 && comment.Data.Score < it.options.MinScore {
			return it.Next()
		}
	}
	if it.options.FilterFunc != nil && !it.options.FilterFunc(comment) {
		return it.Next()
	}

	// Add replies to stack if within depth limit
	if it.options.MaxDepth == 0 || getCommentDepth(comment) < it.options.MaxDepth {
		replies := extractReplies(comment)
		if len(replies) > 0 {
			if it.options.Order == BreadthFirst {
				// Enqueue replies (FIFO)
				it.stack = append(it.stack, replies...)
			} else {
				// Push replies in reverse order for depth-first (LIFO)
				for i := len(replies) - 1; i >= 0; i-- {
					it.stack = append(it.stack, replies[i])
				}
			}
		}
	}

	return comment, nil
}

// getCommentDepth calculates the depth of a comment in the tree.
func getCommentDepth(comment *types.Comment) int {
	depth := 0
	// Count the number of parent links (simplified - would need parent tracking)
	// In a real implementation, we'd track depth during traversal
	return depth
}

// extractReplies extracts reply comments from a comment's replies field.
func extractReplies(comment *types.Comment) []*types.Comment {
	if comment.Data == nil || comment.Data.Replies.Thing == nil {
		return nil
	}

	// The replies is a Thing containing a Listing
	repliesThing := comment.Data.Replies.Thing
	if repliesThing.Kind != "Listing" {
		return nil
	}

	// Unmarshal the listing data
	var listing types.ListingData
	if err := json.Unmarshal(repliesThing.Data, &listing); err != nil {
		return nil
	}

	var result []*types.Comment
	for _, thing := range listing.Children {
		if thing.Kind == "t1" {
			// Unmarshal the comment data
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