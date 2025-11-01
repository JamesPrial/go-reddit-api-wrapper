package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/reqid"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/validation"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"golang.org/x/time/rate"
)

const (
	// maxRequestBodySize is the maximum allowed size for request bodies (1 MB).
	maxRequestBodySize = 1 * 1024 * 1024
	// defaultPostLimit is the default number of posts to return.
	defaultPostLimit = 25
	// maxPostLimit is the maximum number of posts allowed per request.
	maxPostLimit = 100
	// defaultCommentLimit is the default number of comments to return.
	defaultCommentLimit = 25
	// maxCommentLimit is the maximum number of comments allowed per request.
	maxCommentLimit = 100
	// maxOffsetLimit prevents deep pagination performance issues.
	maxOffsetLimit = 10000
	// cacheOperationTimeout is the timeout for background cache operations (posts, comments).
	cacheOperationTimeout = 15 * time.Second
	// bulkSaveOperationTimeout is the timeout for bulk save operations.
	bulkSaveOperationTimeout = 10 * time.Minute
	// maxBulkSaveCount is the maximum number of posts allowed in a bulk save operation.
	maxBulkSaveCount = 2000
	// minBulkSaveCount is the minimum number of posts allowed in a bulk save operation.
	minBulkSaveCount = 1
	// bulkSaveBatchSize is the number of posts to save in each database batch.
	bulkSaveBatchSize = 500
	// bulkSavePageSize is the number of posts to fetch per Reddit API call.
	bulkSavePageSize = 100
	// maxConcurrentJobs is the maximum number of bulk save jobs that can run concurrently.
	maxConcurrentJobs = 10
)

// Validation patterns for input sanitization
var (
	// postIDRegex matches valid Reddit post IDs (base36, 6-10 chars typical but allow up to 20)
	postIDRegex = regexp.MustCompile(`^[0-9a-z]{1,20}$`)
	// paginationFullnameRegex matches valid fullname pagination cursors (t3_xxxxx or t1_xxxxx)
	paginationFullnameRegex = regexp.MustCompile(`^t[13]_[0-9a-z]+$`)
)

// PostData represents a simplified post for API responses.
type PostData struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Subreddit   string  `json:"subreddit"`
	SelfText    string  `json:"selftext"`
	Created     float64 `json:"created_utc"`
	UpvoteRatio float64 `json:"upvote_ratio"`
	IsSelf      bool    `json:"is_self"`
	Over18      bool    `json:"over_18"`
	Stickied    bool    `json:"stickied"`
	Locked      bool    `json:"locked"`
}

// CommentData represents a simplified comment for API responses.
type CommentData struct {
	ID        string  `json:"id"`
	Author    string  `json:"author"`
	Body      string  `json:"body"`
	Score     int     `json:"score"`
	Created   float64 `json:"created_utc"`
	Subreddit string  `json:"subreddit"`
	ParentID  string  `json:"parent_id"`
	Edited    bool    `json:"edited"`
}

// PostsResponse represents a collection of posts from an API response.
type PostsResponse struct {
	Posts          []*PostData `json:"posts"`
	AfterFullname  string      `json:"after"`
	BeforeFullname string      `json:"before"`
	Total          int64       `json:"total"`
}

// CommentsResponse represents comments from a post in an API response.
type CommentsResponse struct {
	Post           *PostData      `json:"post"`
	Comments       []*CommentData `json:"comments"`
	MoreIDs        []string       `json:"more_ids"`
	AfterFullname  string         `json:"after"`
	BeforeFullname string         `json:"before"`
}

// LoginRequest represents the JSON request body for the login endpoint.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the JSON response for successful login.
type LoginResponse struct {
	Success  bool   `json:"success"`
	Token    string `json:"token"`
	Username string `json:"username"`
}

// ErrorResponse represents a JSON error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// StatusResponse represents the JSON response for the status endpoint.
type StatusResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
	LinkKarma     int    `json:"link_karma"`
	CommentKarma  int    `json:"comment_karma"`
}

// SuccessResponse represents a generic success response.
type SuccessResponse struct {
	Success bool `json:"success"`
}

// BulkSaveRequest represents the JSON request body for the bulk save posts endpoint.
type BulkSaveRequest struct {
	Subreddit       string `json:"subreddit"`
	Sort            string `json:"sort"`
	Count           int    `json:"count"`
	IncludeComments bool   `json:"include_comments"`
}

// BulkSaveResponse represents the JSON response for initiating a bulk save operation.
type BulkSaveResponse struct {
	JobID   string `json:"job_id"`
	Message string `json:"message"`
}

// BulkSaveProgress represents the progress of a bulk save operation.
type BulkSaveProgress struct {
	Status        string `json:"status"`                 // "in_progress", "fetching_posts", "saving", "fetching_comments", "completed", "error"
	PostsSaved    int    `json:"posts_saved"`            // Number of posts successfully saved
	PostsTotal    int    `json:"posts_total"`            // Total number of posts to save
	CommentsSaved int    `json:"comments_saved"`         // Number of comments successfully saved
	Error         string `json:"error,omitempty"`        // Error message if status is "error"
	CompletedAt   string `json:"completed_at,omitempty"` // ISO 8601 timestamp when job completed
}

