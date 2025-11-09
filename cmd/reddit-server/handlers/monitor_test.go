package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/monitor"
)

// mockMonitorManager mocks the MonitorManager interface for testing.
type mockMonitorManager struct {
	startFunc   func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error)
	stopFunc    func() error
	statusFunc  func() (*MonitorStatus, error)
	runningFunc func() bool
}

// Start mocks the Start method.
func (m *mockMonitorManager) Start(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
	if m.startFunc != nil {
		return m.startFunc(ctx, config)
	}
	return nil, errors.New("mock not configured")
}

// Stop mocks the Stop method.
func (m *mockMonitorManager) Stop() error {
	if m.stopFunc != nil {
		return m.stopFunc()
	}
	return errors.New("mock not configured")
}

// GetStatus mocks the GetStatus method.
func (m *mockMonitorManager) GetStatus() (*MonitorStatus, error) {
	if m.statusFunc != nil {
		return m.statusFunc()
	}
	return nil, errors.New("mock not configured")
}

// IsRunning mocks the IsRunning method.
func (m *mockMonitorManager) IsRunning() bool {
	if m.runningFunc != nil {
		return m.runningFunc()
	}
	return false
}

// Helper functions for tests

// createValidStartRequest creates a valid StartMonitorRequest with default values.
func createValidStartRequest() *StartMonitorRequest {
	return &StartMonitorRequest{
		Subreddits:    []string{"golang", "programming"},
		Interval:      "30s",
		Limit:         25,
		FetchComments: false,
	}
}

// makeStartMonitorRequest creates an HTTP request with a JSON body for starting a monitor.
func makeStartMonitorRequest(method string, body *StartMonitorRequest) (*http.Request, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	return httptest.NewRequest(method, "/api/v1/monitor/start", &buf), nil
}

// decodeStartMonitorResponse decodes the response body into a StartMonitorResponse.
func decodeStartMonitorResponse(t *testing.T, body []byte) *StartMonitorResponse {
	t.Helper()
	var resp StartMonitorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode StartMonitorResponse: %v", err)
	}
	return &resp
}

// decodeStopMonitorResponse decodes the response body into a StopMonitorResponse.
func decodeStopMonitorResponse(t *testing.T, body []byte) *StopMonitorResponse {
	t.Helper()
	var resp StopMonitorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode StopMonitorResponse: %v", err)
	}
	return &resp
}

// decodeMonitorStatusResponse decodes the response body into a MonitorStatusResponse.
func decodeMonitorStatusResponse(t *testing.T, body []byte) *MonitorStatusResponse {
	t.Helper()
	var resp MonitorStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode MonitorStatusResponse: %v", err)
	}
	return &resp
}

// decodeErrorResponse decodes the response body into an error response.
func decodeErrorResponse(t *testing.T, body []byte) string {
	t.Helper()
	var errResp map[string]string
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return errResp["error"]
}

// TestStartMonitor_Success tests successful monitor start.
func TestStartMonitor_Success(t *testing.T) {
	mockMgr := &mockMonitorManager{
		startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
			return &MonitorInstance{
				ID:            "monitor-123",
				Subreddits:    config.Subreddits,
				Interval:      config.Interval,
				Limit:         config.Limit,
				FetchComments: config.FetchComments,
				StartedAt:     time.Now(),
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req, err := makeStartMonitorRequest(http.MethodPost, createValidStartRequest())
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("StartMonitor() status = %d, want %d", w.Code, http.StatusCreated)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("StartMonitor() Content-Type = %q, want %q", contentType, "application/json")
	}

	resp := decodeStartMonitorResponse(t, w.Body.Bytes())

	if resp.ID != "monitor-123" {
		t.Errorf("StartMonitor() response.ID = %q, want %q", resp.ID, "monitor-123")
	}

	if resp.Status != "started" {
		t.Errorf("StartMonitor() response.Status = %q, want %q", resp.Status, "started")
	}

	if resp.StartedAt.IsZero() {
		t.Error("StartMonitor() response.StartedAt is zero")
	}
}

// TestStartMonitor_InvalidMethod tests that only POST is allowed.
func TestStartMonitor_InvalidMethod(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "GET returns 405", method: http.MethodGet},
		{name: "PUT returns 405", method: http.MethodPut},
		{name: "DELETE returns 405", method: http.MethodDelete},
		{name: "PATCH returns 405", method: http.MethodPatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/monitor/start", nil)
			w := httptest.NewRecorder()

			h.StartMonitor(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("StartMonitor() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "POST" {
				t.Errorf("StartMonitor() Allow header = %q, want %q", allow, "POST")
			}

			errMsg := decodeErrorResponse(t, w.Body.Bytes())
			if errMsg != "method not allowed" {
				t.Errorf("StartMonitor() error = %q, want %q", errMsg, "method not allowed")
			}
		})
	}
}

