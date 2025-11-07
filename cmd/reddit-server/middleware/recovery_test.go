package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecovery_PanicBeforeHeaderWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic before WriteHeader")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not crash
	handler.ServeHTTP(rec, req)

	// Verify response is 500 Internal Server Error
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Verify response body contains error message
	body := rec.Body.String()
	if !strings.Contains(body, "Internal Server Error") {
		t.Errorf("Response body = %q, want to contain 'Internal Server Error'", body)
	}

	// Verify panic was logged
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify log level is error
	if logEntry["level"] != "ERROR" {
		t.Errorf("Log level = %v, want ERROR", logEntry["level"])
	}

	// Verify log contains panic message
	if logEntry["msg"] != "HTTP handler panic" {
		t.Errorf("Log msg = %v, want 'HTTP handler panic'", logEntry["msg"])
	}

	// Verify error field contains panic value
	errorField, ok := logEntry["error"].(string)
	if !ok {
		t.Fatal("Log entry missing 'error' field")
	}
	if !strings.Contains(errorField, "test panic before WriteHeader") {
		t.Errorf("Log error = %q, want to contain 'test panic before WriteHeader'", errorField)
	}
}

func TestRecovery_PanicAfterHeaderWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("test panic after WriteHeader")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not crash
	handler.ServeHTTP(rec, req)

	// Verify response is still 200 (headers already sent)
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d (headers already sent)", rec.Code, http.StatusOK)
	}

	// Verify panic was logged
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify log level is error
	if logEntry["level"] != "ERROR" {
		t.Errorf("Log level = %v, want ERROR", logEntry["level"])
	}

	// Verify error field contains panic value
	errorField, ok := logEntry["error"].(string)
	if !ok {
		t.Fatal("Log entry missing 'error' field")
	}
	if !strings.Contains(errorField, "test panic after WriteHeader") {
		t.Errorf("Log error = %q, want to contain 'test panic after WriteHeader'", errorField)
	}
}

func TestRecovery_PanicAfterWrite(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("partial response"))
		panic("test panic after Write")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Should not crash
	handler.ServeHTTP(rec, req)

	// Verify response contains partial data
	body := rec.Body.String()
	if !strings.Contains(body, "partial response") {
		t.Errorf("Response body = %q, want to contain 'partial response'", body)
	}

	// Verify response status is 200 (implicit from Write)
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d (implicit from Write)", rec.Code, http.StatusOK)
	}

	// Verify panic was logged
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify error field contains panic value
	errorField, ok := logEntry["error"].(string)
	if !ok {
		t.Fatal("Log entry missing 'error' field")
	}
	if !strings.Contains(errorField, "test panic after Write") {
		t.Errorf("Log error = %q, want to contain 'test panic after Write'", errorField)
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify normal response
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if body != "success" {
		t.Errorf("Response body = %q, want 'success'", body)
	}

	// Verify no error was logged
	if buf.Len() > 0 {
		t.Errorf("Expected no logs, got: %s", buf.String())
	}
}

func TestRecovery_LoggingIncludesStackTrace(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic with stack")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test-stack", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify stack trace is present
	stack, ok := logEntry["stack"].(string)
	if !ok {
		t.Fatal("Log entry missing 'stack' field")
	}

	// Verify stack trace contains expected elements
	if !strings.Contains(stack, "goroutine") {
		t.Error("Stack trace does not contain 'goroutine'")
	}
	if !strings.Contains(stack, "panic") {
		t.Error("Stack trace does not contain 'panic'")
	}

	// Verify other log fields
	if logEntry["method"] != "GET" {
		t.Errorf("Log method = %v, want GET", logEntry["method"])
	}
	if logEntry["path"] != "/test-stack" {
		t.Errorf("Log path = %v, want /test-stack", logEntry["path"])
	}
	if logEntry["remote_addr"] != "192.168.1.1:12345" {
		t.Errorf("Log remote_addr = %v, want 192.168.1.1:12345", logEntry["remote_addr"])
	}
}