// bulkSaveJob represents a bulk save operation job.
type bulkSaveJob struct {
	mu            sync.RWMutex
	status        string
	postsSaved    int
	postsTotal    int
	commentsSaved int
	errorMsg      string
	completedAt   time.Time
}

// getProgress returns a snapshot of the current job progress.
func (j *bulkSaveJob) getProgress() BulkSaveProgress {
	j.mu.RLock()
	defer j.mu.RUnlock()

	progress := BulkSaveProgress{
		Status:        j.status,
		PostsSaved:    j.postsSaved,
		PostsTotal:    j.postsTotal,
		CommentsSaved: j.commentsSaved,
		Error:         j.errorMsg,
	}

	if !j.completedAt.IsZero() {
		progress.CompletedAt = j.completedAt.Format(time.RFC3339)
	}

	return progress
}

// bulkSaveJobs is a global map of job IDs to job states.
// It is protected by bulkSaveJobsMutex for concurrent access.
var (
	bulkSaveJobs      = make(map[string]*bulkSaveJob)
	bulkSaveJobsMutex sync.RWMutex
)

// extractBearerToken extracts the JWT token from the Authorization header.
// It expects the header to be in the format "Bearer <token>".
// Returns the token string or an error if the header is invalid.
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}

	// Parse "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", ErrInvalidAuthHeaderFormat
	}

	return parts[1], nil
}

// LoginHandler handles the POST /api/auth/login endpoint.
// It authenticates a user with Reddit and creates a session.
// Implements rate limiting to prevent brute force attacks.
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit (5 requests per second global limit)
	// TODO: Consider implementing per-IP rate limiting for production
	if !h.loginLimiter.Allow() {
		h.logger.Warn("login rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "too many login attempts, please try again later", requestID)
		return
	}

	// Parse request body with size limit
	var req LoginRequest
	limitedBody := io.LimitReader(r.Body, maxRequestBodySize)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "failed to read request body", requestID)
		return
	}
	defer r.Body.Close()

	// Check if body size limit was exceeded
	if len(body) >= maxRequestBodySize {
		h.logger.Warn("request body size limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusRequestEntityTooLarge, "request body too large", requestID)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.Error("failed to unmarshal request", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid request format", requestID)
		return
	}

	// Get Reddit credentials from environment
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		h.logger.Error("missing Reddit credentials in environment", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "server configuration error", requestID)
		return
	}

	// Determine authentication mode
	var authMode string
	var sessionUsername string

	if req.Username != "" && req.Password != "" {
		authMode = "user"
		sessionUsername = req.Username
	} else {
		authMode = "app-only"
		sessionUsername = "app-only"
	}

	// Create Reddit client configuration
	config := &graw.Config{
		Username:     req.Username, // Empty string for app-only mode
		Password:     req.Password, // Empty string for app-only mode
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "reddit-frontend-server/1.0 by /u/yourredditname",
		Logger:       h.logger,
	}

	// Authenticate with Reddit
	authCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	h.logger.Info("authenticating with Reddit", "auth_mode", authMode, slog.String("request_id", requestID))

	client, err := graw.NewClientWithContext(authCtx, config)
	if err != nil {
		h.logger.Error("failed to authenticate with Reddit", "auth_mode", authMode, "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid credentials", requestID)
		return
	}

	// Create session
	sessionID, token, err := h.sessionManager.CreateSession(sessionUsername, client)
	if err != nil {
		h.logger.Error("failed to create session", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to create session", requestID)
		return
	}

	h.logger.Info("user logged in", "auth_mode", authMode, "username", sessionUsername, "session_id", sessionID, slog.String("request_id", requestID))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(LoginResponse{
		Success:  true,
		Token:    token,
		Username: sessionUsername,
	}); err != nil {
		h.logger.Error("failed to encode login response", "error", err)
	}
}

// StatusHandler handles the GET /api/auth/status endpoint.
// It returns the authenticated user's information.
func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Extract JWT from Authorization header using helper
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// For app-only sessions, skip user info fetch
	if session.Username == "app-only" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", requestID)
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(StatusResponse{
			Authenticated: true,
			Username:      "app-only",
			LinkKarma:     0,
			CommentKarma:  0,
		}); err != nil {
			h.logger.Error("failed to encode status response", "error", err, slog.String("request_id", requestID))
		}
		return
	}

	// Get user info from Reddit for user-authenticated sessions
	userCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	accountData, err := session.RedditClient.Me(userCtx)
	if err != nil {
		h.logger.Error("failed to fetch user info", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to fetch user info", requestID)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(StatusResponse{
		Authenticated: true,
		Username:      session.Username,
		LinkKarma:     accountData.LinkKarma,
		CommentKarma:  accountData.CommentKarma,
	}); err != nil {
		h.logger.Error("failed to encode status response", "error", err, slog.String("request_id", requestID))
	}
}

// LogoutHandler handles the POST /api/auth/logout endpoint.
// It invalidates the user's session.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Limit request body size for POST request
	limitedBody := io.LimitReader(r.Body, maxRequestBodySize)
	_, err := io.ReadAll(limitedBody)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "failed to read request body", requestID)
		return
	}
	defer r.Body.Close()

	// Extract JWT from Authorization header using helper
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Delete session
	h.sessionManager.DeleteSession(sessionID)
	h.logger.Info("user logged out", "session_id", sessionID, slog.String("request_id", requestID))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(SuccessResponse{Success: true}); err != nil {
		h.logger.Error("failed to encode logout response", "error", err, slog.String("request_id", requestID))
	}
}

