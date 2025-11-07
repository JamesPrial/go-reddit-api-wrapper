package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/auth"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/cache"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
	validatorpkg "github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/validator"
)

// Scenario benchmarks measure real-world workflows for the Reddit client.
// These benchmarks simulate complete user workflows including API calls,
// parsing, data processing, and typical analysis patterns.
//
// All benchmarks use MockClock for deterministic timing (no real delays),
// mock HTTP servers for realistic responses, and report allocations for
// memory profiling.

// BenchmarkScenario_MonitorSubreddit simulates continuous monitoring of a
// subreddit for new posts, like a bot that watches for new content.
//
// Workflow:
//  1. Fetch hot posts from subreddit
//  2. Check for new posts (GetNew)
//  3. Compare fullnames to detect new content
//  4. Repeat for N iterations to simulate continuous monitoring
func BenchmarkScenario_MonitorSubreddit(b *testing.B) {
	tests := []struct {
		name         string
		pollInterval time.Duration
		postsPerPoll int
		iterations   int
	}{
		{
			name:         "fast_poll_10posts_5iterations",
			pollInterval: 1 * time.Second,
			postsPerPoll: 10,
			iterations:   5,
		},
		{
			name:         "medium_poll_25posts_10iterations",
			pollInterval: 5 * time.Second,
			postsPerPoll: 25,
			iterations:   10,
		},
		{
			name:         "slow_poll_50posts_20iterations",
			pollInterval: 10 * time.Second,
			postsPerPoll: 50,
			iterations:   20,
		},
		{
			name:         "high_freq_100posts_10iterations",
			pollInterval: 1 * time.Second,
			postsPerPoll: 100,
			iterations:   10,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixture BEFORE timing starts (excluding I/O from benchmark)
			fixture := loadScenarioFixture(b, "small_posts.json")

			// Create server with validated fixture
			server := createPollingServer(fixture, tt.iterations)
			defer server.Close()

			// Create client with MockClock
			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			ctx := context.Background()
			seenPosts := make(map[string]bool)

			// Start timing and allocation tracking after all setup is complete
			b.ReportAllocs()
			b.ResetTimer()
			// Set bytes for throughput: fixture size * iterations
			totalBytes := int64(len(fixture) * tt.iterations)
			b.SetBytes(totalBytes)
			for i := 0; i < b.N; i++ {
				for iter := 0; iter < tt.iterations; iter++ {
					// Fetch new posts
					resp, err := redditClient.GetNew(ctx, &types.PostsRequest{
						Subreddit:  "golang",
						Pagination: types.Pagination{Limit: tt.postsPerPoll},
					})
					if err != nil {
						b.Fatalf("GetNew failed: %v", err)
					}

					// Process posts and detect new ones
					newCount := 0
					for _, post := range resp.Posts {
						if post == nil {
							continue
						}
						if !seenPosts[post.ID] {
							seenPosts[post.ID] = true
							newCount++
						}
					}

					_ = newCount

					// Advance mock clock to simulate polling interval
					mockClock.Advance(tt.pollInterval)

					// Clean up seen posts map to prevent memory bloat.
					// Clear oldest entries by iterating and deleting rather than reallocating,
					// which is more memory efficient and maintains the map's underlying storage.
					if len(seenPosts) > 1000 {
						for key := range seenPosts {
							delete(seenPosts, key)
							if len(seenPosts) <= 500 {
								break
							}
						}
					}
				}
			}
		})
	}
}

// BenchmarkScenario_AnalyzeThread simulates deep thread analysis for research
// or discussion pattern analysis.
//
// Workflow:
//  1. Fetch a post's comments with GetComments
//  2. Extract all comment IDs
//  3. Use GetMoreComments to fetch hidden child comments
//  4. Build complete comment tree
//  5. Count total comments, max depth, reply patterns
func BenchmarkScenario_AnalyzeThread(b *testing.B) {
	tests := []struct {
		name              string
		fixture           string
		fetchMoreComments bool
		maxDepth          int
		description       string
	}{
		{
			name:              "shallow_no_more",
			fixture:           "wide_comments.json",
			fetchMoreComments: false,
			maxDepth:          10,
			description:       "Wide thread, no additional fetching",
		},
		{
			name:              "shallow_with_more",
			fixture:           "wide_comments.json",
			fetchMoreComments: true,
			maxDepth:          10,
			description:       "Wide thread, fetch more children",
		},
		{
			name:              "deep_no_more",
			fixture:           "deep_comments.json",
			fetchMoreComments: false,
			maxDepth:          50,
			description:       "Deep thread, no additional fetching",
		},
		{
			name:              "deep_with_more",
			fixture:           "deep_comments.json",
			fetchMoreComments: true,
			maxDepth:          50,
			description:       "Deep thread, fetch more children",
		},
		{
			name:              "deep_unlimited",
			fixture:           "deep_comments.json",
			fetchMoreComments: true,
			maxDepth:          -1,
			description:       "Deep thread, unlimited depth",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixtures BEFORE timing starts (excluding I/O from benchmark)
			commentsFixture := loadScenarioFixture(b, tt.fixture)
			moreCommentsFixture := loadScenarioFixture(b, "wide_comments.json")

			// Create server that handles both endpoints
			server := createAnalyzerServer(commentsFixture, moreCommentsFixture)
			defer server.Close()

			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			ctx := context.Background()

			// Start timing and allocation tracking after all setup is complete
			b.ReportAllocs()
			b.ResetTimer()
			// Set bytes for throughput: comments fixture + optional more comments fetch
			fixtureSize := int64(len(commentsFixture))
			if tt.fetchMoreComments {
				fixtureSize += int64(len(moreCommentsFixture))
			}
			b.SetBytes(fixtureSize)
			for i := 0; i < b.N; i++ {
				// Fetch initial comments
				resp, err := redditClient.GetComments(ctx, &types.CommentsRequest{
					Subreddit:  "golang",
					PostID:     "abc123",
					Pagination: types.Pagination{Limit: 100},
				})
				if err != nil {
					b.Fatalf("GetComments failed: %v", err)
				}

				// Analyze comment structure
				stats := analyzeCommentTree(resp.Comments, tt.maxDepth)

				// Fetch more comments if enabled and available
				if tt.fetchMoreComments && len(resp.MoreIDs) > 0 {
					moreIDs := resp.MoreIDs
					if len(moreIDs) > 20 {
						moreIDs = moreIDs[:20]
					}

					moreComments, err := redditClient.GetMoreComments(ctx, &types.MoreCommentsRequest{
						LinkID:     "abc123",
						CommentIDs: moreIDs,
						Sort:       "confidence",
					})
					if err != nil {
						b.Fatalf("GetMoreComments failed: %v", err)
					}

					// Add to stats
					stats.TotalComments += len(moreComments)
				}

				_ = stats
			}
		})
	}
}

