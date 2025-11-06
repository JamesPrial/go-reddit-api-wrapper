package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
