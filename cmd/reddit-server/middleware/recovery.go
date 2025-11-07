package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// recoveryWriter wraps http.ResponseWriter to track if headers have been written.
type recoveryWriter struct {
	http.ResponseWriter
	headerWritten bool
}

// WriteHeader tracks that headers have been written and calls the underlying WriteHeader.
func (rw *recoveryWriter) WriteHeader(code int) {
	rw.headerWritten = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write tracks that headers have been written (implicitly) and calls the underlying Write.
func (rw *recoveryWriter) Write(b []byte) (int, error) {
	rw.headerWritten = true
	return rw.ResponseWriter.Write(b)
}

// Recovery returns middleware that recovers from panics in HTTP handlers.
// It logs the panic with a stack trace and returns a 500 Internal Server Error to the client.
// This prevents the entire server from crashing due to a panic in a single handler.
//
// The middleware properly handles panics that occur after headers have been sent by only
// logging the error and not attempting to send a 500 response (which would fail).
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wrap the ResponseWriter to track if headers have been written
			wrapped := &recoveryWriter{ResponseWriter: w, headerWritten: false}

			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					logger.Error("HTTP handler panic",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"remote_addr", r.RemoteAddr,
						"stack", string(debug.Stack()),
					)

					// Only send error response if headers haven't been written yet
					// If headers were already sent, we can't change the response
					if !wrapped.headerWritten {
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
				}
			}()

			// Call the next handler with the wrapped ResponseWriter
			next.ServeHTTP(wrapped, r)
		})
	}
}
