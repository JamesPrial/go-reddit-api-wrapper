//go:build integration

package sqlite_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	sqlite "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite/internal"
	"github.com/stretchr/testify/require"
)

// TestValidation_ListPostsInvalidSortByUsesDefault verifies that invalid
// SortBy values are safely handled. The operation should succeed and use
// the default sort order (created_utc DESC).
func TestValidation_ListPostsInvalidSortByUsesDefault(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create posts with different timestamps
	now := time.Now().Unix()
	post1 := testutil.BuildPost("p1", "test", testutil.WithCreatedUTC(float64(now-100)))
	post2 := testutil.BuildPost("p2", "test", testutil.WithCreatedUTC(float64(now-50)))
	post3 := testutil.BuildPost("p3", "test", testutil.WithCreatedUTC(float64(now)))

	err := store.UpsertPosts(ctx, []*types.Post{post1, post2, post3})
	require.NoError(t, err, "failed to insert test posts")

	// List posts with invalid SortBy - should use default (created_utc DESC)
	opts := &storage.ListPostsOptions{
		SortBy:  "invalid_field",
		SortDir: "desc",
		Limit:   100,
	}

	posts, err := store.ListPosts(ctx, opts)
	require.NoError(t, err, "should succeed with invalid SortBy, using default")
	require.Len(t, posts, 3, "should retrieve all 3 posts")

	// Verify default sort order (newest first): p3, p2, p1
	require.Equal(t, "p3", posts[0].ID, "first post should be p3 (newest)")
	require.Equal(t, "p2", posts[1].ID, "second post should be p2")
	require.Equal(t, "p1", posts[2].ID, "third post should be p1 (oldest)")
}

// TestValidation_ListPostsInvalidSortDirUsesDefault verifies that invalid
// SortDir values are safely handled. The operation should succeed and use
// the default sort direction (DESC).
func TestValidation_ListPostsInvalidSortDirUsesDefault(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create posts with different scores for sorting
	post1 := testutil.BuildPost("p1", "test", testutil.WithScore(10))
	post2 := testutil.BuildPost("p2", "test", testutil.WithScore(20))
	post3 := testutil.BuildPost("p3", "test", testutil.WithScore(30))

	err := store.UpsertPosts(ctx, []*types.Post{post1, post2, post3})
	require.NoError(t, err, "failed to insert test posts")

	// List posts with invalid SortDir - should use default (DESC)
	opts := &storage.ListPostsOptions{
		SortBy:  "score",
		SortDir: "sideways", // Invalid direction
		Limit:   100,
	}

	posts, err := store.ListPosts(ctx, opts)
	require.NoError(t, err, "should succeed with invalid SortDir, using default")
	require.Len(t, posts, 3, "should retrieve all 3 posts")

	// Verify default direction (DESC for score): p3 (30), p2 (20), p1 (10)
	require.Equal(t, "p3", posts[0].ID, "first post should be p3 (highest score)")
	require.Equal(t, 30, posts[0].Score)
	require.Equal(t, "p2", posts[1].ID, "second post should be p2")
	require.Equal(t, 20, posts[1].Score)
	require.Equal(t, "p1", posts[2].ID, "third post should be p1")
	require.Equal(t, 10, posts[2].Score)
}

// TestValidation_ListPostsNegativeLimit verifies that negative Limit values
// are handled gracefully. The behavior should be well-defined (e.g., return all
// or error gracefully).
func TestValidation_ListPostsNegativeLimit(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create 3 test posts
	posts := testutil.BuildPostBatch(3, "test")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to insert test posts")

	// List posts with negative limit
	opts := &storage.ListPostsOptions{
		Limit: -1,
	}

	retrievedPosts, err := store.ListPosts(ctx, opts)
	// The operation should either succeed (treating -1 as "no limit")
	// or return a validation error. Both are acceptable.
	if err != nil {
		t.Logf("negative limit returned error (acceptable): %v", err)
	} else {
		// If successful, should return some or all posts
		require.NotNil(t, retrievedPosts, "result should not be nil")
		t.Logf("negative limit succeeded and returned %d posts", len(retrievedPosts))
	}
}

