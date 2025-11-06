package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// GetSubreddit handles GET /api/v1/subreddit/{name} requests.
// Returns information about a specific subreddit.
//
// Path parameters:
//   - name: Subreddit name (e.g., "golang", "programming")
//
// Authentication: Required
// Returns: Subreddit data (description, subscriber count, etc.)
// Status codes:
//   - 200 OK: Successfully retrieved subreddit info
//   - 400 Bad Request: Invalid subreddit name
//   - 401 Unauthorized: Missing or invalid credentials
//   - 404 Not Found: Subreddit not found
//   - 500 Internal Server Error: Server or API error
func (h *Handler) GetSubreddit(w http.ResponseWriter, r *http.Request) {
	// Get subreddit name from URL parameter
	subredditName := chi.URLParam(r, "name")
	if subredditName == "" {
		h.respondError(w, http.StatusBadRequest, "Subreddit name is required", "validation_error")
		return
	}

	// Use the shared Reddit client
	client := h.client

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Fetch subreddit info
	subreddit, err := client.GetSubreddit(ctx, subredditName)
	if err != nil {
		statusCode := errorToStatus(err)
		h.logger.Error("failed to fetch subreddit info",
			slog.String("subreddit", subredditName),
			slog.String("error", err.Error()),
			slog.Int("status_code", statusCode),
		)
		h.respondError(w, statusCode, err.Error(), errorType(statusCode))
		return
	}

	// Return subreddit info in response
	resp := Response{
		Data: subreddit,
	}

	h.respondJSON(w, http.StatusOK, resp)
}
