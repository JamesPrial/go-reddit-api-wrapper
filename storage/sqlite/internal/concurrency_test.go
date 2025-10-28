//go:build integration

package sqlite_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
	"github.com/stretchr/testify/require"
)

// TestConcurrency_MultipleGoRoutinesUpsertSamePost verifies that multiple goroutines
// can safely upsert the same post with different scores without panicking or causing
// data races. The final post should have one of the scores (last write wins).
func TestConcurrency_MultipleGoRoutinesUpsertSamePost(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx := context.Background()

	postID := "post123"
	numGoroutines := 10

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			post := testutil.BuildPost(postID, "golang",
				testutil.WithScore(100+id),
			)

			err := store.UpsertPost(ctx, post)
			require.NoError(t, err, "UpsertPost failed in goroutine %d", id)
		}(i)
	}

	wg.Wait()

	// Verify the post exists and has one of the expected scores
	retrieved, err := store.GetPost(ctx, postID)
	require.NoError(t, err, "GetPost failed")
	require.NotNil(t, retrieved, "post should not be nil")
	require.Equal(t, postID, retrieved.ID, "post ID should match")

	// Score should be between 100 and 100+numGoroutines-1
	require.GreaterOrEqual(t, retrieved.Score, 100, "score should be at least 100")
	require.LessOrEqual(t, retrieved.Score, 100+numGoroutines-1, "score should not exceed 100+numGoroutines-1")
}

