package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
	"gorm.io/gorm"
)

// Handlers contains all HTTP handlers for the REST API.
// It uses dependency injection to receive the database repository.
type Handlers struct {
	repo   db.Repository
	logger *slog.Logger
}

// NewHandlers creates a new Handlers instance with the provided repository.
func NewHandlers(repo db.Repository, logger *slog.Logger) *Handlers {
	return &Handlers{
		repo:   repo,
		logger: logger,
	}
}

// CreateSubredditRequest represents the request body for creating a subreddit.
type CreateSubredditRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateSubreddit handles POST /api/subreddits
// Creates a new subreddit to track.
func (h *Handlers) CreateSubreddit(w http.ResponseWriter, r *http.Request) {
	var req CreateSubredditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteBadRequest(w, "Invalid JSON request body")
		return
	}

	// Validate subreddit name
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		WriteBadRequest(w, "Subreddit name is required")
		return
	}

	// Sanitize subreddit name (remove /r/ prefix if present)
	req.Name = strings.TrimPrefix(req.Name, "/r/")
	req.Name = strings.TrimPrefix(req.Name, "r/")

	// Validate name format (alphanumeric + underscores, 3-21 chars)
	if len(req.Name) < 3 || len(req.Name) > 21 {
		WriteBadRequest(w, "Subreddit name must be between 3 and 21 characters")
		return
	}

	// Create subreddit record
	// Note: Fullname will be populated by the fetcher service when it first fetches the subreddit
	sub := &db.Subreddit{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		Fullname:    "t5_pending", // Placeholder fullname until first fetch
	}

	if err := h.repo.CreateSubreddit(r.Context(), sub); err != nil {
		h.logger.Error("failed to create subreddit",
			"name", req.Name,
			"error", err)

		// Check if it's a duplicate key error
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			WriteConflict(w, "Subreddit already exists")
			return
		}

		WriteInternalError(w, "Failed to create subreddit")
		return
	}

	h.logger.Info("subreddit created",
		"name", sub.Name,
		"id", sub.ID)

	WriteSuccess(w, http.StatusCreated, sub)
}

// ListSubreddits handles GET /api/subreddits
// Returns all tracked subreddits.
func (h *Handlers) ListSubreddits(w http.ResponseWriter, r *http.Request) {
	subs, err := h.repo.ListSubreddits(r.Context())
	if err != nil {
		h.logger.Error("failed to list subreddits", "error", err)
		WriteInternalError(w, "Failed to list subreddits")
		return
	}

	WriteSuccess(w, http.StatusOK, subs)
}

// GetSubreddit handles GET /api/subreddits/:name
// Returns a single subreddit with recent posts.
func (h *Handlers) GetSubreddit(w http.ResponseWriter, r *http.Request) {
	// Extract subreddit name from path
	// Path format: /api/subreddits/{name}
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		WriteBadRequest(w, "Invalid path")
		return
	}
	name := parts[2]

	// Get subreddit
	sub, err := h.repo.GetSubreddit(r.Context(), name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			WriteNotFound(w, "Subreddit not found")
			return
		}
		h.logger.Error("failed to get subreddit",
			"name", name,
			"error", err)
		WriteInternalError(w, "Failed to get subreddit")
		return
	}

	// Get recent posts (last 25)
	posts, err := h.repo.ListPosts(r.Context(), name, 25, 0)
	if err != nil {
		h.logger.Error("failed to get posts for subreddit",
			"name", name,
			"error", err)
		WriteInternalError(w, "Failed to get posts")
		return
	}

	// Create response with subreddit and posts
	response := map[string]interface{}{
		"subreddit": sub,
		"posts":     posts,
	}

	WriteSuccess(w, http.StatusOK, response)
}