// TestValidation_ListPostsNegativeOffset verifies that negative Offset values
// are handled gracefully. The behavior should be well-defined.
func TestValidation_ListPostsNegativeOffset(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create 3 test posts
	posts := testutil.BuildPostBatch(3, "test")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to insert test posts")

	// List posts with negative offset
	opts := &storage.ListPostsOptions{
		Offset: -1,
		Limit:  10,
	}

	retrievedPosts, err := store.ListPosts(ctx, opts)
	// The operation should either succeed (treating -1 as 0)
	// or return a validation error. Both are acceptable.
	if err != nil {
		t.Logf("negative offset returned error (acceptable): %v", err)
	} else {
		// If successful, should return posts
		require.NotNil(t, retrievedPosts, "result should not be nil")
		t.Logf("negative offset succeeded and returned %d posts", len(retrievedPosts))
	}
}

// TestValidation_ListPostsSQLInjectionAttempts verifies that SQL injection
// attempts are properly escaped and safe. After an injection attempt, the
// posts table should still exist and be queryable.
func TestValidation_ListPostsSQLInjectionAttempts(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create test posts in multiple subreddits
	post1 := testutil.BuildPost("p1", "test")
	post2 := testutil.BuildPost("p2", "golang")
	err := store.UpsertPosts(ctx, []*types.Post{post1, post2})
	require.NoError(t, err, "failed to insert test posts")

	// Cast to SQLiteStore for database validation
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Attempt SQL injection via subreddit filter
	maliciousSubreddit := "'; DROP TABLE posts; --"
	opts := &storage.ListPostsOptions{
		Subreddit: maliciousSubreddit,
		Limit:     100,
	}

	// The operation should succeed safely (returning 0 posts for non-matching filter)
	// and NOT execute the DROP TABLE statement
	posts, err := store.ListPosts(ctx, opts)
	require.NoError(t, err, "should safely handle SQL injection attempt")
	require.Empty(t, posts, "should return no posts for non-matching injection string")

	// Verify the posts table still exists and is intact
	var count int
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM posts").Scan(&count)
	require.NoError(t, err, "posts table should still exist after injection attempt")
	require.Equal(t, 2, count, "posts table should still contain 2 rows")

	// Verify we can still query normally
	opts = &storage.ListPostsOptions{
		Subreddit: "test",
		Limit:     100,
	}
	posts, err = store.ListPosts(ctx, opts)
	require.NoError(t, err, "should be able to query normally after injection attempt")
	require.Len(t, posts, 1, "should retrieve the test subreddit post")
}

// TestValidation_CommentTreeNegativeMaxDepth verifies that negative MaxDepth
// values in GetCommentTree are handled gracefully. Behavior should be well-defined
// (e.g., treated as unlimited or error).
func TestValidation_CommentTreeNegativeMaxDepth(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create post and comment hierarchy
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Create a simple hierarchy: parent and child
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),
		testutil.BuildComment("c2", "post1", "c1", 1),
	}
	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err, "failed to insert comments")

	// Get comment tree with negative MaxDepth
	opts := &storage.CommentTreeOptions{
		MaxDepth: -1,
	}

	retrievedComments, err := store.GetCommentTree(ctx, "post1", opts)
	// The operation should either succeed (treating -1 as unlimited)
	// or return a validation error. Both are acceptable.
	if err != nil {
		t.Logf("negative max depth returned error (acceptable): %v", err)
	} else {
		// If successful, should return comments
		require.NotNil(t, retrievedComments, "result should not be nil")
		require.Greater(t, len(retrievedComments), 0, "should return at least one comment")
		t.Logf("negative max depth succeeded and returned %d comments", len(retrievedComments))
	}
}

