package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health() status = %d, want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Health() Content-Type = %s, want application/json", ct)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Health() status = %s, want ok", resp.Status)
	}

	if resp.Version != "1.0" {
		t.Errorf("Health() version = %s, want 1.0", resp.Version)
	}

	// Check that timestamp is recent (within last second)
	now := time.Now().UTC()
	diff := now.Sub(resp.Timestamp)
	if diff < 0 || diff > time.Second {
		t.Errorf("Health() timestamp = %v, want within last second", resp.Timestamp)
	}
}

func TestHealthMultipleCalls(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	handler := New(logger, nil)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Health() call %d status = %d, want %d", i, w.Code, http.StatusOK)
		}

		var resp HealthResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Health() call %d failed to unmarshal: %v", i, err)
		}

		if resp.Status != "ok" {
			t.Errorf("Health() call %d status = %s, want ok", i, resp.Status)
		}
	}
}
