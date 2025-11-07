package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// savePostRequest represents the JSON request body for SavePost.
type savePostRequest struct {
	Post *types.Post `json:"post"`
}

// savePostResponse represents the JSON response for SavePost.
type savePostResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
}

// SavePost handles POST /api/v1/storage/posts requests.
// It saves a single post to the storage backend.
// Request body (JSON):
//
//	{
//	  "post": { /* types.Post object */ }
//	}
//
// Response on success (201):
//
//	{
//	  "success": true,
//	  "id": "abc123"
//	}
func (h *Handlers) SavePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var reqBody savePostRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		slog.Error("failed to decode save post request body", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if reqBody.Post == nil {
		respondError(w, http.StatusBadRequest, "post is required")
		return
	}

	if reqBody.Post.ID == "" {
		respondError(w, http.StatusBadRequest, "post ID is required")
		return
	}

	// Note: Store field is added to Handlers struct by caller
	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	if err := h.store.UpsertPost(r.Context(), reqBody.Post); err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to save post",
			"error", err,
			"postID", reqBody.Post.ID,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusCreated, savePostResponse{
		Success: true,
		ID:      reqBody.Post.ID,
	})
}

// listSavedPostsResponse represents the JSON response for ListSavedPosts.
type listSavedPostsResponse struct {
	Posts []*types.Post `json:"posts"`
	Total int64         `json:"total"`
}

// ListSavedPosts handles GET /api/v1/storage/posts requests.
// It lists saved posts with optional filtering and sorting.
// Query parameters:
//   - subreddit: filter by subreddit name
//   - author: filter by post author
//   - min_score: minimum post score (inclusive)
//   - max_age: maximum age of posts (e.g., "24h", "7d")
//   - sort_by: field to sort by (e.g., "created_utc", "score", "num_comments", "title")
//   - sort_dir: sort direction ("asc" or "desc")
//   - limit: number of posts to return (default 25, max 100)
//   - offset: number of posts to skip for pagination (default 0)
func (h *Handlers) ListSavedPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	opts := parseListPostsOptions(r)

	posts, err := h.store.ListPosts(r.Context(), opts)
	if err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to list saved posts",
			"error", err,
			"subreddit", opts.Subreddit,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	total, err := h.store.CountPosts(r.Context(), opts)
	if err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to count saved posts",
			"error", err,
			"subreddit", opts.Subreddit,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	if posts == nil {
		posts = make([]*types.Post, 0)
	}

	respondJSON(w, http.StatusOK, listSavedPostsResponse{
		Posts: posts,
		Total: total,
	})
}

// GetSavedPost handles GET /api/v1/storage/posts/{id} requests.
// It retrieves a single saved post by ID.
// URL path parameters:
//   - id: post ID without prefix (e.g., "abc123")
func (h *Handlers) GetSavedPost(w http.ResponseWriter, r *http.Request, postID string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if postID == "" {
		respondError(w, http.StatusBadRequest, "post ID is required")
		return
	}

	// Validate post ID for safety
	if !validatePathParameter(postID) {
		slog.Warn("invalid post ID parameter", "postID", postID)
		respondError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	post, err := h.store.GetPost(r.Context(), postID)
	if err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to get saved post",
			"error", err,
			"postID", postID,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusOK, post)
}

// deleteSavedPostResponse represents the JSON response for DeleteSavedPost.
type deleteSavedPostResponse struct {
	Success bool `json:"success"`
}

// DeleteSavedPost handles DELETE /api/v1/storage/posts/{id} requests.
// It removes a saved post by ID.
// URL path parameters:
//   - id: post ID without prefix (e.g., "abc123")
func (h *Handlers) DeleteSavedPost(w http.ResponseWriter, r *http.Request, postID string) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if postID == "" {
		respondError(w, http.StatusBadRequest, "post ID is required")
		return
	}

	// Validate post ID for safety
	if !validatePathParameter(postID) {
		slog.Warn("invalid post ID parameter", "postID", postID)
		respondError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	if err := h.store.DeletePost(r.Context(), postID); err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to delete saved post",
			"error", err,
			"postID", postID,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusOK, deleteSavedPostResponse{
		Success: true,
	})
}

// saveCommentsRequest represents the JSON request body for SaveComments.
type saveCommentsRequest struct {
	Comments []*types.Comment `json:"comments"`
}

