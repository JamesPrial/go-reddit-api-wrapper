//go:build integration

package sqlite_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
)

// setupBenchmarkDB creates a fresh in-memory database for benchmarking.
// Helper function to reduce code duplication across benchmarks.
// NOTE: Run benchmarks with -benchmem to see allocation statistics.
func setupBenchmarkDB(b *testing.B) storage.Store {
	b.Helper()

	cfg := storage.Config{
		DSN: ":memory:",
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		b.Fatalf("failed to create benchmark database: %v", err)
	}

	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Logf("failed to close benchmark database: %v", err)
		}
	})

	return store
}

// BenchmarkUpsertPost measures the performance of upserting a single post.
// Tests the most common storage operation in isolation.
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertPost(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()
	post := testutil.BuildPost("bench1", "golang")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		post.ID = fmt.Sprintf("bench1_%d", i)
		post.Name = "t3_" + post.ID
		b.StartTimer()

		_ = store.UpsertPost(ctx, post)
	}
}

// BenchmarkUpsertPosts_Batch10 measures batch upsert performance with 10 posts.
// Evaluates throughput improvement from batch operations.
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertPosts_Batch10(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		posts := make([]*types.Post, 10)
		for j := 0; j < 10; j++ {
			posts[j] = testutil.BuildPost(fmt.Sprintf("batch10_%d_%d", i, j), "golang")
		}
		b.StartTimer()

		_ = store.UpsertPosts(ctx, posts)
	}
}

// BenchmarkUpsertPosts_Batch100 measures batch upsert performance with 100 posts.
// Evaluates batch performance at moderate scale.
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertPosts_Batch100(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		posts := make([]*types.Post, 100)
		for j := 0; j < 100; j++ {
			posts[j] = testutil.BuildPost(fmt.Sprintf("batch100_%d_%d", i, j), "golang")
		}
		b.StartTimer()

		_ = store.UpsertPosts(ctx, posts)
	}
}

// BenchmarkUpsertPosts_Batch1000 measures batch upsert performance with 1000 posts.
// Evaluates batch performance at large scale.
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertPosts_Batch1000(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		posts := make([]*types.Post, 1000)
		for j := 0; j < 1000; j++ {
			posts[j] = testutil.BuildPost(fmt.Sprintf("batch1k_%d_%d", i, j), "golang")
		}
		b.StartTimer()

		_ = store.UpsertPosts(ctx, posts)
	}
}

// BenchmarkGetPost measures the performance of retrieving a single post by ID.
// Tests read performance in the most common access pattern.
// Setup: Insert one post once before the loop to measure retrieval only.
// NOTE: Run with -benchmem to see allocations
func BenchmarkGetPost(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post once
	post := testutil.BuildPost("getpost", "golang")
	_ = store.UpsertPost(ctx, post)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = store.GetPost(ctx, "getpost")
	}
}

// BenchmarkListPosts_NoFilter measures listing all posts without any filters.
// Baseline for list operation performance.
// Setup: Insert 100 posts before the benchmark loop.
// NOTE: Run with -benchmem to see allocations
func BenchmarkListPosts_NoFilter(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert 100 posts
	posts := testutil.BuildPostBatch(100, "golang")
	_ = store.UpsertPosts(ctx, posts)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = store.ListPosts(ctx, &storage.ListPostsOptions{})
	}
}

// BenchmarkListPosts_WithSubredditFilter measures listing posts filtered by subreddit.
// Tests filter performance with targeted queries.
// Setup: Insert 100 posts across 10 different subreddits.
// NOTE: Run with -benchmem to see allocations
func BenchmarkListPosts_WithSubredditFilter(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert 100 posts across 10 subreddits (10 posts each)
	for sr := 0; sr < 10; sr++ {
		subreddit := fmt.Sprintf("subreddit%d", sr)
		posts := testutil.BuildPostBatch(10, subreddit)
		_ = store.UpsertPosts(ctx, posts)
	}

	b.ResetTimer()

	// Query for posts in subreddit5 to test filter performance
	opts := &storage.ListPostsOptions{
		Subreddit: "subreddit5",
	}

	for i := 0; i < b.N; i++ {
		_, _ = store.ListPosts(ctx, opts)
	}
}

// BenchmarkListPosts_WithPagination measures listing posts with limit and offset.
// Tests pagination performance on large result sets.
// Setup: Insert 1000 posts before the benchmark loop.
// NOTE: Run with -benchmem to see allocations
func BenchmarkListPosts_WithPagination(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert 1000 posts
	batchSize := 100
	for batch := 0; batch < 10; batch++ {
		offset := batch * batchSize
		posts := make([]*types.Post, batchSize)
		for j := 0; j < batchSize; j++ {
			posts[j] = testutil.BuildPost(fmt.Sprintf("pagination_%d", offset+j), "golang")
		}
		_ = store.UpsertPosts(ctx, posts)
	}

	b.ResetTimer()

	// Query with pagination
	opts := &storage.ListPostsOptions{
		Limit:  20,
		Offset: 50,
	}

	for i := 0; i < b.N; i++ {
		_, _ = store.ListPosts(ctx, opts)
	}
}

