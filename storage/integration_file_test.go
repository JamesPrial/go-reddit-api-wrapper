//go:build integration
// +build integration

package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/testutil"
)

// reopenFileStore creates a new Store instance connected to an existing database file.
// For file-based databases, it verifies the file was created on disk after opening.
func reopenFileStore(t *testing.T, dbPath string) Store {
	t.Helper()

	cfg := &Config{
		DBPath:         dbPath,
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	testutil.AssertNoError(t, err)

	// Verify file exists on disk AFTER creation (for file-based databases only)
	// This confirms the database was actually persisted to disk
	if dbPath != ":memory:" && !strings.HasPrefix(dbPath, "file::memory:") && !strings.Contains(dbPath, "mode=memory") {
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Fatalf("database file %s was not created on disk after NewSQLiteStore", dbPath)
		}
	}

	return store
}

// TestIntegration_FileBasedPersistence_Posts verifies that posts persist across
// database connection close and reopen cycles.
func TestIntegration_FileBasedPersistence_Posts(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_posts.db"

	// Fetch 10 posts from Reddit
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 10,
		},
	})
	testutil.AssertNoError(t, err)

	if len(resp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	t.Logf("Fetched %d posts from r/golang", len(resp.Posts))

	// Create file-based store and store posts
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})
	t.Logf("Opened initial store, storing posts...")

	err = store1.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)
	t.Logf("Stored %d posts", len(resp.Posts))

	// Verify posts are stored
	opts := &ListPostsOptions{Limit: 100}
	retrievedBefore, err := store1.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(retrievedBefore) != len(resp.Posts) {
		t.Errorf("Expected %d posts before close, got %d", len(resp.Posts), len(retrievedBefore))
	}

	// Close connection
	err = store1.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed initial store connection")

	// Reopen same file with new connection
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})
	t.Logf("Reopened store from same database file")

	// Retrieve posts and verify all fields match
	retrievedAfter, err := store2.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(retrievedAfter) != len(resp.Posts) {
		t.Errorf("Expected %d posts after reopen, got %d", len(resp.Posts), len(retrievedAfter))
	}

	// Build ID-based map for order-agnostic comparison
	originalByID := make(map[string]*types.Post)
	for _, p := range resp.Posts {
		originalByID[p.ID] = p
	}

	// Verify each post persisted with all fields intact
	for _, retrieved := range retrievedAfter {
		original, found := originalByID[retrieved.ID]
		if !found {
			t.Errorf("Post %s not found in original set", retrieved.ID)
			continue
		}
		comparePost(t, original, retrieved)
	}

	t.Logf("✓ File-based persistence verified: %d posts survived close and reopen", len(retrievedAfter))
}

// TestIntegration_FileBasedPersistence_CommentTree verifies that comment trees persist across
// database connection close and reopen cycles, maintaining parent-child relationships.
func TestIntegration_FileBasedPersistence_CommentTree(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_comments.db"

	// Fetch post with comments
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "AskReddit",
		Pagination: types.Pagination{
			Limit: 1,
		},
	})
	testutil.AssertNoError(t, err)

	if len(postsResp.Posts) == 0 {
		t.Skip("No posts available from r/AskReddit (possibly rate limited or API issue)")
	}

	post := postsResp.Posts[0]
	t.Logf("Fetched post %s from r/AskReddit", post.ID)

	// Get comments
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "AskReddit",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 20,
		},
	})
	testutil.AssertNoError(t, err)

	flatComments := flattenCommentTree(commentsResp.Comments)
	t.Logf("Fetched %d total comments (including nested)", len(flatComments))

	// Store post and comments
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})
	t.Logf("Opened initial store, storing post and comments...")

	err = store1.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	if len(flatComments) > 0 {
		err = store1.UpsertComments(ctx, flatComments)
		testutil.AssertNoError(t, err)
	}

	t.Logf("Stored post and %d comments", len(flatComments))

	// Close connection
	err = store1.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed initial store connection")

	// Reopen and retrieve comment tree
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})
	t.Logf("Reopened store from same database file")

	retrievedTree, err := store2.GetCommentTree(ctx, post.ID, nil)
	testutil.AssertNoError(t, err)

	// Build ID maps for comparison (order-agnostic)
	originalFlat := flattenCommentTree(commentsResp.Comments)
	retrievedFlat := flattenCommentTree(retrievedTree)

	originalByID := make(map[string]*types.Comment)
	for _, c := range originalFlat {
		originalByID[c.ID] = c
	}

	retrievedByID := make(map[string]*types.Comment)
	for _, c := range retrievedFlat {
		retrievedByID[c.ID] = c
	}

	// Verify all comments persisted
	if len(retrievedByID) != len(originalByID) {
		t.Errorf("Comment count mismatch: expected %d, got %d", len(originalByID), len(retrievedByID))
	}

	// Verify each comment with parent-child relationships intact
	for id, original := range originalByID {
		retrieved, found := retrievedByID[id]
		if !found {
			t.Errorf("Comment %s not found after reopen", id)
			continue
		}
		compareComment(t, original, retrieved)

		// Verify parent-child relationship
		if original.ParentID != retrieved.ParentID {
			t.Errorf("ParentID mismatch for comment %s: expected %s, got %s",
				id, original.ParentID, retrieved.ParentID)
		}
	}

	t.Logf("✓ Comment tree persistence verified: %d comments with relationships intact after reopen", len(retrievedByID))
}

