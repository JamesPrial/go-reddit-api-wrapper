package internal

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func BenchmarkClient_Do_WithLogging(b *testing.B) {
	// Mock server that returns a simple JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "3600")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind":"t2","data":{"name":"test","id":"123"}}`))
	}))
	defer server.Close()

	// Create client WITH logging
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	tokenProvider := &mockTokenProvider{token: "test-token"}
	client, _ := NewClient(http.DefaultClient, tokenProvider, server.URL, "bench/1.0", logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := client.NewRequest(ctx, http.MethodGet, "/api/v1/me", nil)
		var thing types.Thing
		client.Do(req, &thing)
	}
}

func BenchmarkClient_Do_WithLoggingDebug(b *testing.B) {
	// Mock server that returns a larger JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "3600")
		w.WriteHeader(http.StatusOK)
		// Simulate a larger response body
		body := bytes.Repeat([]byte(`{"kind":"listing","data":{"children":[]}}`), 100)
		w.Write(body)
	}))
	defer server.Close()

	// Create client with DEBUG logging (includes body logging)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tokenProvider := &mockTokenProvider{token: "test-token"}
	client, _ := NewClient(http.DefaultClient, tokenProvider, server.URL, "bench/1.0", logger)
	client.SetLogBodyLimit(8 * 1024)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := client.NewRequest(ctx, http.MethodGet, "/api/v1/me", nil)
		var thing types.Thing
		client.Do(req, &thing)
	}
}

func BenchmarkClient_Do_WithoutLogging(b *testing.B) {
	// Mock server that returns a simple JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "3600")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind":"t2","data":{"name":"test","id":"123"}}`))
	}))
	defer server.Close()

	// Create client WITHOUT logging (nil logger)
	tokenProvider := &mockTokenProvider{token: "test-token"}
	client, _ := NewClient(http.DefaultClient, tokenProvider, server.URL, "bench/1.0", nil)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := client.NewRequest(ctx, http.MethodGet, "/api/v1/me", nil)
		var thing types.Thing
		client.Do(req, &thing)
	}
}
