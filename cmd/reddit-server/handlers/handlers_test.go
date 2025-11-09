package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

func TestMapErrorToStatus(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "ValidationError returns 400",
			err:            &graw.ValidationError{Field: "test", Reason: "invalid"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "AuthError returns 401",
			err:            &graw.AuthError{Message: "unauthorized"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "RateLimitError returns 429",
			err:            &graw.RateLimitError{Reason: "too many requests"},
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "APIError with 404 returns 404",
			err:            &graw.APIError{StatusCode: http.StatusNotFound, Message: "not found"},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "APIError with other status returns that status",
			err:            &graw.APIError{StatusCode: http.StatusBadGateway, Message: "bad gateway"},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "error message containing 'not found' returns 404",
			err:            errors.New("resource not found"),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "generic error returns 500",
			err:            errors.New("something went wrong"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := mapErrorToStatus(tt.err)
			if status != tt.expectedStatus {
				t.Errorf("mapErrorToStatus() = %d, want %d", status, tt.expectedStatus)
			}
		})
	}
}

func TestContainsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		expected bool
	}{
		{
			name:     "lowercase 'not found'",
			msg:      "resource not found",
			expected: true,
		},
		{
			name:     "title case 'Not Found'",
			msg:      "Item Not Found",
			expected: true,
		},
		{
			name:     "uppercase 'NOT FOUND'",
			msg:      "RESOURCE NOT FOUND",
			expected: true,
		},
		{
			name:     "no spaces 'notfound'",
			msg:      "resourcenotfound",
			expected: true,
		},
		{
			name:     "does not contain pattern",
			msg:      "something else happened",
			expected: false,
		},
		{
			name:     "empty string",
			msg:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsNotFound(tt.msg)
			if result != tt.expected {
				t.Errorf("containsNotFound(%q) = %v, want %v", tt.msg, result, tt.expected)
			}
		})
	}
}

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"message": "hello"}

	respondJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("respondJSON status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("respondJSON Content-Type = %q, want %q", contentType, "application/json")
	}

	expectedBody := "{\"message\":\"hello\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("respondJSON body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()

	respondError(w, http.StatusBadRequest, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("respondError status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("respondError Content-Type = %q, want %q", contentType, "application/json")
	}

	expectedBody := "{\"error\":\"invalid input\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("respondError body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name     string
		query    url.Values
		expected types.Pagination
	}{
		{
			name:  "default values",
			query: url.Values{},
			expected: types.Pagination{
				Limit:  25,
				After:  "",
				Before: "",
			},
		},
		{
			name: "custom limit",
			query: url.Values{
				"limit": []string{"50"},
			},
			expected: types.Pagination{
				Limit:  50,
				After:  "",
				Before: "",
			},
		},
		{
			name: "limit exceeds max",
			query: url.Values{
				"limit": []string{"200"},
			},
			expected: types.Pagination{
				Limit:  100,
				After:  "",
				Before: "",
			},
		},
		{
			name: "limit below min",
			query: url.Values{
				"limit": []string{"0"},
			},
			expected: types.Pagination{
				Limit:  1,
				After:  "",
				Before: "",
			},
		},
		{
			name: "negative limit",
			query: url.Values{
				"limit": []string{"-10"},
			},
			expected: types.Pagination{
				Limit:  1,
				After:  "",
				Before: "",
			},
		},
		{
			name: "invalid limit defaults to 25",
			query: url.Values{
				"limit": []string{"abc"},
			},
			expected: types.Pagination{
				Limit:  25,
				After:  "",
				Before: "",
			},
		},
		{
			name: "with after cursor",
			query: url.Values{
				"limit": []string{"10"},
				"after": []string{"t3_abc123"},
			},
			expected: types.Pagination{
				Limit:  10,
				After:  "t3_abc123",
				Before: "",
			},
		},
		{
			name: "with before cursor",
			query: url.Values{
				"limit":  []string{"10"},
				"before": []string{"t3_xyz789"},
			},
			expected: types.Pagination{
				Limit:  10,
				After:  "",
				Before: "t3_xyz789",
			},
		},
		{
			name: "with both cursors",
			query: url.Values{
				"after":  []string{"t3_abc123"},
				"before": []string{"t3_xyz789"},
			},
			expected: types.Pagination{
				Limit:  25,
				After:  "t3_abc123",
				Before: "t3_xyz789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.query.Encode(), nil)
			result := parsePagination(req)

			if result.Limit != tt.expected.Limit {
				t.Errorf("parsePagination().Limit = %d, want %d", result.Limit, tt.expected.Limit)
			}
			if result.After != tt.expected.After {
				t.Errorf("parsePagination().After = %q, want %q", result.After, tt.expected.After)
			}
			if result.Before != tt.expected.Before {
				t.Errorf("parsePagination().Before = %q, want %q", result.Before, tt.expected.Before)
			}
		})
	}
}

func TestNewHandlers(t *testing.T) {
	// This test just verifies the constructor works
	// We can't create a real Reddit client without credentials,
	// so we just check that NewHandlers doesn't panic
	var client *graw.Reddit
	h := NewHandlers(client, nil, nil)
	if h == nil {
		t.Error("NewHandlers returned nil")
	}
	if h.client != client {
		t.Error("NewHandlers did not store the client correctly")
	}
	if h.store != nil {
		t.Error("NewHandlers did not store the nil store correctly")
	}
}