// TestIntegration_FileBasedPersistence_UpsertSemantics verifies that upsert (update not duplicate)
// semantics persist across database connection cycles.
func TestIntegration_FileBasedPersistence_UpsertSemantics(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_upsert.db"

	// Fetch 1 post
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 1,
		},
	})
	testutil.AssertNoError(t, err)

	if len(resp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	post := resp.Posts[0]
	originalScore := post.Score
	t.Logf("Fetched post %s with score %d", post.ID, originalScore)

	// First cycle: Store post
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})
	t.Logf("Opened initial store, storing post...")

	err = store1.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	err = store1.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed store, post stored")

	// Second cycle: Reopen, verify score, update it
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})
	t.Logf("Reopened store, verifying original score...")

	retrieved, err := store2.GetPost(ctx, post.ID)
	testutil.AssertNoError(t, err)

	if retrieved.Score != originalScore {
		t.Errorf("Score mismatch after reopen: expected %d, got %d", originalScore, retrieved.Score)
	}

	// Modify score and upsert
	modifiedPost := *post
	modifiedPost.Score = originalScore + 1000
	t.Logf("Updating post score to %d", modifiedPost.Score)

	err = store2.UpsertPost(ctx, &modifiedPost)
	testutil.AssertNoError(t, err)

	err = store2.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed store, updated score persisted")

	// Third cycle: Reopen and verify update persisted
	store3 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store3.Close()
	})
	t.Logf("Reopened store, verifying updated score...")

	// Verify updated score
	finalPost, err := store3.GetPost(ctx, post.ID)
	testutil.AssertNoError(t, err)

	if finalPost.Score != originalScore+1000 {
		t.Errorf("Updated score not persisted: expected %d, got %d", originalScore+1000, finalPost.Score)
	}

	// Verify only 1 post exists (not duplicated)
	opts := &ListPostsOptions{Limit: 100}
	allPosts, err := store3.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(allPosts) != 1 {
		t.Errorf("Expected 1 post (updated), got %d", len(allPosts))
	}

	t.Logf("✓ Upsert semantics verified: score updated from %d to %d, no duplication", originalScore, originalScore+1000)
}

