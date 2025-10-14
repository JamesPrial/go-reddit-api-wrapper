package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/storage/testutil"
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

		// Manually update the fetched_at timestamp for old posts to be very old
		oldTimestamp := time.Now().Add(-2 * time.Hour).Unix()
		_, err := store.db.ExecContext(ctx, "UPDATE posts SET fetched_at = ? WHERE id IN (?, ?)",
			oldTimestamp, "old1", "old2")
		require.NoError(t, err)

		// Insert comments for the old and new posts
		oldComments := testutil.BuildCommentTree("old1", 0, 2) // 2 top-level comments
		recentComments := testutil.BuildCommentTree("recent", 0, 2)

		err = store.UpsertComments(ctx, oldComments)
		require.NoError(t, err)
		err = store.UpsertComments(ctx, recentComments)
		require.NoError(t, err)

		// Update fetched_at for old comments
		for _, c := range oldComments {
			_, err := store.db.ExecContext(ctx, "UPDATE comments SET fetched_at = ? WHERE id = ?",
				oldTimestamp, c.ID)
			require.NoError(t, err)
		}

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

		// Update to old timestamp
		oldTimestamp := time.Now().Add(-2 * time.Hour).Unix()
		_, err = store.db.ExecContext(ctx, "UPDATE posts SET fetched_at = ? WHERE id = ?",
			oldTimestamp, "test2")
		require.NoError(t, err)

		// First eviction
		evicted1, err := store.EvictStale(ctx, 1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, int64(1), evicted1, "should evict 1 post")

		// Second eviction (should evict 0)
		evicted2, err := store.EvictStale(ctx, 1*time.Hour)
		require.NoError(t, err)
		require.Equal(t, int64(0), evicted2, "second eviction should find nothing to evict")
	})

	t.Run("evict respects time boundary", func(t *testing.T) {
		store := NewTestDB(t)
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

		now := time.Now()
		veryOld := now.Add(-3 * time.Hour).Unix()
		old := now.Add(-90 * time.Minute).Unix()

		// Update timestamps
		_, err := store.db.ExecContext(ctx, "UPDATE posts SET fetched_at = ? WHERE id = ?", veryOld, "veryold")
		require.NoError(t, err)
		_, err = store.db.ExecContext(ctx, "UPDATE posts SET fetched_at = ? WHERE id = ?", old, "old")
		require.NoError(t, err)

		// Evict entries older than 2 hours
		evicted, err := store.EvictStale(ctx, 2*time.Hour)
		require.NoError(t, err)
		require.Equal(t, int64(1), evicted, "should only evict very old post")

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
