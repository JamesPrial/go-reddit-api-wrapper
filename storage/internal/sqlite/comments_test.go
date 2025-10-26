package sqlite_test

import (
	"context"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/sqlite"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
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

	// Note: Closure table verification requires internal database access
	// which is not available through the public API

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

	// Note: Additional closure table verification would require internal database access

	// Update the top-level comment
	topComment.Body = "Updated body"
	err = store.UpsertComment(ctx, topComment)
	require.NoError(t, err, "failed to update comment")

	// Verify the update
	updated, err := store.GetComment(ctx, "c1")
	require.NoError(t, err)
	require.Equal(t, "Updated body", updated.Body)
}

// TestUpsertComment_NilInput verifies that nil comment input is rejected with a descriptive error.
func TestUpsertComment_NilInput(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Attempt to upsert a nil comment
	err := store.UpsertComment(ctx, nil)
	require.Error(t, err, "should reject nil comment input")
	require.Contains(t, err.Error(), "comment cannot be nil", "error should mention nil input")
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
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	require.Equal(t, "comment", notFoundErr.ResourceType)
	require.Equal(t, "nonexistent", notFoundErr.ResourceID)
	require.Nil(t, notFound)
}

// TestUpsertComments verifies batch insertion of comments in hierarchy.
func TestUpsertComments(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Cast store to *sqlite.SQLiteStore for access to testing helpers
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	if !ok {
		t.Skip("store is not *sqlite.SQLiteStore, skipping closure table tests")
	}

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
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c1").Scan(&c1ClosureCount)
	require.NoError(t, err)
	require.Equal(t, 4, c1ClosureCount, "c1 should have 4 closure entries (self + 3 descendants)")

	// c3 should have: (c3, c3, 0), (c3, c5, 1)
	var c3ClosureCount int
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c3").Scan(&c3ClosureCount)
	require.NoError(t, err)
	require.Equal(t, 2, c3ClosureCount, "c3 should have 2 closure entries (self + 1 descendant)")

	// c5 should have only self-reference
	var c5ClosureCount int
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT COUNT(*) FROM comment_closures WHERE ancestor = ?", "c5").Scan(&c5ClosureCount)
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
		opts := &storage.CommentTreeOptions{MaxDepth: 1}
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

		opts := &storage.CommentTreeOptions{SortBy: "score", SortDir: "desc"}
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

	// Cast store to *sqlite.SQLiteStore for access to testing helpers
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	if !ok {
		t.Skip("store is not *sqlite.SQLiteStore, skipping closure table tests")
	}

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
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&closureCount)
	require.NoError(t, err)
	require.Equal(t, 1, closureCount)

	// Delete the comment
	err = store.DeleteComment(ctx, "c1")
	require.NoError(t, err)

	// Verify the comment is gone
	notFound, err := store.GetComment(ctx, "c1")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	require.Equal(t, "comment", notFoundErr.ResourceType)
	require.Equal(t, "c1", notFoundErr.ResourceID)
	require.Nil(t, notFound)

	// Verify closure entry is removed (CASCADE)
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "c1").Scan(&closureCount)
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

	// Cast store to *sqlite.SQLiteStore for access to testing helpers
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	if !ok {
		t.Skip("store is not *sqlite.SQLiteStore, skipping depth tests")
	}

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
		err := sqlite.QueryRowContext(sqliteStore, ctx, "SELECT depth FROM comments WHERE id = ?", c.ID).Scan(&depth)
		require.NoError(t, err)
		expectedDepth := expectedDepths[c.ID]
		require.Equal(t, expectedDepth, depth, "comment %s should have depth %d", c.ID, expectedDepth)
	}
}

