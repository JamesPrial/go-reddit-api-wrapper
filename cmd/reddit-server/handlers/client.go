package handlers

import (
	"context"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// RedditClient defines the interface for Reddit API operations needed by handlers.
// This allows for easy mocking in tests without requiring a real Reddit client.
type RedditClient interface {
	// Me retrieves the authenticated user's account information.
	Me(ctx context.Context) (*types.AccountData, error)

	// GetSubreddit retrieves information about a specific subreddit.
	GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error)

	// GetHot retrieves hot posts from a subreddit or frontpage.
	GetHot(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)

	// GetNew retrieves new posts from a subreddit or frontpage.
	GetNew(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)

	// GetComments retrieves comments for a specific post.
	GetComments(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error)

	// GetMoreComments expands previously truncated comment trees.
	GetMoreComments(ctx context.Context, req *types.MoreCommentsRequest) ([]*types.Comment, error)
}
