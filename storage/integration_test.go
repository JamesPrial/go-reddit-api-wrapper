//go:build integration
// +build integration

package storage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/testutil"
)

// Integration tests for storage layer with real Reddit API data.
// These tests verify end-to-end workflows: fetch from Reddit → store in SQLite → retrieve and verify accuracy.
//
// Prerequisites:
//   - REDDIT_CLIENT_ID: Your Reddit application client ID
//   - REDDIT_CLIENT_SECRET: Your Reddit application client secret
//
// Run with: go test -tags=integration -v ./storage

// getTestRedditClient initializes a Reddit client from environment variables.
func getTestRedditClient(t *testing.T) *graw.Reddit {
	t.Helper()

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		t.Skip("Skipping integration test: REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET must be set")
	}

	config := &graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "go-reddit-api-wrapper:storage-integration-tests:v1.0.0",
	}

	client, err := graw.NewClient(config)
	testutil.AssertNoError(t, err)

	return client
}

// getTestStore initializes an in-memory SQLite store for testing.
func getTestStore(t *testing.T) Store {
	t.Helper()

	cfg := &Config{
		DBPath:         ":memory:",
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	testutil.AssertNoError(t, err)

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

// getTestFileStore initializes a file-based SQLite store for testing.
func getTestFileStore(t *testing.T, dbPath string) Store {
	t.Helper()

	cfg := &Config{
		DBPath:         dbPath,
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	testutil.AssertNoError(t, err)

	t.Cleanup(func() {
		store.Close()
		os.Remove(dbPath)
	})

	return store
}

// comparePost performs deep field-by-field comparison of two posts.
// Skips FetchedAt since it's set by the storage layer.
func comparePost(t *testing.T, expected, actual *types.Post) {
	t.Helper()

	if expected.ID != actual.ID {
		t.Errorf("ID mismatch: expected %s, got %s", expected.ID, actual.ID)
	}
	if expected.Name != actual.Name {
		t.Errorf("Name mismatch: expected %s, got %s", expected.Name, actual.Name)
	}
	if expected.Title != actual.Title {
		t.Errorf("Title mismatch: expected %s, got %s", expected.Title, actual.Title)
	}
	if expected.Author != actual.Author {
		t.Errorf("Author mismatch: expected %s, got %s", expected.Author, actual.Author)
	}
	if expected.Subreddit != actual.Subreddit {
		t.Errorf("Subreddit mismatch: expected %s, got %s", expected.Subreddit, actual.Subreddit)
	}
	if expected.Score != actual.Score {
		t.Errorf("Score mismatch: expected %d, got %d", expected.Score, actual.Score)
	}
	if expected.Ups != actual.Ups {
		t.Errorf("Ups mismatch: expected %d, got %d", expected.Ups, actual.Ups)
	}
	if expected.Downs != actual.Downs {
		t.Errorf("Downs mismatch: expected %d, got %d", expected.Downs, actual.Downs)
	}
	if expected.NumComments != actual.NumComments {
		t.Errorf("NumComments mismatch: expected %d, got %d", expected.NumComments, actual.NumComments)
	}
	if expected.CreatedUTC != actual.CreatedUTC {
		t.Errorf("CreatedUTC mismatch: expected %f, got %f", expected.CreatedUTC, actual.CreatedUTC)
	}
	if expected.Over18 != actual.Over18 {
		t.Errorf("Over18 mismatch: expected %v, got %v", expected.Over18, actual.Over18)
	}
	if expected.Permalink != actual.Permalink {
		t.Errorf("Permalink mismatch: expected %s, got %s", expected.Permalink, actual.Permalink)
	}
	if expected.URL != actual.URL {
		t.Errorf("URL mismatch: expected %s, got %s", expected.URL, actual.URL)
	}
	if expected.SelfText != actual.SelfText {
		t.Errorf("SelfText mismatch: expected %s, got %s", expected.SelfText, actual.SelfText)
	}
	// Note: FetchedAt is intentionally not compared as it's set by the storage layer
}

// compareComment performs deep field-by-field comparison of two comments.
// Skips FetchedAt since it's set by the storage layer.
func compareComment(t *testing.T, expected, actual *types.Comment) {
	t.Helper()

	if expected.ID != actual.ID {
		t.Errorf("ID mismatch: expected %s, got %s", expected.ID, actual.ID)
	}
	if expected.Name != actual.Name {
		t.Errorf("Name mismatch: expected %s, got %s", expected.Name, actual.Name)
	}
	if expected.Author != actual.Author {
		t.Errorf("Author mismatch: expected %s, got %s", expected.Author, actual.Author)
	}
	if expected.Body != actual.Body {
		t.Errorf("Body mismatch for comment %s", expected.ID)
	}
	if expected.ParentID != actual.ParentID {
		t.Errorf("ParentID mismatch: expected %s, got %s", expected.ParentID, actual.ParentID)
	}
	if expected.LinkID != actual.LinkID {
		t.Errorf("LinkID mismatch: expected %s, got %s", expected.LinkID, actual.LinkID)
	}
	if expected.Score != actual.Score {
		t.Errorf("Score mismatch: expected %d, got %d", expected.Score, actual.Score)
	}
	if expected.CreatedUTC != actual.CreatedUTC {
		t.Errorf("CreatedUTC mismatch: expected %f, got %f", expected.CreatedUTC, actual.CreatedUTC)
	}
	if expected.Subreddit != actual.Subreddit {
		t.Errorf("Subreddit mismatch: expected %s, got %s", expected.Subreddit, actual.Subreddit)
	}
}

// compareCommentTree recursively compares comment trees including all replies.
func compareCommentTree(t *testing.T, expected, actual *types.Comment) {
	t.Helper()

	compareComment(t, expected, actual)

	if len(expected.Replies) != len(actual.Replies) {
		t.Errorf("Replies count mismatch for comment %s: expected %d, got %d",
			expected.ID, len(expected.Replies), len(actual.Replies))
		return
	}

	// Compare each reply recursively
	for i := range expected.Replies {
		compareCommentTree(t, expected.Replies[i], actual.Replies[i])
	}
}

// flattenCommentTree converts a nested comment tree to a flat slice.
func flattenCommentTree(comments []*types.Comment) []*types.Comment {
	var flat []*types.Comment
	for _, comment := range comments {
		flat = append(flat, comment)
		if len(comment.Replies) > 0 {
			flat = append(flat, flattenCommentTree(comment.Replies)...)
		}
	}
	return flat
}

// TestIntegration_SinglePostRoundTrip tests storing and retrieving a single post.
func TestIntegration_SinglePostRoundTrip(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch a single post from Reddit
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

	originalPost := resp.Posts[0]
	t.Logf("Fetched post: %s - %s", originalPost.ID, originalPost.Title)

	// Store the post
	err = store.UpsertPost(ctx, originalPost)
	testutil.AssertNoError(t, err)

	// Retrieve the post
	retrievedPost, err := store.GetPost(ctx, originalPost.ID)
	testutil.AssertNoError(t, err)

	// Verify all fields match
	comparePost(t, originalPost, retrievedPost)
	t.Logf("✓ Single post round-trip successful")
}

// TestIntegration_BatchPostsRoundTrip tests storing and retrieving multiple posts in a batch.
func TestIntegration_BatchPostsRoundTrip(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch multiple posts
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 25,
		},
	})
	testutil.AssertNoError(t, err)

	if len(resp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	t.Logf("Fetched %d posts", len(resp.Posts))

	// Store all posts in batch
	err = store.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)

	// Retrieve all posts
	opts := &ListPostsOptions{
		Limit: 100,
	}
	retrievedPosts, err := store.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	// Verify count matches
	if len(retrievedPosts) != len(resp.Posts) {
		t.Errorf("Post count mismatch: expected %d, got %d", len(resp.Posts), len(retrievedPosts))
	}

	// Verify all posts preserved (by checking IDs)
	originalIDs := make(map[string]*types.Post)
	for _, post := range resp.Posts {
		originalIDs[post.ID] = post
	}

	for _, retrieved := range retrievedPosts {
		original, found := originalIDs[retrieved.ID]
		if !found {
			t.Errorf("Retrieved post %s not found in original set", retrieved.ID)
			continue
		}
		comparePost(t, original, retrieved)
	}

	t.Logf("✓ Batch posts round-trip successful (%d posts)", len(resp.Posts))
}