// TestStartMonitor_InvalidJSON tests that malformed JSON is rejected.
func TestStartMonitor_InvalidJSON(t *testing.T) {
	h := NewHandlers(nil, nil, nil)

	// Create request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/start", strings.NewReader("{invalid json}"))
	w := httptest.NewRecorder()

	h.StartMonitor(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("StartMonitor() with invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	errMsg := decodeErrorResponse(t, w.Body.Bytes())
	if errMsg != "invalid request body" {
		t.Errorf("StartMonitor() error = %q, want %q", errMsg, "invalid request body")
	}
}

// TestStartMonitor_EmptySubreddits tests that empty subreddits array is rejected.
func TestStartMonitor_EmptySubreddits(t *testing.T) {
	h := NewHandlers(nil, nil, nil)

	req := createValidStartRequest()
	req.Subreddits = []string{}

	httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("StartMonitor() with empty subreddits status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	errMsg := decodeErrorResponse(t, w.Body.Bytes())
	if errMsg != "subreddits array cannot be empty" {
		t.Errorf("StartMonitor() error = %q, want %q", errMsg, "subreddits array cannot be empty")
	}
}

// TestStartMonitor_MissingInterval tests that missing interval is rejected.
func TestStartMonitor_MissingInterval(t *testing.T) {
	h := NewHandlers(nil, nil, nil)

	req := createValidStartRequest()
	req.Interval = ""

	httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("StartMonitor() with missing interval status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	errMsg := decodeErrorResponse(t, w.Body.Bytes())
	if errMsg != "interval is required" {
		t.Errorf("StartMonitor() error = %q, want %q", errMsg, "interval is required")
	}
}

// TestStartMonitor_InvalidInterval tests that invalid interval format is rejected.
func TestStartMonitor_InvalidInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{name: "non-numeric interval", interval: "abc"},
		{name: "invalid unit", interval: "30x"},
		{name: "empty interval", interval: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)

			req := createValidStartRequest()
			req.Interval = tt.interval

			// Skip empty interval test since it's tested separately
			if tt.interval == "" {
				return
			}

			httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			h.StartMonitor(w, httpReq)

			if w.Code != http.StatusBadRequest {
				t.Errorf("StartMonitor() with interval %q status = %d, want %d", tt.interval, w.Code, http.StatusBadRequest)
			}

			errMsg := decodeErrorResponse(t, w.Body.Bytes())
			if errMsg != "invalid interval format" {
				t.Errorf("StartMonitor() error = %q, want %q", errMsg, "invalid interval format")
			}
		})
	}
}

// TestStartMonitor_ZeroOrNegativeInterval tests that zero/negative intervals are rejected.
func TestStartMonitor_ZeroOrNegativeInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{name: "zero interval", interval: "0s"},
		{name: "negative interval", interval: "-30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)

			req := createValidStartRequest()
			req.Interval = tt.interval

			httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			h.StartMonitor(w, httpReq)

			if w.Code != http.StatusBadRequest {
				t.Errorf("StartMonitor() with interval %q status = %d, want %d", tt.interval, w.Code, http.StatusBadRequest)
			}

			errMsg := decodeErrorResponse(t, w.Body.Bytes())
			if errMsg != "interval must be greater than 0" {
				t.Errorf("StartMonitor() error = %q, want %q", errMsg, "interval must be greater than 0")
			}
		})
	}
}