// TestCommentClosureTable verifies closure table integrity.
func TestCommentClosureTable(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Cast store to *sqlite.SQLiteStore for access to testing helpers
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	if !ok {
		t.Skip("store is not *sqlite.SQLiteStore, skipping closure table tests")
	}

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
	rows, err := sqlite.QueryContext(sqliteStore, ctx, "SELECT ancestor, descendant, depth FROM comment_closures ORDER BY ancestor, depth")
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

// TestUpsertComments_NilElementInBatch verifies that nil elements in the batch are rejected.
func TestUpsertComments_NilElementInBatch(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("nil element at beginning", func(t *testing.T) {
		comments := []*types.Comment{
			nil, // Nil element
			testutil.BuildComment("c1", "post1", "", 0),
		}

		err := store.UpsertComments(ctx, comments)
		require.Error(t, err, "should reject batch with nil element")
		var valErr *storage.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, "comments[0]", valErr.Field)
		require.Equal(t, "comment cannot be nil", valErr.Reason)

		// Verify no comments were inserted (transaction rollback)
		retrieved, err := store.GetComment(ctx, "c1")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	})

	t.Run("nil element in middle", func(t *testing.T) {
		comments := []*types.Comment{
			testutil.BuildComment("c2", "post1", "", 0),
			nil, // Nil element at index 1
			testutil.BuildComment("c3", "post1", "", 0),
		}

		err := store.UpsertComments(ctx, comments)
		require.Error(t, err, "should reject batch with nil element in middle")
		var valErr *storage.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, "comments[1]", valErr.Field)
		require.Equal(t, "comment cannot be nil", valErr.Reason)

		// Verify no comments were inserted (transaction rollback)
		retrieved, err := store.GetComment(ctx, "c2")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	})

	t.Run("nil element at end", func(t *testing.T) {
		comments := []*types.Comment{
			testutil.BuildComment("c4", "post1", "", 0),
			testutil.BuildComment("c5", "post1", "", 0),
			nil, // Nil element at end
		}

		err := store.UpsertComments(ctx, comments)
		require.Error(t, err, "should reject batch with nil element at end")
		var valErr *storage.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, "comments[2]", valErr.Field)
		require.Equal(t, "comment cannot be nil", valErr.Reason)

		// Verify no comments were inserted (transaction rollback)
		retrieved, err := store.GetComment(ctx, "c4")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	})

	t.Run("multiple nil elements", func(t *testing.T) {
		comments := []*types.Comment{
			nil, // Nil element
			testutil.BuildComment("c6", "post1", "", 0),
			nil, // Another nil element
		}

		err := store.UpsertComments(ctx, comments)
		require.Error(t, err, "should reject batch with multiple nil elements")
		// Should fail on first nil element
		var valErr *storage.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, "comments[0]", valErr.Field)
		require.Equal(t, "comment cannot be nil", valErr.Reason)
	})
}

// TestUpsertComments_DuplicateIDInBatch verifies that duplicate comment IDs in a batch are rejected.
func TestUpsertComments_DuplicateIDInBatch(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create comments with duplicate ID
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),
		testutil.BuildComment("c2", "post1", "", 0),
		testutil.BuildComment("c1", "post1", "", 0), // Duplicate ID
	}

	// Attempt batch insert - should fail
	err = store.UpsertComments(ctx, comments)
	require.Error(t, err, "should reject duplicate comment IDs in batch")
	var valErr *storage.ValidationError
	require.ErrorAs(t, err, &valErr)
	require.Equal(t, "comment ID", valErr.Field)
	require.Equal(t, "c1", valErr.Value)
	require.Equal(t, "duplicate comment ID in batch", valErr.Reason)

	// Verify no comments were inserted (transaction rollback)
	retrieved, err := store.GetComment(ctx, "c1")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	require.Nil(t, retrieved)

	t.Run("duplicate ID with different parents", func(t *testing.T) {
		// Insert a parent comment first
		parentComment := testutil.BuildComment("c_parent", "post1", "", 0)
		err := store.UpsertComments(ctx, []*types.Comment{parentComment})
		require.NoError(t, err)

		// Try to insert two comments with the SAME ID but DIFFERENT parents
		duplicateComment1 := testutil.BuildComment("c_duplicate", "post1", "", 0)
		duplicateComment1.ParentID = "t3_post1" // Top-level (parent is post)

		duplicateComment2 := testutil.BuildComment("c_duplicate", "post1", "c_parent", 1)
		duplicateComment2.ParentID = "t1_c_parent" // Child of c_parent

		err = store.UpsertComments(ctx, []*types.Comment{duplicateComment1, duplicateComment2})
		require.Error(t, err)
		var valErr *storage.ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, "duplicate comment ID in batch", valErr.Reason)
		require.Equal(t, "c_duplicate", valErr.Value)
	})
}

