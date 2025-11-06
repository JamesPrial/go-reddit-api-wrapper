package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns middleware that recovers from panics in handler functions.
// If a panic occurs, it logs the panic with stack trace and returns a 500 error response.
// This prevents the server from crashing due to unhandled panics.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					logger.Error(
						"panic recovered",
						slog.Any("panic", err),
						slog.String("stack", string(debug.Stack())),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
					)

					// Return error response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, `{"error":{"message":"Internal server error","type":"server_error","code":500}}`)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
