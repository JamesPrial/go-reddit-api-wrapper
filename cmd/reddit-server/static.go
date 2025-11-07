package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

// StaticHandler returns an http.Handler that serves embedded static files.
// It wraps the file server with debug logging for each static file request.
// If the static directory cannot be embedded, it returns a handler that responds
// with a 500 Internal Server Error.
func StaticHandler(logger *slog.Logger) http.Handler {
	// Create sub-filesystem for "static" directory
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		logger.Error("failed to create static filesystem", "error", err)
		// Return handler that returns 500
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		})
	}

	// Create file server
	fileServer := http.FileServer(http.FS(staticFS))

	// Wrap with logging, security headers, and cache control
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET and HEAD for static files
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		logger.Debug("serving static file",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)

		// Set security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' cdn.jsdelivr.net;")

		// Set explicit MIME types for assets
		if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}

		// Set cache headers based on content type
		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" {
			// Don't cache HTML
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		} else if strings.HasSuffix(r.URL.Path, ".css") || strings.HasSuffix(r.URL.Path, ".js") {
			// Cache assets for 1 hour
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}

		fileServer.ServeHTTP(w, r)
	})
}