// TestStartMonitor_ValidIntervalFormats tests that various valid interval formats are accepted.
func TestStartMonitor_ValidIntervalFormats(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{name: "seconds", interval: "30s"},
		{name: "minutes", interval: "1m"},
		{name: "hours", interval: "2h"},
		{name: "mixed", interval: "1m30s"},
		{name: "milliseconds", interval: "100ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMgr := &mockMonitorManager{
				startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
					return &MonitorInstance{
						ID:        "monitor-test",
						StartedAt: time.Now(),
					}, nil
				},
			}

			h := NewHandlers(nil, nil, nil)
			h.SetMonitorManager(mockMgr)

			req := createValidStartRequest()
			req.Interval = tt.interval

			httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			h.StartMonitor(w, httpReq)

			if w.Code != http.StatusCreated {
				t.Errorf("StartMonitor() with interval %q status = %d, want %d", tt.interval, w.Code, http.StatusCreated)
			}
		})
	}
}

// TestStartMonitor_InvalidLimit tests that invalid limit values are rejected.
func TestStartMonitor_InvalidLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
	}{
		{name: "negative limit", limit: -1},
		{name: "limit > 100", limit: 101},
		{name: "very large limit", limit: 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			// Set a mock manager to avoid "service not available" error
			h.SetMonitorManager(&mockMonitorManager{})

			req := createValidStartRequest()
			req.Limit = tt.limit

			httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			h.StartMonitor(w, httpReq)

			if w.Code != http.StatusBadRequest {
				t.Errorf("StartMonitor() with limit %d status = %d, want %d", tt.limit, w.Code, http.StatusBadRequest)
			}

			errMsg := decodeErrorResponse(t, w.Body.Bytes())
			if errMsg != "limit must be between 1 and 100" {
				t.Errorf("StartMonitor() error = %q, want %q", errMsg, "limit must be between 1 and 100")
			}
		})
	}
}

// TestStartMonitor_DefaultLimit tests that a limit of 0 applies the default limit.
func TestStartMonitor_DefaultLimit(t *testing.T) {
	capturedConfig := &MonitorConfig{}

	mockMgr := &mockMonitorManager{
		startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
			*capturedConfig = config
			return &MonitorInstance{
				ID:        "test-id",
				StartedAt: time.Now(),
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := createValidStartRequest()
	req.Limit = 0

	httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("StartMonitor() with limit=0 status = %d, want %d", w.Code, http.StatusCreated)
	}

	if capturedConfig.Limit != DefaultPostLimit {
		t.Errorf("StartMonitor() applied limit = %d, want %d", capturedConfig.Limit, DefaultPostLimit)
	}
}

// TestStartMonitor_ValidLimitBoundaries tests that limit boundaries are correct.
func TestStartMonitor_ValidLimitBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		limit int
	}{
		{name: "limit = 1", limit: 1},
		{name: "limit = 50", limit: 50},
		{name: "limit = 100", limit: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMgr := &mockMonitorManager{
				startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
					if config.Limit != tt.limit {
						t.Errorf("expected limit %d, got %d", tt.limit, config.Limit)
					}
					return &MonitorInstance{
						ID:        "monitor-test",
						StartedAt: time.Now(),
					}, nil
				},
			}

			h := NewHandlers(nil, nil, nil)
			h.SetMonitorManager(mockMgr)

			req := createValidStartRequest()
			req.Limit = tt.limit

			httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			h.StartMonitor(w, httpReq)

			if w.Code != http.StatusCreated {
				t.Errorf("StartMonitor() with limit %d status = %d, want %d", tt.limit, w.Code, http.StatusCreated)
			}
		})
	}
}

