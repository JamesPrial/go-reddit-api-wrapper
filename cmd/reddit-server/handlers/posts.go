package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// GetHotPosts handles GET /api/v1/posts/hot requests.
// Returns hot (trending) posts.
//
// Query parameters:
//   - limit: Number of posts to return (1-100, default: 25)
//   - after: Pagination token for next page
//   - before: Pagination token for previous page
//   - subreddit: Subreddit name (optional, empty = frontpage)
//
// Authentication: Required
// Returns: Array of hot posts with pagination metadata
// Status codes:
//   - 200 OK: Successfully retrieved posts
//   - 400 Bad Request: Invalid query parameters
//   - 401 Unauthorized: Missing or invalid credentials
//   - 500 Internal Server Error: Server or API error
func (h *Handler) GetHotPosts(w http.ResponseWriter, r *http.Request) {
	h.handlePostsRequest(w, r, "hot")
}

// GetNewPosts handles GET /api/v1/posts/new requests.
// Returns new (recently submitted) posts.
//
// Query parameters:
//   - limit: Number of posts to return (1-100, default: 25)
//   - after: Pagination token for next page
//   - before: Pagination token for previous page
//   - subreddit: Subreddit name (optional, empty = frontpage)
//
// Authentication: Required
// Returns: Array of new posts with pagination metadata
// Status codes:
//   - 200 OK: Successfully retrieved posts
//   - 400 Bad Request: Invalid query parameters
//   - 401 Unauthorized: Missing or invalid credentials
//   - 500 Internal Server Error: Server or API error
func (h *Handler) GetNewPosts(w http.ResponseWriter, r *http.Request) {
	h.handlePostsRequest(w, r, "new")
}

