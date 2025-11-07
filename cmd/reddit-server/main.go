// Command reddit-server provides an HTTP server that exposes Reddit API operations as REST endpoints.
// It supports both application-only and user authentication modes via environment variables.
//
// Environment Variables:
//
//	Required:
//	  REDDIT_CLIENT_ID       - Reddit OAuth2 client ID
//	  REDDIT_CLIENT_SECRET   - Reddit OAuth2 client secret
//
//	Optional:
//	  PORT                   - HTTP server port (default: 8080)
//	  SHUTDOWN_TIMEOUT       - Graceful shutdown timeout (default: 30s, e.g., "45s", "1m")
//	  REQUEST_TIMEOUT        - HTTP request timeout (default: 30s)
//	  REDDIT_USERNAME        - Reddit username for user authentication
//	  REDDIT_PASSWORD        - Reddit password for user authentication
//	  REDDIT_USER_AGENT      - Custom user agent string
//	  ALLOWED_ORIGINS        - Comma-separated CORS allowed origins (e.g., "http://localhost:5173,https://example.com")
//
// API Endpoints:
//
//	GET  /health                                    - Health check (no auth required)
//	GET  /app/                                      - Web UI frontend (no auth required)
//	GET  /                                          - Redirect to /app/ (no auth required)
//	GET  /api/v1/user/me                            - Get authenticated user info (requires API key)
//	GET  /api/v1/subreddit/{name}                   - Get subreddit information (requires API key)
//	GET  /api/v1/posts/hot                          - Get hot posts (requires API key; query: subreddit, limit, after, before)
//	GET  /api/v1/posts/new                          - Get new posts (requires API key; query: subreddit, limit, after, before)
//	GET  /api/v1/posts/{subreddit}/{postID}/comments - Get post comments (requires API key; query: limit, after, before)
//	POST /api/v1/posts/{linkID}/more-comments       - Load more comments (requires API key; body: {"children": ["id1", "id2"]})
//
// Examples:
//
//	# Start server with required credentials
//	export REDDIT_CLIENT_ID="your-client-id"
//	export REDDIT_CLIENT_SECRET="your-client-secret"
//	./reddit-server
//
//	# Start with user authentication
//	export REDDIT_USERNAME="your-username"
//	export REDDIT_PASSWORD="your-password"
//	./reddit-server
//
//	# Start on custom port with CORS
//	export PORT=3000
//	export ALLOWED_ORIGINS="http://localhost:5173"
//	./reddit-server
//
//	# Test the server
//	curl http://localhost:8080/health
//	curl http://localhost:8080/api/v1/posts/hot?subreddit=golang&limit=10
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/handlers"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/middleware"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load and validate configuration
	cfg, generatedKey, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("server configuration loaded", "config", cfg.String())

	// Log generated API key if one was created
	if generatedKey != "" {
		logger.Warn("API key auto-generated - SAVE THIS SECURELY", "api_key", generatedKey)
	}

	// Create Reddit API client
	redditClient, err := createRedditClient(cfg)
	if err != nil {
		logger.Error("failed to create Reddit client", "error", err)
		os.Exit(1)
	}

	logger.Info("Reddit client created successfully")

	// Create storage
	storeCfg := storage.Config{
		DSN:             cfg.StorageDSN,
		MaxOpenConns:    cfg.StorageMaxOpenConns,
		MaxIdleConns:    cfg.StorageMaxIdleConns,
		ConnMaxLifetime: 0,
		Logger:          logger,
	}

	// Create storage with timeout
	initCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := storage.New(initCtx, storeCfg)
	if err != nil {
		logger.Error("failed to create storage", "error", err)
		os.Exit(1)
	}

	logger.Info("storage initialized successfully", "dsn", cfg.StorageDSN)

	// Create HTTP handlers
	h := handlers.NewHandlers(redditClient, store)

	// Setup HTTP router
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/api/v1/user/me", h.GetUserMe)
	mux.HandleFunc("/api/v1/subreddit/", h.GetSubreddit)
	mux.HandleFunc("/api/v1/posts/hot", h.GetHotPosts)
	mux.HandleFunc("/api/v1/posts/new", h.GetNewPosts)
	mux.HandleFunc("/api/v1/posts/", routePostsHandler(h)) // Routes to GetComments or GetMoreComments based on path

	// Storage routes
	mux.HandleFunc("/api/v1/storage/posts", routeStoragePosts(h))
	mux.HandleFunc("/api/v1/storage/stats", h.GetStorageStats)
	mux.HandleFunc("/api/v1/storage/bulk-save", h.BulkSaveFromSubreddit)

	// Register static file handler (serves frontend at /app/)
	mux.Handle("/app/", http.StripPrefix("/app/", StaticHandler(logger)))

	// Root handler: redirect to /app/ or return JSON 404 for unknown API paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
			return
		}
		// Unknown path - return JSON 404 for API paths, HTML 404 for others
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "endpoint not found"})
			return
		}
		http.NotFound(w, r)
	})

	// Apply middleware stack: APIKey → CORS → Logging → Recovery
	var handler http.Handler = mux
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.CORS(cfg.AllowedOrigins)(handler)
	handler = middleware.APIKey(cfg.APIKeys, []string{"/health", "/", "/app/"})(handler)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       cfg.RequestTimeout,
		WriteTimeout:      cfg.RequestTimeout,
		MaxHeaderBytes:    1 << 20, // 1MB
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", "addr", addr, "port", cfg.Port)
		serverErrors <- srv.ListenAndServe()
	}()

	// Setup signal handling for graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			signal.Stop(shutdown)
			os.Exit(1)
		}

	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig.String())

		// Create context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		// Attempt graceful shutdown
		logger.Info("shutting down server", "timeout", cfg.ShutdownTimeout)
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("error during shutdown", "error", err)
			// Force close after timeout
			if closeErr := srv.Close(); closeErr != nil {
				logger.Error("error closing server", "error", closeErr)
			}
			// Still attempt to close storage
			logger.Info("closing storage after forced shutdown")
			if storeErr := store.Close(); storeErr != nil {
				logger.Error("error closing storage during forced shutdown", "error", storeErr)
			}
			os.Exit(1)
		}

		logger.Info("server shutdown complete, closing storage")
		if err := store.Close(); err != nil {
			logger.Error("error closing storage", "error", err)
			os.Exit(1)
		}

		logger.Info("storage closed successfully")
	}
}