// PostsHandler handles the GET /api/posts endpoint.
// It retrieves hot, new, or other posts from a subreddit.
// Query parameters: subreddit (required), limit (optional, default 25), after (optional for pagination).
func (h *Handler) PostsHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit for API endpoint (10 requests/sec with burst of 5)
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later", requestID)
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// Parse and validate query parameters
	subreddit := r.URL.Query().Get("subreddit")
	if subreddit == "" {
		h.logger.Warn("missing subreddit parameter", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "subreddit parameter is required", requestID)
		return
	}

	// Validate subreddit name using validation package
	if !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name", requestID)
		return
	}

	limit := parseIntParam(r, "limit", defaultPostLimit, maxPostLimit)
	after := r.URL.Query().Get("after")

	// Validate pagination cursor if provided
	if after != "" && !validatePaginationCursor(after) {
		h.logger.Warn("invalid after pagination parameter", "after", after, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid pagination cursor format", requestID)
		return
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "hot" // Default to hot
	}

	// Validate sort parameter - only allow specific values
	if !isValidSortParam(sortBy) {
		h.logger.Warn("invalid sort parameter", "sort", sortBy, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid sort parameter, must be 'hot' or 'new'", requestID)
		return
	}

	// Create context with timeout, respecting request context
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Call appropriate Reddit API method based on sort parameter
	var resp *types.PostsResponse
	postsReq := &types.PostsRequest{
		Subreddit: subreddit,
		Pagination: types.Pagination{
			Limit: limit,
			After: after,
		},
	}

	switch sortBy {
	case "new":
		resp, err = session.RedditClient.GetNew(ctx, postsReq)
	case "hot":
		fallthrough
	default:
		resp, err = session.RedditClient.GetHot(ctx, postsReq)
	}

	if err != nil {
		h.logger.Error("failed to fetch posts", "subreddit", subreddit, "sort", sortBy, "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to fetch posts from Reddit", requestID)
		return
	}

	// Convert Post objects to PostData for response
	postDataList := make([]*PostData, len(resp.Posts))
	for i, post := range resp.Posts {
		postDataList[i] = &PostData{
			ID:          post.ID,
			Title:       post.Title,
			Author:      post.Author,
			Score:       post.Score,
			NumComments: post.NumComments,
			URL:         post.URL,
			Permalink:   post.Permalink,
			Subreddit:   post.Subreddit,
			SelfText:    post.SelfText,
			Created:     post.CreatedUTC,
			UpvoteRatio: post.UpvoteRatio,
			IsSelf:      post.IsSelf,
			Over18:      post.Over18,
			Stickied:    post.Stickied,
			Locked:      post.Locked,
		}
	}

	h.logger.Info("fetched posts", "subreddit", subreddit, "sort", sortBy, "count", len(postDataList), slog.String("request_id", requestID))

	// Auto-cache posts in background if storage is available
	if h.store != nil && len(resp.Posts) > 0 {
		// Capture requestID before spawning goroutine
		go func(bgRequestID string) {
			// Use Background context since cache operation should continue even if request is cancelled
			bgCtx := reqid.WithRequestID(context.Background(), bgRequestID)
			ctx, cancel := context.WithTimeout(bgCtx, cacheOperationTimeout)
			defer cancel()

			if err := h.store.UpsertPosts(ctx, resp.Posts); err != nil {
				h.logger.Error("failed to cache posts", "subreddit", subreddit, "error", err, slog.String("request_id", bgRequestID))
			} else {
				h.logger.Info("posts cached successfully", "subreddit", subreddit, "count", len(resp.Posts), slog.String("request_id", bgRequestID))
			}
		}(requestID)
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(PostsResponse{
		Posts:          postDataList,
		AfterFullname:  resp.AfterFullname,
		BeforeFullname: resp.BeforeFullname,
		Total:          0, // Not applicable for cursor-based pagination
	}); err != nil {
		h.logger.Error("failed to encode posts response", "error", err, slog.String("request_id", requestID))
	}
}

// CommentsHandler handles the GET /api/comments endpoint.
// It retrieves comments for a specific post.
// Query parameters: subreddit (required), post_id (required), limit (optional, default 25), after (optional for pagination).
func (h *Handler) CommentsHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit for API endpoint (10 requests/sec with burst of 5)
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later", requestID)
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// Parse and validate query parameters
	subreddit := r.URL.Query().Get("subreddit")
	if subreddit == "" {
		h.logger.Warn("missing subreddit parameter", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "subreddit parameter is required", requestID)
		return
	}

	// Validate subreddit name using validation package
	if !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name", requestID)
		return
	}

	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		h.logger.Warn("missing post_id parameter", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "post_id parameter is required", requestID)
		return
	}

	// Validate post ID format
	if !validatePostID(postID) {
		h.logger.Warn("invalid post_id parameter", "post_id", postID, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid post_id format", requestID)
		return
	}

	limit := parseIntParam(r, "limit", defaultCommentLimit, maxCommentLimit)
	after := r.URL.Query().Get("after")

	// Validate pagination cursor if provided
	if after != "" && !validatePaginationCursor(after) {
		h.logger.Warn("invalid after pagination parameter", "after", after, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid pagination cursor format", requestID)
		return
	}

	// Create context with timeout, respecting request context
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Fetch comments from Reddit API
	commentsReq := &types.CommentsRequest{
		Subreddit: subreddit,
		PostID:    postID,
		Pagination: types.Pagination{
			Limit: limit,
			After: after,
		},
	}

	resp, err := session.RedditClient.GetComments(ctx, commentsReq)
	if err != nil {
		h.logger.Error("failed to fetch comments", "subreddit", subreddit, "post_id", postID, "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to fetch comments from Reddit", requestID)
		return
	}

	// Convert Post to PostData
	var postData *PostData
	if resp.Post != nil {
		postData = &PostData{
			ID:          resp.Post.ID,
			Title:       resp.Post.Title,
			Author:      resp.Post.Author,
			Score:       resp.Post.Score,
			NumComments: resp.Post.NumComments,
			URL:         resp.Post.URL,
			Permalink:   resp.Post.Permalink,
			Subreddit:   resp.Post.Subreddit,
			SelfText:    resp.Post.SelfText,
			Created:     resp.Post.CreatedUTC,
			UpvoteRatio: resp.Post.UpvoteRatio,
			IsSelf:      resp.Post.IsSelf,
			Over18:      resp.Post.Over18,
			Stickied:    resp.Post.Stickied,
			Locked:      resp.Post.Locked,
		}
	}

	// Convert Comment objects to CommentData
	commentDataList := make([]*CommentData, len(resp.Comments))
	for i, comment := range resp.Comments {
		commentDataList[i] = &CommentData{
			ID:        comment.ID,
			Author:    comment.Author,
			Body:      comment.Body,
			Score:     comment.Score,
			Created:   comment.CreatedUTC,
			Subreddit: comment.Subreddit,
			ParentID:  comment.ParentID,
			Edited:    comment.Edited.IsEdited,
		}
	}

	h.logger.Info("fetched comments", "subreddit", subreddit, "post_id", postID, "count", len(commentDataList), slog.String("request_id", requestID))

	// Auto-cache post and comments in background if storage is available
	if h.store != nil {
		// Capture requestID before spawning goroutine
		go func(bgRequestID string) {
			// Use Background context since cache operation should continue even if request is cancelled
			bgCtx := reqid.WithRequestID(context.Background(), bgRequestID)
			ctx, cancel := context.WithTimeout(bgCtx, cacheOperationTimeout)
			defer cancel()

			// Save the post if present
			if err := h.store.UpsertPost(ctx, resp.Post); err != nil {
				h.logger.Error("failed to cache post", "post_id", postID, "error", err, slog.String("request_id", bgRequestID))
			}

			// Save the comments
			if len(resp.Comments) > 0 {
				if err := h.store.UpsertComments(ctx, resp.Comments); err != nil {
					h.logger.Error("failed to cache comments", "post_id", postID, "error", err, slog.String("request_id", bgRequestID))
				} else {
					h.logger.Info("comments cached successfully", "post_id", postID, "count", len(resp.Comments), slog.String("request_id", bgRequestID))
				}
			}
		}(requestID)
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(CommentsResponse{
		Post:           postData,
		Comments:       commentDataList,
		MoreIDs:        resp.MoreIDs,
		AfterFullname:  resp.AfterFullname,
		BeforeFullname: resp.BeforeFullname,
	}); err != nil {
		h.logger.Error("failed to encode comments response", "error", err, slog.String("request_id", requestID))
	}
}

// handleGetSavedPosts handles the GET /api/saved/posts endpoint.
// It retrieves cached posts from storage with optional filtering and sorting.
// Query parameters: subreddit (optional), limit (optional, default 25, max 100),
// offset (optional, default 0), sort (optional, default "created_utc").
func (h *Handler) handleGetSavedPosts(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit for API endpoint
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later", requestID)
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session to verify it exists
	_, err = h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// Check if storage is available
	if h.store == nil {
		h.logger.Warn("storage not available", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusServiceUnavailable, "caching service not available", requestID)
		return
	}

	// Parse and validate query parameters
	subreddit := r.URL.Query().Get("subreddit")
	if subreddit != "" && !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name", requestID)
		return
	}

	limit := parseIntParam(r, "limit", defaultPostLimit, maxPostLimit)
	offset := parseIntParam(r, "offset", 0, maxOffsetLimit)
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "created_utc"
	}

	// Validate sort parameter for storage queries
	if !isValidStorageSortParam(sortBy) {
		h.logger.Warn("invalid sort parameter", "sort", sortBy, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid sort parameter, must be 'created_utc', 'score', or 'num_comments'", requestID)
		return
	}

	// Create context with timeout (10s to accommodate both ListPosts and CountPosts database operations)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Query storage
	opts := &storage.ListPostsOptions{
		Subreddit: subreddit,
		SortBy:    sortBy,
		SortDir:   "desc",
		Limit:     limit,
		Offset:    offset,
	}

	posts, err := h.store.ListPosts(ctx, opts)
	if err != nil {
		h.logger.Error("failed to list cached posts", "subreddit", subreddit, "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to retrieve cached posts", requestID)
		return
	}

	// Get total count of posts matching the filter criteria.
	// If counting fails, we log a warning and continue with total=-1 to indicate unknown count.
	// This allows the frontend to display posts even if pagination metadata is unavailable.
	total, err := h.store.CountPosts(ctx, opts)
	if err != nil {
		h.logger.Warn("failed to count cached posts", "subreddit", subreddit, "error", err, slog.String("request_id", requestID))
		total = -1 // Signal to frontend that total count is unknown
	}

	// Convert Post objects to PostData for response
	postDataList := make([]*PostData, len(posts))
	for i, post := range posts {
		postDataList[i] = &PostData{
			ID:          post.ID,
			Title:       post.Title,
			Author:      post.Author,
			Score:       post.Score,
			NumComments: post.NumComments,
			URL:         post.URL,
			Permalink:   post.Permalink,
			Subreddit:   post.Subreddit,
			SelfText:    post.SelfText,
			Created:     post.CreatedUTC,
			UpvoteRatio: post.UpvoteRatio,
			IsSelf:      post.IsSelf,
			Over18:      post.Over18,
			Stickied:    post.Stickied,
			Locked:      post.Locked,
		}
	}

	h.logger.Info("retrieved cached posts", "subreddit", subreddit, "count", len(postDataList), "offset", offset, "total", total, slog.String("request_id", requestID))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(PostsResponse{
		Posts:          postDataList,
		AfterFullname:  "", // Cached endpoints use offset-based pagination, not fullname cursors
		BeforeFullname: "", // Cached endpoints use offset-based pagination, not fullname cursors
		Total:          total,
	}); err != nil {
		h.logger.Error("failed to encode saved posts response", "error", err, slog.String("request_id", requestID))
	}
}

// handleGetSavedComments handles the GET /api/saved/comments endpoint.
// It retrieves cached comments for a specific post from storage.
// Query parameters: post_id (required), subreddit (optional).
func (h *Handler) handleGetSavedComments(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit for API endpoint
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later", requestID)
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session to verify it exists
	_, err = h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// Check if storage is available
	if h.store == nil {
		h.logger.Warn("storage not available", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusServiceUnavailable, "caching service not available", requestID)
		return
	}

	// Parse and validate query parameters
	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		h.logger.Warn("missing post_id parameter", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "post_id parameter is required", requestID)
		return
	}

	// Validate post ID format
	if !validatePostID(postID) {
		h.logger.Warn("invalid post_id parameter", "post_id", postID, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid post_id format", requestID)
		return
	}

	subreddit := r.URL.Query().Get("subreddit")
	if subreddit != "" && !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name", requestID)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Query storage for comments tree
	opts := &storage.CommentTreeOptions{
		SortBy:  "score",
		SortDir: "desc",
	}

	comments, err := h.store.GetCommentTree(ctx, postID, opts)
	if err != nil {
		h.logger.Error("failed to get cached comment tree", "post_id", postID, "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to retrieve cached comments", requestID)
		return
	}

	// Convert Comment objects to CommentData for response
	commentDataList := make([]*CommentData, len(comments))
	for i, comment := range comments {
		commentDataList[i] = &CommentData{
			ID:        comment.ID,
			Author:    comment.Author,
			Body:      comment.Body,
			Score:     comment.Score,
			Created:   comment.CreatedUTC,
			Subreddit: comment.Subreddit,
			ParentID:  comment.ParentID,
			Edited:    comment.Edited.IsEdited,
		}
	}

	h.logger.Info("retrieved cached comments", "post_id", postID, "count", len(commentDataList), slog.String("request_id", requestID))

	// Send response (without the post since we're only retrieving comments)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(CommentsResponse{
		Post:           nil,
		Comments:       commentDataList,
		MoreIDs:        nil,
		AfterFullname:  "", // Cached endpoints do not use pagination cursors
		BeforeFullname: "", // Cached endpoints do not use pagination cursors
	}); err != nil {
		h.logger.Error("failed to encode saved comments response", "error", err, slog.String("request_id", requestID))
	}
}

