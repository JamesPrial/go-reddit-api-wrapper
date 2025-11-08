// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// MonitorSubreddits continuously monitors multiple subreddits for new posts and their comments.
//
// This function launches a separate goroutine for each subreddit to monitor, allowing concurrent
// monitoring of multiple subreddits. Each goroutine periodically fetches new posts and stores them
// in the provided storage backend. For each new post, comments are optionally fetched and stored
// based on the fetchComments parameter.
//
// Deduplication is performed across all monitored subreddits using a shared sync.Map. Posts are
// identified by their fullname (e.g., "t3_abc123") to prevent duplicate storage.
//
// The monitor runs until the context is cancelled or a fatal error occurs. Non-fatal errors
// (storage failures, network issues) are logged but do not stop the monitor.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - client: authenticated Reddit API client
//   - subreddits: slice of subreddit names to monitor (required, non-empty)
//   - interval: polling interval for fetching new posts (must be > 0)
//   - limit: maximum number of posts to fetch per poll (passed to Reddit API)
//   - fetchComments: whether to fetch and store comments for each new post
//   - store: storage backend for persisting posts and comments (required)
//
// Returns an error if:
//   - subreddits is empty
//   - interval is <= 0
//   - store is nil
//   - client is nil
//   - a fatal authentication or validation error occurs
//
// Graceful shutdown occurs when the context is cancelled. All goroutines will complete
// their current operation and exit cleanly.
func MonitorSubreddits(ctx context.Context, client *graw.Reddit, subreddits []string, interval time.Duration, limit int, fetchComments bool, store storage.Store) error {
	// Validate inputs
	if client == nil {
		return fmt.Errorf("client cannot be nil")
	}

	if len(subreddits) == 0 {
		return fmt.Errorf("subreddits list cannot be empty")
	}

	if interval <= 0 {
		return fmt.Errorf("interval must be greater than 0")
	}

	if store == nil {
		return fmt.Errorf("store cannot be nil")
	}

	// Create shared map for tracking seen posts across all subreddits
	seenPosts := &sync.Map{}

	// Periodically clear the seenPosts map to prevent unbounded memory growth
	// This is acceptable because re-processing old posts is harmless (database deduplication)
	go func() {
		ticker := time.NewTicker(6 * time.Hour) // Clear every 6 hours
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Replace with new map to free memory
				seenPosts = &sync.Map{}
				slog.Debug("cleared seen posts map to free memory")
			}
		}
	}()

	// Create wait group to track all monitor goroutines
	var wg sync.WaitGroup

	// Channel to collect fatal errors from goroutines
	errChan := make(chan error, len(subreddits))

	slog.Info("starting monitor", "subreddits", subreddits, "interval", interval, "limit", limit)

	// Launch a monitoring goroutine for each subreddit
	for _, subreddit := range subreddits {
		wg.Add(1)
		go func(sub string) {
			if err := monitorSingleSubreddit(ctx, client, sub, interval, limit, fetchComments, store, seenPosts, &wg); err != nil {
				// Only report fatal errors
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					errChan <- fmt.Errorf("monitor failed for %s: %w", sub, err)
				}
			}
		}(subreddit)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect all fatal errors from monitor goroutines
	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Return the first error if any occurred
	if len(errs) > 0 {
		// Log additional errors if multiple failures occurred
		for i, err := range errs[1:] {
			slog.Error("additional monitor error", "index", i+1, "error", err)
		}
		return errs[0]
	}

	slog.Info("monitor stopped", "subreddits", subreddits)
	return nil
}

