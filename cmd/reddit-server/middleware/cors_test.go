package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000", "https://example.com"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	tests := []struct {
		name   string
		origin string
	}{
		{
			name:   "localhost origin",
			origin: "http://localhost:3000",
		},
		{
			name:   "example.com origin",
			origin: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify CORS headers are set
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.origin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.origin)
			}
			if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
				t.Error("Access-Control-Allow-Methods not set")
			}
			if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
				t.Error("Access-Control-Allow-Headers not set")
			}
			if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
				t.Errorf("Access-Control-Max-Age = %q, want %q", got, "86400")
			}

			// Verify response is successful
			if rec.Code != http.StatusOK {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != "OK" {
				t.Errorf("Response body = %q, want %q", rec.Body.String(), "OK")
			}
		})
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	tests := []struct {
		name   string
		origin string
	}{
		{
			name:   "different port",
			origin: "http://localhost:8080",
		},
		{
			name:   "different domain",
			origin: "https://evil.com",
		},
		{
			name:   "different protocol",
			origin: "https://localhost:3000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify CORS headers are NOT set
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
			}
			if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
				t.Errorf("Access-Control-Allow-Methods = %q, want empty", got)
			}
			if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "" {
				t.Errorf("Access-Control-Allow-Headers = %q, want empty", got)
			}
			if got := rec.Header().Get("Access-Control-Max-Age"); got != "" {
				t.Errorf("Access-Control-Max-Age = %q, want empty", got)
			}

			// Verify response is still successful (CORS is permissive for non-OPTIONS)
			if rec.Code != http.StatusOK {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestCORS_PreflightAllowed(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS requests")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify CORS headers are set
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST, OPTIONS")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "Content-Type, Authorization")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, "86400")
	}

	// Verify response is 200 OK for preflight
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCORS_PreflightDisallowed(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS requests")
	}))

	tests := []struct {
		name   string
		origin string
	}{
		{
			name:   "disallowed origin",
			origin: "https://evil.com",
		},
		{
			name:   "different port",
			origin: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify CORS headers are NOT set
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
			}

			// Verify response is 403 Forbidden for disallowed preflight
			if rec.Code != http.StatusForbidden {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusForbidden)
			}
		})
	}
}

func TestCORS_NoCORSConfiguration(t *testing.T) {
	// Empty allowed origins list
	handler := CORS([]string{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify CORS headers are NOT set when no origins are configured
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Access-Control-Allow-Methods = %q, want empty", got)
	}

	// Verify response is still successful
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "OK" {
		t.Errorf("Response body = %q, want %q", rec.Body.String(), "OK")
	}
}

func TestCORS_MaxAgeHeader(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify Access-Control-Max-Age is set to 86400 (24 hours)
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, "86400")
	}
}

func TestCORS_TrailingSlashNormalization(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		shouldAllow    bool
	}{
		{
			name:           "allowed origin with trailing slash, request without",
			allowedOrigins: []string{"http://localhost:3000/"},
			requestOrigin:  "http://localhost:3000",
			shouldAllow:    true,
		},
		{
			name:           "allowed origin without trailing slash, request with",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "http://localhost:3000/",
			shouldAllow:    true,
		},
		{
			name:           "both with trailing slash",
			allowedOrigins: []string{"http://localhost:3000/"},
			requestOrigin:  "http://localhost:3000/",
			shouldAllow:    true,
		},
		{
			name:           "both without trailing slash",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "http://localhost:3000",
			shouldAllow:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CORS(tt.allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.requestOrigin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// The middleware sets Access-Control-Allow-Origin to the request's Origin header value
			// (not the normalized version), so we expect the exact request origin
			if tt.shouldAllow {
				if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.requestOrigin {
					t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.requestOrigin)
				}
			} else {
				if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
					t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
				}
			}
		})
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Request without Origin header (same-origin request)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify CORS headers are NOT set
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}

	// Verify response is still successful
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCORS_EmptyOriginHeader(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify CORS headers are NOT set for empty origin
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}

	// Verify response is still successful
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCORS_MultipleAllowedOrigins(t *testing.T) {
	allowedOrigins := []string{
		"http://localhost:3000",
		"https://example.com",
		"https://app.example.com",
	}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	for _, origin := range allowedOrigins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify CORS header matches the request origin
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, origin)
			}

			// Verify response is successful
			if rec.Code != http.StatusOK {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestCORS_AllowedMethods(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify allowed methods
	allowedMethods := rec.Header().Get("Access-Control-Allow-Methods")
	if allowedMethods != "GET, POST, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", allowedMethods, "GET, POST, OPTIONS")
	}
}

func TestCORS_AllowedHeaders(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify allowed headers
	allowedHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if allowedHeaders != "Content-Type, Authorization" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", allowedHeaders, "Content-Type, Authorization")
	}
}

func TestCORS_CaseSensitiveOrigin(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// NOTE: This test verifies case-sensitive origin matching.
	// Per RFC 6454 Section 4, origin comparison should be case-insensitive for
	// the scheme and host portions. However, this implementation uses exact
	// string matching for simplicity and security. This is a deliberate design
	// choice that errs on the side of being more restrictive.
	// If case-insensitive matching is needed in the future, the isOriginAllowed
	// function in cors.go should be updated to normalize origins before comparison.
	tests := []struct {
		name        string
		origin      string
		shouldAllow bool
	}{
		{
			name:        "exact match",
			origin:      "http://localhost:3000",
			shouldAllow: true,
		},
		{
			name:        "uppercase HTTP",
			origin:      "HTTP://localhost:3000",
			shouldAllow: false, // Case-sensitive comparison
		},
		{
			name:        "uppercase domain",
			origin:      "http://LOCALHOST:3000",
			shouldAllow: false, // Case-sensitive comparison
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.shouldAllow {
				// Normalize expected origin (remove trailing slash)
				expectedOrigin := tt.origin
				if len(expectedOrigin) > 0 && expectedOrigin[len(expectedOrigin)-1] == '/' {
					expectedOrigin = expectedOrigin[:len(expectedOrigin)-1]
				}
				if got != expectedOrigin {
					t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, expectedOrigin)
				}
			} else {
				if got != "" {
					t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
				}
			}
		})
	}
}

func TestCORS_NonOptionsMethodsPassThrough(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handlerCalled := false
	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Handler called"))
	}))

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Verify handler was called for non-OPTIONS methods
			if !handlerCalled {
				t.Error("Handler should be called for non-OPTIONS methods")
			}

			// Verify CORS headers are still set
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
			}

			// Verify response
			if rec.Code != http.StatusOK {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestCORS_OptionsRequestShortCircuits(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}

	handlerCalled := false
	handler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify handler was NOT called for OPTIONS request
	if handlerCalled {
		t.Error("Handler should not be called for OPTIONS requests (middleware should short-circuit)")
	}

	// Verify CORS headers are set
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
	}

	// Verify response is 200 OK
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}
}