// BenchmarkScenario_BulkFetch simulates fetching comments from multiple hot
// posts concurrently, like a data collection tool.
//
// Workflow:
//  1. GetHot to find top posts in subreddit
//  2. Extract post IDs from results
//  3. Use semaphore-controlled goroutines to fetch comments for all posts
//  4. Aggregate total comment count across all posts
//
// The concurrentLimit parameter controls the maximum number of concurrent
// comment requests allowed. This is implemented using a semaphore pattern
// that limits the number of goroutines executing GetComments concurrently.
func BenchmarkScenario_BulkFetch(b *testing.B) {
	tests := []struct {
		name            string
		subredditSize   string
		fixture         string
		postsToFetch    int
		concurrentLimit int // Maximum concurrent comment requests
	}{
		{
			name:            "small_5posts",
			subredditSize:   "small",
			fixture:         "small_posts.json",
			postsToFetch:    5,
			concurrentLimit: 5,
		},
		{
			name:            "small_10posts",
			subredditSize:   "small",
			fixture:         "small_posts.json",
			postsToFetch:    10,
			concurrentLimit: 10,
		},
		{
			name:            "medium_10posts_limit5",
			subredditSize:   "medium",
			fixture:         "medium_posts.json",
			postsToFetch:    10,
			concurrentLimit: 5,
		},
		{
			name:            "medium_25posts_limit10",
			subredditSize:   "medium",
			fixture:         "medium_posts.json",
			postsToFetch:    25,
			concurrentLimit: 10,
		},
		{
			name:            "large_25posts_limit20",
			subredditSize:   "large",
			fixture:         "large_posts.json",
			postsToFetch:    25,
			concurrentLimit: 20,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixtures BEFORE timing starts (excluding I/O from benchmark)
			postsFixture := loadScenarioFixture(b, tt.fixture)
			commentsFixture := loadScenarioFixture(b, "deep_comments.json")

			// Create server
			server := createBulkFetchServer(postsFixture, commentsFixture)
			defer server.Close()

			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			ctx := context.Background()

			// Start timing and allocation tracking after all setup is complete
			b.ReportAllocs()
			b.ResetTimer()
			// Set bytes for throughput calculation: posts fixture + comments for each fetched post
			totalBytes := int64(len(postsFixture) + (len(commentsFixture) * tt.postsToFetch))
			b.SetBytes(totalBytes)
			for i := 0; i < b.N; i++ {
				// Fetch hot posts
				hotResp, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: 100},
				})
				if err != nil {
					b.Fatalf("GetHot failed: %v", err)
				}

				// Extract post IDs
				postIDs := extractPostIDs(hotResp.Posts)
				if len(postIDs) > tt.postsToFetch {
					postIDs = postIDs[:tt.postsToFetch]
				}

				// Fetch comments concurrently with semaphore-controlled concurrency
				var wg sync.WaitGroup
				var mu sync.Mutex
				responses := make([]*types.CommentsResponse, len(postIDs))
				var fetchErr error
				sem := make(chan struct{}, tt.concurrentLimit) // Semaphore to limit concurrency

				for idx, postID := range postIDs {
					wg.Add(1)
					go func(index int, id string) {
						defer wg.Done()
						defer func() {
							if r := recover(); r != nil {
								mu.Lock()
								if fetchErr == nil {
									fetchErr = fmt.Errorf("panic in goroutine for post %s: %v", id, r)
								}
								mu.Unlock()
								b.Errorf("panic recovered in goroutine for post %s: %v", id, r)
							}
						}()

						// Acquire semaphore or respect context cancellation
						select {
						case sem <- struct{}{}:
						case <-ctx.Done():
							mu.Lock()
							if fetchErr == nil {
								fetchErr = ctx.Err()
							}
							mu.Unlock()
							return
						}

						// Release semaphore after successful acquisition
						defer func() { <-sem }()

						// Fetch comments for this post
						resp, err := redditClient.GetComments(ctx, &types.CommentsRequest{
							Subreddit:  "golang",
							PostID:     id,
							Pagination: types.Pagination{Limit: 100},
						})
						if err != nil {
							mu.Lock()
							if fetchErr == nil {
								fetchErr = fmt.Errorf("GetComments failed for post %s: %w", id, err)
							}
							mu.Unlock()
							return
						}

						mu.Lock()
						responses[index] = resp
						mu.Unlock()
					}(idx, postID)
				}

				wg.Wait()

				if fetchErr != nil {
					b.Fatalf("concurrent fetch failed: %v", fetchErr)
				}

				// Aggregate total comments
				totalComments := 0
				for _, resp := range responses {
					if resp != nil {
						totalComments += len(resp.Comments)
					}
				}

				_ = totalComments
			}
		})
	}
}

