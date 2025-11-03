package graw

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// Integration benchmarks for the Reddit client measuring real-world performance
// across authentication, HTTP requests, parsing, and response handling.
//
// These benchmarks use actual test fixtures from benchmarks/testdata/:
//   - small_posts.json (10 posts, ~17KB)
//   - medium_posts.json (100 posts, ~138KB)
//   - large_posts.json (1000 posts, ~1.4MB)
//   - deep_comments.json (51 comments, max depth)
//   - wide_comments.json (600 comments, breadth-first)
//
// All benchmarks use MockClock for deterministic timing and report allocations
// to track memory efficiency. They simulate complete client operations from
// authentication through parsing.

// BenchmarkReddit_GetHot measures complete GetHot() operation performance
// including authentication, HTTP round-trip, and JSON parsing.
func BenchmarkReddit_GetHot(b *testing.B) {
	tests := []struct {
		name         string
		fixture      string
		expectedSize int64
	}{
		{"small_10posts", "small_posts.json", 17000},
		{"medium_100posts", "medium_posts.json", 138000},
		{"large_1000posts", "large_posts.json", 1400000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(tt.expectedSize)

			fixture := loadFixture(b, tt.fixture)
			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				if err != nil {
					b.Fatalf("GetHot failed: %v", err)
				}
				_ = resp
			}
		})
	}
}

// BenchmarkReddit_GetHot_Pagination benchmarks GetHot with different pagination parameters.
func BenchmarkReddit_GetHot_Pagination(b *testing.B) {
	tests := []struct {
		name   string
		limit  int
		after  string
		before string
	}{
		{"no_pagination", 25, "", ""},
		{"limit_10", 10, "", ""},
		{"limit_100", 100, "", ""},
		{"with_after", 25, "t3_abc123", ""},
		{"with_before", 25, "", "t3_xyz789"},
	}

	fixture := loadFixture(b, "medium_posts.json")

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit:  tt.limit,
						After:  tt.after,
						Before: tt.before,
					},
				})
				if err != nil {
					b.Fatalf("GetHot failed: %v", err)
				}
				_ = resp
			}
		})
	}
}

// BenchmarkReddit_GetNew measures complete GetNew() operation performance.
func BenchmarkReddit_GetNew(b *testing.B) {
	tests := []struct {
		name         string
		fixture      string
		expectedSize int64
	}{
		{"small_10posts", "small_posts.json", 17000},
		{"medium_100posts", "medium_posts.json", 138000},
		{"large_1000posts", "large_posts.json", 1400000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(tt.expectedSize)

			fixture := loadFixture(b, tt.fixture)
			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := redditClient.GetNew(ctx, &types.PostsRequest{
					Subreddit: "golang",
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				if err != nil {
					b.Fatalf("GetNew failed: %v", err)
				}
				_ = resp
			}
		})
	}
}

// BenchmarkReddit_GetComments measures single post comment fetching performance.
func BenchmarkReddit_GetComments(b *testing.B) {
	tests := []struct {
		name         string
		fixture      string
		expectedSize int64
		description  string
	}{
		{"deep_comments_51levels", "deep_comments.json", 50000, "max depth recursion"},
		{"wide_comments_600total", "wide_comments.json", 300000, "breadth-first wide tree"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(tt.expectedSize)

			fixture := loadFixture(b, tt.fixture)
			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := redditClient.GetComments(ctx, &types.CommentsRequest{
					Subreddit: "golang",
					PostID:    "abc123",
					Pagination: types.Pagination{
						Limit: 100,
					},
				})
				if err != nil {
					b.Fatalf("GetComments failed: %v", err)
				}
				_ = resp
			}
		})
	}
}

// BenchmarkReddit_GetCommentsMultiple measures concurrent comment fetching performance.
func BenchmarkReddit_GetCommentsMultiple(b *testing.B) {
	tests := []struct {
		name         string
		requestCount int
		fixture      string
	}{
		{"1_post", 1, "deep_comments.json"},
		{"5_posts", 5, "deep_comments.json"},
		{"10_posts", 10, "deep_comments.json"},
		{"25_posts", 25, "wide_comments.json"},
		{"50_posts", 50, "wide_comments.json"},
		{"100_posts", 100, "wide_comments.json"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			fixture := loadFixture(b, tt.fixture)
			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			// Pre-create requests to avoid allocation overhead in benchmark
			requests := make([]*types.CommentsRequest, tt.requestCount)
			for i := 0; i < tt.requestCount; i++ {
				requests[i] = &types.CommentsRequest{
					Subreddit: "golang",
					PostID:    "abc123",
					Pagination: types.Pagination{
						Limit: 100,
					},
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				responses, err := redditClient.GetCommentsMultiple(ctx, requests)
				if err != nil {
					b.Fatalf("GetCommentsMultiple failed: %v", err)
				}
				_ = responses
			}
		})
	}
}

