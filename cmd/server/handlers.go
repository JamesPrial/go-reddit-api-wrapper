package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/reqid"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// Server represents the HTTP API server.
type Server struct {
	reddit *graw.Reddit
	logger *slog.Logger
}

// NewServer creates a new API server instance.
func NewServer(reddit *graw.Reddit, logger *slog.Logger) *Server {
	return &Server{
		reddit: reddit,
		logger: logger,
	}
}

// writeError writes a standardized error response.
func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{
		Error:     message,
		RequestID: requestID,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("failed to encode error response",
			"error", err.Error(),
			"request_id", requestID,
			"status_code", statusCode,
		)
	}
}

// writeJSON writes a JSON response with proper headers.
func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response",
			"error", err.Error(),
			"status_code", statusCode,
		)
	}
}

// HealthHandler handles GET /health requests.
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	resp := HealthResponse{
		Status: "ok",
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// GetHotHandler handles GET /api/v1/r/{subreddit}/hot requests.
func (s *Server) GetHotHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", reqid.FromContext(r.Context()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	subreddit := r.PathValue("subreddit")
	if subreddit == "" {
		s.writeError(w, http.StatusBadRequest, "subreddit is required", reqid.FromContext(ctx))
		return
	}

	// Parse query parameters
	limit := s.parseLimit(r, 25, 100)
	after := r.URL.Query().Get("after")
	before := r.URL.Query().Get("before")

	request := &types.PostsRequest{
		Subreddit: subreddit,
		Pagination: types.Pagination{
			Limit:  limit,
			After:  after,
			Before: before,
		},
	}

	response, err := s.reddit.GetHot(ctx, request)
	if err != nil {
		s.handleRedditError(w, err, reqid.FromContext(ctx))
		return
	}

	posts := make([]*PostData, len(response.Posts))
	for i, post := range response.Posts {
		posts[i] = convertPost(post)
	}

	result := PostsResponse{
		Posts:  posts,
		After:  response.AfterFullname,
		Before: response.BeforeFullname,
		Count:  len(posts),
	}

	s.writeJSON(w, http.StatusOK, result)
}

// GetNewHandler handles GET /api/v1/r/{subreddit}/new requests.
func (s *Server) GetNewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", reqid.FromContext(r.Context()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	subreddit := r.PathValue("subreddit")
	if subreddit == "" {
		s.writeError(w, http.StatusBadRequest, "subreddit is required", reqid.FromContext(ctx))
		return
	}

	// Parse query parameters
	limit := s.parseLimit(r, 25, 100)
	after := r.URL.Query().Get("after")
	before := r.URL.Query().Get("before")

	request := &types.PostsRequest{
		Subreddit: subreddit,
		Pagination: types.Pagination{
			Limit:  limit,
			After:  after,
			Before: before,
		},
	}

	response, err := s.reddit.GetNew(ctx, request)
	if err != nil {
		s.handleRedditError(w, err, reqid.FromContext(ctx))
		return
	}

	posts := make([]*PostData, len(response.Posts))
	for i, post := range response.Posts {
		posts[i] = convertPost(post)
	}

	result := PostsResponse{
		Posts:  posts,
		After:  response.AfterFullname,
		Before: response.BeforeFullname,
		Count:  len(posts),
	}

	s.writeJSON(w, http.StatusOK, result)
}

// GetCommentsHandler handles GET /api/v1/r/{subreddit}/posts/{postId}/comments requests.
func (s *Server) GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", reqid.FromContext(r.Context()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	subreddit := r.PathValue("subreddit")
	if subreddit == "" {
		s.writeError(w, http.StatusBadRequest, "subreddit is required", reqid.FromContext(ctx))
		return
	}

	postID := r.PathValue("postId")
	if postID == "" {
		s.writeError(w, http.StatusBadRequest, "postId is required", reqid.FromContext(ctx))
		return
	}

	// Parse query parameters
	limit := s.parseLimit(r, 25, 100)
	after := r.URL.Query().Get("after")
	before := r.URL.Query().Get("before")

	request := &types.CommentsRequest{
		Subreddit: subreddit,
		PostID:    postID,
		Pagination: types.Pagination{
			Limit:  limit,
			After:  after,
			Before: before,
		},
	}

	response, err := s.reddit.GetComments(ctx, request)
	if err != nil {
		s.handleRedditError(w, err, reqid.FromContext(ctx))
		return
	}

	// Convert post and comments
	post := convertPost(response.Post)
	comments := make([]*CommentData, len(response.Comments))
	for i, comment := range response.Comments {
		comments[i] = convertComment(comment)
	}

	result := CommentsResponse{
		Post:     post,
		Comments: comments,
		After:    response.AfterFullname,
		Before:   response.BeforeFullname,
		Count:    len(comments),
	}

	s.writeJSON(w, http.StatusOK, result)
}