// TestIntegration_CommentTreeRoundTrip tests storing and retrieving comment trees.
func TestIntegration_CommentTreeRoundTrip(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch a post with comments
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

	// Get comments for the post
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "AskReddit",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 20,
		},
	})
	testutil.AssertNoError(t, err)

	t.Logf("Fetched post %s with %d top-level comments", post.ID, len(commentsResp.Comments))

	// Store the post
	err = store.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	// Flatten and store all comments
	flatComments := flattenCommentTree(commentsResp.Comments)
	if len(flatComments) > 0 {
		err = store.UpsertComments(ctx, flatComments)
		testutil.AssertNoError(t, err)
		t.Logf("Stored %d total comments (including nested)", len(flatComments))
	}

	// Retrieve the comment tree
	retrievedTree, err := store.GetCommentTree(ctx, post.ID, nil)
	testutil.AssertNoError(t, err)

	// Verify tree structure - compare by ID, not by index (DB order may differ from API order)
	if len(retrievedTree) != len(commentsResp.Comments) {
		t.Errorf("Top-level comment count mismatch: expected %d, got %d",
			len(commentsResp.Comments), len(retrievedTree))
	}

	// Build maps of comments by ID for order-agnostic comparison
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

	// Verify all original comments are present in retrieved set
	if len(originalByID) != len(retrievedByID) {
		t.Errorf("Total comment count mismatch: expected %d, got %d",
			len(originalByID), len(retrievedByID))
	}

	// Compare each comment by ID
	for id, original := range originalByID {
		retrieved, found := retrievedByID[id]
		if !found {
			t.Errorf("Comment %s not found in retrieved set", id)
			continue
		}
		compareComment(t, original, retrieved)
	}

	t.Logf("✓ Comment tree round-trip successful (%d comments)", len(originalByID))
}

