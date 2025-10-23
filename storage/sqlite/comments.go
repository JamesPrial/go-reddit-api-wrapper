package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
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
		return &storage.ValidationError{Operation: "UpsertComment", Field: "comment", Reason: "comment cannot be nil"}
	}

	// Begin transaction for atomicity (comment + closure entries)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return &storage.TransactionError{Operation: "begin", Message: "UpsertComment", Err: err}
	}
	defer tx.Rollback() // Rollback if we don't reach Commit

	// Calculate depth from parent
	depth, err := calculateDepth(ctx, tx, comment.ParentID)
	if err != nil {
		return &storage.DatabaseError{Operation: "UpsertComment", Message: fmt.Sprintf("failed to calculate depth for %s", comment.ID), Err: err}
	}

	// Extract post_id from link_id
	postID := extractPostID(comment.LinkID)
	if postID == "" {
		return &storage.ValidationError{Operation: "UpsertComment", Field: "comment.LinkID", Value: comment.LinkID, Reason: fmt.Sprintf("invalid link_id for comment %s", comment.ID)}
	}

	s.logger.Debug("upserting comment",
		"comment_id", comment.ID,
		"parent_id", comment.ParentID,
		"depth", depth,
		"post_id", postID,
	)

	// Convert comment to insert args
	args := commentToInsertArgs(comment, depth)
	// Add post_id to args
	args = append(args, postID)

	// Execute upsert
	if _, err := tx.ExecContext(ctx, queryUpsertComment, args...); err != nil {
		return &storage.DatabaseError{Operation: "UpsertComment", Message: fmt.Sprintf("failed to insert comment %s", comment.ID), Err: err}
	}

	// Delete existing closure entries for this comment (in case of reparenting)
	// This ensures we rebuild the closure table correctly
	if _, err := tx.ExecContext(ctx, queryDeleteCommentClosures, comment.ID); err != nil {
		return &storage.DatabaseError{Operation: "UpsertComment", Message: fmt.Sprintf("failed to delete old closure entries for %s", comment.ID), Err: err}
	}

	// Insert closure entries
	if err := insertClosureEntries(ctx, tx, comment.ID, comment.ParentID); err != nil {
		return &storage.DatabaseError{Operation: "UpsertComment", Message: fmt.Sprintf("failed to insert closure entries for %s", comment.ID), Err: err}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &storage.TransactionError{Operation: "commit", Message: "UpsertComment", Err: err}
	}

	s.logger.Debug("comment upserted successfully", "comment_id", comment.ID)
	return nil
}

// GetComment retrieves a comment by its ID (without prefix, e.g., "xyz789").
// Returns the comment if found, or NotFoundError if not found.
// Does NOT populate the Replies field - use GetCommentTree for hierarchical data.
func (s *SQLiteStore) GetComment(ctx context.Context, id string) (*types.Comment, error) {
	// Validate input
	if id == "" {
		return nil, &storage.ValidationError{Operation: "GetComment", Field: "id", Reason: "comment ID cannot be empty"}
	}

	dest := newCommentScanDest()
	err := s.db.QueryRowContext(ctx, queryGetComment, id).Scan(dest.dest()...)
	if err != nil {
		// Return NotFoundError if comment doesn't exist
		if err == sql.ErrNoRows {
			return nil, &storage.NotFoundError{ResourceType: "comment", ResourceID: id}
		}
		return nil, &storage.DatabaseError{Operation: "GetComment", Message: fmt.Sprintf("failed to query comment %s", id), Err: err}
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
		return &storage.TransactionError{Operation: "begin", Message: "UpsertComments", Err: err}
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
			return &storage.ValidationError{Operation: "UpsertComments", Field: fmt.Sprintf("comments[%d]", i), Reason: "comment cannot be nil"}
		}
		if _, exists := nodeMap[c.ID]; exists {
			return &storage.ValidationError{Operation: "UpsertComments", Field: "comment ID", Value: c.ID, Reason: "duplicate comment ID in batch"}
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
			return &storage.IntegrityError{Operation: "UpsertComments", ResourceType: "comment", ResourceID: actualID, Reason: "comment references itself as parent"}
		}

		// Top-level comment must have post parent (t3_*)
		// Empty ParentID is invalid data (including whitespace-only)
		if strings.TrimSpace(parentID) == "" {
			return &storage.IntegrityError{Operation: "UpsertComments", ResourceType: "comment", ResourceID: node.comment.ID, Reason: "comment has empty parent_id"}
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
				return &storage.IntegrityError{Operation: "UpsertComments", ResourceType: "comment", ResourceID: node.comment.ID, Reason: fmt.Sprintf("malformed parent_id %s (empty after prefix)", parentID)}
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
				return &storage.IntegrityError{Operation: "UpsertComments", ResourceType: "comment", ResourceID: parentID, Reason: "parent not in batch and not in database"}
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
			return &storage.IntegrityError{Operation: "UpsertComments", ResourceType: "comment", ResourceID: id, Reason: "unreachable - possible cycle, orphaned comment, or self-reference"}
		}
	}

	// Prepare statement for batch insert
	stmt, err := tx.PrepareContext(ctx, queryUpsertComment)
	if err != nil {
		return &storage.DatabaseError{Operation: "UpsertComments", Message: "failed to prepare statement", Err: err}
	}
	defer stmt.Close()

	// Insert comments in dependency order (orderedNodes is already in BFS order)
	for _, node := range orderedNodes {
		comment := node.comment
		depth := node.depth

		// Extract post_id
		postID := extractPostID(comment.LinkID)
		if postID == "" {
			return &storage.ValidationError{Operation: "UpsertComments", Field: "comment.LinkID", Value: comment.LinkID, Reason: fmt.Sprintf("invalid link_id for comment %s", comment.ID)}
		}

		// Build arguments for insert
		args := commentToInsertArgs(comment, depth)
		args = append(args, postID)

		// Execute insert
		_, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return &storage.DatabaseError{Operation: "UpsertComments", Message: fmt.Sprintf("failed to insert comment %s", comment.ID), Err: err}
		}

		// Clear old closure entries for this comment
		_, err = tx.ExecContext(ctx, queryDeleteCommentClosures, comment.ID)
		if err != nil {
			return &storage.DatabaseError{Operation: "UpsertComments", Message: fmt.Sprintf("failed to delete old closure entries for %s", comment.ID), Err: err}
		}

		// Insert closure entries
		if err := insertClosureEntries(ctx, tx, comment.ID, comment.ParentID); err != nil {
			return &storage.DatabaseError{Operation: "UpsertComments", Message: fmt.Sprintf("failed to insert closure entries for %s", comment.ID), Err: err}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &storage.TransactionError{Operation: "commit", Message: "UpsertComments", Err: err}
	}

	s.logger.Debug("comments batch upserted successfully", "count", len(comments))
	return nil
}