// BenchmarkScenario_UserActivityTracking simulates tracking a user's activity
// across subreddits, like a profile analyzer.
//
// Workflow:
//  1. Call Me() to get authenticated user info
//  2. Parse user data
//  3. Fetch user's recent posts (simulated with GetNew)
//  4. Track karma, post count, comment patterns
func BenchmarkScenario_UserActivityTracking(b *testing.B) {
	tests := []struct {
		name               string
		activitySpan       string
		postCount          int
		includeComments    bool
		subredditDiversity int
	}{
		{
			name:               "recent_10posts",
			activitySpan:       "recent",
			postCount:          10,
			includeComments:    false,
			subredditDiversity: 1,
		},
		{
			name:               "moderate_50posts_no_comments",
			activitySpan:       "moderate",
			postCount:          50,
			includeComments:    false,
			subredditDiversity: 5,
		},
		{
			name:               "moderate_50posts_with_comments",
			activitySpan:       "moderate",
			postCount:          50,
			includeComments:    true,
			subredditDiversity: 5,
		},
		{
			name:               "extensive_100posts_with_comments",
			activitySpan:       "extensive",
			postCount:          100,
			includeComments:    true,
			subredditDiversity: 10,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixtures BEFORE timing starts (excluding I/O from benchmark)
			postsFixture := loadScenarioFixture(b, "medium_posts.json")
			commentsFixture := loadScenarioFixture(b, "deep_comments.json")

			// Create server with user endpoint
			server := createUserActivityServer(postsFixture, commentsFixture, tt.postCount)
			defer server.Close()

			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			ctx := context.Background()

			// Start timing and allocation tracking after all setup is complete
			b.ReportAllocs()
			b.ResetTimer()
			// Set bytes for throughput: posts + optional comments
			totalBytes := int64(len(postsFixture))
			if tt.includeComments {
				// Comments fetched for up to 5 posts
				postsWithComments := tt.postCount
				if postsWithComments > 5 {
					postsWithComments = 5
				}
				totalBytes += int64(len(commentsFixture) * postsWithComments)
			}
			b.SetBytes(totalBytes)
			for i := 0; i < b.N; i++ {
				// Get user info
				userInfo, err := redditClient.Me(ctx)
				if err != nil {
					b.Fatalf("Me failed: %v", err)
				}

				// Fetch user's posts
				// NOTE: This currently uses GetNew from a subreddit as a workaround.
				// A real implementation would use Reddit's user activity endpoint
				// (e.g., GET /user/{username}/submitted) to fetch user-specific posts.
				// The current Reddit client does not implement user activity endpoints yet.
				// This limitation means we're benchmarking subreddit post fetching instead
				// of true user activity tracking.
				postsResp, err := redditClient.GetNew(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: tt.postCount},
				})
				if err != nil {
					b.Fatalf("GetNew failed: %v", err)
				}

				// Track activity metrics
				activity := &UserActivity{
					Username:     userInfo.Name,
					LinkKarma:    userInfo.LinkKarma,
					CommentKarma: userInfo.CommentKarma,
					PostCount:    len(postsResp.Posts),
					Subreddits:   make(map[string]int),
					TotalScore:   0,
				}

				for _, post := range postsResp.Posts {
					if post == nil {
						continue
					}
					activity.Subreddits[post.Subreddit]++
					activity.TotalScore += post.Score
				}

				// Fetch comments if enabled
				if tt.includeComments && len(postsResp.Posts) > 0 {
					// Fetch comments for first few posts
					postsToCheck := postsResp.Posts
					if len(postsToCheck) > 5 {
						postsToCheck = postsToCheck[:5]
					}

					for _, post := range postsToCheck {
						if post == nil {
							continue
						}
						commentsResp, err := redditClient.GetComments(ctx, &types.CommentsRequest{
							Subreddit:  post.Subreddit,
							PostID:     post.ID,
							Pagination: types.Pagination{Limit: 50},
						})
						if err != nil {
							continue
						}
						activity.CommentCount += len(commentsResp.Comments)
					}
				}

				_ = activity
			}
		})
	}
}

