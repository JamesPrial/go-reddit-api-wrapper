package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// mockRedditClient implements the RedditClient interface for testing.
type mockRedditClient struct {
	meResponse           *types.AccountData
	meError              error
	subredditResponse    *types.SubredditData
	subredditError       error
	hotResponse          *types.PostsResponse
	hotError             error
	newResponse          *types.PostsResponse
	newError             error
	commentsResponse     *types.CommentsResponse
	commentsError        error
	moreCommentsResponse []*types.Comment
	moreCommentsError    error
}

func (m *mockRedditClient) Me(ctx context.Context) (*types.AccountData, error) {
	return m.meResponse, m.meError
}

func (m *mockRedditClient) GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error) {
	return m.subredditResponse, m.subredditError
}

func (m *mockRedditClient) GetHot(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
	return m.hotResponse, m.hotError
}

func (m *mockRedditClient) GetNew(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
	return m.newResponse, m.newError
}

func (m *mockRedditClient) GetComments(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error) {
	return m.commentsResponse, m.commentsError
}

func (m *mockRedditClient) GetMoreComments(ctx context.Context, req *types.MoreCommentsRequest) ([]*types.Comment, error) {
	return m.moreCommentsResponse, m.moreCommentsError
}

func TestGetUserMe_MethodNotAllowed(t *testing.T) {
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
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/user/me", nil)
			w := httptest.NewRecorder()

			h.GetUserMe(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetUserMe() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("GetUserMe() Allow header = %q, want %q", allow, "GET")
			}

			expectedBody := "{\"error\":\"method not allowed\"}\n"
			if w.Body.String() != expectedBody {
				t.Errorf("GetUserMe() body = %q, want %q", w.Body.String(), expectedBody)
			}
		})
	}
}

// Since we can't easily mock the Reddit client without changing the Handlers struct,
// we'll test the error mapping logic which is the core of what the handler does.
// The actual Reddit client integration is tested in integration tests.

func TestGetUserMe_ErrorMapping(t *testing.T) {
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
			err:                &graw.ValidationError{Field: "user", Reason: "invalid"},
			expectedStatus:     http.StatusBadRequest,
			expectedErrMessage: "invalid request parameters",
		},
		{
			name:               "RateLimitError returns 429",
			err:                &graw.RateLimitError{Reason: "too many requests"},
			expectedStatus:     http.StatusTooManyRequests,
			expectedErrMessage: "rate limit exceeded",
		},
		{
			name:               "NetworkError returns 500",
			err:                &graw.NetworkError{Method: "GET", URL: "/api/v1/me", Err: errors.New("connection failed")},
			expectedStatus:     http.StatusInternalServerError,
			expectedErrMessage: "internal server error",
		},
		{
			name:               "Generic error returns 500",
			err:                errors.New("something went wrong"),
			expectedStatus:     http.StatusInternalServerError,
			expectedErrMessage: "internal server error",
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

func TestGetUserMe_ContentType(t *testing.T) {
	// Test that even with nil client, we get proper content-type on method check
	h := NewHandlers(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/me", nil)
	w := httptest.NewRecorder()

	h.GetUserMe(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetUserMe() Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestGetUserMe_Success_ReturnsAccountData(t *testing.T) {
	// Create mock account data
	mockAccount := &types.AccountData{
		ThingData: types.ThingData{
			ID:   "abc123",
			Name: "test_user",
		},
		Created: types.Created{
			CreatedUTC: 1234567890.0,
		},
		LinkKarma:    1234,
		CommentKarma: 5678,
	}

	mock := &mockRedditClient{
		meResponse: mockAccount,
		meError:    nil,
	}

	h := NewHandlers(mock, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	w := httptest.NewRecorder()

	h.GetUserMe(w, req)

	// Verify successful status
	if w.Code != http.StatusOK {
		t.Errorf("GetUserMe() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetUserMe() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body contains expected data
	body := w.Body.String()
	expectedFields := []string{
		`"id":"abc123"`,
		`"name":"test_user"`,
		`"link_karma":1234`,
		`"comment_karma":5678`,
	}

	for _, field := range expectedFields {
		if !contains(body, field) {
			t.Errorf("GetUserMe() body missing expected field: %q\nGot: %s", field, body)
		}
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
