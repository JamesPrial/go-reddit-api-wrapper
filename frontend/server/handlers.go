package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"golang.org/x/time/rate"
)

const (
	// maxRequestBodySize is the maximum allowed size for request bodies (1 MB).
	maxRequestBodySize = 1 * 1024 * 1024
)

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

	// Validate input
	if req.Username == "" || req.Password == "" {
		sendErrorResponse(w, http.StatusBadRequest, "username and password are required")
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

	// Create Reddit client configuration
	config := &graw.Config{
		Username:     req.Username,
		Password:     req.Password,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "reddit-frontend-server/1.0 by /u/yourredditname",
		Logger:       h.logger,
	}

	// Authenticate with Reddit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := graw.NewClientWithContext(ctx, config)
	if err != nil {
		h.logger.Error("failed to authenticate with Reddit", "error", err)
		sendErrorResponse(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Create session
	sessionID, token, err := h.sessionManager.CreateSession(req.Username, client)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		sendErrorResponse(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	h.logger.Info("user logged in", "username", req.Username, "session_id", sessionID)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(LoginResponse{
		Success:  true,
		Token:    token,
		Username: req.Username,
	})
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

	// Get user info from Reddit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	json.NewEncoder(w).Encode(StatusResponse{
		Authenticated: true,
		Username:      session.Username,
		LinkKarma:     accountData.LinkKarma,
		CommentKarma:  accountData.CommentKarma,
	})
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
	json.NewEncoder(w).Encode(SuccessResponse{Success: true})
}

// sendErrorResponse sends a JSON error response.
func sendErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
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
}

// NewHandler creates a new Handler instance.
// It initializes rate limiting for the login endpoint (5 requests per second).
func NewHandler(sessionManager *SessionManager, logger *slog.Logger) *Handler {
	return &Handler{
		sessionManager: sessionManager,
		logger:         logger,
		// Initialize rate limiter: 5 requests per second, unlimited burst
		// TODO: Consider implementing per-IP rate limiting for production deployments
		loginLimiter: rate.NewLimiter(rate.Limit(5), 1),
	}
}
