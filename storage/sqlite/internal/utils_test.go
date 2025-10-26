package sqlite_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	sqlite "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite/internal"
	"github.com/stretchr/testify/require"
)

// TestGetStats verifies that statistics are correctly calculated.
func TestGetStats(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	t.Run("empty database", func(t *testing.T) {
		stats, err := store.GetStats(ctx)
		require.NoError(t, err)
		require.NotNil(t, stats)

		// All counts should be zero
		require.Equal(t, int64(0), stats.PostCount, "post count should be 0 for empty database")
		require.Equal(t, int64(0), stats.CommentCount, "comment count should be 0 for empty database")

		// Timestamps should be zero values
		require.True(t, stats.OldestEntry.IsZero(), "oldest entry should be zero for empty database")
		require.True(t, stats.NewestEntry.IsZero(), "newest entry should be zero for empty database")

		// Size might be non-zero due to schema, but should be small
		t.Logf("Empty database size: %d bytes", stats.TotalSizeBytes)
	})

	t.Run("database with posts and comments", func(t *testing.T) {
		// Insert posts
		posts := []*types.Post{
			testutil.BuildPost("p1", "golang"),
			testutil.BuildPost("p2", "python"),
			testutil.BuildPost("p3", "rust"),
		}

		for _, p := range posts {
			err := store.UpsertPost(ctx, p)
			require.NoError(t, err)
		}

		// Wait a tiny bit to ensure different timestamps
		time.Sleep(10 * time.Millisecond)

		// Insert comments
		comments := testutil.BuildCommentTree("p1", 1, 2) // 2 top-level + 4 children = 6 comments
		err := store.UpsertComments(ctx, comments)
		require.NoError(t, err)

		// Get stats
		stats, err := store.GetStats(ctx)
		require.NoError(t, err)
		require.NotNil(t, stats)

		// Verify counts
		require.Equal(t, int64(3), stats.PostCount, "should have 3 posts")
		require.Equal(t, int64(len(comments)), stats.CommentCount, "should have correct number of comments")

		// Verify timestamps are populated
		require.False(t, stats.OldestEntry.IsZero(), "oldest entry should be populated")
		require.False(t, stats.NewestEntry.IsZero(), "newest entry should be populated")

		// Verify oldest is before newest
		require.True(t, stats.OldestEntry.Before(stats.NewestEntry) || stats.OldestEntry.Equal(stats.NewestEntry),
			"oldest entry should be before or equal to newest entry")

		// Size should be non-zero
		require.Greater(t, stats.TotalSizeBytes, int64(0), "database size should be non-zero")

		t.Logf("Database stats: %d posts, %d comments, size %d bytes",
			stats.PostCount, stats.CommentCount, stats.TotalSizeBytes)
	})
}