// BenchmarkScenario_TrendingTopics simulates identifying trending topics
// across multiple subreddits, like a trend analysis tool.
//
// Workflow:
//  1. GetHot from multiple subreddits concurrently
//  2. Extract titles and keywords
//  3. Identify common terms across subreddits
//  4. Rank by frequency
func BenchmarkScenario_TrendingTopics(b *testing.B) {
	tests := []struct {
		name              string
		subredditCount    int
		postsPerSubreddit int
		concurrentFetch   bool
	}{
		{
			name:              "3subs_10posts_sequential",
			subredditCount:    3,
			postsPerSubreddit: 10,
			concurrentFetch:   false,
		},
		{
			name:              "3subs_10posts_concurrent",
			subredditCount:    3,
			postsPerSubreddit: 10,
			concurrentFetch:   true,
		},
		{
			name:              "5subs_25posts_sequential",
			subredditCount:    5,
			postsPerSubreddit: 25,
			concurrentFetch:   false,
		},
		{
			name:              "5subs_25posts_concurrent",
			subredditCount:    5,
			postsPerSubreddit: 25,
			concurrentFetch:   true,
		},
		{
			name:              "10subs_50posts_concurrent",
			subredditCount:    10,
			postsPerSubreddit: 50,
			concurrentFetch:   true,
		},
	}

	subreddits := []string{"golang", "programming", "coding", "webdev", "learnprogramming",
		"javascript", "python", "rust", "java", "cpp"}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixture BEFORE timing starts (excluding I/O from benchmark)
			postsFixture := loadScenarioFixture(b, "medium_posts.json")

			// Create server
			server := createTrendingTopicsServer(postsFixture)
			defer server.Close()

			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			ctx := context.Background()
			targetSubs := subreddits[:tt.subredditCount]

			// Start timing and allocation tracking after all setup is complete
			b.ReportAllocs()
			b.ResetTimer()
			// Set bytes for throughput calculation: fixture size * number of subreddits
			totalBytes := int64(len(postsFixture) * tt.subredditCount)
			b.SetBytes(totalBytes)
			for i := 0; i < b.N; i++ {
				allPosts := make([]*types.Post, 0, tt.subredditCount*tt.postsPerSubreddit)

				if tt.concurrentFetch {
					// Concurrent fetch using goroutines with semaphore pattern
					var wg sync.WaitGroup
					var mu sync.Mutex
					results := make([][]*types.Post, len(targetSubs))
					var fetchErr error
					sem := make(chan struct{}, 10) // Semaphore to limit to 10 concurrent requests

					for idx, sub := range targetSubs {
						wg.Add(1)
						go func(index int, subreddit string) {
							defer wg.Done()
							defer func() {
								if r := recover(); r != nil {
									mu.Lock()
									if fetchErr == nil {
										fetchErr = fmt.Errorf("panic in goroutine for subreddit %s: %v", subreddit, r)
									}
									mu.Unlock()
									b.Errorf("panic recovered in goroutine for subreddit %s: %v", subreddit, r)
								}
							}()

							// Acquire semaphore or respect context cancellation
							select {
							case sem <- struct{}{}:
							case <-ctx.Done():
								mu.Lock()
								if fetchErr == nil {
									fetchErr = ctx.Err()
								}
								mu.Unlock()
								return
							}

							// Release semaphore after successful acquisition
							defer func() { <-sem }()

							hotResp, err := redditClient.GetHot(ctx, &types.PostsRequest{
								Subreddit:  subreddit,
								Pagination: types.Pagination{Limit: tt.postsPerSubreddit},
							})
							if err != nil {
								mu.Lock()
								if fetchErr == nil {
									fetchErr = fmt.Errorf("GetHot failed for %s: %w", subreddit, err)
								}
								mu.Unlock()
								return
							}

							mu.Lock()
							results[index] = hotResp.Posts
							mu.Unlock()
						}(idx, sub)
					}

					wg.Wait()

					if fetchErr != nil {
						b.Fatalf("concurrent fetch failed: %v", fetchErr)
					}

					// Combine results
					for _, posts := range results {
						allPosts = append(allPosts, posts...)
					}
				} else {
					// Sequential fetch
					for _, sub := range targetSubs {
						hotResp, err := redditClient.GetHot(ctx, &types.PostsRequest{
							Subreddit:  sub,
							Pagination: types.Pagination{Limit: tt.postsPerSubreddit},
						})
						if err != nil {
							b.Fatalf("GetHot failed for %s: %v", sub, err)
						}
						allPosts = append(allPosts, hotResp.Posts...)
					}
				}

				// Extract and analyze keywords
				keywords := extractKeywords(allPosts)
				trendingTerms := rankKeywords(keywords, 10)

				_ = trendingTerms
			}
		})
	}
}

// BenchmarkScenario_ConcurrentFetch simulates fetching posts from multiple
// subreddits concurrently, measuring the performance benefits of concurrent
// operations vs sequential fetching.
//
// Workflow:
//  1. Fetch hot posts from N subreddits concurrently using goroutines
//  2. Use semaphore pattern to control max concurrent requests
//  3. Collect and aggregate results from all subreddits
//  4. Compare concurrent vs sequential performance
func BenchmarkScenario_ConcurrentFetch(b *testing.B) {
	tests := []struct {
		name            string
		subredditCount  int
		postsPerSub     int
		concurrentLimit int
		sequential      bool
	}{
		{
			name:            "3subs_25posts_concurrent",
			subredditCount:  3,
			postsPerSub:     25,
			concurrentLimit: 3,
			sequential:      false,
		},
		{
			name:            "3subs_25posts_sequential",
			subredditCount:  3,
			postsPerSub:     25,
			concurrentLimit: 1,
			sequential:      true,
		},
		{
			name:            "5subs_25posts_concurrent",
			subredditCount:  5,
			postsPerSub:     25,
			concurrentLimit: 5,
			sequential:      false,
		},
		{
			name:            "5subs_25posts_sequential",
			subredditCount:  5,
			postsPerSub:     25,
			concurrentLimit: 1,
			sequential:      true,
		},
		{
			name:            "10subs_50posts_concurrent",
			subredditCount:  10,
			postsPerSub:     50,
			concurrentLimit: 10,
			sequential:      false,
		},
		{
			name:            "10subs_50posts_sequential",
			subredditCount:  10,
			postsPerSub:     50,
			concurrentLimit: 1,
			sequential:      true,
		},
		{
			name:            "10subs_50posts_concurrent_limited",
			subredditCount:  10,
			postsPerSub:     50,
			concurrentLimit: 5,
			sequential:      false,
		},
	}

	subreddits := []string{"golang", "programming", "coding", "webdev", "learnprogramming",
		"javascript", "python", "rust", "java", "cpp"}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixture BEFORE timing starts
			fixture := loadScenarioFixture(b, "medium_posts.json")

			// Create server
			server := createTrendingTopicsServer(fixture)
			defer server.Close()

			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			ctx := context.Background()
			targetSubs := subreddits[:tt.subredditCount]

			// Start timing and allocation tracking
			b.ReportAllocs()
			b.ResetTimer()
			// Set bytes for throughput calculation: fixture size * number of subreddits
			totalBytes := int64(len(fixture) * tt.subredditCount)
			b.SetBytes(totalBytes)
			for i := 0; i < b.N; i++ {
				if tt.sequential {
					// Sequential fetch
					allPosts := make([]*types.Post, 0, tt.subredditCount*tt.postsPerSub)
					for _, sub := range targetSubs {
						resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
							Subreddit:  sub,
							Pagination: types.Pagination{Limit: tt.postsPerSub},
						})
						if err != nil {
							b.Fatalf("GetHot failed for %s: %v", sub, err)
						}
						allPosts = append(allPosts, resp.Posts...)
					}
					_ = allPosts
				} else {
					// Concurrent fetch with semaphore control
					var wg sync.WaitGroup
					var mu sync.Mutex
					allPosts := make([]*types.Post, 0, tt.subredditCount*tt.postsPerSub)
					var fetchErr error
					sem := make(chan struct{}, tt.concurrentLimit)

					for _, sub := range targetSubs {
						wg.Add(1)
						go func(subreddit string) {
							defer wg.Done()
							defer func() {
								if r := recover(); r != nil {
									mu.Lock()
									if fetchErr == nil {
										fetchErr = fmt.Errorf("panic in goroutine for subreddit %s: %v", subreddit, r)
									}
									mu.Unlock()
									b.Errorf("panic recovered: %v", r)
								}
							}()

							// Acquire semaphore or respect context cancellation
							select {
							case sem <- struct{}{}:
							case <-ctx.Done():
								mu.Lock()
								if fetchErr == nil {
									fetchErr = ctx.Err()
								}
								mu.Unlock()
								return
							}

							// Release semaphore after successful acquisition
							defer func() { <-sem }()

							// Fetch posts
							resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
								Subreddit:  subreddit,
								Pagination: types.Pagination{Limit: tt.postsPerSub},
							})
							if err != nil {
								mu.Lock()
								if fetchErr == nil {
									fetchErr = fmt.Errorf("GetHot failed for %s: %w", subreddit, err)
								}
								mu.Unlock()
								return
							}

							mu.Lock()
							allPosts = append(allPosts, resp.Posts...)
							mu.Unlock()
						}(sub)
					}

					wg.Wait()

					if fetchErr != nil {
						b.Fatalf("concurrent fetch failed: %v", fetchErr)
					}

					_ = allPosts
				}
			}
		})
	}
}

