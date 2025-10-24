package e2e

import (
	"context"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// E2E (End-to-End) benchmarks for Reddit API core endpoints.
//
// These benchmarks measure real-world performance against Reddit's live API,
// testing complete workflows from authentication through data retrieval and parsing.
// They require valid Reddit OAuth2 credentials set in environment variables.
//
// Prerequisites:
//   - REDDIT_CLIENT_ID: OAuth2 client ID from Reddit app registration
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret from Reddit app registration
//
// Optional (for user authentication):
//   - REDDIT_USERNAME: Reddit username
//   - REDDIT_PASSWORD: Reddit password
//
// Run with:
//
//	go test -bench=BenchmarkE2E ./benchmarks/e2e -benchmem
//
// Note: These benchmarks make real API calls and respect Reddit's rate limits.
// They will be skipped if credentials are not available.

// BenchmarkE2E_GetHot benchmarks fetching hot posts from different subreddits
// with varying limits. This measures real-world performance for the most common
// Reddit API operation: getting trending posts.
//
// Tests against:
//   - Small subreddit (r/golang) with minimal load
//   - Medium subreddit (r/programming) with moderate activity
//   - Large subreddit (r/AskReddit) with high traffic
//
// Limit variations test Reddit's pagination behavior and response size impact.
func BenchmarkE2E_GetHot(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		subreddit   string
		limit       int
		description string
	}{
		{
			name:        "golang_limit10",
			subreddit:   "golang",
			limit:       10,
			description: "Small tech subreddit, minimal response size",
		},
		{
			name:        "golang_limit25",
			subreddit:   "golang",
			limit:       25,
			description: "Small tech subreddit, default Reddit limit",
		},
		{
			name:        "programming_limit25",
			subreddit:   "programming",
			limit:       25,
			description: "Medium tech subreddit, moderate activity",
		},
		{
			name:        "programming_limit50",
			subreddit:   "programming",
			limit:       50,
			description: "Medium tech subreddit, larger response",
		},
		{
			name:        "AskReddit_limit50",
			subreddit:   "AskReddit",
			limit:       50,
			description: "Large popular subreddit, high traffic",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx := context.Background()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit: tt.subreddit,
					Pagination: types.Pagination{
						Limit: tt.limit,
					},
				})
				if err != nil {
					b.Fatalf("GetHot failed for r/%s: %v", tt.subreddit, err)
				}

				// Capture first iteration response for inspection
				if i == 0 {
					saveFixture(b, tt.name, resp)
				}

				// Validate response contains expected data
				if len(resp.Posts) == 0 {
					b.Errorf("Expected posts in response, got empty array")
				}
			}
		})
	}
}

// BenchmarkE2E_GetNew benchmarks fetching new posts from a subreddit with
// varying limits. This tests Reddit's chronological sorting and measures
// performance across different response sizes.
//
// Uses r/golang as a stable, moderately-active subreddit for consistent benchmarking.
// Tests all common limit values from small (10) to Reddit's maximum (100).
func BenchmarkE2E_GetNew(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		limit       int
		description string
	}{
		{
			name:        "limit10",
			limit:       10,
			description: "Minimal response size, quick refresh scenarios",
		},
		{
			name:        "limit25",
			limit:       25,
			description: "Reddit's default limit, most common use case",
		},
		{
			name:        "limit50",
			limit:       50,
			description: "Medium batch size for moderate data collection",
		},
		{
			name:        "limit100",
			limit:       100,
			description: "Reddit's maximum limit per request, bulk fetching",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx := context.Background()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.GetNew(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit: tt.limit,
					},
				})
				if err != nil {
					b.Fatalf("GetNew failed: %v", err)
				}

				// Capture first iteration response for inspection
				if i == 0 {
					saveFixture(b, "new_"+tt.name, resp)
				}

				// Validate response
				if len(resp.Posts) == 0 {
					b.Errorf("Expected posts in response, got empty array")
				}
			}
		})
	}
}

