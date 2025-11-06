// Package handlers provides HTTP request handlers for Reddit API endpoints.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/middleware"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// Handler contains dependencies for HTTP handlers.
type Handler struct {
	logger *slog.Logger
	client *graw.Reddit
}

// ErrorResponse represents a standard API error response.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// PaginationMeta contains pagination metadata.
type PaginationMeta struct {
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
}

// Response is a wrapper for API responses with data and pagination metadata.
type Response struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination,omitempty"`
}

// New creates a new Handler with the provided logger and Reddit client.
func New(logger *slog.Logger, client *graw.Reddit) *Handler {
	return &Handler{
		logger: logger,
		client: client,
	}
}

// Router returns a configured chi router with all endpoints and middleware.
// apiKeys should be a non-empty slice of valid API keys for client authentication.
func (h *Handler) Router(corsConfig config.CORS, apiKeys []string) *chi.Mux {
	r := chi.NewRouter()

	// Add CORS middleware with parsed config
	corsMiddleware := cors.Handler(cors.Options{
		AllowedOrigins:   strings.Split(corsConfig.AllowedOrigins, ","),
		AllowedMethods:   strings.Split(corsConfig.AllowedMethods, ","),
		AllowedHeaders:   strings.Split(corsConfig.AllowedHeaders, ","),
		MaxAge:           corsConfig.MaxAge,
		AllowCredentials: false,
	})
	r.Use(corsMiddleware)

	// Health check endpoint (no auth required)
	r.Get("/health", h.Health)

	// API routes (all require API key authentication)
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.RequireAPIKey(apiKeys))

		r.Route("/v1", func(r chi.Router) {
			// User endpoints
			r.Get("/user/me", h.GetUserMe)

			// Subreddit endpoints
			r.Get("/subreddit/{name}", h.GetSubreddit)

			// Posts endpoints
			r.Get("/posts/hot", h.GetHotPosts)
			r.Get("/posts/new", h.GetNewPosts)

			// Comments endpoints
			r.Get("/posts/{subreddit}/{postID}/comments", h.GetComments)
			r.Post("/posts/{linkID}/more-comments", h.GetMoreComments)
		})
	})

	return r
}

// respondJSON writes a JSON response with the given status code.
func (h *Handler) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", slog.String("error", err.Error()))
	}
}

// respondError writes an error response in the standard format.
func (h *Handler) respondError(w http.ResponseWriter, statusCode int, message, errorType string) {
	resp := ErrorResponse{}
	resp.Error.Message = message
	resp.Error.Type = errorType
	resp.Error.Code = statusCode

	h.respondJSON(w, statusCode, resp)
}

// errorToStatus maps error types to HTTP status codes.
func errorToStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Check for specific error types
	errMsg := err.Error()

	// Validation errors
	var validationErr *graw.ValidationError
	if errors.As(err, &validationErr) ||
		contains(errMsg, "validation") ||
		contains(errMsg, "invalid") ||
		contains(errMsg, "cannot be empty") ||
		contains(errMsg, "is required") {
		return http.StatusBadRequest
	}

	// Auth errors
	var authErr *graw.AuthError
	if errors.As(err, &authErr) ||
		contains(errMsg, "authentication") ||
		contains(errMsg, "unauthorized") {
		return http.StatusUnauthorized
	}

	// Not found errors
	if contains(errMsg, "not found") ||
		contains(errMsg, "404") {
		return http.StatusNotFound
	}

	// Rate limit errors
	var rateLimitErr *graw.RateLimitError
	if errors.As(err, &rateLimitErr) ||
		contains(errMsg, "rate limit") ||
		contains(errMsg, "429") {
		return http.StatusTooManyRequests
	}

	// Default to internal server error
	return http.StatusInternalServerError
}

// errorType determines the error type string for error responses.
func errorType(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "validation_error"
	case http.StatusUnauthorized:
		return "auth_error"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	default:
		return "server_error"
	}
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// getPaginationParams extracts and validates pagination parameters from query string.
func (h *Handler) getPaginationParams(r *http.Request) (limit int, after, before string, err error) {
	// Get and validate limit
	limitStr := r.URL.Query().Get("limit")
	limit = 25 // Default
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			return 0, "", "", fmt.Errorf("limit must be between 1 and 100")
		}
	}

	// Get pagination tokens
	after = r.URL.Query().Get("after")
	before = r.URL.Query().Get("before")

	return limit, after, before, nil
}

// getCredentials retrieves authentication credentials from the middleware context.
// Returns an error if credentials are not found.
func (h *Handler) getCredentials(r *http.Request) (*middleware.Credentials, error) {
	creds := middleware.GetCredentials(r)
	if creds == nil {
		return nil, fmt.Errorf("credentials not found in context")
	}
	return creds, nil
}
