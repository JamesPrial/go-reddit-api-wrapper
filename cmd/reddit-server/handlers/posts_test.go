package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
)

// TestGetComments_MissingParams tests comments endpoint with missing path parameters
func TestGetComments_MissingSubreddit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	req := NewAuthenticatedRequest("GET", "/api/v1/posts//post123/comments", nil)
	chiCtx := chi.NewRouteContext()
	// Don't add subreddit
	chiCtx.URLParams.Add("postID", "post123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetComments() status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestGetComments_MissingPostID tests comments endpoint without post ID
func TestGetComments_MissingPostID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	req := NewAuthenticatedRequest("GET", "/api/v1/posts/golang//comments", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("subreddit", "golang")
	// Don't add postID
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetComments() status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestGetComments_InvalidPagination tests comments with invalid limit
func TestGetComments_InvalidPagination(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	req := NewAuthenticatedRequest("GET", "/api/v1/posts/golang/abc123/comments?limit=200", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("subreddit", "golang")
	chiCtx.URLParams.Add("postID", "abc123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetComments() status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestGetMoreComments_InvalidBody tests more comments with invalid JSON body
func TestGetMoreComments_InvalidBody(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	req := NewAuthenticatedRequest("POST", "/api/v1/posts/post123/more-comments", bytes.NewReader([]byte("invalid json")))
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("linkID", "post123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if resp.Error.Type != "validation_error" {
			t.Errorf("GetMoreComments() error type = %s, want validation_error", resp.Error.Type)
		}
	}
}

// TestGetMoreComments_MissingLinkID tests more comments without link ID
func TestGetMoreComments_MissingLinkID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	bodyData := MoreCommentsRequest{
		CommentIDs: []string{"c1"},
	}
	bodyBytes, _ := json.Marshal(bodyData)

	req := NewAuthenticatedRequest("POST", "/api/v1/posts//more-comments", bytes.NewReader(bodyBytes))
	chiCtx := chi.NewRouteContext()
	// Don't add linkID
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestGetMoreComments_MissingCommentIDs tests more comments without comment IDs
func TestGetMoreComments_MissingCommentIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	bodyData := MoreCommentsRequest{
		LinkID: "t3_post123",
		// Empty comment IDs
	}
	bodyBytes, _ := json.Marshal(bodyData)

	req := NewAuthenticatedRequest("POST", "/api/v1/posts/post123/more-comments", bytes.NewReader(bodyBytes))
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("linkID", "post123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	req = AddCredentialsToContext(req, "test-id", "test-secret", "test-agent")

	w := httptest.NewRecorder()
	handler.GetMoreComments(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("GetMoreComments() status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if resp.Error.Type != "validation_error" {
			t.Errorf("GetMoreComments() error type = %s, want validation_error", resp.Error.Type)
		}
	}
}

// TestGetComments_NoAPIKey tests that the endpoint requires API key authentication
func TestGetComments_NoAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	config := config.CORS{
		AllowedOrigins: "*",
		AllowedMethods: "GET,POST,OPTIONS",
		AllowedHeaders: "Content-Type,Authorization",
		MaxAge:         300,
	}
	router := handler.Router(config, []string{testAPIKey})

	// Create request WITHOUT API key
	req := httptest.NewRequest("GET", "/api/v1/posts/golang/abc123/comments", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetComments without API key: expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestGetMoreComments_NoAPIKey tests that the endpoint requires API key authentication
func TestGetMoreComments_NoAPIKey(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	config := config.CORS{
		AllowedOrigins: "*",
		AllowedMethods: "GET,POST,OPTIONS",
		AllowedHeaders: "Content-Type,Authorization",
		MaxAge:         300,
	}
	router := handler.Router(config, []string{testAPIKey})

	bodyData := MoreCommentsRequest{
		LinkID:     "t3_post123",
		CommentIDs: []string{"c1"},
	}
	bodyBytes, _ := json.Marshal(bodyData)

	// Create request WITHOUT API key
	req := httptest.NewRequest("POST", "/api/v1/posts/post123/more-comments", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GetMoreComments without API key: expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
