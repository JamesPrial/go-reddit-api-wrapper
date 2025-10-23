package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

// Benchmarks for HTTP client focusing on memory allocations and throughput.
// These benchmarks measure performance-critical operations to ensure the client
// remains efficient even under high load.

// BenchmarkBufferPool_GetPut measures the performance of buffer pool operations.
// This is critical as buffer pooling is used for every HTTP response to reduce allocations.
func BenchmarkBufferPool_GetPut(b *testing.B) {
	tests := []struct {
		name string
		size int
	}{
		{"small 1KB", 1024},
		{"medium 10KB", 10 * 1024},
		{"large 100KB", 100 * 1024},
		{"max 256KB", 256 * 1024},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(tt.size))

			// Pre-allocate test data to avoid allocation overhead in benchmark
			testData := make([]byte, tt.size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf := getBuffer()
				// Simulate writing data to the buffer
				buf.Grow(tt.size)
				buf.Write(testData)
				putBuffer(buf)
			}
		})
	}
}

// BenchmarkBufferPool_Concurrent measures concurrent buffer pool access from multiple goroutines.
// This tests the thread-safety and performance of the sync.Pool implementation.
func BenchmarkBufferPool_Concurrent(b *testing.B) {
	tests := []struct {
		name       string
		size       int
		goroutines int
	}{
		{"small/2g", 1024, 2},
		{"small/10g", 1024, 10},
		{"medium/2g", 10 * 1024, 2},
		{"medium/10g", 10 * 1024, 10},
		{"large/2g", 100 * 1024, 2},
		{"large/10g", 100 * 1024, 10},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(tt.size))

			// Pre-allocate test data to avoid allocation overhead in benchmark
			testData := make([]byte, tt.size)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					buf := getBuffer()
					buf.Grow(tt.size)
					buf.Write(testData)
					putBuffer(buf)
				}
			})
		})
	}
}

// BenchmarkRateLimit_Wait measures the overhead of checking rate limit state.
// This is called for every request and must be efficient.
// Note: We measure the no-wait path (when limiter is nil) since the actual wait
// uses golang's rate.Limiter which cannot be meaningfully benchmarked with mock time.
func BenchmarkRateLimit_Wait(b *testing.B) {
	b.ReportAllocs()

	mockClock := clock.NewMockClock(time.Now())
	// Create client without rate limiter to measure fast path
	client, err := NewClientWithRateLimit(
		nil,
		"https://example.com",
		"test-agent",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		RateLimitConfig{},
		mockClock,
	)
	if err != nil {
		b.Fatalf("failed to create client: %v", err)
	}
	// Set limiter to nil to test the fast path (no rate limiting)
	client.limiter = nil

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This tests the overhead of the rate limit check itself
		if err := client.waitForRateLimit(ctx); err != nil {
			b.Fatalf("rate limit wait failed: %v", err)
		}
	}
}

// BenchmarkRateLimit_ApplyHeaders measures the cost of parsing and applying rate limit headers.
// This happens on every response and must be fast to avoid adding latency.
func BenchmarkRateLimit_ApplyHeaders(b *testing.B) {
	tests := []struct {
		name      string
		remaining string
		reset     string
	}{
		{"plenty remaining", "60", "60"},
		{"low remaining", "5", "30"},
		{"very low", "1", "10"},
		{"exhausted", "0", "60"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			mockClock := clock.NewMockClock(time.Now())
			client, err := NewClientWithRateLimit(
				nil,
				"https://example.com",
				"test-agent",
				slog.New(slog.NewTextHandler(io.Discard, nil)),
				RateLimitConfig{},
				mockClock,
			)
			if err != nil {
				b.Fatalf("failed to create client: %v", err)
			}

			// Create a response with rate limit headers
			resp := &http.Response{
				Header: http.Header{
					"X-Ratelimit-Remaining": []string{tt.remaining},
					"X-Ratelimit-Reset":     []string{tt.reset},
				},
				Request: &http.Request{},
			}
			resp.Request = resp.Request.WithContext(context.Background())

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				client.applyRateHeaders(resp)
			}
		})
	}
}

