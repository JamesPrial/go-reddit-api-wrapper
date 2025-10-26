//go:build integration

package sqlite_test

import (
	"context"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite/internal"
	"github.com/stretchr/testify/require"
)

// TestClosureTable_SelfReferences verifies that single comments have proper self-referential closure entries.
// Self-referential entries have ancestor=descendant and depth=0.
func TestClosureTable_SelfReferences(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Cast to SQLiteStore for direct database access
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert a post (required for foreign key constraint)
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Insert a single top-level comment
	comment := testutil.BuildComment("c1", "post1", "", 0)
	err = store.UpsertComment(ctx, comment)
	require.NoError(t, err, "failed to insert comment")

	// Verify self-referential entry exists
	var count int
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ? AND descendant = ? AND depth = ?",
		"c1", "c1", 0).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "should have exactly one self-referential entry")

	// Verify no other entries exist for this comment
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ? OR descendant = ?",
		"c1", "c1").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "should have only self-referential entry")
}

// TestClosureTable_ImmediateParentChild verifies closure table entries for immediate parent-child relationships.
// Top-level comments (parent is post) should only have self-reference.
// Child comments should have self-reference and parent entry (depth=1).
func TestClosureTable_ImmediateParentChild(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("top-level comment has only self-reference", func(t *testing.T) {
		// Insert top-level comment (parent is the post)
		topComment := testutil.BuildComment("c1", "post1", "", 0)
		err := store.UpsertComment(ctx, topComment)
		require.NoError(t, err)

		// Verify only self-reference exists
		var count int
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "top-level comment should have only self-reference")

		// Verify self-ref has depth=0
		var depth int
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT depth FROM comment_closures WHERE ancestor = ? AND descendant = ?",
			"c1", "c1").Scan(&depth)
		require.NoError(t, err)
		require.Equal(t, 0, depth, "self-reference should have depth=0")
	})

	t.Run("child comment has self-ref and parent entry", func(t *testing.T) {
		// Insert another top-level comment
		parent := testutil.BuildComment("c2", "post1", "", 0)
		err := store.UpsertComment(ctx, parent)
		require.NoError(t, err)

		// Insert child comment with parent=c2
		child := testutil.BuildComment("c3", "post1", "c2", 1)
		err = store.UpsertComment(ctx, child)
		require.NoError(t, err)

		// Verify child has 2 entries: self-ref and parent entry
		var count int
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c3").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "child comment should have 2 closure entries")

		// Verify entries have correct depths
		var entries []struct {
			ancestor string
			depth    int
		}

		rows, err := sqlite.QueryContext(sqliteStore, ctx,
			"SELECT ancestor, depth FROM comment_closures WHERE descendant = ? ORDER BY depth",
			"c3")
		require.NoError(t, err)
		defer rows.Close()

		for rows.Next() {
			var e struct {
				ancestor string
				depth    int
			}
			err := rows.Scan(&e.ancestor, &e.depth)
			require.NoError(t, err)
			entries = append(entries, e)
		}

		require.Len(t, entries, 2)
		require.Equal(t, "c3", entries[0].ancestor) // Self-ref
		require.Equal(t, 0, entries[0].depth)
		require.Equal(t, "c2", entries[1].ancestor) // Parent
		require.Equal(t, 1, entries[1].depth)
	})
}

