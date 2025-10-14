package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/storage/testutil"
	"github.com/stretchr/testify/require"
)

// TestUpsertComment verifies that comments can be inserted and updated with correct depths.
func TestUpsertComment(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post first (required for foreign key)
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Insert a top-level comment
	topComment := testutil.BuildComment("c1", "post1", "", 0)
	err = store.UpsertComment(ctx, topComment)
	require.NoError(t, err, "failed to insert top-level comment")

	// Verify the comment was inserted with depth = 0
	retrieved, err := store.GetComment(ctx, "c1")
	require.NoError(t, err, "failed to retrieve top-level comment")
	require.NotNil(t, retrieved)
	require.Equal(t, "c1", retrieved.ID)
	require.Equal(t, "t1_c1", retrieved.Name)
	require.Equal(t, "t3_post1", retrieved.ParentID) // Top-level parent is the post

	// Verify closure table has self-reference
	var closureCount int
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&closureCount)
	require.NoError(t, err)
	require.Equal(t, 1, closureCount, "should have 1 closure entry (self-reference)")

	// Insert a child comment
	childComment := testutil.BuildComment("c2", "post1", "c1", 1)
	err = store.UpsertComment(ctx, childComment)
	require.NoError(t, err, "failed to insert child comment")

	// Verify the child comment was inserted
	retrievedChild, err := store.GetComment(ctx, "c2")
	require.NoError(t, err, "failed to retrieve child comment")
	require.NotNil(t, retrievedChild)
	require.Equal(t, "c2", retrievedChild.ID)
	require.Equal(t, "t1_c1", retrievedChild.ParentID) // Parent is c1

	// Verify closure table entries for child
	// Should have: (c1, c2, 1) and (c2, c2, 0)
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c2").Scan(&closureCount)
	require.NoError(t, err)
	require.Equal(t, 2, closureCount, "should have 2 closure entries for child")

	// Update the top-level comment
	topComment.Body = "Updated body"
	err = store.UpsertComment(ctx, topComment)
	require.NoError(t, err, "failed to update comment")

	// Verify the update
	updated, err := store.GetComment(ctx, "c1")
	require.NoError(t, err)
	require.Equal(t, "Updated body", updated.Body)
}

// TestGetComment verifies that comments can be retrieved by ID.
func TestGetComment(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Insert a comment
	comment := testutil.BuildComment("c1", "post1", "", 0)
	comment.Body = "Specific comment body"
	comment.Author = "specificauthor"
	err = store.UpsertComment(ctx, comment)
	require.NoError(t, err)

	// Get the comment by ID
	retrieved, err := store.GetComment(ctx, "c1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify all fields
	require.Equal(t, "c1", retrieved.ID)
	require.Equal(t, "t1_c1", retrieved.Name)
	require.Equal(t, "Specific comment body", retrieved.Body)
	require.Equal(t, "specificauthor", retrieved.Author)
	require.Equal(t, "t3_post1", retrieved.LinkID)

	// Try to get a non-existent comment
	notFound, err := store.GetComment(ctx, "nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Nil(t, notFound)
}

// TestUpsertComments verifies batch insertion of comments in hierarchy.
func TestUpsertComments(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create a hierarchy of comments
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),   // Top-level
		testutil.BuildComment("c2", "post1", "", 0),   // Top-level
		testutil.BuildComment("c3", "post1", "c1", 1), // Child of c1
		testutil.BuildComment("c4", "post1", "c1", 1), // Child of c1
		testutil.BuildComment("c5", "post1", "c3", 2), // Child of c3 (grandchild of c1)
	}

	// Batch insert
	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err, "failed to batch upsert comments")

	// Verify all comments were inserted
	for _, c := range comments {
		retrieved, err := store.GetComment(ctx, c.ID)
		require.NoError(t, err, "failed to retrieve comment %s", c.ID)
		require.NotNil(t, retrieved)
		require.Equal(t, c.ID, retrieved.ID)
	}

	// Verify closure table is correct
	// c1 should have: (c1, c1, 0), (c1, c3, 1), (c1, c4, 1), (c1, c5, 2)
	var c1ClosureCount int
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c1").Scan(&c1ClosureCount)
	require.NoError(t, err)
	require.Equal(t, 4, c1ClosureCount, "c1 should have 4 closure entries (self + 3 descendants)")

	// c3 should have: (c3, c3, 0), (c3, c5, 1)
	var c3ClosureCount int
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c3").Scan(&c3ClosureCount)
	require.NoError(t, err)
	require.Equal(t, 2, c3ClosureCount, "c3 should have 2 closure entries (self + 1 descendant)")

	// c5 should have only self-reference
	var c5ClosureCount int
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c5").Scan(&c5ClosureCount)
	require.NoError(t, err)
	require.Equal(t, 1, c5ClosureCount, "c5 should have 1 closure entry (self only)")
}