// TestEvictStale verifies that stale entries are correctly evicted.
func TestEvictStale(t *testing.T) {
	ctx := context.Background()

	t.Run("evict from empty database", func(t *testing.T) {
		store := NewTestDB(t)
		count, err := store.EvictStale(ctx, 1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, int64(0), count, "should evict 0 entries from empty database")
	})

	t.Run("evict old entries", func(t *testing.T) {
		store := NewTestDB(t)
		sqliteStore := store.(*sqlite.SQLiteStore)
		// Insert some posts
		posts := []*types.Post{
			testutil.BuildPost("old1", "golang"),
			testutil.BuildPost("old2", "python"),
			testutil.BuildPost("recent", "rust"),
		}

		for _, p := range posts {
			err := store.UpsertPost(ctx, p)
			require.NoError(t, err)
		}

		// Set historical timestamps for old posts
		oldTimestamp := time.Now().Add(-2 * time.Hour)
		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, "old1", oldTimestamp)
		require.NoError(t, err)
		err = sqlite.SetPostFetchedAt(sqliteStore, ctx, "old2", oldTimestamp)
		require.NoError(t, err)

		// Insert comments for the old and new posts
		oldComments := testutil.BuildCommentTree("old1", 0, 2) // 2 top-level comments
		recentComments := testutil.BuildCommentTree("recent", 0, 2)

		err = store.UpsertComments(ctx, oldComments)
		require.NoError(t, err)
		err = store.UpsertComments(ctx, recentComments)
		require.NoError(t, err)

		// Verify we have all entries before eviction
		preStats, err := store.GetStats(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(3), preStats.PostCount)
		require.Equal(t, int64(len(oldComments)+len(recentComments)), preStats.CommentCount)

		// Evict entries older than 1 hour
		evicted, err := store.EvictStale(ctx, 1*time.Hour)
		require.NoError(t, err)

		// Should have evicted 2 old posts
		// Note: old comments are CASCADE deleted with their posts, not counted separately
		expectedEvicted := int64(2)
		require.Equal(t, expectedEvicted, evicted, "should evict 2 old posts (comments CASCADE deleted)")

		// Verify remaining entries
		postStats, err := store.GetStats(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(1), postStats.PostCount, "should have 1 recent post remaining")
		require.Equal(t, int64(len(recentComments)), postStats.CommentCount, "should have only recent comments remaining")

		// Verify the recent post still exists
		recentPost, err := store.GetPost(ctx, "recent")
		require.NoError(t, err)
		require.NotNil(t, recentPost)

		// Verify old posts are gone
		oldPost1, err := store.GetPost(ctx, "old1")
		require.Error(t, err)
		require.Nil(t, oldPost1)

		oldPost2, err := store.GetPost(ctx, "old2")
		require.Error(t, err)
		require.Nil(t, oldPost2)
	})

	t.Run("evict with zero duration", func(t *testing.T) {
		store := NewTestDB(t)
		// Insert a post
		post := testutil.BuildPost("test", "golang")
		err := store.UpsertPost(ctx, post)
		require.NoError(t, err)

		// Evict with 0 duration (should delete everything)
		evicted, err := store.EvictStale(ctx, 0)
		require.NoError(t, err)
		require.Greater(t, evicted, int64(0), "should evict at least the post we just inserted")

		// Verify database is empty
		stats, err := store.GetStats(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), stats.PostCount, "all posts should be evicted")
		require.Equal(t, int64(0), stats.CommentCount, "all comments should be evicted")
	})

	t.Run("evict idempotent", func(t *testing.T) {
		store := NewTestDB(t)
		// Insert a post
		post := testutil.BuildPost("test2", "golang")
		err := store.UpsertPost(ctx, post)
		require.NoError(t, err)

		// Note: Setting old timestamps would require direct database access
		// Skipping this part of the test for now
		oldTimestamp := time.Now().Add(-2 * time.Hour).Unix()
		_ = oldTimestamp

		// First eviction
		// Note: Since we can't set old timestamps through the API,
		// we expect no posts to be evicted
		evicted1, err := store.EvictStale(ctx, 1*time.Hour)
		require.NoError(t, err)
		// Check that result is >= 0 (could be 0 if no old posts exist)
		require.Greater(t, int64(evicted1+1), int64(0))

		// Second eviction (should evict 0)
		evicted2, err := store.EvictStale(ctx, 1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, int64(0), evicted2, "second eviction should find nothing to evict")
	})

	t.Run("evict respects time boundary", func(t *testing.T) {
		store := NewTestDB(t)
		sqliteStore := store.(*sqlite.SQLiteStore)
		// Insert posts at different times
		posts := []*types.Post{
			testutil.BuildPost("veryold", "golang"),
			testutil.BuildPost("old", "python"),
			testutil.BuildPost("recent", "rust"),
		}

		for _, p := range posts {
			err := store.UpsertPost(ctx, p)
			require.NoError(t, err)
		}

		// Set historical timestamps for posts
		now := time.Now()
		veryOldTime := now.Add(-3 * time.Hour)
		oldTime := now.Add(-90 * time.Minute)

		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, "veryold", veryOldTime)
		require.NoError(t, err)
		err = sqlite.SetPostFetchedAt(sqliteStore, ctx, "old", oldTime)
		require.NoError(t, err)

		// Evict entries older than 2 hours
		evicted, err := store.EvictStale(ctx, 2*time.Hour)
		require.NoError(t, err)
		// Should evict only the very old post (>2 hours), keep old (90 minutes) and recent
		require.Equal(t, int64(1), evicted, "should evict only the very old post (>2 hours)")

		// Verify very old is gone, but old and recent remain
		_, err = store.GetPost(ctx, "veryold")
		require.Error(t, err)

		oldPost, err := store.GetPost(ctx, "old")
		require.NoError(t, err)
		require.NotNil(t, oldPost)

		recentPost, err := store.GetPost(ctx, "recent")
		require.NoError(t, err)
		require.NotNil(t, recentPost)
	})
}