// TestClosureTable_MultiLevelHierarchy verifies proper closure table entries for deep hierarchies.
// Creates a 4-level tree: C1→C2→C3→C4
// Verifies each comment has correct number of entries with proper depths.
func TestClosureTable_MultiLevelHierarchy(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create deep tree: C1→C2→C3→C4
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),   // Level 0
		testutil.BuildComment("c2", "post1", "c1", 1), // Level 1 (child of c1)
		testutil.BuildComment("c3", "post1", "c2", 2), // Level 2 (child of c2)
		testutil.BuildComment("c4", "post1", "c3", 3), // Level 3 (child of c3)
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	t.Run("C1 has 4 closure entries", func(t *testing.T) {
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c1").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 4, count, "C1 should have 4 entries (self + C2 + C3 + C4)")

		// Verify depths: 0 for self, 1 for C2, 2 for C3, 3 for C4
		expectedDepths := map[string]int{"c1": 0, "c2": 1, "c3": 2, "c4": 3}
		for descendant, expectedDepth := range expectedDepths {
			var depth int
			err := sqlite.QueryRowContext(sqliteStore, ctx,
				"SELECT depth FROM comment_closures WHERE ancestor = ? AND descendant = ?",
				"c1", descendant).Scan(&depth)
			require.NoError(t, err)
			require.Equal(t, expectedDepth, depth)
		}
	})

	t.Run("C2 has 3 closure entries", func(t *testing.T) {
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c2").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 3, count, "C2 should have 3 entries (self + C3 + C4)")

		// Verify depths
		expectedDepths := map[string]int{"c2": 0, "c3": 1, "c4": 2}
		for descendant, expectedDepth := range expectedDepths {
			var depth int
			err := sqlite.QueryRowContext(sqliteStore, ctx,
				"SELECT depth FROM comment_closures WHERE ancestor = ? AND descendant = ?",
				"c2", descendant).Scan(&depth)
			require.NoError(t, err)
			require.Equal(t, expectedDepth, depth)
		}
	})

	t.Run("C3 has 2 closure entries", func(t *testing.T) {
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c3").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "C3 should have 2 entries (self + C4)")

		// Verify depths
		expectedDepths := map[string]int{"c3": 0, "c4": 1}
		for descendant, expectedDepth := range expectedDepths {
			var depth int
			err := sqlite.QueryRowContext(sqliteStore, ctx,
				"SELECT depth FROM comment_closures WHERE ancestor = ? AND descendant = ?",
				"c3", descendant).Scan(&depth)
			require.NoError(t, err)
			require.Equal(t, expectedDepth, depth)
		}
	})

	t.Run("C4 has 1 closure entry (self only)", func(t *testing.T) {
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c4").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count, "C4 should have 1 entry (self only)")
	})

	t.Run("query all C1 descendants with depth>0", func(t *testing.T) {
		rows, err := sqlite.QueryContext(sqliteStore, ctx,
			"SELECT descendant FROM comment_closures WHERE ancestor = ? AND depth > 0 ORDER BY depth",
			"c1")
		require.NoError(t, err)
		defer rows.Close()

		var descendants []string
		for rows.Next() {
			var d string
			err := rows.Scan(&d)
			require.NoError(t, err)
			descendants = append(descendants, d)
		}

		require.Equal(t, []string{"c2", "c3", "c4"}, descendants)
	})
}

// TestClosureTable_WideTree verifies closure table for wide hierarchies.
// Creates a post with 5 top-level comments, each with 5 children.
// Verifies each top-level has 6 entries (self + 5 children).
func TestClosureTable_WideTree(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Use fixture to create wide tree: 5 top-level with 5 children each
	// BuildCommentTree(postID, depth, breadth) creates breadth comments at each level
	comments := testutil.BuildCommentTree("post1", 1, 5)
	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	t.Run("each top-level comment has 6 closure entries", func(t *testing.T) {
		// Get all top-level comments (those with parent=post_id)
		rows, err := sqlite.QueryContext(sqliteStore, ctx,
			"SELECT id FROM comments WHERE parent_id = ? ORDER BY id", "t3_post1")
		require.NoError(t, err)
		defer rows.Close()

		var topLevelIDs []string
		for rows.Next() {
			var id string
			err := rows.Scan(&id)
			require.NoError(t, err)
			topLevelIDs = append(topLevelIDs, id)
		}

		require.Equal(t, 5, len(topLevelIDs), "should have 5 top-level comments")

		// Verify each has 6 closure entries (self + 5 children)
		for _, topID := range topLevelIDs {
			var count int
			err := sqlite.QueryRowContext(sqliteStore, ctx,
				"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", topID).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 6, count, "top-level %s should have 6 closure entries", topID)
		}
	})

	t.Run("GetCommentTree returns proper structure", func(t *testing.T) {
		tree, err := store.GetCommentTree(ctx, "post1", nil)
		require.NoError(t, err)
		require.Len(t, tree, 5, "should have 5 top-level comments")

		// Each top-level should have 5 replies (children)
		for i, topComment := range tree {
			require.Len(t, topComment.Replies, 5,
				"top-level comment %d should have 5 children", i)
		}
	})
}

