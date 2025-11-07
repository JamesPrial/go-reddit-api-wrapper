package handlers

import (
	"log/slog"
	"net/http"
	"strings"
)

// GetSubreddit handles GET /api/v1/subreddit/{name} requests.
// It retrieves information about a specific subreddit.
//
// URL Parameters:
//   - name: The subreddit name (e.g., "golang", "programming")
//
// Response:
//   - 200 OK: Returns SubredditData as JSON
//   - 400 Bad Request: Invalid subreddit name or missing parameter
//   - 401 Unauthorized: Authentication failed
//   - 404 Not Found: Subreddit does not exist
//   - 429 Too Many Requests: Rate limit exceeded
//   - 500 Internal Server Error: Server error
func (h *Handlers) GetSubreddit(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract subreddit name from URL path
	// Expected format: /api/v1/subreddit/{name}
	path := r.URL.Path
	pathPrefix := "/api/v1/subreddit/"

	if !strings.HasPrefix(path, pathPrefix) {
		slog.Error("unexpected path format", "path", path)
		respondError(w, http.StatusBadRequest, "invalid request path")
		return
	}

	// Extract the name part after the prefix
	name := strings.TrimPrefix(path, pathPrefix)

	// Remove any trailing slashes
	name = strings.TrimSuffix(name, "/")

	// Validate that name is not empty and safe
	if name == "" {
		respondError(w, http.StatusBadRequest, "subreddit name is required")
		return
	}

	// Validate path parameter for safety
	if !validatePathParameter(name) {
		slog.Warn("invalid subreddit name parameter", "name", name)
		respondError(w, http.StatusBadRequest, "invalid subreddit name")
		return
	}

	// Call the Reddit client to get subreddit information
	subreddit, err := h.client.GetSubreddit(r.Context(), name)
	if err != nil {
		status := mapErrorToStatus(err)
		slog.Error("failed to get subreddit",
			"name", name,
			"error", err,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	// Return the subreddit data as JSON
	respondJSON(w, http.StatusOK, subreddit)
}
