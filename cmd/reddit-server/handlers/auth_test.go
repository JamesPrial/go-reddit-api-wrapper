package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// mockUserStore is a test implementation of UserStore.
type mockUserStore struct {
	users map[string]*mockUser
}

type mockUser struct {
	username     string
	passwordHash string
	role         string
}

func newMockUserStore() *mockUserStore {
	// Hash passwords during initialization
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("secret"), 12)
	userHash, _ := bcrypt.GenerateFromPassword([]byte("password"), 12)
	testuserHash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), 12)

	return &mockUserStore{
		users: map[string]*mockUser{
			"admin": {
				username:     "admin",
				passwordHash: string(adminHash),
				role:         "admin",
			},
			"user": {
				username:     "user",
				passwordHash: string(userHash),
				role:         "viewer",
			},
			"testuser": {
				username:     "testuser",
				passwordHash: string(testuserHash),
				role:         "viewer",
			},
		},
	}
}

func (m *mockUserStore) ValidateCredentials(username, password string) (*UserData, error) {
	user, exists := m.users[username]
	if !exists {
		return nil, errors.New("invalid credentials")
	}

	// Use bcrypt comparison
	err := bcrypt.CompareHashAndPassword([]byte(user.passwordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return &UserData{
		Username: user.username,
		Role:     user.role,
	}, nil
}

// mockJWTService is a test implementation of JWTService.
type mockJWTService struct {
	tokens map[string]*UserData // token -> user data
}

func newMockJWTService() *mockJWTService {
	return &mockJWTService{
		tokens: make(map[string]*UserData),
	}
}

func (m *mockJWTService) GenerateToken(user *UserData, expiresAt time.Time) (string, error) {
	// Create a simple token for testing
	token := "token_" + user.Username + "_" + expiresAt.Format("20060102150405")
	m.tokens[token] = user
	return token, nil
}

func (m *mockJWTService) ValidateToken(token string) (*UserData, error) {
	if user, exists := m.tokens[token]; exists {
		return user, nil
	}
	return nil, errors.New("invalid token")
}

func newTestAuthHandlers() *AuthHandlers {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewAuthHandlers(newMockUserStore(), newMockJWTService(), logger, 24*time.Hour)
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "successful login with admin credentials",
			method:         http.MethodPost,
			body:           LoginRequest{Username: "admin", Password: "secret"},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"token", "expires_at", "user"},
		},
		{
			name:           "successful login with regular user credentials",
			method:         http.MethodPost,
			body:           LoginRequest{Username: "user", Password: "password"},
			expectedStatus: http.StatusOK,
			expectedFields: []string{"token", "expires_at", "user"},
		},
		{
			name:           "login with invalid credentials",
			method:         http.MethodPost,
			body:           LoginRequest{Username: "admin", Password: "wrongpass"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "login with non-existent user",
			method:         http.MethodPost,
			body:           LoginRequest{Username: "nonexistent", Password: "password"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "login with empty username",
			method:         http.MethodPost,
			body:           LoginRequest{Username: "", Password: "password"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "login with empty password",
			method:         http.MethodPost,
			body:           LoginRequest{Username: "admin", Password: ""},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "login with GET method",
			method:         http.MethodGet,
			body:           LoginRequest{Username: "admin", Password: "secret"},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "login with invalid JSON",
			method:         http.MethodPost,
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestAuthHandlers()
			w := httptest.NewRecorder()
			r := buildAuthRequest(t, tt.method, "/api/v1/auth/login", tt.body)

			h.Login(w, r)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// For successful login, verify response structure
			if tt.expectedStatus == http.StatusOK {
				var resp LoginResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.Token == "" {
					t.Error("token should not be empty")
				}
				if resp.ExpiresAt.IsZero() {
					t.Error("expires_at should not be zero")
				}
				if resp.User.Username == "" {
					t.Error("user.username should not be empty")
				}
			}
		})
	}
}

func TestLoginXLargePayload(t *testing.T) {
	h := newTestAuthHandlers()
	w := httptest.NewRecorder()

	// Create valid JSON with a field that's larger than 1MB
	// This ensures the MaxBytesReader actually triggers when decoding
	largeValue := strings.Repeat("A", 2<<20) // 2MB string
	payload := map[string]string{
		"username": largeValue,
		"password": "test",
	}
	largeBody, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(largeBody))
	r.ContentLength = int64(len(largeBody))

	h.Login(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d for large payload, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "successful logout",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "logout with GET method",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "logout with PUT method",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestAuthHandlers()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, "/api/v1/auth/logout", nil)

			h.Logout(w, r)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// For successful logout, verify response message
			if tt.expectedStatus == http.StatusOK {
				var resp logoutResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.Message != "Logged out successfully" {
					t.Errorf("expected message 'Logged out successfully', got %q", resp.Message)
				}
			}
		})
	}
}

