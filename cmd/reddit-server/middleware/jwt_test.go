package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// mockJWTService is a mock implementation of JWTService for testing.
type mockJWTService struct {
	validTokens map[string]*UserClaims
	errOnToken  map[string]bool // tokens that should return an error
}

// newMockJWTService creates a new mock JWT service.
func newMockJWTService() *mockJWTService {
	return &mockJWTService{
		validTokens: make(map[string]*UserClaims),
		errOnToken:  make(map[string]bool),
	}
}

// ValidateToken validates a token against the mock service.
func (m *mockJWTService) ValidateToken(token string) (*UserClaims, error) {
	if m.errOnToken[token] {
		return nil, ErrInvalidToken
	}

	claims, ok := m.validTokens[token]
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// addToken adds a valid token to the mock service.
func (m *mockJWTService) addToken(token string, claims *UserClaims) {
	m.validTokens[token] = claims
}

// addInvalidToken marks a token as invalid.
func (m *mockJWTService) addInvalidToken(token string) {
	m.errOnToken[token] = true
}

// dummyJWTHandler returns a simple 200 OK response.
func dummyJWTHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// TestJWTAuthWithRole_ValidToken tests that valid tokens pass authentication.
func TestJWTAuthWithRole_ValidToken(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addToken("valid-token", &UserClaims{
		Username: "testuser",
		Role:     "user",
	})

	middleware := JWTAuthWithRole(mockService, []string{})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response is 200 OK (handler was called)
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify response body is from the handler
	if rec.Body.String() != "OK" {
		t.Errorf("Response body = %q, want %q", rec.Body.String(), "OK")
	}
}

// TestJWTAuth_InvalidToken tests that invalid tokens are rejected with 401.
func TestJWTAuthWithRole_InvalidToken(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addInvalidToken("invalid-token")

	middleware := JWTAuthWithRole(mockService, []string{})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response is 401 Unauthorized
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Verify JSON error response format
	var errResp jwtErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if errResp.Error == "" {
		t.Error("Error message is empty")
	}
	if errResp.Code != "INVALID_TOKEN" {
		t.Errorf("Error code = %q, want %q", errResp.Code, "INVALID_TOKEN")
	}

	// Verify response content-type
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
}

// TestJWTAuth_MissingToken tests that missing Authorization header returns 401.
func TestJWTAuthWithRole_MissingToken(t *testing.T) {
	mockService := newMockJWTService()

	middleware := JWTAuthWithRole(mockService, []string{})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	// Don't set Authorization header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response is 401 Unauthorized
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	// Verify JSON error response format
	var errResp jwtErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if errResp.Code != "MISSING_TOKEN" {
		t.Errorf("Error code = %q, want %q", errResp.Code, "MISSING_TOKEN")
	}
}

// TestJWTAuth_MalformedHeader tests that malformed Authorization header returns 401.
func TestJWTAuthWithRole_MalformedHeader(t *testing.T) {
	mockService := newMockJWTService()

	middleware := JWTAuthWithRole(mockService, []string{})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	tests := []struct {
		name            string
		authHeaderValue string
		expectedCode    string
	}{
		{
			name:            "missing Bearer prefix",
			authHeaderValue: "valid-token",
			expectedCode:    "INVALID_FORMAT",
		},
		{
			name:            "wrong prefix",
			authHeaderValue: "Basic valid-token",
			expectedCode:    "INVALID_FORMAT",
		},
		{
			name:            "Bearer without token",
			authHeaderValue: "Bearer ",
			expectedCode:    "INVALID_FORMAT",
		},
		{
			name:            "Bearer only",
			authHeaderValue: "Bearer",
			expectedCode:    "INVALID_FORMAT",
		},
		{
			name:            "lowercase bearer",
			authHeaderValue: "bearer valid-token",
			expectedCode:    "INVALID_FORMAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
			req.Header.Set("Authorization", tt.authHeaderValue)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify response is 401 Unauthorized
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}

			// Verify error code
			var errResp jwtErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode JSON response: %v", err)
			}
			if errResp.Code != tt.expectedCode {
				t.Errorf("Error code = %q, want %q", errResp.Code, tt.expectedCode)
			}
		})
	}
}

