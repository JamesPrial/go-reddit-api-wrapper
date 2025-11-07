package handlers

import (
	"log/slog"
	"net/http"
)

// GetUserMe handles the GET /api/v1/user/me endpoint.
// It fetches the authenticated user's account information.
func (h *Handlers) GetUserMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		slog.Warn("GetUserMe called with non-GET method", "method", r.Method)
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Fetch the authenticated user's account data
	accountData, err := h.client.Me(r.Context())
	if err != nil {
		status := mapErrorToStatus(err)
		slog.Error("failed to fetch user account data",
			"error", err,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	// Return the account data as JSON
	respondJSON(w, http.StatusOK, accountData)
}
