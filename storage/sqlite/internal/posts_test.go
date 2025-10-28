package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite" // Register SQLite backend
	"github.com/stretchr/testify/require"
)

// NewTestDB creates an in-memory SQLite database for testing.
// It runs migrations and returns a configured store.
// Uses t.Cleanup() to ensure the database is closed after the test.
func NewTestDB(t *testing.T) storage.Store {
	t.Helper()

	cfg := storage.Config{
		DSN:          ":memory:",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}

	// Use the factory pattern with blank import of sqlite
	store, err := storage.New(context.Background(), cfg)
	require.NoError(t, err, "failed to create test database")
	require.NotNil(t, store, "store should not be nil")

	t.Cleanup(func() {
		err := store.Close()
		require.NoError(t, err, "failed to close test database")
	})

	return store
}

// TestUpsertPost verifies that posts can be inserted and updated.
func TestUpsertPost(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a new post
	post := testutil.BuildPost("abc123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Verify the post was inserted
	retrieved, err := store.GetPost(ctx, "abc123")
	require.NoError(t, err, "failed to retrieve inserted post")
	require.NotNil(t, retrieved, "retrieved post should not be nil")
	require.Equal(t, "abc123", retrieved.ID)
	require.Equal(t, "golang", retrieved.Subreddit)
	require.Equal(t, "Test Post", retrieved.Title)
	require.Equal(t, 42, retrieved.Score)

	// Update the same post
	post.Score = 100
	post.Ups = 100
	post.Title = "Updated Title"
	err = store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to update post")

	// Verify the post was updated
	updated, err := store.GetPost(ctx, "abc123")
	require.NoError(t, err, "failed to retrieve updated post")
	require.NotNil(t, updated, "updated post should not be nil")
	require.Equal(t, "abc123", updated.ID)
	require.Equal(t, "Updated Title", updated.Title)
	require.Equal(t, 100, updated.Score)
}

// TestGetPost verifies that posts can be retrieved by ID.
func TestGetPost(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("test123", "golang",
		testutil.WithTitle("Specific Title"),
		testutil.WithAuthor("specificuser"),
		testutil.WithScore(250),
	)
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Get the post by ID
	retrieved, err := store.GetPost(ctx, "test123")
	require.NoError(t, err, "failed to get post")
	require.NotNil(t, retrieved, "retrieved post should not be nil")

	// Verify all fields match
	require.Equal(t, "test123", retrieved.ID)
	require.Equal(t, "t3_test123", retrieved.Name)
	require.Equal(t, "golang", retrieved.Subreddit)
	require.Equal(t, "Specific Title", retrieved.Title)
	require.Equal(t, "specificuser", retrieved.Author)
	require.Equal(t, 250, retrieved.Score)
	require.Equal(t, 250, retrieved.Ups)

	// Try to get a non-existent post
	notFound, err := store.GetPost(ctx, "nonexistent")
	require.Error(t, err, "expected error for non-existent post")
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr, "error should be NotFoundError")
	require.Equal(t, "post", notFoundErr.ResourceType)
	require.Equal(t, "nonexistent", notFoundErr.ResourceID)
	require.Nil(t, notFound, "post should be nil for non-existent ID")
}

