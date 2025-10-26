//go:build integration

package sqlite_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite/internal"
	"github.com/stretchr/testify/require"
)

// TestTransactions_UpsertPostsCommitAll verifies that all posts in a batch
// are committed successfully. If any post is valid, all should be committed.
func TestTransactions_UpsertPostsCommitAll(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create 5 valid posts
	posts := testutil.BuildPostBatch(5, "golang")

	// Upsert all posts
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to upsert 5 valid posts")

	// Verify all 5 posts were committed by querying database directly
	testutil.AssertRowCount(t, store, "posts", 5)

	// Verify each post can be retrieved individually
	for i := 0; i < 5; i++ {
		retrieved, err := store.GetPost(ctx, posts[i].ID)
		require.NoError(t, err, "failed to retrieve post %d", i)
		require.NotNil(t, retrieved, "post %d should exist", i)
		require.Equal(t, posts[i].ID, retrieved.ID, "post %d ID mismatch", i)
	}
}

// TestTransactions_UpsertPostsRollbackOnNil verifies that UpsertPosts rejects
// batches containing nil entries and rolls back all changes.
// The implementation validates that all posts are non-nil before transaction.
func TestTransactions_UpsertPostsRollbackOnNil(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create batch with valid, nil, and valid posts
	posts := []*types.Post{
		testutil.BuildPost("p1", "golang"),
		nil, // Nil entry should cause entire batch to be rejected
		testutil.BuildPost("p3", "golang"),
	}

	// UpsertPosts must reject the batch due to nil entry
	err := store.UpsertPosts(ctx, posts)
	require.Error(t, err, "UpsertPosts should reject batch with nil entry")

	// Verify no posts were inserted (transaction rolled back)
	testutil.AssertRowCount(t, store, "posts", 0)

	// Verify we can insert valid posts after rejected operation
	validPosts := testutil.BuildPostBatch(2, "test")
	err = store.UpsertPosts(ctx, validPosts)
	require.NoError(t, err, "should be able to insert posts after rejected operation")

	// Verify the 2 valid posts were inserted
	testutil.AssertRowCount(t, store, "posts", 2)
}

// TestTransactions_UpsertCommentsCommitAll verifies that all comments in a batch
// are committed successfully when the parent post exists.
func TestTransactions_UpsertCommentsCommitAll(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create and insert parent post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert parent post")

	// Create 5 comments for the post
	comments := testutil.BuildCommentTree("post1", 0, 5) // 0 depth, 5 breadth = 5 top-level comments
	require.Equal(t, 5, len(comments), "should have 5 comments")

	// Upsert all comments
	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err, "failed to upsert 5 comments")

	// Verify all 5 comments were committed
	testutil.AssertRowCount(t, store, "comments", 5)

	// Verify each comment can be retrieved individually
	for i := 0; i < 5; i++ {
		retrieved, err := store.GetComment(ctx, comments[i].ID)
		require.NoError(t, err, "failed to retrieve comment %d", i)
		require.NotNil(t, retrieved, "comment %d should exist", i)
		require.Equal(t, comments[i].ID, retrieved.ID, "comment %d ID mismatch", i)
	}
}

// TestTransactions_UpsertCommentsRollbackOnInvalid verifies that UpsertComments
// rejects batches containing nil entries and rolls back all changes.
// The implementation validates that all comments are non-nil before transaction.
func TestTransactions_UpsertCommentsRollbackOnInvalid(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create and insert parent post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert parent post")

	// Create batch with valid, nil, and valid comments
	comments := []*types.Comment{
		testutil.BuildComment("c1", "post1", "", 0),
		nil, // Nil entry should cause entire batch to be rejected
		testutil.BuildComment("c3", "post1", "", 0),
	}

	// UpsertComments must reject the batch due to nil entry
	err = store.UpsertComments(ctx, comments)
	require.Error(t, err, "UpsertComments should reject batch with nil entry")

	// Verify no comments were inserted (transaction rolled back)
	testutil.AssertRowCount(t, store, "comments", 0)

	// Verify we can insert valid comments after rejected operation
	validComments := testutil.BuildCommentTree("post1", 0, 2)
	err = store.UpsertComments(ctx, validComments)
	require.NoError(t, err, "should be able to insert comments after rejected operation")

	// Verify the 2 valid comments were inserted
	testutil.AssertRowCount(t, store, "comments", 2)
}

// TestTransactions_UpsertCommentsDependencyOrdering verifies that comments
// with parent-child hierarchy are properly ordered during insertion, even when
// provided in shuffled order. The closure table should have correct depth entries.
func TestTransactions_UpsertCommentsDependencyOrdering(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Cast to SQLiteStore for direct database access
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Create and insert parent post
	post := testutil.BuildPost("post1", "test")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert parent post")

	// Create a comment hierarchy: parent, child, grandchild
	parent := testutil.BuildComment("parent", "post1", "", 0)
	child := testutil.BuildComment("child", "post1", "parent", 1)
	grandchild := testutil.BuildComment("grandchild", "post1", "child", 2)

	// Shuffle the order: insert grandchild, child, parent
	// This tests that the implementation handles out-of-order dependencies
	shuffledComments := []*types.Comment{grandchild, child, parent}

	// Upsert in shuffled order
	err = store.UpsertComments(ctx, shuffledComments)
	require.NoError(t, err, "failed to upsert comments in shuffled order")

	// Verify all 3 comments were committed
	testutil.AssertRowCount(t, store, "comments", 3)

	// Verify closure table has correct depth entries
	// Parent should have: (parent->parent, depth=0)
	var count int
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM comment_closures WHERE ancestor = ? AND descendant = ? AND depth = ?",
		"parent", "parent", 0).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "parent should have self-reference with depth=0")

	// Child should have: (child->child, depth=0) and (parent->child, depth=1)
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM comment_closures WHERE descendant = ? AND (depth = 0 OR depth = 1)",
		"child").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count, "child should have 2 closure entries")

	// Grandchild should have: (grandchild->grandchild, depth=0), (child->grandchild, depth=1), (parent->grandchild, depth=2)
	err = sqlite.QueryRowContext(sqliteStore, ctx,
		"SELECT COUNT(*) FROM comment_closures WHERE descendant = ?",
		"grandchild").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 3, count, "grandchild should have 3 closure entries")
}