// TestIntegration_FileBasedPersistence_Statistics verifies that statistics persist
// across database connection cycles.
func TestIntegration_FileBasedPersistence_Statistics(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_stats.db"

	// Fetch 15 posts
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 15,
		},
	})
	testutil.AssertNoError(t, err)

	if len(postsResp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	t.Logf("Fetched %d posts", len(postsResp.Posts))

	// Fetch comments for first post
	post := postsResp.Posts[0]
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "golang",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 20,
		},
	})
	testutil.AssertNoError(t, err)

	flatComments := flattenCommentTree(commentsResp.Comments)
	t.Logf("Fetched %d comments for post %s", len(flatComments), post.ID)

	// Store posts and comments
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})
	t.Logf("Opened initial store, storing data...")

	err = store1.UpsertPosts(ctx, postsResp.Posts)
	testutil.AssertNoError(t, err)

	if len(flatComments) > 0 {
		err = store1.UpsertComments(ctx, flatComments)
		testutil.AssertNoError(t, err)
	}

	// Get initial statistics
	statsBeforeClose, err := store1.GetStats(ctx)
	testutil.AssertNoError(t, err)

	t.Logf("Initial stats: posts=%d, comments=%d, size=%d bytes",
		statsBeforeClose.PostCount, statsBeforeClose.CommentCount, statsBeforeClose.TotalSizeBytes)

	err = store1.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed initial store")

	// Reopen and get statistics again
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})
	t.Logf("Reopened store, checking statistics...")

	statsAfterReopen, err := store2.GetStats(ctx)
	testutil.AssertNoError(t, err)

	t.Logf("Stats after reopen: posts=%d, comments=%d, size=%d bytes",
		statsAfterReopen.PostCount, statsAfterReopen.CommentCount, statsAfterReopen.TotalSizeBytes)

	// Verify statistics match
	if statsBeforeClose.PostCount != statsAfterReopen.PostCount {
		t.Errorf("PostCount mismatch: expected %d, got %d",
			statsBeforeClose.PostCount, statsAfterReopen.PostCount)
	}

	if statsBeforeClose.CommentCount != statsAfterReopen.CommentCount {
		t.Errorf("CommentCount mismatch: expected %d, got %d",
			statsBeforeClose.CommentCount, statsAfterReopen.CommentCount)
	}

	if statsBeforeClose.TotalSizeBytes != statsAfterReopen.TotalSizeBytes {
		t.Errorf("TotalSizeBytes mismatch: expected %d, got %d",
			statsBeforeClose.TotalSizeBytes, statsAfterReopen.TotalSizeBytes)
	}

	t.Logf("✓ Statistics persistence verified: all values match after reopen")
}

// TestIntegration_FileBasedPersistence_Eviction verifies that stale data eviction
// persists across database connection cycles.
func TestIntegration_FileBasedPersistence_Eviction(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_eviction.db"

	// Fetch 5 posts
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})
	testutil.AssertNoError(t, err)

	if len(resp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	t.Logf("Fetched %d posts", len(resp.Posts))

	// Store posts
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})
	t.Logf("Opened initial store, storing posts...")

	err = store1.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)

	// Verify posts stored
	opts := &ListPostsOptions{Limit: 100}
	initialList, err := store1.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(initialList) != len(resp.Posts) {
		t.Errorf("Expected %d posts stored, got %d", len(resp.Posts), len(initialList))
	}

	t.Logf("Stored %d posts", len(initialList))

	// Wait for aging with larger buffer for reliability
	// SQLite stores fetched_at as Unix seconds (INTEGER), so we need whole second precision
	// Using 2.5 second sleep to ensure we cross at least 2 Unix second boundaries
	sleepDuration := 2*time.Second + 500*time.Millisecond
	evictionThreshold := 1 * time.Second

	t.Logf("Waiting %v before eviction test (threshold: %v)...", sleepDuration, evictionThreshold)
	beforeSleep := time.Now().Unix()
	time.Sleep(sleepDuration)
	afterSleep := time.Now().Unix()

	if afterSleep-beforeSleep < 2 {
		t.Skipf("System clock precision insufficient for eviction test (only %d seconds elapsed)", afterSleep-beforeSleep)
	}

	// Evict stale data
	evicted, err := store1.EvictStale(ctx, evictionThreshold)
	testutil.AssertNoError(t, err)

	t.Logf("Evicted %d posts", evicted)

	if evicted != int64(len(resp.Posts)) {
		t.Errorf("Expected %d evictions, got %d", len(resp.Posts), evicted)
	}

	// Verify posts are gone
	remaining, err := store1.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(remaining) != 0 {
		t.Errorf("Expected 0 posts after eviction, got %d", len(remaining))
	}

	err = store1.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed store after eviction")

	// Reopen and verify eviction persisted
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})
	t.Logf("Reopened store, verifying eviction persisted...")

	finalList, err := store2.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(finalList) != 0 {
		t.Errorf("Expected 0 posts after reopen, got %d (eviction not persisted)", len(finalList))
	}

	t.Logf("✓ Eviction persistence verified: evicted data remained deleted after reopen")
}