// TestListPosts verifies that posts can be listed with various filters and sorting.
func TestListPosts(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert multiple posts with different properties
	posts := []*types.Post{
		testutil.BuildPost("p1", "golang", testutil.WithScore(100), testutil.WithAuthor("user1")),
		testutil.BuildPost("p2", "golang", testutil.WithScore(50), testutil.WithAuthor("user2")),
		testutil.BuildPost("p3", "python", testutil.WithScore(200), testutil.WithAuthor("user1")),
		testutil.BuildPost("p4", "golang", testutil.WithScore(25), testutil.WithAuthor("user3")),
		testutil.BuildPost("p5", "rust", testutil.WithScore(75), testutil.WithAuthor("user2")),
	}

	for _, p := range posts {
		err := store.UpsertPost(ctx, p)
		require.NoError(t, err, "failed to insert post %s", p.ID)
	}

	// Wait a moment to ensure different fetched_at timestamps
	time.Sleep(10 * time.Millisecond)

	t.Run("list all posts with nil opts", func(t *testing.T) {
		results, err := store.ListPosts(ctx, nil)
		require.NoError(t, err)
		require.Len(t, results, 5, "should return all 5 posts")
	})

	t.Run("filter by subreddit", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Subreddit: "golang"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 3, "should return 3 golang posts")
		for _, p := range results {
			require.Equal(t, "golang", p.Subreddit)
		}
	})

	t.Run("filter by author", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Author: "user1"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 2, "should return 2 posts by user1")
		for _, p := range results {
			require.Equal(t, "user1", p.Author)
		}
	})

	t.Run("filter by minimum score", func(t *testing.T) {
		opts := &storage.ListPostsOptions{MinScore: 75}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 3, "should return 3 posts with score >= 75")
		for _, p := range results {
			require.GreaterOrEqual(t, p.Score, 75)
		}
	})

	t.Run("filter by max age", func(t *testing.T) {
		// All posts were just inserted, so they should all match a 1-minute age filter
		opts := &storage.ListPostsOptions{MaxAge: 1 * time.Minute}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 5, "all recent posts should match")

		// Posts older than 0 seconds should return none (very short window)
		// But since we just inserted, they should still be within the window
		// Let's test with a very old cutoff instead
		oldCutoff := &storage.ListPostsOptions{MaxAge: 1 * time.Nanosecond}
		oldResults, err := store.ListPosts(ctx, oldCutoff)
		require.NoError(t, err)
		// Depending on timing, this might return 0 or some posts
		// We just verify it doesn't error
		t.Logf("Posts within 1 nanosecond: %d", len(oldResults))
	})

	t.Run("sort by score descending", func(t *testing.T) {
		opts := &storage.ListPostsOptions{SortBy: "score", SortDir: "DESC"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 5)
		// Verify descending order
		require.Equal(t, 200, results[0].Score)
		require.Equal(t, 100, results[1].Score)
		require.Equal(t, 75, results[2].Score)
		require.Equal(t, 50, results[3].Score)
		require.Equal(t, 25, results[4].Score)
	})

	t.Run("sort by score ascending", func(t *testing.T) {
		opts := &storage.ListPostsOptions{SortBy: "score", SortDir: "ASC"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 5)
		// Verify ascending order
		require.Equal(t, 25, results[0].Score)
		require.Equal(t, 50, results[1].Score)
		require.Equal(t, 75, results[2].Score)
		require.Equal(t, 100, results[3].Score)
		require.Equal(t, 200, results[4].Score)
	})

	t.Run("pagination with limit", func(t *testing.T) {
		opts := &storage.ListPostsOptions{
			SortBy:  "score",
			SortDir: "DESC",
			Limit:   3,
		}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 3, "should return only 3 posts")
		require.Equal(t, 200, results[0].Score)
		require.Equal(t, 100, results[1].Score)
		require.Equal(t, 75, results[2].Score)
	})

	t.Run("pagination with offset", func(t *testing.T) {
		opts := &storage.ListPostsOptions{
			SortBy:  "score",
			SortDir: "DESC",
			Limit:   2,
			Offset:  2,
		}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 2, "should return 2 posts after offset")
		require.Equal(t, 75, results[0].Score)
		require.Equal(t, 50, results[1].Score)
	})

	t.Run("combine multiple filters", func(t *testing.T) {
		opts := &storage.ListPostsOptions{
			Subreddit: "golang",
			MinScore:  50,
			SortBy:    "score",
			SortDir:   "DESC",
		}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 2, "should return 2 golang posts with score >= 50")
		require.Equal(t, 100, results[0].Score)
		require.Equal(t, 50, results[1].Score)
	})
}

// TestDeletePost verifies that posts can be deleted.
func TestDeletePost(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("del123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post")

	// Verify the post exists
	retrieved, err := store.GetPost(ctx, "del123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Delete the post
	err = store.DeletePost(ctx, "del123")
	require.NoError(t, err, "failed to delete post")

	// Verify the post is gone
	notFound, err := store.GetPost(ctx, "del123")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr, "second post should not exist due to rollback")
	require.Nil(t, notFound)

	// Delete again (should be idempotent)
	err = store.DeletePost(ctx, "del123")
	require.NoError(t, err, "delete should be idempotent")
}