// monitorSingleSubreddit monitors a single subreddit for new posts.
//
// This function runs in its own goroutine and periodically fetches new posts from the specified
// subreddit. Posts are filtered using the shared seenPosts map to prevent duplicates across all
// monitored subreddits. New posts are stored in the database, and their comments are optionally
// fetched and stored concurrently based on the fetchComments parameter.
//
// The function includes panic recovery to prevent a crash in one monitor from affecting others.
// Non-fatal errors (storage, network) are logged but do not stop the monitor. Fatal errors
// (authentication, validation) are returned and will stop the monitor.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - client: authenticated Reddit API client
//   - subreddit: name of the subreddit to monitor
//   - interval: polling interval for fetching new posts
//   - limit: maximum number of posts to fetch per poll
//   - fetchComments: whether to fetch and store comments for each new post
//   - store: storage backend for persisting posts and comments
//   - seenPosts: shared map for tracking seen posts across all monitors
//   - wg: wait group for coordinating goroutine shutdown
//
// Returns an error if a fatal error occurs. Returns nil on clean shutdown.
func monitorSingleSubreddit(ctx context.Context, client *graw.Reddit, subreddit string, interval time.Duration, limit int, fetchComments bool, store storage.Store, seenPosts *sync.Map, wg *sync.WaitGroup) error {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in monitor goroutine",
				"subreddit", subreddit,
				"panic", r,
				"stack", string(debug.Stack()))
		}
		wg.Done()
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("started monitoring subreddit", "subreddit", subreddit, "interval", interval)

	// Perform initial fetch immediately
	if err := fetchAndProcessPosts(ctx, client, subreddit, limit, fetchComments, store, seenPosts); err != nil {
		// Check if error is due to context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			slog.Info("initial fetch cancelled", "subreddit", subreddit, "reason", err)
			return nil
		}
		// Check if it's a fatal error
		if isFatalError(err) {
			return err
		}
		// Log non-fatal errors but continue
		slog.Error("failed to fetch initial posts", "subreddit", subreddit, "error", err)
	}

	// Continue monitoring on ticker interval
	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping monitor", "subreddit", subreddit, "reason", ctx.Err())
			return nil
		case <-ticker.C:
			slog.Debug("starting post fetch cycle", "subreddit", subreddit)
			if err := fetchAndProcessPosts(ctx, client, subreddit, limit, fetchComments, store, seenPosts); err != nil {
				// Check if error is due to context cancellation
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					slog.Info("post fetch cancelled", "subreddit", subreddit, "reason", err)
					return nil
				}
				// Check if it's a fatal error
				if isFatalError(err) {
					return err
				}
				// Log non-fatal errors but continue
				slog.Error("failed to fetch posts", "subreddit", subreddit, "error", err)
			}
		}
	}
}

// fetchAndProcessPosts fetches new posts from a subreddit, filters duplicates, stores them,
// and optionally fetches comments for each new post based on the fetchComments parameter.
func fetchAndProcessPosts(ctx context.Context, client *graw.Reddit, subreddit string, limit int, fetchComments bool, store storage.Store, seenPosts *sync.Map) error {
	// Check context before expensive operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Build the posts request
	request := &types.PostsRequest{
		Subreddit: subreddit,
		Pagination: types.Pagination{
			Limit: limit,
		},
	}

	// Fetch new posts from the API
	response, err := client.GetNew(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to fetch new posts: %w", err)
	}

	if response == nil || len(response.Posts) == 0 {
		slog.Debug("no posts found", "subreddit", subreddit)
		return nil
	}

	// Filter new posts (but don't mark as seen yet)
	newPosts := make([]*types.Post, 0, len(response.Posts))
	for _, post := range response.Posts {
		if post == nil || post.Name == "" {
			slog.Warn("skipping post with empty name", "subreddit", subreddit)
			continue
		}

		// Check if already seen
		if _, seen := seenPosts.Load(post.Name); !seen {
			newPosts = append(newPosts, post)
		}
	}

	if len(newPosts) == 0 {
		return nil
	}

	// Store posts FIRST
	if err := store.UpsertPosts(ctx, newPosts); err != nil {
		slog.Error("failed to store posts", "subreddit", subreddit, "count", len(newPosts), "error", err)
		return nil // Don't mark as seen on storage failure
	}

	// Mark as seen ONLY after successful storage
	for _, post := range newPosts {
		seenPosts.Store(post.Name, true)
	}

	slog.Info("fetched posts", "subreddit", subreddit, "total", len(response.Posts), "new", len(newPosts))

	// Fetch and store comments for each new post if enabled
	if fetchComments {
		var commentsWg sync.WaitGroup
		const maxConcurrentCommentRequests = 10 // Match reddit package constant
		semaphore := make(chan struct{}, maxConcurrentCommentRequests)

		for _, post := range newPosts {
			commentsWg.Add(1)
			go func(p *types.Post) {
				defer commentsWg.Done()

				// Acquire semaphore slot with context awareness
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-ctx.Done():
					slog.Warn("comment fetch cancelled during semaphore acquisition", "post_id", p.ID, "reason", ctx.Err())
					return
				}

				fetchAndStoreComments(ctx, client, store, p)
			}(post)
		}

		// Wait for all comment fetches to complete
		commentsWg.Wait()
	}

	return nil
}