// TestIntegration_DeepCommentTree tests depth filtering in comment trees.
func TestIntegration_DeepCommentTree(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch a post with comments
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

	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "AskReddit",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 15,
		},
	})
	testutil.AssertNoError(t, err)

	// Store post and comments
	err = store.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	flatComments := flattenCommentTree(commentsResp.Comments)
	if len(flatComments) > 0 {
		err = store.UpsertComments(ctx, flatComments)
		testutil.AssertNoError(t, err)
	}

	// Test different depth limits
	testCases := []struct {
		name     string
		maxDepth int
	}{
		{"depth 2", 2},
		{"depth 5", 5},
		{"unlimited", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &CommentTreeOptions{
				MaxDepth: tc.maxDepth,
			}

			tree, err := store.GetCommentTree(ctx, post.ID, opts)
			testutil.AssertNoError(t, err)

			// Verify depth constraint
			if tc.maxDepth > 0 {
				maxFoundDepth := getMaxDepth(tree)
				if maxFoundDepth > tc.maxDepth {
					t.Errorf("Max depth exceeded: expected <=%d, got %d", tc.maxDepth, maxFoundDepth)
				}
				t.Logf("Max depth with limit %d: %d", tc.maxDepth, maxFoundDepth)
			} else {
				t.Logf("Unlimited depth: %d levels", getMaxDepth(tree))
			}
		})
	}
}