// BenchmarkUpsertComment measures the performance of upserting a single comment.
// Tests the basic comment storage operation.
// Setup: Insert one post before the benchmark loop (required for foreign key).
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertComment(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post (required for foreign key)
	post := testutil.BuildPost("commentpost", "golang")
	_ = store.UpsertPost(ctx, post)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		comment := testutil.BuildComment(fmt.Sprintf("comment_%d", i), "commentpost", "", 0)
		b.StartTimer()

		_ = store.UpsertComment(ctx, comment)
	}
}

// BenchmarkUpsertComments_Batch10 measures batch comment upsert performance with 10 comments.
// Evaluates batch throughput improvement for comments.
// Setup: Insert post first (required for foreign key).
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertComments_Batch10(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post
	post := testutil.BuildPost("batchcommentpost10", "golang")
	_ = store.UpsertPost(ctx, post)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		comments := make([]*types.Comment, 10)
		for j := 0; j < 10; j++ {
			comments[j] = testutil.BuildComment(
				fmt.Sprintf("batchcomment10_%d_%d", i, j),
				"batchcommentpost10",
				"",
				0,
			)
		}
		b.StartTimer()

		_ = store.UpsertComments(ctx, comments)
	}
}

// BenchmarkUpsertComments_Batch100 measures batch comment upsert with 100 comments.
// Tests comment hierarchy and batch performance at moderate scale.
// Setup: Insert post and creates a hierarchical comment structure.
// NOTE: Run with -benchmem to see allocations
func BenchmarkUpsertComments_Batch100(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post
	post := testutil.BuildPost("batchcommentpost100", "golang")
	_ = store.UpsertPost(ctx, post)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Build hierarchy: 10 top-level comments with 9 child comments each
		comments := make([]*types.Comment, 100)
		commentIdx := 0
		for topLevel := 0; topLevel < 10; topLevel++ {
			parentID := fmt.Sprintf("parent_%d_%d", i, topLevel)
			comments[commentIdx] = testutil.BuildComment(
				parentID,
				"batchcommentpost100",
				"",
				0,
			)
			commentIdx++

			for child := 0; child < 9; child++ {
				comments[commentIdx] = testutil.BuildComment(
					fmt.Sprintf("child_%d_%d_%d", i, topLevel, child),
					"batchcommentpost100",
					parentID,
					1,
				)
				commentIdx++
			}
		}
		b.StartTimer()

		_ = store.UpsertComments(ctx, comments)
	}
}

// BenchmarkGetCommentTree_Depth1 measures retrieving comment tree with MaxDepth=1.
// Tests shallow tree retrieval performance.
// Setup: Insert post with a comment tree (depth=5, breadth=5).
// NOTE: Run with -benchmem to see allocations
func BenchmarkGetCommentTree_Depth1(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post
	post := testutil.BuildPost("treepost1", "golang")
	_ = store.UpsertPost(ctx, post)

	// Insert comment tree: 5 levels deep, 5 children per level
	commentTree := testutil.BuildCommentTree("treepost1", 5, 5)
	_ = store.UpsertComments(ctx, commentTree)

	b.ResetTimer()

	// Query with MaxDepth=1 (only top-level comments)
	opts := &storage.CommentTreeOptions{
		MaxDepth: 1,
	}

	for i := 0; i < b.N; i++ {
		_, _ = store.GetCommentTree(ctx, "treepost1", opts)
	}
}

// BenchmarkGetCommentTree_Depth5 measures retrieving comment tree with MaxDepth=5.
// Tests moderate depth tree retrieval performance.
// Setup: Insert post with a comment tree (depth=5, breadth=5).
// NOTE: Run with -benchmem to see allocations
func BenchmarkGetCommentTree_Depth5(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post
	post := testutil.BuildPost("treepost5", "golang")
	_ = store.UpsertPost(ctx, post)

	// Insert comment tree: 5 levels deep, 5 children per level
	commentTree := testutil.BuildCommentTree("treepost5", 5, 5)
	_ = store.UpsertComments(ctx, commentTree)

	b.ResetTimer()

	// Query with MaxDepth=5
	opts := &storage.CommentTreeOptions{
		MaxDepth: 5,
	}

	for i := 0; i < b.N; i++ {
		_, _ = store.GetCommentTree(ctx, "treepost5", opts)
	}
}