// TestValidation_EvictStaleZeroMaxAge verifies that EvictStale with maxAge=0
// is handled gracefully. It should either evict everything or be treated as
// a special case.
func TestValidation_EvictStaleZeroMaxAge(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create posts with various timestamps
	posts := testutil.BuildPostBatch(5, "test")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to insert posts")

	// Verify posts were inserted
	testutil.AssertRowCount(t, store, "posts", 5)

	// Call EvictStale with zero maxAge
	evicted, err := store.EvictStale(ctx, 0)
	// The operation should either evict all entries (since they're older than 0 duration)
	// or treat 0 as a special case and evict none or error.
	if err != nil {
		t.Logf("zero max age returned error (acceptable): %v", err)
	} else {
		t.Logf("zero max age evicted %d entries", evicted)
		// After operation, check how many entries remain
		// Both evicting all and evicting none are acceptable depending on implementation
	}
}

// TestValidation_EvictStaleNegativeMaxAge documents the behavior when EvictStale
// is called with a negative maxAge. A negative duration creates a future cutoff
// (NOW + duration), so all entries with past timestamps are considered stale.
func TestValidation_EvictStaleNegativeMaxAge(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create posts
	posts := testutil.BuildPostBatch(5, "test")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to insert posts")

	// Verify posts were inserted
	testutil.AssertRowCount(t, store, "posts", 5)

	// Call EvictStale with negative maxAge
	// This creates a cutoff time in the future (NOW - (-1 hour) = NOW + 1 hour)
	// All entries with timestamps in the past will be considered stale
	evicted, err := store.EvictStale(ctx, -1*time.Hour)
	require.NoError(t, err, "should handle negative max age")

	// The behavior depends on implementation:
	// With a future cutoff, all entries may be evicted (they're older than the future time)
	// Log the actual behavior for documentation
	t.Logf("negative max age evicted %d entries", evicted)
	require.Greater(t, evicted, int64(0), "negative max age creates future cutoff, may evict all entries")
}

// TestValidation_PostTitleMaxLength verifies that posts with very long titles
// are handled correctly. The implementation should either accept them or
// return a validation error.
func TestValidation_PostTitleMaxLength(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create post with very long title (10000 characters)
	longTitle := strings.Repeat("A", 10000)
	post := testutil.BuildPost("p1", "test", testutil.WithTitle(longTitle))

	// Attempt to upsert post with long title
	err := store.UpsertPost(ctx, post)
	// The operation should either succeed (truncating or storing as-is)
	// or return a validation error. Both are acceptable.
	if err != nil {
		t.Logf("long title returned error (acceptable): %v", err)
	} else {
		// If successful, retrieve and verify
		retrieved, err := store.GetPost(ctx, "p1")
		require.NoError(t, err, "should be able to retrieve post with long title")
		require.NotNil(t, retrieved, "post should exist")
		require.Equal(t, longTitle, retrieved.Title, "title should be preserved")
		t.Logf("long title succeeded, stored title length: %d", len(retrieved.Title))
	}
}

// TestValidation_CommentBodyMaxLength verifies that comments with very long
// bodies are handled correctly. The implementation should either accept them
// or return a validation error.
func TestValidation_CommentBodyMaxLength(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create post first (required for comment foreign key)
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Create comment with very long body (100000 characters)
	longBody := strings.Repeat("B", 100000)
	comment := testutil.BuildComment("c1", "post1", "", 0)
	comment.Body = longBody
	comment.BodyHTML = "<p>" + longBody + "</p>"

	// Attempt to upsert comment with long body
	err = store.UpsertComment(ctx, comment)
	// The operation should either succeed (truncating or storing as-is)
	// or return a validation error. Both are acceptable.
	if err != nil {
		t.Logf("long comment body returned error (acceptable): %v", err)
	} else {
		// If successful, retrieve and verify
		retrieved, err := store.GetComment(ctx, "c1")
		require.NoError(t, err, "should be able to retrieve comment with long body")
		require.NotNil(t, retrieved, "comment should exist")
		require.Equal(t, longBody, retrieved.Body, "body should be preserved")
		t.Logf("long comment body succeeded, stored body length: %d", len(retrieved.Body))
	}
}
