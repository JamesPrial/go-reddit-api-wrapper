package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/monitor"
)

const (
	MaxRequestBodySize  = 1 << 20 // 1MB
	MinPostLimit        = 1
	MaxPostLimit        = 100
	DefaultPostLimit    = 25
	MaxMonitorInterval  = 24 * time.Hour
	MaxSubredditNameLen = 21
)

// StartMonitorRequest is the request body for starting a monitor.
type StartMonitorRequest struct {
	Subreddits    []string `json:"subreddits"`
	Interval      string   `json:"interval"` // e.g., "30s", "1m"
	Limit         int      `json:"limit"`
	FetchComments bool     `json:"fetch_comments"`
}

// StartMonitorResponse is the response when starting a monitor.
type StartMonitorResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// StopMonitorResponse is the response when stopping a monitor.
type StopMonitorResponse struct {
	Status string                `json:"status"`
	Stats  *MonitorStatsResponse `json:"stats,omitempty"`
}

// MonitorStatsResponse contains monitoring statistics.
type MonitorStatsResponse struct {
	TotalFetches  uint64     `json:"total_fetches"`
	TotalPosts    uint64     `json:"total_posts"`
	TotalComments uint64     `json:"total_comments"`
	LastFetchTime *time.Time `json:"last_fetch_time,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

// MonitorStatusResponse is the response for status requests.
type MonitorStatusResponse struct {
	Status     string                `json:"status"`
	ID         string                `json:"id,omitempty"`
	Subreddits []string              `json:"subreddits,omitempty"`
	Interval   string                `json:"interval,omitempty"`
	StartedAt  *time.Time            `json:"started_at,omitempty"`
	Stats      *MonitorStatsResponse `json:"stats,omitempty"`
}

// StartMonitor handles POST /api/v1/monitor/start requests.
// It starts a new monitor for the specified subreddits with the given configuration.
//
// Request body (JSON):
//
//	{
//	  "subreddits": ["subreddit1", "subreddit2"],
//	  "interval": "30s",
//	  "limit": 25,
//	  "fetch_comments": true
//	}
//
// Returns 201 Created with StartMonitorResponse on success.
// Returns 400 Bad Request for validation errors.
// Returns 409 Conflict if monitor is already running.
// Returns 500 Internal Server Error for other errors.
func (h *Handlers) StartMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Parse request body
	var req StartMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode monitor start request",
			"method", r.Method,
			"path", r.RequestURI,
			"error", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate subreddits
	if len(req.Subreddits) == 0 {
		respondError(w, http.StatusBadRequest, "subreddits array cannot be empty")
		return
	}

	// Validate and normalize individual subreddit names
	seenSubreddits := make(map[string]bool)
	normalizedSubreddits := make([]string, 0, len(req.Subreddits))

	for _, sub := range req.Subreddits {
		// Trim whitespace
		sub = strings.TrimSpace(sub)

		// Check for empty/whitespace-only strings
		if sub == "" {
			respondError(w, http.StatusBadRequest, "subreddit names cannot be empty or whitespace-only")
			return
		}

		// Check length (max 21 characters per Reddit's naming rules)
		if len(sub) > MaxSubredditNameLen {
			slog.Warn("subreddit name exceeds maximum length",
				"subreddit", sub,
				"max_length", MaxSubredditNameLen)
			respondError(w, http.StatusBadRequest, "subreddit name exceeds maximum length of 21 characters")
			return
		}

		// Check for duplicates (case-insensitive)
		lowerSub := strings.ToLower(sub)
		if seenSubreddits[lowerSub] {
			respondError(w, http.StatusBadRequest, "duplicate subreddit names are not allowed")
			return
		}

		seenSubreddits[lowerSub] = true
		normalizedSubreddits = append(normalizedSubreddits, sub)
	}

	// Validate interval
	if req.Interval == "" {
		respondError(w, http.StatusBadRequest, "interval is required")
		return
	}

	interval, err := time.ParseDuration(req.Interval)
	if err != nil {
		slog.Error("failed to parse interval",
			"interval", req.Interval,
			"method", r.Method,
			"path", r.RequestURI,
			"error", err)
		respondError(w, http.StatusBadRequest, "invalid interval format")
		return
	}

	if interval <= 0 {
		respondError(w, http.StatusBadRequest, "interval must be greater than 0")
		return
	}

	// Validate interval upper bound (24 hours)
	if interval > MaxMonitorInterval {
		respondError(w, http.StatusBadRequest, "interval cannot exceed 24 hours")
		return
	}

	// Apply default limit if not specified
	if req.Limit == 0 {
		req.Limit = DefaultPostLimit
	}

	// Validate limit range
	if req.Limit < MinPostLimit || req.Limit > MaxPostLimit {
		respondError(w, http.StatusBadRequest, "limit must be between 1 and 100")
		return
	}

	// Create monitor config with normalized subreddits
	config := MonitorConfig{
		Subreddits:    normalizedSubreddits,
		Interval:      interval,
		Limit:         req.Limit,
		FetchComments: req.FetchComments,
	}

	// Check if monitor manager is available
	if h.monitorMgr == nil {
		respondError(w, http.StatusServiceUnavailable, "monitor service not available")
		return
	}

	// Start monitor with background context to ensure it runs independently of HTTP request lifecycle
	instance, err := h.monitorMgr.Start(context.Background(), config)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, monitor.ErrMonitorAlreadyRunning) {
			slog.Warn("attempted to start monitor when already running")
			respondError(w, http.StatusConflict, "monitor is already running")
			return
		}

		if errors.Is(err, monitor.ErrInvalidConfig) {
			slog.Warn("invalid monitor configuration",
				"error", err,
				"subreddits", normalizedSubreddits,
				"method", r.Method,
				"path", r.RequestURI)
			respondError(w, http.StatusBadRequest, "invalid monitor configuration: "+err.Error())
			return
		}

		status := mapErrorToStatus(err)
		slog.Error("failed to start monitor",
			"error", err,
			"subreddits", normalizedSubreddits,
			"method", r.Method,
			"path", r.RequestURI,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	resp := StartMonitorResponse{
		ID:        instance.ID,
		Status:    "started",
		StartedAt: instance.StartedAt,
	}

	respondJSON(w, http.StatusCreated, resp)
}

// StopMonitor handles POST /api/v1/monitor/stop requests.
// It stops the currently running monitor and returns final statistics.
//
// Returns 200 OK with StopMonitorResponse on success.
// Returns 404 Not Found if no monitor is running.
// Returns 500 Internal Server Error for other errors.
func (h *Handlers) StopMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 1MB for defensive programming
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	if h.monitorMgr == nil {
		respondError(w, http.StatusServiceUnavailable, "monitor service not available")
		return
	}

	// Stop monitor
	err := h.monitorMgr.Stop()
	if err != nil {
		// Check for specific error types
		if errors.Is(err, monitor.ErrNoMonitorRunning) {
			slog.Debug("attempted to stop monitor when none is running",
				"method", r.Method,
				"path", r.RequestURI)
			respondError(w, http.StatusNotFound, "no monitor is currently running")
			return
		}

		status := mapErrorToStatus(err)
		slog.Error("failed to stop monitor",
			"error", err,
			"method", r.Method,
			"path", r.RequestURI,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	// Get final status to include stats
	status, err := h.monitorMgr.GetStatus()
	if err != nil {
		slog.Error("failed to get monitor status after stop", "error", err)
		respondJSON(w, http.StatusOK, StopMonitorResponse{
			Status: "stopped",
		})
		return
	}

	// Convert stats to response format
	var statsResp *MonitorStatsResponse
	if status.Stats != nil {
		statsResp = &MonitorStatsResponse{
			TotalFetches:  status.Stats.TotalFetches,
			TotalPosts:    status.Stats.TotalPosts,
			TotalComments: status.Stats.TotalComments,
			LastFetchTime: status.Stats.LastFetchTime,
			LastError:     status.Stats.LastError,
		}
	}

	resp := StopMonitorResponse{
		Status: "stopped",
		Stats:  statsResp,
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetMonitorStatus handles GET /api/v1/monitor/status requests.
// It returns the current status of the monitor (running or stopped).
//
// Returns 200 OK with MonitorStatusResponse.
// Returns 500 Internal Server Error for errors.
func (h *Handlers) GetMonitorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 1MB for defensive programming
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	if h.monitorMgr == nil {
		respondError(w, http.StatusServiceUnavailable, "monitor service not available")
		return
	}

	// Get monitor status
	status, err := h.monitorMgr.GetStatus()
	if err != nil {
		statusCode := mapErrorToStatus(err)
		slog.Error("failed to get monitor status",
			"error", err,
			"status", statusCode)
		respondError(w, statusCode, getClientErrorMessage(err, statusCode))
		return
	}

	// Convert to response format
	resp := MonitorStatusResponse{
		Status:     status.Status,
		ID:         status.ID,
		Subreddits: status.Subreddits,
		Interval:   status.Interval,
		StartedAt:  status.StartedAt,
	}

	// Convert stats if present
	if status.Stats != nil {
		resp.Stats = &MonitorStatsResponse{
			TotalFetches:  status.Stats.TotalFetches,
			TotalPosts:    status.Stats.TotalPosts,
			TotalComments: status.Stats.TotalComments,
			LastFetchTime: status.Stats.LastFetchTime,
			LastError:     status.Stats.LastError,
		}
	}

	respondJSON(w, http.StatusOK, resp)
}