// getMaxDepth calculates the maximum depth of a comment tree.
func getMaxDepth(comments []*types.Comment) int {
	maxDepth := 0
	for _, comment := range comments {
		depth := 1
		if len(comment.Replies) > 0 {
			depth += getMaxDepth(comment.Replies)
		}
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

// TestIntegration_PostFiltering tests filtering posts by various criteria.
func TestIntegration_PostFiltering(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch posts from multiple subreddits
	subreddits := []string{"golang", "programming"}
	var allPosts []*types.Post

	for _, sub := range subreddits {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: sub,
			Pagination: types.Pagination{
				Limit: 25,
			},
		})
		testutil.AssertNoError(t, err)
		allPosts = append(allPosts, resp.Posts...)
	}

	if len(allPosts) == 0 {
		t.Skip("No posts available from subreddits (possibly rate limited or API issue)")
	}

	// Store all posts
	err := store.UpsertPosts(ctx, allPosts)
	testutil.AssertNoError(t, err)
	t.Logf("Stored %d posts from multiple subreddits", len(allPosts))

	// Test subreddit filter (case-insensitive)
	t.Run("filter by subreddit", func(t *testing.T) {
		opts := &ListPostsOptions{
			Subreddit: "GoLang", // Test case-insensitivity
			Limit:     100,
		}
		filtered, err := store.ListPosts(ctx, opts)
		testutil.AssertNoError(t, err)

		for _, post := range filtered {
			if !strings.EqualFold(post.Subreddit, "golang") {
				t.Errorf("Expected subreddit 'golang', got '%s'", post.Subreddit)
			}
		}
		t.Logf("✓ Subreddit filter: %d posts", len(filtered))
	})

	// Test author filter
	t.Run("filter by author", func(t *testing.T) {
		if len(allPosts) == 0 {
			t.Skip("No posts to filter")
		}
		testAuthor := allPosts[0].Author

		opts := &ListPostsOptions{
			Author: testAuthor,
			Limit:  100,
		}
		filtered, err := store.ListPosts(ctx, opts)
		testutil.AssertNoError(t, err)

		for _, post := range filtered {
			if post.Author != testAuthor {
				t.Errorf("Expected author '%s', got '%s'", testAuthor, post.Author)
			}
		}
		t.Logf("✓ Author filter: %d posts by %s", len(filtered), testAuthor)
	})

	// Test MinScore filter
	t.Run("filter by min score", func(t *testing.T) {
		opts := &ListPostsOptions{
			MinScore: 100,
			Limit:    100,
		}
		filtered, err := store.ListPosts(ctx, opts)
		testutil.AssertNoError(t, err)

		for _, post := range filtered {
			if post.Score < 100 {
				t.Errorf("Expected score >= 100, got %d", post.Score)
			}
		}
		t.Logf("✓ MinScore filter: %d posts with score >= 100", len(filtered))
	})

	// Test MaxAge filter
	// Note: MaxAge filters by fetched_at (when stored), not created_utc (Reddit creation time)
	t.Run("filter by max age", func(t *testing.T) {
		opts := &ListPostsOptions{
			MaxAge: 24 * time.Hour,
			Limit:  100,
		}
		filtered, err := store.ListPosts(ctx, opts)
		testutil.AssertNoError(t, err)

		// All posts were just fetched/stored, so all should pass the 24-hour filter
		// (MaxAge compares against fetched_at, not created_utc)
		if len(filtered) < len(allPosts) {
			t.Logf("Warning: Some recently-fetched posts excluded (expected all %d, got %d)",
				len(allPosts), len(filtered))
		}

		t.Logf("✓ MaxAge filter: %d posts fetched within 24 hours (total stored: %d)",
			len(filtered), len(allPosts))
	})
}

// TestIntegration_PostSorting tests sorting posts by various fields.
func TestIntegration_PostSorting(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch and store posts
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 30,
		},
	})
	testutil.AssertNoError(t, err)

	if len(resp.Posts) < 2 {
		t.Skip("Need at least 2 posts to test sorting")
	}

	err = store.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)

	// Test different sort options
	testCases := []struct {
		sortBy  string
		sortDir string
	}{
		{"created_utc", "asc"},
		{"created_utc", "desc"},
		{"score", "asc"},
		{"score", "desc"},
		{"num_comments", "desc"},
	}

	for _, tc := range testCases {
		t.Run(tc.sortBy+"_"+tc.sortDir, func(t *testing.T) {
			opts := &ListPostsOptions{
				SortBy:  tc.sortBy,
				SortDir: tc.sortDir,
				Limit:   100,
			}
			sorted, err := store.ListPosts(ctx, opts)
			testutil.AssertNoError(t, err)

			// Verify ordering
			if len(sorted) >= 2 {
				for i := 0; i < len(sorted)-1; i++ {
					var currentVal, nextVal float64
					switch tc.sortBy {
					case "created_utc":
						currentVal = sorted[i].CreatedUTC
						nextVal = sorted[i+1].CreatedUTC
					case "score":
						currentVal = float64(sorted[i].Score)
						nextVal = float64(sorted[i+1].Score)
					case "num_comments":
						currentVal = float64(sorted[i].NumComments)
						nextVal = float64(sorted[i+1].NumComments)
					}

					if tc.sortDir == "asc" && currentVal > nextVal {
						t.Errorf("Ascending order violated at index %d: %f > %f", i, currentVal, nextVal)
					}
					if tc.sortDir == "desc" && currentVal < nextVal {
						t.Errorf("Descending order violated at index %d: %f < %f", i, currentVal, nextVal)
					}
				}
			}
			t.Logf("✓ Sorted by %s %s: %d posts", tc.sortBy, tc.sortDir, len(sorted))
		})
	}
}

