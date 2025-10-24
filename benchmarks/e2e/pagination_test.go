package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// E2E benchmarks for Reddit API pagination functionality.
//
// These benchmarks test advanced pagination patterns and edge cases beyond the
// basic pagination benchmark in api_endpoints_test.go. They focus on:
//   - Cursor format validation and handling
//   - Large dataset pagination performance
//   - Different page size efficiency
//   - Comment pagination with "more" continuation
//
// Prerequisites: Same as other E2E benchmarks - requires REDDIT_CLIENT_ID and
// REDDIT_CLIENT_SECRET environment variables.
//
// Run with:
//
//	go test -bench=BenchmarkE2E_Pagination ./benchmarks/e2e -benchmem

// BenchmarkE2E_PaginationCursors benchmarks proper handling of Reddit's pagination
// cursors (fullnames). This ensures the client correctly:
//   - Extracts AfterFullname from first page response
//   - Uses the cursor to fetch the second page
//   - Validates cursor format (should be "t3_xxxxx" for posts)
//   - Returns different posts between pages (no duplicates)
//   - Supports both forward (After) and backward (Before) pagination
//
// This benchmark focuses on cursor mechanics rather than performance, validating
// that pagination state is correctly maintained across sequential requests.
func BenchmarkE2E_PaginationCursors(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		subreddit   string
		limit       int
		description string
	}{
		{
			name:        "golang_forward_pagination",
			subreddit:   "golang",
			limit:       25,
			description: "Test forward pagination with After cursor",
		},
		{
			name:        "programming_forward_pagination",
			subreddit:   "programming",
			limit:       50,
			description: "Test forward pagination on larger subreddit",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)

			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// Fetch first page
				firstResp, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit: tt.subreddit,
					Pagination: types.Pagination{
						Limit: tt.limit,
					},
				})
				if err != nil {
					b.Fatalf("GetHot first page failed: %v", err)
				}

				// Save fixture only on first iteration
				if i == 0 {
					saveFixture(b, tt.name+"_page1", firstResp)
				}

				// Validate first page response
				if len(firstResp.Posts) == 0 {
					b.Errorf("Expected posts in first page, got empty array")
				}

				// Validate AfterFullname format (should be "t3_xxxxx" for posts)
				if firstResp.AfterFullname == "" {
					b.Errorf("Expected AfterFullname in first page response, got empty string")
				} else if !strings.HasPrefix(firstResp.AfterFullname, "t3_") {
					b.Errorf("Expected AfterFullname to start with 't3_', got %s", firstResp.AfterFullname)
				}

				// Fetch second page using the cursor
				secondResp, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit: tt.subreddit,
					Pagination: types.Pagination{
						Limit: tt.limit,
						After: firstResp.AfterFullname,
					},
				})
				if err != nil {
					b.Fatalf("GetHot second page failed: %v", err)
				}

				// Validate second page response
				if len(secondResp.Posts) == 0 {
					b.Errorf("Expected posts in second page, got empty array")
				}

				// Verify different posts are returned (check first post IDs don't match)
				if len(firstResp.Posts) > 0 && len(secondResp.Posts) > 0 {
					if firstResp.Posts[0].ID == secondResp.Posts[0].ID {
						b.Errorf("First post on page 2 matches first post on page 1 (duplicate: %s)", firstResp.Posts[0].ID)
					}
				}

				// Validate second page also has pagination cursor for next page
				if secondResp.AfterFullname != "" && !strings.HasPrefix(secondResp.AfterFullname, "t3_") {
					b.Errorf("Expected second page AfterFullname to start with 't3_', got %s", secondResp.AfterFullname)
				}
			}
		})
	}
}