// BenchmarkE2E_GetComments benchmarks fetching comments from real Reddit posts.
// This tests the most complex API operation: parsing nested comment trees with
// varying depths and sizes.
//
// Test cases:
//   - Shallow threads: Recent posts from smaller communities (r/golang)
//   - Deep threads: Active discussions from popular subreddits (r/programming, r/AskReddit)
//
// Note: Post IDs are intentionally left as placeholders. In real usage, replace these
// with actual post IDs from popular threads, or fetch hot posts first and use their IDs.
// The benchmark will fail if the post doesn't exist, which is expected behavior.
func BenchmarkE2E_GetComments(b *testing.B) {
	skipIfNoCredentials(b)

	// First, fetch some real post IDs from hot posts to use in benchmarks
	client := newE2EClient(b)
	ctx := context.Background()

	// Get post IDs from different subreddits
	hotGolang, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit:  "golang",
		Pagination: types.Pagination{Limit: 5},
	})
	if err != nil || len(hotGolang.Posts) == 0 {
		b.Skipf("Could not fetch hot posts from r/golang to get test post IDs: %v", err)
	}

	hotProgramming, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit:  "programming",
		Pagination: types.Pagination{Limit: 5},
	})
	if err != nil || len(hotProgramming.Posts) == 0 {
		b.Skipf("Could not fetch hot posts from r/programming to get test post IDs: %v", err)
	}

	// Validate post IDs are not empty
	if hotGolang.Posts[0].ID == "" {
		b.Skip("Post ID from r/golang is empty")
	}
	if hotProgramming.Posts[0].ID == "" {
		b.Skip("Post ID from r/programming is empty")
	}

	tests := []struct {
		name        string
		subreddit   string
		postID      string
		description string
	}{
		{
			name:        "golang_shallow",
			subreddit:   "golang",
			postID:      hotGolang.Posts[0].ID,
			description: "Small subreddit, typically shorter comment threads",
		},
		{
			name:        "golang_medium",
			subreddit:   "golang",
			postID:      hotGolang.Posts[1].ID,
			description: "Small subreddit, alternative thread structure",
		},
		{
			name:        "programming_deep",
			subreddit:   "programming",
			postID:      hotProgramming.Posts[0].ID,
			description: "Medium subreddit, more active discussions",
		},
		{
			name:        "programming_deeper",
			subreddit:   "programming",
			postID:      hotProgramming.Posts[1].ID,
			description: "Medium subreddit, potentially deeper nesting",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ctx := context.Background()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.GetComments(ctx, &types.CommentsRequest{
					Subreddit: tt.subreddit,
					PostID:    tt.postID,
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				if err != nil {
					b.Fatalf("GetComments failed for post %s: %v", tt.postID, err)
				}

				// Capture first iteration response for inspection
				if i == 0 {
					saveFixture(b, "comments_"+tt.name, resp)
				}

				// Comments might be empty for very new posts, but response should exist
				if resp == nil {
					b.Fatal("Expected non-nil comments response")
				}
			}
		})
	}
}

// BenchmarkE2E_GetSubreddit benchmarks fetching subreddit information.
// This tests a simpler API operation that returns metadata about a community.
//
// Tests against subreddits of varying sizes to measure any performance differences
// based on subreddit popularity or complexity.
func BenchmarkE2E_GetSubreddit(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		subreddit   string
		description string
	}{
		{
			name:        "golang",
			subreddit:   "golang",
			description: "Small tech subreddit (~500K subscribers)",
		},
		{
			name:        "programming",
			subreddit:   "programming",
			description: "Medium tech subreddit (~5M subscribers)",
		},
		{
			name:        "AskReddit",
			subreddit:   "AskReddit",
			description: "Large popular subreddit (~40M+ subscribers)",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx := context.Background()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.GetSubreddit(ctx, tt.subreddit)
				if err != nil {
					b.Fatalf("GetSubreddit failed for r/%s: %v", tt.subreddit, err)
				}

				// Capture first iteration response for inspection
				if i == 0 {
					saveFixture(b, "subreddit_"+tt.name, resp)
				}

				// Validate response contains expected data
				if resp == nil {
					b.Fatal("Expected non-nil subreddit response")
				}
				if resp.DisplayName != tt.subreddit {
					b.Errorf("Expected display name %s, got %s", tt.subreddit, resp.DisplayName)
				}
			}
		})
	}
}

