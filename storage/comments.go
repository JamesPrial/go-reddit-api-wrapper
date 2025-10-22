package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// isValidCommentSortField returns true if the field is allowed for sorting comments
func isValidCommentSortField(field string) bool {
	switch field {
	case "score", "created_utc", "created":
		return true
	default:
		return false
	}
}

// UpsertComment inserts a new comment or updates an existing comment if it already exists.
// The comment ID (comment.ID) is used as the unique identifier.
// This method maintains the closure table for efficient tree queries.
// Returns an error if the operation fails.
func (s *SQLiteStore) UpsertComment(ctx context.Context, comment *types.Comment) error {
	// Validate input
	if comment == nil {
		return fmt.Errorf("UpsertComment: comment cannot be nil")
	}

	// Begin transaction for atomicity (comment + closure entries)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("UpsertComment: failed to begin transaction for %s: %w", comment.ID, err)
	}
	defer tx.Rollback() // Rollback if we don't reach Commit

	// Calculate depth from parent
	depth, err := calculateDepth(ctx, tx, comment.ParentID)
	if err != nil {
		return fmt.Errorf("UpsertComment: failed to calculate depth for %s: %w", comment.ID, err)
	}

	// Extract post_id from link_id
	postID := extractPostID(comment.LinkID)
	if postID == "" {
		return fmt.Errorf("UpsertComment: invalid link_id %s for comment %s", comment.LinkID, comment.ID)
	}

	s.logger.Debug("upserting comment",
		"comment_id", comment.ID,
		"parent_id", comment.ParentID,
		"depth", depth,
		"post_id", postID,
	)

	// Prepare insert/update statement
	// Using UPSERT (INSERT ... ON CONFLICT DO UPDATE)
	query := `
		INSERT INTO comments (
			id, name, score, ups, downs, likes, created, created_utc,
			approved_by, author, author_flair_css_class, author_flair_text, banned_by,
			body, body_html, edited_is_edited, edited_timestamp, gilded,
			link_author, link_id, link_title, link_url, num_reports, parent_id,
			saved, score_hidden, subreddit, subreddit_id, distinguished, depth, post_id, fetched_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
		)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			score = excluded.score,
			ups = excluded.ups,
			downs = excluded.downs,
			likes = excluded.likes,
			created = excluded.created,
			created_utc = excluded.created_utc,
			approved_by = excluded.approved_by,
			author = excluded.author,
			author_flair_css_class = excluded.author_flair_css_class,
			author_flair_text = excluded.author_flair_text,
			banned_by = excluded.banned_by,
			body = excluded.body,
			body_html = excluded.body_html,
			edited_is_edited = excluded.edited_is_edited,
			edited_timestamp = excluded.edited_timestamp,
			gilded = excluded.gilded,
			link_author = excluded.link_author,
			link_id = excluded.link_id,
			link_title = excluded.link_title,
			link_url = excluded.link_url,
			num_reports = excluded.num_reports,
			parent_id = excluded.parent_id,
			saved = excluded.saved,
			score_hidden = excluded.score_hidden,
			subreddit = excluded.subreddit,
			subreddit_id = excluded.subreddit_id,
			distinguished = excluded.distinguished,
			depth = excluded.depth,
			post_id = excluded.post_id,
			fetched_at = strftime('%s', 'now')
	`

	// Convert comment to insert args
	args := commentToInsertArgs(comment, depth)
	// Add post_id to args
	args = append(args, postID)

	// Execute upsert
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("UpsertComment: failed to upsert comment %s: %w", comment.ID, err)
	}

	// Delete existing closure entries for this comment (in case of reparenting)
	// This ensures we rebuild the closure table correctly
	deleteClosureQuery := `DELETE FROM comment_closures WHERE descendant = ?`
	if _, err := tx.ExecContext(ctx, deleteClosureQuery, comment.ID); err != nil {
		return fmt.Errorf("UpsertComment: failed to delete old closure entries for %s: %w", comment.ID, err)
	}

	// Insert closure entries
	if err := insertClosureEntries(ctx, tx, comment.ID, comment.ParentID); err != nil {
		return fmt.Errorf("UpsertComment: failed to insert closure entries for %s: %w", comment.ID, err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("UpsertComment: failed to commit transaction for %s: %w", comment.ID, err)
	}

	s.logger.Debug("comment upserted successfully", "comment_id", comment.ID)
	return nil
}