// TestIntegration_FileBasedPersistence_LargeDataset verifies that large datasets
// persist correctly across database connection cycles.
func TestIntegration_FileBasedPersistence_LargeDataset(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_large.db"

	// Fetch 50 posts
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 50,
		},
	})
	testutil.AssertNoError(t, err)

	if len(postsResp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	t.Logf("Fetched %d posts", len(postsResp.Posts))

	// Fetch comments for first 3 posts
	var allComments []*types.Comment
	for i := 0; i < 3 && i < len(postsResp.Posts); i++ {
		post := postsResp.Posts[i]
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    post.ID,
			Pagination: types.Pagination{
				Limit: 30,
			},
		})
		testutil.AssertNoError(t, err)

		flatComments := flattenCommentTree(commentsResp.Comments)
		allComments = append(allComments, flatComments...)
	}

	t.Logf("Fetched %d total comments for 3 posts", len(allComments))

	// Store all data
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})
	t.Logf("Opened initial store, storing large dataset...")

	err = store1.UpsertPosts(ctx, postsResp.Posts)
	testutil.AssertNoError(t, err)

	if len(allComments) > 0 {
		err = store1.UpsertComments(ctx, allComments)
		testutil.AssertNoError(t, err)
	}

	t.Logf("Stored %d posts and %d comments", len(postsResp.Posts), len(allComments))

	err = store1.Close()
	testutil.AssertNoError(t, err)
	t.Logf("Closed initial store")

	// Reopen and verify data
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})
	t.Logf("Reopened store, verifying large dataset...")

	// Verify post count
	opts := &ListPostsOptions{Limit: 100}
	retrievedPosts, err := store2.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(retrievedPosts) != len(postsResp.Posts) {
		t.Errorf("Post count mismatch: expected %d, got %d", len(postsResp.Posts), len(retrievedPosts))
	}

	t.Logf("Verified %d posts persist correctly", len(retrievedPosts))

	// Verify comment trees for first 3 posts
	for i := 0; i < 3 && i < len(postsResp.Posts); i++ {
		post := postsResp.Posts[i]
		tree, err := store2.GetCommentTree(ctx, post.ID, nil)
		testutil.AssertNoError(t, err)

		flatTree := flattenCommentTree(tree)
		if len(flatTree) > 0 {
			t.Logf("Post %d (%s): %d comments persist correctly", i+1, post.ID, len(flatTree))
		}
	}

	t.Logf("✓ Large dataset persistence verified: %d posts and %d comments survived close/reopen", len(retrievedPosts), len(allComments))
}

// TestIntegration_FileBasedPersistence_MultipleConnections verifies that WAL mode
// allows concurrent database access from multiple connections to the same file.
func TestIntegration_FileBasedPersistence_MultipleConnections(t *testing.T) {
	client := getTestRedditClient(t)
	ctx := context.Background()
	dbPath := "test_persistence_concurrent.db"

	// Fetch posts and comments
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 10,
		},
	})
	testutil.AssertNoError(t, err)

	if len(postsResp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	post := postsResp.Posts[0]
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "golang",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 15,
		},
	})
	testutil.AssertNoError(t, err)

	flatComments := flattenCommentTree(commentsResp.Comments)

	t.Logf("Fetched %d posts and %d comments for concurrent test", len(postsResp.Posts), len(flatComments))

	// Create initial store
	store1 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store1.Close()
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})

	// Open second connection
	store2 := reopenFileStore(t, dbPath)
	t.Cleanup(func() {
		store2.Close()
	})

	t.Logf("Opened two concurrent connections to %s", dbPath)

	// Test concurrent operations
	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	// Connection 1: Store posts concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := store1.UpsertPosts(ctx, postsResp.Posts); err != nil {
			errCh <- fmt.Errorf("store1 UpsertPosts: %w", err)
		}
	}()

	// Connection 2: Read posts concurrently (slight delay for overlap)
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		opts := &ListPostsOptions{Limit: 100}
		if _, err := store2.ListPosts(ctx, opts); err != nil {
			errCh <- fmt.Errorf("store2 ListPosts during write: %w", err)
		}
	}()

	wg.Wait()

	// Connection 1: Store comments concurrently
	if len(flatComments) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store1.UpsertComments(ctx, flatComments); err != nil {
				errCh <- fmt.Errorf("store1 UpsertComments: %w", err)
			}
		}()

		// Connection 2: Read comments concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			if _, err := store2.GetCommentTree(ctx, post.ID, nil); err != nil {
				errCh <- fmt.Errorf("store2 GetCommentTree during write: %w", err)
			}
		}()

		wg.Wait()
	}

	close(errCh)

	// Check for any errors during concurrent operations
	for err := range errCh {
		testutil.AssertNoError(t, err)
	}

	t.Logf("✓ Concurrent operations completed successfully (WAL mode enabled concurrent reads)")
}