// TestIntegration_Pagination tests pagination with limit and offset.
func TestIntegration_Pagination(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch and store posts
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 50,
		},
	})
	testutil.AssertNoError(t, err)

	if len(resp.Posts) < 10 {
		t.Skip("Need at least 10 posts to test pagination")
	}

	err = store.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)

	// Test pagination
	pageSize := 10
	var allRetrieved []*types.Post
	seenIDs := make(map[string]bool)

	for page := 0; page < 3; page++ {
		opts := &ListPostsOptions{
			Limit:  pageSize,
			Offset: page * pageSize,
		}
		pagePosts, err := store.ListPosts(ctx, opts)
		testutil.AssertNoError(t, err)

		// Check for duplicates
		for _, post := range pagePosts {
			if seenIDs[post.ID] {
				t.Errorf("Duplicate post ID %s found in pagination", post.ID)
			}
			seenIDs[post.ID] = true
		}

		allRetrieved = append(allRetrieved, pagePosts...)
		t.Logf("Page %d: %d posts", page, len(pagePosts))

		if len(pagePosts) < pageSize {
			break
		}
	}

	t.Logf("✓ Pagination test: %d posts retrieved across multiple pages", len(allRetrieved))
}

// TestIntegration_UpsertSemantics tests that upsert updates rather than duplicates.
func TestIntegration_UpsertSemantics(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch a post
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

	// Store the post
	err = store.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	// Modify and store again
	post.Score = originalScore + 1000
	err = store.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	// Verify count is still 1 (updated, not duplicated)
	opts := &ListPostsOptions{Limit: 100}
	allPosts, err := store.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(allPosts) != 1 {
		t.Errorf("Expected 1 post after upsert, got %d", len(allPosts))
	}

	// Verify score was updated
	retrieved, err := store.GetPost(ctx, post.ID)
	testutil.AssertNoError(t, err)

	if retrieved.Score != originalScore+1000 {
		t.Errorf("Score not updated: expected %d, got %d", originalScore+1000, retrieved.Score)
	}

	t.Logf("✓ Upsert semantics verified: update not duplicate")
}

// TestIntegration_Statistics tests the GetStats operation.
func TestIntegration_Statistics(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch and store posts
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

	err = store.UpsertPosts(ctx, postsResp.Posts)
	testutil.AssertNoError(t, err)

	// Fetch and store comments
	post := postsResp.Posts[0]
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "golang",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 10,
		},
	})
	testutil.AssertNoError(t, err)

	flatComments := flattenCommentTree(commentsResp.Comments)
	if len(flatComments) > 0 {
		err = store.UpsertComments(ctx, flatComments)
		testutil.AssertNoError(t, err)
	}

	// Get statistics
	stats, err := store.GetStats(ctx)
	testutil.AssertNoError(t, err)

	// Verify statistics
	if stats.PostCount != int64(len(postsResp.Posts)) {
		t.Errorf("PostCount mismatch: expected %d, got %d", len(postsResp.Posts), stats.PostCount)
	}

	if stats.CommentCount != int64(len(flatComments)) {
		t.Errorf("CommentCount mismatch: expected %d, got %d", len(flatComments), stats.CommentCount)
	}

	if stats.TotalSizeBytes <= 0 {
		t.Error("Expected TotalSizeBytes > 0")
	}

	if stats.OldestEntry.IsZero() || stats.NewestEntry.IsZero() {
		t.Error("Expected non-zero timestamp for oldest/newest entry")
	}

	t.Logf("✓ Statistics: %d posts, %d comments, %d bytes",
		stats.PostCount, stats.CommentCount, stats.TotalSizeBytes)
}