// parseIntParam parses an integer query parameter with a default value.
// If the parameter is missing or invalid, the default value is returned.
// If the value is greater than max, max is returned.
func parseIntParam(r *http.Request, paramName string, defaultValue, max int) int {
	valueStr := r.URL.Query().Get(paramName)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	if value > max {
		return max
	}

	if value < 0 {
		return defaultValue
	}

	return value
}

// sendErrorResponse sends a JSON error response with the provided request ID.
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	if requestID != "" {
		w.Header().Set("X-Request-ID", requestID)
	}
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		// If encoding fails, we can't send a proper response, just log it
		// since headers are already sent
		slog.Error("failed to encode error response", "error", err)
	}
}

// validatePostID validates a Reddit post ID format (base36, 1-20 chars).
func validatePostID(id string) bool {
	if id == "" || len(id) > 20 {
		return false
	}
	return postIDRegex.MatchString(id)
}

// validatePaginationCursor validates a Reddit pagination cursor (fullname format: t3_xxxxx or t1_xxxxx).
func validatePaginationCursor(cursor string) bool {
	if cursor == "" || len(cursor) > 110 {
		return false
	}
	return paginationFullnameRegex.MatchString(cursor)
}

// isValidSortParam validates the sort parameter value for listing posts.
func isValidSortParam(sort string) bool {
	switch sort {
	case "hot", "new":
		return true
	default:
		return false
	}
}