// BenchmarkReddit_GetSubreddit measures subreddit info fetching performance.
func BenchmarkReddit_GetSubreddit(b *testing.B) {
	b.ReportAllocs()

	// Create a minimal subreddit response
	subredditResponse := map[string]interface{}{
		"kind": "t5",
		"data": map[string]interface{}{
			"id":                 "2rc7j",
			"name":               "t5_2rc7j",
			"display_name":       "golang",
			"title":              "The Go Programming Language",
			"public_description": "Ask questions and post articles about the Go programming language and related tools, events etc.",
			"subscribers":        500000,
			"active_user_count":  1500,
			"created":            1292625108.0,
			"created_utc":        1292625108.0,
			"over18":             false,
			"lang":               "en",
		},
	}
	fixture, _ := json.Marshal(subredditResponse)

	server := setupMockRedditServer(fixture, map[string]string{
		"X-Ratelimit-Remaining": "60",
		"X-Ratelimit-Reset":     "60",
	})
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := redditClient.GetSubreddit(ctx, "golang")
		if err != nil {
			b.Fatalf("GetSubreddit failed: %v", err)
		}
		_ = resp
	}
}

// BenchmarkReddit_Me measures authenticated user info fetching performance.
func BenchmarkReddit_Me(b *testing.B) {
	b.ReportAllocs()

	// Create a minimal account response with Thing wrapper
	accountResponse := map[string]interface{}{
		"kind": "t2",
		"data": map[string]interface{}{
			"id":                 "user123",
			"name":               "t2_user123",
			"created":            1292625108.0,
			"created_utc":        1292625108.0,
			"link_karma":         10000,
			"comment_karma":      5000,
			"is_gold":            false,
			"is_mod":             false,
			"has_verified_email": true,
		},
	}
	fixture, _ := json.Marshal(accountResponse)

	server := setupMockRedditServer(fixture, map[string]string{
		"X-Ratelimit-Remaining": "60",
		"X-Ratelimit-Reset":     "60",
	})
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := redditClient.Me(ctx)
		if err != nil {
			b.Fatalf("Me failed: %v", err)
		}
		_ = resp
	}
}

// BenchmarkReddit_EndToEnd_LargeResponse measures stress test with large responses.
func BenchmarkReddit_EndToEnd_LargeResponse(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1400000) // ~1.4MB

	fixture := loadFixture(b, "large_posts.json")
	server := setupMockRedditServer(fixture, map[string]string{
		"X-Ratelimit-Remaining": "60",
		"X-Ratelimit-Reset":     "60",
	})
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 100,
			},
		})
		if err != nil {
			b.Fatalf("GetHot failed: %v", err)
		}
		// Access data to ensure full parsing
		for _, post := range resp.Posts {
			_ = post.Title
			_ = post.Author
			_ = post.Score
		}
	}
}

// BenchmarkReddit_EndToEnd_DeepComments measures recursion stress with deep comment trees.
func BenchmarkReddit_EndToEnd_DeepComments(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(50000)

	fixture := loadFixture(b, "deep_comments.json")
	server := setupMockRedditServer(fixture, map[string]string{
		"X-Ratelimit-Remaining": "60",
		"X-Ratelimit-Reset":     "60",
	})
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := redditClient.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    "abc123",
			Pagination: types.Pagination{
				Limit: 100,
			},
		})
		if err != nil {
			b.Fatalf("GetComments failed: %v", err)
		}
		// Access comments to ensure full parsing
		for _, comment := range resp.Comments {
			_ = comment.Body
			_ = comment.Author
			_ = comment.ParentID
		}
	}
}