// TestClosureTable_GetCommentTreeWithMaxDepth verifies MaxDepth filter in GetCommentTree.
// Builds a deep tree and retrieves with different MaxDepth values.
func TestClosureTable_GetCommentTreeWithMaxDepth(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create tree with depth=5
	comments := testutil.BuildCommentTree("post1", 5, 2)
	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	t.Run("MaxDepth=2 returns only comments up to depth 2", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{MaxDepth: 2}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.NotEmpty(t, tree)

		// Verify tree is truncated at depth 2
		// Top level should exist
		require.Len(t, tree, 2, "should have top-level comments")

		// Check depth of tree
		maxDepthFound := verifyTreeDepth(tree, 0)
		require.LessOrEqual(t, maxDepthFound, 2, "should not exceed MaxDepth=2")
	})

	t.Run("MaxDepth=0 (unlimited) returns all comments", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{MaxDepth: 0}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.NotEmpty(t, tree)

		// Verify deep nesting exists
		maxDepthFound := verifyTreeDepth(tree, 0)
		require.Equal(t, 5, maxDepthFound, "should have full depth when MaxDepth=0")
	})

	t.Run("MaxDepth=1 returns only top-level and their children", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{MaxDepth: 1}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.Len(t, tree, 2, "should have top-level comments")

		// All children of top-level should have no replies or be truncated
		for _, comment := range tree {
			if len(comment.Replies) > 0 {
				// Children of top-level can exist, but they shouldn't have replies
				for _, child := range comment.Replies {
					require.Empty(t, child.Replies, "grandchildren should not be present")
				}
			}
		}

		maxDepthFound := verifyTreeDepth(tree, 0)
		require.LessOrEqual(t, maxDepthFound, 1, "should not exceed MaxDepth=1")
	})
}

// verifyTreeDepth recursively checks the maximum depth of a comment tree.
// Returns the maximum depth found in the tree.
func verifyTreeDepth(comments []*types.Comment, currentDepth int) int {
	if len(comments) == 0 {
		return currentDepth
	}

	maxDepth := currentDepth
	for _, comment := range comments {
		if len(comment.Replies) > 0 {
			childMaxDepth := verifyTreeDepth(comment.Replies, currentDepth+1)
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}
	return maxDepth
}

// TestClosureTable_CascadeDelete verifies delete behavior and closure entry cleanup.
// Deletes a comment and verifies its closure entries are removed.
// Note: Child comments are NOT automatically deleted - only the comment itself and its closure entries.
func TestClosureTable_CascadeDelete(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create tree: C1→C2→C3
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),
		testutil.BuildComment("c2", "post1", "c1", 1),
		testutil.BuildComment("c3", "post1", "c2", 2),
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	// Verify all comments exist
	for _, c := range comments {
		retrieved, err := store.GetComment(ctx, c.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
	}

	t.Run("delete leaf comment removes its closure entries", func(t *testing.T) {
		// Verify C3 has closure entries before delete
		// C3 is at depth 2, so it should have 3 entries: (C3,C3,0), (C2,C3,1), (C1,C3,2)
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c3").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 3, count, "C3 should have 3 closure entries before delete")

		// Delete C3
		err = store.DeleteComment(ctx, "c3")
		require.NoError(t, err)

		// Verify C3 is deleted
		_, err = store.GetComment(ctx, "c3")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)

		// Verify C3's closure entries are deleted
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c3").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "no closure entries should exist for deleted comment C3")

		// Verify C1 and C2 still exist (no cascade delete)
		for _, id := range []string{"c1", "c2"} {
			retrieved, err := store.GetComment(ctx, id)
			require.NoError(t, err)
			require.NotNil(t, retrieved)
		}
	})

	t.Run("delete parent comment removes only its own closure entries", func(t *testing.T) {
		// C1 should have 2 closure entries (self, C2) - C3 was already deleted in the previous test
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c1").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "C1 should have 2 closure entries (self + C2) after C3 was deleted")

		// Delete C1
		err = store.DeleteComment(ctx, "c1")
		require.NoError(t, err)

		// Verify C1 is deleted
		_, err = store.GetComment(ctx, "c1")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)

		// Verify C1's closure entries are deleted
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ? OR descendant = ?",
			"c1", "c1").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count, "no closure entries should exist for deleted comment C1")

		// Verify C2 still exists (orphaned)
		retrieved, err := store.GetComment(ctx, "c2")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
	})
}

