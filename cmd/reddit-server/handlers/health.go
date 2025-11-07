package handlers

import (
	"log/slog"
	"net/http"
)

// healthResponse represents the health check response.
type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// Health handles the health check endpoint.
// It only accepts GET requests and returns a simple status response.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		slog.Warn("health check called with non-GET method", "method", r.Method)
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	response := healthResponse{
		Status:  "ok",
		Service: "reddit-api-server",
	}

	respondJSON(w, http.StatusOK, response)
}