// isValidStorageSortParam validates the sort parameter value for cached storage queries.
func isValidStorageSortParam(sort string) bool {
	switch sort {
	case "created_utc", "score", "num_comments":
		return true
	default:
		return false
	}
}

// generateJobID generates a unique job ID using crypto/rand.
func generateJobID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// jobIDRegex matches valid job IDs (hex string, exactly 32 chars).
var jobIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// validateJobID validates a job ID format (hex string, exactly 32 chars).
func validateJobID(id string) bool {
	if id == "" || len(id) != 32 {
		return false
	}
	return jobIDRegex.MatchString(id)
}

// cleanupOldJobs removes completed or errored jobs older than the specified duration.
// It should be called periodically to prevent the jobs map from growing unbounded.
func cleanupOldJobs(maxAge time.Duration, logger *slog.Logger) {
	bulkSaveJobsMutex.Lock()
	defer bulkSaveJobsMutex.Unlock()

	now := time.Now()
	deletedCount := 0

	for jobID, job := range bulkSaveJobs {
		job.mu.RLock()
		status := job.status
		completedAt := job.completedAt
		job.mu.RUnlock()

		// Only clean up completed or errored jobs
		if (status == "completed" || status == "error") && !completedAt.IsZero() {
			if now.Sub(completedAt) > maxAge {
				delete(bulkSaveJobs, jobID)
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		logger.Info("cleaned up old bulk save jobs", "count", deletedCount)
	}
}

// handleBulkSavePosts handles the POST /api/bulk-save/posts endpoint.
// It initiates a background job to save posts (and optionally comments) from a subreddit.
func (h *Handler) handleBulkSavePosts(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit for API endpoint
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later", requestID)
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// Check if storage is available
	if h.store == nil {
		h.logger.Warn("storage not available", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusServiceUnavailable, "storage service not available", requestID)
		return
	}

	// Parse request body with size limit
	var req BulkSaveRequest
	limitedBody := io.LimitReader(r.Body, maxRequestBodySize)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "failed to read request body", requestID)
		return
	}
	defer r.Body.Close()

	// Check if body size limit was exceeded
	if len(body) >= maxRequestBodySize {
		h.logger.Warn("request body size limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusRequestEntityTooLarge, "request body too large", requestID)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.Error("failed to unmarshal request", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid request format", requestID)
		return
	}

	// Validate subreddit name
	if req.Subreddit == "" {
		h.logger.Warn("missing subreddit parameter", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "subreddit is required", requestID)
		return
	}

	if !validation.IsValidSubreddit(req.Subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", req.Subreddit, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name", requestID)
		return
	}

	// Validate sort parameter
	if req.Sort == "" {
		req.Sort = "hot" // Default to hot
	}

	if !isValidSortParam(req.Sort) {
		h.logger.Warn("invalid sort parameter", "sort", req.Sort, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "sort must be 'hot' or 'new'", requestID)
		return
	}

	// Validate count parameter
	if req.Count < minBulkSaveCount || req.Count > maxBulkSaveCount {
		h.logger.Warn("invalid count parameter", "count", req.Count, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "count must be between 1 and 2000", requestID)
		return
	}

	// Check if we've reached max concurrent jobs
	bulkSaveJobsMutex.RLock()
	activeJobsCount := 0
	for _, job := range bulkSaveJobs {
		job.mu.RLock()
		if job.status == "in_progress" {
			activeJobsCount++
		}
		job.mu.RUnlock()
	}
	bulkSaveJobsMutex.RUnlock()

	if activeJobsCount >= maxConcurrentJobs {
		h.logger.Warn("max concurrent jobs limit reached", "active_jobs", activeJobsCount, "max", maxConcurrentJobs, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "maximum number of concurrent bulk save jobs reached, please try again later", requestID)
		return
	}

	// Generate unique job ID
	jobID, err := generateJobID()
	if err != nil {
		h.logger.Error("failed to generate job ID", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusInternalServerError, "failed to create job", requestID)
		return
	}

	// Create job record
	job := &bulkSaveJob{
		status:     "in_progress",
		postsTotal: req.Count,
	}

	// Register job in global map
	bulkSaveJobsMutex.Lock()
	bulkSaveJobs[jobID] = job
	bulkSaveJobsMutex.Unlock()

	h.logger.Info("starting bulk save job",
		"job_id", jobID,
		"subreddit", req.Subreddit,
		"sort", req.Sort,
		"count", req.Count,
		"include_comments", req.IncludeComments,
		slog.String("request_id", requestID),
	)

	// Start background goroutine to perform bulk save
	go h.performBulkSave(jobID, job, session.RedditClient, req)

	// Send immediate response with job ID
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(BulkSaveResponse{
		JobID:   jobID,
		Message: "Bulk save operation started successfully",
	}); err != nil {
		h.logger.Error("failed to encode bulk save response", "error", err, slog.String("request_id", requestID))
	}
}