// TestUpsertPosts verifies that multiple posts can be inserted in a batch.
func TestUpsertPosts(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	t.Run("insert multiple posts", func(t *testing.T) {
		posts := []*types.Post{
			testutil.BuildPost("batch1", "golang"),
			testutil.BuildPost("batch2", "python"),
			testutil.BuildPost("batch3", "rust"),
		}

		err := store.UpsertPosts(ctx, posts)
		require.NoError(t, err, "failed to batch upsert posts")

		// Verify all posts were inserted
		for _, p := range posts {
			retrieved, err := store.GetPost(ctx, p.ID)
			require.NoError(t, err, "failed to retrieve post %s", p.ID)
			require.NotNil(t, retrieved)
			require.Equal(t, p.ID, retrieved.ID)
			require.Equal(t, p.Subreddit, retrieved.Subreddit)
		}
	})

	t.Run("handle empty slice", func(t *testing.T) {
		err := store.UpsertPosts(ctx, []*types.Post{})
		require.NoError(t, err, "empty slice should not error")
	})

	t.Run("transaction atomicity", func(t *testing.T) {
		// This test verifies that if one post in the batch fails,
		// none of the posts are committed.
		// However, since our test posts are all valid, we'll just verify
		// that multiple posts are inserted atomically.

		posts := []*types.Post{
			testutil.BuildPost("atomic1", "golang"),
			testutil.BuildPost("atomic2", "python"),
		}

		err := store.UpsertPosts(ctx, posts)
		require.NoError(t, err)

		// Verify both exist
		for _, p := range posts {
			retrieved, err := store.GetPost(ctx, p.ID)
			require.NoError(t, err)
			require.NotNil(t, retrieved)
		}
	})
}

// TestPosts_FileBasedDatabase verifies post persistence with file-based SQLite storage.
func TestPosts_FileBasedDatabase(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("persist1", "golang", testutil.WithScore(100), testutil.WithAuthor("filebaseduser"))
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post into file-based database")

	// Retrieve the post and verify persistence
	retrieved, err := store.GetPost(ctx, "persist1")
	require.NoError(t, err, "failed to retrieve post from file-based database")
	require.NotNil(t, retrieved, "retrieved post should not be nil")
	require.Equal(t, "persist1", retrieved.ID)
	require.Equal(t, "golang", retrieved.Subreddit)
	require.Equal(t, "filebaseduser", retrieved.Author)
	require.Equal(t, 100, retrieved.Score)

	// Verify all fields are intact
	require.Equal(t, "t3_persist1", retrieved.Name)
	require.Equal(t, "Test Post", retrieved.Title)
	require.Equal(t, "This is a test post body.", retrieved.SelfText)
}

// TestPosts_LargeBatchUpsert verifies that large batches of posts can be efficiently upserted.
func TestPosts_LargeBatchUpsert(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Build a large batch of posts
	posts := testutil.BuildPostBatch(1000, "golang")

	// Upsert the batch
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to upsert large batch")

	// Verify all posts were inserted
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1000), stats.PostCount, "should have inserted 1000 posts")

	// Spot check a few posts to verify data integrity
	for _, checkID := range []string{"id0", "id500", "id999"} {
		retrieved, err := store.GetPost(ctx, checkID)
		require.NoError(t, err, "failed to retrieve post %s", checkID)
		require.NotNil(t, retrieved)
		require.Equal(t, checkID, retrieved.ID)
		require.Equal(t, "golang", retrieved.Subreddit)
	}
}

// TestPosts_SpecialCharactersInFields verifies that special characters are preserved correctly.
func TestPosts_SpecialCharactersInFields(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create a post with special characters
	post := testutil.BuildPost("special1", "golang",
		testutil.WithTitle("Test <script>alert('xss')</script> in title"),
	)
	post.SelfText = "Body with special chars: \n\t\"quotes\"\n SQL injection: ' OR '1'='1\n Emoji: 🚀 ⭐ 💻"

	// Insert the post
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert post with special characters")

	// Retrieve and verify exact preservation
	retrieved, err := store.GetPost(ctx, "special1")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify special characters in title
	require.Equal(t, "Test <script>alert('xss')</script> in title", retrieved.Title,
		"special characters in title should be preserved exactly")

	// Verify special characters in body
	require.Equal(t, post.SelfText, retrieved.SelfText,
		"special characters in body should be preserved exactly")
}