// fetchAndStoreComments fetches and stores comments for a single post.
//
// This function extracts the subreddit and post ID from the post, fetches all comments
// using the Reddit API, and stores them in the database. The function includes panic
// recovery to prevent crashes and logs errors without returning them, as comment
// fetching failures are considered non-fatal.
//
// Parameters:
//   - ctx: context for request cancellation and timeouts
//   - client: authenticated Reddit API client
//   - store: storage backend for persisting comments
//   - post: the post whose comments should be fetched
func fetchAndStoreComments(ctx context.Context, client *graw.Reddit, store storage.Store, post *types.Post) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in fetchAndStoreComments",
				"post_id", post.ID,
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	// Extract subreddit and post ID
	subreddit := post.Subreddit
	postID := post.ID

	// Handle case where subreddit might be in the permalink
	if subreddit == "" && post.Permalink != "" {
		// Permalink format: /r/subreddit/comments/postid/title/
		parts := strings.Split(strings.Trim(post.Permalink, "/"), "/")
		if len(parts) >= 2 && parts[0] == "r" {
			subreddit = parts[1]
		}
	}

	if subreddit == "" || postID == "" {
		slog.Error("missing subreddit or post ID", "subreddit", subreddit, "post_id", postID, "permalink", post.Permalink)
		return
	}

	// Build the comments request
	request := &types.CommentsRequest{
		Subreddit: subreddit,
		PostID:    postID,
		Pagination: types.Pagination{
			Limit: 100, // Fetch up to 100 comments per post
		},
	}

	// Fetch comments from the API
	response, err := client.GetComments(ctx, request)
	if err != nil {
		slog.Error("failed to fetch comments", "subreddit", subreddit, "post_id", postID, "error", err)
		return
	}

	if response == nil {
		slog.Debug("no comments response", "subreddit", subreddit, "post_id", postID)
		return
	}

	// Store comments using the existing helper function
	if len(response.Comments) > 0 {
		if err := StoreCommentsFromResponse(ctx, store, response); err != nil {
			slog.Error("failed to store comments", "subreddit", subreddit, "post_id", postID, "count", len(response.Comments), "error", err)
			return
		}
		slog.Debug("stored comments for post", "subreddit", subreddit, "post_id", postID, "count", len(response.Comments))
	}
}

// isFatalError determines if an error should stop the monitor.
//
// Fatal errors include authentication failures and validation errors that indicate
// a configuration problem. Rate limiting and network errors are considered non-fatal
// and should be retried.
func isFatalError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types that are fatal
	var authErr *graw.AuthError
	var validationErr *graw.ValidationError
	var configErr *graw.ConfigError

	// Authentication and validation errors are fatal
	if errors.As(err, &authErr) || errors.As(err, &validationErr) || errors.As(err, &configErr) {
		return true
	}

	// All other errors (rate limit, network, API, parse, storage) are non-fatal
	return false
}
