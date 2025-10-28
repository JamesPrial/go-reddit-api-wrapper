package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check rate limit (5 requests per second global limit)
	// TODO: Consider implementing per-IP rate limiting for production
	if !h.loginLimiter.Allow() {
		h.logger.Warn("login rate limit exceeded")
		sendErrorResponse(w, http.StatusTooManyRequests, "too many login attempts, please try again later")
		return
	}

	// Parse request body with size limit
	var req LoginRequest
	limitedBody := io.LimitReader(r.Body, maxRequestBodySize)
	body, err := io.ReadAll(limitedBody)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		sendErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Check if body size limit was exceeded
	if len(body) >= maxRequestBodySize {
		h.logger.Warn("request body size limit exceeded")
		sendErrorResponse(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		h.logger.Error("failed to unmarshal request", "error", err)
		sendErrorResponse(w, http.StatusBadRequest, "invalid request format")
		return
	}

	// Get Reddit credentials from environment
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		h.logger.Error("missing Reddit credentials in environment")
		sendErrorResponse(w, http.StatusInternalServerError, "server configuration error")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	h.logger.Info("authenticating with Reddit", "auth_mode", authMode)

	client, err := graw.NewClientWithContext(ctx, config)
	if err != nil {
		h.logger.Error("failed to authenticate with Reddit", "auth_mode", authMode, "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Create session
	sessionID, token, err := h.sessionManager.CreateSession(sessionUsername, client)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	h.logger.Info("user logged in", "auth_mode", authMode, "username", sessionUsername, "session_id", sessionID)

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
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract JWT from Authorization header using helper
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err)
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error")
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "session not found")
		return
	}

	// For app-only sessions, skip user info fetch
	if session.Username == "app-only" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(StatusResponse{
			Authenticated: true,
			Username:      "app-only",
			LinkKarma:     0,
			CommentKarma:  0,
		}); err != nil {
			h.logger.Error("failed to encode status response", "error", err)
		}
		return
	}

	// Get user info from Reddit for user-authenticated sessions
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	accountData, err := session.RedditClient.Me(ctx)
	if err != nil {
		h.logger.Error("failed to fetch user info", "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to fetch user info")
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(StatusResponse{
		Authenticated: true,
		Username:      session.Username,
		LinkKarma:     accountData.LinkKarma,
		CommentKarma:  accountData.CommentKarma,
	}); err != nil {
		h.logger.Error("failed to encode status response", "error", err)
	}
}

// LogoutHandler handles the POST /api/auth/logout endpoint.
// It invalidates the user's session.
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size for POST request
	limitedBody := io.LimitReader(r.Body, maxRequestBodySize)
	_, err := io.ReadAll(limitedBody)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		sendErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Extract JWT from Authorization header using helper
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err)
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error")
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Delete session
	h.sessionManager.DeleteSession(sessionID)
	h.logger.Info("user logged out", "session_id", sessionID)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(SuccessResponse{Success: true}); err != nil {
		h.logger.Error("failed to encode logout response", "error", err)
	}
}

