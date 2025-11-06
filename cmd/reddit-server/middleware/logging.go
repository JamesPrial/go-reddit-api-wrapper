package middleware

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code and response size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size.
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Logging returns middleware that logs HTTP requests with slog.
// Logs include:
//   - Request method and path
//   - Response status code
//   - Duration in milliseconds
//   - Response size in bytes
//   - Request errors (if any)
//
// Requests to /health are logged at Debug level, others at Info level.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer to capture status and size
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default to 200
				size:           0,
			}

			// Call next handler
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(start)

			// Skip logging /health checks at full level (log at debug)
			level := slog.LevelInfo
			if r.URL.Path == "/health" {
				level = slog.LevelDebug
			}

			// Log the request
			logger.LogAttrs(r.Context(), level,
				fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				slog.Int("status", rw.statusCode),
				slog.Duration("duration", duration),
				slog.Int("size", rw.size),
			)
		})
	}
}

// LoggingWithWriter returns middleware that logs HTTP requests to a provided writer.
// This is useful for testing or custom logging configurations.
func LoggingWithWriter(w io.Writer) func(http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(w, nil))
	return Logging(logger)
}
