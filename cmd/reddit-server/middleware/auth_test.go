package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit-server/config"
)

func TestAuthFromConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		creds := GetCredentials(r)
		if creds == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(creds)
	})

	cfg := &config.Reddit{
		ClientID:     "config-id",
		ClientSecret: "config-secret",
		Username:     "config-user",
		Password:     "config-pass",
		UserAgent:    "config-agent",
	}

	middleware := AuthFromConfig(cfg)(handler)

	req := httptest.NewRequest("GET", "/", nil)

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AuthFromConfig() status = %d, want %d", w.Code, http.StatusOK)
	}

	var creds Credentials
	if err := json.Unmarshal(w.Body.Bytes(), &creds); err != nil {
		t.Fatalf("failed to unmarshal credentials: %v", err)
	}

	if creds.ClientID != "config-id" {
		t.Errorf("ClientID = %s, want config-id", creds.ClientID)
	}
	if creds.ClientSecret != "config-secret" {
		t.Errorf("ClientSecret = %s, want config-secret", creds.ClientSecret)
	}
	if creds.Username != "config-user" {
		t.Errorf("Username = %s, want config-user", creds.Username)
	}
}

func TestRequireAPIKey_MissingAPIKey(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAPIKey([]string{"valid-key"})(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("RequireAPIKey() status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("RequireAPIKey() Content-Type = %s, want application/json", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "API key required") {
		t.Errorf("RequireAPIKey() body should contain error message, got %s", body)
	}
}

func TestRequireAPIKey_InvalidAPIKey(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAPIKey([]string{"valid-key"})(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("RequireAPIKey() status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("RequireAPIKey() Content-Type = %s, want application/json", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Invalid API key") {
		t.Errorf("RequireAPIKey() body should contain error message, got %s", body)
	}
}

func TestRequireAPIKey_ValidAPIKeyViaHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAPIKey([]string{"valid-key"})(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "valid-key")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequireAPIKey() status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAPIKey_ValidAPIKeyViaBearer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAPIKey([]string{"valid-key"})(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequireAPIKey() status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAPIKey_HeaderPreferredOverBearer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAPIKey([]string{"header-key"})(handler)

	// Set both headers, with valid key in X-API-Key and invalid in Bearer
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "header-key")
	req.Header.Set("Authorization", "Bearer invalid-key")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	// Should succeed because X-API-Key header is valid
	if w.Code != http.StatusOK {
		t.Errorf("RequireAPIKey() status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAPIKey_MultipleAPIKeys(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireAPIKey([]string{"key1", "key2", "key3"})(handler)

	tests := []struct {
		name   string
		key    string
		status int
	}{
		{"first key", "key1", http.StatusOK},
		{"second key", "key2", http.StatusOK},
		{"third key", "key3", http.StatusOK},
		{"invalid key", "key4", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-API-Key", tt.key)
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("RequireAPIKey() status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}
