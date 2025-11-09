package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// UserStore defines the interface for user credential validation.
// Implementations of this interface are responsible for validating
// credentials and returning user information.
type UserStore interface {
	// ValidateCredentials checks if the provided username and password are valid.
	// It returns user information if valid, or an error if validation fails.
	// Implementations should use constant-time comparison to prevent timing attacks.
	ValidateCredentials(username, password string) (*UserData, error)
}

// JWTService defines the interface for JWT token generation and validation.
// Implementations of this interface handle all JWT operations including
// token creation, validation, and claim extraction.
type JWTService interface {
	// GenerateToken creates a new JWT token for the given user with the specified expiry time.
	// It returns the token string or an error if generation fails.
	GenerateToken(user *UserData, expiresAt time.Time) (string, error)

	// ValidateToken verifies the authenticity and validity of a JWT token.
	// It returns the user data encoded in the token or an error if validation fails.
	ValidateToken(token string) (*UserData, error)
}

// LoginRequest represents the request body for the login endpoint.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response body for successful login.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// RefreshResponse represents the response body for token refresh.
type RefreshResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// StatusResponse represents the response body for the status endpoint.
type StatusResponse struct {
	Authenticated bool     `json:"authenticated"`
	User          UserInfo `json:"user"`
}

// UserInfo represents public user information in responses.
type UserInfo struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

// UserData represents user information stored in tokens and validation results.
type UserData struct {
	Username string
	Role     string
}

// logoutResponse represents the response body for the logout endpoint.
type logoutResponse struct {
	Message string `json:"message"`
}

// AuthHandlers contains dependencies for authentication-related HTTP handlers.
type AuthHandlers struct {
	userStore   UserStore
	jwtService  JWTService
	logger      *slog.Logger
	tokenExpiry time.Duration
}

// NewAuthHandlers creates a new AuthHandlers instance with the provided dependencies.
// The tokenExpiry parameter specifies how long generated JWT tokens remain valid.
func NewAuthHandlers(userStore UserStore, jwtService JWTService, logger *slog.Logger, tokenExpiry time.Duration) *AuthHandlers {
	return &AuthHandlers{
		userStore:   userStore,
		jwtService:  jwtService,
		logger:      logger,
		tokenExpiry: tokenExpiry,
	}
}

// Login handles the POST /api/v1/auth/login endpoint.
// It validates user credentials, generates a JWT token, and returns user information.
//
// Request body: {"username": "admin", "password": "secret"}
// Response: {"token": "eyJ...", "expires_at": "2025-11-10T...", "user": {"username": "admin", "role": "admin"}}
//
// Status codes:
//   - 200: Login successful
//   - 400: Bad request (invalid JSON or missing fields)
//   - 401: Invalid credentials
//   - 413: Request body too large
//   - 500: Server error
//
// Security considerations:
//   - Request body limited to 1MB to prevent DoS attacks
//   - No credentials are logged
//   - Input validation prevents excessive field sizes
//   - Rate limiting should be enforced at the middleware level
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		slog.Warn("Login called with non-POST method", "method", r.Method)
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Limit request body size to 1MB to prevent DoS attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var loginReq LoginRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&loginReq); err != nil {
		// Check for request body too large
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			slog.Warn("Login request body too large", "limit", maxBytesErr.Limit)
			respondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		if errors.Is(err, io.EOF) {
			slog.Warn("Login request with empty body")
			respondError(w, http.StatusBadRequest, "username and password required")
			return
		}

		slog.Warn("Login request decode error", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	// Validate input - prevent excessively long strings
	if err := validateLoginInput(loginReq.Username, loginReq.Password); err != nil {
		slog.Warn("Login request validation failed", "error", err)
		respondError(w, http.StatusBadRequest, "invalid request parameters")
		return
	}

	// Validate credentials against user store
	userData, err := h.userStore.ValidateCredentials(loginReq.Username, loginReq.Password)
	if err != nil {
		// Log validation error but don't expose details to client
		slog.Warn("credential validation failed", "error", err)
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Generate JWT token with configured expiry
	expiresAt := time.Now().Add(h.tokenExpiry)
	token, err := h.jwtService.GenerateToken(userData, expiresAt)
	if err != nil {
		slog.Error("failed to generate JWT token",
			"username", userData.Username,
			"error", err)
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Build response
	response := LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			Username: userData.Username,
			Role:     userData.Role,
		},
	}

	slog.Info("user logged in successfully", "username", userData.Username)
	respondJSON(w, http.StatusOK, response)
}

// Logout handles the POST /api/v1/auth/logout endpoint.
// JWT tokens are stateless, so logout is a client-side operation that discards the token.
// This endpoint confirms the logout action and returns a success message.
//
// No authentication required (logout is always allowed).
// Response: {"message": "Logged out successfully"}
//
// Status codes:
//   - 200: Logout successful
//   - 405: Method not allowed
//   - 500: Server error
func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		slog.Warn("Logout called with non-POST method", "method", r.Method)
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Since JWT tokens are stateless, logout is a client-side operation.
	// The client simply discards the token.
	response := logoutResponse{
		Message: "Logged out successfully",
	}

	slog.Info("user logged out")
	respondJSON(w, http.StatusOK, response)
}

