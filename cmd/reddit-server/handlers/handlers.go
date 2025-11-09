package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/middleware"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/monitor"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// MonitorManager defines the interface for monitor lifecycle management.
// It provides thread-safe operations for starting, stopping, and querying
// the status of background Reddit monitoring operations.
//
// All methods are thread-safe and can be called concurrently. The implementation
// must ensure proper context handling and graceful shutdown semantics.
//
// Start begins a new monitoring session with the given configuration. It returns
// an error if a monitor is already running or if the configuration is invalid.
// The returned MonitorInstance contains the metadata about the started monitor.
//
// Stop gracefully terminates the currently running monitor, if any. It returns
// ErrNoMonitorRunning if no monitor is currently active. The stop operation
// waits for the monitor loop to finish cleanly before returning.
//
// GetStatus returns the current status of the monitor. It returns a MonitorStatus
// struct with "stopped" status if no monitor is running, or "running" status with
// detailed statistics if a monitor is active.
//
// IsRunning returns true if a monitor is currently active, false otherwise.
// This is useful for quick status checks without fetching full status details.
type MonitorManager interface {
	Start(ctx context.Context, config MonitorConfig) (*MonitorInstance, error)
	Stop() error
	GetStatus() (*MonitorStatus, error)
	IsRunning() bool
}

// Type aliases for monitor package types - these ensure compatibility
// between the handlers package and the monitor package implementation.
type (
	MonitorConfig   = monitor.MonitorConfig
	MonitorInstance = monitor.MonitorInstance
	MonitorStatus   = monitor.MonitorStatus
	StatsSnapshot   = monitor.StatsSnapshot
)

// Handlers contains dependencies for all HTTP handlers.
type Handlers struct {
	client     RedditClient
	store      storage.Store
	monitorMgr MonitorManager
	shutdownCh chan<- struct{}
	setOnce    sync.Once
}

// NewHandlers creates a new Handlers instance with the provided Reddit client and storage.
// The client parameter must implement the RedditClient interface.
// The store parameter is the storage backend for persisting data.
// The shutdownCh parameter is used to signal server shutdown from handlers.
func NewHandlers(client RedditClient, store storage.Store, shutdownCh chan<- struct{}) *Handlers {
	return &Handlers{
		client:     client,
		store:      store,
		shutdownCh: shutdownCh,
	}
}

// SetMonitorManager sets the monitor manager for the handlers.
// This is called after creating the Handlers to inject the monitor dependency.
// The method is thread-safe and ensures the manager is set only once.
// If called with a nil manager, it will panic as this indicates a programmer error.
func (h *Handlers) SetMonitorManager(mgr MonitorManager) {
	h.setOnce.Do(func() {
		if mgr == nil {
			panic("monitor manager cannot be nil")
		}
		h.monitorMgr = mgr
	})
}

// mapErrorToStatus maps Reddit API errors to HTTP status codes.
// It uses errors.As to check error types from the graw package.
func mapErrorToStatus(err error) int {
	var validationErr *graw.ValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest
	}

	var authErr *graw.AuthError
	if errors.As(err, &authErr) {
		return http.StatusUnauthorized
	}

	var rateLimitErr *graw.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return http.StatusTooManyRequests
	}

	var apiErr *graw.APIError
	if errors.As(err, &apiErr) {
		// Check for 404 status in APIError
		if apiErr.StatusCode == http.StatusNotFound {
			return http.StatusNotFound
		}
		// Return the actual API status code for other cases
		return apiErr.StatusCode
	}

	// Check if error message contains "not found" for other error types
	if err != nil && containsNotFound(err.Error()) {
		return http.StatusNotFound
	}

	// Default to internal server error
	return http.StatusInternalServerError
}

// containsNotFound checks if an error message contains "not found" variations.
func containsNotFound(msg string) bool {
	lowerMsg := strings.ToLower(msg)
	return strings.Contains(lowerMsg, "not found") || strings.Contains(lowerMsg, "notfound")
}

// errorResponse is the standard error response format.
type errorResponse struct {
	Error string `json:"error"`
}

// respondJSON sends a JSON response with the specified status code.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	// Encode to buffer first to avoid race condition where WriteHeader is called
	// but encoding fails afterwards
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

