package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// TestGetHotPosts tests the GetHotPosts handler
func TestGetHotPosts_MethodNotAllowed(t *testing.T) {
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
			req := httptest.NewRequest(tt.method, "/api/v1/posts/hot", nil)
			w := httptest.NewRecorder()

			h.GetHotPosts(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetHotPosts() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("GetHotPosts() Allow header = %q, want %q", allow, "GET")
			}
		})
	}
}

func TestGetHotPosts_ContentType(t *testing.T) {
	// Test that method not allowed returns proper content type
	h := NewHandlers(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/hot", nil)
	w := httptest.NewRecorder()

	h.GetHotPosts(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetHotPosts() Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestGetNewPosts tests the GetNewPosts handler
func TestGetNewPosts_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "POST returns 405", method: http.MethodPost},
		{name: "PUT returns 405", method: http.MethodPut},
		{name: "DELETE returns 405", method: http.MethodDelete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/posts/new", nil)
			w := httptest.NewRecorder()

			h.GetNewPosts(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetNewPosts() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("GetNewPosts() Allow header = %q, want %q", allow, "GET")
			}
		})
	}
}

func TestGetNewPosts_ContentType(t *testing.T) {
	// Test that method not allowed returns proper content type
	h := NewHandlers(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/new", nil)
	w := httptest.NewRecorder()

	h.GetNewPosts(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetNewPosts() Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestGetComments tests the GetComments handler
func TestGetComments_PathTraversal(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "parent in subreddit", path: "/api/v1/posts/../etc/abc123/comments"},
		{name: "parent in postID", path: "/api/v1/posts/golang/../abc123/comments"},
		{name: "dot in subreddit", path: "/api/v1/posts/./abc123/comments"},
		{name: "double dot in subreddit", path: "/api/v1/posts/test../abc123/comments"},
		{name: "slash dot in postID", path: "/api/v1/posts/golang/test/./comments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			h.GetComments(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("GetComments() status = %d, want %d for path %q", w.Code, http.StatusBadRequest, tt.path)
			}

			body := w.Body.String()
			if !strings.Contains(body, "invalid") {
				t.Errorf("GetComments() body should contain 'invalid' for path traversal, got: %s", body)
			}
		})
	}
}

func TestGetComments_EmptySegments(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedBody string
	}{
		{
			name:         "empty subreddit",
			path:         "/api/v1/posts//abc123/comments",
			expectedBody: "subreddit is required",
		},
		{
			name:         "empty postID",
			path:         "/api/v1/posts/golang//comments",
			expectedBody: "postID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			h.GetComments(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("GetComments() status = %d, want %d for path %q", w.Code, http.StatusBadRequest, tt.path)
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("GetComments() body = %q, want to contain %q", body, tt.expectedBody)
			}
		})
	}
}

func TestGetComments_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "POST returns 405", method: http.MethodPost},
		{name: "PUT returns 405", method: http.MethodPut},
		{name: "DELETE returns 405", method: http.MethodDelete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/posts/golang/abc123/comments", nil)
			w := httptest.NewRecorder()

			h.GetComments(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetComments() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("GetComments() Allow header = %q, want %q", allow, "GET")
			}
		})
	}
}