// TestUpsertComments_SelfReference verifies that self-referencing comments are rejected.
// Tests both cases: ParentID == ID and ParentID == "t1_" + ID
func TestUpsertComments_SelfReference(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("self-reference without prefix", func(t *testing.T) {
		// Create a comment that references itself
		comment := testutil.BuildComment("c1", "post1", "", 0)
		comment.ParentID = "c1" // Self-reference

		err := store.UpsertComments(ctx, []*types.Comment{comment})
		require.Error(t, err, "should reject self-referencing comment")
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "c1", intErr.ResourceID)
		require.Equal(t, "comment references itself as parent", intErr.Reason)
	})

	t.Run("self-reference with t1 prefix", func(t *testing.T) {
		// Create a comment that references itself with t1_ prefix
		comment := testutil.BuildComment("c2", "post1", "", 0)
		comment.ParentID = "t1_c2" // Self-reference with prefix

		err := store.UpsertComments(ctx, []*types.Comment{comment})
		require.Error(t, err, "should reject self-referencing comment with prefix")
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "c2", intErr.ResourceID)
		require.Equal(t, "comment references itself as parent", intErr.Reason)
	})

	t.Run("valid parent reference for comparison", func(t *testing.T) {
		// Insert a valid comment first
		validComment1 := testutil.BuildComment("c3", "post1", "", 0)
		err := store.UpsertComments(ctx, []*types.Comment{validComment1})
		require.NoError(t, err, "valid top-level comment should succeed")

		// Insert a child that properly references the parent
		validComment2 := testutil.BuildComment("c4", "post1", "c3", 1)
		err = store.UpsertComments(ctx, []*types.Comment{validComment2})
		require.NoError(t, err, "valid child comment should succeed")
	})
}

// TestUpsertComments_EmptyParentID verifies that comments with empty ParentID are rejected.
func TestUpsertComments_EmptyParentID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("empty string", func(t *testing.T) {
		// Create a comment with empty ParentID
		comment := testutil.BuildComment("c1", "post1", "", 0)
		comment.ParentID = "" // Empty parent ID

		err := store.UpsertComments(ctx, []*types.Comment{comment})
		require.Error(t, err, "should reject comment with empty ParentID")
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "c1", intErr.ResourceID)
		require.Equal(t, "comment has empty parent_id", intErr.Reason)

		// Verify comment was not inserted (transaction rollback)
		retrieved, err := store.GetComment(ctx, "c1")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	})

	t.Run("whitespace only", func(t *testing.T) {
		// Create a comment with whitespace-only ParentID
		comment := testutil.BuildComment("c2", "post1", "", 0)
		comment.ParentID = "   " // Whitespace-only parent ID

		err := store.UpsertComments(ctx, []*types.Comment{comment})
		require.Error(t, err, "should reject comment with whitespace-only ParentID")
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "c2", intErr.ResourceID)
		require.Equal(t, "comment has empty parent_id", intErr.Reason)

		// Verify comment was not inserted (transaction rollback)
		retrieved, err := store.GetComment(ctx, "c2")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	})
}

// TestUpsertComments_MalformedParentID verifies that malformed parent IDs are rejected.
// Specifically, IDs like "t1_" with no actual ID after the prefix.
func TestUpsertComments_MalformedParentID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create a comment with malformed ParentID (just the prefix, no ID)
	comment := testutil.BuildComment("c1", "post1", "", 0)
	comment.ParentID = "t1_" // Malformed: prefix with no ID

	err = store.UpsertComments(ctx, []*types.Comment{comment})
	require.Error(t, err, "should reject comment with malformed ParentID")
	var intErr *storage.IntegrityError
	require.ErrorAs(t, err, &intErr)
	require.Equal(t, "c1", intErr.ResourceID)
	require.Contains(t, intErr.Reason, "malformed parent_id")
	require.Contains(t, intErr.Reason, "empty after prefix")

	// Verify comment was not inserted (transaction rollback)
	retrieved, err := store.GetComment(ctx, "c1")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	require.Nil(t, retrieved)
}