// respondError sends a JSON error response with the specified status code.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, errorResponse{Error: message})
}

// getClientErrorMessage returns a sanitized error message for the client
// based on the error type and HTTP status code. This prevents exposing
// internal implementation details while providing helpful feedback.
func getClientErrorMessage(err error, status int) string {
	// For validation errors, we can be more specific
	var validationErr *graw.ValidationError
	if errors.As(err, &validationErr) {
		return "invalid request parameters"
	}

	// Map status codes to safe generic messages
	switch status {
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusUnauthorized:
		return "authentication required"
	case http.StatusForbidden:
		return "access forbidden"
	case http.StatusNotFound:
		return "resource not found"
	case http.StatusMethodNotAllowed:
		return "method not allowed"
	case http.StatusTooManyRequests:
		return "rate limit exceeded"
	case http.StatusInternalServerError:
		return "internal server error"
	case http.StatusServiceUnavailable:
		return "service unavailable"
	default:
		return "an error occurred"
	}
}

// validatePathParameter checks if a path parameter is safe and valid.
// It rejects path traversal attempts and empty strings.
func validatePathParameter(param string) bool {
	if param == "" || param == "." || param == ".." {
		return false
	}
	if strings.Contains(param, "..") || strings.Contains(param, "./") || strings.Contains(param, "/.") {
		return false
	}
	return true
}

// parsePagination extracts pagination parameters from the request query string.
// It returns a types.Pagination struct with sensible defaults:
// - limit: defaults to 25, max 100
// - after: optional pagination cursor for next page
// - before: optional pagination cursor for previous page
func parsePagination(r *http.Request) types.Pagination {
	query := r.URL.Query()

	// Parse limit with default of 25 and max of 100
	limit := 25
	if limitStr := query.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
			if limit < 1 {
				limit = 1
			}
			if limit > 100 {
				limit = 100
			}
		}
	}

	return types.Pagination{
		Limit:  limit,
		After:  query.Get("after"),
		Before: query.Get("before"),
	}
}

// GetUserFromContext extracts user information from the request context.
// Returns username, role, and ok boolean indicating if user was found.
//
// This is a convenience wrapper around the middleware.GetUserFromContext function
// that provides the same functionality. Use this in handlers to access authenticated user info
// that was injected by JWT or other authentication middleware.
//
// Example:
//
//	username, role, ok := GetUserFromContext(r)
//	if !ok {
//		respondError(w, http.StatusUnauthorized, "authentication required")
//		return
//	}
func GetUserFromContext(r *http.Request) (username string, role string, ok bool) {
	return middleware.GetUserFromContext(r)
}

// IsAuthenticated checks if the request has an authenticated user.
// Returns true if both username and role are present in the context.
//
// This is useful for quick checks before accessing user context values.
func IsAuthenticated(r *http.Request) bool {
	_, _, ok := GetUserFromContext(r)
	return ok
}

// RequireRole checks if the user has the required role.
// Returns an error if the user is not authenticated or doesn't have sufficient permissions.
//
// Role hierarchy: admin can access all endpoints that require any role.
// A specific role requirement (e.g., "admin", "viewer") must match exactly,
// unless the user has a higher privilege level (as defined in middleware.hasRequiredRole).
//
// Usage in handlers:
//
//	if err := RequireRole(r, "admin"); err != nil {
//		respondError(w, http.StatusForbidden, err.Error())
//		return
//	}
func RequireRole(r *http.Request, requiredRole string) error {
	username, role, ok := GetUserFromContext(r)
	if !ok {
		return fmt.Errorf("user not authenticated")
	}

	// Admin role can access everything
	if role == "admin" {
		return nil
	}

	// Check for exact role match
	if role == requiredRole {
		return nil
	}

	return fmt.Errorf("insufficient permissions: user %s has role %s, requires %s",
		username, role, requiredRole)
}

// SetUserHeaders adds user information to response headers for debugging purposes.
// Only use this in development/debug mode to avoid exposing user details in production.
// Adds X-Auth-User and X-Auth-Role headers if user is authenticated.
func SetUserHeaders(w http.ResponseWriter, r *http.Request) {
	username, role, ok := GetUserFromContext(r)
	if ok {
		w.Header().Set("X-Auth-User", username)
		w.Header().Set("X-Auth-Role", role)
	}
}
