package handlers

import (
	"net/http"
	"time"
)

// HealthResponse represents the response from the health check endpoint.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// Health handles GET /health requests.
// Returns a 200 OK response with health status information.
// This endpoint does not require authentication.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Version:   "1.0",
	}

	h.respondJSON(w, http.StatusOK, resp)
}
