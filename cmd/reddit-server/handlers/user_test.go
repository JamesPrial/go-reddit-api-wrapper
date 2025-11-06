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
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

func TestGetUserMe_APIErrors(t *testing.T) {
	tests := []struct {
		name           string
		errMsg         string
		errType        string
		expectedStatus int
	}{
		{
			name:           "validation error",
			errMsg:         "validation error: invalid input",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "authentication error",
			errMsg:         "authentication failed",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "rate limit error",
			errMsg:         "rate limit exceeded",
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "generic error",
			errMsg:         "unexpected server error",
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

func TestGetUserMe_ErrorTypes(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		expectedErr string
	}{
		{
			name:        "BadRequest",
			statusCode:  http.StatusBadRequest,
			expectedErr: "validation_error",
		},
		{
			name:        "Unauthorized",
			statusCode:  http.StatusUnauthorized,
			expectedErr: "auth_error",
		},
		{
			name:        "TooManyRequests",
			statusCode:  http.StatusTooManyRequests,
			expectedErr: "rate_limit_error",
		},
		{
			name:        "InternalServerError",
			statusCode:  http.StatusInternalServerError,
			expectedErr: "server_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errType := errorType(tt.statusCode)
			if errType != tt.expectedErr {
				t.Errorf("errorType() = %s, want %s", errType, tt.expectedErr)
			}
		})
	}
}

func TestGetUserMe_ErrorTypeMapping(t *testing.T) {
	// Test validation error
	err := &graw.ValidationError{Field: "email", Reason: "invalid email"}
	status := errorToStatus(err)
	if status != http.StatusBadRequest {
		t.Errorf("ValidationError mapped to %d, want %d", status, http.StatusBadRequest)
	}

	// Test auth error
	authErr := &graw.AuthError{Message: "auth failed"}
	status = errorToStatus(authErr)
	if status != http.StatusUnauthorized {
		t.Errorf("AuthError mapped to %d, want %d", status, http.StatusUnauthorized)
	}

	// Test rate limit error
	rateLimitErr := &graw.RateLimitError{Reason: "rate limited"}
	status = errorToStatus(rateLimitErr)
	if status != http.StatusTooManyRequests {
		t.Errorf("RateLimitError mapped to %d, want %d", status, http.StatusTooManyRequests)
	}

	// Test API error
	apiErr := &graw.APIError{Message: "api error"}
	status = errorToStatus(apiErr)
	if status != http.StatusInternalServerError {
		t.Errorf("APIError mapped to %d, want %d", status, http.StatusInternalServerError)
	}
}

// TestGetUserMe_NoAPIKey tests that the endpoint requires API key authentication
func TestGetUserMe_NoAPIKey(t *testing.T) {
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
	req := httptest.NewRequest("GET", "/api/v1/user/me", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetUserMe without API key: expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if resp.Error.Type != "auth_error" {
			t.Errorf("expected error type 'auth_error', got '%s'", resp.Error.Type)
		}
	}
}

// TestGetUserMe_InvalidAPIKey tests that invalid API keys are rejected
func TestGetUserMe_InvalidAPIKey(t *testing.T) {
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
	req := httptest.NewRequest("GET", "/api/v1/user/me", nil)
	req.Header.Set("X-API-Key", "invalid-api-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetUserMe with invalid API key: expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