// handlePostsRequest is the internal handler for both hot and new posts.
func (h *Handler) handlePostsRequest(w http.ResponseWriter, r *http.Request, postType string) {
	// Get pagination parameters
	limit, after, before, err := h.getPaginationParams(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error(), "validation_error")
		return
	}

	// Get subreddit from query params
	subreddit := r.URL.Query().Get("subreddit")

	// Use the shared Reddit client
	client := h.client

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Fetch posts
	request := &types.PostsRequest{
		Subreddit: subreddit,
		Pagination: types.Pagination{
			Limit:  limit,
			After:  after,
			Before: before,
		},
	}

	var response *types.PostsResponse
	switch postType {
	case "hot":
		response, err = client.GetHot(ctx, request)
	case "new":
		response, err = client.GetNew(ctx, request)
	default:
		h.respondError(w, http.StatusBadRequest, "Invalid post type", "validation_error")
		return
	}

	if err != nil {
		statusCode := errorToStatus(err)
		h.logger.Error("failed to fetch posts",
			slog.String("post_type", postType),
			slog.String("subreddit", subreddit),
			slog.String("error", err.Error()),
			slog.Int("status_code", statusCode),
		)
		h.respondError(w, statusCode, err.Error(), errorType(statusCode))
		return
	}

	// Build response with pagination metadata
	resp := Response{
		Data: response.Posts,
		Pagination: PaginationMeta{
			After:  response.AfterFullname,
			Before: response.BeforeFullname,
		},
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// GetComments handles GET /api/v1/posts/{subreddit}/{postID}/comments requests.
// Returns comments for a specific post.
//
// Path parameters:
//   - subreddit: Subreddit name
//   - postID: Post ID (without "t3_" prefix)
//
// Query parameters:
//   - limit: Number of comments to return (1-100, default: 25)
//   - after: Pagination token for next page
//   - before: Pagination token for previous page
//
// Authentication: Required
// Returns: Post data with array of comments and pagination metadata
// Status codes:
//   - 200 OK: Successfully retrieved comments
//   - 400 Bad Request: Invalid parameters
//   - 401 Unauthorized: Missing or invalid credentials
//   - 404 Not Found: Post not found
//   - 500 Internal Server Error: Server or API error
func (h *Handler) GetComments(w http.ResponseWriter, r *http.Request) {
	// Get path parameters
	subreddit := chi.URLParam(r, "subreddit")
	postID := chi.URLParam(r, "postID")

	if subreddit == "" || postID == "" {
		h.respondError(w, http.StatusBadRequest, "Subreddit and post ID are required", "validation_error")
		return
	}

	// Get pagination parameters
	limit, after, before, err := h.getPaginationParams(r)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error(), "validation_error")
		return
	}

	// Use the shared Reddit client
	client := h.client

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Fetch comments
	request := &types.CommentsRequest{
		Subreddit: subreddit,
		PostID:    postID,
		Pagination: types.Pagination{
			Limit:  limit,
			After:  after,
			Before: before,
		},
	}

	response, err := client.GetComments(ctx, request)
	if err != nil {
		statusCode := errorToStatus(err)
		h.logger.Error("failed to fetch comments",
			slog.String("subreddit", subreddit),
			slog.String("post_id", postID),
			slog.String("error", err.Error()),
			slog.Int("status_code", statusCode),
		)
		h.respondError(w, statusCode, err.Error(), errorType(statusCode))
		return
	}

	// Build response with pagination metadata
	respData := map[string]interface{}{
		"post":     response.Post,
		"comments": response.Comments,
	}

	resp := Response{
		Data: respData,
		Pagination: PaginationMeta{
			After:  response.AfterFullname,
			Before: response.BeforeFullname,
		},
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// MoreCommentsRequest represents the request body for getting more comments.
type MoreCommentsRequest struct {
	LinkID     string   `json:"link_id"`
	CommentIDs []string `json:"comment_ids"`
}

// GetMoreComments handles POST /api/v1/posts/{linkID}/more-comments requests.
// Fetches additional comments referenced in a post's comment tree.
//
// Path parameters:
//   - linkID: Full name or ID of the post (with or without "t3_" prefix)
//
// Request body:
//   - link_id: Full name or ID of the post (can be in path or body)
//   - comment_ids: Array of comment IDs to load (required, max 100)
//
// Authentication: Required
// Returns: Array of loaded comments
// Status codes:
//   - 200 OK: Successfully loaded comments
//   - 400 Bad Request: Invalid parameters
//   - 401 Unauthorized: Missing or invalid credentials
//   - 500 Internal Server Error: Server or API error
func (h *Handler) GetMoreComments(w http.ResponseWriter, r *http.Request) {
	// Get linkID from path
	linkID := chi.URLParam(r, "linkID")

	// Limit request body size to 1MB and disallow unknown fields
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	// Parse request body
	var req MoreCommentsRequest
	if err := decoder.Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", "validation_error")
		return
	}

	// Use linkID from path if not in body
	if req.LinkID == "" {
		req.LinkID = linkID
	}

	// Validate required fields
	if req.LinkID == "" {
		h.respondError(w, http.StatusBadRequest, "Link ID is required", "validation_error")
		return
	}

	if len(req.CommentIDs) == 0 {
		h.respondError(w, http.StatusBadRequest, "At least one comment ID is required", "validation_error")
		return
	}

	// Validate comment IDs count (max 100)
	if len(req.CommentIDs) > 100 {
		h.respondError(w, http.StatusBadRequest, "Maximum 100 comment IDs allowed per request", "validation_error")
		return
	}

	// Use the shared Reddit client
	client := h.client

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Fetch more comments
	moreRequest := &types.MoreCommentsRequest{
		LinkID:     req.LinkID,
		CommentIDs: req.CommentIDs,
	}

	comments, err := client.GetMoreComments(ctx, moreRequest)
	if err != nil {
		statusCode := errorToStatus(err)
		h.logger.Error("failed to fetch more comments",
			slog.String("link_id", req.LinkID),
			slog.String("error", err.Error()),
			slog.Int("status_code", statusCode),
		)
		h.respondError(w, statusCode, err.Error(), errorType(statusCode))
		return
	}

	// Return comments in response
	resp := Response{
		Data: comments,
	}

	h.respondJSON(w, http.StatusOK, resp)
}