// TestTransactions_EvictStaleCommitsAllOrNone verifies that EvictStale properly
// evicts posts based on their fetched_at timestamp. Posts older than maxAge
// should be evicted, while newer posts remain.
func TestTransactions_EvictStaleCommitsAllOrNone(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Cast to SQLiteStore for timestamp manipulation
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	require.True(t, ok, "store should be *sqlite.SQLiteStore")

	// Create posts with unique IDs to avoid conflicts
	oldPosts := make([]*types.Post, 10)
	for i := 0; i < 10; i++ {
		oldPosts[i] = testutil.BuildPost("old_"+fmt.Sprintf("%d", i), "old")
	}
	err := store.UpsertPosts(ctx, oldPosts)
	require.NoError(t, err, "failed to insert old posts")

	// Create different posts for recent
	recentPosts := make([]*types.Post, 5)
	for i := 0; i < 5; i++ {
		recentPosts[i] = testutil.BuildPost("recent_"+fmt.Sprintf("%d", i), "recent")
	}
	err = store.UpsertPosts(ctx, recentPosts)
	require.NoError(t, err, "failed to insert recent posts")

	// Verify we have 15 total posts
	testutil.AssertRowCount(t, store, "posts", 15)

	// Set fetched_at timestamps
	// Old posts: 2 hours ago (will be older than 1-hour threshold)
	oldTime := time.Now().Add(-2 * time.Hour)
	for _, post := range oldPosts {
		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, post.ID, oldTime)
		require.NoError(t, err, "failed to set old timestamp for post %s", post.ID)
	}

	// Recent posts: 30 minutes ago (will be newer than 1-hour threshold)
	recentTime := time.Now().Add(-30 * time.Minute)
	for _, post := range recentPosts {
		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, post.ID, recentTime)
		require.NoError(t, err, "failed to set recent timestamp for post %s", post.ID)
	}

	// Evict stale entries older than 1 hour
	maxAge := time.Hour
	evicted, err := store.EvictStale(ctx, maxAge)
	require.NoError(t, err, "failed to evict stale entries")

	// Verify 10 old posts were evicted
	require.Equal(t, int64(10), evicted, "should have evicted 10 old posts")

	// Verify 5 posts remain
	testutil.AssertRowCount(t, store, "posts", 5)

	// Verify remaining posts are the recent ones
	for _, post := range recentPosts {
		retrieved, err := store.GetPost(ctx, post.ID)
		require.NoError(t, err, "failed to retrieve recent post %s", post.ID)
		require.NotNil(t, retrieved, "recent post %s should still exist", post.ID)
	}

	// Verify old posts are gone
	for _, post := range oldPosts {
		_, err := store.GetPost(ctx, post.ID)
		require.Error(t, err, "old post %s should have been evicted", post.ID)
	}
}

// TestTransactions_ConcurrentTransactions verifies that multiple goroutines
// can safely upsert posts concurrently without deadlocks or data corruption.
// Each goroutine inserts different posts and all operations should complete
// successfully with all data committed.
func TestTransactions_ConcurrentTransactions(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	numGoroutines := 5
	postsPerGoroutine := 10
	var wg sync.WaitGroup

	// Track any errors from goroutines
	errChan := make(chan error, numGoroutines)

	// Launch goroutines to upsert posts concurrently
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Create posts with unique IDs for this goroutine
			posts := make([]*types.Post, postsPerGoroutine)
			for i := 0; i < postsPerGoroutine; i++ {
				// Generate unique ID: g{goroutineID}_p{postID}
				id := testutil.BuildPost(
					fmt.Sprintf("g%d_p%d", goroutineID, i),
					"concurrent",
				).ID
				posts[i] = testutil.BuildPost(id, "concurrent")
			}

			// Upsert posts for this goroutine
			if err := store.UpsertPosts(ctx, posts); err != nil {
				errChan <- err
				return
			}
		}(g)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors from goroutines
	for err := range errChan {
		require.NoError(t, err, "goroutine failed: %v", err)
	}

	// Verify all posts were committed
	// Total posts: 5 goroutines * 10 posts per goroutine = 50 posts
	expectedCount := int64(numGoroutines * postsPerGoroutine)
	testutil.AssertRowCount(t, store, "posts", expectedCount)

	// Verify we can retrieve posts from all goroutines
	for g := 0; g < numGoroutines; g++ {
		for i := 0; i < postsPerGoroutine; i++ {
			// Reconstruct the post ID
			id := testutil.BuildPost(
				fmt.Sprintf("g%d_p%d", g, i),
				"concurrent",
			).ID
			retrieved, err := store.GetPost(ctx, id)
			require.NoError(t, err, "failed to retrieve post from goroutine %d, post %d", g, i)
			require.NotNil(t, retrieved, "post from goroutine %d, post %d should exist", g, i)
		}
	}
}
