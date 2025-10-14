package client

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

func BenchmarkClient_Do_WithLogging(b *testing.B) {
	// Setup: Create mock server with simple response using testutil
	account := testutil.NewAccount("testuser").Build()
	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Create client WITH INFO level logging
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	client, _ := NewClient(http.DefaultClient, server.URL(), "bench/1.0", logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := client.NewRequest(ctx, http.MethodGet, "/api/v1/me", nil)
		var thing types.Thing
		client.Do(req, &thing)
	}
}

func BenchmarkClient_Do_WithLoggingDebug(b *testing.B) {
	// Setup: Create posts for larger response body to test body logging overhead
	posts := make([]*types.Post, 100)
	for i := 0; i < 100; i++ {
		posts[i] = testutil.NewPostBuilder().Build()
	}

	server := testutil.NewMockServer().
		WithPosts("golang", "hot", posts...).
		Start()
	defer server.Close()

	// Create client with DEBUG logging (includes body logging)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client, _ := NewClient(http.DefaultClient, server.URL(), "bench/1.0", logger)
	client.SetLogBodyLimit(8 * 1024)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := client.NewRequest(ctx, http.MethodGet, "/r/golang/hot", nil)
		var thing types.Thing
		client.Do(req, &thing)
	}
}

func BenchmarkClient_Do_WithoutLogging(b *testing.B) {
	// Setup: Create mock server with simple response using testutil
	account := testutil.NewAccount("testuser").Build()
	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Create client WITHOUT logging (nil logger)
	client, _ := NewClient(http.DefaultClient, server.URL(), "bench/1.0", nil)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := client.NewRequest(ctx, http.MethodGet, "/api/v1/me", nil)
		var thing types.Thing
		client.Do(req, &thing)
	}
}