// BenchmarkClient_NewRequest measures the cost of creating HTTP requests.
// This is called for every API operation and must minimize allocations.
func BenchmarkClient_NewRequest(b *testing.B) {
	tests := []struct {
		name       string
		path       string
		withParams bool
		paramCount int
	}{
		{"simple path", "/r/golang/hot", false, 0},
		{"with subreddit", "/r/programming/new", false, 0},
		{"with params 3", "/r/golang/hot", true, 3},
		{"with params 10", "/r/golang/top", true, 10},
		{"long path", "/r/verylongsubredditname/comments/abc123xyz/very_long_post_title_slug/", false, 0},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			mockClock := clock.NewMockClock(time.Now())
			client, err := NewClientWithRateLimit(
				nil,
				"https://oauth.reddit.com",
				"test-agent",
				slog.New(slog.NewTextHandler(io.Discard, nil)),
				RateLimitConfig{},
				mockClock,
			)
			if err != nil {
				b.Fatalf("failed to create client: %v", err)
			}

			ctx := context.Background()

			var params url.Values
			if tt.withParams {
				params = make(url.Values)
				for i := 0; i < tt.paramCount; i++ {
					params.Add(fmt.Sprintf("param%d", i), fmt.Sprintf("value%d", i))
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req, err := client.NewRequest(ctx, "GET", tt.path, nil, params)
				if err != nil {
					b.Fatalf("failed to create request: %v", err)
				}
				_ = req
			}
		})
	}
}

// BenchmarkClient_DoRequest measures full request/response cycle performance.
// This includes rate limiting, HTTP round-trip, response reading, and buffer pooling.
func BenchmarkClient_DoRequest(b *testing.B) {
	tests := []struct {
		name         string
		responseSize int
		makeResponse func(size int) []byte
	}{
		{"empty response", 0, makeEmptyListing},
		{"1KB listing", 1024, makeListingResponse},
		{"10KB listing", 10 * 1024, makeListingResponse},
		{"100KB listing", 100 * 1024, makeListingResponse},
		{"1MB listing", 1024 * 1024, makeListingResponse},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(tt.responseSize))

			// Create a test server that returns the desired response size
			responseBody := tt.makeResponse(tt.responseSize)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Ratelimit-Remaining", "60")
				w.Header().Set("X-Ratelimit-Reset", "60")
				w.WriteHeader(http.StatusOK)
				w.Write(responseBody)
			}))
			defer server.Close()

			mockClock := clock.NewMockClock(time.Now())
			client, err := NewClientWithRateLimit(
				server.Client(),
				server.URL,
				"test-agent",
				slog.New(slog.NewTextHandler(io.Discard, nil)),
				RateLimitConfig{},
				mockClock,
			)
			if err != nil {
				b.Fatalf("failed to create client: %v", err)
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req, err := client.NewRequest(ctx, "GET", "/r/golang/hot", nil)
				if err != nil {
					b.Fatalf("failed to create request: %v", err)
				}

				bodyBytes, _, err := client.doRequest(req)
				if err != nil {
					b.Fatalf("request failed: %v", err)
				}
				_ = bodyBytes
			}
		})
	}
}

// BenchmarkResponseBody_Read measures the performance of reading response bodies into buffers.
// This tests the buffer pooling and io.Copy operations used for every response.
func BenchmarkResponseBody_Read(b *testing.B) {
	tests := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(tt.size))

			// Pre-generate test data
			testData := make([]byte, tt.size)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf := getBuffer()
				reader := bytes.NewReader(testData)
				limitedReader := io.LimitReader(reader, MAX_RESPONSE_BODY_SIZE)

				_, err := io.Copy(buf, limitedReader)
				if err != nil {
					b.Fatalf("copy failed: %v", err)
				}

				putBuffer(buf)
			}
		})
	}
}