// TestClosureTable_OrphanedComment verifies behavior with non-existent parent IDs.
// Attempting to insert a comment with a non-existent parent should fail
// due to foreign key constraint violation.
func TestClosureTable_OrphanedComment(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Try to insert comment with non-existent parent comment ID
	// Note: This should fail in current implementation due to foreign key constraint
	orphanComment := testutil.BuildComment("c1", "post1", "nonexistent", 1)

	err = store.UpsertComments(ctx, []*types.Comment{orphanComment})
	require.Error(t, err, "should reject comment with non-existent parent")

	// Verify comment was not inserted
	_, err = store.GetComment(ctx, "c1")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)

	// Verify no closure entries exist
	var count int
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "no closure entries should exist for non-inserted comment")
}

// TestClosureTable_BatchInsertMaintainsDepth verifies batch insert with proper dependency sorting.
// Inserts multiple comments in a single batch and verifies closure entries are correct
// even when comments are not in dependency order.
func TestClosureTable_BatchInsertMaintainsDepth(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("comments inserted in correct order", func(t *testing.T) {
		// Insert in proper dependency order: parents before children
		comments := []*types.Comment{
			testutil.BuildComment("c1", "post1", "", 0),
			testutil.BuildComment("c2", "post1", "c1", 1),
			testutil.BuildComment("c3", "post1", "c2", 2),
		}

		err := store.UpsertComments(ctx, comments)
		require.NoError(t, err)

		// Verify closure entries are correct
		expectedEntries := map[string]int{
			"c1": 3, // self + c2 + c3
			"c2": 2, // self + c3
			"c3": 1, // self only
		}

		for commentID, expectedCount := range expectedEntries {
			var count int
			err := sqlite.QueryRowContext(sqliteStore, ctx,
				"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", commentID).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, expectedCount, count,
				"comment %s should have %d closure entries", commentID, expectedCount)
		}
	})

	t.Run("comments in shuffled order still work", func(t *testing.T) {
		// Create new test DB for clean state
		store2 := NewTestDB(t)
		sqliteStore2, ok := store2.(*sqlite.SQLiteStore)
		require.True(t, ok)

		ctx := context.Background()

		// Insert post
		post2 := testutil.BuildPost("post2", "test")
		err := store2.UpsertPost(ctx, post2)
		require.NoError(t, err)

		// Insert comments in shuffled order: children before parents
		comments := []*types.Comment{
			testutil.BuildComment("d2", "post2", "d1", 1),
			testutil.BuildComment("d1", "post2", "", 0),
			testutil.BuildComment("d3", "post2", "d2", 2),
		}

		// UpsertComments should handle dependency sorting
		err = store2.UpsertComments(ctx, comments)
		require.NoError(t, err)

		// Verify closure entries are correct despite insert order
		expectedEntries := map[string]int{
			"d1": 3, // self + d2 + d3
			"d2": 2, // self + d3
			"d3": 1, // self only
		}

		for commentID, expectedCount := range expectedEntries {
			var count int
			err := sqlite.QueryRowContext(sqliteStore2, ctx,
				"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", commentID).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, expectedCount, count,
				"comment %s should have %d closure entries", commentID, expectedCount)
		}
	})
}