// TestJWTAuth_ExemptPath tests that exempt paths work without authentication.
func TestJWTAuthWithRole_ExemptPath(t *testing.T) {
	mockService := newMockJWTService()

	tests := []struct {
		name        string
		path        string
		isExempt    bool
		exemptPaths []string
	}{
		{
			name:        "/health endpoint without auth",
			path:        "/health",
			isExempt:    true,
			exemptPaths: []string{"/health"},
		},
		{
			name:        "/api/v1/auth/ prefix match",
			path:        "/api/v1/auth/login",
			isExempt:    true,
			exemptPaths: []string{"/api/v1/auth/"},
		},
		{
			name:        "/app/ prefix match",
			path:        "/app/index.html",
			isExempt:    true,
			exemptPaths: []string{"/app/"},
		},
		{
			name:        "/api/v1/user/me requires auth",
			path:        "/api/v1/user/me",
			isExempt:    false,
			exemptPaths: []string{"/health"},
		},
		{
			name:        "path not in exempt list",
			path:        "/api/test",
			isExempt:    false,
			exemptPaths: []string{"/health"},
		},
		{
			name:        "empty exempt paths list",
			path:        "/any/path",
			isExempt:    false,
			exemptPaths: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := JWTAuthWithRole(mockService, tt.exemptPaths)
			handler := middleware(http.HandlerFunc(dummyJWTHandler))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			// Don't set Authorization header to test exemption
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if tt.isExempt {
				// Exempt path should pass through without auth
				if rec.Code != http.StatusOK {
					t.Errorf("Response status = %d, want %d (exempt path should allow access)", rec.Code, http.StatusOK)
				}
			} else {
				// Non-exempt path should require auth
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("Response status = %d, want %d (non-exempt path should require auth)", rec.Code, http.StatusUnauthorized)
				}
			}
		})
	}
}

// TestJWTAuth_ContextInjection tests that user claims are injected into context.
func TestJWTAuthWithRole_ContextInjection(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addToken("valid-token", &UserClaims{
		Username: "alice",
		Role:     "admin",
	})

	var capturedUsername string
	var capturedRole string
	var capturedClaims *UserClaims

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, role, ok := GetUserFromContext(r)
		if ok {
			capturedUsername = username
			capturedRole = role
		}

		claims, ok := r.Context().Value(userContextKey).(*UserClaims)
		if ok {
			capturedClaims = claims
		}

		w.WriteHeader(http.StatusOK)
	})

	middleware := JWTAuthWithRole(mockService, []string{})
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Verify context values were injected
	if capturedUsername != "alice" {
		t.Errorf("Username = %q, want %q", capturedUsername, "alice")
	}
	if capturedRole != "admin" {
		t.Errorf("Role = %q, want %q", capturedRole, "admin")
	}
	if capturedClaims == nil {
		t.Error("UserClaims not found in context")
	} else {
		if capturedClaims.Username != "alice" {
			t.Errorf("Claims.Username = %q, want %q", capturedClaims.Username, "alice")
		}
		if capturedClaims.Role != "admin" {
			t.Errorf("Claims.Role = %q, want %q", capturedClaims.Role, "admin")
		}
	}
}

// TestRoleRequired_AdminAccess tests that admin role can access all endpoints.
func TestRoleRequired_AdminAccess(t *testing.T) {
	mockService := newMockJWTService()
	adminToken := &UserClaims{Username: "admin_user", Role: "admin"}
	mockService.addToken("admin-token", adminToken)

	jwtMiddleware := JWTAuthWithRole(mockService, []string{})
	roleMiddleware := RoleRequired("viewer")
	handler := roleMiddleware(http.HandlerFunc(dummyJWTHandler))
	handler = jwtMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Admin accessing viewer endpoint: status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestRoleRequired_ModeratorAccess tests role hierarchy.
func TestRoleRequired_ModeratorAccess(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		claims       *UserClaims
		requiredRole string
		shouldPass   bool
	}{
		{
			name:         "moderator accessing moderator endpoint",
			token:        "mod-token",
			claims:       &UserClaims{Username: "moderator", Role: "moderator"},
			requiredRole: "moderator",
			shouldPass:   true,
		},
		{
			name:         "moderator accessing viewer endpoint",
			token:        "mod-token-2",
			claims:       &UserClaims{Username: "moderator2", Role: "moderator"},
			requiredRole: "viewer",
			shouldPass:   true,
		},
		{
			name:         "viewer accessing moderator endpoint",
			token:        "viewer-token",
			claims:       &UserClaims{Username: "viewer", Role: "viewer"},
			requiredRole: "moderator",
			shouldPass:   false,
		},
		{
			name:         "viewer accessing viewer endpoint",
			token:        "viewer-token-2",
			claims:       &UserClaims{Username: "viewer2", Role: "viewer"},
			requiredRole: "viewer",
			shouldPass:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testService := newMockJWTService()
			testService.addToken(tt.token, tt.claims)

			jwtMiddleware := JWTAuthWithRole(testService, []string{})
			roleMiddleware := RoleRequired(tt.requiredRole)
			handler := roleMiddleware(http.HandlerFunc(dummyJWTHandler))
			handler = jwtMiddleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			expectedStatus := http.StatusOK
			if !tt.shouldPass {
				expectedStatus = http.StatusForbidden
			}

			if rec.Code != expectedStatus {
				t.Errorf("Status = %d, want %d", rec.Code, expectedStatus)
			}
		})
	}
}