// GetComment retrieves a comment by its ID (without prefix, e.g., "xyz789").
// Returns the comment if found, or nil with sql.ErrNoRows if not found.
// Does NOT populate the Replies field - use GetCommentTree for hierarchical data.
func (s *SQLiteStore) GetComment(ctx context.Context, id string) (*types.Comment, error) {
	query := `
		SELECT
			id, name, score, ups, downs, likes, created, created_utc,
			approved_by, author, author_flair_css_class, author_flair_text, banned_by,
			body, body_html, edited_is_edited, edited_timestamp, gilded,
			link_author, link_id, link_title, link_url, num_reports, parent_id,
			saved, score_hidden, subreddit, subreddit_id, distinguished, depth
		FROM comments
		WHERE id = ?
	`

	dest := newCommentScanDest()
	err := s.db.QueryRowContext(ctx, query, id).Scan(dest.dest()...)
	if err != nil {
		// Return sql.ErrNoRows without wrapping (caller handles "not found")
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("GetComment: failed to query comment %s: %w", id, err)
	}

	comment := dest.toComment()
	s.logger.Debug("comment retrieved", "comment_id", id)
	return comment, nil
}

// UpsertComments performs a batch upsert of multiple comments.
// Comments are processed in dependency order (parents before children).
// Uses a transaction for atomicity - if any comment fails, all changes are rolled back.
// Returns an error if any operation fails.
func (s *SQLiteStore) UpsertComments(ctx context.Context, comments []*types.Comment) error {
	if len(comments) == 0 {
		return nil // Nothing to do
	}

	s.logger.Debug("upserting comments batch", "count", len(comments))

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("UpsertComments: failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build dependency graph from comment relationships
	type commentNode struct {
		comment  *types.Comment
		depth    int
		children []*commentNode
	}

	// Map comment ID to node
	nodeMap := make(map[string]*commentNode, len(comments))
	for i, c := range comments {
		if c == nil {
			return fmt.Errorf("UpsertComments: comment at index %d is nil", i)
		}
		if _, exists := nodeMap[c.ID]; exists {
			return fmt.Errorf("UpsertComments: duplicate comment ID %s in batch", c.ID)
		}
		nodeMap[c.ID] = &commentNode{comment: c, depth: -1} // depth -1 means not calculated yet
	}

	// Link children to parents and identify root comments
	var roots []*commentNode
	for _, node := range nodeMap {
		parentID := node.comment.ParentID

		// Check for self-reference before any other processing
		actualID := node.comment.ID
		if parentID != "" && (parentID == actualID || parentID == "t1_"+actualID) {
			return fmt.Errorf("UpsertComments: comment %s references itself as parent", actualID)
		}

		// Top-level comment must have post parent (t3_*)
		// Empty ParentID is invalid data (including whitespace-only)
		if strings.TrimSpace(parentID) == "" {
			return fmt.Errorf("UpsertComments: comment %s has empty parent_id", node.comment.ID)
		}
		if strings.HasPrefix(parentID, "t3_") {
			roots = append(roots, node)
			node.depth = 0 // Top-level comments have depth 0
			continue
		}

		// Extract actual parent ID (remove "t1_" prefix if present)
		actualParentID := parentID
		if strings.HasPrefix(parentID, "t1_") {
			actualParentID = strings.TrimPrefix(parentID, "t1_")
			if actualParentID == "" {
				return fmt.Errorf("UpsertComments: malformed parent_id %s for comment %s (empty after prefix)", parentID, node.comment.ID)
			}
		}

		// Link child to parent if parent is in this batch
		if parent, exists := nodeMap[actualParentID]; exists {
			parent.children = append(parent.children, node)
		} else {
			// Parent not in batch - it must already exist in DB
			// Calculate its depth from DB to determine this comment's depth
			parentDepth, err := calculateDepth(ctx, tx, parentID)
			if err != nil {
				return fmt.Errorf("UpsertComments: parent %s not in batch and not in database: %w", parentID, err)
			}
			node.depth = parentDepth + 1
			roots = append(roots, node) // Treat as root since parent already inserted
		}
	}

	// Calculate depths for comments whose parents are in the batch
	// Use BFS to process parents before children
	var orderedNodes []*commentNode
	queue := roots
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		orderedNodes = append(orderedNodes, current)

		// Set depth for children (parent depth + 1)
		for _, child := range current.children {
			child.depth = current.depth + 1
			queue = append(queue, child)
		}
	}

	// Verify ALL comments have depths calculated (check entire nodeMap, not just orderedNodes)
	for id, node := range nodeMap {
		if node.depth < 0 {
			return fmt.Errorf("UpsertComments: comment %s unreachable - possible cycle, orphaned comment, or self-reference", id)
		}
	}

	// Prepare statement for batch insert
	upsertCommentQuery := `
		INSERT INTO comments (
			id, name, score, ups, downs, likes, created, created_utc,
			approved_by, author, author_flair_css_class, author_flair_text, banned_by,
			body, body_html, edited_is_edited, edited_timestamp, gilded,
			link_author, link_id, link_title, link_url, num_reports, parent_id,
			saved, score_hidden, subreddit, subreddit_id, distinguished, depth, post_id, fetched_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
		)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			score = excluded.score,
			ups = excluded.ups,
			downs = excluded.downs,
			likes = excluded.likes,
			created = excluded.created,
			created_utc = excluded.created_utc,
			approved_by = excluded.approved_by,
			author = excluded.author,
			author_flair_css_class = excluded.author_flair_css_class,
			author_flair_text = excluded.author_flair_text,
			banned_by = excluded.banned_by,
			body = excluded.body,
			body_html = excluded.body_html,
			edited_is_edited = excluded.edited_is_edited,
			edited_timestamp = excluded.edited_timestamp,
			gilded = excluded.gilded,
			link_author = excluded.link_author,
			link_id = excluded.link_id,
			link_title = excluded.link_title,
			link_url = excluded.link_url,
			num_reports = excluded.num_reports,
			parent_id = excluded.parent_id,
			saved = excluded.saved,
			score_hidden = excluded.score_hidden,
			subreddit = excluded.subreddit,
			subreddit_id = excluded.subreddit_id,
			distinguished = excluded.distinguished,
			depth = excluded.depth,
			post_id = excluded.post_id,
			fetched_at = strftime('%s', 'now')
	`

	stmt, err := tx.PrepareContext(ctx, upsertCommentQuery)
	if err != nil {
		return fmt.Errorf("UpsertComments: failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	deleteClosureQuery := `DELETE FROM comment_closures WHERE descendant = ?`

	// Insert comments in dependency order (orderedNodes is already in BFS order)
	for _, node := range orderedNodes {
		comment := node.comment
		depth := node.depth

		// Extract post_id
		postID := extractPostID(comment.LinkID)
		if postID == "" {
			return fmt.Errorf("UpsertComments: invalid link_id %s for comment %s", comment.LinkID, comment.ID)
		}

		// Build arguments for insert
		args := commentToInsertArgs(comment, depth)
		args = append(args, postID)

		// Execute insert
		_, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return fmt.Errorf("UpsertComments: failed to upsert comment %s: %w", comment.ID, err)
		}

		// Clear old closure entries for this comment
		_, err = tx.ExecContext(ctx, deleteClosureQuery, comment.ID)
		if err != nil {
			return fmt.Errorf("UpsertComments: failed to delete old closure entries for %s: %w", comment.ID, err)
		}

		// Insert closure entries
		if err := insertClosureEntries(ctx, tx, comment.ID, comment.ParentID); err != nil {
			return fmt.Errorf("UpsertComments: failed to insert closure entries for %s: %w", comment.ID, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("UpsertComments: failed to commit transaction: %w", err)
	}

	s.logger.Debug("comments batch upserted successfully", "count", len(comments))
	return nil
}

// GetCommentTree retrieves all comments for a specific post in tree structure.
// The postID should be without prefix (e.g., "abc123").
// Returns comments with Replies populated according to the comment hierarchy.
// Returns an empty slice if no comments exist for the post.
// Supports filtering by MaxDepth and sorting via opts.
func (s *SQLiteStore) GetCommentTree(ctx context.Context, postID string, opts *CommentTreeOptions) ([]*types.Comment, error) {
	// Set default sort options
	sortBy := "created_utc"
	sortDir := "asc"
	maxDepth := 0 // 0 = unlimited

	if opts != nil {
		maxDepth = opts.MaxDepth

		// Validate sort field against whitelist
		if opts.SortBy != "" && isValidCommentSortField(opts.SortBy) {
			sortBy = opts.SortBy
		}

		// Validate sort direction
		if opts.SortDir != "" {
			dir := strings.ToUpper(opts.SortDir)
			if isValidSortDirection(dir) {
				sortDir = strings.ToLower(opts.SortDir)
			}
		}
	}

	// SECURITY: sortBy and sortDir are validated against whitelists above.
	// They CANNOT be parameterized as they are SQL identifiers (column names).
	// Never remove the whitelist validation without replacing with equivalent protection.

	s.logger.Debug("getting comment tree",
		"post_id", postID,
		"max_depth", maxDepth,
		"sort_by", sortBy,
		"sort_dir", sortDir,
	)

	// Build query with optional depth filter
	query := `
		SELECT DISTINCT
			c.id, c.name, c.score, c.ups, c.downs, c.likes, c.created, c.created_utc,
			c.approved_by, c.author, c.author_flair_css_class, c.author_flair_text, c.banned_by,
			c.body, c.body_html, c.edited_is_edited, c.edited_timestamp, c.gilded,
			c.link_author, c.link_id, c.link_title, c.link_url, c.num_reports, c.parent_id,
			c.saved, c.score_hidden, c.subreddit, c.subreddit_id, c.distinguished, c.depth
		FROM comments c
		WHERE c.post_id = ?
	`

	args := []interface{}{postID}

	// Add depth filter if specified
	if maxDepth > 0 {
		query += " AND c.depth < ?"
		args = append(args, maxDepth)
	}

	// Add ordering
	query += fmt.Sprintf(" ORDER BY c.parent_id, c.%s %s", sortBy, strings.ToUpper(sortDir))

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetCommentTree: failed to query comments for post %s: %w", postID, err)
	}
	defer rows.Close()

	// Scan all comments into a map
	commentMap := make(map[string]*types.Comment)
	var allComments []*types.Comment

	for rows.Next() {
		dest := newCommentScanDest()
		if err := rows.Scan(dest.dest()...); err != nil {
			return nil, fmt.Errorf("GetCommentTree: failed to scan comment: %w", err)
		}

		comment := dest.toComment()
		commentMap[comment.ID] = comment
		allComments = append(allComments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetCommentTree: error iterating comments: %w", err)
	}

	if len(allComments) == 0 {
		s.logger.Debug("no comments found for post", "post_id", postID)
		return []*types.Comment{}, nil // Empty slice, not error
	}

	// Build tree structure by linking Replies
	var topLevel []*types.Comment

	for _, comment := range allComments {
		// Check if this is a top-level comment
		// Top-level: parent_id is empty or starts with "t3_" (post prefix)
		if comment.ParentID == "" || strings.HasPrefix(comment.ParentID, "t3_") {
			topLevel = append(topLevel, comment)
		} else {
			// This is a child comment - parent should be another comment (t1_)
			if !strings.HasPrefix(comment.ParentID, "t1_") {
				s.logger.Warn("invalid parent_id format for child comment",
					"comment_id", comment.ID,
					"parent_id", comment.ParentID,
				)
				continue // Skip this comment with invalid parent format
			}

			// Extract parent ID without "t1_" prefix
			parentID := strings.TrimPrefix(comment.ParentID, "t1_")
			if parentID == "" {
				s.logger.Warn("malformed parent_id (empty after prefix removal)",
					"comment_id", comment.ID,
					"parent_id", comment.ParentID,
				)
				continue // Skip this malformed comment
			}

			// Find parent in map and add to Replies
			if parent, exists := commentMap[parentID]; exists {
				if parent.Replies == nil {
					parent.Replies = []*types.Comment{}
				}
				parent.Replies = append(parent.Replies, comment)
			} else {
				// Parent not found in map - could be missing or filtered out by depth
				// Treat as orphan (could add to top-level or skip)
				s.logger.Debug("orphaned comment found",
					"comment_id", comment.ID,
					"parent_id", comment.ParentID,
				)
			}
		}
	}

	s.logger.Debug("comment tree built",
		"post_id", postID,
		"total_comments", len(allComments),
		"top_level_comments", len(topLevel),
	)

	return topLevel, nil
}

// DeleteComment removes a comment by its ID (without prefix, e.g., "xyz789").
// Closure table entries are automatically deleted via CASCADE foreign keys.
// Implements idempotent delete (succeeds even if comment doesn't exist).
// Returns an error if the operation fails.
func (s *SQLiteStore) DeleteComment(ctx context.Context, id string) error {
	query := `DELETE FROM comments WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("DeleteComment: failed to delete comment %s: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteComment: failed to get rows affected for %s: %w", id, err)
	}

	s.logger.Debug("comment deleted",
		"comment_id", id,
		"rows_affected", rowsAffected,
	)

	return nil // Idempotent - success even if comment didn't exist
}

// calculateDepth determines the depth of a comment based on its parent.
// If parentID is empty or starts with "t3_" (post prefix), returns 0 (top-level).
// Otherwise, queries the parent comment's depth and returns parent_depth + 1.
// Returns an error if the parent comment doesn't exist.
func calculateDepth(ctx context.Context, tx *sql.Tx, parentID string) (int, error) {
	// Top-level comment: parent is a post (t3_) or empty
	if parentID == "" || strings.HasPrefix(parentID, "t3_") {
		return 0, nil
	}

	// Child comment: query parent's depth
	// Extract parent ID without "t1_" prefix if present
	actualParentID := parentID
	if strings.HasPrefix(parentID, "t1_") {
		actualParentID = parentID[3:]
	}

	var parentDepth int
	query := `SELECT depth FROM comments WHERE id = ?`
	err := tx.QueryRowContext(ctx, query, actualParentID).Scan(&parentDepth)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("parent comment not found: %s", actualParentID)
		}
		return 0, fmt.Errorf("failed to query parent depth for %s: %w", actualParentID, err)
	}

	return parentDepth + 1, nil
}

// extractPostID extracts the post ID from a Reddit link_id field.
// Example: "t3_abc123" → "abc123"
// Returns empty string if the link_id is invalid or empty.
func extractPostID(linkID string) string {
	if linkID == "" {
		return ""
	}

	// Check if it has the expected "t3_" prefix
	if strings.HasPrefix(linkID, "t3_") {
		return linkID[3:]
	}

	// If no prefix, assume it's already just the ID
	// (though this would be non-standard)
	return linkID
}

// insertClosureEntries inserts closure table entries for a comment.
// It inserts:
//  1. Self-reference: (commentID, commentID, 0)
//  2. Parent ancestry: All ancestor relationships inherited from parent
//
// The parentID should be the full Reddit fullname (e.g., "t1_xyz789" for comments, "t3_abc123" for posts).
// If parentID is empty or a post ID (t3_), only the self-reference is inserted.
// Must be called within a transaction.
func insertClosureEntries(ctx context.Context, tx *sql.Tx, commentID, parentID string) error {
	// Always insert self-reference
	selfRefQuery := `INSERT INTO comment_closures (ancestor, descendant, depth) VALUES (?, ?, 0)`
	if _, err := tx.ExecContext(ctx, selfRefQuery, commentID, commentID); err != nil {
		return fmt.Errorf("failed to insert self-reference: %w", err)
	}

	// If parent is empty or a post (t3_), we're done (top-level comment)
	if parentID == "" || strings.HasPrefix(parentID, "t3_") {
		return nil
	}

	// Extract actual parent ID (remove "t1_" prefix if present)
	actualParentID := parentID
	if strings.HasPrefix(parentID, "t1_") {
		actualParentID = parentID[3:]
	}

	// Copy parent's ancestry and add this comment as descendant
	copyAncestryQuery := `
		INSERT INTO comment_closures (ancestor, descendant, depth)
		SELECT ancestor, ?, depth + 1
		FROM comment_closures
		WHERE descendant = ?
	`

	result, err := tx.ExecContext(ctx, copyAncestryQuery, commentID, actualParentID)
	if err != nil {
		return fmt.Errorf("failed to copy parent ancestry: %w", err)
	}

	// Check if we copied any rows - if not, parent might not exist yet
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Parent comment has no closure entries - verify if parent exists at all
		var parentExists bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM comments WHERE id = ?)", actualParentID).Scan(&parentExists)
		if err != nil {
			return fmt.Errorf("failed to check parent existence: %w", err)
		}
		if !parentExists {
			return fmt.Errorf("parent comment %s does not exist", actualParentID)
		}
		// Parent exists but has no closure entries - this indicates corruption
		return fmt.Errorf("parent comment %s exists but has no closure entries (closure table corrupted)", actualParentID)
	}

	return nil
}
