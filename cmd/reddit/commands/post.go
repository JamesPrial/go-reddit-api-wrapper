// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// GetComments fetches comments for a specific post and formats the output.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - client: authenticated Reddit API client
//   - subreddit: name of the subreddit (required)
//   - postID: ID of the post without prefix (e.g., "abc123")
//   - pagination: pagination parameters (limit, after, before)
//   - formatter: output formatter for displaying results
//   - store: optional storage backend for persisting comments (can be nil to disable storage)
//
// Returns an error if:
//   - client is nil
//   - subreddit or postID is empty
//   - API call fails
//   - formatting fails
//
// Storage errors are logged but do not cause the command to fail, allowing graceful degradation
// when storage is unavailable.
func GetComments(ctx context.Context, client *graw.Reddit, subreddit, postID string, pagination types.Pagination, formatter output.Formatter, store storage.Store) error {
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	if subreddit == "" {
		return fmt.Errorf("subreddit name is required")
	}

	if postID == "" {
		return fmt.Errorf("post ID is required")
	}

	// Build the comments request
	request := &types.CommentsRequest{
		Subreddit: subreddit,
		PostID:    postID,
		Pagination: types.Pagination{
			Limit:  pagination.Limit,
			After:  pagination.After,
			Before: pagination.Before,
		},
	}

	// Fetch comments from the API
	response, err := client.GetComments(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}

	// Handle empty comment list
	if response == nil {
		return fmt.Errorf("received nil response from API")
	}

	// Store comments if storage is enabled
	if store != nil {
		if err := StoreCommentsFromResponse(ctx, store, response); err != nil {
			slog.Error("failed to store comments from response", "postID", postID, "error", err)
		}
	}

	if response.Post != nil && len(response.Comments) == 0 {
		// Post exists but has no comments - this is not an error, just empty results
		return formatter.FormatComments(response)
	}

	// Format and output the response
	return formatter.FormatComments(response)
}

// GetMoreComments loads additional comments referenced in a post's comment tree.
//
// When a post has many comments, Reddit truncates the list and includes "more comments"
// placeholders that reference comment IDs. This function loads those additional comments.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - client: authenticated Reddit API client
//   - linkID: full name or ID of the post (with or without "t3_" prefix)
//   - commentIDs: slice of comment IDs to load (without prefix)
//   - formatter: output formatter for displaying results
//   - store: optional storage backend for persisting comments (can be nil to disable storage)
//
// Returns an error if:
//   - client is nil
//   - linkID is empty
//   - commentIDs is empty or nil
//   - API call fails
//   - formatting fails
//
// Storage errors are logged but do not cause the command to fail, allowing graceful degradation
// when storage is unavailable.
//
// Note: The formatter will receive a CommentsResponse with a nil Post field
// and only the loaded Comments field populated.
func GetMoreComments(ctx context.Context, client *graw.Reddit, linkID string, commentIDs []string, formatter output.Formatter, store storage.Store) error {
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	if linkID == "" {
		return fmt.Errorf("link ID is required")
	}

	if len(commentIDs) == 0 {
		return fmt.Errorf("at least one comment ID is required")
	}

	// Build the more comments request
	request := &types.MoreCommentsRequest{
		LinkID:     linkID,
		CommentIDs: commentIDs,
	}

	// Fetch additional comments from the API
	comments, err := client.GetMoreComments(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to get more comments: %w", err)
	}

	// Create a response with only comments (no post data for "more comments")
	response := &types.CommentsResponse{
		Post:     nil,
		Comments: comments,
	}

	// Store comments if storage is enabled
	if store != nil {
		if len(comments) > 0 {
			if err := StoreComments(ctx, store, comments); err != nil {
				slog.Error("failed to store more comments", "linkID", linkID, "count", len(comments), "error", err)
			}
		}
	}

	// Format and output the response
	return formatter.FormatComments(response)
}