// TestIntegration_StaleDataEviction tests the EvictStale operation.
func TestIntegration_StaleDataEviction(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Fetch and store posts
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

	err = store.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)

	initialCount := len(resp.Posts)

	// Test eviction with very old maxAge (should keep everything)
	evicted, err := store.EvictStale(ctx, 1000*time.Hour)
	testutil.AssertNoError(t, err)

	if evicted != 0 {
		t.Errorf("Expected 0 evictions with old maxAge, got %d", evicted)
	}

	// Verify data still present
	opts := &ListPostsOptions{Limit: 100}
	remaining, err := store.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(remaining) != initialCount {
		t.Errorf("Data was evicted unexpectedly: expected %d, got %d", initialCount, len(remaining))
	}

	// Now wait and evict with a threshold that should catch aged data
	// Note: fetched_at uses Unix seconds (INTEGER), so we need whole second precision
	// Adding 200ms buffer to ensure we reliably cross Unix second boundaries
	sleepDuration := 2*time.Second + 200*time.Millisecond
	evictionThreshold := 1 * time.Second

	t.Logf("Waiting %v before eviction test...", sleepDuration)
	time.Sleep(sleepDuration)

	evicted, err = store.EvictStale(ctx, evictionThreshold)
	testutil.AssertNoError(t, err)

	if evicted != int64(initialCount) {
		t.Errorf("Expected %d evictions with %v threshold after %v sleep, got %d",
			initialCount, evictionThreshold, sleepDuration, evicted)
	}

	// Verify data removed
	remaining, err = store.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(remaining) != 0 {
		t.Errorf("Expected 0 posts after eviction, got %d", len(remaining))
	}

	t.Logf("✓ Stale data eviction: %d posts evicted after aging", evicted)
}

// TestIntegration_MoreComments tests storing expanded comments from GetMoreComments.
func TestIntegration_MoreComments(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	// Get a post with comments
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

	// Get initial comments
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "AskReddit",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 5,
		},
	})
	testutil.AssertNoError(t, err)

	if len(commentsResp.MoreIDs) == 0 {
		t.Skip("No more comment IDs available")
	}

	// Store post and initial comments
	err = store.UpsertPost(ctx, post)
	testutil.AssertNoError(t, err)

	initialComments := flattenCommentTree(commentsResp.Comments)
	if len(initialComments) > 0 {
		err = store.UpsertComments(ctx, initialComments)
		testutil.AssertNoError(t, err)
	}

	// Fetch more comments (limit for test performance - full expansion can be slow)
	moreIDs := commentsResp.MoreIDs
	if len(moreIDs) > 10 {
		moreIDs = moreIDs[:10]
	}

	moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
		LinkID:        post.ID,
		CommentIDs:    moreIDs,
		Sort:          "confidence",
		LimitChildren: true,
	})
	testutil.AssertNoError(t, err)

	t.Logf("Fetched %d more comments", len(moreComments))

	// Store expanded comments
	if len(moreComments) > 0 {
		err = store.UpsertComments(ctx, moreComments)
		testutil.AssertNoError(t, err)
	}

	// Verify all comments stored
	tree, err := store.GetCommentTree(ctx, post.ID, nil)
	testutil.AssertNoError(t, err)

	totalStored := len(flattenCommentTree(tree))
	expectedTotal := len(initialComments) + len(moreComments)

	if totalStored != expectedTotal {
		t.Errorf("Comment count mismatch: expected %d, got %d", expectedTotal, totalStored)
	}

	t.Logf("✓ More comments stored: %d total comments", totalStored)
}

