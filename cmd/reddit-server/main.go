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
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/auth"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/handlers"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/middleware"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/monitor"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

func main() {
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	// Load and validate configuration first
	cfg, generatedKey, err := config.Load()
	if err != nil {
		// Use temporary logger for config load errors
		tempLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		tempLogger.Error("failed to load configuration", "error", err)
		exitCode = 1
		return
	}

	if err := cfg.Validate(); err != nil {
		// Use temporary logger for validation errors
		tempLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		tempLogger.Error("invalid configuration", "error", err)
		exitCode = 1
		return
	}

	// Create logger from configuration with multi-writer support
	logger, logFile, err := createLoggerFromConfig(cfg)
	if err != nil {
		// Use temporary logger for logger creation errors
		tempLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		tempLogger.Error("failed to create logger", "error", err)
		exitCode = 1
		return
	}
	defer func() {
		if logFile != nil {
			// Sync to ensure all data is written to disk
			if err := logFile.Sync(); err != nil {
				fmt.Fprintf(os.Stderr, "error syncing log file: %v\n", err)
			}
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing log file: %v\n", err)
			}
		}
	}()
	slog.SetDefault(logger)

	logger.Info("server configuration loaded", "config", cfg.String())

	// Log generated API key if one was created
	if generatedKey != "" {
		logger.Warn("API key auto-generated - SAVE THIS SECURELY", "api_key", generatedKey)
	}

	// Create Reddit API client
	redditClient, err := createRedditClient(cfg)
	if err != nil {
		logger.Error("failed to create Reddit client", "error", err)
		exitCode = 1
		return
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
		exitCode = 1
		return
	}

	logger.Info("storage initialized successfully", "dsn", cfg.StorageDSN)

	// Create shutdown trigger channel
	shutdownTrigger := make(chan struct{}, 1)

	// Create HTTP handlers
	h := handlers.NewHandlers(redditClient, store, shutdownTrigger)

	// Create monitor manager
	monitorMgr := monitor.NewMonitorManager(redditClient, store, logger)
	h.SetMonitorManager(monitorMgr)
	logger.Info("monitor manager created")

	// Initialize authentication system if enabled
	var jwtService handlers.JWTService
	var authHandlers *handlers.AuthHandlers
	if cfg.Auth != nil && cfg.Auth.Enabled {
		// Create user store from configuration
		users := make([]*auth.User, len(cfg.Auth.Users))
		for i, userCfg := range cfg.Auth.Users {
			users[i] = &auth.User{
				Username:     userCfg.Username,
				PasswordHash: userCfg.PasswordHash,
				Role:         userCfg.Role,
				CreatedAt:    time.Now(),
			}
		}
		authUserStore := auth.NewInMemoryUserStore(users)

		// Create JWT service
		jwtSvc, err := auth.NewJWTService(cfg.Auth.JWTSecret, "reddit-server")
		if err != nil {
			logger.Error("failed to create JWT service", "error", err)
			exitCode = 1
			return
		}

		// Use adapters to implement handlers interfaces
		handlersUserStore := auth.NewHandlersUserStore(authUserStore)
		jwtService = auth.NewHandlersJWTService(jwtSvc)

		// Create auth handlers with configured token expiry
		authHandlers = handlers.NewAuthHandlers(handlersUserStore, jwtService, logger, cfg.Auth.TokenExpiry)

		logger.Info("authentication system initialized",
			"user_count", len(cfg.Auth.Users),
			"token_expiry", cfg.Auth.TokenExpiry,
		)
	} else {
		logger.Debug("authentication system disabled")
	}

	// Setup HTTP router
	mux := http.NewServeMux()

	// Register auth routes if authentication is enabled
	if authHandlers != nil {
		mux.HandleFunc("/api/v1/auth/login", authHandlers.Login)
		mux.HandleFunc("/api/v1/auth/logout", authHandlers.Logout)
		mux.HandleFunc("/api/v1/auth/refresh", authHandlers.Refresh)
		mux.HandleFunc("/api/v1/auth/status", authHandlers.Status)
	}

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

	// Monitor endpoints
	mux.HandleFunc("/api/v1/monitor/start", h.StartMonitor)
	mux.HandleFunc("/api/v1/monitor/stop", h.StopMonitor)
	mux.HandleFunc("/api/v1/monitor/status", h.GetMonitorStatus)

	// Server endpoints
	mux.HandleFunc("/api/v1/server/shutdown", h.Shutdown)

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

	// Apply middleware stack in order (innermost to outermost):
	// 1. Recovery - catches panics and returns 500 errors
	// 2. Logging - logs all requests and responses
	// 3. CORS - handles cross-origin requests
	// 4. JWT Auth (if enabled) - primary authentication (validates JWT tokens)
	// 5. API Key - fallback authentication (for programmatic access)
	//
	// Authentication priority:
	// - If JWT token is present and valid, request is authenticated
	// - If JWT is missing/invalid, falls through to API key check
	// - If API key is valid, request is authenticated
	// - If both fail, request is rejected with 401
	//
	// Exempt paths bypass authentication:
	// - /health: Health check endpoint
	// - /: Root redirect
	// - /app/: Static files and frontend application
	// - /api/v1/auth/login: Login endpoint (needs anonymous access)
	// - /api/v1/auth/logout: Logout endpoint (accessible with valid JWT)
	var handler http.Handler = mux
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.Logging(logger)(handler)
	handler = middleware.CORS(cfg.AllowedOrigins)(handler)

	// Authentication: Load either JWT or API key middleware (not both)
	exemptPaths := []string{
		"/health",
		"/",
		"/app/",
		"/api/v1/auth/login",
		"/api/v1/auth/logout",
	}

	if jwtService != nil {
		// JWT authentication mode - use JWT tokens only
		jwtValidator := &jwtValidatorAdapter{jwtService: jwtService}
		handler = middleware.JWTAuth(jwtValidator, exemptPaths)(handler)
	} else {
		// API key authentication mode - use API keys only
		handler = middleware.APIKey(cfg.APIKeys, exemptPaths)(handler)
	}

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
		jwtAuthEnabled := cfg.Auth != nil && cfg.Auth.Enabled
		apiKeyAuthEnabled := len(cfg.APIKeys) > 0
		logger.Info("starting HTTP server",
			"addr", addr,
			"port", cfg.Port,
			"jwt_auth_enabled", jwtAuthEnabled,
			"api_key_auth_enabled", apiKeyAuthEnabled,
			"api_keys_count", len(cfg.APIKeys),
		)
		serverErrors <- srv.ListenAndServe()
	}()

	// Setup signal handling for graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	// Wait for either server error, shutdown signal, or API shutdown trigger
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			exitCode = 1
			return
		}

	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig.String())
		exitCode = performGracefulShutdown(context.Background(), logger, srv, monitorMgr, store, cfg.ShutdownTimeout)

	case <-shutdownTrigger:
		logger.Info("shutdown initiated via API endpoint")
		signal.Stop(shutdown)
		exitCode = performGracefulShutdown(context.Background(), logger, srv, monitorMgr, store, cfg.ShutdownTimeout)
	}
}