// routePostsHandler routes requests under /api/v1/posts/ to the appropriate handler
// based on the URL path pattern. It routes:
//   - /api/v1/posts/{subreddit}/{postID}/comments -> GetComments
//   - /api/v1/posts/{linkID}/more-comments -> GetMoreComments
func routePostsHandler(h *handlers.Handlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if the path ends with /more-comments
		if strings.HasSuffix(r.URL.Path, "/more-comments") {
			h.GetMoreComments(w, r)
			return
		}

		// Check if the path ends with /comments
		if strings.HasSuffix(r.URL.Path, "/comments") {
			h.GetComments(w, r)
			return
		}

		// No matching pattern
		http.NotFound(w, r)
	}
}

// routeStoragePosts routes requests under /api/v1/storage/posts to the appropriate handler
// based on the HTTP method and URL path pattern. It routes:
//   - GET  /api/v1/storage/posts -> ListSavedPosts
//   - POST /api/v1/storage/posts -> SavePost
//   - GET  /api/v1/storage/posts/{id} -> GetSavedPost
//   - DELETE /api/v1/storage/posts/{id} -> DeleteSavedPost
//   - GET  /api/v1/storage/posts/{id}/comments -> GetCommentTree
//   - POST /api/v1/storage/posts/{id}/comments -> SaveComments
func routeStoragePosts(h *handlers.Handlers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Remove trailing slashes to normalize
		path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/storage/posts"), "/")

		// Handle base path /api/v1/storage/posts
		if path == "" {
			switch r.Method {
			case http.MethodGet:
				h.ListSavedPosts(w, r)
			case http.MethodPost:
				h.SavePost(w, r)
			default:
				w.Header().Set("Allow", "GET, POST")
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

		// Validate no empty segments (prevents /posts//comments attacks)
		for _, part := range parts {
			if part == "" {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
		}

		// /api/v1/storage/posts/{id}/comments
		if len(parts) == 2 && parts[1] == "comments" {
			postID := parts[0]
			switch r.Method {
			case http.MethodPost:
				h.SaveComments(w, r, postID)
			case http.MethodGet:
				h.GetCommentTree(w, r, postID)
			default:
				w.Header().Set("Allow", "GET, POST")
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// /api/v1/storage/posts/{id}
		if len(parts) == 1 {
			postID := parts[0]
			switch r.Method {
			case http.MethodGet:
				h.GetSavedPost(w, r, postID)
			case http.MethodDelete:
				h.DeleteSavedPost(w, r, postID)
			default:
				w.Header().Set("Allow", "GET, DELETE")
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		// Invalid path
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

// createRedditClient creates and configures a Reddit API client from the server configuration.
// It supports both application-only authentication (client credentials) and user authentication
// (password grant) depending on whether username and password are provided.
func createRedditClient(cfg *config.Config) (*graw.Reddit, error) {
	redditCfg := &graw.Config{
		ClientID:     cfg.RedditClientID,
		ClientSecret: cfg.RedditClientSecret,
		Username:     cfg.RedditUsername,
		Password:     cfg.RedditPassword,
	}

	// Set user agent (required or default with hostname)
	if cfg.RedditUserAgent != "" {
		redditCfg.UserAgent = cfg.RedditUserAgent
	} else {
		// Use hostname for more specific default user agent
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		redditCfg.UserAgent = fmt.Sprintf("reddit-api-server/1.0 (host:%s)", hostname)
	}

	// Create the client
	client, err := graw.NewClient(redditCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reddit client: %w", err)
	}

	return client, nil
}
