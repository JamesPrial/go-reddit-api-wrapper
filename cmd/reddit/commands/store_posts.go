// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// StorePosts stores a collection of posts to the database using batch operations.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - store: storage backend for persisting posts
//   - posts: slice of posts to store
//
// Returns an error if:
//   - store is nil
//   - posts is nil
//   - any post in the slice is nil
//   - context is cancelled
//   - storage operation fails
//
// On success, logs the number of posts stored.
func StorePosts(ctx context.Context, store storage.Store, posts []*types.Post) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	if posts == nil {
		return fmt.Errorf("posts cannot be nil")
	}

	// Validate that no posts in the slice are nil
	for i, post := range posts {
		if post == nil {
			return fmt.Errorf("post at index %d cannot be nil", i)
		}
	}

	// Check context before expensive operation
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Perform batch upsert for efficiency
	if err := store.UpsertPosts(ctx, posts); err != nil {
		return fmt.Errorf("failed to store posts: %w", err)
	}

	// Log success with number of posts stored
	slog.InfoContext(ctx, "posts stored successfully",
		slog.Int("post_count", len(posts)),
	)

	return nil
}

// StorePostsFromResponse extracts posts from a PostsResponse and stores them to the database.
//
// This is a convenience function that handles the common pattern of:
// 1. Fetching posts via the Reddit API (which returns a PostsResponse)
// 2. Storing those posts to the database
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - store: storage backend for persisting posts
//   - resp: API response containing posts to store
//
// Returns an error if:
//   - store is nil
//   - response is nil
//   - response posts field is nil
//   - storage operation fails
//
// This function extracts the Posts slice from the response and delegates to StorePosts.
func StorePostsFromResponse(ctx context.Context, store storage.Store, resp *types.PostsResponse) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	if resp == nil {
		return fmt.Errorf("response cannot be nil")
	}

	if resp.Posts == nil {
		return fmt.Errorf("response posts cannot be nil")
	}

	// Extract posts from response and store them
	return StorePosts(ctx, store, resp.Posts)
}
