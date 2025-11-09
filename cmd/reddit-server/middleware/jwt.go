// Package middleware provides HTTP middleware for the Reddit API server.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// contextKey is a type for context values to avoid collisions.
type contextKey string

const (
	// userContextKey is the context key for user claims.
	userContextKey contextKey = "user"
	// usernameContextKey is the context key for username.
	usernameContextKey contextKey = "username"
	// roleContextKey is the context key for user role.
	roleContextKey contextKey = "role"
)

var (
	// ErrMissingToken is returned when Authorization header is missing.
	ErrMissingToken = errors.New("missing authorization token")
	// ErrInvalidTokenFormat is returned when Authorization header format is invalid.
	ErrInvalidTokenFormat = errors.New("invalid token format")
	// ErrInvalidToken is returned when token validation fails.
	ErrInvalidToken = errors.New("invalid token")
	// ErrInsufficientRole is returned when user role is insufficient.
	ErrInsufficientRole = errors.New("insufficient role permissions")
)

// UserClaims represents the claims extracted from a JWT token.
type UserClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// RBACJWTService defines the interface for JWT token validation with role-based access control.
// This is different from the basic JWTService in auth.go and returns structured UserClaims
// with role information for use with the RoleRequired middleware.
type RBACJWTService interface {
	// ValidateToken validates a JWT token and returns the claims with role information.
	// Returns an error if the token is invalid or expired.
	ValidateToken(token string) (*UserClaims, error)
}

// jwtErrorResponse is the standard error response format for JWT errors.
type jwtErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// respondJWTError sends a JSON error response with the specified status code.
func respondJWTError(w http.ResponseWriter, status int, message string, code string) {
	resp := jwtErrorResponse{
		Error: message,
		Code:  code,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode JWT error response", "error", err)
	}
}

// JWTAuthWithRole returns middleware that validates JWT tokens from the Authorization header
// with support for role-based access control.
// It extracts the token, validates it, and injects user claims into the request context.
//
// The middleware:
//   - Skips authentication for paths in the exemptPaths slice (prefix match for paths ending with "/")
//   - Extracts the JWT token from the Authorization header (expected format: "Bearer <token>")
//   - Validates the token using the provided RBACJWTService
//   - Injects user claims (with role information) into the request context for use by downstream handlers
//   - Returns 401 Unauthorized for missing, malformed, or invalid tokens
//   - Logs authentication failures with structured logging
//
// This middleware should be used with RoleRequired middleware for role-based access control.
// For basic JWT validation without role-based access control, use the JWTAuth function in auth.go.
//
// Example usage:
//
//	exemptPaths := []string{"/health", "/api/v1/auth/", "/app/"}
//	handler := JWTAuthWithRole(jwtService, exemptPaths)(myHandler)
//	handler = RoleRequired("admin")(handler) // Apply to specific routes
func JWTAuthWithRole(jwtService RBACJWTService, exemptPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication for exempt paths
			if matchesExemptPath(r.URL.Path, exemptPaths) {
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
				respondJWTError(w, http.StatusUnauthorized, "authentication required", "MISSING_TOKEN")
				return
			}

			// Parse Bearer token
			token, err := extractTokenFromHeader(authHeader)
			if err != nil {
				slog.Warn("invalid authorization header format",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"error", err.Error(),
				)
				respondJWTError(w, http.StatusUnauthorized, "authentication required", "INVALID_FORMAT")
				return
			}

			// Validate token and extract claims
			claims, err := jwtService.ValidateToken(token)
			if err != nil {
				slog.Warn("token validation failed",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"error", err.Error(),
				)
				respondJWTError(w, http.StatusUnauthorized, "authentication required", "INVALID_TOKEN")
				return
			}

			// Inject user claims into request context
			ctx := context.WithValue(r.Context(), userContextKey, claims)
			ctx = context.WithValue(ctx, usernameContextKey, claims.Username)
			ctx = context.WithValue(ctx, roleContextKey, claims.Role)

			// Call the next handler with the enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RoleRequired returns middleware that enforces role-based access control.
// It checks if the authenticated user has the required role.
//
// Role hierarchy (from highest to lowest):
//   - admin: can access all endpoints
//   - moderator: can access moderator and viewer endpoints
//   - viewer: can access viewer endpoints only
//
// The middleware assumes that JWTAuth middleware has already authenticated the request
// and injected user claims into the context. If claims are not found in the context,
// it returns 401 Unauthorized (indicating missing authentication).
//
// Example usage:
//
//	handler := RoleRequired("admin")(myHandler)
func RoleRequired(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user claims from context
			username, role, ok := GetUserFromContext(r)
			if !ok {
				// User not authenticated (JWTAuth not applied or context missing)
				slog.Warn("missing user context in role check",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
				respondJWTError(w, http.StatusUnauthorized, "authentication required", "MISSING_TOKEN")
				return
			}

			// Check if user has sufficient role
			if !hasRequiredRole(role, requiredRole) {
				slog.Warn("insufficient role permissions",
					"path", r.URL.Path,
					"username", username,
					"required_role", requiredRole,
					"actual_role", role,
				)
				respondJWTError(w, http.StatusForbidden, "insufficient permissions", "INSUFFICIENT_ROLE")
				return
			}

			// Role is sufficient, call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext extracts user information from the request context.
// It returns the username, role, and a boolean indicating success.
// Returns (username, role, false) if user claims are not found in the context.
func GetUserFromContext(r *http.Request) (string, string, bool) {
	claims, ok := r.Context().Value(userContextKey).(*UserClaims)
	if !ok {
		return "", "", false
	}
	return claims.Username, claims.Role, true
}

// extractTokenFromHeader extracts the JWT token from an Authorization header.
// Expected format: "Bearer <token>"
// Returns an error if the header is malformed.
func extractTokenFromHeader(authHeader string) (string, error) {
	const bearerPrefix = "Bearer "

	// Check if the header starts with "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", ErrInvalidTokenFormat
	}

	// Extract the token part
	token := strings.TrimPrefix(authHeader, bearerPrefix)

	// Ensure the token is not empty
	if token == "" {
		return "", ErrInvalidTokenFormat
	}

	return token, nil
}

// matchesExemptPath checks if the given path should skip authentication.
// Paths ending with "/" are treated as prefixes (e.g., "/api/v1/auth/" matches "/api/v1/auth/login").
// Other paths are matched exactly.
func matchesExemptPath(path string, exemptPaths []string) bool {
	for _, exemptPath := range exemptPaths {
		// If exempt path ends with /, treat it as a prefix match
		if strings.HasSuffix(exemptPath, "/") {
			if strings.HasPrefix(path, exemptPath) || path == strings.TrimSuffix(exemptPath, "/") {
				return true
			}
		} else {
			// Exact match for paths not ending with /
			if path == exemptPath {
				return true
			}
		}
	}
	return false
}

// hasRequiredRole checks if the user's role meets the required role level.
// Role hierarchy: admin > moderator > viewer
//
// Admin can access all resources.
// Moderator can access moderator and viewer resources.
// Viewer can access viewer resources only.
func hasRequiredRole(userRole string, requiredRole string) bool {
	// Define role hierarchy (higher index = higher privilege)
	roleHierarchy := map[string]int{
		"viewer":    0,
		"moderator": 1,
		"admin":     2,
	}

	userLevel, userExists := roleHierarchy[userRole]
	requiredLevel, requiredExists := roleHierarchy[requiredRole]

	// If either role is not recognized, deny access (be conservative)
	if !userExists || !requiredExists {
		return false
	}

	// User role must be at or above the required level
	return userLevel >= requiredLevel
}
