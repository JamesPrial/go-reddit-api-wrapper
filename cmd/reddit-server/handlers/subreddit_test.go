package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

func TestGetSubreddit_PathTraversal(t *testing.T) {
	// Test that path traversal attempts are rejected
	tests := []struct {
		name string
		path string
	}{
		{name: "parent directory", path: "/api/v1/subreddit/.."},
		{name: "parent with path", path: "/api/v1/subreddit/../etc/passwd"},
		{name: "dot slash", path: "/api/v1/subreddit/./test"},
		{name: "slash dot", path: "/api/v1/subreddit/test/."},
		{name: "double dot in middle", path: "/api/v1/subreddit/test../foo"},
		{name: "double dot at end", path: "/api/v1/subreddit/test.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			h.GetSubreddit(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("GetSubreddit() status = %d, want %d for path %q", w.Code, http.StatusBadRequest, tt.path)
			}

			expectedBody := "{\"error\":\"invalid subreddit name\"}\n"
			if w.Body.String() != expectedBody {
				t.Errorf("GetSubreddit() body = %q, want %q", w.Body.String(), expectedBody)
			}
		})
	}
}

func TestGetSubreddit_EmptyName(t *testing.T) {
	// Test that empty subreddit name is rejected
	tests := []struct {
		name string
		path string
	}{
		{name: "no name", path: "/api/v1/subreddit/"},
		{name: "just slash", path: "/api/v1/subreddit//"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			h.GetSubreddit(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("GetSubreddit() status = %d, want %d for path %q", w.Code, http.StatusBadRequest, tt.path)
			}

			expectedBody := "{\"error\":\"subreddit name is required\"}\n"
			if w.Body.String() != expectedBody {
				t.Errorf("GetSubreddit() body = %q, want %q", w.Body.String(), expectedBody)
			}
		})
	}
}

func TestGetSubreddit_MethodNotAllowed(t *testing.T) {
	// Test that non-GET methods are rejected
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
			h := NewHandlers(nil)
			req := httptest.NewRequest(tt.method, "/api/v1/subreddit/golang", nil)
			w := httptest.NewRecorder()

			h.GetSubreddit(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetSubreddit() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("GetSubreddit() Allow header = %q, want %q", allow, "GET")
			}

			expectedBody := "{\"error\":\"method not allowed\"}\n"
			if w.Body.String() != expectedBody {
				t.Errorf("GetSubreddit() body = %q, want %q", w.Body.String(), expectedBody)
			}
		})
	}
}

func TestGetSubreddit_ErrorMapping(t *testing.T) {
	// Test error mapping for different error types
	tests := []struct {
		name               string
		err                error
		expectedStatus     int
		expectedErrMessage string
	}{
		{
			name:               "AuthError returns 401",
			err:                &graw.AuthError{Message: "unauthorized"},
			expectedStatus:     http.StatusUnauthorized,
			expectedErrMessage: "authentication required",
		},
		{
			name:               "ValidationError returns 400",
			err:                &graw.ValidationError{Field: "subreddit", Reason: "invalid"},
			expectedStatus:     http.StatusBadRequest,
			expectedErrMessage: "invalid request parameters",
		},
		{
			name:               "APIError with 404 returns 404",
			err:                &graw.APIError{StatusCode: http.StatusNotFound, Message: "not found"},
			expectedStatus:     http.StatusNotFound,
			expectedErrMessage: "resource not found",
		},
		{
			name:               "Error containing 'not found' returns 404",
			err:                errors.New("subreddit not found"),
			expectedStatus:     http.StatusNotFound,
			expectedErrMessage: "resource not found",
		},
		{
			name:               "RateLimitError returns 429",
			err:                &graw.RateLimitError{Reason: "too many requests"},
			expectedStatus:     http.StatusTooManyRequests,
			expectedErrMessage: "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that mapErrorToStatus returns correct status
			status := mapErrorToStatus(tt.err)
			if status != tt.expectedStatus {
				t.Errorf("mapErrorToStatus() = %d, want %d", status, tt.expectedStatus)
			}

			// Test that getClientErrorMessage returns correct message
			message := getClientErrorMessage(tt.err, status)
			if message != tt.expectedErrMessage {
				t.Errorf("getClientErrorMessage() = %q, want %q", message, tt.expectedErrMessage)
			}
		})
	}
}

func TestGetSubreddit_PathParsing(t *testing.T) {
	// Test that the validatePathParameter function correctly validates subreddit names
	// We test the validation logic directly rather than through the handler
	// to avoid nil pointer issues with the client
	tests := []struct {
		name  string
		param string
		valid bool
	}{
		{name: "simple name", param: "golang", valid: true},
		{name: "name with numbers", param: "golang123", valid: true},
		{name: "name with underscore", param: "golang_tips", valid: true},
		{name: "single char", param: "a", valid: true},
		{name: "long name", param: "this_is_a_very_long_subreddit_name", valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePathParameter(tt.param)
			if result != tt.valid {
				t.Errorf("validatePathParameter(%q) = %v, want %v", tt.param, result, tt.valid)
			}
		})
	}
}

func TestGetSubreddit_ContentType(t *testing.T) {
	// Test that responses have correct content-type
	h := NewHandlers(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subreddit/../invalid", nil)
	w := httptest.NewRecorder()

	h.GetSubreddit(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetSubreddit() Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestValidatePathParameter(t *testing.T) {
	// Test the validatePathParameter function directly
	tests := []struct {
		name  string
		param string
		valid bool
	}{
		{name: "valid simple", param: "golang", valid: true},
		{name: "valid with underscore", param: "golang_tips", valid: true},
		{name: "valid with numbers", param: "golang123", valid: true},
		{name: "empty string", param: "", valid: false},
		{name: "single dot", param: ".", valid: false},
		{name: "double dot", param: "..", valid: false},
		{name: "contains ..", param: "foo..bar", valid: false},
		{name: "contains ./", param: "foo./bar", valid: false},
		{name: "contains /.", param: "foo/.bar", valid: false},
		{name: "starts with ..", param: "../etc", valid: false},
		{name: "ends with ..", param: "etc/..", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePathParameter(tt.param)
			if result != tt.valid {
				t.Errorf("validatePathParameter(%q) = %v, want %v", tt.param, result, tt.valid)
			}
		})
	}
}

func TestGetSubreddit_Success_ReturnsSubredditData(t *testing.T) {
	// Create mock subreddit data
	mockSubreddit := &types.SubredditData{
		ThingData: types.ThingData{
			ID:   "golang",
			Name: "t5_golang",
		},
		DisplayName:       "golang",
		Subscribers:       123456,
		Description:       "A subreddit for Go programming",
		PublicDescription: "Ask questions and share news about Go",
	}

	mock := &mockRedditClient{
		subredditResponse: mockSubreddit,
		subredditError:    nil,
	}

	h := NewHandlers(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subreddit/golang", nil)
	w := httptest.NewRecorder()

	h.GetSubreddit(w, req)

	// Verify successful status
	if w.Code != http.StatusOK {
		t.Errorf("GetSubreddit() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetSubreddit() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body contains expected data
	body := w.Body.String()
	expectedFields := []string{
		`"id":"golang"`,
		`"name":"t5_golang"`,
		`"display_name":"golang"`,
		`"subscribers":123456`,
	}

	for _, field := range expectedFields {
		if !contains(body, field) {
			t.Errorf("GetSubreddit() body missing expected field: %q\nGot: %s", field, body)
		}
	}
}
