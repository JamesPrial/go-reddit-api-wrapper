package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/api"
	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
	"github.com/jamesprial/go-reddit-api-wrapper/internal/services"
	"github.com/jamesprial/go-reddit-api-wrapper/internal/websocket"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

func main() {
	// Configure JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}))
	slog.SetDefault(logger)

	logger.Info("starting reddit tracker application")

	// Initialize database
	dbPath := getEnv("DB_PATH", "/data/reddit.db")
	logger.Info("initializing database", "path", dbPath)

	database, err := db.InitDB(db.Config{
		Path:        dbPath,
		EnableDebug: getEnv("DEBUG", "false") == "true",
	})
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Get underlying SQL database for connection management
	sqlDB, err := database.DB()
	if err != nil {
		logger.Error("failed to get SQL database", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	// Create repository
	repo := db.NewRepository(database)

	// Create WebSocket hub
	hub := websocket.NewHub(logger)
	go hub.Run()

	// Create Reddit client
	redditConfig := &graw.Config{
		ClientID:     mustGetEnv("REDDIT_CLIENT_ID"),
		ClientSecret: mustGetEnv("REDDIT_CLIENT_SECRET"),
		Username:     getEnv("REDDIT_USERNAME", ""),
		Password:     getEnv("REDDIT_PASSWORD", ""),
		UserAgent:    getEnv("USER_AGENT", "reddit-tracker/1.0"),
	}

	logger.Info("creating reddit client",
		"client_id", redditConfig.ClientID,
		"has_username", redditConfig.Username != "")

	redditClient, err := graw.NewClient(redditConfig)
	if err != nil {
		logger.Error("failed to create reddit client", "error", err)
		os.Exit(1)
	}

	// Parse poll interval
	pollIntervalStr := getEnv("POLL_INTERVAL", "60s")
	pollInterval, err := time.ParseDuration(pollIntervalStr)
	if err != nil {
		logger.Error("invalid poll interval", "value", pollIntervalStr, "error", err)
		os.Exit(1)
	}

	// Create polling service
	logger.Info("creating polling service",
		"interval", pollInterval,
		"workers", 10)

	poller, err := services.NewPoller(services.Config{
		RedditClient: redditClient,
		Repository:   repo,
		PollInterval: pollInterval,
		WorkerCount:  10,
		Logger:       logger,
	})
	if err != nil {
		logger.Error("failed to create poller", "error", err)
		os.Exit(1)
	}

	// Create context for coordinating shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start polling service
	go func() {
		logger.Info("starting polling service")
		if err := poller.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("poller error", "error", err)
		}
	}()

	// Create HTTP router
	port := getEnv("PORT", "8080")
	corsOrigins := getEnv("CORS_ORIGINS", "*")
	enableCORS := getEnv("ENABLE_CORS", "true") == "true"

	// Create API handlers
	handlers := api.NewHandlers(repo, logger)

	// Create router and register all routes
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	})

	// WebSocket endpoint
	mux.HandleFunc("/ws", websocket.Handler(hub))

	// API endpoints
	mux.HandleFunc("/api/subreddits", handlers.CreateSubreddit)
	mux.HandleFunc("/api/subreddits/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Check if this is a list or get by checking path
			if r.URL.Path == "/api/subreddits" || r.URL.Path == "/api/subreddits/" {
				handlers.ListSubreddits(w, r)
			} else {
				handlers.GetSubreddit(w, r)
			}
		case "POST":
			handlers.CreateSubreddit(w, r)
		case "DELETE":
			handlers.DeleteSubreddit(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/posts", handlers.ListPosts)
	mux.HandleFunc("/api/posts/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/posts" || r.URL.Path == "/api/posts/" {
			handlers.ListPosts(w, r)
		} else {
			handlers.GetPost(w, r)
		}
	})

	mux.HandleFunc("/api/comments/", handlers.GetComment)

	// Apply middleware
	var handler http.Handler = mux

	// Add CORS if enabled
	if enableCORS {
		handler = api.CORS(corsOrigins)(handler)
	}

	// Add logging and recovery middleware
	handler = api.Logger(logger)(handler)
	handler = api.Recovery(logger)(handler)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle shutdown signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server",
			"port", port,
			"cors_enabled", enableCORS,
			"cors_origins", corsOrigins)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		logger.Info("shutdown signal received")
	case err := <-errChan:
		logger.Error("server error", "error", err)
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop polling service
	cancel()
	logger.Info("stopping poller")
	if err := poller.Stop(); err != nil {
		logger.Error("error stopping poller", "error", err)
	}

	// Shutdown WebSocket hub
	logger.Info("shutting down websocket hub")
	if err := hub.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down hub", "error", err)
	}

	// Shutdown HTTP server
	logger.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("error shutting down server", "error", err)
	}

	logger.Info("shutdown complete")
}

// getEnv retrieves an environment variable with a fallback default value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// mustGetEnv retrieves a required environment variable, panicking if not set
func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return value
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
