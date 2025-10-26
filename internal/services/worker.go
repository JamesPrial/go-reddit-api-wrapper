package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"gorm.io/gorm"
)

// job represents a single unit of work for a worker - polling one subreddit.
// Workers process jobs from a channel, fetching posts and comments for the subreddit.
type job struct {
	subreddit db.Subreddit
}

// worker processes jobs from a job channel.
// Each worker:
//   - Fetches hot posts from the subreddit using the Reddit API
//   - Stores new posts in the database (deduplication via fullname uniqueness)
//   - Fetches comments for each new post
//   - Stores comments in the database
//   - Logs progress and errors with structured logging
type worker struct {
	id           int
	jobs         <-chan job
	redditClient *graw.Reddit
	repository   db.Repository
	logger       *slog.Logger
}

// newWorker creates a new worker instance.
// Parameters:
//   - id: Worker identifier for logging
//   - jobs: Channel from which to receive jobs
//   - redditClient: Reddit API client for fetching data
//   - repository: Database repository for storing data
//   - logger: Structured logger for diagnostics
func newWorker(id int, jobs <-chan job, redditClient *graw.Reddit, repository db.Repository, logger *slog.Logger) *worker {
	return &worker{
		id:           id,
		jobs:         jobs,
		redditClient: redditClient,
		repository:   repository,
		logger:       logger,
	}
}

// start begins processing jobs from the job channel.
// This method blocks until the context is cancelled or the jobs channel is closed.
// It implements panic recovery to prevent a single worker failure from deadlocking the pool.
func (w *worker) start(ctx context.Context) {
	w.logger.Info("worker started", slog.Int("worker_id", w.id))

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopping due to context cancellation", slog.Int("worker_id", w.id))
			return
		case j, ok := <-w.jobs:
			if !ok {
				w.logger.Info("worker stopping - jobs channel closed", slog.Int("worker_id", w.id))
				return
			}

			// Process job with panic recovery
			func() {
				defer func() {
					if r := recover(); r != nil {
						w.logger.Error("worker panic recovered",
							slog.Int("worker_id", w.id),
							slog.String("subreddit", j.subreddit.Name),
							slog.Any("panic", r))
					}
				}()

				if err := w.process(ctx, j); err != nil {
					w.logger.Error("failed to process job",
						slog.Int("worker_id", w.id),
						slog.String("subreddit", j.subreddit.Name),
						slog.String("error", err.Error()))
				}
			}()
		}
	}
}

// process handles a single job: fetch posts, store them, fetch comments, store comments.
// Error handling strategy:
//   - Network/API errors are logged but don't stop the worker
//   - Database errors for duplicates are ignored (idempotent operations)
//   - Other database errors are logged and returned
//   - Individual post/comment failures don't prevent processing other items
func (w *worker) process(ctx context.Context, j job) error {
	w.logger.Debug("processing job",
		slog.Int("worker_id", w.id),
		slog.String("subreddit", j.subreddit.Name))

	// Fetch hot posts from the subreddit (limit to 25 posts per poll)
	postsResp, err := w.redditClient.GetHot(ctx, &types.PostsRequest{
		Subreddit: j.subreddit.Name,
		Pagination: types.Pagination{
			Limit: 25,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to fetch hot posts: %w", err)
	}

	w.logger.Info("fetched posts",
		slog.Int("worker_id", w.id),
		slog.String("subreddit", j.subreddit.Name),
		slog.Int("post_count", len(postsResp.Posts)))

	// Process each post
	newPostCount := 0
	for _, redditPost := range postsResp.Posts {
		if redditPost == nil {
			continue
		}

		// Convert Reddit post to database model
		dbPost := PostToModel(redditPost, j.subreddit.ID)
		if dbPost == nil {
			continue
		}

		// Check if post already exists
		existingPost, err := w.repository.GetPost(ctx, dbPost.Fullname)
		if err == nil {
			// Post exists - update it
			existingPost.Score = dbPost.Score
			existingPost.NumComments = dbPost.NumComments
			if err := w.repository.UpdatePost(ctx, existingPost); err != nil {
				w.logger.Warn("failed to update existing post",
					slog.String("fullname", dbPost.Fullname),
					slog.String("error", err.Error()))
			}
			continue
		}

		// Check if error is "not found" - if so, create the post
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Warn("failed to check if post exists",
				slog.String("fullname", dbPost.Fullname),
				slog.String("error", err.Error()))
			continue
		}

		// Store new post in database
		if err := w.repository.CreatePost(ctx, dbPost); err != nil {
			w.logger.Warn("failed to store post",
				slog.String("fullname", dbPost.Fullname),
				slog.String("title", dbPost.Title),
				slog.String("error", err.Error()))
			continue
		}

		newPostCount++

		// Fetch comments for the new post
		// Extract post ID without "t3_" prefix for the GetComments API
		postID := redditPost.ID
		if postID == "" {
			w.logger.Warn("post has no ID, skipping comment fetch",
				slog.String("fullname", dbPost.Fullname))
			continue
		}

		commentsResp, err := w.redditClient.GetComments(ctx, &types.CommentsRequest{
			Subreddit: j.subreddit.Name,
			PostID:    postID,
			Pagination: types.Pagination{
				Limit: 100, // Fetch up to 100 comments per post
			},
		})
		if err != nil {
			w.logger.Warn("failed to fetch comments",
				slog.String("post_id", postID),
				slog.String("error", err.Error()))
			continue
		}

		if len(commentsResp.Comments) > 0 {
			// Convert Reddit comments to database models
			dbComments := CommentsToModels(commentsResp.Comments, dbPost.ID)

			// Store comments in database (batch insert for efficiency)
			if err := w.repository.CreateComments(ctx, dbComments); err != nil {
				w.logger.Warn("failed to store comments",
					slog.String("post_id", postID),
					slog.Int("comment_count", len(dbComments)),
					slog.String("error", err.Error()))
				continue
			}

			w.logger.Debug("stored comments",
				slog.Int("worker_id", w.id),
				slog.String("post_id", postID),
				slog.Int("comment_count", len(dbComments)))
		}
	}

	w.logger.Info("job completed",
		slog.Int("worker_id", w.id),
		slog.String("subreddit", j.subreddit.Name),
		slog.Int("new_posts", newPostCount))

	return nil
}