func TestRefresh(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		authHeader     string
		expectedStatus int
		hasToken       bool
	}{
		{
			name:           "successful token refresh",
			method:         http.MethodPost,
			authHeader:     "Bearer token_admin_20250101120000",
			expectedStatus: http.StatusOK,
			hasToken:       true,
		},
		{
			name:           "refresh without Authorization header",
			method:         http.MethodPost,
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "refresh with malformed Authorization header",
			method:         http.MethodPost,
			authHeader:     "InvalidFormat token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "refresh with invalid token",
			method:         http.MethodPost,
			authHeader:     "Bearer invalid_token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "refresh with GET method",
			method:         http.MethodGet,
			authHeader:     "Bearer token_admin_20250101120000",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestAuthHandlers()

			// First, generate a valid token
			userData := &UserData{Username: "admin", Role: "admin"}
			token, _ := h.jwtService.GenerateToken(userData, time.Now().Add(24*time.Hour))

			// Update test with valid token
			if tt.name == "successful token refresh" {
				tt.authHeader = "Bearer " + token
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, "/api/v1/auth/refresh", nil)
			if tt.authHeader != "" {
				r.Header.Set("Authorization", tt.authHeader)
			}

			h.Refresh(w, r)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// For successful refresh, verify response structure
			if tt.expectedStatus == http.StatusOK && tt.hasToken {
				var resp RefreshResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if resp.Token == "" {
					t.Error("token should not be empty")
				}
				if resp.ExpiresAt.IsZero() {
					t.Error("expires_at should not be zero")
				}
			}
		})
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "successful status check",
			method:         http.MethodGet,
			authHeader:     "Bearer token_admin_20250101120000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "status without Authorization header",
			method:         http.MethodGet,
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "status with malformed Authorization header",
			method:         http.MethodGet,
			authHeader:     "InvalidFormat token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "status with invalid token",
			method:         http.MethodGet,
			authHeader:     "Bearer invalid_token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "status with POST method",
			method:         http.MethodPost,
			authHeader:     "Bearer token_admin_20250101120000",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestAuthHandlers()

			// First, generate a valid token
			userData := &UserData{Username: "admin", Role: "admin"}
			token, _ := h.jwtService.GenerateToken(userData, time.Now().Add(24*time.Hour))

			// Update test with valid token
			if tt.name == "successful status check" {
				tt.authHeader = "Bearer " + token
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(tt.method, "/api/v1/auth/status", nil)
			if tt.authHeader != "" {
				r.Header.Set("Authorization", tt.authHeader)
			}

			h.Status(w, r)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// For successful status, verify response structure
			if tt.expectedStatus == http.StatusOK {
				var resp StatusResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if !resp.Authenticated {
					t.Error("authenticated should be true")
				}
				if resp.User.Username == "" {
					t.Error("user.username should not be empty")
				}
				if resp.User.Role == "" {
					t.Error("user.role should not be empty")
				}
			}
		})
	}
}