// PostsHandler handles the GET /api/posts endpoint.
// It retrieves hot, new, or other posts from a subreddit.
// Query parameters: subreddit (required), limit (optional, default 25), after (optional for pagination).
func (h *Handler) PostsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check rate limit for API endpoint (10 requests/sec with burst of 5)
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded")
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later")
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err)
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error")
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "session not found")
		return
	}

	// Parse and validate query parameters
	subreddit := r.URL.Query().Get("subreddit")
	if subreddit == "" {
		h.logger.Warn("missing subreddit parameter")
		sendErrorResponse(w, http.StatusBadRequest, "subreddit parameter is required")
		return
	}

	// Validate subreddit name using validation package
	if !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit)
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name")
		return
	}

	limit := parseIntParam(r, "limit", defaultPostLimit, maxPostLimit)
	after := r.URL.Query().Get("after")

	// Validate pagination cursor if provided
	if after != "" && !validatePaginationCursor(after) {
		h.logger.Warn("invalid after pagination parameter", "after", after)
		sendErrorResponse(w, http.StatusBadRequest, "invalid pagination cursor format")
		return
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "hot" // Default to hot
	}

	// Validate sort parameter - only allow specific values
	if !isValidSortParam(sortBy) {
		h.logger.Warn("invalid sort parameter", "sort", sortBy)
		sendErrorResponse(w, http.StatusBadRequest, "invalid sort parameter, must be 'hot' or 'new'")
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
		h.logger.Error("failed to fetch posts", "subreddit", subreddit, "sort", sortBy, "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to fetch posts from Reddit")
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

	h.logger.Info("fetched posts", "subreddit", subreddit, "sort", sortBy, "count", len(postDataList))

	// Auto-cache posts in background if storage is available
	if h.store != nil && len(resp.Posts) > 0 {
		go func() {
			// Use Background context since cache operation should continue even if request is cancelled
			ctx, cancel := context.WithTimeout(context.Background(), cacheOperationTimeout)
			defer cancel()

			if err := h.store.UpsertPosts(ctx, resp.Posts); err != nil {
				h.logger.Error("failed to cache posts", "subreddit", subreddit, "error", err)
			} else {
				h.logger.Info("posts cached successfully", "subreddit", subreddit, "count", len(resp.Posts))
			}
		}()
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(PostsResponse{
		Posts:          postDataList,
		AfterFullname:  resp.AfterFullname,
		BeforeFullname: resp.BeforeFullname,
		Total:          0, // Not applicable for cursor-based pagination
	}); err != nil {
		h.logger.Error("failed to encode posts response", "error", err)
	}
}

// CommentsHandler handles the GET /api/comments endpoint.
// It retrieves comments for a specific post.
// Query parameters: subreddit (required), post_id (required), limit (optional, default 25), after (optional for pagination).
func (h *Handler) CommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check rate limit for API endpoint (10 requests/sec with burst of 5)
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded")
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later")
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err)
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error")
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Get session
	session, err := h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "session not found")
		return
	}

	// Parse and validate query parameters
	subreddit := r.URL.Query().Get("subreddit")
	if subreddit == "" {
		h.logger.Warn("missing subreddit parameter")
		sendErrorResponse(w, http.StatusBadRequest, "subreddit parameter is required")
		return
	}

	// Validate subreddit name using validation package
	if !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit)
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name")
		return
	}

	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		h.logger.Warn("missing post_id parameter")
		sendErrorResponse(w, http.StatusBadRequest, "post_id parameter is required")
		return
	}

	// Validate post ID format
	if !validatePostID(postID) {
		h.logger.Warn("invalid post_id parameter", "post_id", postID)
		sendErrorResponse(w, http.StatusBadRequest, "invalid post_id format")
		return
	}

	limit := parseIntParam(r, "limit", defaultCommentLimit, maxCommentLimit)
	after := r.URL.Query().Get("after")

	// Validate pagination cursor if provided
	if after != "" && !validatePaginationCursor(after) {
		h.logger.Warn("invalid after pagination parameter", "after", after)
		sendErrorResponse(w, http.StatusBadRequest, "invalid pagination cursor format")
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
		h.logger.Error("failed to fetch comments", "subreddit", subreddit, "post_id", postID, "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to fetch comments from Reddit")
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

	h.logger.Info("fetched comments", "subreddit", subreddit, "post_id", postID, "count", len(commentDataList))

	// Auto-cache post and comments in background if storage is available
	if h.store != nil {
		go func() {
			// Use Background context since cache operation should continue even if request is cancelled
			ctx, cancel := context.WithTimeout(context.Background(), cacheOperationTimeout)
			defer cancel()

			// Save the post if present
			if err := h.store.UpsertPost(ctx, resp.Post); err != nil {
				h.logger.Error("failed to cache post", "post_id", postID, "error", err)
			}

			// Save the comments
			if len(resp.Comments) > 0 {
				if err := h.store.UpsertComments(ctx, resp.Comments); err != nil {
					h.logger.Error("failed to cache comments", "post_id", postID, "error", err)
				} else {
					h.logger.Info("comments cached successfully", "post_id", postID, "count", len(resp.Comments))
				}
			}
		}()
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(CommentsResponse{
		Post:           postData,
		Comments:       commentDataList,
		MoreIDs:        resp.MoreIDs,
		AfterFullname:  resp.AfterFullname,
		BeforeFullname: resp.BeforeFullname,
	}); err != nil {
		h.logger.Error("failed to encode comments response", "error", err)
	}
}

// handleGetSavedPosts handles the GET /api/saved/posts endpoint.
// It retrieves cached posts from storage with optional filtering and sorting.
// Query parameters: subreddit (optional), limit (optional, default 25, max 100),
// offset (optional, default 0), sort (optional, default "created_utc").
func (h *Handler) handleGetSavedPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check rate limit for API endpoint
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded")
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later")
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err)
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error")
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Get session to verify it exists
	_, err = h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "session not found")
		return
	}

	// Check if storage is available
	if h.store == nil {
		h.logger.Warn("storage not available")
		sendErrorResponse(w, http.StatusServiceUnavailable, "caching service not available")
		return
	}

	// Parse and validate query parameters
	subreddit := r.URL.Query().Get("subreddit")
	if subreddit != "" && !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit)
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name")
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
		h.logger.Warn("invalid sort parameter", "sort", sortBy)
		sendErrorResponse(w, http.StatusBadRequest, "invalid sort parameter, must be 'created_utc', 'score', or 'num_comments'")
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
		h.logger.Error("failed to list cached posts", "subreddit", subreddit, "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to retrieve cached posts")
		return
	}

	// Get total count of posts matching the filter criteria.
	// If counting fails, we log a warning and continue with total=-1 to indicate unknown count.
	// This allows the frontend to display posts even if pagination metadata is unavailable.
	total, err := h.store.CountPosts(ctx, opts)
	if err != nil {
		h.logger.Warn("failed to count cached posts", "subreddit", subreddit, "error", err)
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

	h.logger.Info("retrieved cached posts", "subreddit", subreddit, "count", len(postDataList), "offset", offset, "total", total)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(PostsResponse{
		Posts:          postDataList,
		AfterFullname:  "", // Cached endpoints use offset-based pagination, not fullname cursors
		BeforeFullname: "", // Cached endpoints use offset-based pagination, not fullname cursors
		Total:          total,
	}); err != nil {
		h.logger.Error("failed to encode saved posts response", "error", err)
	}
}

