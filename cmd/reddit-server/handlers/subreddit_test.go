package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetSubreddit_MissingName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	req := httptest.NewRequest("GET", "/api/v1/subreddit/", nil)
	// Don't add subreddit name to URL params

	w := httptest.NewRecorder()
	handler.GetSubreddit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetSubreddit() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if resp.Error.Type != "validation_error" {
		t.Errorf("GetSubreddit() error type = %s, want validation_error", resp.Error.Type)
	}
}

func TestGetSubreddit_ErrorMapping(t *testing.T) {
	tests := []struct {
		name           string
		errMsg         string
		expectedStatus int
	}{
		{
			name:           "not found error",
			errMsg:         "subreddit not found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "validation error message",
			errMsg:         "validation failed for subreddit",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "generic API error",
			errMsg:         "API error from Reddit",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := errorToStatus(fmt.Errorf("%s", tt.errMsg))
			if status != tt.expectedStatus {
				t.Errorf("errorToStatus() = %d, want %d", status, tt.expectedStatus)
			}
		})
	}
}

func TestGetSubreddit_ErrorTypeMapping(t *testing.T) {
	tests := []struct {
		statusCode  int
		expectedErr string
	}{
		{
			statusCode:  http.StatusBadRequest,
			expectedErr: "validation_error",
		},
		{
			statusCode:  http.StatusNotFound,
			expectedErr: "not_found",
		},
		{
			statusCode:  http.StatusUnauthorized,
			expectedErr: "auth_error",
		},
		{
			statusCode:  http.StatusInternalServerError,
			expectedErr: "server_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expectedErr, func(t *testing.T) {
			errType := errorType(tt.statusCode)
			if errType != tt.expectedErr {
				t.Errorf("errorType() = %s, want %s", errType, tt.expectedErr)
			}
		})
	}
}
