package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_GET_Success(t *testing.T) {
	// Create handlers with nil client (health check doesn't use it)
	h := NewHandlers(nil, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Health(w, req)

	// Verify status code
	if w.Code != http.StatusOK {
		t.Errorf("Health() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Health() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body
	expectedBody := "{\"status\":\"ok\",\"service\":\"reddit-api-server\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Health() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestHealth_POST_MethodNotAllowed(t *testing.T) {
	// Create handlers with nil client
	h := NewHandlers(nil, nil)

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Health(w, req)

	// Verify status code
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Health() status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}

	// Verify Allow header
	allow := w.Header().Get("Allow")
	if allow != "GET" {
		t.Errorf("Health() Allow header = %q, want %q", allow, "GET")
	}

	// Verify error response
	expectedBody := "{\"error\":\"method not allowed\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Health() body = %q, want %q", w.Body.String(), expectedBody)
	}
}

func TestHealth_ResponseFormat(t *testing.T) {
	// Create handlers with nil client
	h := NewHandlers(nil, nil)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Health(w, req)

	// Verify the JSON structure is correct
	// We already tested the exact body in TestHealth_GET_Success,
	// but this test documents the expected structure
	if w.Code != http.StatusOK {
		t.Fatalf("Health() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Parse and verify structure
	type response struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}

	var resp response
	if err := parseJSON(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Health() failed to parse JSON response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Health() response.Status = %q, want %q", resp.Status, "ok")
	}

	if resp.Service != "reddit-api-server" {
		t.Errorf("Health() response.Service = %q, want %q", resp.Service, "reddit-api-server")
	}
}

func TestHealth_OtherMethods(t *testing.T) {
	// Test table for various HTTP methods that should be rejected
	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "PUT returns 405",
			method: http.MethodPut,
		},
		{
			name:   "DELETE returns 405",
			method: http.MethodDelete,
		},
		{
			name:   "PATCH returns 405",
			method: http.MethodPatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, nil)
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			h.Health(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Health() with %s status = %d, want %d", tt.method, w.Code, http.StatusMethodNotAllowed)
			}

			allow := w.Header().Get("Allow")
			if allow != "GET" {
				t.Errorf("Health() with %s Allow header = %q, want %q", tt.method, allow, "GET")
			}
		})
	}
}