// BenchmarkScenario_ContextCancellation simulates long-running operations with
// context cancellation to measure cancellation response time and cleanup overhead.
//
// Workflow:
//  1. Start fetching comments from multiple posts
//  2. Cancel context at various points (immediate, delayed, timeout)
//  3. Measure cancellation response time
//  4. Verify proper cleanup and goroutine termination
func BenchmarkScenario_ContextCancellation(b *testing.B) {
	tests := []struct {
		name         string
		postCount    int
		cancelDelay  time.Duration
		useTimeout   bool
		timeoutDelay time.Duration
		description  string
	}{
		{
			name:        "immediate_cancel_10posts",
			postCount:   10,
			cancelDelay: 0,
			useTimeout:  false,
			description: "Cancel immediately after starting",
		},
		{
			name:        "delayed_cancel_10posts_100ms",
			postCount:   10,
			cancelDelay: 100 * time.Millisecond,
			useTimeout:  false,
			description: "Cancel after 100ms delay",
		},
		{
			name:        "delayed_cancel_25posts_250ms",
			postCount:   25,
			cancelDelay: 250 * time.Millisecond,
			useTimeout:  false,
			description: "Cancel after 250ms delay",
		},
		{
			name:         "timeout_10posts_50ms",
			postCount:    10,
			cancelDelay:  0,
			useTimeout:   true,
			timeoutDelay: 50 * time.Millisecond,
			description:  "Timeout after 50ms",
		},
		{
			name:         "timeout_25posts_100ms",
			postCount:    25,
			cancelDelay:  0,
			useTimeout:   true,
			timeoutDelay: 100 * time.Millisecond,
			description:  "Timeout after 100ms",
		},
		{
			name:        "no_cancel_5posts",
			postCount:   5,
			cancelDelay: -1, // No cancellation
			useTimeout:  false,
			description: "Complete without cancellation (baseline)",
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Load and validate fixtures BEFORE timing starts
			postsFixture := loadScenarioFixture(b, "medium_posts.json")
			commentsFixture := loadScenarioFixture(b, "deep_comments.json")

			// Create server
			server := createBulkFetchServer(postsFixture, commentsFixture)
			defer server.Close()

			mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
			redditClient := createScenarioClient(b, server.URL, mockClock)

			// Start timing and allocation tracking
			b.ReportAllocs()
			b.ResetTimer()
			// Note: b.SetBytes() is not set for cancellation tests because throughput
			// metrics are meaningless when operations are intentionally cancelled before
			// completion. This benchmark measures cancellation latency, not data throughput.
			for i := 0; i < b.N; i++ {
				// Create context with optional timeout
				var ctx context.Context
				var cancel context.CancelFunc
				if tt.useTimeout {
					ctx, cancel = context.WithTimeout(context.Background(), tt.timeoutDelay)
				} else {
					ctx, cancel = context.WithCancel(context.Background())
				}

				// Schedule cancellation if needed
				if tt.cancelDelay >= 0 && !tt.useTimeout {
					if tt.cancelDelay == 0 {
						// Cancel immediately (but after goroutine start)
						defer cancel()
						cancel()
					} else {
						// Schedule delayed cancellation using real time.AfterFunc
						// (MockClock doesn't support AfterFunc for background operations)
						timer := time.AfterFunc(tt.cancelDelay, cancel)
						defer timer.Stop()
						defer cancel()
					}
				} else {
					defer cancel()
				}

				// Fetch hot posts first
				hotResp, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: 100},
				})

				// If context was cancelled immediately, we might get an error here
				if err != nil {
					if ctx.Err() != nil {
						// Expected cancellation
						continue
					}
					b.Fatalf("GetHot failed: %v", err)
				}

				// Extract post IDs
				postIDs := extractPostIDs(hotResp.Posts)
				if len(postIDs) > tt.postCount {
					postIDs = postIDs[:tt.postCount]
				}

				// Fetch comments concurrently with proper cancellation handling
				var wg sync.WaitGroup
				var mu sync.Mutex
				var fetchErr error
				completedCount := 0
				cancelledCount := 0
				sem := make(chan struct{}, 10) // Limit concurrent requests

				for _, postID := range postIDs {
					wg.Add(1)
					go func(id string) {
						defer wg.Done()
						defer func() {
							if r := recover(); r != nil {
								mu.Lock()
								if fetchErr == nil {
									fetchErr = fmt.Errorf("panic in goroutine for post %s: %v", id, r)
								}
								mu.Unlock()
								b.Errorf("panic recovered: %v", r)
							}
						}()

						// Acquire semaphore or respect context cancellation
						select {
						case sem <- struct{}{}:
						case <-ctx.Done():
							mu.Lock()
							cancelledCount++
							mu.Unlock()
							return
						}

						// Release semaphore after successful acquisition
						defer func() { <-sem }()

						// Fetch comments with context
						_, err := redditClient.GetComments(ctx, &types.CommentsRequest{
							Subreddit:  "golang",
							PostID:     id,
							Pagination: types.Pagination{Limit: 100},
						})

						mu.Lock()
						if err != nil {
							if ctx.Err() != nil {
								// Expected cancellation
								cancelledCount++
							} else if fetchErr == nil {
								fetchErr = fmt.Errorf("GetComments failed for post %s: %w", id, err)
							}
						} else {
							completedCount++
						}
						mu.Unlock()
					}(postID)
				}

				wg.Wait()

				// Don't fail on expected cancellation errors
				if fetchErr != nil && ctx.Err() == nil {
					b.Fatalf("fetch failed unexpectedly: %v", fetchErr)
				}

				// Track metrics (don't use them, but they're available for analysis)
				_ = completedCount
				_ = cancelledCount
			}
		})
	}
}