// performGracefulShutdown orchestrates the graceful shutdown sequence for the server.
// It stops active monitors, shuts down the HTTP server with the given timeout,
// and closes the storage layer. Returns an exit code: 0 for success, 1 for errors.
func performGracefulShutdown(ctx context.Context, logger *slog.Logger, srv *http.Server, monitorMgr *monitor.MonitorManager, store storage.Store, shutdownTimeout time.Duration) int {
	// Create context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	// Stop active monitors
	logger.Info("stopping active monitors")
	if err := monitorMgr.Stop(); err != nil {
		// Only log if error is not "no monitor running"
		if !errors.Is(err, monitor.ErrNoMonitorRunning) {
			logger.Error("error stopping monitor", "error", err)
		}
	}

	// Attempt graceful shutdown
	logger.Info("shutting down server", "timeout", shutdownTimeout)
	if err := srv.Shutdown(shutdownCtx); err != nil {
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
		return 1
	}

	logger.Info("server shutdown complete, closing storage")
	if err := store.Close(); err != nil {
		logger.Error("error closing storage", "error", err)
		return 1
	}

	logger.Info("storage closed successfully")
	return 0
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

// createLoggerFromConfig creates a structured logger from the server configuration.
// It supports configurable log levels and formats, with optional file output in addition to stderr.
// Returns the logger, the file handle (or nil), and an error if creation fails.
// The caller is responsible for closing the returned file handle.
func createLoggerFromConfig(cfg *config.Config) (*slog.Logger, *os.File, error) {
	// Map log level string to slog.Level
	logLevel := slog.LevelInfo // default
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	// Create multi-writer for logging (stderr + optional file)
	// Use stderr for consistency with early startup error logging
	var logWriters []io.Writer
	logWriters = append(logWriters, os.Stderr)

	var logFile *os.File
	if cfg.LogFile != "" {
		var err error
		// Use 0600 permissions for security (owner read/write only, consistent with directory 0o700)
		logFile, err = os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file %q: %w", cfg.LogFile, err)
		}
		logWriters = append(logWriters, logFile)
	}

	multiWriter := io.MultiWriter(logWriters...)

	// Create handler based on configured format
	var handler slog.Handler
	switch cfg.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
			Level: logLevel,
		})
	case "text":
		handler = slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
			Level: logLevel,
		})
	default:
		// Default to JSON if unknown format (should not happen due to validation)
		handler = slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
			Level: logLevel,
		})
	}

	logger := slog.New(handler)
	return logger, logFile, nil
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

// jwtValidatorAdapter adapts handlers.JWTService to middleware.JWTAuthValidator interface
// by converting the return type from *handlers.UserData to interface{}.
type jwtValidatorAdapter struct {
	jwtService handlers.JWTService
}

// ValidateToken validates a JWT token and returns nil for valid tokens, error otherwise.
// This implements the middleware.JWTAuthValidator interface.
func (a *jwtValidatorAdapter) ValidateToken(tokenString string) (interface{}, error) {
	// JWTService.ValidateToken returns (*handlers.UserData, error)
	// We need to return (interface{}, error) for the middleware
	userData, err := a.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	return userData, nil
}