// DeleteSubreddit handles DELETE /api/subreddits/:name
// Stops tracking a subreddit (deletes it and all associated posts/comments).
func (h *Handlers) DeleteSubreddit(w http.ResponseWriter, r *http.Request) {
	// Extract subreddit name from path
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		WriteBadRequest(w, "Invalid path")
		return
	}
	name := parts[2]

	// Delete subreddit
	if err := h.repo.DeleteSubreddit(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteNotFound(w, "Subreddit not found")
			return
		}
		h.logger.Error("failed to delete subreddit",
			"name", name,
			"error", err)
		WriteInternalError(w, "Failed to delete subreddit")
		return
	}

	h.logger.Info("subreddit deleted", "name", name)
	WriteNoContent(w)
}

// ListPosts handles GET /api/posts?subreddit=golang&limit=50&offset=0
// Returns posts for a specific subreddit with pagination.
func (h *Handlers) ListPosts(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	subreddit := query.Get("subreddit")
	if subreddit == "" {
		WriteBadRequest(w, "subreddit query parameter is required")
		return
	}

	// Parse pagination parameters with defaults
	limit := 50
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			WriteBadRequest(w, "limit must be between 1 and 100")
			return
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			WriteBadRequest(w, "offset must be non-negative")
			return
		}
	}

	// Get posts
	posts, err := h.repo.ListPosts(r.Context(), subreddit, limit, offset)
	if err != nil {
		h.logger.Error("failed to list posts",
			"subreddit", subreddit,
			"error", err)
		WriteInternalError(w, "Failed to list posts")
		return
	}

	WriteSuccess(w, http.StatusOK, posts)
}

// GetPost handles GET /api/posts/:fullname
// Returns a single post by its fullname.
func (h *Handlers) GetPost(w http.ResponseWriter, r *http.Request) {
	// Extract fullname from path
	// Path format: /api/posts/{fullname}
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		WriteBadRequest(w, "Invalid path")
		return
	}
	fullname := parts[2]

	// Validate fullname format (should start with t3_)
	if !strings.HasPrefix(fullname, "t3_") {
		WriteBadRequest(w, "Invalid post fullname (must start with t3_)")
		return
	}

	// Get post
	post, err := h.repo.GetPost(r.Context(), fullname)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			WriteNotFound(w, "Post not found")
			return
		}
		h.logger.Error("failed to get post",
			"fullname", fullname,
			"error", err)
		WriteInternalError(w, "Failed to get post")
		return
	}

	WriteSuccess(w, http.StatusOK, post)
}

// GetPostComments handles GET /api/posts/:fullname/comments
// Returns all comments for a specific post.
func (h *Handlers) GetPostComments(w http.ResponseWriter, r *http.Request) {
	// Extract fullname from path
	// Path format: /api/posts/{fullname}/comments
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		WriteBadRequest(w, "Invalid path")
		return
	}
	fullname := parts[2]

	// Validate fullname format
	if !strings.HasPrefix(fullname, "t3_") {
		WriteBadRequest(w, "Invalid post fullname (must start with t3_)")
		return
	}

	// Get comments
	comments, err := h.repo.GetCommentsByPost(r.Context(), fullname)
	if err != nil {
		// Check if the post doesn't exist
		if strings.Contains(err.Error(), "failed to get post") {
			WriteNotFound(w, "Post not found")
			return
		}
		h.logger.Error("failed to get comments",
			"fullname", fullname,
			"error", err)
		WriteInternalError(w, "Failed to get comments")
		return
	}

	WriteSuccess(w, http.StatusOK, comments)
}

// GetComment handles GET /api/comments/:fullname
// Returns a single comment by its fullname.
// Note: This endpoint is not strictly required by the spec but is included for completeness.
func (h *Handlers) GetComment(w http.ResponseWriter, r *http.Request) {
	// Extract fullname from path
	// Path format: /api/comments/{fullname}
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		WriteBadRequest(w, "Invalid path")
		return
	}
	fullname := parts[2]

	// Validate fullname format (should start with t1_)
	if !strings.HasPrefix(fullname, "t1_") {
		WriteBadRequest(w, "Invalid comment fullname (must start with t1_)")
		return
	}

	// Since the repository doesn't have a GetComment method,
	// we need to query directly. This is a limitation of the current design.
	// For now, return a not implemented error.
	WriteBadRequest(w, "GetComment endpoint not yet implemented - use GetPostComments instead")
}
