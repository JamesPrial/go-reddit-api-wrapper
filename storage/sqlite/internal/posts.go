package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// isValidPostSortField returns true if the field is allowed for sorting posts
func isValidPostSortField(field string) bool {
	switch field {
	case "created_utc", "score", "num_comments", "fetched_at":
		return true
	default:
		return false
	}
}

// isValidSortDirection returns true if the direction is ASC or DESC
func isValidSortDirection(dir string) bool {
	return dir == "ASC" || dir == "DESC"
}

// UpsertPost inserts a new post or updates an existing post if it already exists.
// The post ID (post.ID) is used as the unique identifier.
// Sets fetched_at to current Unix timestamp on both insert and update.
// Returns an error if the operation fails.
func (s *SQLiteStore) UpsertPost(ctx context.Context, post *types.Post) error {
	if post == nil {
		return &storage.ValidationError{Operation: "UpsertPost", Field: "post", Reason: "post cannot be nil"}
	}
	if post.ID == "" {
		return &storage.ValidationError{Operation: "UpsertPost", Field: "post.ID", Value: post.ID, Reason: "post ID cannot be empty"}
	}
	if post.Name == "" {
		return &storage.ValidationError{Operation: "UpsertPost", Field: "post.Name", Reason: "post name cannot be empty"}
	}
	if post.Subreddit == "" {
		return &storage.ValidationError{Operation: "UpsertPost", Field: "post.Subreddit", Reason: "post subreddit cannot be empty"}
	}

	s.logger.Debug("upserting post", "post_id", post.ID, "subreddit", post.Subreddit)

	// Convert post to insert arguments
	args := postToInsertArgs(post)

	// Execute the upsert
	_, err := s.db.ExecContext(ctx, queryUpsertPost, args...)
	if err != nil {
		return &storage.DatabaseError{Operation: "UpsertPost", Message: fmt.Sprintf("failed to insert post %s", post.ID), Err: err}
	}

	s.logger.Debug("successfully upserted post", "post_id", post.ID)
	return nil
}

// GetPost retrieves a post by its ID (without prefix, e.g., "abc123").
// Returns the post if found.
// Returns NotFoundError if the post is not found.
// Returns an error for other database failures.
func (s *SQLiteStore) GetPost(ctx context.Context, id string) (*types.Post, error) {
	if id == "" {
		return nil, &storage.ValidationError{Operation: "GetPost", Field: "id", Reason: "post ID cannot be empty"}
	}

	s.logger.Debug("getting post", "post_id", id)

	row := s.db.QueryRowContext(ctx, queryGetPost, id)

	// Create a scan destination and scan into it
	dest := newPostScanDest()

	err := row.Scan(dest.dest()...)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return NotFoundError for caller to handle
			return nil, &storage.NotFoundError{ResourceType: "post", ResourceID: id}
		}
		return nil, &storage.DatabaseError{Operation: "GetPost", Message: fmt.Sprintf("failed to scan post %s", id), Err: err}
	}

	post := dest.toPost()
	s.logger.Debug("successfully retrieved post", "post_id", id)
	return post, nil
}

// ListPosts retrieves posts matching the specified criteria.
// Returns an empty slice if no posts match the criteria.
// The opts parameter allows filtering by subreddit, author, score, age, and sorting.
// Returns an error if the operation fails.
func (s *SQLiteStore) ListPosts(ctx context.Context, opts *storage.ListPostsOptions) ([]*types.Post, error) {
	s.logger.Debug("listing posts", "opts", opts)

	// Build base query
	var query strings.Builder
	query.WriteString(queryListPostsBase)

	// Build WHERE clauses and args
	var whereClauses []string
	var args []interface{}

	// Handle nil opts gracefully
	if opts != nil {
		// Subreddit filter (case-insensitive)
		if opts.Subreddit != "" {
			whereClauses = append(whereClauses, "LOWER(subreddit) = LOWER(?)")
			args = append(args, opts.Subreddit)
		}

		// Author filter
		if opts.Author != "" {
			whereClauses = append(whereClauses, "author = ?")
			args = append(args, opts.Author)
		}

		// MinScore filter
		if opts.MinScore > 0 {
			whereClauses = append(whereClauses, "score >= ?")
			args = append(args, opts.MinScore)
		}

		// MaxAge filter (compare fetched_at to current time minus maxAge)
		if opts.MaxAge > 0 {
			cutoffTime := time.Now().Unix() - int64(opts.MaxAge.Seconds())
			whereClauses = append(whereClauses, "fetched_at >= ?")
			args = append(args, cutoffTime)
		}
	}

	// Add WHERE clause if we have any filters
	if len(whereClauses) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(whereClauses, " AND "))
	}

	// Build ORDER BY clause
	orderBy := "created_utc" // Default sort field
	sortDir := "DESC"        // Default sort direction

	if opts != nil {
		// Validate sort parameters against whitelist
		if opts.SortBy != "" && isValidPostSortField(opts.SortBy) {
			orderBy = opts.SortBy
		}

		if opts.SortDir != "" {
			dir := strings.ToUpper(opts.SortDir)
			if isValidSortDirection(dir) {
				sortDir = dir
			}
		}
	}

	// SECURITY: orderBy and sortDir are validated against whitelists above.
	// They CANNOT be parameterized as they are SQL identifiers (column names).
	// Never remove the whitelist validation without replacing with equivalent protection.
	if !isValidPostSortField(orderBy) || !isValidSortDirection(sortDir) {
		return nil, &storage.ValidationError{
			Operation: "ListPosts",
			Field:     "sort parameters",
			Reason:    fmt.Sprintf("invalid sort field %q or direction %q", orderBy, sortDir),
		}
	}
	query.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderBy, sortDir))

	// Add LIMIT and OFFSET
	if opts != nil && opts.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, opts.Limit)

		if opts.Offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, opts.Offset)
		}
	}

	// Execute query
	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "ListPosts", Message: "failed to execute query", Err: err}
	}
	defer rows.Close()

	// Scan results
	var posts []*types.Post
	for rows.Next() {
		dest := newPostScanDest()

		if err := rows.Scan(dest.dest()...); err != nil {
			return nil, &storage.DatabaseError{Operation: "ListPosts", Message: "failed to scan post", Err: err}
		}

		post := dest.toPost()
		posts = append(posts, post)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, &storage.DatabaseError{
			Operation: "ListPosts",
			Message:   "error iterating rows",
			Err:       err,
		}
	}

	s.logger.Debug("successfully listed posts", "count", len(posts))

	// Return empty slice if no results (not an error)
	if posts == nil {
		return []*types.Post{}, nil
	}

	return posts, nil
}