// TestClosureTable_CommentSorting verifies sorting functionality in GetCommentTree.
// Creates comments with different scores and timestamps, then retrieves with various sort options.
func TestClosureTable_CommentSorting(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create comments with varying scores
	now := float64(1000)
	comments := []*types.Comment{
		func() *types.Comment {
			c := testutil.BuildComment("c1", "post1", "", 0)
			c.Score = 100
			c.Created.CreatedUTC = now
			return c
		}(),
		func() *types.Comment {
			c := testutil.BuildComment("c2", "post1", "", 0)
			c.Score = 50
			c.Created.CreatedUTC = now + 100
			return c
		}(),
		func() *types.Comment {
			c := testutil.BuildComment("c3", "post1", "", 0)
			c.Score = 75
			c.Created.CreatedUTC = now + 50
			return c
		}(),
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	t.Run("sort by score descending", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{SortBy: "score", SortDir: "desc"}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.Len(t, tree, 3)

		// Verify sorted order: 100, 75, 50
		require.Equal(t, "c1", tree[0].ID, "highest score should be first")
		require.Equal(t, "c3", tree[1].ID, "medium score should be second")
		require.Equal(t, "c2", tree[2].ID, "lowest score should be last")
	})

	t.Run("sort by score ascending", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{SortBy: "score", SortDir: "asc"}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.Len(t, tree, 3)

		// Verify reverse order: 50, 75, 100
		require.Equal(t, "c2", tree[0].ID, "lowest score should be first")
		require.Equal(t, "c3", tree[1].ID, "medium score should be second")
		require.Equal(t, "c1", tree[2].ID, "highest score should be last")
	})

	t.Run("sort by created_utc descending", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{SortBy: "created_utc", SortDir: "desc"}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.Len(t, tree, 3)

		// Verify order by creation time (newest first)
		require.Equal(t, "c2", tree[0].ID, "newest should be first")
		require.Equal(t, "c3", tree[1].ID, "middle should be second")
		require.Equal(t, "c1", tree[2].ID, "oldest should be last")
	})

	t.Run("sort by created_utc ascending", func(t *testing.T) {
		opts := &storage.CommentTreeOptions{SortBy: "created_utc", SortDir: "asc"}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.Len(t, tree, 3)

		// Verify order by creation time (oldest first)
		require.Equal(t, "c1", tree[0].ID, "oldest should be first")
		require.Equal(t, "c3", tree[1].ID, "middle should be second")
		require.Equal(t, "c2", tree[2].ID, "newest should be last")
	})
}

// TestClosureTable_MixedTopLevelAndNested verifies proper handling of mixed hierarchies.
// Creates a post with both top-level and nested comments.
// Verifies closure entries are correct and tree reconstruction works.
func TestClosureTable_MixedTopLevelAndNested(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Insert post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create mixed hierarchy:
	// Top-level comments: c1, c4
	// Nested under c1: c2, c3
	// Nested under c2: c3_child
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),         // Top-level
		testutil.BuildComment("c2", "post1", "c1", 1),       // Child of c1
		testutil.BuildComment("c3", "post1", "c2", 2),       // Grandchild of c1
		testutil.BuildComment("c3_child", "post1", "c3", 3), // Great-grandchild of c1
		testutil.BuildComment("c4", "post1", "", 0),         // Top-level (independent)
		testutil.BuildComment("c5", "post1", "c4", 1),       // Child of c4
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	t.Run("top-level comments have correct closure counts", func(t *testing.T) {
		// c1 is top-level with descendants: c2, c3, c3_child (4 total: self + 3 descendants)
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c1").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 4, count, "c1 should have 4 closure entries (self + c2 + c3 + c3_child)")

		// c4 is top-level with descendants: c5 (2 total: self + 1 descendant)
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c4").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 2, count, "c4 should have 2 closure entries (self + c5)")
	})

	t.Run("nested comments have proper ancestor chains", func(t *testing.T) {
		// c3 should have ancestors: c3, c2, c1
		var count int
		err := sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c3").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 3, count, "c3 should have 3 ancestors (self, c2, c1)")

		// c3_child should have ancestors: c3_child, c3, c2, c1
		err = sqlite.QueryRowContext(sqliteStore, ctx,
			"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c3_child").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 4, count, "c3_child should have 4 ancestors (self, c3, c2, c1)")
	})

	t.Run("GetCommentTree returns proper tree structure", func(t *testing.T) {
		tree, err := store.GetCommentTree(ctx, "post1", nil)
		require.NoError(t, err)
		require.Len(t, tree, 2, "should have 2 top-level comments")

		// Verify c1 structure
		c1 := findCommentByID(tree, "c1")
		require.NotNil(t, c1)
		require.Len(t, c1.Replies, 1, "c1 should have 1 child (c2)")

		c2 := findCommentByID(c1.Replies, "c2")
		require.NotNil(t, c2)
		require.Len(t, c2.Replies, 1, "c2 should have 1 child (c3)")

		c3 := findCommentByID(c2.Replies, "c3")
		require.NotNil(t, c3)
		require.Len(t, c3.Replies, 1, "c3 should have 1 child (c3_child)")

		// Verify c4 structure
		c4 := findCommentByID(tree, "c4")
		require.NotNil(t, c4)
		require.Len(t, c4.Replies, 1, "c4 should have 1 child (c5)")
	})
}

// findCommentByID searches for a comment by ID in a tree.
func findCommentByID(comments []*types.Comment, id string) *types.Comment {
	for _, c := range comments {
		if c.ID == id {
			return c
		}
	}
	return nil
}