// Helper functions for scenario benchmarks

// UserActivity tracks user activity metrics
type UserActivity struct {
	Username     string
	LinkKarma    int
	CommentKarma int
	PostCount    int
	CommentCount int
	Subreddits   map[string]int
	TotalScore   int
}

// ThreadStats holds comment tree analysis results
type ThreadStats struct {
	TotalComments int
	MaxDepth      int
	AvgDepth      float64
	UniqueAuthors int
	TotalScore    int
}

// analyzeCommentTree analyzes a comment tree structure.
// When maxDepth >= 0, only analyzes comments with depth <= maxDepth.
// When maxDepth == -1, analyzes all comments regardless of depth (unlimited).
func analyzeCommentTree(comments []*types.Comment, maxDepth int) *ThreadStats {
	stats := &ThreadStats{}

	if len(comments) == 0 {
		return stats
	}

	authors := make(map[string]bool)
	depths := make([]int, 0, len(comments))
	processedCount := 0

	for _, comment := range comments {
		if comment == nil {
			continue
		}

		// Calculate depth (simplified - just count by parent chain)
		depth := calculateCommentDepth(comment, comments)

		// Skip comments that exceed maxDepth when maxDepth >= 0
		// When maxDepth == -1, analyze all depths (unlimited)
		if maxDepth >= 0 && depth > maxDepth {
			continue
		}

		// Process this comment
		processedCount++
		authors[comment.Author] = true
		stats.TotalScore += comment.Score

		if depth > stats.MaxDepth {
			stats.MaxDepth = depth
		}
		depths = append(depths, depth)
	}

	stats.TotalComments = processedCount
	stats.UniqueAuthors = len(authors)

	// Calculate average depth
	if len(depths) > 0 {
		sum := 0
		for _, d := range depths {
			sum += d
		}
		stats.AvgDepth = float64(sum) / float64(len(depths))
	}

	return stats
}

// calculateCommentDepth calculates the depth of a comment in the tree
// by traversing the parent chain. Returns 0 for top-level comments,
// 1 for direct replies to top-level, 2 for replies to those, etc.
func calculateCommentDepth(comment *types.Comment, allComments []*types.Comment) int {
	if comment.ParentID == "" || strings.HasPrefix(comment.ParentID, "t3_") {
		return 0 // Top-level comment (parent is the post itself)
	}

	// Build a lookup map for O(1) parent access
	commentsByID := make(map[string]*types.Comment, len(allComments))
	for _, c := range allComments {
		if c != nil && c.ID != "" {
			// Store by fullname (e.g., "t1_abc123") and plain ID
			commentsByID[c.ID] = c
			commentsByID["t1_"+c.ID] = c
		}
	}

	// Traverse parent chain with cycle detection
	const maxDepth = 1000 // Safety limit to prevent infinite loops
	seen := make(map[string]bool)
	depth := 0
	currentID := comment.ParentID

	for depth < maxDepth {
		// Check for cycles
		if seen[currentID] {
			break
		}
		seen[currentID] = true

		// Find parent comment
		parent, exists := commentsByID[currentID]
		if !exists {
			// Parent not in our collection, might be outside the fetched set
			// Assume it exists and increment depth
			depth++
			break
		}

		// Increment depth for this level
		depth++

		// Check if parent is top-level
		if parent.ParentID == "" || strings.HasPrefix(parent.ParentID, "t3_") {
			break
		}

		// Move to next parent
		currentID = parent.ParentID
	}

	return depth
}