// BenchmarkGetCommentTree_Unlimited measures retrieving entire comment tree (MaxDepth=0).
// Tests maximum depth tree retrieval performance.
// Setup: Insert post with a comment tree (depth=5, breadth=5).
// NOTE: Run with -benchmem to see allocations
func BenchmarkGetCommentTree_Unlimited(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert post
	post := testutil.BuildPost("treepostunlimited", "golang")
	_ = store.UpsertPost(ctx, post)

	// Insert comment tree: 5 levels deep, 5 children per level
	commentTree := testutil.BuildCommentTree("treepostunlimited", 5, 5)
	_ = store.UpsertComments(ctx, commentTree)

	b.ResetTimer()

	// Query with MaxDepth=0 (unlimited)
	opts := &storage.CommentTreeOptions{
		MaxDepth: 0,
	}

	for i := 0; i < b.N; i++ {
		_, _ = store.GetCommentTree(ctx, "treepostunlimited", opts)
	}
}

// BenchmarkEvictStale_100Posts measures eviction performance on 100 posts.
// Tests cleanup operation performance at small scale.
// Setup: Insert 100 posts with varied creation times.
// NOTE: Run with -benchmem to see allocations
func BenchmarkEvictStale_100Posts(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert 100 posts with varied ages
	oldTime := time.Now().Add(-72 * time.Hour) // 3 days old
	posts := make([]*types.Post, 100)
	for i := 0; i < 100; i++ {
		posts[i] = testutil.BuildPost(
			fmt.Sprintf("evict100_%d", i),
			"golang",
			testutil.WithCreatedAt(oldTime),
		)
	}
	_ = store.UpsertPosts(ctx, posts)

	b.ResetTimer()

	// Evict posts older than 48 hours
	maxAge := 48 * time.Hour

	for i := 0; i < b.N; i++ {
		_, _ = store.EvictStale(ctx, maxAge)
	}
}

// BenchmarkEvictStale_1000Posts measures eviction performance on 1000 posts.
// Tests cleanup operation performance at moderate scale.
// Setup: Insert 1000 posts with varied creation times.
// NOTE: Run with -benchmem to see allocations
func BenchmarkEvictStale_1000Posts(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert 1000 posts with varied ages
	oldTime := time.Now().Add(-72 * time.Hour) // 3 days old
	posts := make([]*types.Post, 1000)
	for i := 0; i < 1000; i++ {
		posts[i] = testutil.BuildPost(
			fmt.Sprintf("evict1k_%d", i),
			"golang",
			testutil.WithCreatedAt(oldTime),
		)
	}
	_ = store.UpsertPosts(ctx, posts)

	b.ResetTimer()

	// Evict posts older than 48 hours
	maxAge := 48 * time.Hour

	for i := 0; i < b.N; i++ {
		_, _ = store.EvictStale(ctx, maxAge)
	}
}

// BenchmarkEvictStale_10000Posts measures eviction performance on 10000 posts.
// Tests cleanup operation performance at large scale.
// Setup: Insert 10000 posts with varied creation times.
// NOTE: Run with -benchmem to see allocations
func BenchmarkEvictStale_10000Posts(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert 10000 posts with varied ages
	oldTime := time.Now().Add(-72 * time.Hour) // 3 days old
	batchSize := 1000
	for batch := 0; batch < 10; batch++ {
		posts := make([]*types.Post, batchSize)
		offset := batch * batchSize
		for i := 0; i < batchSize; i++ {
			posts[i] = testutil.BuildPost(
				fmt.Sprintf("evict10k_%d", offset+i),
				"golang",
				testutil.WithCreatedAt(oldTime),
			)
		}
		_ = store.UpsertPosts(ctx, posts)
	}

	b.ResetTimer()

	// Evict posts older than 48 hours
	maxAge := 48 * time.Hour

	for i := 0; i < b.N; i++ {
		_, _ = store.EvictStale(ctx, maxAge)
	}
}

// BenchmarkGetStats measures the performance of retrieving storage statistics.
// Tests metadata collection performance.
// Setup: Insert posts and comments before the benchmark loop.
// NOTE: Run with -benchmem to see allocations
func BenchmarkGetStats(b *testing.B) {
	store := setupBenchmarkDB(b)
	ctx := context.Background()

	// Setup: Insert posts and comments
	posts := testutil.BuildPostBatch(50, "golang")
	_ = store.UpsertPosts(ctx, posts)

	// Insert comments for the first post
	commentTree := testutil.BuildCommentTree("id0", 3, 5)
	_ = store.UpsertComments(ctx, commentTree)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = store.GetStats(ctx)
	}
}