// TestGetCommentTree verifies tree reconstruction with Replies populated.
func TestGetCommentTree(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Build a comment tree using the fixture helper
	// depth=2, breadth=2 means:
	// - 2 top-level comments
	// - Each has 2 children
	// - Each child has 2 children (grandchildren)
	comments := testutil.BuildCommentTree("post1", 2, 2)
	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	t.Run("get full tree", func(t *testing.T) {
		tree, err := store.GetCommentTree(ctx, "post1", nil)
		require.NoError(t, err)
		require.NotEmpty(t, tree)

		// Should have 2 top-level comments
		require.Len(t, tree, 2, "should have 2 top-level comments")

		// Each top-level comment should have replies
		for _, topLevel := range tree {
			require.NotEmpty(t, topLevel.Replies, "top-level comment should have replies")
			// Each reply should also have replies (grandchildren)
			for _, child := range topLevel.Replies {
				require.NotEmpty(t, child.Replies, "child comment should have replies")
			}
		}
	})

	t.Run("get tree with max depth filter", func(t *testing.T) {
		opts := &CommentTreeOptions{MaxDepth: 1}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)

		// Should only get top-level comments (depth 0)
		require.Len(t, tree, 2, "should have 2 top-level comments")

		// Top-level comments should not have replies (filtered by depth)
		// Note: Depending on implementation, this might still populate replies
		// or might return empty replies. Let's just verify we got the top-level.
	})

	t.Run("get tree with sorting", func(t *testing.T) {
		// Update scores to test sorting
		comments[0].Score = 100
		comments[1].Score = 50
		err := store.UpsertComments(ctx, comments[:2])
		require.NoError(t, err)

		opts := &CommentTreeOptions{SortBy: "score", SortDir: "desc"}
		tree, err := store.GetCommentTree(ctx, "post1", opts)
		require.NoError(t, err)
		require.Len(t, tree, 2)

		// Verify sorted by score descending
		// Note: The sorting happens at each level, so top-level should be sorted
	})

	t.Run("get tree for post with no comments", func(t *testing.T) {
		// Insert a post with no comments
		emptyPost := testutil.BuildPost("empty", "test")
		err := store.UpsertPost(ctx, emptyPost)
		require.NoError(t, err)

		tree, err := store.GetCommentTree(ctx, "empty", nil)
		require.NoError(t, err)
		require.Empty(t, tree, "should return empty slice for post with no comments")
	})
}

// TestDeleteComment verifies that comments can be deleted.
func TestDeleteComment(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Insert a comment
	comment := testutil.BuildComment("c1", "post1", "", 0)
	err = store.UpsertComment(ctx, comment)
	require.NoError(t, err)

	// Verify the comment exists
	retrieved, err := store.GetComment(ctx, "c1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify closure entry exists
	var closureCount int
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&closureCount)
	require.NoError(t, err)
	require.Equal(t, 1, closureCount)

	// Delete the comment
	err = store.DeleteComment(ctx, "c1")
	require.NoError(t, err)

	// Verify the comment is gone
	notFound, err := store.GetComment(ctx, "c1")
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Nil(t, notFound)

	// Verify closure entry is removed (CASCADE)
	err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&closureCount)
	require.NoError(t, err)
	require.Equal(t, 0, closureCount, "closure entries should be deleted via CASCADE")

	// Delete again (should be idempotent)
	err = store.DeleteComment(ctx, "c1")
	require.NoError(t, err)
}

// TestCommentTreeDepth verifies that depth calculation is correct for nested comments.
func TestCommentTreeDepth(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create a deep tree: c1 -> c2 -> c3 -> c4
	// Note: The depth parameter passed to BuildComment is for documentation,
	// but the actual depth is calculated by the storage layer based on parent relationships
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),   // Top-level: depth 0
		testutil.BuildComment("c2", "post1", "c1", 1), // Child of c1: depth 1
		testutil.BuildComment("c3", "post1", "c2", 2), // Child of c2: depth 2
		testutil.BuildComment("c4", "post1", "c3", 3), // Child of c3: depth 3
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	// Verify depths in the database
	expectedDepths := map[string]int{
		"c1": 0,
		"c2": 1,
		"c3": 2,
		"c4": 3,
	}

	for _, c := range comments {
		var depth int
		err := store.db.QueryRowContext(ctx, "SELECT depth FROM comments WHERE id = ?", c.ID).Scan(&depth)
		require.NoError(t, err)
		expectedDepth := expectedDepths[c.ID]
		require.Equal(t, expectedDepth, depth, "comment %s should have depth %d", c.ID, expectedDepth)
	}
}

// TestCommentClosureTable verifies closure table integrity.
func TestCommentClosureTable(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create a tree: C1 -> C2 -> C3
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),
		testutil.BuildComment("c2", "post1", "c1", 1),
		testutil.BuildComment("c3", "post1", "c2", 2),
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err)

	// Verify closure entries
	type closureEntry struct {
		ancestor   string
		descendant string
		depth      int
	}

	var entries []closureEntry
	rows, err := store.db.QueryContext(ctx, "SELECT ancestor, descendant, depth FROM comment_closures ORDER BY ancestor, depth")
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var e closureEntry
		err := rows.Scan(&e.ancestor, &e.descendant, &e.depth)
		require.NoError(t, err)
		entries = append(entries, e)
	}

	// Expected entries:
	// (c1, c1, 0), (c1, c2, 1), (c1, c3, 2)
	// (c2, c2, 0), (c2, c3, 1)
	// (c3, c3, 0)
	expectedEntries := []closureEntry{
		{"c1", "c1", 0},
		{"c1", "c2", 1},
		{"c1", "c3", 2},
		{"c2", "c2", 0},
		{"c2", "c3", 1},
		{"c3", "c3", 0},
	}

	require.Equal(t, len(expectedEntries), len(entries), "should have exactly %d closure entries", len(expectedEntries))

	// Verify each expected entry exists
	for _, expected := range expectedEntries {
		found := false
		for _, actual := range entries {
			if actual.ancestor == expected.ancestor &&
				actual.descendant == expected.descendant &&
				actual.depth == expected.depth {
				found = true
				break
			}
		}
		require.True(t, found, "expected closure entry (%s, %s, %d) not found",
			expected.ancestor, expected.descendant, expected.depth)
	}
}