// handleBulkSaveProgress handles the GET /api/bulk-save/progress/{jobId} endpoint.
// It retrieves the current progress of a bulk save operation.
func (h *Handler) handleBulkSaveProgress(w http.ResponseWriter, r *http.Request) {
	// Ensure request ID exists in context
	ctx := reqid.Ensure(r.Context())
	requestID := reqid.FromContext(ctx)
	r = r.WithContext(ctx)

	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed", requestID)
		return
	}

	// Check rate limit for API endpoint
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded", slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later", requestID)
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err, slog.String("request_id", requestID))
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header", requestID)
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format", requestID)
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error", requestID)
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token", requestID)
		return
	}

	// Get session to verify it exists
	_, err = h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusUnauthorized, "session not found", requestID)
		return
	}

	// Extract job ID from URL path
	// Expected format: /api/bulk-save/progress/{jobId}
	path := r.URL.Path
	jobID := strings.TrimPrefix(path, "/api/bulk-save/progress/")
	if jobID == "" || jobID == path {
		h.logger.Warn("missing or invalid job ID in URL path", "path", path, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "job ID is required in URL path", requestID)
		return
	}

	// Validate job ID format
	if !validateJobID(jobID) {
		h.logger.Warn("invalid job ID format", "job_id", jobID, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusBadRequest, "invalid job ID format", requestID)
		return
	}

	// Look up job in the global jobs map
	bulkSaveJobsMutex.RLock()
	job, exists := bulkSaveJobs[jobID]
	bulkSaveJobsMutex.RUnlock()

	if !exists {
		h.logger.Warn("job not found", "job_id", jobID, slog.String("request_id", requestID))
		sendErrorResponse(w, http.StatusNotFound, "job not found", requestID)
		return
	}

	// Get current progress from the job
	progress := job.getProgress()

	h.logger.Info("bulk save progress retrieved", "job_id", jobID, "status", progress.Status, slog.String("request_id", requestID))

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(progress); err != nil {
		h.logger.Error("failed to encode bulk save progress response", "error", err, slog.String("request_id", requestID))
	}
}