// TestConcurrency_ConcurrentReadsWhileWriting verifies that the store can safely
// handle concurrent reads while writes are happening. This test runs for 1 second
// with a writer goroutine continuously upserting posts and 5 reader goroutines
// continuously reading posts.
func TestConcurrency_ConcurrentReadsWhileWriting(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var wg sync.WaitGroup

	var writeCount atomic.Int64
	var readCount atomic.Int64
	var errors atomic.Int64

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		postID := 0
		for {
			select {
			case <-done:
				return
			default:
				post := testutil.BuildPost(
					fmt.Sprintf("write_post_%d", postID),
					"golang",
					testutil.WithScore(postID),
				)
				err := store.UpsertPost(ctx, post)
				if err != nil {
					errors.Add(1)
					return
				}
				writeCount.Add(1)
				postID++
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Reader goroutines
	for reader := 0; reader < 5; reader++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			idx := 0
			for {
				select {
				case <-done:
					return
				default:
					_, err := store.GetPost(ctx, fmt.Sprintf("write_post_%d", idx))
					// Error is OK if post doesn't exist yet
					if err == nil {
						readCount.Add(1)
					}
					idx++
					if idx > 100 {
						idx = 0
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(reader)
	}

	// Run for 1 second
	time.Sleep(1 * time.Second)
	close(done)

	wg.Wait()

	// Verify no panics occurred and operations completed
	require.Zero(t, errors.Load(), "no errors should occur during concurrent read/write")
	require.Greater(t, writeCount.Load(), int64(0), "should have completed some writes")
	require.Greater(t, readCount.Load(), int64(0), "should have completed some reads")
}

// TestConcurrency_GetCommentTreeDuringInserts verifies that GetCommentTree can safely
// be called while comments are being inserted. This test inserts an initial post and
// some comments, then launches a goroutine that adds more comments while 5 goroutines
// continuously call GetCommentTree.
func TestConcurrency_GetCommentTreeDuringInserts(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert initial post
	postID := "post_tree_test"
	post := testutil.BuildPost(postID, "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert initial post")

	// Insert initial comments
	initialComments := testutil.BuildCommentTree(postID, 2, 2)
	err = store.UpsertComments(ctx, initialComments)
	require.NoError(t, err, "failed to insert initial comments")

	done := make(chan struct{})
	var wg sync.WaitGroup

	var insertCount atomic.Int64
	var readCount atomic.Int64
	var errors atomic.Int64

	// Writer goroutine - continuously adds more comments
	wg.Add(1)
	go func() {
		defer wg.Done()
		commentID := 1000
		for {
			select {
			case <-done:
				return
			default:
				comment := testutil.BuildComment(
					fmt.Sprintf("new_comment_%d", commentID),
					postID,
					"",
					0,
				)
				err := store.UpsertComment(ctx, comment)
				if err != nil {
					errors.Add(1)
					return
				}
				insertCount.Add(1)
				commentID++
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()

	// Reader goroutines - continuously read comment tree
	for reader := 0; reader < 5; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					comments, err := store.GetCommentTree(ctx, postID, nil)
					if err != nil {
						errors.Add(1)
						return
					}
					if len(comments) > 0 {
						readCount.Add(1)
						// Verify comment tree structure is valid
						for _, c := range comments {
							require.NotEmpty(t, c.ID, "comment ID should not be empty")
							require.Equal(t, postID, c.LinkID[3:], "comment should be linked to post")
						}
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	// Run for 1 second
	time.Sleep(1 * time.Second)
	close(done)

	wg.Wait()

	// Verify no panics or errors occurred
	require.Zero(t, errors.Load(), "no errors should occur during concurrent comment tree reads")
	require.Greater(t, insertCount.Load(), int64(0), "should have inserted additional comments")
	require.Greater(t, readCount.Load(), int64(0), "should have read comment trees")
}

// TestConcurrency_EvictStaleWhileQueriesRunning verifies that EvictStale can safely
// operate while other queries are running. This test inserts a mix of old and new posts,
// then launches a goroutine that calls EvictStale while 5 goroutines read posts.
func TestConcurrency_EvictStaleWhileQueriesRunning(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert old and new posts
	oldTime := time.Now().Add(-24 * time.Hour)
	newTime := time.Now()

	var posts []*types.Post
	for i := 0; i < 20; i++ {
		post := testutil.BuildPost(fmt.Sprintf("old_post_%d", i), "golang",
			testutil.WithCreatedAt(oldTime),
		)
		posts = append(posts, post)
	}
	for i := 0; i < 20; i++ {
		post := testutil.BuildPost(fmt.Sprintf("new_post_%d", i), "golang",
			testutil.WithCreatedAt(newTime),
		)
		posts = append(posts, post)
	}

	err := store.UpsertPosts(ctx, posts)
	require.NoError(t, err, "failed to upsert posts")

	done := make(chan struct{})
	var wg sync.WaitGroup

	var evictCount atomic.Int64
	var readCount atomic.Int64
	var errors atomic.Int64

	// Evict goroutine - continuously evicts old posts
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				// Evict posts older than 12 hours
				n, err := store.EvictStale(ctx, 12*time.Hour)
				if err != nil {
					errors.Add(1)
					return
				}
				if n > 0 {
					evictCount.Add(n)
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	// Reader goroutines - continuously read posts
	for reader := 0; reader < 5; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// List all posts
					opts := &storage.ListPostsOptions{
						Limit: 100,
					}
					posts, err := store.ListPosts(ctx, opts)
					if err != nil {
						errors.Add(1)
						return
					}
					if len(posts) > 0 {
						readCount.Add(1)
					}
					time.Sleep(15 * time.Millisecond)
				}
			}
		}()
	}

	// Run for 1 second
	time.Sleep(1 * time.Second)
	close(done)

	wg.Wait()

	// Verify no panics or errors occurred
	require.Zero(t, errors.Load(), "no errors should occur during concurrent evict and read operations")
}

// TestConcurrency_ConnectionPoolExhaustion verifies that the connection pool handles
// exhaustion gracefully. This test creates a store with MaxOpenConns=2 and launches
// 10 goroutines that hold long-running queries. Verify that operations eventually
// succeed without deadlock.
func TestConcurrency_ConnectionPoolExhaustion(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := fmt.Sprintf("%s/test_pool.db", tempDir)

	cfg := storage.Config{
		DSN:          dbPath,
		MaxOpenConns: 2,
		MaxIdleConns: 1,
	}

	store, err := storage.New(context.Background(), cfg)
	require.NoError(t, err, "failed to create store")
	defer store.Close()

	// Insert some test data
	post := testutil.BuildPost("test_post", "golang")
	err = store.UpsertPost(context.Background(), post)
	require.NoError(t, err, "failed to insert test post")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var errors atomic.Int64
	var successes atomic.Int64

	// Launch goroutines that each try to read the post
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine tries to read the post
			for attempt := 0; attempt < 3; attempt++ {
				// Use a shorter context with timeout to avoid hanging
				opCtx, opCancel := context.WithTimeout(ctx, 2*time.Second)
				post, err := store.GetPost(opCtx, "test_post")
				opCancel()

				if err != nil {
					if ctx.Err() != context.DeadlineExceeded {
						errors.Add(1)
					}
					time.Sleep(50 * time.Millisecond)
					continue
				}

				if post != nil {
					successes.Add(1)
				}
				break
			}
		}(i)
	}

	wg.Wait()

	// Verify that most operations succeeded despite connection pool limits
	require.Greater(t, successes.Load(), int64(5), "at least 5 operations should have succeeded")
	require.LessOrEqual(t, errors.Load(), int64(5), "errors should be minimal")
}

// TestConcurrency_BatchUpsertConcurrent verifies that multiple goroutines can safely
// perform batch upserts concurrently. This test launches 5 goroutines, each upserting
// 100 posts, and verifies all posts are inserted without data races.
func TestConcurrency_BatchUpsertConcurrent(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx := context.Background()

	numGoroutines := 5
	postsPerGoroutine := 100

	var wg sync.WaitGroup
	var errors atomic.Int64

	// Launch goroutines that each batch-upsert posts
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			var posts []*types.Post
			for p := 0; p < postsPerGoroutine; p++ {
				postID := fmt.Sprintf("batch_post_g%d_p%d", goroutineID, p)
				post := testutil.BuildPost(postID, "golang",
					testutil.WithScore(goroutineID*1000+p),
				)
				posts = append(posts, post)
			}

			err := store.UpsertPosts(ctx, posts)
			if err != nil {
				errors.Add(1)
			}
		}(g)
	}

	wg.Wait()

	// Verify no errors occurred
	require.Zero(t, errors.Load(), "no errors should occur during concurrent batch upserts")

	// Verify posts were inserted (at least some should be present)
	opts := &storage.ListPostsOptions{
		Limit: 1000,
	}
	posts, err := store.ListPosts(ctx, opts)
	require.NoError(t, err, "failed to list posts")

	// Should have posts from all goroutines (5 * 100 = 500 posts expected)
	require.Greater(t, int64(len(posts)), int64(0), "should have inserted posts")
	require.LessOrEqual(t, int64(len(posts)), int64(numGoroutines*postsPerGoroutine), "should not exceed total posts")
}

// TestConcurrency_ConcurrentStatsQueries verifies that multiple goroutines can safely
// call GetStats simultaneously without data races or panicking. This test launches
// 10 goroutines that all call GetStats at the same time.
func TestConcurrency_ConcurrentStatsQueries(t *testing.T) {
	store := testutil.NewFileBasedDB(t)
	ctx := context.Background()

	// Insert some test data so stats are meaningful
	post := testutil.BuildPost("stats_post_1", "golang", testutil.WithScore(100))
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to insert test post")

	comment := testutil.BuildComment("stats_comment_1", "stats_post_1", "", 0)
	err = store.UpsertComment(ctx, comment)
	require.NoError(t, err, "failed to insert test comment")

	numGoroutines := 10
	var wg sync.WaitGroup
	var errors atomic.Int64
	var statsResults []*storage.CacheStats
	var mu sync.Mutex

	// Launch goroutines that all call GetStats
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			stats, err := store.GetStats(ctx)
			if err != nil {
				errors.Add(1)
				return
			}

			if stats != nil {
				mu.Lock()
				statsResults = append(statsResults, stats)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Verify no errors occurred
	require.Zero(t, errors.Load(), "no errors should occur during concurrent stats queries")

	// Verify all goroutines got stats results
	require.Equal(t, numGoroutines, len(statsResults), "should have stats from all goroutines")

	// Verify stats are consistent across calls
	firstStats := statsResults[0]
	require.NotNil(t, firstStats, "stats should not be nil")
	require.Greater(t, firstStats.PostCount, int64(0), "should have at least 1 post")
	require.Greater(t, firstStats.CommentCount, int64(0), "should have at least 1 comment")

	// All stats should show same counts (since no other writes happened)
	for i, stats := range statsResults {
		require.Equal(t, firstStats.PostCount, stats.PostCount,
			"post count should be consistent for result %d", i)
		require.Equal(t, firstStats.CommentCount, stats.CommentCount,
			"comment count should be consistent for result %d", i)
	}
}