// saveCommentsResponse represents the JSON response for SaveComments.
type saveCommentsResponse struct {
	Success bool `json:"success"`
	Count   int  `json:"count"`
}

// SaveComments handles POST /api/v1/storage/posts/{id}/comments requests.
// It saves multiple comments to the storage backend.
// URL path parameters:
//   - id: post ID without prefix (e.g., "abc123")
//
// Request body (JSON):
//
//	{
//	  "comments": [ /* array of types.Comment objects */ ]
//	}
//
// Response on success (201):
//
//	{
//	  "success": true,
//	  "count": 42
//	}
func (h *Handlers) SaveComments(w http.ResponseWriter, r *http.Request, postID string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if postID == "" {
		respondError(w, http.StatusBadRequest, "post ID is required")
		return
	}

	// Validate post ID for safety
	if !validatePathParameter(postID) {
		slog.Warn("invalid post ID parameter", "postID", postID)
		respondError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	// Limit request body size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var reqBody saveCommentsRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		slog.Error("failed to decode save comments request body", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(reqBody.Comments) == 0 {
		respondError(w, http.StatusBadRequest, "comments array cannot be empty")
		return
	}

	// Add maximum limit on comments array
	if len(reqBody.Comments) > 1000 {
		respondError(w, http.StatusBadRequest, "comments array exceeds maximum of 1000 items")
		return
	}

	// Add nil check for individual comment elements
	for i, comment := range reqBody.Comments {
		if comment == nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("comments[%d] cannot be nil", i))
			return
		}
		if comment.ID == "" {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("comments[%d].id cannot be empty", i))
			return
		}
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	if err := h.store.UpsertComments(r.Context(), reqBody.Comments); err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to save comments",
			"error", err,
			"postID", postID,
			"commentCount", len(reqBody.Comments),
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusCreated, saveCommentsResponse{
		Success: true,
		Count:   len(reqBody.Comments),
	})
}

// getCommentTreeResponse represents the JSON response for GetCommentTree.
type getCommentTreeResponse struct {
	Comments []*types.Comment `json:"comments"`
	Count    int              `json:"count"`
}

// GetCommentTree handles GET /api/v1/storage/posts/{id}/comments requests.
// It retrieves the comment tree for a specific post.
// URL path parameters:
//   - id: post ID without prefix (e.g., "abc123")
//
// Query parameters:
//   - max_depth: maximum depth of replies to retrieve (0 for unlimited)
//   - sort_by: field to sort comments by (e.g., "score", "created_utc")
//   - sort_dir: sort direction ("asc" or "desc")
func (h *Handlers) GetCommentTree(w http.ResponseWriter, r *http.Request, postID string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if postID == "" {
		respondError(w, http.StatusBadRequest, "post ID is required")
		return
	}

	// Validate post ID for safety
	if !validatePathParameter(postID) {
		slog.Warn("invalid post ID parameter", "postID", postID)
		respondError(w, http.StatusBadRequest, "invalid post ID")
		return
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	opts := parseCommentTreeOptions(r)

	comments, err := h.store.GetCommentTree(r.Context(), postID, opts)
	if err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to get comment tree",
			"error", err,
			"postID", postID,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	if comments == nil {
		comments = make([]*types.Comment, 0)
	}

	respondJSON(w, http.StatusOK, getCommentTreeResponse{
		Comments: comments,
		Count:    len(comments),
	})
}

// getStorageStatsResponse represents the JSON response for GetStorageStats.
type getStorageStatsResponse struct {
	PostCount      int64      `json:"post_count"`
	CommentCount   int64      `json:"comment_count"`
	OldestEntry    *time.Time `json:"oldest_entry,omitempty"`
	NewestEntry    *time.Time `json:"newest_entry,omitempty"`
	TotalSizeBytes int64      `json:"total_size_bytes"`
}

// GetStorageStats handles GET /api/v1/storage/stats requests.
// It returns statistics about the stored data (post/comment counts, ages, size).
func (h *Handlers) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	stats, err := h.store.GetStats(r.Context())
	if err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to get storage stats",
			"error", err,
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	resp := getStorageStatsResponse{
		PostCount:      stats.PostCount,
		CommentCount:   stats.CommentCount,
		TotalSizeBytes: stats.TotalSizeBytes,
	}

	// Only include oldest/newest entry if they are set
	if !stats.OldestEntry.IsZero() {
		resp.OldestEntry = &stats.OldestEntry
	}
	if !stats.NewestEntry.IsZero() {
		resp.NewestEntry = &stats.NewestEntry
	}

	respondJSON(w, http.StatusOK, resp)
}

