// Package main is the entry point for the Reddit HTTP server.
// It starts an HTTP server that exposes Reddit API endpoints as REST API.
//
// Configuration:
//   - SERVER_PORT: HTTP server port (default: 8080)
//   - SERVER_READ_TIMEOUT: Read timeout in seconds (default: 15)
//   - SERVER_WRITE_TIMEOUT: Write timeout in seconds (default: 15)
//   - SERVER_IDLE_TIMEOUT: Idle timeout in seconds (default: 60)
//   - REDDIT_CLIENT_ID: OAuth2 client ID (required)
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret (required)
//   - REDDIT_USERNAME: Optional username for user authentication
//   - REDDIT_PASSWORD: Optional password for user authentication
//   - REDDIT_USER_AGENT: Optional custom user agent string (default: reddit-server/1.0)
//   - CORS_ALLOWED_ORIGINS: Comma-separated allowed origins (default: *)
//   - API_KEYS: Comma-separated list of valid API keys (optional - auto-generated if not provided)
//
// The server provides the following endpoints:
//   - GET /health: Health check (no authentication required)
//   - GET /api/v1/user/me: Current user information (requires API key)
//   - GET /api/v1/subreddit/{name}: Subreddit information (requires API key)
//   - GET /api/v1/posts/hot: Hot posts (requires API key)
//   - GET /api/v1/posts/new: New posts (requires API key)
//   - GET /api/v1/posts/{subreddit}/{postID}/comments: Post comments (requires API key)
//   - POST /api/v1/posts/{linkID}/more-comments: Load more comments (requires API key, max 100 comment IDs per request)
//
// All endpoints except /health require:
// - Authentication via REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET environment variables
// - An API key via X-API-Key header or Authorization: Bearer <key> header
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/handlers"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/middleware"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

func main() {
	// Parse command-line flags
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	helpFlag := flag.Bool("help", false, "Print help message")
	flag.Parse()

	if *helpFlag {
		printHelp()
		os.Exit(0)
	}

	// Create logger
	level := slog.LevelInfo
	if *debugFlag {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	// Load configuration from environment
	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("Configuration loaded",
		slog.Int("port", cfg.Server.Port),
		slog.Duration("read_timeout", cfg.Server.ReadTimeout),
		slog.Duration("write_timeout", cfg.Server.WriteTimeout),
	)

	// Log generated API key if one was auto-generated
	if cfg.Auth.GeneratedKey != "" {
		logger.Warn("No API_KEYS environment variable provided - generated a random API key for this session")
		logger.Warn("IMPORTANT: Use this API key for all requests to secure endpoints", slog.String("api_key", cfg.Auth.GeneratedKey))
		logger.Warn("To use a persistent API key, set the API_KEYS environment variable before starting the server")
	}

	// Create Reddit client (used to validate credentials on startup)
	redditCfg := &graw.Config{
		ClientID:     cfg.Reddit.ClientID,
		ClientSecret: cfg.Reddit.ClientSecret,
		Username:     cfg.Reddit.Username,
		Password:     cfg.Reddit.Password,
		UserAgent:    cfg.Reddit.UserAgent,
	}

	client, err := graw.NewClient(redditCfg)
	if err != nil {
		logger.Error("failed to create Reddit client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create handler
	handler := handlers.New(logger, client)

	// Create router with API key authentication
	router := handler.Router(cfg.CORS, cfg.Auth.APIKeys)

	// Add global middleware
	// Note: middleware is applied in reverse order
	r := router
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logging(logger))
	r.Use(middleware.AuthFromConfig(&cfg.Reddit))

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: 10 * time.Second, // Slowloris protection
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    1 << 20, // 1MB header size limit
	}

	// Start server in a goroutine
	go func() {
		logger.Info("starting server", slog.String("address", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown with timeout
	logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("server stopped")
}

// printHelp prints usage information.
func printHelp() {
	fmt.Fprintf(os.Stderr, `Reddit HTTP Server

An HTTP server that exposes Reddit API endpoints as REST API.

Usage:
  reddit-server [flags]

Flags:
  -debug       Enable debug logging
  -help        Print this help message

Environment Variables:
  SERVER_PORT              HTTP server port (default: 8080)
  SERVER_READ_TIMEOUT      Read timeout in seconds (default: 15)
  SERVER_WRITE_TIMEOUT     Write timeout in seconds (default: 15)
  SERVER_IDLE_TIMEOUT      Idle timeout in seconds (default: 60)

  REDDIT_CLIENT_ID         OAuth2 client ID (required)
  REDDIT_CLIENT_SECRET     OAuth2 client secret (required)
  REDDIT_USERNAME          Optional username for user authentication
  REDDIT_PASSWORD          Optional password for user authentication
  REDDIT_USER_AGENT        Optional custom user agent string

  API_KEYS                 Comma-separated list of valid API keys (optional)
                           If not provided, a random key is auto-generated
                           Example: API_KEYS="key1,key2,key3"

  CORS_ALLOWED_ORIGINS     Comma-separated allowed origins (default: *)
  CORS_ALLOWED_METHODS     Comma-separated allowed methods (default: GET,OPTIONS)
  CORS_ALLOWED_HEADERS     Comma-separated allowed headers (default: Content-Type,Authorization)
  CORS_MAX_AGE             Preflight cache max age in seconds (default: 300)

Authentication:
  All API endpoints except /health require an API key. Provide the key via:
  - X-API-Key header: curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/user/me
  - Authorization header: curl -H "Authorization: Bearer your-api-key" http://localhost:8080/api/v1/user/me

Examples:
  # Start server with default configuration (auto-generates API key)
  reddit-server

  # Start server with debug logging
  reddit-server -debug

  # Start server with custom port
  SERVER_PORT=3000 reddit-server

  # Start server with custom Reddit credentials
  REDDIT_CLIENT_ID=your-id \
  REDDIT_CLIENT_SECRET=your-secret \
  reddit-server

  # Start server with persistent API key(s)
  API_KEYS="my-secret-key" \
  REDDIT_CLIENT_ID=your-id \
  REDDIT_CLIENT_SECRET=your-secret \
  reddit-server

  # Generate a secure API key (example)
  openssl rand -base64 32

Endpoints:
  GET    /health                                  Health check
  GET    /api/v1/user/me                         Current user info
  GET    /api/v1/subreddit/{name}                Subreddit info
  GET    /api/v1/posts/hot                       Hot posts
  GET    /api/v1/posts/new                       New posts
  GET    /api/v1/posts/{subreddit}/{postID}/comments
                                                  Post comments
  POST   /api/v1/posts/{linkID}/more-comments    Load more comments

`)
}
