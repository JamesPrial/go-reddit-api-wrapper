package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
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
		return fmt.Errorf("UpsertPost: post cannot be nil")
	}
	if post.ID == "" {
		return fmt.Errorf("UpsertPost: post.ID cannot be empty")
	}
	if post.Name == "" {
		return fmt.Errorf("UpsertPost: post.Name cannot be empty")
	}
	if post.Subreddit == "" {
		return fmt.Errorf("UpsertPost: post.Subreddit cannot be empty")
	}

	s.logger.Debug("upserting post", "post_id", post.ID, "subreddit", post.Subreddit)

	// Build the upsert query with all 36 columns
	query := `
		INSERT INTO posts (
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio, fetched_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, strftime('%s', 'now')
		)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			score = excluded.score,
			ups = excluded.ups,
			downs = excluded.downs,
			likes = excluded.likes,
			created = excluded.created,
			created_utc = excluded.created_utc,
			author = excluded.author,
			author_flair_css_class = excluded.author_flair_css_class,
			author_flair_text = excluded.author_flair_text,
			clicked = excluded.clicked,
			domain = excluded.domain,
			hidden = excluded.hidden,
			is_self = excluded.is_self,
			link_flair_css_class = excluded.link_flair_css_class,
			link_flair_text = excluded.link_flair_text,
			locked = excluded.locked,
			media = excluded.media,
			media_embed = excluded.media_embed,
			num_comments = excluded.num_comments,
			over_18 = excluded.over_18,
			permalink = excluded.permalink,
			saved = excluded.saved,
			selftext = excluded.selftext,
			selftext_html = excluded.selftext_html,
			subreddit = excluded.subreddit,
			subreddit_id = excluded.subreddit_id,
			thumbnail = excluded.thumbnail,
			title = excluded.title,
			url = excluded.url,
			edited_is_edited = excluded.edited_is_edited,
			edited_timestamp = excluded.edited_timestamp,
			distinguished = excluded.distinguished,
			stickied = excluded.stickied,
			upvote_ratio = excluded.upvote_ratio,
			fetched_at = strftime('%s', 'now')
	`

	// Convert post to insert arguments
	args := postToInsertArgs(post)

	// Execute the upsert
	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("UpsertPost: failed to insert post %s: %w", post.ID, err)
	}

	s.logger.Debug("successfully upserted post", "post_id", post.ID)
	return nil
}

// GetPost retrieves a post by its ID (without prefix, e.g., "abc123").
// Returns the post if found.
// Returns sql.ErrNoRows if the post is not found (caller should check this).
// Returns an error for other database failures.
func (s *SQLiteStore) GetPost(ctx context.Context, id string) (*types.Post, error) {
	s.logger.Debug("getting post", "post_id", id)

	query := `
		SELECT
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio
		FROM posts
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)

	// Create a scan destination and scan into it
	dest := newPostScanDest()

	err := row.Scan(dest.dest()...)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return sql.ErrNoRows as-is for caller to handle
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("GetPost: failed to scan post %s: %w", id, err)
	}

	post := dest.toPost()
	s.logger.Debug("successfully retrieved post", "post_id", id)
	return post, nil
}

// ListPosts retrieves posts matching the specified criteria.
// Returns an empty slice if no posts match the criteria.
// The opts parameter allows filtering by subreddit, author, score, age, and sorting.
// Returns an error if the operation fails.
func (s *SQLiteStore) ListPosts(ctx context.Context, opts *ListPostsOptions) ([]*types.Post, error) {
	s.logger.Debug("listing posts", "opts", opts)

	// Build base query
	var query strings.Builder
	query.WriteString(`
		SELECT
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio
		FROM posts
	`)

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
		return nil, fmt.Errorf("ListPosts: invalid sort parameters")
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
		return nil, fmt.Errorf("ListPosts: failed to execute query: %w", err)
	}
	defer rows.Close()

	// Scan results
	var posts []*types.Post
	for rows.Next() {
		dest := newPostScanDest()

		if err := rows.Scan(dest.dest()...); err != nil {
			return nil, fmt.Errorf("ListPosts: failed to scan post: %w", err)
		}

		post := dest.toPost()
		posts = append(posts, post)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListPosts: error iterating rows: %w", err)
	}

	s.logger.Debug("successfully listed posts", "count", len(posts))

	// Return empty slice if no results (not an error)
	if posts == nil {
		return []*types.Post{}, nil
	}

	return posts, nil
}

// DeletePost removes a post by its ID (without prefix, e.g., "abc123").
// Returns nil even if the post doesn't exist (idempotent delete).
// Comments cascade automatically via foreign key constraints.
// Returns an error only if the operation fails for other reasons.
func (s *SQLiteStore) DeletePost(ctx context.Context, id string) error {
	s.logger.Debug("deleting post", "post_id", id)

	query := `DELETE FROM posts WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("DeletePost: failed to delete post %s: %w", id, err)
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
		return fmt.Errorf("UpsertPosts: failed to begin transaction: %w", err)
	}

	// Ensure rollback on error or panic (safe to call after Commit)
	defer tx.Rollback()

	// Prepare the upsert statement
	query := `
		INSERT INTO posts (
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio, fetched_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, strftime('%s', 'now')
		)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			score = excluded.score,
			ups = excluded.ups,
			downs = excluded.downs,
			likes = excluded.likes,
			created = excluded.created,
			created_utc = excluded.created_utc,
			author = excluded.author,
			author_flair_css_class = excluded.author_flair_css_class,
			author_flair_text = excluded.author_flair_text,
			clicked = excluded.clicked,
			domain = excluded.domain,
			hidden = excluded.hidden,
			is_self = excluded.is_self,
			link_flair_css_class = excluded.link_flair_css_class,
			link_flair_text = excluded.link_flair_text,
			locked = excluded.locked,
			media = excluded.media,
			media_embed = excluded.media_embed,
			num_comments = excluded.num_comments,
			over_18 = excluded.over_18,
			permalink = excluded.permalink,
			saved = excluded.saved,
			selftext = excluded.selftext,
			selftext_html = excluded.selftext_html,
			subreddit = excluded.subreddit,
			subreddit_id = excluded.subreddit_id,
			thumbnail = excluded.thumbnail,
			title = excluded.title,
			url = excluded.url,
			edited_is_edited = excluded.edited_is_edited,
			edited_timestamp = excluded.edited_timestamp,
			distinguished = excluded.distinguished,
			stickied = excluded.stickied,
			upvote_ratio = excluded.upvote_ratio,
			fetched_at = strftime('%s', 'now')
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("UpsertPosts: failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Execute statement for each post
	for i, post := range posts {
		if post == nil {
			return fmt.Errorf("UpsertPosts: post at index %d is nil", i)
		}
		args := postToInsertArgs(post)
		_, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return fmt.Errorf("UpsertPosts: failed to insert post %s: %w", post.ID, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("UpsertPosts: failed to commit transaction: %w", err)
	}

	s.logger.Debug("successfully upserted posts batch", "count", len(posts))
	return nil
}
