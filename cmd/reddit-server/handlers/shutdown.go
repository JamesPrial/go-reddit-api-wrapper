package handlers

import (
	"log/slog"
	"net/http"
)

// shutdownResponse represents the shutdown endpoint response.
type shutdownResponse struct {
	Message string `json:"message"`
}

// Shutdown handles POST /api/v1/shutdown requests.
// It initiates a graceful server shutdown by signaling the shutdown channel.
//
// The shutdown process:
//   - Stops all running monitors and waits for them to complete
//   - Waits for in-flight HTTP requests to complete (up to SHUTDOWN_TIMEOUT)
//   - Closes database connections and other resources
//   - Terminates the server process
//
// Only accepts POST requests - returns 405 for other methods.
// Returns 202 Accepted to indicate shutdown has been initiated.
// The shutdown is non-blocking from the API perspective - the endpoint returns
// immediately while the server completes active requests before terminating.
// The shutdown process may take up to SHUTDOWN_TIMEOUT (default 30s).
func (h *Handlers) Shutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		slog.Warn("shutdown endpoint called with non-POST method",
			"method", r.Method,
			"remote_addr", r.RemoteAddr)
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.shutdownCh == nil {
		respondError(w, http.StatusServiceUnavailable, "shutdown service not available")
		return
	}

	// Log shutdown request with remote address for audit trail
	slog.Info("shutdown initiated via API",
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent())

	// Non-blocking write to shutdown channel
	// Use select with default to prevent blocking if channel is full
	select {
	case h.shutdownCh <- struct{}{}:
		slog.Info("shutdown signal sent successfully")
	default:
		slog.Warn("shutdown signal already pending, duplicate request ignored",
			"remote_addr", r.RemoteAddr)
	}

	// Return 202 Accepted to indicate the shutdown has been accepted and will be processed
	response := shutdownResponse{
		Message: "server shutdown initiated",
	}

	respondJSON(w, http.StatusAccepted, response)
}