// extractPostIDs extracts post IDs from a list of posts
func extractPostIDs(posts []*types.Post) []string {
	ids := make([]string, 0, len(posts))
	for _, post := range posts {
		if post != nil && post.ID != "" {
			ids = append(ids, post.ID)
		}
	}
	return ids
}

// extractKeywords extracts keywords from post titles
func extractKeywords(posts []*types.Post) map[string]int {
	keywords := make(map[string]int)

	for _, post := range posts {
		if post == nil {
			continue
		}

		// Simple keyword extraction - split title into words
		words := strings.Fields(strings.ToLower(post.Title))
		for _, word := range words {
			// Filter out common words (simplified)
			if len(word) > 3 {
				keywords[word]++
			}
		}
	}

	return keywords
}

// rankKeywords returns top N keywords by frequency
func rankKeywords(keywords map[string]int, topN int) []string {
	type kv struct {
		key   string
		value int
	}

	// Convert map to slice for sorting
	kvs := make([]kv, 0, len(keywords))
	for k, v := range keywords {
		kvs = append(kvs, kv{k, v})
	}

	// Sort by frequency (descending) using O(n log n) sort.Slice
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].value > kvs[j].value
	})

	// Extract top N
	result := make([]string, 0, topN)
	for i := 0; i < topN && i < len(kvs); i++ {
		result = append(result, kvs[i].key)
	}

	return result
}

// loadScenarioFixture loads and validates a JSON fixture file from benchmarks/testdata/.
// This function performs comprehensive validation to ensure fixtures have the correct structure
// and will fail early with clear error messages if fixtures are invalid.
// All validation happens before benchmark timing starts, excluding I/O from measurements.
func loadScenarioFixture(b *testing.B, filename string) []byte {
	b.Helper()

	wd, err := os.Getwd()
	if err != nil {
		b.Fatalf("failed to get working directory: %v", err)
	}

	fixturePath := filepath.Join(wd, "..", "benchmarks", "testdata", filename)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		b.Fatalf("failed to load fixture %s: %v", filename, err)
	}

	// Check that fixture is not empty
	if len(data) == 0 {
		b.Fatalf("fixture %s is empty", filename)
	}

	// Validate it's valid JSON
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		b.Fatalf("fixture %s is not valid JSON: %v", filename, err)
	}

	// Validate Reddit API structure based on filename pattern
	if strings.Contains(filename, "posts") {
		if err := validatePostsFixture(data, filename); err != nil {
			b.Fatalf("fixture %s failed posts validation: %v", filename, err)
		}
	} else if strings.Contains(filename, "comments") {
		if err := validateCommentsFixture(data, filename); err != nil {
			b.Fatalf("fixture %s failed comments validation: %v", filename, err)
		}
	}

	return data
}

// validatePostsFixture validates that a fixture contains valid Reddit posts structure
func validatePostsFixture(data []byte, filename string) error {
	var thing types.Thing
	if err := json.Unmarshal(data, &thing); err != nil {
		return fmt.Errorf("not a valid Thing structure: %w", err)
	}

	// Check for required "kind" field
	if thing.Kind == "" {
		return fmt.Errorf("missing required field 'kind'")
	}

	// Check for required "data" field
	if thing.Data == nil {
		return fmt.Errorf("missing required field 'data'")
	}

	// Validate it's a Listing
	if thing.Kind != "Listing" {
		return fmt.Errorf("expected kind 'Listing', got '%s'", thing.Kind)
	}

	// Parse listing data
	var listingData types.ListingData
	if err := json.Unmarshal(thing.Data, &listingData); err != nil {
		return fmt.Errorf("invalid listing data structure: %w", err)
	}

	// Check that children array exists and is not empty
	if listingData.Children == nil {
		return fmt.Errorf("missing 'children' array in listing data")
	}

	if len(listingData.Children) == 0 {
		return fmt.Errorf("empty 'children' array - fixture must contain at least one post")
	}

	// Validate first child has proper structure
	firstChild := listingData.Children[0]
	if firstChild == nil {
		return fmt.Errorf("first child in children array is null")
	}

	if firstChild.Kind == "" {
		return fmt.Errorf("first child missing 'kind' field")
	}

	if firstChild.Data == nil {
		return fmt.Errorf("first child missing 'data' field")
	}

	return nil
}

// validateCommentsFixture validates that a fixture contains valid Reddit comments structure
func validateCommentsFixture(data []byte, filename string) error {
	// Comments endpoint returns an array [post, comments]
	var response []types.Thing
	if err := json.Unmarshal(data, &response); err != nil {
		// Try single Thing structure for /api/morechildren response
		var thing types.Thing
		if err := json.Unmarshal(data, &thing); err != nil {
			return fmt.Errorf("not a valid Thing or array structure: %w", err)
		}

		// Validate single Thing structure
		if thing.Kind == "" {
			return fmt.Errorf("missing required field 'kind'")
		}

		if thing.Data == nil {
			return fmt.Errorf("missing required field 'data'")
		}

		return nil
	}

	// Validate array response
	if len(response) < 2 {
		return fmt.Errorf("comments response must contain at least 2 elements [post, comments], got %d", len(response))
	}

	// Validate post element (first)
	if response[0].Kind == "" {
		return fmt.Errorf("first element (post) missing 'kind' field")
	}

	if response[0].Data == nil {
		return fmt.Errorf("first element (post) missing 'data' field")
	}

	// Validate comments element (second)
	if response[1].Kind != "Listing" {
		return fmt.Errorf("second element (comments) should have kind 'Listing', got '%s'", response[1].Kind)
	}

	if response[1].Data == nil {
		return fmt.Errorf("second element (comments) missing 'data' field")
	}

	// Parse comments listing
	var listingData types.ListingData
	if err := json.Unmarshal(response[1].Data, &listingData); err != nil {
		return fmt.Errorf("invalid comments listing data: %w", err)
	}

	// Check that children array exists
	if listingData.Children == nil {
		return fmt.Errorf("missing 'children' array in comments listing")
	}

	return nil
}