// BenchmarkReddit_EndToEnd_Pagination measures real-world pagination workflow.
func BenchmarkReddit_EndToEnd_Pagination(b *testing.B) {
	b.ReportAllocs()

	fixture := loadFixture(b, "medium_posts.json")
	server := setupMockRedditServer(fixture, map[string]string{
		"X-Ratelimit-Remaining": "60",
		"X-Ratelimit-Reset":     "60",
	})
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate fetching 3 pages
		var after string
		for page := 0; page < 3; page++ {
			resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
				Subreddit: "golang",
				Pagination: types.Pagination{
					Limit: 25,
					After: after,
				},
			})
			if err != nil {
				b.Fatalf("GetHot page %d failed: %v", page, err)
			}
			after = resp.AfterFullname
			if after == "" {
				break
			}
		}
	}
}

// BenchmarkReddit_EndToEnd_MixedOperations measures realistic mixed operation workflow.
func BenchmarkReddit_EndToEnd_MixedOperations(b *testing.B) {
	b.ReportAllocs()

	postsFixture := loadFixture(b, "small_posts.json")
	commentsFixture := loadFixture(b, "deep_comments.json")

	// Create a server that handles multiple endpoints
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// Route based on path
		if r.URL.Path == "/r/golang/hot" || r.URL.Path == "/r/golang/new" {
			w.Write(postsFixture)
		} else if r.URL.Path == "/r/golang/comments/abc123" {
			w.Write(commentsFixture)
		} else {
			w.Write([]byte(`{"kind":"Listing","data":{"children":[]}}`))
		}
	}))
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Fetch hot posts
		hotResp, err := redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit:  "golang",
			Pagination: types.Pagination{Limit: 10},
		})
		if err != nil {
			b.Fatalf("GetHot failed: %v", err)
		}

		// Fetch new posts
		newResp, err := redditClient.GetNew(ctx, &types.PostsRequest{
			Subreddit:  "golang",
			Pagination: types.Pagination{Limit: 10},
		})
		if err != nil {
			b.Fatalf("GetNew failed: %v", err)
		}

		// Fetch comments for first post
		if len(hotResp.Posts) > 0 {
			_, err := redditClient.GetComments(ctx, &types.CommentsRequest{
				Subreddit:  "golang",
				PostID:     "abc123",
				Pagination: types.Pagination{Limit: 50},
			})
			if err != nil {
				b.Fatalf("GetComments failed: %v", err)
			}
		}

		_ = newResp
	}
}

// BenchmarkReddit_Auth_TokenCached measures overhead when token is already cached.
func BenchmarkReddit_Auth_TokenCached(b *testing.B) {
	b.ReportAllocs()

	fixture := loadFixture(b, "small_posts.json")
	server := setupMockRedditServer(fixture, map[string]string{
		"X-Ratelimit-Remaining": "60",
		"X-Ratelimit-Reset":     "60",
	})
	defer server.Close()

	redditClient := createTestRedditClient(b, server.URL)
	ctx := context.Background()

	// Pre-warm token cache by making one request
	_, err := redditClient.GetHot(ctx, &types.PostsRequest{
		Subreddit:  "golang",
		Pagination: types.Pagination{Limit: 10},
	})
	if err != nil {
		b.Fatalf("Pre-warm request failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
			Subreddit:  "golang",
			Pagination: types.Pagination{Limit: 10},
		})
		if err != nil {
			b.Fatalf("GetHot failed: %v", err)
		}
		_ = resp
	}
}

// BenchmarkReddit_RateLimitHeaders measures overhead of rate limit header processing.
func BenchmarkReddit_RateLimitHeaders(b *testing.B) {
	tests := []struct {
		name      string
		remaining string
		reset     string
	}{
		{"plenty_remaining", "60", "60"},
		{"low_remaining", "5", "30"},
		{"very_low", "2", "10"},
		{"exhausted", "0", "60"},
	}

	fixture := loadFixture(b, "small_posts.json")

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": tt.remaining,
				"X-Ratelimit-Reset":     tt.reset,
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := redditClient.GetHot(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: 10},
				})
				if err != nil {
					b.Fatalf("GetHot failed: %v", err)
				}
				_ = resp
			}
		})
	}
}