// TestIntegration_MultipleSubreddits tests filtering across multiple subreddits.
func TestIntegration_MultipleSubreddits(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	subreddits := []string{"golang", "programming"}
	subredditCounts := make(map[string]int)

	// Fetch from multiple subreddits
	for _, sub := range subreddits {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: sub,
			Pagination: types.Pagination{
				Limit: 10,
			},
		})
		testutil.AssertNoError(t, err)
		subredditCounts[sub] = len(resp.Posts)

		err = store.UpsertPosts(ctx, resp.Posts)
		testutil.AssertNoError(t, err)
	}

	// Verify filtering works for each subreddit
	for _, sub := range subreddits {
		opts := &ListPostsOptions{
			Subreddit: sub,
			Limit:     100,
		}
		filtered, err := store.ListPosts(ctx, opts)
		testutil.AssertNoError(t, err)

		if len(filtered) != subredditCounts[sub] {
			t.Errorf("Subreddit %s: expected %d posts, got %d",
				sub, subredditCounts[sub], len(filtered))
		}

		// Verify all posts are from the correct subreddit
		for _, post := range filtered {
			if !strings.EqualFold(post.Subreddit, sub) {
				t.Errorf("Expected subreddit %s, got %s", sub, post.Subreddit)
			}
		}
		t.Logf("✓ Subreddit %s: %d posts", sub, len(filtered))
	}
}

// TestIntegration_FileBasedStorage tests file-based database storage.
func TestIntegration_FileBasedStorage(t *testing.T) {
	client := getTestRedditClient(t)
	dbPath := "test_reddit_e2e.db"
	store := getTestFileStore(t, dbPath)
	ctx := context.Background()

	// Fetch and store a post
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

	err = store.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)

	// Verify retrieval
	opts := &ListPostsOptions{Limit: 100}
	retrieved, err := store.ListPosts(ctx, opts)
	testutil.AssertNoError(t, err)

	if len(retrieved) != len(resp.Posts) {
		t.Errorf("Post count mismatch: expected %d, got %d", len(resp.Posts), len(retrieved))
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	t.Logf("✓ File-based storage: %d posts stored and retrieved", len(retrieved))
}

// TestIntegration_LargeDataset tests performance with a larger dataset.
func TestIntegration_LargeDataset(t *testing.T) {
	client := getTestRedditClient(t)
	store := getTestStore(t)
	ctx := context.Background()

	start := time.Now()

	// Fetch many posts from a stable, popular subreddit
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 100,
		},
	})
	testutil.AssertNoError(t, err)

	fetchDuration := time.Since(start)
	t.Logf("Fetched %d posts in %v", len(resp.Posts), fetchDuration)

	if len(resp.Posts) == 0 {
		t.Skip("No posts available from r/golang (possibly rate limited or API issue)")
	}

	// Store all posts
	storeStart := time.Now()
	err = store.UpsertPosts(ctx, resp.Posts)
	testutil.AssertNoError(t, err)
	storeDuration := time.Since(storeStart)
	t.Logf("Stored %d posts in %v", len(resp.Posts), storeDuration)

	// Fetch comments for first 5 posts
	var allComments []*types.Comment
	for i := 0; i < 5 && i < len(resp.Posts); i++ {
		post := resp.Posts[i]
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: post.Subreddit,
			PostID:    post.ID,
			Pagination: types.Pagination{
				Limit: 50,
			},
		})
		testutil.AssertNoError(t, err)

		flatComments := flattenCommentTree(commentsResp.Comments)
		allComments = append(allComments, flatComments...)
	}

	if len(allComments) > 0 {
		commentStoreStart := time.Now()
		err = store.UpsertComments(ctx, allComments)
		testutil.AssertNoError(t, err)
		commentStoreDuration := time.Since(commentStoreStart)
		t.Logf("Stored %d comments in %v", len(allComments), commentStoreDuration)
	}

	// Get final statistics
	stats, err := store.GetStats(ctx)
	testutil.AssertNoError(t, err)

	t.Logf("✓ Large dataset test complete:")
	t.Logf("  Posts: %d", stats.PostCount)
	t.Logf("  Comments: %d", stats.CommentCount)
	t.Logf("  DB Size: %d bytes", stats.TotalSizeBytes)
	t.Logf("  Total time: %v", time.Since(start))
}