func TestValidateLoginInput(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		password  string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid credentials",
			username:  "admin",
			password:  "secret",
			wantError: false,
		},
		{
			name:      "empty username",
			username:  "",
			password:  "secret",
			wantError: true,
			errorMsg:  "username is required",
		},
		{
			name:      "empty password",
			username:  "admin",
			password:  "",
			wantError: true,
			errorMsg:  "password is required",
		},
		{
			name:      "username too long",
			username:  strings.Repeat("a", 257),
			password:  "secret",
			wantError: true,
			errorMsg:  "username is too long",
		},
		{
			name:      "password too long",
			username:  "admin",
			password:  strings.Repeat("a", 1025),
			wantError: true,
			errorMsg:  "password is too long",
		},
		{
			name:      "username at max length",
			username:  strings.Repeat("a", 256),
			password:  "secret",
			wantError: false,
		},
		{
			name:      "password at max length",
			username:  "admin",
			password:  strings.Repeat("a", 1024),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLoginInput(tt.username, tt.password)

			if (err != nil) != tt.wantError {
				t.Errorf("expected error: %v, got: %v", tt.wantError, err)
			}

			if tt.wantError && err.Error() != tt.errorMsg {
				t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantToken  string
		wantError  bool
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer eyJhbGc...",
			wantToken:  "eyJhbGc...",
			wantError:  false,
		},
		{
			name:       "empty header",
			authHeader: "",
			wantToken:  "",
			wantError:  true,
		},
		{
			name:       "missing bearer prefix",
			authHeader: "eyJhbGc...",
			wantToken:  "",
			wantError:  true,
		},
		{
			name:       "only bearer prefix",
			authHeader: "Bearer ",
			wantToken:  "",
			wantError:  true,
		},
		{
			name:       "bearer with extra spaces",
			authHeader: "Bearer  ",
			wantToken:  "",
			wantError:  true,
		},
		{
			name:       "case sensitive bearer",
			authHeader: "bearer eyJhbGc...",
			wantToken:  "",
			wantError:  true,
		},
		{
			name:       "token with spaces",
			authHeader: "Bearer token with spaces",
			wantToken:  "token with spaces",
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := parseBearerToken(tt.authHeader)

			if (err != nil) != tt.wantError {
				t.Errorf("expected error: %v, got: %v", tt.wantError, err)
			}

			if !tt.wantError && token != tt.wantToken {
				t.Errorf("expected token %q, got %q", tt.wantToken, token)
			}
		})
	}
}

// buildAuthRequest is a helper to build authentication-related requests.
func buildAuthRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()

	var bodyReader io.Reader
	isInvalidJSON := false

	if body != nil {
		switch v := body.(type) {
		case string:
			if v == "invalid json" {
				isInvalidJSON = true
				bodyReader = strings.NewReader(v)
			} else {
				bodyReader = strings.NewReader(v)
			}
		default:
			jsonBody, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("failed to marshal body: %v", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
		}
	}

	r := httptest.NewRequest(method, path, bodyReader)
	if body != nil && !isInvalidJSON {
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

func TestNewAuthHandlers(t *testing.T) {
	userStore := newMockUserStore()
	jwtService := newMockJWTService()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	handlers := NewAuthHandlers(userStore, jwtService, logger, 24*time.Hour)

	if handlers == nil {
		t.Error("NewAuthHandlers returned nil")
	}
	if handlers.userStore != userStore {
		t.Error("userStore not set correctly")
	}
	if handlers.jwtService != jwtService {
		t.Error("jwtService not set correctly")
	}
	if handlers.logger != logger {
		t.Error("logger not set correctly")
	}
}

func BenchmarkLogin(b *testing.B) {
	h := newTestAuthHandlers()
	body, _ := json.Marshal(LoginRequest{Username: "admin", Password: "secret"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.Login(w, r)
	}
}

func BenchmarkValidateLoginInput(b *testing.B) {
	username := "admin"
	password := "secret"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateLoginInput(username, password)
	}
}

func BenchmarkParseBearerToken(b *testing.B) {
	authHeader := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseBearerToken(authHeader)
	}
}