// BenchmarkJSON_Unmarshal_Thing benchmarks unmarshaling different types of Reddit Things.
// JSON parsing is a major performance concern for API clients.
func BenchmarkJSON_Unmarshal_Thing(b *testing.B) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			"simple post",
			[]byte(`{"kind":"t3","data":{"id":"abc123","name":"t3_abc123","title":"Test Post","author":"testuser","score":100,"subreddit":"golang","created_utc":1609459200.0}}`),
		},
		{
			"complex post with media",
			makeComplexPost(),
		},
		{
			"comment",
			[]byte(`{"kind":"t1","data":{"id":"def456","name":"t1_def456","body":"Test comment body","author":"testuser","score":50,"link_id":"t3_abc123","parent_id":"t3_abc123","created_utc":1609459200.0}}`),
		},
		{
			"listing with 10 posts",
			makeListingWithPosts(10),
		},
		{
			"listing with 100 posts",
			makeListingWithPosts(100),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tt.data)))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var thing types.Thing
				if err := json.Unmarshal(tt.data, &thing); err != nil {
					b.Fatalf("unmarshal failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkExtractAPIErrorDetails benchmarks the error detail extraction from API responses.
// This happens on every error response and should be efficient.
func BenchmarkExtractAPIErrorDetails(b *testing.B) {
	tests := []struct {
		name string
		body []byte
	}{
		{
			"simple error",
			[]byte(`{"error":"NOT_FOUND","message":"Subreddit not found"}`),
		},
		{
			"nested error",
			[]byte(`{"json":{"errors":[["RATELIMIT","You are doing that too much. Try again in 5 minutes.","5"]]}}`),
		},
		{
			"complex error with details",
			[]byte(`{"error":404,"message":"Not found","reason":"The requested resource does not exist","details":{"resource":"post","id":"abc123"}}`),
		},
		{
			"minimal error",
			[]byte(`{"error":"GENERIC_ERROR"}`),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tt.body)))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				code, message, details := extractAPIErrorDetails(tt.body)
				_, _, _ = code, message, details
			}
		})
	}
}

// BenchmarkTruncateBody benchmarks the body truncation used in error messages.
// This happens frequently during error handling and logging.
func BenchmarkTruncateBody(b *testing.B) {
	tests := []struct {
		name   string
		body   []byte
		maxLen int
	}{
		{"short body no truncate", []byte("short body"), 100},
		{"exact length", bytes.Repeat([]byte("x"), 200), 200},
		{"needs truncate 1KB", bytes.Repeat([]byte("x"), 1024), 200},
		{"needs truncate 10KB", bytes.Repeat([]byte("x"), 10*1024), 200},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result := truncateBody(tt.body, tt.maxLen)
				_ = result
			}
		})
	}
}

// BenchmarkBuildLimiter benchmarks the rate limiter construction.
// This happens once per client but should still be efficient.
func BenchmarkBuildLimiter(b *testing.B) {
	tests := []struct {
		name string
		cfg  RateLimitConfig
	}{
		{"default config", RateLimitConfig{}},
		{"custom config", RateLimitConfig{RequestsPerMinute: 60, Burst: 10}},
		{"high burst", RateLimitConfig{RequestsPerMinute: 100, Burst: 50}},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				limiter := buildLimiter(tt.cfg)
				_ = limiter
			}
		})
	}
}

// BenchmarkDeferRequests benchmarks the proactive rate limiting delay mechanism.
// This is called when rate limit headers indicate we're approaching the limit.
func BenchmarkDeferRequests(b *testing.B) {
	tests := []struct {
		name     string
		duration time.Duration
		reason   string
	}{
		{"short delay 100ms", 100 * time.Millisecond, "proactive_ratelimit"},
		{"medium delay 1s", 1 * time.Second, "proactive_ratelimit"},
		{"long delay 1m", 1 * time.Minute, "ratelimit_exhausted"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()

			mockClock := clock.NewMockClock(time.Now())
			client, err := NewClientWithRateLimit(
				nil,
				"https://example.com",
				"test-agent",
				slog.New(slog.NewTextHandler(io.Discard, nil)),
				RateLimitConfig{},
				mockClock,
			)
			if err != nil {
				b.Fatalf("failed to create client: %v", err)
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// With MockClock, this operation is instant (no actual delay)
				client.deferRequests(ctx, tt.duration, tt.reason)
			}
		})
	}
}