// TestStartMonitor_AlreadyRunning tests that starting when already running returns 409.
func TestStartMonitor_AlreadyRunning(t *testing.T) {
	mockMgr := &mockMonitorManager{
		startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
			return nil, monitor.ErrMonitorAlreadyRunning
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req, err := makeStartMonitorRequest(http.MethodPost, createValidStartRequest())
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("StartMonitor() with already running status = %d, want %d", w.Code, http.StatusConflict)
	}

	errMsg := decodeErrorResponse(t, w.Body.Bytes())
	if errMsg != "monitor is already running" {
		t.Errorf("StartMonitor() error = %q, want %q", errMsg, "monitor is already running")
	}
}

// TestStartMonitor_ManagerError tests that manager errors return 500.
func TestStartMonitor_ManagerError(t *testing.T) {
	mockMgr := &mockMonitorManager{
		startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
			return nil, errors.New("internal error")
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req, err := makeStartMonitorRequest(http.MethodPost, createValidStartRequest())
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("StartMonitor() with manager error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestStartMonitor_RequestBodyTooLarge tests that request body > 1MB is rejected.
func TestStartMonitor_RequestBodyTooLarge(t *testing.T) {
	h := NewHandlers(nil, nil, nil)

	// Create a large body (2MB)
	largeBody := strings.NewReader(strings.Repeat("x", 2*1024*1024))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/start", largeBody)
	w := httptest.NewRecorder()

	h.StartMonitor(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("StartMonitor() with large body status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	errMsg := decodeErrorResponse(t, w.Body.Bytes())
	if errMsg != "invalid request body" {
		t.Errorf("StartMonitor() error = %q, want %q", errMsg, "invalid request body")
	}
}

// TestStartMonitor_ConfigurationPassed tests that the configuration is correctly passed to the manager.
func TestStartMonitor_ConfigurationPassed(t *testing.T) {
	capturedConfig := MonitorConfig{}
	mockMgr := &mockMonitorManager{
		startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
			capturedConfig = config
			return &MonitorInstance{
				ID:        "monitor-test",
				StartedAt: time.Now(),
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := createValidStartRequest()
	req.Subreddits = []string{"golang", "rust", "python"}
	req.Interval = "1m"
	req.Limit = 50
	req.FetchComments = true

	httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("StartMonitor() status = %d, want %d", w.Code, http.StatusCreated)
	}

	if len(capturedConfig.Subreddits) != 3 {
		t.Errorf("captured config subreddits count = %d, want 3", len(capturedConfig.Subreddits))
	}
	if capturedConfig.Interval != time.Minute {
		t.Errorf("captured config interval = %v, want %v", capturedConfig.Interval, time.Minute)
	}
	if capturedConfig.Limit != 50 {
		t.Errorf("captured config limit = %d, want 50", capturedConfig.Limit)
	}
	if !capturedConfig.FetchComments {
		t.Error("captured config FetchComments = false, want true")
	}
}

// TestStopMonitor_Success tests successful monitor stop.
func TestStopMonitor_Success(t *testing.T) {
	now := time.Now()
	mockMgr := &mockMonitorManager{
		stopFunc: func() error {
			return nil
		},
		statusFunc: func() (*MonitorStatus, error) {
			return &MonitorStatus{
				Status: "stopped",
				ID:     "monitor-123",
				Stats: &StatsSnapshot{
					TotalFetches:  10,
					TotalPosts:    100,
					TotalComments: 500,
					LastFetchTime: &now,
					LastError:     "",
				},
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/stop", nil)
	w := httptest.NewRecorder()

	h.StopMonitor(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("StopMonitor() status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("StopMonitor() Content-Type = %q, want %q", contentType, "application/json")
	}

	resp := decodeStopMonitorResponse(t, w.Body.Bytes())

	if resp.Status != "stopped" {
		t.Errorf("StopMonitor() response.Status = %q, want %q", resp.Status, "stopped")
	}

	if resp.Stats == nil {
		t.Error("StopMonitor() response.Stats is nil")
	} else {
		if resp.Stats.TotalFetches != 10 {
			t.Errorf("StopMonitor() Stats.TotalFetches = %d, want 10", resp.Stats.TotalFetches)
		}
		if resp.Stats.TotalPosts != 100 {
			t.Errorf("StopMonitor() Stats.TotalPosts = %d, want 100", resp.Stats.TotalPosts)
		}
		if resp.Stats.TotalComments != 500 {
			t.Errorf("StopMonitor() Stats.TotalComments = %d, want 500", resp.Stats.TotalComments)
		}
	}
}

// TestStopMonitor_NoMonitorRunning tests that stopping when none is running returns 404.
func TestStopMonitor_NoMonitorRunning(t *testing.T) {
	mockMgr := &mockMonitorManager{
		stopFunc: func() error {
			return monitor.ErrNoMonitorRunning
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/stop", nil)
	w := httptest.NewRecorder()

	h.StopMonitor(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("StopMonitor() with no monitor running status = %d, want %d", w.Code, http.StatusNotFound)
	}

	errMsg := decodeErrorResponse(t, w.Body.Bytes())
	if errMsg != "no monitor is currently running" {
		t.Errorf("StopMonitor() error = %q, want %q", errMsg, "no monitor is currently running")
	}
}

// TestStopMonitor_InvalidMethod tests that only POST is allowed.
func TestStopMonitor_InvalidMethod(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "GET returns 405", method: http.MethodGet},
		{name: "PUT returns 405", method: http.MethodPut},
		{name: "DELETE returns 405", method: http.MethodDelete},
		{name: "PATCH returns 405", method: http.MethodPatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/monitor/stop", nil)
			w := httptest.NewRecorder()

			h.StopMonitor(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("StopMonitor() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "POST" {
				t.Errorf("StopMonitor() Allow header = %q, want %q", allow, "POST")
			}

			errMsg := decodeErrorResponse(t, w.Body.Bytes())
			if errMsg != "method not allowed" {
				t.Errorf("StopMonitor() error = %q, want %q", errMsg, "method not allowed")
			}
		})
	}
}

// TestStopMonitor_ManagerError tests that manager errors return 500.
func TestStopMonitor_ManagerError(t *testing.T) {
	mockMgr := &mockMonitorManager{
		stopFunc: func() error {
			return errors.New("internal error")
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/stop", nil)
	w := httptest.NewRecorder()

	h.StopMonitor(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("StopMonitor() with manager error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestStopMonitor_StatusRetrievalFails tests that stop succeeds even if status retrieval fails.
func TestStopMonitor_StatusRetrievalFails(t *testing.T) {
	mockMgr := &mockMonitorManager{
		stopFunc: func() error {
			return nil
		},
		statusFunc: func() (*MonitorStatus, error) {
			return nil, errors.New("status retrieval failed")
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/stop", nil)
	w := httptest.NewRecorder()

	h.StopMonitor(w, req)

	// Even if status retrieval fails, stop should return OK
	if w.Code != http.StatusOK {
		t.Errorf("StopMonitor() status = %d, want %d", w.Code, http.StatusOK)
	}

	resp := decodeStopMonitorResponse(t, w.Body.Bytes())
	if resp.Status != "stopped" {
		t.Errorf("StopMonitor() response.Status = %q, want %q", resp.Status, "stopped")
	}
}

// TestGetMonitorStatus_Running tests status when monitor is running.
func TestGetMonitorStatus_Running(t *testing.T) {
	now := time.Now()
	mockMgr := &mockMonitorManager{
		statusFunc: func() (*MonitorStatus, error) {
			return &MonitorStatus{
				Status:     "running",
				ID:         "monitor-123",
				Subreddits: []string{"golang", "programming"},
				Interval:   "30s",
				StartedAt:  &now,
				Stats: &StatsSnapshot{
					TotalFetches:  5,
					TotalPosts:    50,
					TotalComments: 200,
					LastFetchTime: &now,
					LastError:     "",
				},
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/status", nil)
	w := httptest.NewRecorder()

	h.GetMonitorStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetMonitorStatus() status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetMonitorStatus() Content-Type = %q, want %q", contentType, "application/json")
	}

	resp := decodeMonitorStatusResponse(t, w.Body.Bytes())

	if resp.Status != "running" {
		t.Errorf("GetMonitorStatus() response.Status = %q, want %q", resp.Status, "running")
	}

	if resp.ID != "monitor-123" {
		t.Errorf("GetMonitorStatus() response.ID = %q, want %q", resp.ID, "monitor-123")
	}

	if len(resp.Subreddits) != 2 {
		t.Errorf("GetMonitorStatus() response.Subreddits count = %d, want 2", len(resp.Subreddits))
	}

	if resp.Interval != "30s" {
		t.Errorf("GetMonitorStatus() response.Interval = %q, want %q", resp.Interval, "30s")
	}

	if resp.StartedAt == nil {
		t.Error("GetMonitorStatus() response.StartedAt is nil")
	}

	if resp.Stats == nil {
		t.Error("GetMonitorStatus() response.Stats is nil")
	} else {
		if resp.Stats.TotalFetches != 5 {
			t.Errorf("GetMonitorStatus() Stats.TotalFetches = %d, want 5", resp.Stats.TotalFetches)
		}
		if resp.Stats.TotalPosts != 50 {
			t.Errorf("GetMonitorStatus() Stats.TotalPosts = %d, want 50", resp.Stats.TotalPosts)
		}
		if resp.Stats.TotalComments != 200 {
			t.Errorf("GetMonitorStatus() Stats.TotalComments = %d, want 200", resp.Stats.TotalComments)
		}
	}
}

// TestGetMonitorStatus_Stopped tests status when monitor is stopped.
func TestGetMonitorStatus_Stopped(t *testing.T) {
	mockMgr := &mockMonitorManager{
		statusFunc: func() (*MonitorStatus, error) {
			return &MonitorStatus{
				Status: "stopped",
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/status", nil)
	w := httptest.NewRecorder()

	h.GetMonitorStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetMonitorStatus() status = %d, want %d", w.Code, http.StatusOK)
	}

	resp := decodeMonitorStatusResponse(t, w.Body.Bytes())

	if resp.Status != "stopped" {
		t.Errorf("GetMonitorStatus() response.Status = %q, want %q", resp.Status, "stopped")
	}

	if resp.ID != "" {
		t.Errorf("GetMonitorStatus() response.ID should be empty for stopped monitor, got %q", resp.ID)
	}

	if resp.Stats != nil {
		t.Error("GetMonitorStatus() response.Stats should be nil for stopped monitor")
	}
}

// TestGetMonitorStatus_InvalidMethod tests that only GET is allowed.
func TestGetMonitorStatus_InvalidMethod(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "POST returns 405", method: http.MethodPost},
		{name: "PUT returns 405", method: http.MethodPut},
		{name: "DELETE returns 405", method: http.MethodDelete},
		{name: "PATCH returns 405", method: http.MethodPatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/monitor/status", nil)
			w := httptest.NewRecorder()

			h.GetMonitorStatus(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetMonitorStatus() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("GetMonitorStatus() Allow header = %q, want %q", allow, "GET")
			}

			errMsg := decodeErrorResponse(t, w.Body.Bytes())
			if errMsg != "method not allowed" {
				t.Errorf("GetMonitorStatus() error = %q, want %q", errMsg, "method not allowed")
			}
		})
	}
}

// TestGetMonitorStatus_ManagerError tests that manager errors return 500.
func TestGetMonitorStatus_ManagerError(t *testing.T) {
	mockMgr := &mockMonitorManager{
		statusFunc: func() (*MonitorStatus, error) {
			return nil, errors.New("internal error")
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/status", nil)
	w := httptest.NewRecorder()

	h.GetMonitorStatus(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("GetMonitorStatus() with manager error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestStartMonitor_AllFieldsInResponse tests that response contains all expected fields.
func TestStartMonitor_AllFieldsInResponse(t *testing.T) {
	mockMgr := &mockMonitorManager{
		startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
			return &MonitorInstance{
				ID:        "test-id-123",
				StartedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req, err := makeStartMonitorRequest(http.MethodPost, createValidStartRequest())
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	w := httptest.NewRecorder()
	h.StartMonitor(w, req)

	resp := decodeStartMonitorResponse(t, w.Body.Bytes())

	// Verify all fields are present
	if resp.ID == "" {
		t.Error("StartMonitor() response.ID is empty")
	}
	if resp.Status == "" {
		t.Error("StartMonitor() response.Status is empty")
	}
	if resp.StartedAt.IsZero() {
		t.Error("StartMonitor() response.StartedAt is zero")
	}
}

// TestStartMonitor_UniversalSubreddits tests with various subreddit names.
func TestStartMonitor_VariousSubredditNames(t *testing.T) {
	tests := []struct {
		name       string
		subreddits []string
	}{
		{name: "single subreddit", subreddits: []string{"golang"}},
		{name: "multiple subreddits", subreddits: []string{"golang", "programming", "code"}},
		{name: "subreddit with numbers", subreddits: []string{"golang2", "r123"}},
		{name: "subreddit with underscores", subreddits: []string{"golang_news", "computer_science"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedSubreddits := []string{}
			mockMgr := &mockMonitorManager{
				startFunc: func(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
					capturedSubreddits = config.Subreddits
					return &MonitorInstance{
						ID:        "monitor-test",
						StartedAt: time.Now(),
					}, nil
				},
			}

			h := NewHandlers(nil, nil, nil)
			h.SetMonitorManager(mockMgr)

			req := createValidStartRequest()
			req.Subreddits = tt.subreddits

			httpReq, err := makeStartMonitorRequest(http.MethodPost, req)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			w := httptest.NewRecorder()
			h.StartMonitor(w, httpReq)

			if w.Code != http.StatusCreated {
				t.Errorf("StartMonitor() status = %d, want %d", w.Code, http.StatusCreated)
			}

			if len(capturedSubreddits) != len(tt.subreddits) {
				t.Errorf("captured subreddits count = %d, want %d", len(capturedSubreddits), len(tt.subreddits))
			}

			for i, sub := range tt.subreddits {
				if i < len(capturedSubreddits) && capturedSubreddits[i] != sub {
					t.Errorf("captured subreddit[%d] = %q, want %q", i, capturedSubreddits[i], sub)
				}
			}
		})
	}
}

// TestGetMonitorStatus_WithoutStats tests response when stats are not available.
func TestGetMonitorStatus_WithoutStats(t *testing.T) {
	mockMgr := &mockMonitorManager{
		statusFunc: func() (*MonitorStatus, error) {
			return &MonitorStatus{
				Status: "running",
				ID:     "monitor-123",
				Stats:  nil,
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/monitor/status", nil)
	w := httptest.NewRecorder()

	h.GetMonitorStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetMonitorStatus() status = %d, want %d", w.Code, http.StatusOK)
	}

	resp := decodeMonitorStatusResponse(t, w.Body.Bytes())

	if resp.Stats != nil {
		t.Error("GetMonitorStatus() response.Stats should be nil when not provided")
	}
}

// TestStopMonitor_WithEmptyStats tests stop response when stats are empty.
func TestStopMonitor_WithoutStats(t *testing.T) {
	mockMgr := &mockMonitorManager{
		stopFunc: func() error {
			return nil
		},
		statusFunc: func() (*MonitorStatus, error) {
			return &MonitorStatus{
				Status: "stopped",
				Stats:  nil,
			}, nil
		},
	}

	h := NewHandlers(nil, nil, nil)
	h.SetMonitorManager(mockMgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/monitor/stop", nil)
	w := httptest.NewRecorder()

	h.StopMonitor(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("StopMonitor() status = %d, want %d", w.Code, http.StatusOK)
	}

	resp := decodeStopMonitorResponse(t, w.Body.Bytes())

	if resp.Stats != nil {
		t.Error("StopMonitor() response.Stats should be nil when not provided")
	}
}
