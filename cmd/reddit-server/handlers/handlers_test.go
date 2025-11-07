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

func TestErrorToStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error returns OK",
			err:      nil,
			expected: http.StatusOK,
		},
		{
			name:     "ValidationError returns BadRequest",
			err:      &graw.ValidationError{Field: "input", Reason: "invalid input"},
			expected: http.StatusBadRequest,
		},
		{
			name:     "error message with validation returns BadRequest",
			err:      fmt.Errorf("validation failed"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "error message with invalid returns BadRequest",
			err:      fmt.Errorf("invalid value"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "error message with cannot be empty returns BadRequest",
			err:      fmt.Errorf("field cannot be empty"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "error message with is required returns BadRequest",
			err:      fmt.Errorf("field is required"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "AuthError returns Unauthorized",
			err:      &graw.AuthError{Message: "auth failed"},
			expected: http.StatusUnauthorized,
		},
		{
			name:     "error message with authentication returns Unauthorized",
			err:      fmt.Errorf("authentication failed"),
			expected: http.StatusUnauthorized,
		},
		{
			name:     "error message with unauthorized returns Unauthorized",
			err:      fmt.Errorf("unauthorized access"),
			expected: http.StatusUnauthorized,
		},
		{
			name:     "error message with not found returns NotFound",
			err:      fmt.Errorf("post not found"),
			expected: http.StatusNotFound,
		},
		{
			name:     "error message with 404 returns NotFound",
			err:      fmt.Errorf("404 error"),
			expected: http.StatusNotFound,
		},
		{
			name:     "RateLimitError returns TooManyRequests",
			err:      &graw.RateLimitError{Reason: "rate limited"},
			expected: http.StatusTooManyRequests,
		},
		{
			name:     "error message with rate limit returns TooManyRequests",
			err:      fmt.Errorf("rate limit exceeded"),
			expected: http.StatusTooManyRequests,
		},
		{
			name:     "error message with 429 returns TooManyRequests",
			err:      fmt.Errorf("429 too many requests"),
			expected: http.StatusTooManyRequests,
		},
		{
			name:     "generic error returns InternalServerError",
			err:      fmt.Errorf("unexpected error"),
			expected: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorToStatus(tt.err)
			if got != tt.expected {
				t.Errorf("errorToStatus() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestErrorType(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   string
	}{
		{
			name:       "BadRequest returns validation_error",
			statusCode: http.StatusBadRequest,
			expected:   "validation_error",
		},
		{
			name:       "Unauthorized returns auth_error",
			statusCode: http.StatusUnauthorized,
			expected:   "auth_error",
		},
		{
			name:       "NotFound returns not_found",
			statusCode: http.StatusNotFound,
			expected:   "not_found",
		},
		{
			name:       "TooManyRequests returns rate_limit_error",
			statusCode: http.StatusTooManyRequests,
			expected:   "rate_limit_error",
		},
		{
			name:       "InternalServerError returns server_error",
			statusCode: http.StatusInternalServerError,
			expected:   "server_error",
		},
		{
			name:       "default returns server_error",
			statusCode: http.StatusServiceUnavailable,
			expected:   "server_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorType(tt.statusCode)
			if got != tt.expected {
				t.Errorf("errorType() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestRespondJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	w := httptest.NewRecorder()

	data := map[string]string{"message": "success"}
	handler.respondJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("respondJSON() status = %d, want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("respondJSON() Content-Type = %s, want application/json", ct)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if result["message"] != "success" {
		t.Errorf("respondJSON() body = %v, want success", result["message"])
	}
}

func TestRespondError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	w := httptest.NewRecorder()
	handler.respondError(w, http.StatusBadRequest, "invalid input", "validation_error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("respondError() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error.Message != "invalid input" {
		t.Errorf("respondError() message = %s, want invalid input", resp.Error.Message)
	}
	if resp.Error.Type != "validation_error" {
		t.Errorf("respondError() type = %s, want validation_error", resp.Error.Type)
	}
	if resp.Error.Code != http.StatusBadRequest {
		t.Errorf("respondError() code = %d, want %d", resp.Error.Code, http.StatusBadRequest)
	}
}

func TestGetPaginationParams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantAfter  string
		wantBefore string
		wantErr    bool
	}{
		{
			name:       "default pagination params",
			query:      "",
			wantLimit:  25,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    false,
		},
		{
			name:       "custom limit",
			query:      "limit=50",
			wantLimit:  50,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    false,
		},
		{
			name:       "limit with after",
			query:      "limit=10&after=abc123",
			wantLimit:  10,
			wantAfter:  "abc123",
			wantBefore: "",
			wantErr:    false,
		},
		{
			name:       "limit with before",
			query:      "limit=20&before=xyz789",
			wantLimit:  20,
			wantAfter:  "",
			wantBefore: "xyz789",
			wantErr:    false,
		},
		{
			name:       "all pagination params",
			query:      "limit=30&after=after_token&before=before_token",
			wantLimit:  30,
			wantAfter:  "after_token",
			wantBefore: "before_token",
			wantErr:    false,
		},
		{
			name:       "invalid limit - non-numeric",
			query:      "limit=abc",
			wantLimit:  0,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    true,
		},
		{
			name:       "invalid limit - zero",
			query:      "limit=0",
			wantLimit:  0,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    true,
		},
		{
			name:       "invalid limit - negative",
			query:      "limit=-10",
			wantLimit:  0,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    true,
		},
		{
			name:       "invalid limit - exceeds max",
			query:      "limit=101",
			wantLimit:  0,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    true,
		},
		{
			name:       "limit at minimum valid",
			query:      "limit=1",
			wantLimit:  1,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    false,
		},
		{
			name:       "limit at maximum valid",
			query:      "limit=100",
			wantLimit:  100,
			wantAfter:  "",
			wantBefore: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tt.query, nil)
			limit, after, before, err := handler.getPaginationParams(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("getPaginationParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if limit != tt.wantLimit {
				t.Errorf("getPaginationParams() limit = %d, want %d", limit, tt.wantLimit)
			}
			if after != tt.wantAfter {
				t.Errorf("getPaginationParams() after = %s, want %s", after, tt.wantAfter)
			}
			if before != tt.wantBefore {
				t.Errorf("getPaginationParams() before = %s, want %s", before, tt.wantBefore)
			}
		})
	}
}

func TestGetCredentials(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	t.Run("missing credentials", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		creds, err := handler.getCredentials(req)

		if err == nil {
			t.Errorf("getCredentials() should error when credentials missing")
		}

		if creds != nil {
			t.Errorf("getCredentials() should return nil when credentials missing")
		}
	})
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{
			name:   "exact match",
			s:      "validation error",
			substr: "validation error",
			want:   true,
		},
		{
			name:   "substring at start",
			s:      "validation error occurred",
			substr: "validation",
			want:   true,
		},
		{
			name:   "substring in middle",
			s:      "the validation error",
			substr: "validation",
			want:   true,
		},
		{
			name:   "substring at end",
			s:      "this is validation",
			substr: "validation",
			want:   true,
		},
		{
			name:   "no match",
			s:      "error message",
			substr: "validation",
			want:   false,
		},
		{
			name:   "empty substring",
			s:      "test",
			substr: "",
			want:   true,
		},
		{
			name:   "empty string",
			s:      "",
			substr: "test",
			want:   false,
		},
		{
			name:   "case insensitive match lowercase",
			s:      "Validation Error",
			substr: "validation",
			want:   true,
		},
		{
			name:   "case insensitive match uppercase",
			s:      "this is a validation error",
			substr: "VALIDATION",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHealthEndpointNoAuthRequired tests that the health endpoint works without authentication.
func TestHealthEndpointNoAuthRequired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	// Create router with test API keys
	corsConfig := config.CORS{
		AllowedOrigins: "*",
		AllowedMethods: "GET,OPTIONS",
		AllowedHeaders: "Content-Type,Authorization",
		MaxAge:         300,
	}
	router := handler.Router(corsConfig, []string{"test-key"})

	// Test health endpoint without API key
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health endpoint status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("Health status = %v, want ok", resp["status"])
	}
}