// bulkSaveFromSubredditRequest represents the JSON request body for BulkSaveFromSubreddit.
type bulkSaveFromSubredditRequest struct {
	Subreddit string `json:"subreddit"`
	Sort      string `json:"sort"` // "hot" or "new"
	Limit     int    `json:"limit"`
}

// bulkSaveFromSubredditResponse represents the JSON response for BulkSaveFromSubreddit.
type bulkSaveFromSubredditResponse struct {
	Success bool          `json:"success"`
	Saved   int           `json:"saved"`
	Posts   []*types.Post `json:"posts"`
}

// BulkSaveFromSubreddit handles POST /api/v1/storage/bulk-save requests.
// It fetches posts from a subreddit and saves them to storage.
// Request body (JSON):
//
//	{
//	  "subreddit": "golang",
//	  "sort": "hot",
//	  "limit": 25
//	}
//
// Response on success (201):
//
//	{
//	  "success": true,
//	  "saved": 25,
//	  "posts": [ /* array of saved posts */ ]
//	}
func (h *Handlers) BulkSaveFromSubreddit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var reqBody bulkSaveFromSubredditRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		slog.Error("failed to decode bulk save request body", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if reqBody.Subreddit == "" {
		respondError(w, http.StatusBadRequest, "subreddit is required")
		return
	}

	if reqBody.Sort == "" {
		reqBody.Sort = "hot"
	}

	if reqBody.Limit <= 0 {
		reqBody.Limit = 25
	}
	if reqBody.Limit > 100 {
		reqBody.Limit = 100
	}

	if h.store == nil {
		slog.Error("storage backend not initialized")
		respondError(w, http.StatusInternalServerError, "storage not available")
		return
	}

	// Fetch posts from Reddit API
	var posts []*types.Post

	pagination := types.Pagination{
		Limit: reqBody.Limit,
	}

	switch strings.ToLower(reqBody.Sort) {
	case "new":
		resp, err := h.client.GetNew(r.Context(), &types.PostsRequest{
			Subreddit:  reqBody.Subreddit,
			Pagination: pagination,
		})
		if err != nil {
			status := mapErrorToStatus(err)
			slog.Error("failed to fetch new posts from Reddit",
				"error", err,
				"subreddit", reqBody.Subreddit,
				"status", status)
			respondError(w, status, getClientErrorMessage(err, status))
			return
		}
		posts = resp.Posts

	case "hot":
		fallthrough
	default:
		resp, err := h.client.GetHot(r.Context(), &types.PostsRequest{
			Subreddit:  reqBody.Subreddit,
			Pagination: pagination,
		})
		if err != nil {
			status := mapErrorToStatus(err)
			slog.Error("failed to fetch hot posts from Reddit",
				"error", err,
				"subreddit", reqBody.Subreddit,
				"status", status)
			respondError(w, status, getClientErrorMessage(err, status))
			return
		}
		posts = resp.Posts
	}

	if len(posts) == 0 {
		// No posts to save, return empty success response
		respondJSON(w, http.StatusCreated, bulkSaveFromSubredditResponse{
			Success: true,
			Saved:   0,
			Posts:   make([]*types.Post, 0),
		})
		return
	}

	// Save all posts to storage
	if err := h.store.UpsertPosts(r.Context(), posts); err != nil {
		status := mapStorageErrorToStatus(err)
		slog.Error("failed to bulk save posts",
			"error", err,
			"subreddit", reqBody.Subreddit,
			"postCount", len(posts),
			"status", status)
		respondError(w, status, getStorageErrorMessage(err, status))
		return
	}

	respondJSON(w, http.StatusCreated, bulkSaveFromSubredditResponse{
		Success: true,
		Saved:   len(posts),
		Posts:   posts,
	})
}

// mapStorageErrorToStatus maps storage errors to HTTP status codes.
// It uses errors.As to check storage error types.
func mapStorageErrorToStatus(err error) int {
	var notFoundErr *storage.NotFoundError
	if errors.As(err, &notFoundErr) {
		return http.StatusNotFound
	}

	var validationErr *storage.ValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest
	}

	var conflictErr *storage.ConflictError
	if errors.As(err, &conflictErr) {
		return http.StatusConflict
	}

	var integrityErr *storage.IntegrityError
	if errors.As(err, &integrityErr) {
		return http.StatusBadRequest
	}

	var transactionErr *storage.TransactionError
	if errors.As(err, &transactionErr) {
		return http.StatusInternalServerError
	}

	var databaseErr *storage.DatabaseError
	if errors.As(err, &databaseErr) {
		return http.StatusInternalServerError
	}

	// Default to internal server error
	return http.StatusInternalServerError
}