// BenchmarkConcurrentRequests benchmarks multiple concurrent requests to test thread safety.
// This simulates realistic high-load scenarios with many goroutines making requests.
func BenchmarkConcurrentRequests(b *testing.B) {
	tests := []struct {
		name         string
		goroutines   int
		responseSize int
	}{
		{"2g/1KB", 2, 1024},
		{"10g/1KB", 10, 1024},
		{"50g/1KB", 50, 1024},
		{"10g/100KB", 10, 100 * 1024},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(tt.responseSize))

			// Create a test server
			responseBody := makeListingResponse(tt.responseSize)
			var requestCount int
			var mu sync.Mutex

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requestCount++
				mu.Unlock()

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Ratelimit-Remaining", "60")
				w.Header().Set("X-Ratelimit-Reset", "60")
				w.WriteHeader(http.StatusOK)
				w.Write(responseBody)
			}))
			defer server.Close()

			mockClock := clock.NewMockClock(time.Now())
			client, err := NewClientWithRateLimit(
				server.Client(),
				server.URL,
				"test-agent",
				slog.New(slog.NewTextHandler(io.Discard, nil)),
				RateLimitConfig{},
				mockClock,
			)
			if err != nil {
				b.Fatalf("failed to create client: %v", err)
			}

			ctx := context.Background()

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req, err := client.NewRequest(ctx, "GET", "/r/golang/hot", nil)
					if err != nil {
						b.Fatalf("failed to create request: %v", err)
					}

					_, _, err = client.doRequest(req)
					if err != nil {
						b.Fatalf("request failed: %v", err)
					}
				}
			})
		})
	}
}

// Helper functions for benchmark test data generation

// makeEmptyListing creates an empty Reddit listing response.
func makeEmptyListing(size int) []byte {
	return []byte(`{"kind":"Listing","data":{"children":[],"after":"","before":""}}`)
}

// makeListingResponse creates a Reddit listing response of approximately the target size.
func makeListingResponse(targetSize int) []byte {
	// Estimate posts needed (rough approximation based on typical post size)
	postCount := targetSize / 300
	if postCount < 1 {
		postCount = 1
	}
	return makeListingWithPosts(postCount)
}

// makeListingWithPosts creates a listing with the specified number of posts.
func makeListingWithPosts(count int) []byte {
	var buf bytes.Buffer
	buf.WriteString(`{"kind":"Listing","data":{"children":[`)

	for i := 0; i < count; i++ {
		if i > 0 {
			buf.WriteString(",")
		}
		fmt.Fprintf(&buf, `{"kind":"t3","data":{"id":"post%d","name":"t3_post%d","title":"Test Post %d","author":"user%d","score":%d,"subreddit":"golang","created_utc":1609459200.0}}`,
			i, i, i, i%100, (i*13)%1000)
	}

	buf.WriteString(`],"after":"","before":""}}`)
	return buf.Bytes()
}

// makeComplexPost creates a complex post with many fields for benchmarking.
func makeComplexPost() []byte {
	post := map[string]interface{}{
		"kind": "t3",
		"data": map[string]interface{}{
			"id":                     "abc123",
			"name":                   "t3_abc123",
			"title":                  "A Complex Post Title With Many Words For Testing Performance",
			"author":                 "testuser",
			"score":                  1500,
			"ups":                    1500,
			"downs":                  0,
			"subreddit":              "golang",
			"subreddit_id":           "t5_2rc7j",
			"created":                1609459200.0,
			"created_utc":            1609459200.0,
			"permalink":              "/r/golang/comments/abc123/a_complex_post_title_with_many_words_for_testing_performance/",
			"url":                    "https://example.com/article",
			"domain":                 "example.com",
			"num_comments":           150,
			"is_self":                false,
			"selftext":               "",
			"thumbnail":              "https://example.com/thumb.jpg",
			"over_18":                false,
			"locked":                 false,
			"stickied":               false,
			"distinguished":          nil,
			"author_flair_text":      "Gopher",
			"author_flair_css_class": "gopher-blue",
			"link_flair_text":        "Question",
			"link_flair_css_class":   "question",
			"media": map[string]interface{}{
				"type": "video",
				"oembed": map[string]interface{}{
					"provider_name": "YouTube",
					"title":         "Video Title",
					"html":          "<iframe src='https://example.com/embed'></iframe>",
				},
			},
		},
	}

	data, _ := json.Marshal(post)
	return data
}
