// Package middleware provides HTTP middleware for the Reddit API server.
package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code and response size.
type responseWriter struct {
	http.ResponseWriter
	statusCode    int
	size          int
	headerWritten bool
}

// newResponseWriter creates a new responseWriter that wraps the provided http.ResponseWriter.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default to 200 if WriteHeader is never called
		size:           0,
		headerWritten:  false,
	}
}

// WriteHeader captures the status code and calls the underlying WriteHeader.
// It prevents multiple calls to WriteHeader from propagating to the underlying writer.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.headerWritten {
		return
	}
	rw.statusCode = code
	rw.headerWritten = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size and calls the underlying Write.
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Flush implements http.Flusher interface.
// It propagates the Flush call to the underlying ResponseWriter if it supports flushing.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements http.Hijacker interface.
// It propagates the Hijack call to the underlying ResponseWriter if it supports hijacking.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

// Push implements http.Pusher interface.
// It propagates the Push call to the underlying ResponseWriter if it supports HTTP/2 server push.
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return errors.New("push not supported")
}

// Unwrap returns the underlying http.ResponseWriter.
// This is used by http.ResponseController to access the original ResponseWriter.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Logging returns middleware that logs HTTP requests using structured logging.
// It logs each request with method, path, remote address, duration, status code, and response size.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := newResponseWriter(w)

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Log the request after it completes
			duration := time.Since(start)
			logger.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"size_bytes", wrapped.size,
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