// handleGetSavedComments handles the GET /api/saved/comments endpoint.
// It retrieves cached comments for a specific post from storage.
// Query parameters: post_id (required), subreddit (optional).
func (h *Handler) handleGetSavedComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check rate limit for API endpoint
	if !h.apiLimiter.Allow() {
		h.logger.Warn("API rate limit exceeded")
		sendErrorResponse(w, http.StatusTooManyRequests, "rate limit exceeded, please try again later")
		return
	}

	// Extract JWT from Authorization header
	tokenString, err := extractBearerToken(r)
	if err != nil {
		h.logger.Warn("authorization header error", "error", err)
		switch err {
		case ErrMissingAuthHeader:
			sendErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
		case ErrInvalidAuthHeaderFormat:
			sendErrorResponse(w, http.StatusUnauthorized, "invalid authorization header format")
		default:
			sendErrorResponse(w, http.StatusUnauthorized, "authorization error")
		}
		return
	}

	// Validate JWT
	sessionID, err := h.sessionManager.ValidateJWT(tokenString)
	if err != nil {
		h.logger.Error("invalid JWT token", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Get session to verify it exists
	_, err = h.sessionManager.GetSession(sessionID)
	if err != nil {
		h.logger.Error("session not found", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "session not found")
		return
	}

	// Check if storage is available
	if h.store == nil {
		h.logger.Warn("storage not available")
		sendErrorResponse(w, http.StatusServiceUnavailable, "caching service not available")
		return
	}

	// Parse and validate query parameters
	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		h.logger.Warn("missing post_id parameter")
		sendErrorResponse(w, http.StatusBadRequest, "post_id parameter is required")
		return
	}

	// Validate post ID format
	if !validatePostID(postID) {
		h.logger.Warn("invalid post_id parameter", "post_id", postID)
		sendErrorResponse(w, http.StatusBadRequest, "invalid post_id format")
		return
	}

	subreddit := r.URL.Query().Get("subreddit")
	if subreddit != "" && !validation.IsValidSubreddit(subreddit) {
		h.logger.Warn("invalid subreddit parameter", "subreddit", subreddit)
		sendErrorResponse(w, http.StatusBadRequest, "invalid subreddit name")
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
		h.logger.Error("failed to get cached comment tree", "post_id", postID, "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to retrieve cached comments")
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

	h.logger.Info("retrieved cached comments", "post_id", postID, "count", len(commentDataList))

	// Send response (without the post since we're only retrieving comments)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(CommentsResponse{
		Post:           nil,
		Comments:       commentDataList,
		MoreIDs:        nil,
		AfterFullname:  "", // Cached endpoints do not use pagination cursors
		BeforeFullname: "", // Cached endpoints do not use pagination cursors
	}); err != nil {
		h.logger.Error("failed to encode saved comments response", "error", err)
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

// sendErrorResponse sends a JSON error response.
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
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