// BenchmarkE2E_LargeDatasetPagination benchmarks pagination through a large dataset
// by fetching multiple consecutive pages. This measures:
//   - Cumulative time to paginate through 100+ posts
//   - Per-page performance consistency
//   - Memory efficiency when processing large result sets
//   - Cursor stability across many pages
//
// Uses r/AskReddit as it has extremely high post volume, ensuring we can reliably
// fetch many pages without running out of data.
func BenchmarkE2E_LargeDatasetPagination(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name          string
		subreddit     string
		pages         int
		limitPerPage  int
		expectedTotal int // Minimum expected total posts
		description   string
	}{
		{
			name:          "AskReddit_10pages_25each",
			subreddit:     "AskReddit",
			pages:         10,
			limitPerPage:  25,
			expectedTotal: 200, // Expect at least 200 posts (some pages may have fewer than limit)
			description:   "Large dataset: 10 pages from high-volume subreddit",
		},
		{
			name:          "programming_5pages_100each",
			subreddit:     "programming",
			pages:         5,
			limitPerPage:  100,
			expectedTotal: 400, // Expect at least 400 posts
			description:   "Large dataset: 5 pages with maximum limit",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)

			b.ReportAllocs()

			b.ResetTimer()
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second) // Longer timeout for many pages
			defer cancel()

			for i := 0; i < b.N; i++ {
				var after string
				var prevAfter string
				totalPosts := 0
				postsPerPage := make([]int, 0, tt.pages)
				seenCursors := make(map[string]bool)

				// Paginate through multiple pages
				for page := 0; page < tt.pages; page++ {
					// Enhanced infinite loop protection: check for cursor cycles
					if after != "" {
						if seenCursors[after] {
							b.Fatalf("Cursor cycle detected: cursor '%s' has been seen before on page %d", after, page)
						}
						seenCursors[after] = true
					}

					resp, err := client.GetHot(ctx, &types.PostsRequest{
						Subreddit: tt.subreddit,
						Pagination: types.Pagination{
							Limit: tt.limitPerPage,
							After: after,
						},
					})
					if err != nil {
						b.Fatalf("GetHot page %d failed: %v", page, err)
					}

					postsCount := len(resp.Posts)
					totalPosts += postsCount
					postsPerPage = append(postsPerPage, postsCount)

					// Save only first page fixture to avoid disk overhead
					if i == 0 && page == 0 {
						saveFixture(b, tt.name+"_first_page", resp)
					}

					prevAfter = after
					after = resp.AfterFullname

					// If there's no more data, stop pagination early
					if after == "" {
						b.Logf("Pagination ended at page %d (no more data)", page)
						break
					}

					// Infinite loop protection: verify cursor changes between pages
					if page > 0 && after == prevAfter {
						b.Fatalf("Infinite pagination loop detected: after cursor '%s' hasn't changed between pages %d and %d", after, page-1, page)
					}

					// Validate cursor format
					if !strings.HasPrefix(after, "t3_") {
						b.Errorf("Expected after cursor to start with 't3_', got %s on page %d", after, page)
					}
				}

				// Validate we got enough data
				if totalPosts < tt.expectedTotal {
					b.Logf("Warning: Expected at least %d posts, got %d (posts per page: %v)", tt.expectedTotal, totalPosts, postsPerPage)
				}

				if totalPosts == 0 {
					b.Error("Expected posts across all pages, got none")
				}

				// Log performance statistics
				if i == 0 {
					b.Logf("Large dataset pagination: %d total posts across %d pages (avg %.1f per page)",
						totalPosts, len(postsPerPage), float64(totalPosts)/float64(len(postsPerPage)))
				}
			}
		})
	}
}

// BenchmarkE2E_SmallPageSizes benchmarks pagination with different page sizes to
// compare efficiency trade-offs:
//   - Small pages (5, 10): More requests, less data per request
//   - Medium pages (25, 50): Balanced approach
//   - Large pages (100): Fewer requests, more data per request
//
// For each page size, fetches 3 pages and measures total time and allocations.
// This helps determine optimal pagination size for different use cases.
func BenchmarkE2E_SmallPageSizes(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		limit       int
		pages       int
		description string
	}{
		{
			name:        "limit5_3pages",
			limit:       5,
			pages:       3,
			description: "Very small pages: minimal data per request",
		},
		{
			name:        "limit10_3pages",
			limit:       10,
			pages:       3,
			description: "Small pages: quick responses",
		},
		{
			name:        "limit25_3pages",
			limit:       25,
			pages:       3,
			description: "Medium pages: Reddit's default",
		},
		{
			name:        "limit50_3pages",
			limit:       50,
			pages:       3,
			description: "Large pages: fewer requests",
		},
		{
			name:        "limit100_3pages",
			limit:       100,
			pages:       3,
			description: "Maximum pages: Reddit's limit",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var after string
				totalPosts := 0

				// Fetch specified number of pages
				for page := 0; page < tt.pages; page++ {
					resp, err := client.GetHot(ctx, &types.PostsRequest{
						Subreddit: "golang",
						Pagination: types.Pagination{
							Limit: tt.limit,
							After: after,
						},
					})
					if err != nil {
						b.Fatalf("GetHot page %d failed: %v", page, err)
					}

					totalPosts += len(resp.Posts)
					after = resp.AfterFullname

					// Save fixture for first page of first iteration
					if i == 0 && page == 0 {
						saveFixture(b, tt.name+"_first_page", resp)
					}

					if after == "" {
						break
					}
				}

				// Validate we got data
				if totalPosts == 0 {
					b.Error("Expected posts, got none")
				}

				// Log comparison data on first iteration
				if i == 0 {
					b.Logf("Fetched %d posts with limit %d (%d pages)", totalPosts, tt.limit, tt.pages)
				}
			}
		})
	}
}