// BenchmarkReddit_ValidationOverhead measures input validation performance.
func BenchmarkReddit_ValidationOverhead(b *testing.B) {
	tests := []struct {
		name      string
		operation string
	}{
		{"subreddit_name", "subreddit"},
		{"post_id", "post"},
		{"pagination", "pagination"},
	}

	fixture := loadFixture(b, "small_posts.json")

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				switch tt.operation {
				case "subreddit":
					_, err := redditClient.GetHot(ctx, &types.PostsRequest{
						Subreddit: "golang",
					})
					if err != nil {
						b.Fatalf("GetHot failed: %v", err)
					}
				case "post":
					_, err := redditClient.GetComments(ctx, &types.CommentsRequest{
						Subreddit: "golang",
						PostID:    "abc123",
					})
					if err != nil {
						b.Fatalf("GetComments failed: %v", err)
					}
				case "pagination":
					_, err := redditClient.GetHot(ctx, &types.PostsRequest{
						Subreddit: "golang",
						Pagination: types.Pagination{
							Limit: 25,
							After: "t3_abc123",
						},
					})
					if err != nil {
						b.Fatalf("GetHot failed: %v", err)
					}
				}
			}
		})
	}
}

// BenchmarkReddit_MemoryFootprint measures memory allocation patterns for different operations.
func BenchmarkReddit_MemoryFootprint(b *testing.B) {
	tests := []struct {
		name    string
		fixture string
		op      func(client *Reddit, ctx context.Context) error
	}{
		{
			"small_posts",
			"small_posts.json",
			func(client *Reddit, ctx context.Context) error {
				_, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: 10},
				})
				return err
			},
		},
		{
			"medium_posts",
			"medium_posts.json",
			func(client *Reddit, ctx context.Context) error {
				_, err := client.GetHot(ctx, &types.PostsRequest{
					Subreddit:  "golang",
					Pagination: types.Pagination{Limit: 100},
				})
				return err
			},
		},
		{
			"deep_comments",
			"deep_comments.json",
			func(client *Reddit, ctx context.Context) error {
				_, err := client.GetComments(ctx, &types.CommentsRequest{
					Subreddit:  "golang",
					PostID:     "abc123",
					Pagination: types.Pagination{Limit: 100},
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			fixture := loadFixture(b, tt.fixture)
			server := setupMockRedditServer(fixture, map[string]string{
				"X-Ratelimit-Remaining": "60",
				"X-Ratelimit-Reset":     "60",
			})
			defer server.Close()

			redditClient := createTestRedditClient(b, server.URL)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := tt.op(redditClient, ctx); err != nil {
					b.Fatalf("Operation failed: %v", err)
				}
			}
		})
	}
}

// Helper functions

// loadFixture loads a JSON fixture file from benchmarks/testdata/.
func loadFixture(b *testing.B, filename string) []byte {
	b.Helper()

	// Get the project root directory
	wd, err := os.Getwd()
	if err != nil {
		b.Fatalf("failed to get working directory: %v", err)
	}

	// Construct path to testdata
	fixturePath := filepath.Join(wd, "..", "benchmarks", "testdata", filename)
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		b.Fatalf("failed to load fixture %s: %v", filename, err)
	}

	return data
}

// setupMockRedditServer creates an httptest server that serves fixture data with realistic headers.
func setupMockRedditServer(fixture []byte, headers map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set content type
		w.Header().Set("Content-Type", "application/json")

		// Set provided headers (rate limits, etc.)
		for key, value := range headers {
			w.Header().Set(key, value)
		}

		// Write fixture data
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
}

// createTestRedditClient creates a Reddit client configured for benchmarking.
// Uses MockClock for deterministic timing and discard logger to minimize overhead.
func createTestRedditClient(b *testing.B, serverURL string) *Reddit {
	b.Helper()

	// Create mock clock for deterministic timing
	mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	// Create discard logger to minimize logging overhead
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create authenticator with mock clock
	// We use a mock auth server that always returns a valid token
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "test-token-12345", "token_type": "bearer", "expires_in": 3600}`))
	}))
	b.Cleanup(func() { authServer.Close() })

	testCache := cache.NewMemoryCache(mockClock)
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
		testCache,
	)
	if err != nil {
		b.Fatalf("failed to create authenticator: %v", err)
	}

	// Pre-populate auth cache to avoid auth overhead in benchmarks
	// Call GetToken once to cache the token
	_, err = authenticator.GetToken(context.Background())
	if err != nil {
		b.Fatalf("failed to pre-populate auth cache: %v", err)
	}

	// Create internal HTTP client with mock clock
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

	// Create Reddit client with all components
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
		parser:    parse.NewParser(logger),
		validator: validatorpkg.NewValidator(),
	}
}