// CountPosts returns the total number of posts matching the specified criteria.
// It applies the same filters as ListPosts (subreddit, author, score, age) but
// ignores pagination parameters (Limit, Offset) and sorting parameters.
// Returns 0 count (not an error) for empty results.
// Returns an error if the operation fails.
func (s *SQLiteStore) CountPosts(ctx context.Context, opts *storage.ListPostsOptions) (int64, error) {
	s.logger.Debug("counting posts", "opts", opts)

	// Build base query for counting
	var query strings.Builder
	query.WriteString("SELECT COUNT(*) FROM posts")

	// Build WHERE clauses and args
	var whereClauses []string
	var args []interface{}

	// Handle nil opts gracefully
	if opts != nil {
		// Subreddit filter (case-insensitive)
		if opts.Subreddit != "" {
			whereClauses = append(whereClauses, "LOWER(subreddit) = LOWER(?)")
			args = append(args, opts.Subreddit)
		}

		// Author filter
		if opts.Author != "" {
			whereClauses = append(whereClauses, "author = ?")
			args = append(args, opts.Author)
		}

		// MinScore filter
		if opts.MinScore > 0 {
			whereClauses = append(whereClauses, "score >= ?")
			args = append(args, opts.MinScore)
		}

		// MaxAge filter (compare fetched_at to current time minus maxAge)
		if opts.MaxAge > 0 {
			cutoffTime := time.Now().Unix() - int64(opts.MaxAge.Seconds())
			whereClauses = append(whereClauses, "fetched_at >= ?")
			args = append(args, cutoffTime)
		}
	}

	// Add WHERE clause if we have any filters
	if len(whereClauses) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(whereClauses, " AND "))
	}

	// Execute query
	var count int64
	row := s.db.QueryRowContext(ctx, query.String(), args...)
	err := row.Scan(&count)
	if err != nil {
		return 0, &storage.DatabaseError{Operation: "CountPosts", Message: "failed to execute count query", Err: err}
	}

	s.logger.Debug("successfully counted posts", "count", count)
	return count, nil
}

// DeletePost removes a post by its ID (without prefix, e.g., "abc123").
// Returns nil even if the post doesn't exist (idempotent delete).
// Comments cascade automatically via foreign key constraints.
// Returns an error only if the operation fails for other reasons.
func (s *SQLiteStore) DeletePost(ctx context.Context, id string) error {
	s.logger.Debug("deleting post", "post_id", id)

	_, err := s.db.ExecContext(ctx, queryDeletePost, id)
	if err != nil {
		return &storage.DatabaseError{Operation: "DeletePost", Message: fmt.Sprintf("failed to delete post %s", id), Err: err}
	}

	s.logger.Debug("successfully deleted post", "post_id", id)
	return nil
}

// UpsertPosts performs a batch upsert of multiple posts.
// Each post is inserted or updated based on its ID.
// Uses a transaction for atomicity - either all succeed or all fail.
// Returns an error if any operation fails.
// Handles empty slice gracefully (returns nil without error).
func (s *SQLiteStore) UpsertPosts(ctx context.Context, posts []*types.Post) error {
	// Handle empty slice gracefully
	if len(posts) == 0 {
		s.logger.Debug("no posts to upsert")
		return nil
	}

	s.logger.Debug("upserting posts batch", "count", len(posts))

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return &storage.TransactionError{Operation: "begin", Message: "UpsertPosts", Err: err}
	}

	// Ensure rollback on error or panic (safe to call after Commit)
	defer tx.Rollback()

	// Prepare the upsert statement
	stmt, err := tx.PrepareContext(ctx, queryUpsertPost)
	if err != nil {
		return &storage.DatabaseError{
			Operation: "UpsertPosts",
			Message:   "failed to prepare statement",
			Err:       err,
		}
	}
	defer stmt.Close()

	// Execute statement for each post
	for i, post := range posts {
		if post == nil {
			return &storage.ValidationError{Operation: "UpsertPosts", Field: fmt.Sprintf("posts[%d]", i), Reason: "post cannot be nil"}
		}
		args := postToInsertArgs(post)
		_, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return &storage.DatabaseError{
				Operation: "UpsertPosts",
				Message:   fmt.Sprintf("failed to insert post %s", post.ID),
				Err:       err,
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &storage.TransactionError{Operation: "commit", Message: "UpsertPosts", Err: err}
	}

	s.logger.Debug("successfully upserted posts batch", "count", len(posts))
	return nil
}