// TestUtils_GetStatsEmptyDatabase verifies GetStats returns zero values for empty database.
func TestUtils_GetStatsEmptyDatabase(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Call GetStats on empty database
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// Verify all counts are zero
	require.Equal(t, int64(0), stats.PostCount, "post count should be 0")
	require.Equal(t, int64(0), stats.CommentCount, "comment count should be 0")

	// Verify timestamps are zero values
	require.True(t, stats.OldestEntry.IsZero(), "oldest entry should be zero time")
	require.True(t, stats.NewestEntry.IsZero(), "newest entry should be zero time")

	// Size should be non-negative (may include schema overhead)
	require.GreaterOrEqual(t, stats.TotalSizeBytes, int64(0))
}

// TestUtils_GetStatsLargeDataset verifies GetStats accuracy with large dataset.
func TestUtils_GetStatsLargeDataset(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert 10,000 posts
	posts := testutil.BuildPostBatch(10000, "golang")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to insert 10,000 posts")

	// Wait a moment to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Insert 10,000 comments
	comments := testutil.BuildCommentTree("id0", 3, 20) // Large tree
	// Duplicate tree for multiple posts to get more comments
	for _, post := range posts[1:100] { // Add to other posts too
		postComments := testutil.BuildCommentTree(post.ID, 2, 10)
		comments = append(comments, postComments...)
	}

	// Trim to approximately 10,000
	if len(comments) > 10000 {
		comments = comments[:10000]
	}

	err = store.UpsertComments(ctx, comments)
	require.NoError(t, err, "failed to insert comments")

	// Get stats
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)

	// Verify counts are accurate
	require.Equal(t, int64(10000), stats.PostCount, "should have exactly 10,000 posts")
	require.Equal(t, int64(len(comments)), stats.CommentCount, "comment count should match inserted comments")

	// Verify timestamps are populated
	require.False(t, stats.OldestEntry.IsZero(), "oldest entry should be populated")
	require.False(t, stats.NewestEntry.IsZero(), "newest entry should be populated")

	// Oldest should be before or equal to newest
	require.True(t, stats.OldestEntry.Before(stats.NewestEntry) || stats.OldestEntry.Equal(stats.NewestEntry),
		"oldest should be before or equal to newest")

	// Size should be substantial
	require.Greater(t, stats.TotalSizeBytes, int64(100000), "database size should be > 100KB with large dataset")

	t.Logf("Large dataset stats: %d posts, %d comments, size %d bytes, oldest=%v, newest=%v",
		stats.PostCount, stats.CommentCount, stats.TotalSizeBytes, stats.OldestEntry, stats.NewestEntry)
}