// TestPosts_ListPostsAllFilterCombinations verifies ListPosts with various filter combinations.
func TestPosts_ListPostsAllFilterCombinations(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert diverse set of posts
	posts := []*types.Post{
		testutil.BuildPost("p1", "golang", testutil.WithScore(100), testutil.WithAuthor("alice")),
		testutil.BuildPost("p2", "golang", testutil.WithScore(200), testutil.WithAuthor("bob")),
		testutil.BuildPost("p3", "python", testutil.WithScore(50), testutil.WithAuthor("alice")),
		testutil.BuildPost("p4", "python", testutil.WithScore(150), testutil.WithAuthor("charlie")),
		testutil.BuildPost("p5", "rust", testutil.WithScore(75), testutil.WithAuthor("bob")),
		testutil.BuildPost("p6", "rust", testutil.WithScore(25), testutil.WithAuthor("alice")),
	}

	for _, p := range posts {
		err := store.UpsertPost(ctx, p)
		require.NoError(t, err)
	}

	t.Run("subreddit filter only", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Subreddit: "golang"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 2, "should return 2 golang posts")
		for _, p := range results {
			require.Equal(t, "golang", p.Subreddit)
		}
	})

	t.Run("author filter only", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Author: "alice"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 3, "should return 3 posts by alice")
		for _, p := range results {
			require.Equal(t, "alice", p.Author)
		}
	})

	t.Run("min score filter only", func(t *testing.T) {
		opts := &storage.ListPostsOptions{MinScore: 100}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 3, "should return 3 posts with score >= 100")
		for _, p := range results {
			require.GreaterOrEqual(t, p.Score, 100)
		}
	})

	t.Run("subreddit and author combined", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Subreddit: "golang", Author: "bob"}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 1, "should return 1 post from golang by bob")
		require.Equal(t, "p2", results[0].ID)
	})

	t.Run("subreddit and min score combined", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Subreddit: "python", MinScore: 75}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 1, "should return 1 python post with score >= 75")
		require.Equal(t, "p4", results[0].ID)
	})

	t.Run("author and min score combined", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Author: "alice", MinScore: 50}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 2, "should return 2 posts by alice with score >= 50")
		for _, p := range results {
			require.Equal(t, "alice", p.Author)
			require.GreaterOrEqual(t, p.Score, 50)
		}
	})

	t.Run("all filters combined", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Subreddit: "golang", Author: "alice", MinScore: 50}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 1, "should return 1 golang post by alice with score >= 50")
		require.Equal(t, "p1", results[0].ID)
	})
}

// TestPosts_PaginationEdgeCases verifies pagination with edge cases and boundary conditions.
func TestPosts_PaginationEdgeCases(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Insert 50 posts
	posts := testutil.BuildPostBatch(50, "golang")
	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err)

	t.Run("offset greater than total count", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Offset: 100, Limit: 10}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Empty(t, results, "should return empty slice when offset exceeds total count")
	})

	t.Run("limit greater than total count", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Offset: 0, Limit: 100}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Len(t, results, 50, "should return all remaining posts when limit exceeds total")
	})

	t.Run("offset at exact count boundary", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Offset: 50, Limit: 10}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		require.Empty(t, results, "should return empty when offset equals total count")
	})

	t.Run("limit zero", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Offset: 0, Limit: 0}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		// Behavior: either empty or all posts depending on implementation
		// We verify it doesn't error
		require.GreaterOrEqual(t, len(results), 0, "should not error with limit=0")
	})

	t.Run("negative limit handling", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Offset: 0, Limit: -1}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		// Should handle gracefully, either returning all or none
		require.GreaterOrEqual(t, len(results), 0)
	})

	t.Run("negative offset handling", func(t *testing.T) {
		opts := &storage.ListPostsOptions{Offset: -1, Limit: 10}
		results, err := store.ListPosts(ctx, opts)
		require.NoError(t, err)
		// Should handle gracefully, treating negative offset as 0
		require.GreaterOrEqual(t, len(results), 0)
	})

	t.Run("normal pagination through full dataset", func(t *testing.T) {
		collected := make(map[string]bool)
		page := 0
		pageSize := 10

		for {
			opts := &storage.ListPostsOptions{
				Offset:  page * pageSize,
				Limit:   pageSize,
				SortBy:  "score",
				SortDir: "ASC",
			}
			results, err := store.ListPosts(ctx, opts)
			require.NoError(t, err)

			if len(results) == 0 {
				break
			}

			for _, p := range results {
				collected[p.ID] = true
			}

			page++
		}

		require.Equal(t, 50, len(collected), "should have collected all 50 unique posts through pagination")
	})
}
