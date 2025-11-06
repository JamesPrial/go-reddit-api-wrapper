// Package middleware provides HTTP middleware for request processing.
package middleware

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
)

// Credentials represents extracted Reddit API credentials.
type Credentials struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	UserAgent    string
}

// credentialsKey is the context key for storing credentials.
type credentialsKey struct{}

// GetCredentials retrieves Reddit API credentials from the request context.
// Returns nil if credentials are not found in the context.
func GetCredentials(r *http.Request) *Credentials {
	creds, ok := r.Context().Value(credentialsKey{}).(*Credentials)
	if !ok {
		return nil
	}
	return creds
}

// SetCredentialsInContext returns a new request with the given credentials added to the context.
// This is useful for testing to simulate what the AuthFromConfig middleware does.
func SetCredentialsInContext(r *http.Request, creds *Credentials) *http.Request {
	ctx := context.WithValue(r.Context(), credentialsKey{}, creds)
	return r.WithContext(ctx)
}

// AuthFromConfig returns middleware that uses credentials from a Config object.
// This is useful for server configurations where credentials are loaded from environment
// variables once at startup.
func AuthFromConfig(cfg *config.Reddit) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds := &Credentials{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				Username:     cfg.Username,
				Password:     cfg.Password,
				UserAgent:    cfg.UserAgent,
			}

			// Store credentials in request context
			ctx := context.WithValue(r.Context(), credentialsKey{}, creds)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAPIKey returns middleware that validates incoming requests have a valid API key.
// It checks for the API key in two places (in order of preference):
//  1. X-API-Key header
//  2. Authorization: Bearer <key> header
//
// If no valid API key is found, it returns 401 Unauthorized with a JSON error response.
// The API key comparison uses constant-time comparison to prevent timing attacks.
func RequireAPIKey(apiKeys []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check X-API-Key header first
			apiKey := r.Header.Get("X-API-Key")

			// Check Authorization: Bearer <key> header if X-API-Key not present
			if apiKey == "" {
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					apiKey = strings.TrimPrefix(auth, "Bearer ")
				}
			}

			// Validate that API key is present
			if apiKey == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":{"message":"API key required. Provide via X-API-Key header or Authorization: Bearer header","type":"auth_error","code":401}}`)
				return
			}

			// Use constant-time comparison to prevent timing attacks
			valid := false
			for _, validKey := range apiKeys {
				if subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1 {
					valid = true
					break
				}
			}

			if !valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":{"message":"Invalid API key","type":"auth_error","code":401}}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
