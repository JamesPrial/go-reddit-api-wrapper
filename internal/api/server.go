package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
)

// Config contains configuration for the API server.
type Config struct {
	// Port is the port number to listen on (e.g., "8080").
	Port string

	// Repository is the database repository for data access.
	Repository db.Repository

	// EnableCORS enables Cross-Origin Resource Sharing support.
	// When true, CORS headers are added to allow browser-based API access.
	EnableCORS bool

	// CORSOrigins specifies which origins are allowed for CORS requests.
	// Use "*" to allow all origins (development only).
	// Defaults to "*" if EnableCORS is true.
	CORSOrigins string

	// LogLevel sets the logging verbosity.
	// Use slog.LevelDebug for detailed logs, slog.LevelInfo for normal operation.
	LogLevel slog.Level

	// ReadTimeout is the maximum duration for reading the entire request.
	// Defaults to 15 seconds.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	// Defaults to 15 seconds.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request.
	// Defaults to 60 seconds.
	IdleTimeout time.Duration
}

// Server represents the HTTP API server.
// It manages the HTTP server lifecycle and routing.
type Server struct {
	config   Config
	handlers *Handlers
	logger   *slog.Logger
	srv      *http.Server
}

// NewServer creates a new API server with the provided configuration.
// It initializes the router, handlers, and middleware.
func NewServer(cfg Config) *Server {
	// Set defaults
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.CORSOrigins == "" {
		cfg.CORSOrigins = "*"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 15 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 60 * time.Second
	}

	// Create logger
	logger := slog.New(slog.NewJSONHandler(nil, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	// Create handlers
	handlers := NewHandlers(cfg.Repository, logger)

	s := &Server{
		config:   cfg,
		handlers: handlers,
		logger:   logger,
	}

	// Create HTTP server
	s.srv = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      s.setupRoutes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// setupRoutes configures the HTTP router with all endpoints and middleware.
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Register routes
	// Subreddit endpoints
	mux.HandleFunc("/api/subreddits", s.routeSubreddits)

	// Post endpoints
	mux.HandleFunc("/api/posts", s.routePosts)

	// Comment endpoints
	mux.HandleFunc("/api/comments/", s.routeComments)

	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)

	// Apply middleware (in reverse order of execution)
	var handler http.Handler = mux

	// Recovery middleware (innermost, catches panics)
	handler = Recovery(s.logger)(handler)

	// Logger middleware
	handler = Logger(s.logger)(handler)

	// CORS middleware (outermost, processes first)
	if s.config.EnableCORS {
		handler = CORS(s.config.CORSOrigins)(handler)
	}

	return handler
}

// routeSubreddits routes /api/subreddits requests based on method and path.
func (s *Server) routeSubreddits(w http.ResponseWriter, r *http.Request) {
	// Parse path to determine if this is a collection or item request
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	// /api/subreddits (collection)
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			s.handlers.ListSubreddits(w, r)
		case http.MethodPost:
			s.handlers.CreateSubreddit(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		}
		return
	}

	// /api/subreddits/:name (item)
	if len(parts) == 3 {
		switch r.Method {
		case http.MethodGet:
			s.handlers.GetSubreddit(w, r)
		case http.MethodDelete:
			s.handlers.DeleteSubreddit(w, r)
		default:
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		}
		return
	}

	WriteNotFound(w, "Endpoint not found")
}

// routePosts routes /api/posts requests based on method and path.
func (s *Server) routePosts(w http.ResponseWriter, r *http.Request) {
	// Parse path
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	// /api/posts (collection with query params)
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		s.handlers.ListPosts(w, r)
		return
	}

	// /api/posts/:fullname (item)
	if len(parts) == 3 {
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		s.handlers.GetPost(w, r)
		return
	}

	// /api/posts/:fullname/comments (nested resource)
	if len(parts) == 4 && parts[3] == "comments" {
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		s.handlers.GetPostComments(w, r)
		return
	}

	WriteNotFound(w, "Endpoint not found")
}

// routeComments routes /api/comments requests.
func (s *Server) routeComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	s.handlers.GetComment(w, r)
}

// handleHealth handles the /health endpoint for health checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	WriteSuccess(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// Start starts the HTTP server and blocks until the context is cancelled.
// It supports graceful shutdown, waiting for active connections to complete
// before shutting down (up to 30 seconds).
func (s *Server) Start(ctx context.Context) error {
	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("starting API server",
			"port", s.config.Port,
			"cors_enabled", s.config.EnableCORS)

		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Info("shutting down API server")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}

		s.logger.Info("API server stopped gracefully")
		return ctx.Err()

	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server without interrupting active connections.
// It waits for active connections to complete for up to 30 seconds.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down API server")
	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}
	s.logger.Info("API server stopped")
	return nil
}