// createPollingServer creates a mock server for polling scenarios.
// Returns different posts on each request to simulate new content.
func createPollingServer(baseFixture []byte, iterations int) *httptest.Server {
	var requestCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// Modify fixture to include iteration-specific data
		count := atomic.AddInt32(&requestCount, 1)

		// Parse base fixture
		var listing types.Thing
		if err := json.Unmarshal(baseFixture, &listing); err != nil {
			http.Error(w, "invalid fixture", http.StatusInternalServerError)
			return
		}

		// Modify post IDs to simulate new posts each iteration
		var listingData types.ListingData
		if err := json.Unmarshal(listing.Data, &listingData); err == nil {
			for i, child := range listingData.Children {
				if child != nil {
					// Modify child data to have unique ID
					var postData map[string]interface{}
					if err := json.Unmarshal(child.Data, &postData); err == nil {
						postData["id"] = fmt.Sprintf("post_%d_%d", count, i)
						postData["name"] = fmt.Sprintf("t3_post_%d_%d", count, i)
						modifiedData, _ := json.Marshal(postData)
						child.Data = modifiedData
					}
				}
			}
			// Marshal the modified listing data back
			modifiedListingData, err := json.Marshal(listingData)
			if err == nil {
				listing.Data = modifiedListingData
			}
		}

		// Marshal the entire modified listing and send
		modifiedResponse, err := json.Marshal(listing)
		if err != nil {
			http.Error(w, "failed to marshal modified fixture", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(modifiedResponse)
	}))
}

// createAnalyzerServer creates a mock server for thread analysis scenarios.
func createAnalyzerServer(commentsFixture, moreCommentsFixture []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// Route based on path
		if strings.Contains(r.URL.Path, "/comments/") {
			w.Write(commentsFixture)
		} else if strings.Contains(r.URL.Path, "/api/morechildren") {
			w.Write(moreCommentsFixture)
		} else {
			w.Write([]byte(`{"kind":"Listing","data":{"children":[]}}`))
		}
	}))
}

// createBulkFetchServer creates a mock server for bulk fetch scenarios.
func createBulkFetchServer(postsFixture, commentsFixture []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// Route based on path
		if strings.Contains(r.URL.Path, "/hot") || strings.Contains(r.URL.Path, "/new") {
			w.Write(postsFixture)
		} else if strings.Contains(r.URL.Path, "/comments/") {
			w.Write(commentsFixture)
		} else {
			w.Write([]byte(`{"kind":"Listing","data":{"children":[]}}`))
		}
	}))
}

// createUserActivityServer creates a mock server for user activity scenarios.
func createUserActivityServer(postsFixture, commentsFixture []byte, postCount int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// Route based on path
		if strings.Contains(r.URL.Path, "/api/v1/me") {
			// Return user data directly without Thing wrapper (matches actual Reddit API)
			response := map[string]interface{}{
				"id":            "abc123xyz",
				"name":          "t2_abc123xyz",
				"link_karma":    10000,
				"comment_karma": 5000,
				"created":       1704067200.0,
				"created_utc":   1704067200.0,
			}
			json.NewEncoder(w).Encode(response)
		} else if strings.Contains(r.URL.Path, "/new") {
			w.Write(postsFixture)
		} else if strings.Contains(r.URL.Path, "/comments/") {
			w.Write(commentsFixture)
		} else {
			w.Write([]byte(`{"kind":"Listing","data":{"children":[]}}`))
		}
	}))
}

// createTrendingTopicsServer creates a mock server for trending topics scenarios.
func createTrendingTopicsServer(postsFixture []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// All subreddits return the same posts for simplicity
		w.Write(postsFixture)
	}))
}

// createScenarioClient creates a Reddit client configured for scenario benchmarks.
func createScenarioClient(b *testing.B, serverURL string, mockClock clock.Clock) *Reddit {
	b.Helper()

	// Create discard logger to minimize overhead
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "test-token-12345", "token_type": "bearer", "expires_in": 3600}`))
	}))
	b.Cleanup(func() { authServer.Close() })

	authenticator, err := auth.NewAuthenticator(
		httpClient,
		"",
		"",
		"test-client-id",
		"test-client-secret",
		"test-agent/1.0",
		authServer.URL,
		"client_credentials",
		logger,
		mockClock,
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	// Pre-populate auth cache
	_, _, err = authenticator.GetToken(context.Background())
	if err != nil {
		b.Fatalf("failed to pre-populate auth cache: %v", err)
	}

	// Create internal HTTP client
	internalHTTPClient, err := client.NewClientWithRateLimit(
		httpClient,
		serverURL,
		"test-agent/1.0",
		logger,
		client.RateLimitConfig{
			RequestsPerMinute:  100000, // Very high to effectively disable rate limiting
			Burst:              10,
			ProactiveThreshold: 10,
		},
		mockClock,
	)
	if err != nil {
		b.Fatalf("failed to create HTTP client: %v", err)
	}

	// Create Reddit client
	return &Reddit{
		httpClient: internalHTTPClient,
		auth:       authenticator,
		config: &Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			UserAgent:    "test-agent/1.0",
			BaseURL:      serverURL,
			AuthURL:      authServer.URL,
			HTTPClient:   httpClient,
			Logger:       logger,
		},
		parser:     parse.NewParser(logger),
		validator:  validatorpkg.NewValidator(),
		tokenCache: cache.NewMemoryCache(mockClock, logger),
	}
}