// BenchmarkE2E_Pagination benchmarks real pagination workflows across multiple pages.
// This measures the complete pagination experience including following "after" cursors
// through multiple pages of results.
//
// Tests sequential page fetching which is a common pattern in Reddit clients for
// infinite scroll, content archiving, or bulk data collection.
func BenchmarkE2E_Pagination(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name         string
		subreddit    string
		pages        int
		limitPerPage int
		description  string
	}{
		{
			name:         "golang_3pages_25each",
			subreddit:    "golang",
			pages:        3,
			limitPerPage: 25,
			description:  "Common pagination scenario: 3 pages of 25 posts",
		},
		{
			name:         "programming_5pages_10each",
			subreddit:    "programming",
			pages:        5,
			limitPerPage: 10,
			description:  "Shallow pagination: more pages, fewer items each",
		},
		{
			name:         "golang_2pages_50each",
			subreddit:    "golang",
			pages:        2,
			limitPerPage: 50,
			description:  "Deep pagination: fewer pages, more items each",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var after string
				var prevAfter string
				totalPosts := 0

				// Fetch multiple pages following "after" cursors
				for page := 0; page < tt.pages; page++ {
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

					totalPosts += len(resp.Posts)
					prevAfter = after
					after = resp.AfterFullname

					// If there's no more data, stop pagination
					if after == "" {
						break
					}

					// Detect infinite loop: if "after" cursor hasn't changed
					if page > 0 && after == prevAfter {
						b.Fatalf("Infinite pagination loop detected: after cursor '%s' hasn't changed between pages %d and %d", after, page-1, page)
					}
				}

				// Validate we got data
				if totalPosts == 0 {
					b.Error("Expected posts across all pages, got none")
				}
			}
		})
	}
}

// BenchmarkE2E_MixedOperations benchmarks realistic workflows that combine multiple
// API calls. This simulates real application usage patterns.
//
// Example workflow:
//  1. Fetch hot posts from a subreddit
//  2. Get comments for the top post
//  3. Get subreddit info for context
//
// This measures the total latency and overhead of a typical user session.
func BenchmarkE2E_MixedOperations(b *testing.B) {
	skipIfNoCredentials(b)

	tests := []struct {
		name        string
		subreddit   string
		description string
	}{
		{
			name:        "golang_full_workflow",
			subreddit:   "golang",
			description: "Complete workflow: posts + comments + subreddit info",
		},
		{
			name:        "programming_full_workflow",
			subreddit:   "programming",
			description: "Complete workflow on larger subreddit",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			client := newE2EClient(b)
			ctx := context.Background()

			// Report allocations for memory profiling
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Step 1: Fetch hot posts
				hotResp, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit:  tt.subreddit,
					Pagination: types.Pagination{Limit: 10},
				})
				if err != nil {
					b.Fatalf("GetHot failed: %v", err)
				}

				// Step 2: Get comments for the first post (if available)
				if hotResp != nil && len(hotResp.Posts) > 0 && hotResp.Posts[0] != nil {
					_, err := client.GetComments(ctx, &types.CommentsRequest{
						Subreddit:  tt.subreddit,
						PostID:     hotResp.Posts[0].ID,
						Pagination: types.Pagination{Limit: 50},
					})
					if err != nil {
						b.Fatalf("GetComments failed: %v", err)
					}
				}

				// Step 3: Get subreddit information
				_, err = client.GetSubreddit(ctx, tt.subreddit)
				if err != nil {
					b.Fatalf("GetSubreddit failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkE2E_RateLimitRespect benchmarks how the client handles Reddit's rate limits
// under realistic load. This measures the overhead of rate limit tracking and
// proactive throttling.
//
// Note: This benchmark may take longer to run as it intentionally triggers rate limiting
// behavior to measure the client's response.
func BenchmarkE2E_RateLimitRespect(b *testing.B) {
	skipIfNoCredentials(b)

	client := newE2EClient(b)
	ctx := context.Background()

	// Report allocations for memory profiling
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make sequential requests to exercise rate limiting
		// The client should automatically throttle based on Reddit's rate limit headers
		for j := 0; j < 5; j++ {
			_, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 10},
			})
			if err != nil {
				b.Fatalf("Request %d failed: %v", j, err)
			}
		}
	}
}
