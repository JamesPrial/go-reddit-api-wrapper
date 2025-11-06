package main

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/reqid"
	"golang.org/x/time/rate"
)

// statusRecorder wraps http.ResponseWriter to capture the HTTP status code.
// This allows us to log the response status in the logging middleware.
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

// LoggingMiddleware logs incoming HTTP requests with status codes and duration.
// It generates a request ID and adds it to the context for propagation.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID and add to context if not already present
			ctx := reqid.Ensure(r.Context())
			requestID := reqid.FromContext(ctx)
			r = r.WithContext(ctx)

			start := time.Now()
			logger.Info("incoming request",
				"method", r.Method,
				"path", r.RequestURI,
				"remote_addr", r.RemoteAddr,
				slog.String("request_id", requestID),
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
				slog.String("request_id", requestID),
			)
		})
	}
}

// CORSMiddleware adds CORS headers to responses.
// It allows requests from the configured origin.
func CORSMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if allowedOrigin == "*" || origin == allowedOrigin {
				if allowedOrigin == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware implements per-client rate limiting using golang.org/x/time/rate.
// It uses the client's IP address to identify unique clients.
type RateLimitMiddleware struct {
	limiter *rate.Limiter
	logger  *slog.Logger
}

// NewRateLimitMiddleware creates a new rate limiting middleware.
func NewRateLimitMiddleware(requestsPerSecond float64, burst int, logger *slog.Logger) *RateLimitMiddleware {
	limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burst)
	return &RateLimitMiddleware{
		limiter: limiter,
		logger:  logger,
	}
}

// Middleware returns the rate limiting middleware handler.
func (rl *RateLimitMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestID := reqid.FromContext(ctx)

			if !rl.limiter.Allow() {
				rl.logger.Warn("rate limit exceeded",
					"remote_addr", r.RemoteAddr,
					slog.String("request_id", requestID),
				)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded","request_id":"` + requestID + `"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ResponseHeaderMiddleware adds consistent response headers.
func ResponseHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// RecoveryMiddleware recovers from panics and returns a 500 error.
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					ctx := r.Context()
					requestID := reqid.FromContext(ctx)
					logger.Error("panic recovered",
						"error", err,
						"method", r.Method,
						"path", r.RequestURI,
						slog.String("request_id", requestID),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"internal server error","request_id":"` + requestID + `"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request.
// It checks X-Forwarded-For header first (for proxied requests),
// then falls back to RemoteAddr.
func getClientIP(r *http.Request) string {
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	return r.RemoteAddr
}