// Refresh handles the POST /api/v1/auth/refresh endpoint.
// It validates the provided JWT token and issues a new token with extended expiry.
//
// Requires valid JWT token in Authorization header (format: "Bearer <token>").
// Response: {"token": "eyJ...", "expires_at": "2025-11-10T..."}
//
// Status codes:
//   - 200: Token refreshed successfully
//   - 401: Invalid or expired token
//   - 405: Method not allowed
//   - 500: Server error
//
// Security considerations:
//   - Only accepts valid, non-expired tokens
//   - Authorization header is required
//   - No request body is processed
func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		slog.Warn("Refresh called with non-POST method", "method", r.Method)
		w.Header().Set("Allow", "POST")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	token, err := parseBearerToken(authHeader)
	if err != nil {
		slog.Warn("Refresh request with invalid Authorization header",
			"error", err)
		respondError(w, http.StatusUnauthorized, "invalid or missing token")
		return
	}

	// Validate token
	userData, err := h.jwtService.ValidateToken(token)
	if err != nil {
		slog.Warn("Refresh request with invalid token",
			"error", err)
		respondError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Generate new token with configured expiry
	expiresAt := time.Now().Add(h.tokenExpiry)
	newToken, err := h.jwtService.GenerateToken(userData, expiresAt)
	if err != nil {
		slog.Error("failed to generate refreshed JWT token",
			"username", userData.Username,
			"error", err)
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := RefreshResponse{
		Token:     newToken,
		ExpiresAt: expiresAt,
	}

	slog.Info("token refreshed", "username", userData.Username)
	respondJSON(w, http.StatusOK, response)
}

// Status handles the GET /api/v1/auth/status endpoint.
// It returns the current authentication status and user information based on the JWT token.
//
// Requires valid JWT token in Authorization header (format: "Bearer <token>").
// Response: {"authenticated": true, "user": {"username": "admin", "role": "admin"}}
//
// Status codes:
//   - 200: Authenticated
//   - 401: Not authenticated
//   - 405: Method not allowed
//   - 500: Server error
//
// Security considerations:
//   - Authorization header is required
//   - Only returns information from the validated token
//   - No sensitive data is exposed beyond what's in the token
func (h *AuthHandlers) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		slog.Warn("Status called with non-GET method", "method", r.Method)
		w.Header().Set("Allow", "GET")
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	token, err := parseBearerToken(authHeader)
	if err != nil {
		slog.Warn("Status request with invalid Authorization header",
			"error", err)
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Validate token
	userData, err := h.jwtService.ValidateToken(token)
	if err != nil {
		slog.Warn("Status request with invalid token",
			"error", err)
		respondError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	response := StatusResponse{
		Authenticated: true,
		User: UserInfo{
			Username: userData.Username,
			Role:     userData.Role,
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// validateLoginInput validates the login request input to prevent excessively large values.
// It checks that username and password are non-empty and within reasonable length limits.
// Max username length: 256 characters (standard for most systems)
// Max password length: 1024 characters (reasonable upper bound)
func validateLoginInput(username, password string) error {
	if username == "" {
		return errors.New("username is required")
	}
	if password == "" {
		return errors.New("password is required")
	}
	if len(username) > 256 {
		return errors.New("username is too long")
	}
	if len(password) > 1024 {
		return errors.New("password is too long")
	}
	return nil
}

// parseBearerToken extracts the JWT token from an Authorization header.
// Expected format: "Bearer <token>"
// Returns an error if the header is malformed or empty.
func parseBearerToken(authHeader string) (string, error) {
	const bearerPrefix = "Bearer "

	// Check if the header is empty
	if authHeader == "" {
		return "", errors.New("authorization header is missing")
	}

	// Check if the header starts with "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", errors.New("authorization header must start with 'Bearer '")
	}

	// Extract the token part
	token := strings.TrimPrefix(authHeader, bearerPrefix)

	// Trim any leading/trailing whitespace from the token
	token = strings.TrimSpace(token)

	// Ensure the token is not empty
	if token == "" {
		return "", errors.New("bearer token is empty")
	}

	return token, nil
}