// GetMeHandler handles GET /api/v1/me requests.
func (s *Server) GetMeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", reqid.FromContext(r.Context()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	account, err := s.reddit.Me(ctx)
	if err != nil {
		s.handleRedditError(w, err, reqid.FromContext(ctx))
		return
	}

	result := convertAccountData(account)
	s.writeJSON(w, http.StatusOK, result)
}

// GetSubredditHandler handles GET /api/v1/r/{subreddit}/about requests.
func (s *Server) GetSubredditHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", reqid.FromContext(r.Context()))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	subreddit := r.PathValue("subreddit")
	if subreddit == "" {
		s.writeError(w, http.StatusBadRequest, "subreddit is required", reqid.FromContext(ctx))
		return
	}

	subredditData, err := s.reddit.GetSubreddit(ctx, subreddit)
	if err != nil {
		s.handleRedditError(w, err, reqid.FromContext(ctx))
		return
	}

	result := convertSubredditData(subredditData)
	s.writeJSON(w, http.StatusOK, result)
}

// parseLimit parses and validates the limit query parameter.
// Returns the limit or defaultLimit if not provided, capped at maxLimit.
func (s *Server) parseLimit(r *http.Request, defaultLimit, maxLimit int) int {
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		return defaultLimit
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}

	if limit > maxLimit {
		return maxLimit
	}

	return limit
}

// handleRedditError converts Reddit API errors to HTTP responses.
func (s *Server) handleRedditError(w http.ResponseWriter, err error, requestID string) {
	s.logger.Error("reddit API error",
		"error", err.Error(),
		"request_id", requestID,
	)

	var statusCode int
	var message string

	// Use proper error type assertions instead of string matching
	var authErr *graw.AuthError
	var validationErr *graw.ValidationError
	var apiErr *graw.APIError
	var rateLimitErr *graw.RateLimitError
	var networkErr *graw.NetworkError

	switch {
	case err == nil:
		statusCode = http.StatusOK
		message = "success"
	case errors.As(err, &authErr):
		statusCode = http.StatusUnauthorized
		message = "authentication failed"
	case errors.As(err, &validationErr):
		statusCode = http.StatusBadRequest
		message = "invalid request"
	case errors.As(err, &rateLimitErr):
		statusCode = http.StatusTooManyRequests
		message = "rate limited by Reddit API"
	case errors.As(err, &apiErr):
		// Map Reddit API status codes to appropriate HTTP responses
		switch apiErr.StatusCode {
		case http.StatusNotFound:
			statusCode = http.StatusNotFound
			message = "not found"
		case http.StatusForbidden:
			statusCode = http.StatusForbidden
			message = "forbidden"
		case http.StatusUnauthorized:
			statusCode = http.StatusUnauthorized
			message = "authentication failed"
		case http.StatusBadRequest:
			statusCode = http.StatusBadRequest
			message = "invalid request"
		case http.StatusTooManyRequests:
			statusCode = http.StatusTooManyRequests
			message = "rate limited by Reddit API"
		default:
			statusCode = http.StatusInternalServerError
			message = "Reddit API error"
		}
	case errors.As(err, &networkErr):
		statusCode = http.StatusServiceUnavailable
		message = "service temporarily unavailable"
	default:
		statusCode = http.StatusInternalServerError
		message = "internal server error"
	}

	s.writeError(w, statusCode, message, requestID)
}