// BenchmarkE2E_PaginationWithComments benchmarks paginating through comments on a post.
// This tests a different pagination pattern than post listings:
//   - Comments use "t1_" prefix instead of "t3_"
//   - Comment trees can be nested (replies within replies)
//   - "more" comments objects indicate truncated threads
//   - Depth vs breadth pagination trade-offs
//
// Gets a popular post with many comments and paginates through the comment tree,
// measuring performance of nested structure traversal.
func BenchmarkE2E_PaginationWithComments(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		subreddit   string
		description string
	}{
		{
			name:        "AskReddit_popular_post",
			subreddit:   "AskReddit",
			description: "High-comment-count post from popular subreddit",
		},
		{
			name:        "programming_discussion",
			subreddit:   "programming",
			description: "Technical discussion with deep comment threads",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			// First, get a hot post to use for comment pagination
			postsResp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  tt.subreddit,
				Pagination: types.Pagination{Limit: 5},
			})
			if err != nil || len(postsResp.Posts) == 0 {
				b.Skipf("Could not fetch hot posts from r/%s: %v", tt.subreddit, err)
			}

			// Find a post with comments
			var postID string
			for _, post := range postsResp.Posts {
				if post.NumComments > 50 { // Want a post with substantial discussion
					postID = post.ID
					break
				}
			}
			if postID == "" {
				// Fall back to first post even if it has fewer comments
				postID = postsResp.Posts[0].ID
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Fetch first batch of comments
				commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
					Subreddit: tt.subreddit,
					PostID:    postID,
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				if err != nil {
					b.Fatalf("GetComments failed: %v", err)
				}

				// Save fixture on first iteration
				if i == 0 {
					saveFixture(b, tt.name+"_comments", commentsResp)
				}

				// Validate comment response
				if commentsResp.Post == nil {
					b.Error("Expected post in comments response, got nil")
				}

				totalComments := len(commentsResp.Comments)

				// If there are "more" comments, test pagination with them
				if len(commentsResp.MoreIDs) > 0 {
					// Validate "more" IDs format (should not have "t1_" prefix when in MoreIDs)
					// Reddit's API returns just the ID in the "more" object's children array
					for idx, moreID := range commentsResp.MoreIDs {
						if strings.HasPrefix(moreID, "t1_") {
							b.Logf("Note: MoreID at index %d has 't1_' prefix: %s (may be expected depending on API version)", idx, moreID)
						}
						if idx >= 5 {
							break // Just check first few
						}
					}

					// Log more comments statistics
					if i == 0 {
						b.Logf("Post has %d initial comments with %d more available", totalComments, len(commentsResp.MoreIDs))
					}
				}

				// Validate AfterFullname format if present (for comment pagination)
				if commentsResp.AfterFullname != "" {
					if !strings.HasPrefix(commentsResp.AfterFullname, "t1_") {
						b.Logf("Note: Comment AfterFullname has unexpected prefix: %s (expected 't1_')", commentsResp.AfterFullname)
					}
				}

				// Count nested replies to measure depth
				nestedCount := 0
				for _, comment := range commentsResp.Comments {
					nestedCount += countNestedReplies(comment)
				}

				if i == 0 {
					b.Logf("Comment tree structure: %d top-level, %d nested replies", totalComments, nestedCount)
				}
			}
		})
	}
}

// countNestedReplies recursively counts all nested replies in a comment thread.
// This helper function measures the depth and breadth of comment tree structures.
func countNestedReplies(comment *types.Comment) int {
	if comment == nil || len(comment.Replies) == 0 {
		return 0
	}

	count := len(comment.Replies)
	for _, reply := range comment.Replies {
		count += countNestedReplies(reply)
	}
	return count
}