// TestRoleRequired_MissingContext tests that missing user context returns 401.
func TestRoleRequired_MissingContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	roleMiddleware := RoleRequired("admin")
	wrappedHandler := roleMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	// Don't set Authorization header, so context will be missing
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	// Should return 401 because user context is missing
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Response status = %d, want %d (missing context should return 401)", rec.Code, http.StatusUnauthorized)
	}

	var errResp jwtErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if errResp.Code != "MISSING_TOKEN" {
		t.Errorf("Error code = %q, want %q", errResp.Code, "MISSING_TOKEN")
	}
}

// TestRoleRequired_InsufficientRole tests that insufficient role returns 403.
func TestRoleRequired_InsufficientRole(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addToken("viewer-token", &UserClaims{
		Username: "viewer_user",
		Role:     "viewer",
	})

	jwtMiddleware := JWTAuthWithRole(mockService, []string{})
	roleMiddleware := RoleRequired("admin")
	handler := roleMiddleware(http.HandlerFunc(dummyJWTHandler))
	handler = jwtMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer viewer-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 403 because role is insufficient
	if rec.Code != http.StatusForbidden {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	// Verify error response
	var errResp jwtErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if errResp.Code != "INSUFFICIENT_ROLE" {
		t.Errorf("Error code = %q, want %q", errResp.Code, "INSUFFICIENT_ROLE")
	}
}

// TestExtractTokenFromHeader tests the token extraction function.
func TestExtractTokenFromHeader(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		shouldSucceed bool
		expectedToken string
	}{
		{
			name:          "valid format",
			authHeader:    "Bearer valid-token",
			shouldSucceed: true,
			expectedToken: "valid-token",
		},
		{
			name:          "no Bearer prefix",
			authHeader:    "valid-token",
			shouldSucceed: false,
		},
		{
			name:          "wrong prefix",
			authHeader:    "Basic valid-token",
			shouldSucceed: false,
		},
		{
			name:          "Bearer only",
			authHeader:    "Bearer",
			shouldSucceed: false,
		},
		{
			name:          "Bearer with space",
			authHeader:    "Bearer ",
			shouldSucceed: false,
		},
		{
			name:          "lowercase bearer",
			authHeader:    "bearer valid-token",
			shouldSucceed: false,
		},
		{
			name:          "token with spaces",
			authHeader:    "Bearer token with spaces",
			shouldSucceed: true,
			expectedToken: "token with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := extractTokenFromHeader(tt.authHeader)

			if tt.shouldSucceed {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if token != tt.expectedToken {
					t.Errorf("Token = %q, want %q", token, tt.expectedToken)
				}
			} else {
				if err == nil {
					t.Error("Expected error but got success")
				}
				if token != "" {
					t.Errorf("Expected empty token but got %q", token)
				}
			}
		})
	}
}

// TestMatchesExemptPath tests the path matching function.
func TestMatchesExemptPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		exemptPaths []string
		shouldMatch bool
	}{
		{
			name:        "exact match",
			path:        "/health",
			exemptPaths: []string{"/health"},
			shouldMatch: true,
		},
		{
			name:        "prefix match with trailing slash",
			path:        "/api/v1/auth/login",
			exemptPaths: []string{"/api/v1/auth/"},
			shouldMatch: true,
		},
		{
			name:        "prefix match for root of prefix path",
			path:        "/api/v1/auth",
			exemptPaths: []string{"/api/v1/auth/"},
			shouldMatch: true,
		},
		{
			name:        "no match - different path",
			path:        "/api/v1/user/me",
			exemptPaths: []string{"/health"},
			shouldMatch: false,
		},
		{
			name:        "prefix match for root of prefix path",
			path:        "/app",
			exemptPaths: []string{"/app/"},
			shouldMatch: true,
		},
		{
			name:        "no match - case sensitive",
			path:        "/Health",
			exemptPaths: []string{"/health"},
			shouldMatch: false,
		},
		{
			name:        "multiple exempt paths - first match",
			path:        "/health",
			exemptPaths: []string{"/health", "/status", "/metrics"},
			shouldMatch: true,
		},
		{
			name:        "multiple exempt paths - middle match",
			path:        "/status",
			exemptPaths: []string{"/health", "/status", "/metrics"},
			shouldMatch: true,
		},
		{
			name:        "multiple exempt paths - no match",
			path:        "/api/test",
			exemptPaths: []string{"/health", "/status", "/metrics"},
			shouldMatch: false,
		},
		{
			name:        "empty exempt paths",
			path:        "/any/path",
			exemptPaths: []string{},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesExemptPath(tt.path, tt.exemptPaths)
			if result != tt.shouldMatch {
				t.Errorf("matchesExemptPath(%q, ...) = %v, want %v", tt.path, result, tt.shouldMatch)
			}
		})
	}
}