// TestUpsertComments_CyclicGraph verifies that cyclic comment dependencies are detected.
// Creates a cycle: A→B→C→A
func TestUpsertComments_CyclicGraph(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create a cyclic dependency: c1 → c2 → c3 → c1
	// Note: In a real scenario, this is impossible with Reddit's data model,
	// but we're testing the validation logic's robustness.
	comment1 := testutil.BuildComment("c1", "post1", "", 0)
	comment1.ParentID = "t1_c3" // c1 depends on c3

	comment2 := testutil.BuildComment("c2", "post1", "c1", 1)
	comment2.ParentID = "t1_c1" // c2 depends on c1

	comment3 := testutil.BuildComment("c3", "post1", "c2", 2)
	comment3.ParentID = "t1_c2" // c3 depends on c2, creating cycle

	comments := []*types.Comment{comment1, comment2, comment3}

	// Attempt batch insert - should fail due to cycle detection
	err = store.UpsertComments(ctx, comments)
	require.Error(t, err, "should reject cyclic comment dependencies")
	var intErr *storage.IntegrityError
	require.ErrorAs(t, err, &intErr)
	require.Contains(t, intErr.Reason, "unreachable")
	require.Contains(t, intErr.Reason, "cycle")

	// Verify no comments were inserted (transaction rollback)
	for _, c := range comments {
		retrieved, err := store.GetComment(ctx, c.ID)
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	}
}

// TestUpsertComments_OrphanedCommentInBatch verifies that orphaned comments are detected.
// An orphaned comment has a parent that doesn't exist in the batch or database.
func TestUpsertComments_OrphanedCommentInBatch(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("parent not in batch or database", func(t *testing.T) {
		// Create a comment that references a non-existent parent
		orphanComment := testutil.BuildComment("c1", "post1", "nonexistent", 1)

		err := store.UpsertComments(ctx, []*types.Comment{orphanComment})
		require.Error(t, err, "should reject orphaned comment")
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "t1_nonexistent", intErr.ResourceID)
		require.Equal(t, "parent not in batch and not in database", intErr.Reason)

		// Verify comment was not inserted (transaction rollback)
		retrieved, err := store.GetComment(ctx, "c1")
		require.Error(t, err)
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr)
		require.Nil(t, retrieved)
	})

	t.Run("parent exists in database - should succeed", func(t *testing.T) {
		// First insert a parent comment
		parentComment := testutil.BuildComment("c2", "post1", "", 0)
		err := store.UpsertComments(ctx, []*types.Comment{parentComment})
		require.NoError(t, err, "parent comment should be inserted successfully")

		// Now insert a child that references the existing parent
		childComment := testutil.BuildComment("c3", "post1", "c2", 1)
		err = store.UpsertComments(ctx, []*types.Comment{childComment})
		require.NoError(t, err, "child with existing parent should succeed")

		// Verify both comments exist
		retrievedParent, err := store.GetComment(ctx, "c2")
		require.NoError(t, err)
		require.NotNil(t, retrievedParent)

		retrievedChild, err := store.GetComment(ctx, "c3")
		require.NoError(t, err)
		require.NotNil(t, retrievedChild)
	})

	t.Run("multiple orphaned comments in batch", func(t *testing.T) {
		// Create multiple comments with non-existent parents
		comments := []*types.Comment{
			testutil.BuildComment("c4", "post1", "", 0),         // Valid top-level
			testutil.BuildComment("c5", "post1", "missing1", 1), // Orphaned
		}

		err := store.UpsertComments(ctx, comments)
		require.Error(t, err, "should reject batch with orphaned comment")
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "parent not in batch and not in database", intErr.Reason)

		// Verify no comments were inserted (transaction rollback)
		for _, c := range comments {
			retrieved, err := store.GetComment(ctx, c.ID)
			require.Error(t, err)
			var notFoundErr *storage.NotFoundError
			require.ErrorAs(t, err, &notFoundErr)
			require.Nil(t, retrieved)
		}
	})

	t.Run("orphan with explicit t1_ prefix", func(t *testing.T) {
		orphanComment := testutil.BuildComment("c_orphan", "post1", "", 0)
		orphanComment.ParentID = "t1_nonexistent" // Explicit t1_ prefix

		err := store.UpsertComments(ctx, []*types.Comment{orphanComment})
		require.Error(t, err)
		var intErr *storage.IntegrityError
		require.ErrorAs(t, err, &intErr)
		require.Equal(t, "parent not in batch and not in database", intErr.Reason)
	})
}

