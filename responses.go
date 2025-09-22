package graw

import "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"

// PostsResponse represents a collection of posts from a subreddit
type PostsResponse struct {
	Posts  []*types.Post
	After  string // For pagination
	Before string // For pagination
}

// CommentsResponse represents a post with its comments
type CommentsResponse struct {
	Post     *types.Post
	Comments []*types.Comment
	MoreIDs  []string // IDs of additional comments that can be loaded
}

// SubredditResponse represents subreddit information
type SubredditResponse struct {
	Subreddit *types.SubredditData
}

// UserResponse represents user account information
type UserResponse struct {
	User *types.AccountData
}

// Parser interface for parsing Reddit API responses
type Parser interface {
	ParseThing(thing *types.Thing) (any, error)
	ExtractPosts(listing *types.Thing) ([]*types.Post, error)
	ExtractComments(thing *types.Thing) ([]*types.Comment, []string, error)
	ExtractPostAndComments(response []*types.Thing) (*types.Post, []*types.Comment, []string, error)
}