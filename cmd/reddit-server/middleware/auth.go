// Package middleware provides HTTP middleware for request processing.
package middleware

import (
	"context"
	"net/http"

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
