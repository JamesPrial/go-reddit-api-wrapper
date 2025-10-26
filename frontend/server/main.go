package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
// This allows us to log the response status code in the logging middleware.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code before writing it to the response.
func (sr *statusRecorder) WriteHeader(code int) {
	if !sr.written {
		sr.statusCode = code
		sr.written = true
	}
	sr.ResponseWriter.WriteHeader(code)
}

// Write ensures WriteHeader is called before writing body content.
func (sr *statusRecorder) Write(b []byte) (int, error) {
	if !sr.written {
		sr.statusCode = http.StatusOK
		sr.written = true
	}
	return sr.ResponseWriter.Write(b)
}

// CORSMiddleware adds CORS headers to responses.
// It allows requests from http://localhost:5173 (Vite default).
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Allow both localhost:5173 and localhost:3000 for flexibility
		if origin == "http://localhost:5173" || origin == "http://localhost:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs incoming HTTP requests with status codes and duration.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logger.Info("incoming request",
				"method", r.Method,
				"path", r.RequestURI,
				"remote_addr", r.RemoteAddr,
			)

			// Wrap response writer to capture status code
			recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			logger.Info("request completed",
				"method", r.Method,
				"path", r.RequestURI,
				"status_code", recorder.statusCode,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

// main starts the HTTP server and handles graceful shutdown.
func main() {
	// Create logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create session manager (initialize JWT secret from environment)
	sessionManager, err := NewSessionManager()
	if err != nil {
		logger.Error("failed to create session manager", "error", err)
		os.Exit(1)
	}

	// Create handler
	handler := NewHandler(sessionManager, logger)

	// Create router
	mux := http.NewServeMux()

	// Register authentication routes
	mux.HandleFunc("/api/auth/login", handler.LoginHandler)
	mux.HandleFunc("/api/auth/status", handler.StatusHandler)
	mux.HandleFunc("/api/auth/logout", handler.LogoutHandler)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Apply middleware
	var muxWithMiddleware http.Handler = mux
	muxWithMiddleware = CORSMiddleware(muxWithMiddleware)
	muxWithMiddleware = LoggingMiddleware(logger)(muxWithMiddleware)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      muxWithMiddleware,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
	logger.Info("shutting down server...")

	// Stop session cleanup goroutine
	sessionManager.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown server gracefully", "error", err)
		os.Exit(1)
	}

	logger.Info("server shutdown successfully")
}
