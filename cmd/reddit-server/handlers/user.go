package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// GetUserMe handles GET /api/v1/user/me requests.
// Returns information about the authenticated user.
//
// Authentication: Required
// Returns: User account data (name, karma, etc.)
// Status codes:
//   - 200 OK: Successfully retrieved user info
//   - 400 Bad Request: Invalid input
//   - 401 Unauthorized: Missing or invalid credentials
//   - 500 Internal Server Error: Server or API error
func (h *Handler) GetUserMe(w http.ResponseWriter, r *http.Request) {
	// Use the shared Reddit client
	client := h.client

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Fetch user info
	account, err := client.Me(ctx)
	if err != nil {
		statusCode := errorToStatus(err)
		h.logger.Error("failed to fetch user info",
			slog.String("error", err.Error()),
			slog.Int("status_code", statusCode),
		)
		h.respondError(w, statusCode, err.Error(), errorType(statusCode))
		return
	}

	// Return user info in response
	resp := Response{
		Data: account,
	}

	h.respondJSON(w, http.StatusOK, resp)
}