// GetCommentTree retrieves all comments for a specific post in tree structure.
// The postID should be without prefix (e.g., "abc123").
// Returns comments with Replies populated according to the comment hierarchy.
// Returns an empty slice if no comments exist for the post.
// Supports filtering by MaxDepth and sorting via opts.
func (s *SQLiteStore) GetCommentTree(ctx context.Context, postID string, opts *storage.CommentTreeOptions) ([]*types.Comment, error) {
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
	query := queryGetCommentTree

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
		return nil, &storage.DatabaseError{Operation: "GetCommentTree", Message: fmt.Sprintf("failed to query comments for post %s", postID), Err: err}
	}
	defer rows.Close()

	// Scan all comments into a map
	commentMap := make(map[string]*types.Comment)
	var allComments []*types.Comment

	for rows.Next() {
		dest := newCommentScanDest()
		if err := rows.Scan(dest.dest()...); err != nil {
			return nil, &storage.DatabaseError{Operation: "GetCommentTree", Message: "failed to scan comment", Err: err}
		}

		comment := dest.toComment()
		commentMap[comment.ID] = comment
		allComments = append(allComments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, &storage.DatabaseError{Operation: "GetCommentTree", Message: "error iterating comments", Err: err}
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
	result, err := s.db.ExecContext(ctx, queryDeleteComment, id)
	if err != nil {
		return &storage.DatabaseError{Operation: "DeleteComment", Message: fmt.Sprintf("failed to delete comment %s", id), Err: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &storage.DatabaseError{Operation: "DeleteComment", Message: fmt.Sprintf("failed to get rows affected for %s", id), Err: err}
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
	err := tx.QueryRowContext(ctx, queryGetCommentDepth, actualParentID).Scan(&parentDepth)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, &storage.NotFoundError{ResourceType: "comment", ResourceID: actualParentID}
		}
		return 0, &storage.DatabaseError{Operation: "calculateDepth", Message: fmt.Sprintf("failed to query parent depth for %s", actualParentID), Err: err}
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
	if _, err := tx.ExecContext(ctx, queryInsertCommentClosure, commentID, commentID); err != nil {
		return &storage.DatabaseError{Operation: "insertClosureEntries", Message: "failed to insert self-reference", Err: err}
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
	result, err := tx.ExecContext(ctx, queryInsertCommentClosureCopyAncestry, commentID, actualParentID)
	if err != nil {
		return &storage.DatabaseError{Operation: "insertClosureEntries", Message: "failed to copy parent ancestry", Err: err}
	}

	// Check if we copied any rows - if not, parent might not exist yet
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &storage.DatabaseError{Operation: "insertClosureEntries", Message: "failed to get rows affected", Err: err}
	}

	if rowsAffected == 0 {
		// Parent comment has no closure entries - verify if parent exists at all
		var parentExists bool
		err := tx.QueryRowContext(ctx, queryCheckCommentExists, actualParentID).Scan(&parentExists)
		if err != nil {
			return &storage.DatabaseError{Operation: "insertClosureEntries", Message: "failed to check parent existence", Err: err}
		}
		if !parentExists {
			return &storage.IntegrityError{Operation: "insertClosureEntries", ResourceType: "comment", ResourceID: actualParentID, Reason: "parent comment does not exist"}
		}
		// Parent exists but has no closure entries - this indicates corruption
		return &storage.IntegrityError{Operation: "insertClosureEntries", ResourceType: "comment", ResourceID: actualParentID, Reason: "parent comment exists but has no closure entries (closure table corrupted)"}
	}

	return nil
}
