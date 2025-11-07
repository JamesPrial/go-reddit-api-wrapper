package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
)

func TestGetSubreddit_MissingName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	// Call handler directly to test validation logic
	// (using the router would return 404 for missing route parameter)
	req := NewAuthenticatedRequest("GET", "/api/v1/subreddit/", nil)
	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetSubreddit(w, req)

	// Should get validation error for missing subreddit name
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

// TestGetSubreddit_NoAPIKey tests that the endpoint requires API key authentication
func TestGetSubreddit_NoAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	corsConfig := config.CORS{
		AllowedOrigins: "*",
		AllowedMethods: "GET,OPTIONS",
		AllowedHeaders: "Content-Type,Authorization",
		MaxAge:         300,
	}
	router := handler.Router(corsConfig, []string{testAPIKey})

	// Create request WITHOUT API key
	req := httptest.NewRequest("GET", "/api/v1/subreddit/golang", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetSubreddit without API key: expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if resp.Error.Type != "auth_error" {
			t.Errorf("expected error type 'auth_error', got '%s'", resp.Error.Type)
		}
	}
}

// TestGetSubreddit_InvalidAPIKey tests that invalid API keys are rejected
func TestGetSubreddit_InvalidAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	corsConfig := config.CORS{
		AllowedOrigins: "*",
		AllowedMethods: "GET,OPTIONS",
		AllowedHeaders: "Content-Type,Authorization",
		MaxAge:         300,
	}
	router := handler.Router(corsConfig, []string{testAPIKey})

	// Create request with invalid API key
	req := httptest.NewRequest("GET", "/api/v1/subreddit/golang", nil)
	req.Header.Set("X-API-Key", "invalid-api-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetSubreddit with invalid API key: expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
