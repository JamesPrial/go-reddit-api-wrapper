package storage

import (
	"context"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Store defines the interface for persistent storage of Reddit posts and comments.
// This is a pure storage layer without caching logic or business rules.
// All operations accept a context for cancellation and timeout control.
// Implementations should return standard errors with appropriate context.
type Store interface {
	// PostOperations defines operations for managing posts.
	PostOperations

	// CommentOperations defines operations for managing comments.
	CommentOperations

	// UtilityOperations defines utility operations for store management.
	UtilityOperations
}

// PostOperations defines storage operations for Reddit posts.
type PostOperations interface {
	// UpsertPost inserts a new post or updates an existing post if it already exists.
	// The post ID (post.ID) is used as the unique identifier.
	// Returns an error if the operation fails.
	UpsertPost(ctx context.Context, post *types.Post) error

	// GetPost retrieves a post by its ID (without prefix, e.g., "abc123").
	// Returns the post if found, or nil with an error if not found.
	// Implementations should return a distinguishable error for "not found" cases.
	GetPost(ctx context.Context, id string) (*types.Post, error)

	// ListPosts retrieves posts matching the specified criteria.
	// Returns an empty slice if no posts match the criteria.
	// The opts parameter allows filtering by subreddit, author, score, age, and sorting.
	// Returns an error if the operation fails.
	ListPosts(ctx context.Context, opts *ListPostsOptions) ([]*types.Post, error)

	// CountPosts returns the total number of posts matching the specified criteria.
	// It applies the same filters as ListPosts (subreddit, author, score, age) but
	// ignores pagination parameters (Limit, Offset) and returns only the count.
	// Sorting parameters are also ignored as they don't affect the count.
	// Returns an error if the operation fails.
	CountPosts(ctx context.Context, opts *ListPostsOptions) (int64, error)

	// DeletePost removes a post by its ID (without prefix, e.g., "abc123").
	// Returns an error if the operation fails.
	// Implementations may choose to return an error if the post doesn't exist,
	// or succeed silently (idempotent delete).
	DeletePost(ctx context.Context, id string) error

	// UpsertPosts performs a batch upsert of multiple posts.
	// Each post is inserted or updated based on its ID.
	// Returns an error if any operation fails.
	// Implementations should aim for transactional behavior where possible.
	UpsertPosts(ctx context.Context, posts []*types.Post) error
}

// CommentOperations defines storage operations for Reddit comments.
type CommentOperations interface {
	// UpsertComment inserts a new comment or updates an existing comment if it already exists.
	// The comment ID (comment.ID) is used as the unique identifier.
	// Returns an error if the operation fails.
	UpsertComment(ctx context.Context, comment *types.Comment) error

	// GetComment retrieves a comment by its ID (without prefix, e.g., "xyz789").
	// Returns the comment if found, or nil with an error if not found.
	// Implementations should return a distinguishable error for "not found" cases.
	GetComment(ctx context.Context, id string) (*types.Comment, error)

	// GetCommentTree retrieves all comments for a specific post, optionally filtered
	// and sorted according to the provided options.
	// The postID should be without prefix (e.g., "abc123").
	// Returns comments in tree structure (with Replies populated) if the implementation supports it,
	// or as a flat list otherwise.
	// Returns an empty slice if no comments exist for the post.
	GetCommentTree(ctx context.Context, postID string, opts *CommentTreeOptions) ([]*types.Comment, error)

	// DeleteComment removes a comment by its ID (without prefix, e.g., "xyz789").
	// Returns an error if the operation fails.
	// Implementations may choose to return an error if the comment doesn't exist,
	// or succeed silently (idempotent delete).
	DeleteComment(ctx context.Context, id string) error

	// UpsertComments performs a batch upsert of multiple comments.
	// Each comment is inserted or updated based on its ID.
	// Returns an error if any operation fails.
	// Implementations should aim for transactional behavior where possible.
	UpsertComments(ctx context.Context, comments []*types.Comment) error
}

// UtilityOperations defines utility operations for store management and monitoring.
type UtilityOperations interface {
	// Close cleanly shuts down the store, releasing any resources.
	// Should be called when the store is no longer needed.
	// Returns an error if cleanup fails.
	Close() error

	// Ping verifies that the store is accessible and operational.
	// Returns an error if the store cannot be reached or is not functioning.
	Ping(ctx context.Context) error

	// GetStats returns statistics about the stored data.
	// Returns an error if the operation fails.
	GetStats(ctx context.Context) (*CacheStats, error)

	// EvictStale removes entries older than the specified maxAge.
	// Returns the number of entries evicted, or an error if the operation fails.
	// The maxAge parameter specifies how old an entry must be to be considered stale,
	// measured from its creation time (Created/CreatedUTC fields).
	EvictStale(ctx context.Context, maxAge time.Duration) (int64, error)
}

// ListPostsOptions specifies criteria for listing posts.
// All fields are optional. If a field is not set (zero value), it is not used as a filter.
type ListPostsOptions struct {
	// Subreddit filters posts to a specific subreddit (e.g., "golang").
	// Empty string means no subreddit filter.
	Subreddit string

	// Author filters posts by author username (e.g., "johndoe").
	// Empty string means no author filter.
	Author string

	// MinScore filters posts to those with a score greater than or equal to this value.
	// Zero means no minimum score filter.
	MinScore int

	// MaxAge filters posts to those created within this duration from now.
	// Zero means no age filter.
	MaxAge time.Duration

	// SortBy specifies the field to sort by.
	// Common values: "created_utc", "score", "num_comments", "title".
	// Empty string uses implementation default (typically "created_utc").
	SortBy string

	// SortDir specifies the sort direction.
	// Valid values: "asc" (ascending), "desc" (descending).
	// Empty string uses implementation default (typically "desc").
	SortDir string

	// Limit specifies the maximum number of posts to return.
	// Zero means no limit (use with caution).
	Limit int

	// Offset specifies the number of posts to skip before returning results.
	// Used for pagination. Zero means start from the beginning.
	Offset int
}

// CommentTreeOptions specifies criteria for retrieving comment trees.
// All fields are optional.
type CommentTreeOptions struct {
	// MaxDepth specifies the maximum depth of replies to retrieve.
	// 0 means unlimited depth (retrieve entire tree).
	// 1 means only top-level comments (no replies).
	// 2 means top-level comments plus one level of replies, etc.
	MaxDepth int

	// SortBy specifies the field to sort comments by.
	// Common values: "score", "created_utc".
	// Empty string uses implementation default (typically "score").
	SortBy string

	// SortDir specifies the sort direction.
	// Valid values: "asc" (ascending), "desc" (descending).
	// Empty string uses implementation default (typically "desc" for score, "asc" for created_utc).
	SortDir string
}

// CacheStats provides statistics about the stored data.
// All fields represent counts or metadata about the current state of the store.
type CacheStats struct {
	// PostCount is the total number of posts stored.
	PostCount int64

	// CommentCount is the total number of comments stored.
	CommentCount int64

	// OldestEntry is the creation time of the oldest entry (post or comment) in the store.
	// Zero value indicates no entries exist.
	OldestEntry time.Time

	// NewestEntry is the creation time of the newest entry (post or comment) in the store.
	// Zero value indicates no entries exist.
	NewestEntry time.Time

	// TotalSizeBytes is the approximate total size of stored data in bytes.
	// This is implementation-specific and may be an estimate.
	// Zero indicates either no data or size tracking is not supported.
	TotalSizeBytes int64
}
