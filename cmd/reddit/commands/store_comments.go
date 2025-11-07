// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// StoreComments stores a batch of comments to the database.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - store: storage backend for persisting comments
//   - comments: slice of comments to store
//
// Returns an error if:
//   - store is nil
//   - comments is nil
//   - storage operation fails
//
// On success, logs the number of comments stored.
func StoreComments(ctx context.Context, store storage.Store, comments []*types.Comment) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	if comments == nil {
		return fmt.Errorf("comments cannot be nil")
	}

	// Attempt to store all comments in a batch operation
	if err := store.UpsertComments(ctx, comments); err != nil {
		return fmt.Errorf("failed to store comments: %w", err)
	}

	slog.InfoContext(ctx, "stored comments", slog.Int("count", len(comments)))
	return nil
}

// StoreCommentsFromResponse stores a post and all its comments from a CommentsResponse to the database.
//
// This function extracts the post (if present) and all comments from the response,
// flattens the comment tree structure, and stores everything to the database.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - store: storage backend for persisting data
//   - resp: CommentsResponse containing a post and comment tree
//
// Returns an error if:
//   - store is nil
//   - resp is nil
//   - post storage fails
//   - comment storage fails
//
// On success, logs the number of posts and comments stored.
func StoreCommentsFromResponse(ctx context.Context, store storage.Store, resp *types.CommentsResponse) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	if resp == nil {
		return fmt.Errorf("response cannot be nil")
	}

	// Store the post if it exists in the response
	if resp.Post != nil {
		if err := store.UpsertPost(ctx, resp.Post); err != nil {
			return fmt.Errorf("failed to store post: %w", err)
		}
		slog.InfoContext(ctx, "stored post", slog.String("id", resp.Post.ID))
	}

	// Flatten the comment tree and store all comments
	flatComments := flattenComments(ctx, resp.Comments)
	if len(flatComments) > 0 {
		if err := StoreComments(ctx, store, flatComments); err != nil {
			return fmt.Errorf("failed to store comments from response: %w", err)
		}
	}

	return nil
}

// flattenComments extracts all comments from a comment tree,
// converting the tree structure (where comments have replies) into a flat slice.
//
// The function uses a stack-based approach to avoid recursion depth limits and includes
// safety measures to prevent infinite loops, excessive memory usage, and resource exhaustion:
//   - Circular reference detection via a seen map
//   - Maximum comment limit (50000 comments)
//   - Context cancellation support
//   - Nil comment handling
//   - Empty comment ID handling
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - comments: slice of root-level comments, each potentially having nested replies
//
// Returns a flat slice containing all comments from the tree.
// If limits are exceeded or context is cancelled, a warning is logged and processing stops.
func flattenComments(ctx context.Context, comments []*types.Comment) []*types.Comment {
	if len(comments) == 0 {
		return []*types.Comment{}
	}

	const (
		maxFlattenedComments = 50000
		contextCheckInterval = 10
	)

	result := make([]*types.Comment, 0)
	seen := make(map[string]bool)
	iterations := 0

	// Use a stack-based approach to avoid recursion depth limits
	stack := make([]*types.Comment, len(comments))
	copy(stack, comments)

	for len(stack) > 0 {
		iterations++

		// Check context cancellation every contextCheckInterval iterations
		if iterations%contextCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				slog.WarnContext(ctx, "comment flattening cancelled", slog.String("reason", err.Error()), slog.Int("comments_processed", len(result)))
				break
			}
		}

		// Check flattened comment count limit
		if len(result) >= maxFlattenedComments {
			slog.WarnContext(ctx, "flattened comment limit exceeded", slog.Int("max_comments", maxFlattenedComments), slog.Int("stack_remaining", len(stack)))
			break
		}

		// Pop from stack
		comment := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Skip nil comments
		if comment == nil {
			slog.WarnContext(ctx, "skipping nil comment from stack")
			continue
		}

		// Skip comments with empty IDs
		if comment.ID == "" {
			slog.WarnContext(ctx, "skipping comment with empty ID")
			continue
		}

		// Check for circular references
		if seen[comment.ID] {
			slog.WarnContext(ctx, "skipping circular reference in comment tree", slog.String("comment_id", comment.ID))
			continue
		}

		// Mark comment as seen
		seen[comment.ID] = true

		// Add current comment to result
		result = append(result, comment)

		// Push replies onto stack (in reverse order to maintain original order when popping)
		if len(comment.Replies) > 0 {
			for i := len(comment.Replies) - 1; i >= 0; i-- {
				// Skip nil replies
				if comment.Replies[i] != nil {
					stack = append(stack, comment.Replies[i])
				}
			}
		}
	}

	return result
}
