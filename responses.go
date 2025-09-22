package graw

import "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"

// PostsResponse represents a collection of posts from a subreddit with pagination info.
type PostsResponse struct {
	Posts  []*types.Post
	After  string // For pagination
	Before string // For pagination
}

// CommentsResponse represents a post with its comments and more IDs for loading truncated comments.
type CommentsResponse struct {
	Post     *types.Post
	Comments []*types.Comment
	MoreIDs  []string // IDs of additional comments that can be loaded
}