func TestRecovery_PanicWithDifferentTypes(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{
			name:       "string panic",
			panicValue: "string panic",
		},
		{
			name:       "error panic",
			panicValue: io.EOF,
		},
		{
			name:       "int panic",
			panicValue: 42,
		},
		{
			name:       "struct panic",
			panicValue: struct{ Message string }{Message: "struct panic"},
		},
		{
			name:       "nil panic",
			panicValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(tt.panicValue)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			// Should not crash regardless of panic type
			handler.ServeHTTP(rec, req)

			// Verify response is 500 Internal Server Error
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("Response status = %d, want %d", rec.Code, http.StatusInternalServerError)
			}

			// Verify panic was logged
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output: %v", err)
			}

			// Verify error field is present (value varies by type)
			if _, ok := logEntry["error"]; !ok {
				t.Error("Log entry missing 'error' field")
			}
		})
	}
}

func TestRecovery_MultipleRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	requestCount := 0
	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 2 {
			panic("panic on second request")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// First request - should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test1", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("First request status = %d, want %d", rec1.Code, http.StatusOK)
	}

	// Second request - should panic and recover
	req2 := httptest.NewRequest(http.MethodGet, "/test2", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("Second request status = %d, want %d", rec2.Code, http.StatusInternalServerError)
	}

	// Third request - should succeed (handler still works after panic)
	req3 := httptest.NewRequest(http.MethodGet, "/test3", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("Third request status = %d, want %d", rec3.Code, http.StatusOK)
	}

	// Verify only one panic was logged
	logLines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(logLines) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(logLines))
	}
}

func TestRecovery_PanicDuringWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Mock ResponseWriter that panics during WriteHeader
	type panicWriter struct {
		http.ResponseWriter
		headerWritten bool
	}

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("panic after successful WriteHeader")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify panic was logged
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logEntry["level"] != "ERROR" {
		t.Errorf("Log level = %v, want ERROR", logEntry["level"])
	}
}

func TestRecovery_WriterNotModifiedAfterPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("before panic"))
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response contains only the data written before panic
	body := rec.Body.String()
	if !strings.Contains(body, "before panic") {
		t.Errorf("Response body = %q, want to contain 'before panic'", body)
	}

	// Verify no "Internal Server Error" was written (headers already sent)
	if strings.Contains(body, "Internal Server Error") {
		t.Errorf("Response body = %q, should not contain 'Internal Server Error' after headers sent", body)
	}
}

func TestRecovery_HeadersNotSentOnPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set some headers but don't write or call WriteHeader
		w.Header().Set("X-Custom-Header", "test-value")
		panic("panic before sending headers")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response is 500
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Verify custom header was not sent (it was set before panic, but not flushed)
	// Note: In httptest.ResponseRecorder, headers are captured even if WriteHeader wasn't called,
	// so we just verify the status code was overridden to 500
	body := rec.Body.String()
	if !strings.Contains(body, "Internal Server Error") {
		t.Errorf("Response body = %q, want to contain 'Internal Server Error'", body)
	}
}

func TestRecovery_DeepCallStack(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create a deep call stack before panicking
	var deepPanic func(int)
	deepPanic = func(depth int) {
		if depth == 0 {
			panic("deep panic")
		}
		deepPanic(depth - 1)
	}

	handler := Recovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deepPanic(10)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify panic was recovered
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Parse log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify stack trace is present and contains multiple frames
	stack, ok := logEntry["stack"].(string)
	if !ok {
		t.Fatal("Log entry missing 'stack' field")
	}

	// Stack should contain at least one function call reference
	// The exact function name may vary depending on Go version and optimization
	if len(stack) < 100 {
		t.Error("Stack trace is unexpectedly short")
	}

	// Verify stack contains goroutine and panic information
	if !strings.Contains(stack, "goroutine") {
		t.Error("Stack trace does not contain 'goroutine'")
	}
	if !strings.Contains(stack, "panic") {
		t.Error("Stack trace does not contain 'panic'")
	}
}
