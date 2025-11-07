package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// Handlers contains dependencies for all HTTP handlers.
type Handlers struct {
	client RedditClient
	store  storage.Store
}

// NewHandlers creates a new Handlers instance with the provided Reddit client and storage.
// The client parameter must implement the RedditClient interface.
// The store parameter is the storage backend for persisting data.
func NewHandlers(client RedditClient, store storage.Store) *Handlers {
	return &Handlers{
		client: client,
		store:  store,
	}
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