func TestGetComments_ContentType(t *testing.T) {
	// Test that errors return proper content type
	h := NewHandlers(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/golang/abc123/comments", nil)
	w := httptest.NewRecorder()

	h.GetComments(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetComments() Content-Type = %q, want %q", contentType, "application/json")
	}
}

// TestGetMoreComments tests the GetMoreComments handler
func TestGetMoreComments_EmptyChildren(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	body := `{"children":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with empty children status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"children array cannot be empty\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_TooManyChildren(t *testing.T) {
	h := NewHandlers(nil, nil, nil)

	// Create an array with 101 items
	children := make([]string, 101)
	for i := 0; i < 101; i++ {
		children[i] = "id" + string(rune(i))
	}
	reqBody := map[string]interface{}{"children": children}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with >100 children status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"children array exceeds maximum of 100 items\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_EmptyChildID(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	body := `{"children":["id1","","id3"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with empty child ID status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"children array contains empty ID\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_DuplicateIDs(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	body := `{"children":["id1","id2","id1"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with duplicate IDs status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"children array contains duplicate IDs\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_LongID(t *testing.T) {
	h := NewHandlers(nil, nil, nil)

	// Create a string longer than 100 characters
	longID := strings.Repeat("a", 101)
	body := `{"children":["` + longID + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with long ID status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"child ID exceeds maximum length of 100 characters\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_MethodNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "GET returns 405", method: http.MethodGet},
		{name: "PUT returns 405", method: http.MethodPut},
		{name: "DELETE returns 405", method: http.MethodDelete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil, nil)
			req := httptest.NewRequest(tt.method, "/api/v1/posts/t3_abc123/more-comments", nil)
			w := httptest.NewRecorder()

			h.GetMoreComments(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("GetMoreComments() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "POST" {
				t.Errorf("GetMoreComments() Allow header = %q, want %q", allow, "POST")
			}
		})
	}
}

func TestGetMoreComments_ContentType(t *testing.T) {
	// Test that responses have proper content type
	h := NewHandlers(nil, nil, nil)
	body := `{"children":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetMoreComments() Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestGetMoreComments_InvalidJSON(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"invalid request body\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_EmptyLinkID(t *testing.T) {
	// Test that empty linkID in path is rejected
	h := NewHandlers(nil, nil, nil)
	body := `{"children":["id1"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts//more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with empty linkID status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"linkID is required\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_PathTraversal(t *testing.T) {
	// Test that path traversal in linkID is rejected
	h := NewHandlers(nil, nil, nil)
	body := `{"children":["id1"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/../etc/more-comments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() with path traversal status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	expectedBody := "{\"error\":\"invalid linkID\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("GetMoreComments() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestGetMoreComments_ErrorMapping(t *testing.T) {
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
			err:                &graw.ValidationError{Field: "children", Reason: "invalid"},
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
			name:               "Generic error returns 500",
			err:                errors.New("database error"),
			expectedStatus:     http.StatusInternalServerError,
			expectedErrMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := mapErrorToStatus(tt.err)
			if status != tt.expectedStatus {
				t.Errorf("mapErrorToStatus() = %d, want %d", status, tt.expectedStatus)
			}

			message := getClientErrorMessage(tt.err, status)
			if message != tt.expectedErrMessage {
				t.Errorf("getClientErrorMessage() = %q, want %q", message, tt.expectedErrMessage)
			}
		})
	}
}

func TestGetHotPosts_Success_ReturnsPosts(t *testing.T) {
	// Create mock posts response
	mockResponse := &types.PostsResponse{
		Posts: []*types.Post{
			{
				ThingData: types.ThingData{
					ID:   "abc123",
					Name: "t3_abc123",
				},
				Votable: types.Votable{
					Score: 100,
				},
				Title:  "Test Post 1",
				Author: "user1",
			},
			{
				ThingData: types.ThingData{
					ID:   "def456",
					Name: "t3_def456",
				},
				Votable: types.Votable{
					Score: 200,
				},
				Title:  "Test Post 2",
				Author: "user2",
			},
		},
		AfterFullname: "t3_ghi789",
	}

	mock := &mockRedditClient{
		hotResponse: mockResponse,
		hotError:    nil,
	}

	h := NewHandlers(mock, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/hot?subreddit=golang&limit=10", nil)
	w := httptest.NewRecorder()

	h.GetHotPosts(w, req)

	// Verify successful status
	if w.Code != http.StatusOK {
		t.Errorf("GetHotPosts() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetHotPosts() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body contains expected data
	body := w.Body.String()
	expectedFields := []string{
		`"id":"abc123"`,
		`"title":"Test Post 1"`,
		`"author":"user1"`,
		`"score":100`,
		`"after":"t3_ghi789"`,
	}

	for _, field := range expectedFields {
		if !contains(body, field) {
			t.Errorf("GetHotPosts() body missing expected field: %q\nGot: %s", field, body)
		}
	}
}

func TestGetNewPosts_Success_ReturnsPosts(t *testing.T) {
	// Create mock posts response
	mockResponse := &types.PostsResponse{
		Posts: []*types.Post{
			{
				ThingData: types.ThingData{
					ID:   "new123",
					Name: "t3_new123",
				},
				Votable: types.Votable{
					Score: 50,
				},
				Title:  "New Post 1",
				Author: "newuser1",
			},
		},
		AfterFullname: "t3_new456",
	}

	mock := &mockRedditClient{
		newResponse: mockResponse,
		newError:    nil,
	}

	h := NewHandlers(mock, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/new?subreddit=golang&limit=25", nil)
	w := httptest.NewRecorder()

	h.GetNewPosts(w, req)

	// Verify successful status
	if w.Code != http.StatusOK {
		t.Errorf("GetNewPosts() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetNewPosts() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body contains expected data
	body := w.Body.String()
	expectedFields := []string{
		`"id":"new123"`,
		`"title":"New Post 1"`,
		`"author":"newuser1"`,
		`"score":50`,
	}

	for _, field := range expectedFields {
		if !contains(body, field) {
			t.Errorf("GetNewPosts() body missing expected field: %q\nGot: %s", field, body)
		}
	}
}

func TestGetComments_Success_ReturnsComments(t *testing.T) {
	// Create mock comments response
	mockResponse := &types.CommentsResponse{
		Post: &types.Post{
			ThingData: types.ThingData{
				ID:   "post123",
				Name: "t3_post123",
			},
			Votable: types.Votable{
				Score: 500,
			},
			Title:  "Test Post",
			Author: "postauthor",
		},
		Comments: []*types.Comment{
			{
				ThingData: types.ThingData{
					ID:   "comment1",
					Name: "t1_comment1",
				},
				Votable: types.Votable{
					Score: 10,
				},
				Author: "commenter1",
				Body:   "Great post!",
			},
			{
				ThingData: types.ThingData{
					ID:   "comment2",
					Name: "t1_comment2",
				},
				Votable: types.Votable{
					Score: 5,
				},
				Author: "commenter2",
				Body:   "I agree",
			},
		},
	}

	mock := &mockRedditClient{
		commentsResponse: mockResponse,
		commentsError:    nil,
	}

	h := NewHandlers(mock, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/posts/golang/post123/comments", nil)
	w := httptest.NewRecorder()

	h.GetComments(w, req)

	// Verify successful status
	if w.Code != http.StatusOK {
		t.Errorf("GetComments() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetComments() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body contains expected data
	body := w.Body.String()
	expectedFields := []string{
		`"id":"post123"`,
		`"title":"Test Post"`,
		`"id":"comment1"`,
		`"body":"Great post!"`,
		`"author":"commenter1"`,
	}

	for _, field := range expectedFields {
		if !contains(body, field) {
			t.Errorf("GetComments() body missing expected field: %q\nGot: %s", field, body)
		}
	}
}

func TestGetMoreComments_Success_ReturnsComments(t *testing.T) {
	// Create mock more comments response (as a slice of comment pointers)
	mockResponse := []*types.Comment{
		{
			ThingData: types.ThingData{
				ID:   "morecomment1",
				Name: "t1_morecomment1",
			},
			Votable: types.Votable{
				Score: 15,
			},
			Author: "user1",
			Body:   "Additional comment",
		},
		{
			ThingData: types.ThingData{
				ID:   "morecomment2",
				Name: "t1_morecomment2",
			},
			Votable: types.Votable{
				Score: 20,
			},
			Author: "user2",
			Body:   "Another comment",
		},
	}

	mock := &mockRedditClient{
		moreCommentsResponse: mockResponse,
		moreCommentsError:    nil,
	}

	h := NewHandlers(mock, nil, nil)
	body := `{"children":["child1","child2","child3"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/posts/t3_abc123/more-comments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GetMoreComments(w, req)

	// Verify successful status
	if w.Code != http.StatusOK {
		t.Errorf("GetMoreComments() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("GetMoreComments() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body contains expected data
	respBody := w.Body.String()
	expectedFields := []string{
		`"id":"morecomment1"`,
		`"body":"Additional comment"`,
		`"author":"user1"`,
		`"score":15`,
	}

	for _, field := range expectedFields {
		if !contains(respBody, field) {
			t.Errorf("GetMoreComments() body missing expected field: %q\nGot: %s", field, respBody)
		}
	}
}
