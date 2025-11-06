package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRecovery_HandlerPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic, should return 500
	recoveryHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Recovery() status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Recovery() Content-Type = %s, want application/json", ct)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if !strings.Contains(w.Body.String(), "Internal server error") {
		t.Errorf("Recovery() response should contain error message")
	}
	if !strings.Contains(w.Body.String(), "server_error") {
		t.Errorf("Recovery() response should contain server_error type")
	}
}

func TestRecovery_NormalHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"success"}`))
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Recovery() status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != `{"message":"success"}` {
		t.Errorf("Recovery() body = %s, want {\"message\":\"success\"}", body)
	}
}

func TestRecovery_PanicWithValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic with message")
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Recovery() status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Errorf("Recovery() should log panic recovery")
	}
	if !strings.Contains(logOutput, "GET") {
		t.Errorf("Recovery() should log request method")
	}
	if !strings.Contains(logOutput, "/api/test") {
		t.Errorf("Recovery() should log request path")
	}
	if !strings.Contains(logOutput, "stack") {
		t.Errorf("Recovery() should log stack trace")
	}
}

func TestRecovery_PanicWithCustomError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	type CustomError struct {
		Message string
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(CustomError{Message: "custom error"})
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Recovery() status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if !strings.Contains(w.Body.String(), "server_error") {
		t.Errorf("Recovery() should contain server_error type")
	}
}

func TestRecovery_DifferentHTTPMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "GET request panic",
			method: "GET",
		},
		{
			name:   "POST request panic",
			method: "POST",
		},
		{
			name:   "PUT request panic",
			method: "PUT",
		},
		{
			name:   "DELETE request panic",
			method: "DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			})

			middleware := Recovery(logger)
			recoveryHandler := middleware(handler)

			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			recoveryHandler.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("Recovery() status = %d, want %d", w.Code, http.StatusInternalServerError)
			}

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.method) {
				t.Errorf("Recovery() should log method %s", tt.method)
			}
		})
	}
}

func TestRecovery_StackTraceIncluded(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "recovery_test.go") && !strings.Contains(logOutput, "goroutine") {
		t.Logf("Recovery log output: %s", logOutput)
		t.Errorf("Recovery() should log stack trace with file information")
	}
}

func TestRecovery_MultipleRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("panic")
	})

	normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	middleware := Recovery(logger)

	tests := []struct {
		name     string
		handler  http.Handler
		wantCode int
	}{
		{
			name:     "first panic",
			handler:  middleware(panicHandler),
			wantCode: http.StatusInternalServerError,
		},
		{
			name:     "normal request",
			handler:  middleware(normalHandler),
			wantCode: http.StatusOK,
		},
		{
			name:     "second panic",
			handler:  middleware(panicHandler),
			wantCode: http.StatusInternalServerError,
		},
		{
			name:     "another normal request",
			handler:  middleware(normalHandler),
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			tt.handler.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("Recovery() status = %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestRecovery_ResponseNotWritten(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Panic before writing response
		panic("panic before response")
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Recovery() status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if !strings.Contains(w.Body.String(), "500") {
		t.Errorf("Recovery() response should contain error code 500")
	}
}

func TestRecovery_ResponseAlreadyWritten(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial response"))
		panic("panic after write")
	})

	middleware := Recovery(logger)
	recoveryHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(w, req)

	// Status code should still be 200 since it was already written
	if w.Code != http.StatusOK {
		t.Errorf("Recovery() status = %d, want %d (already written)", w.Code, http.StatusOK)
	}

	// Response body should contain the partial response and error message
	body := w.Body.String()
	if !strings.Contains(body, "partial response") {
		t.Errorf("Recovery() should preserve partial response body")
	}
}