// performBulkSave performs the actual bulk save operation in the background.
func (h *Handler) performBulkSave(jobID string, job *bulkSaveJob, redditClient *graw.Reddit, req BulkSaveRequest) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), bulkSaveOperationTimeout)
	defer cancel()

	// Defer cleanup to ensure job status is updated even on panic
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("panic in bulk save operation", "job_id", jobID, "panic", r)
			job.mu.Lock()
			job.status = "error"
			job.errorMsg = "internal error: operation panicked"
			job.completedAt = time.Now()
			job.mu.Unlock()
		}
	}()

	// Calculate number of pages needed (Reddit max is 100 posts per page)
	pagesNeeded := int(math.Ceil(float64(req.Count) / float64(bulkSavePageSize)))
	var allPosts []*types.Post
	afterCursor := ""

	// Update status to indicate fetching posts
	job.mu.Lock()
	job.status = "fetching_posts"
	job.mu.Unlock()

	// Fetch posts page by page
	for page := 0; page < pagesNeeded; page++ {
		// Check if context is cancelled
		if ctx.Err() != nil {
			job.mu.Lock()
			job.status = "error"
			job.errorMsg = "operation timed out"
			job.completedAt = time.Now()
			job.mu.Unlock()
			h.logger.Error("bulk save timed out", "job_id", jobID)
			return
		}

		// Determine limit for this page
		remainingPosts := req.Count - len(allPosts)
		limit := bulkSavePageSize
		if remainingPosts < bulkSavePageSize {
			limit = remainingPosts
		}

		// Prepare request
		postsReq := &types.PostsRequest{
			Subreddit: req.Subreddit,
			Pagination: types.Pagination{
				Limit: limit,
				After: afterCursor,
			},
		}

		// Fetch posts based on sort parameter
		var resp *types.PostsResponse
		var err error

		switch req.Sort {
		case "new":
			resp, err = redditClient.GetNew(ctx, postsReq)
		case "hot":
			resp, err = redditClient.GetHot(ctx, postsReq)
		default:
			resp, err = redditClient.GetHot(ctx, postsReq)
		}

		if err != nil {
			job.mu.Lock()
			job.status = "error"
			job.errorMsg = "failed to fetch posts: " + err.Error()
			job.completedAt = time.Now()
			job.mu.Unlock()
			h.logger.Error("failed to fetch posts in bulk save", "job_id", jobID, "error", err)
			return
		}

		// Add posts to collection
		allPosts = append(allPosts, resp.Posts...)

		// Update job progress
		job.mu.Lock()
		job.postsSaved = len(allPosts)
		job.mu.Unlock()

		h.logger.Info("fetched posts page",
			"job_id", jobID,
			"page", page+1,
			"posts_fetched", len(resp.Posts),
			"total_posts", len(allPosts),
		)

		// Update cursor for next page
		afterCursor = resp.AfterFullname

		// Stop if we've reached the end or have enough posts
		if afterCursor == "" || len(allPosts) >= req.Count {
			break
		}
	}

	// Truncate to exact count if we fetched more
	if len(allPosts) > req.Count {
		allPosts = allPosts[:req.Count]
	}

	// Update postsTotal if we fetched fewer posts than requested
	if len(allPosts) < req.Count {
		job.mu.Lock()
		job.postsTotal = len(allPosts)
		job.mu.Unlock()
	}

	// Update status to indicate saving posts
	job.mu.Lock()
	job.status = "saving"
	job.mu.Unlock()

	// Save posts in batches
	for i := 0; i < len(allPosts); i += bulkSaveBatchSize {
		// Check if context is cancelled
		if ctx.Err() != nil {
			job.mu.Lock()
			job.status = "error"
			job.errorMsg = "operation timed out during save"
			job.completedAt = time.Now()
			job.mu.Unlock()
			h.logger.Error("bulk save timed out during save", "job_id", jobID)
			return
		}

		end := i + bulkSaveBatchSize
		if end > len(allPosts) {
			end = len(allPosts)
		}

		batch := allPosts[i:end]

		if err := h.store.UpsertPosts(ctx, batch); err != nil {
			job.mu.Lock()
			job.status = "error"
			job.errorMsg = "failed to save posts: " + err.Error()
			job.completedAt = time.Now()
			job.mu.Unlock()
			h.logger.Error("failed to save posts in bulk save", "job_id", jobID, "error", err)
			return
		}

		h.logger.Info("saved posts batch",
			"job_id", jobID,
			"batch_size", len(batch),
			"total_saved", end,
		)
	}

	// Fetch and save comments if requested
	if req.IncludeComments && len(allPosts) > 0 {
		// Check if context is cancelled before starting comment fetching
		if ctx.Err() != nil {
			job.mu.Lock()
			job.status = "error"
			job.errorMsg = "operation timed out before fetching comments"
			job.completedAt = time.Now()
			job.mu.Unlock()
			h.logger.Error("bulk save timed out before fetching comments", "job_id", jobID)
			return
		}

		// Update status to indicate fetching comments
		job.mu.Lock()
		job.status = "fetching_comments"
		job.mu.Unlock()

		// Collect post IDs
		postIDs := make([]string, len(allPosts))
		for i, post := range allPosts {
			postIDs[i] = post.ID
		}

		// Prepare comments requests
		commentsReqs := make([]*types.CommentsRequest, len(postIDs))
		for i, postID := range postIDs {
			commentsReqs[i] = &types.CommentsRequest{
				Subreddit: req.Subreddit,
				PostID:    postID,
				Pagination: types.Pagination{
					Limit: 100, // Fetch up to 100 comments per post
				},
			}
		}

		// Fetch comments for all posts using GetCommentsMultiple
		commentsResponses, err := redditClient.GetCommentsMultiple(ctx, commentsReqs)
		if err != nil {
			// Log error but don't fail the entire job since posts were saved
			h.logger.Error("failed to fetch comments in bulk save", "job_id", jobID, "error", err)
			job.mu.Lock()
			job.errorMsg = "posts saved, but failed to fetch comments: " + err.Error()
			job.mu.Unlock()
		} else {
			// Collect all comments
			var allComments []*types.Comment
			for _, resp := range commentsResponses {
				allComments = append(allComments, resp.Comments...)
			}

			// Save comments
			if len(allComments) > 0 {
				if err := h.store.UpsertComments(ctx, allComments); err != nil {
					h.logger.Error("failed to save comments in bulk save", "job_id", jobID, "error", err)
					job.mu.Lock()
					job.errorMsg = "posts saved, but failed to save comments: " + err.Error()
					job.mu.Unlock()
				} else {
					job.mu.Lock()
					job.commentsSaved = len(allComments)
					job.mu.Unlock()
					h.logger.Info("saved comments",
						"job_id", jobID,
						"comments_saved", len(allComments),
					)
				}
			}
		}
	}

	// Mark job as completed
	job.mu.Lock()
	// Mark as completed if no error occurred (regardless of intermediate status)
	if job.status != "error" {
		job.status = "completed"
	}
	job.completedAt = time.Now()
	job.mu.Unlock()

	h.logger.Info("bulk save job completed",
		"job_id", jobID,
		"posts_saved", len(allPosts),
		"comments_saved", job.commentsSaved,
	)
}