// TestHasRequiredRole tests the role hierarchy checking function.
func TestHasRequiredRole(t *testing.T) {
	tests := []struct {
		name         string
		userRole     string
		requiredRole string
		shouldAllow  bool
	}{
		{
			name:         "admin accessing admin",
			userRole:     "admin",
			requiredRole: "admin",
			shouldAllow:  true,
		},
		{
			name:         "admin accessing moderator",
			userRole:     "admin",
			requiredRole: "moderator",
			shouldAllow:  true,
		},
		{
			name:         "admin accessing viewer",
			userRole:     "admin",
			requiredRole: "viewer",
			shouldAllow:  true,
		},
		{
			name:         "moderator accessing admin",
			userRole:     "moderator",
			requiredRole: "admin",
			shouldAllow:  false,
		},
		{
			name:         "moderator accessing moderator",
			userRole:     "moderator",
			requiredRole: "moderator",
			shouldAllow:  true,
		},
		{
			name:         "moderator accessing viewer",
			userRole:     "moderator",
			requiredRole: "viewer",
			shouldAllow:  true,
		},
		{
			name:         "viewer accessing admin",
			userRole:     "viewer",
			requiredRole: "admin",
			shouldAllow:  false,
		},
		{
			name:         "viewer accessing moderator",
			userRole:     "viewer",
			requiredRole: "moderator",
			shouldAllow:  false,
		},
		{
			name:         "viewer accessing viewer",
			userRole:     "viewer",
			requiredRole: "viewer",
			shouldAllow:  true,
		},
		{
			name:         "unknown role",
			userRole:     "unknown",
			requiredRole: "viewer",
			shouldAllow:  false,
		},
		{
			name:         "unknown required role",
			userRole:     "admin",
			requiredRole: "unknown",
			shouldAllow:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRequiredRole(tt.userRole, tt.requiredRole)
			if result != tt.shouldAllow {
				t.Errorf("hasRequiredRole(%q, %q) = %v, want %v", tt.userRole, tt.requiredRole, result, tt.shouldAllow)
			}
		})
	}
}

// TestGetUserFromContext tests the context extraction function.
func TestGetUserFromContext(t *testing.T) {
	tests := []struct {
		name         string
		setupCtx     func() context.Context
		shouldFind   bool
		expectedUsr  string
		expectedRole string
	}{
		{
			name: "context with valid claims",
			setupCtx: func() context.Context {
				claims := &UserClaims{Username: "alice", Role: "admin"}
				ctx := context.WithValue(context.Background(), userContextKey, claims)
				return ctx
			},
			shouldFind:   true,
			expectedUsr:  "alice",
			expectedRole: "admin",
		},
		{
			name: "empty context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			shouldFind: false,
		},
		{
			name: "context with wrong type",
			setupCtx: func() context.Context {
				ctx := context.WithValue(context.Background(), userContextKey, "not a pointer")
				return ctx
			},
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

			username, role, ok := GetUserFromContext(req)

			if ok != tt.shouldFind {
				t.Errorf("Found claims = %v, want %v", ok, tt.shouldFind)
			}

			if tt.shouldFind {
				if username != tt.expectedUsr {
					t.Errorf("Username = %q, want %q", username, tt.expectedUsr)
				}
				if role != tt.expectedRole {
					t.Errorf("Role = %q, want %q", role, tt.expectedRole)
				}
			}
		})
	}
}