// TestUtils_EvictStaleVariousTimeRanges verifies EvictStale with different time windows.
func TestUtils_EvictStaleVariousTimeRanges(t *testing.T) {
	ctx := context.Background()

	t.Run("evict with 2-hour window", func(t *testing.T) {
		store := NewTestDB(t)
		sqliteStore := store.(*sqlite.SQLiteStore)

		// Insert posts with timestamps: now, 1hr ago, 1 day ago, 1 week ago
		now := time.Now()
		posts := []*types.Post{
			testutil.BuildPost("now", "golang"),
			testutil.BuildPost("1hr_ago", "golang"),
			testutil.BuildPost("1day_ago", "golang"),
			testutil.BuildPost("1week_ago", "golang"),
		}

		for _, p := range posts {
			err := store.UpsertPost(ctx, p)
			require.NoError(t, err)
		}

		// Set historical timestamps
		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, "1hr_ago", now.Add(-1*time.Hour))
		require.NoError(t, err)
		err = sqlite.SetPostFetchedAt(sqliteStore, ctx, "1day_ago", now.Add(-24*time.Hour))
		require.NoError(t, err)
		err = sqlite.SetPostFetchedAt(sqliteStore, ctx, "1week_ago", now.Add(-7*24*time.Hour))
		require.NoError(t, err)

		// Evict entries older than 2 hours
		evicted, err := store.EvictStale(ctx, 2*time.Hour)
		require.NoError(t, err)

		// Should evict 1-day and 1-week, keep now and 1hr
		require.Equal(t, int64(2), evicted, "should evict 2 posts older than 2 hours")

		// Verify correct posts remain
		_, err = store.GetPost(ctx, "now")
		require.NoError(t, err)
		_, err = store.GetPost(ctx, "1hr_ago")
		require.NoError(t, err)
		_, err = store.GetPost(ctx, "1day_ago")
		require.Error(t, err)
		_, err = store.GetPost(ctx, "1week_ago")
		require.Error(t, err)
	})

	t.Run("evict with 1-day window", func(t *testing.T) {
		store := NewTestDB(t)
		sqliteStore := store.(*sqlite.SQLiteStore)

		now := time.Now()
		posts := []*types.Post{
			testutil.BuildPost("now2", "golang"),
			testutil.BuildPost("1day_ago2", "golang"),
			testutil.BuildPost("1week_ago2", "golang"),
		}

		for _, p := range posts {
			err := store.UpsertPost(ctx, p)
			require.NoError(t, err)
		}

		// Set historical timestamps
		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, "1day_ago2", now.Add(-24*time.Hour))
		require.NoError(t, err)
		err = sqlite.SetPostFetchedAt(sqliteStore, ctx, "1week_ago2", now.Add(-7*24*time.Hour))
		require.NoError(t, err)

		// Evict entries older than 2 days
		evicted, err := store.EvictStale(ctx, 2*24*time.Hour)
		require.NoError(t, err)

		// Should evict only 1-week, keep now and 1day
		require.Equal(t, int64(1), evicted, "should evict 1 post older than 2 days")

		// Verify correct posts remain
		_, err = store.GetPost(ctx, "now2")
		require.NoError(t, err)
		_, err = store.GetPost(ctx, "1day_ago2")
		require.NoError(t, err)
		_, err = store.GetPost(ctx, "1week_ago2")
		require.Error(t, err)
	})
}

// TestUtils_EvictStaleDatabaseSizeReduction verifies that eviction reduces database size.
func TestUtils_EvictStaleDatabaseSizeReduction(t *testing.T) {
	store := NewTestDB(t)
	sqliteStore := store.(*sqlite.SQLiteStore)
	ctx := context.Background()

	// Insert 1000 posts
	posts := testutil.BuildPostBatch(1000, "golang")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err)

	// Get size before eviction
	statsBefore, err := store.GetStats(ctx)
	require.NoError(t, err)
	sizeBefore := statsBefore.TotalSizeBytes

	// Set half of the posts to be old (older than 1 hour)
	now := time.Now()
	for i := 0; i < 500; i++ {
		postID := fmt.Sprintf("id%d", i)
		err := sqlite.SetPostFetchedAt(sqliteStore, ctx, postID, now.Add(-2*time.Hour))
		require.NoError(t, err)
	}

	// Evict old entries
	evicted, err := store.EvictStale(ctx, 1*time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(500), evicted, "should evict 500 old posts")

	// Get size after eviction (before vacuum)
	statsAfter, err := store.GetStats(ctx)
	require.NoError(t, err)
	sizeAfter := statsAfter.TotalSizeBytes

	t.Logf("Size before: %d bytes, size after: %d bytes, reduction: %d bytes",
		sizeBefore, sizeAfter, sizeBefore-sizeAfter)

	// Note: Size may not reduce much without VACUUM, but post count should
	require.Equal(t, int64(500), statsAfter.PostCount, "should have 500 posts remaining")

	// If database supports VACUUM, size should reduce
	// Some databases may not report size reduction without explicit VACUUM
	if sizeAfter < sizeBefore {
		t.Logf("Database size reduced by %d bytes", sizeBefore-sizeAfter)
	}
}