// getStorageErrorMessage returns a sanitized error message for the client
// based on the storage error type and HTTP status code.
func getStorageErrorMessage(err error, status int) string {
	// For validation errors, we can be more specific
	var validationErr *storage.ValidationError
	if errors.As(err, &validationErr) {
		return "invalid request parameters"
	}

	// Map status codes to safe generic messages
	switch status {
	case http.StatusBadRequest:
		return "bad request"
	case http.StatusNotFound:
		return "resource not found"
	case http.StatusConflict:
		return "resource conflict"
	case http.StatusInternalServerError:
		return "internal server error"
	default:
		return "an error occurred"
	}
}

// parseListPostsOptions extracts ListPostsOptions from the request query string.
// It parses filters (subreddit, author, min_score, max_age) and pagination/sorting parameters.
func parseListPostsOptions(r *http.Request) *storage.ListPostsOptions {
	query := r.URL.Query()

	// Add length validation for filters
	subreddit := query.Get("subreddit")
	if len(subreddit) > 100 {
		subreddit = ""
	}

	author := query.Get("author")
	if len(author) > 100 {
		author = ""
	}

	opts := &storage.ListPostsOptions{
		Subreddit: subreddit,
		Author:    author,
	}

	// Add validation of sort_by field
	if sortBy := query.Get("sort_by"); sortBy != "" {
		validSortFields := map[string]bool{
			"created_utc":  true,
			"score":        true,
			"num_comments": true,
			"title":        true,
		}
		if validSortFields[sortBy] {
			opts.SortBy = sortBy
		}
		// else: ignore invalid value, use storage default
	}

	// Add validation of sort_dir field
	if sortDir := query.Get("sort_dir"); sortDir != "" {
		sortDirLower := strings.ToLower(sortDir)
		if sortDirLower == "asc" || sortDirLower == "desc" {
			opts.SortDir = sortDirLower
		}
	}

	// Parse min_score (allow negative scores)
	if minScoreStr := query.Get("min_score"); minScoreStr != "" {
		if minScore, err := strconv.Atoi(minScoreStr); err == nil {
			opts.MinScore = minScore
		}
	}

	// Parse max_age (e.g., "24h", "7d")
	if maxAgeStr := query.Get("max_age"); maxAgeStr != "" {
		if maxAge, err := time.ParseDuration(maxAgeStr); err == nil && maxAge > 0 {
			opts.MaxAge = maxAge
		}
	}

	// Parse limit with default of 25 and max of 100
	limit := 25
	if limitStr := query.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
			if limit < 1 {
				limit = 1
			}
			if limit > 100 {
				limit = 100
			}
		}
	}
	opts.Limit = limit

	// Parse offset
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}

	return opts
}

// parseCommentTreeOptions extracts CommentTreeOptions from the request query string.
// It parses max_depth, sort_by, and sort_dir parameters.
func parseCommentTreeOptions(r *http.Request) *storage.CommentTreeOptions {
	query := r.URL.Query()

	opts := &storage.CommentTreeOptions{}

	// Add validation of sort_by field
	if sortBy := query.Get("sort_by"); sortBy != "" {
		validSortFields := map[string]bool{
			"score":       true,
			"created_utc": true,
			"created":     true,
		}
		if validSortFields[sortBy] {
			opts.SortBy = sortBy
		}
	}

	// Add validation of sort_dir field
	if sortDir := query.Get("sort_dir"); sortDir != "" {
		sortDirLower := strings.ToLower(sortDir)
		if sortDirLower == "asc" || sortDirLower == "desc" {
			opts.SortDir = sortDirLower
		}
	}

	// Parse max_depth (0 = unlimited, must be non-negative)
	if maxDepthStr := query.Get("max_depth"); maxDepthStr != "" {
		if maxDepth, err := strconv.Atoi(maxDepthStr); err == nil && maxDepth >= 0 {
			opts.MaxDepth = maxDepth
		}
	}

	return opts
}