// TestJWTAuth_ConcurrentRequests verifies the middleware handles concurrent requests correctly.
func TestJWTAuthWithRole_ConcurrentRequests(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addToken("valid-token-1", &UserClaims{Username: "user1", Role: "admin"})
	mockService.addToken("valid-token-2", &UserClaims{Username: "user2", Role: "viewer"})
	mockService.addInvalidToken("invalid-token")

	middleware := JWTAuthWithRole(mockService, []string{"/health"})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	const numGoroutines = 50
	const requestsPerGoroutine = 20
	var wg sync.WaitGroup

	resultsChan := make(chan struct {
		status int
		err    string
	}, numGoroutines*requestsPerGoroutine)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for req := 0; req < requestsPerGoroutine; req++ {
				var expectedStatus int
				var authHeader string
				var path string

				// Mix different types of requests
				switch req % 3 {
				case 0:
					// Valid token request
					expectedStatus = http.StatusOK
					authHeader = "Bearer valid-token-" + string(rune('1'+(req%2)))
					path = "/api/v1/user/me"

				case 1:
					// Invalid token request
					expectedStatus = http.StatusUnauthorized
					authHeader = "Bearer invalid-token"
					path = "/api/v1/user/me"

				case 2:
					// Exempt path request
					expectedStatus = http.StatusOK
					path = "/health"
				}

				httpReq := httptest.NewRequest(http.MethodGet, path, nil)
				if authHeader != "" {
					httpReq.Header.Set("Authorization", authHeader)
				}
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, httpReq)

				if rec.Code != expectedStatus {
					resultsChan <- struct {
						status int
						err    string
					}{status: rec.Code, err: "unexpected status"}
				} else {
					resultsChan <- struct {
						status int
						err    string
					}{status: rec.Code, err: ""}
				}
			}
		}(g)
	}

	wg.Wait()
	close(resultsChan)

	// Check for errors
	errorCount := 0
	for result := range resultsChan {
		if result.err != "" {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors in concurrent requests, but got %d", errorCount)
	}
}

// TestJWTAuth_ErrorResponseFormat verifies error response format is consistent.
func TestJWTAuthWithRole_ErrorResponseFormat(t *testing.T) {
	mockService := newMockJWTService()

	middleware := JWTAuthWithRole(mockService, []string{})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	tests := []struct {
		name            string
		authHeaderValue string
		expectedCode    string
	}{
		{
			name:            "missing header",
			authHeaderValue: "",
			expectedCode:    "MISSING_TOKEN",
		},
		{
			name:            "invalid format",
			authHeaderValue: "InvalidFormat",
			expectedCode:    "INVALID_FORMAT",
		},
		{
			name:            "invalid token",
			authHeaderValue: "Bearer invalid-token",
			expectedCode:    "INVALID_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid token" {
				// Set up this token to be invalid
				mockService.addInvalidToken("invalid-token")
			}

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.authHeaderValue != "" {
				req.Header.Set("Authorization", tt.authHeaderValue)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify status is 401
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}

			// Verify JSON format
			var errResp jwtErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode JSON response: %v", err)
			}

			if errResp.Error == "" {
				t.Error("Error message is empty")
			}

			if errResp.Code != tt.expectedCode {
				t.Errorf("Error code = %q, want %q", errResp.Code, tt.expectedCode)
			}
		})
	}
}

// TestJWTAuth_AllHTTPMethods verifies middleware works with all HTTP methods.
func TestJWTAuthWithRole_AllHTTPMethods(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addToken("valid-token", &UserClaims{Username: "user", Role: "user"})

	middleware := JWTAuthWithRole(mockService, []string{})
	handler := middleware(http.HandlerFunc(dummyJWTHandler))

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want %d", method, rec.Code, http.StatusOK)
			}
		})
	}
}

// TestRoleRequired_AdminCanAccessAllEndpoints verifies admin role can access all role levels.
func TestRoleRequired_AdminCanAccessAllEndpoints(t *testing.T) {
	mockService := newMockJWTService()
	mockService.addToken("admin-token", &UserClaims{Username: "admin", Role: "admin"})

	roles := []string{"viewer", "moderator", "admin"}

	for _, requiredRole := range roles {
		t.Run("admin accessing "+requiredRole, func(t *testing.T) {
			jwtMiddleware := JWTAuthWithRole(mockService, []string{})
			roleMiddleware := RoleRequired(requiredRole)
			handler := roleMiddleware(http.HandlerFunc(dummyJWTHandler))
			handler = jwtMiddleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			req.Header.Set("Authorization", "Bearer admin-token")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Admin accessing %s: status = %d, want %d", requiredRole, rec.Code, http.StatusOK)
			}
		})
	}
}
