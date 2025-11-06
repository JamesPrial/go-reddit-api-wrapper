package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	// Create logger with JSON output
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded successfully",
		"port", config.Port,
		"rate_limit", config.RateLimit,
		"rate_burst", config.RateBurst,
		"cors_origin", config.CORSOrigin,
		"use_user_auth", config.UseUserAuth,
	)

	// Create Reddit client
	reddit, err := config.CreateRedditClient()
	if err != nil {
		logger.Error("failed to create Reddit client", "error", err)
		os.Exit(1)
	}

	logger.Info("Reddit client initialized")

	// Create API server
	apiServer := NewServer(reddit, logger)

	// Create router with explicit pattern matching
	mux := http.NewServeMux()

	// Register endpoints
	mux.HandleFunc("GET /health", apiServer.HealthHandler)
	mux.HandleFunc("GET /api/v1/r/{subreddit}/hot", apiServer.GetHotHandler)
	mux.HandleFunc("GET /api/v1/r/{subreddit}/new", apiServer.GetNewHandler)
	mux.HandleFunc("GET /api/v1/r/{subreddit}/about", apiServer.GetSubredditHandler)
	mux.HandleFunc("GET /api/v1/r/{subreddit}/posts/{postId}/comments", apiServer.GetCommentsHandler)
	mux.HandleFunc("GET /api/v1/me", apiServer.GetMeHandler)

	logger.Info("routes registered")

	// Create rate limiter
	// Convert requests per second from rate limit (requests per second)
	rateLimiter := NewRateLimitMiddleware(
		float64(config.RateLimit),
		config.RateBurst,
		logger,
	)

	// Chain middleware in order of execution (innermost to outermost)
	// 1. Response headers (always set, closest to handler)
	// 2. Recovery (catch panics)
	// 3. Logging (log all requests)
	// 4. CORS (handle cross-origin requests)
	// 5. Rate limiting (limit requests)
	var handler http.Handler = mux
	handler = ResponseHeaderMiddleware(handler)
	handler = RecoveryMiddleware(logger)(handler)
	handler = LoggingMiddleware(logger)(handler)
	handler = CORSMiddleware(config.CORSOrigin)(handler)
	handler = rateLimiter.Middleware()(handler)

	logger.Info("middleware chain initialized")

	// Create HTTP server with configured port
	addr := ":" + strconv.Itoa(config.Port)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("HTTP server configured", "addr", addr)

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for interrupt signal or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	case sig := <-sigChan:
		logger.Info("received signal", "signal", sig)
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown server gracefully", "error", err)
		os.Exit(1)
	}

	logger.Info("server shutdown successfully")
}
