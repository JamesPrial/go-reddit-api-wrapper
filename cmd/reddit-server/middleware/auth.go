package middleware

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

var (
	// ErrMalformedAuthHeader is returned when the Authorization header does not start with "Bearer ".
	ErrMalformedAuthHeader = errors.New("authorization header must start with 'Bearer '")

	// ErrEmptyToken is returned when the Authorization header contains "Bearer " but no token.
	ErrEmptyToken = errors.New("bearer token is empty")
)

// errorResponse is the standard error response format.
type errorResponse struct {
	Error string `json:"error"`
}

// respondError sends a JSON error response with the specified status code.
// This is used by the API key middleware to respond with consistent error formatting.
func respondError(w http.ResponseWriter, status int, message string) {
	// Encode to buffer first to avoid race condition where WriteHeader is called
	// but encoding fails afterwards
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(errorResponse{Error: message}); err != nil {
		slog.Error("failed to encode JSON error response", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

// APIKey returns middleware that validates API key authentication from the Authorization header.
// It uses constant-time comparison to prevent timing attacks.
//
// The middleware:
//   - Extracts the API key from the Authorization header (expected format: "Bearer <api-key>")
//   - Validates the key against the provided keys slice using constant-time comparison
//   - Skips authentication for paths in the exemptPaths slice
//   - Returns 401 Unauthorized for missing, malformed, or invalid API keys
//   - Logs failed authentication attempts with structured logging
//
// Example usage:
//
//	allowedKeys := []string{"key1", "key2"}
//	exemptPaths := []string{"/health"}
//	handler := APIKey(allowedKeys, exemptPaths)(myHandler)
func APIKey(keys []string, exemptPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for exempt paths
			if isExemptPath(r.URL.Path, exemptPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")

			// Check if Authorization header is present
			if authHeader == "" {
				slog.Warn("missing authorization header",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				respondError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Parse Bearer token
			apiKey, err := parseBearerToken(authHeader)
			if err != nil {
				slog.Warn("invalid authorization header format",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"error", err.Error(),
				)
				respondError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Validate API key against allowed keys using constant-time comparison
			if !validateAPIKey(apiKey, keys) {
				slog.Warn("invalid API key",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				respondError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			// Key is valid, call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// parseBearerToken extracts the API key from an Authorization header.
// Expected format: "Bearer <api-key>"
// Returns an error if the header is malformed.
func parseBearerToken(authHeader string) (string, error) {
	const bearerPrefix = "Bearer "

	// Check if the header starts with "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", ErrMalformedAuthHeader
	}

	// Extract the token part
	token := strings.TrimPrefix(authHeader, bearerPrefix)

	// Ensure the token is not empty
	if token == "" {
		return "", ErrEmptyToken
	}

	return token, nil
}

// validateAPIKey checks if the provided key matches any of the allowed keys.
// It uses crypto/subtle.ConstantTimeCompare to prevent timing attacks.
func validateAPIKey(providedKey string, allowedKeys []string) bool {
	// If no allowed keys are configured, still perform constant-time comparison
	// against a dummy key to prevent timing differences from revealing configuration
	if len(allowedKeys) == 0 {
		dummyKey := make([]byte, len(providedKey))
		subtle.ConstantTimeCompare([]byte(providedKey), dummyKey)
		return false
	}

	providedKeyBytes := []byte(providedKey)

	for _, allowedKey := range allowedKeys {
		allowedKeyBytes := []byte(allowedKey)
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare(providedKeyBytes, allowedKeyBytes) == 1 {
			return true
		}
	}

	return false
}

// isExemptPath checks if the given path is in the exempt paths list.
// Comparison is exact match (strings equality).
func isExemptPath(path string, exemptPaths []string) bool {
	for _, exemptPath := range exemptPaths {
		if path == exemptPath {
			return true
		}
	}
	return false
}