// Error types for authentication header parsing
var (
	ErrMissingAuthHeader       = &AuthError{message: "missing authorization header"}
	ErrInvalidAuthHeaderFormat = &AuthError{message: "invalid authorization header format"}
)

// AuthError represents an authentication-related error.
type AuthError struct {
	message string
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	return e.message
}

// Handler contains the dependencies for HTTP handlers.
type Handler struct {
	sessionManager *SessionManager
	logger         *slog.Logger
	loginLimiter   *rate.Limiter
	apiLimiter     *rate.Limiter
	store          storage.Store
}

// NewHandler creates a new Handler instance.
// It initializes rate limiting for the login endpoint (5 requests per second)
// and for the API endpoints (10 requests per second with burst of 5).
// The store parameter is optional and may be nil if caching is disabled.
func NewHandler(sessionManager *SessionManager, logger *slog.Logger, store storage.Store) *Handler {
	return &Handler{
		sessionManager: sessionManager,
		logger:         logger,
		store:          store,
		// Initialize rate limiter for login: 5 requests per second
		// TODO: Consider implementing per-IP rate limiting for production deployments
		loginLimiter: rate.NewLimiter(rate.Limit(5), 1),
		// Initialize rate limiter for API: 10 requests per second with burst of 5
		apiLimiter: rate.NewLimiter(rate.Limit(10), 5),
	}
}
