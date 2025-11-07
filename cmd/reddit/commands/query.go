// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// ListStoredPosts queries stored posts with optional filtering and displays them.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - store: storage backend for accessing stored posts
//   - cfg: configuration including filters and output options
//
// Configuration fields used:
//   - cfg.SubredditFilter: optional filter for specific subreddit
//   - cfg.MinScoreFilter: optional minimum score threshold
//   - cfg.Limit: number of posts to retrieve (default: 25)
//   - cfg.Output: output format (text, json, table)
//
// Returns an error if:
//   - store is nil
//   - cfg is nil
//   - query fails
//   - formatting fails
func ListStoredPosts(ctx context.Context, store storage.Store, cfg *config.Config) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	logger := slog.Default()

	// Build list options from config
	opts := &storage.ListPostsOptions{
		Subreddit: cfg.SubredditFilter,
		MinScore:  cfg.MinScoreFilter,
		Limit:     cfg.Limit,
		SortBy:    "created_utc",
		SortDir:   "desc",
	}

	logger.Debug("listing stored posts", "subreddit", cfg.SubredditFilter, "minScore", cfg.MinScoreFilter, "limit", cfg.Limit)

	// Check context before querying
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Query posts from storage
	posts, err := store.ListPosts(ctx, opts)
	if err != nil {
		logger.Error("failed to query posts", "error", err)
		return fmt.Errorf("failed to query stored posts: %w", err)
	}

	if len(posts) == 0 {
		logger.Info("no posts found matching criteria")
		fmt.Println("No posts found matching the criteria.")
		return nil
	}

	logger.Info("retrieved posts", "count", len(posts))

	// Create formatter based on config
	formatter, err := output.New(output.Config{
		Writer: os.Stdout,
		Format: cfg.Output,
	})
	if err != nil {
		logger.Error("failed to create formatter", "error", err, "format", cfg.Output)
		return fmt.Errorf("failed to create formatter: %w", err)
	}

	// Format and output the posts
	if err := formatter.FormatPosts(posts); err != nil {
		logger.Error("failed to format posts", "error", err)
		return fmt.Errorf("failed to format posts: %w", err)
	}

	return nil
}

// ShowStats retrieves and displays statistics about the stored data.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - store: storage backend for accessing statistics
//   - cfg: configuration (used for logging consistency)
//
// Displays:
//   - Post count: total number of stored posts
//   - Comment count: total number of stored comments
//   - Oldest entry: creation time of the oldest entry
//   - Newest entry: creation time of the newest entry
//
// Returns an error if:
//   - store is nil
//   - cfg is nil
//   - query fails
//   - formatting fails
func ShowStats(ctx context.Context, store storage.Store, cfg *config.Config) error {
	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	logger := slog.Default()

	logger.Debug("retrieving storage statistics")

	// Check context before querying
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Get statistics from storage
	stats, err := store.GetStats(ctx)
	if err != nil {
		logger.Error("failed to get statistics", "error", err)
		return fmt.Errorf("failed to get storage statistics: %w", err)
	}

	if stats == nil {
		logger.Error("received nil statistics")
		return fmt.Errorf("received nil statistics from store")
	}

	logger.Debug("retrieved statistics", "posts", stats.PostCount, "comments", stats.CommentCount)

	// Format and display statistics
	if err := formatStats(os.Stdout, stats); err != nil {
		logger.Error("failed to format statistics", "error", err)
		return fmt.Errorf("failed to format statistics: %w", err)
	}

	return nil
}

// formatStats formats and displays cache statistics.
// This helper function writes the statistics to the provided writer.
func formatStats(w io.Writer, stats *storage.CacheStats) error {
	lines := []string{
		"Storage Statistics",
		"==================",
		"",
		fmt.Sprintf("Posts Stored:      %d", stats.PostCount),
		fmt.Sprintf("Comments Stored:   %d", stats.CommentCount),
	}

	// Add oldest entry timestamp if available
	if !stats.OldestEntry.IsZero() {
		lines = append(lines, fmt.Sprintf("Oldest Entry:      %s", formatTime(stats.OldestEntry)))
	} else {
		lines = append(lines, "Oldest Entry:      (no data)")
	}

	// Add newest entry timestamp if available
	if !stats.NewestEntry.IsZero() {
		lines = append(lines, fmt.Sprintf("Newest Entry:      %s", formatTime(stats.NewestEntry)))
	} else {
		lines = append(lines, "Newest Entry:      (no data)")
	}

	// Add total size if available
	if stats.TotalSizeBytes > 0 {
		lines = append(lines, fmt.Sprintf("Total Size:        %s", formatBytes(stats.TotalSizeBytes)))
	}

	lines = append(lines, "")

	// Write all lines to output
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}

	return nil
}

// formatTime formats a time.Time as a human-readable string.
func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05 MST")
}

// formatBytes converts bytes to human-readable format (B, KB, MB, GB).
func formatBytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}

	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