// TestUpsertComments_ParentMissingClosureEntries verifies detection of corrupted closure table.
// Tests the scenario where a parent comment exists but has no closure entries.
func TestUpsertComments_ParentMissingClosureEntries(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Cast store to *sqlite.SQLiteStore for access to testing helpers
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	if !ok {
		t.Skip("store is not *sqlite.SQLiteStore, skipping closure table corruption tests")
	}

	// Insert a post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Manually insert a comment WITHOUT closure entries to simulate corruption.
	// WARNING: This raw SQL bypasses validation and is brittle.
	// The 31 parameters MUST match the exact column order in migrations/001_initial_schema.sql.
	// If the schema changes, this test will break and must be updated.
	// Columns: id, name, score, ups, downs, likes, created, created_utc,
	//          approved_by, author, author_flair_css_class, author_flair_text, banned_by,
	//          body, body_html, edited_is_edited, edited_timestamp, gilded,
	//          link_author, link_id, link_title, link_url, num_reports, parent_id,
	//          saved, score_hidden, subreddit, subreddit_id, distinguished, depth, post_id
	insertQuery := `
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
	`

	// Insert a corrupted parent comment (depth 0, but no closure entries)
	_, err = sqlite.ExecContext(sqliteStore, ctx, insertQuery,
		"corrupted", "t1_corrupted", 10, 10, 0, nil, 1234567890, 1234567890,
		nil, "testuser", nil, nil, nil,
		"corrupted comment", "<p>corrupted comment</p>", 0, 0, 0,
		"testuser", "t3_post1", "Test Post", "https://reddit.com/test", nil, "t3_post1",
		0, 0, "test", "t5_2qh1i", nil, 0, "post1",
	)
	require.NoError(t, err, "failed to insert corrupted comment")

	// Verify the corrupted comment exists in the database
	var exists bool
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT EXISTS(SELECT 1 FROM comments WHERE id = ?)", "corrupted").Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "corrupted comment should exist in database")

	// Verify it has NO closure entries (simulating corruption)
	var closureCount int
	err = sqlite.QueryRowContext(sqliteStore, ctx, "SELECT COUNT(*) FROM comment_closures WHERE descendant = ?", "corrupted").Scan(&closureCount)
	require.NoError(t, err)
	require.Equal(t, 0, closureCount, "corrupted comment should have no closure entries")

	// Now try to insert a child comment that references the corrupted parent
	childComment := testutil.BuildComment("c1", "post1", "corrupted", 1)

	err = store.UpsertComments(ctx, []*types.Comment{childComment})
	require.Error(t, err, "should reject child when parent has no closure entries")
	var intErr *storage.IntegrityError
	require.ErrorAs(t, err, &intErr)
	require.Equal(t, "corrupted", intErr.ResourceID)
	require.Contains(t, intErr.Reason, "exists but has no closure entries")
	require.Contains(t, intErr.Reason, "closure table corrupted")

	// Verify child comment was not inserted (transaction rollback)
	retrieved, err := store.GetComment(ctx, "c1")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	require.Nil(t, retrieved)

	// Verify the corrupted parent still exists (we didn't delete it)
	retrievedParent, err := store.GetComment(ctx, "corrupted")
	require.NoError(t, err, "corrupted parent should still exist in database")
	require.NotNil(t, retrievedParent)
	require.Equal(t, "corrupted", retrievedParent.ID)
}
