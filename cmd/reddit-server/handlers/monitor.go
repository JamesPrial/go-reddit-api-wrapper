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
	TotalFetches      uint64     `json:"total_fetches"`
	TotalPosts        uint64     `json:"total_posts"`
	TotalComments     uint64     `json:"total_comments"`
	FailedFetches     uint64     `json:"failed_fetches"`
	ConsecutiveErrors uint64     `json:"consecutive_errors"`
	LastFetchTime     *time.Time `json:"last_fetch_time,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
}

// MonitorStatusResponse is the response for status requests.
type MonitorStatusResponse struct {
	Status         string                `json:"status"`
	ID             string                `json:"id,omitempty"`
	Subreddits     []string              `json:"subreddits,omitempty"`
	Interval       string                `json:"interval,omitempty"`
	Limit          int                   `json:"limit,omitempty"`
	FetchComments  bool                  `json:"fetch_comments,omitempty"`
	StartedAt      *time.Time            `json:"started_at,omitempty"`
	Stats          *MonitorStatsResponse `json:"stats,omitempty"`
	LastPostIDs    map[string]string     `json:"last_post_ids,omitempty"`
	CanResume      bool                  `json:"can_resume"`
	StatePersisted bool                  `json:"state_persisted"`
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
			TotalFetches:      status.Stats.TotalFetches,
			TotalPosts:        status.Stats.TotalPosts,
			TotalComments:     status.Stats.TotalComments,
			FailedFetches:     status.Stats.FailedFetches,
			ConsecutiveErrors: status.Stats.ConsecutiveErrors,
			LastFetchTime:     status.Stats.LastFetchTime,
			LastError:         status.Stats.LastError,
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
		Status:         status.Status,
		ID:             status.ID,
		Subreddits:     status.Subreddits,
		Interval:       status.Interval,
		Limit:          status.Limit,
		FetchComments:  status.FetchComments,
		StartedAt:      status.StartedAt,
		LastPostIDs:    status.LastPostIDs,
		CanResume:      h.store != nil && status.ID != "",
		StatePersisted: h.store != nil,
	}

	// Convert stats if present
	if status.Stats != nil {
		resp.Stats = &MonitorStatsResponse{
			TotalFetches:      status.Stats.TotalFetches,
			TotalPosts:        status.Stats.TotalPosts,
			TotalComments:     status.Stats.TotalComments,
			FailedFetches:     status.Stats.FailedFetches,
			ConsecutiveErrors: status.Stats.ConsecutiveErrors,
			LastFetchTime:     status.Stats.LastFetchTime,
			LastError:         status.Stats.LastError,
		}
	}

	respondJSON(w, http.StatusOK, resp)
}

// PauseMonitor handles POST /api/v1/monitor/pause requests.
// It pauses the currently running monitor by stopping it and marking its status as "paused".
// The monitor state is preserved and can be resumed later.
//
// Returns 200 OK with pause confirmation on success.
// Returns 404 Not Found if no monitor is running.
// Returns 500 Internal Server Error for other errors.
func (h *Handlers) PauseMonitor(w http.ResponseWriter, r *http.Request) {
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

	// Get the current monitor ID before stopping
	status, err := h.monitorMgr.GetStatus()
	if err != nil {
		statusCode := mapErrorToStatus(err)
		slog.Error("failed to get monitor status before pause",
			"error", err,
			"status", statusCode)
		respondError(w, statusCode, getClientErrorMessage(err, statusCode))
		return
	}

	if status.Status != "running" {
		slog.Debug("attempted to pause monitor when none is running",
			"method", r.Method,
			"path", r.RequestURI)
		respondError(w, http.StatusNotFound, "no monitor is currently running")
		return
	}

	monitorID := status.ID

	// Stop monitor
	err = h.monitorMgr.Stop()
	if err != nil {
		// If monitor is already stopped, consider pause successful (idempotent)
		if errors.Is(err, monitor.ErrNoMonitorRunning) {
			slog.Debug("monitor already stopped during pause request",
				slog.String("method", r.Method),
				slog.String("path", r.RequestURI),
			)
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "stopped",
				"message": "monitor was already stopped",
			})
			return
		}
		// Other errors
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update status from "stopped" to "paused" in storage
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.store.UpdateMonitorStatus(ctx, monitorID, "paused"); err != nil {
		slog.Warn("failed to update monitor status to paused",
			"monitor_id", monitorID,
			"error", err)
		// Still return success since the monitor was stopped successfully
	} else {
		slog.Info("monitor paused successfully", "monitor_id", monitorID)
	}

	resp := map[string]interface{}{
		"status":  "paused",
		"message": "monitor paused successfully",
		"id":      monitorID,
	}

	respondJSON(w, http.StatusOK, resp)
}

// ResumeMonitor handles POST /api/v1/monitor/resume requests.
// It resumes a paused monitor from its saved state.
//
// Returns 200 OK with StartMonitorResponse on success.
// Returns 404 Not Found if no paused monitor exists.
// Returns 409 Conflict if a monitor is already running.
// Returns 500 Internal Server Error for other errors.
func (h *Handlers) ResumeMonitor(w http.ResponseWriter, r *http.Request) {
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

	// Query storage for paused monitors
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	pausedMonitors, err := h.store.GetPausedMonitors(ctx)
	if err != nil {
		slog.Error("failed to query paused monitors",
			"error", err,
			"method", r.Method,
			"path", r.RequestURI)
		respondError(w, http.StatusInternalServerError, "failed to query paused monitors")
		return
	}

	if len(pausedMonitors) == 0 {
		slog.Debug("no paused monitors found",
			"method", r.Method,
			"path", r.RequestURI)
		respondError(w, http.StatusNotFound, "no paused monitor found")
		return
	}

	// Get the most recently paused monitor (first in list due to ORDER BY stopped_at DESC)
	state := pausedMonitors[0]

	// Restore the monitor from state using background context
	instance, err := h.monitorMgr.RestoreFromState(context.Background(), state)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, monitor.ErrMonitorAlreadyRunning) {
			slog.Warn("attempted to resume monitor when one is already running",
				"method", r.Method,
				"path", r.RequestURI)
			respondError(w, http.StatusConflict, "monitor is already running")
			return
		}

		if errors.Is(err, monitor.ErrInvalidConfig) {
			slog.Warn("invalid monitor configuration in paused state",
				"error", err,
				"monitor_id", state.ID,
				"method", r.Method,
				"path", r.RequestURI)
			respondError(w, http.StatusBadRequest, "invalid monitor configuration: "+err.Error())
			return
		}

		statusCode := mapErrorToStatus(err)
		slog.Error("failed to resume monitor",
			"error", err,
			"monitor_id", state.ID,
			"method", r.Method,
			"path", r.RequestURI,
			"status", statusCode)
		respondError(w, statusCode, getClientErrorMessage(err, statusCode))
		return
	}

	resp := StartMonitorResponse{
		ID:        instance.ID,
		Status:    "resumed",
		StartedAt: instance.StartedAt,
	}

	respondJSON(w, http.StatusOK, resp)
}
