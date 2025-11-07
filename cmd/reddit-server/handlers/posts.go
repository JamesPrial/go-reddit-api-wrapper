package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// GetHotPosts handles GET /api/v1/posts/hot requests.
// It retrieves hot posts from a subreddit (or frontpage if no subreddit specified).
// Query parameters:
//   - subreddit: optional subreddit name (empty for frontpage)
//   - limit: number of posts to retrieve (default 25, max 100)
//   - after: pagination cursor for next page
//   - before: pagination cursor for previous page
func (h *Handlers) GetHotPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	subreddit := r.URL.Query().Get("subreddit")
	pagination := parsePagination(r)

	req := &types.PostsRequest{
		Subreddit:  subreddit,
		Pagination: pagination,
	}

	resp, err := h.client.GetHot(r.Context(), req)
	if err != nil {
		status := mapErrorToStatus(err)
		slog.Error("failed to get hot posts",
			"error", err,
			"subreddit", subreddit,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetNewPosts handles GET /api/v1/posts/new requests.
// It retrieves new posts from a subreddit (or frontpage if no subreddit specified).
// Query parameters:
//   - subreddit: optional subreddit name (empty for frontpage)
//   - limit: number of posts to retrieve (default 25, max 100)
//   - after: pagination cursor for next page
//   - before: pagination cursor for previous page
func (h *Handlers) GetNewPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	subreddit := r.URL.Query().Get("subreddit")
	pagination := parsePagination(r)

	req := &types.PostsRequest{
		Subreddit:  subreddit,
		Pagination: pagination,
	}

	resp, err := h.client.GetNew(r.Context(), req)
	if err != nil {
		status := mapErrorToStatus(err)
		slog.Error("failed to get new posts",
			"error", err,
			"subreddit", subreddit,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetComments handles GET /api/v1/posts/{subreddit}/{postID}/comments requests.
// It retrieves comments for a specific post.
// URL path parameters:
//   - subreddit: subreddit name (required)
//   - postID: post ID without prefix (required)
//
// Query parameters:
//   - limit: number of comments to retrieve (default 25, max 100)
//   - after: pagination cursor for next page
//   - before: pagination cursor for previous page
func (h *Handlers) GetComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract path parameters from URL
	// Expected pattern: /api/v1/posts/{subreddit}/{postID}/comments
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/posts/")
	path = strings.TrimSuffix(path, "/comments")

	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		respondError(w, http.StatusBadRequest, "invalid URL format: expected /api/v1/posts/{subreddit}/{postID}/comments")
		return
	}

	subreddit := parts[0]
	postID := parts[1]

	if subreddit == "" {
		respondError(w, http.StatusBadRequest, "subreddit is required")
		return
	}

	if postID == "" {
		respondError(w, http.StatusBadRequest, "postID is required")
		return
	}

	// Validate path parameters for safety
	if !validatePathParameter(subreddit) {
		slog.Warn("invalid subreddit parameter", "subreddit", subreddit)
		respondError(w, http.StatusBadRequest, "invalid subreddit")
		return
	}

	if !validatePathParameter(postID) {
		slog.Warn("invalid postID parameter", "postID", postID)
		respondError(w, http.StatusBadRequest, "invalid postID")
		return
	}

	pagination := parsePagination(r)

	req := &types.CommentsRequest{
		Subreddit:  subreddit,
		PostID:     postID,
		Pagination: pagination,
	}

	resp, err := h.client.GetComments(r.Context(), req)
	if err != nil {
		status := mapErrorToStatus(err)
		slog.Error("failed to get comments",
			"error", err,
			"subreddit", subreddit,
			"postID", postID,
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// moreCommentsRequest represents the JSON request body for GetMoreComments.
type moreCommentsRequest struct {
	Children []string `json:"children"`
}

// GetMoreComments handles POST /api/v1/posts/{linkID}/more-comments requests.
// It expands previously truncated comment trees.
// URL path parameters:
//   - linkID: post link ID (fullname like "t3_abc123") (required)
//
// Request body (JSON):
//
//	{
//	  "children": ["id1", "id2", ...]
//	}
func (h *Handlers) GetMoreComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 1MB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	// Extract linkID from URL path
	// Expected pattern: /api/v1/posts/{linkID}/more-comments
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/posts/")
	path = strings.TrimSuffix(path, "/more-comments")

	linkID := path

	if linkID == "" {
		respondError(w, http.StatusBadRequest, "linkID is required")
		return
	}

	// Validate linkID for safety
	if !validatePathParameter(linkID) {
		slog.Warn("invalid linkID parameter", "linkID", linkID)
		respondError(w, http.StatusBadRequest, "invalid linkID")
		return
	}

	// Parse request body
	var reqBody moreCommentsRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		slog.Error("failed to decode request body", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Comprehensive validation of children array
	if len(reqBody.Children) == 0 {
		respondError(w, http.StatusBadRequest, "children array cannot be empty")
		return
	}

	if len(reqBody.Children) > 100 {
		respondError(w, http.StatusBadRequest, "children array exceeds maximum of 100 items")
		return
	}

	// Track seen IDs to detect duplicates
	seenIDs := make(map[string]bool, len(reqBody.Children))

	// Validate each child ID
	for i, childID := range reqBody.Children {
		// Check if ID is empty
		if childID == "" {
			respondError(w, http.StatusBadRequest, "children array contains empty ID")
			return
		}

		// Check ID length
		if len(childID) > 100 {
			slog.Warn("child ID exceeds maximum length", "index", i, "length", len(childID))
			respondError(w, http.StatusBadRequest, "child ID exceeds maximum length of 100 characters")
			return
		}

		// Check for duplicates
		if seenIDs[childID] {
			slog.Warn("children array contains duplicate ID", "childID", childID)
			respondError(w, http.StatusBadRequest, "children array contains duplicate IDs")
			return
		}
		seenIDs[childID] = true
	}

	req := &types.MoreCommentsRequest{
		LinkID:     linkID,
		CommentIDs: reqBody.Children,
	}

	resp, err := h.client.GetMoreComments(r.Context(), req)
	if err != nil {
		status := mapErrorToStatus(err)
		slog.Error("failed to get more comments",
			"error", err,
			"linkID", linkID,
			"childrenCount", len(reqBody.Children),
			"status", status)
		respondError(w, status, getClientErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
