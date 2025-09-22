package graw

import (
	"context"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// PostIterator provides an iterator for paginating through posts.
type PostIterator interface {
	HasNext() bool
	Next() (*types.Post, error)
}

// CommentIterator provides an iterator for traversing comment trees.
type CommentIterator interface {
	HasNext() bool
	Next() (*types.Comment, error)
}

// postIteratorImpl implements PostIterator
type postIteratorImpl struct {
	client    *Client
	ctx       context.Context
	subreddit string
	listType  string // "hot" or "new"
	buffer    []*types.Post
	bufferIdx int
	after     string
	hasMore   bool
	limit     int
}

// NewHotIterator creates an iterator for hot posts.
func (c *Client) NewHotIterator(ctx context.Context, subreddit string, limit int) PostIterator {
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 25
	}
	return &postIteratorImpl{
		client:    c,
		ctx:       ctx,
		subreddit: subreddit,
		listType:  "hot",
		hasMore:   true,
		limit:     limit,
	}
}

// NewNewIterator creates an iterator for new posts.
func (c *Client) NewNewIterator(ctx context.Context, subreddit string, limit int) PostIterator {
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 25
	}
	return &postIteratorImpl{
		client:    c,
		ctx:       ctx,
		subreddit: subreddit,
		listType:  "new",
		hasMore:   true,
		limit:     limit,
	}
}

func (it *postIteratorImpl) HasNext() bool {
	return it.bufferIdx < len(it.buffer) || it.hasMore
}

func (it *postIteratorImpl) Next() (*types.Post, error) {
	// If buffer is empty or exhausted, fetch more posts
	if it.bufferIdx >= len(it.buffer) {
		if !it.hasMore {
			return nil, fmt.Errorf("no more posts available")
		}

		opts := &ListingOptions{
			Limit: it.limit,
			After: it.after,
		}

		var resp *PostsResponse
		var err error

		if it.listType == "hot" {
			resp, err = it.client.GetHot(it.ctx, it.subreddit, opts)
		} else {
			resp, err = it.client.GetNew(it.ctx, it.subreddit, opts)
		}

		if err != nil {
			return nil, err
		}

		it.buffer = resp.Posts
		it.bufferIdx = 0
		it.after = resp.After

		if len(it.buffer) == 0 || resp.After == "" {
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

// NewCommentIterator creates an iterator for traversing comment trees.
func NewCommentIterator(comments []*types.Comment, depthFirst bool) CommentIterator {
	return internal.NewCommentIterator(comments, &internal.CommentIteratorOptions{
		DepthFirst: depthFirst,
	})
}