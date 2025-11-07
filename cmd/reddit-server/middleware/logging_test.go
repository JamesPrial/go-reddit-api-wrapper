package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogging_StatusCodeCapture(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
	}{
		{
			name: "200 OK",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "404 Not Found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("Not Found"))
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "500 Internal Server Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "default 200 when WriteHeader not called",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			handler := Logging(logger)(tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Parse log output
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output: %v", err)
			}

			// Verify status code in log
			status, ok := logEntry["status"].(float64)
			if !ok {
				t.Fatal("Log entry missing 'status' field")
			}
			if int(status) != tt.wantStatus {
				t.Errorf("Logged status = %d, want %d", int(status), tt.wantStatus)
			}

			// Verify actual response status
			if rec.Code != tt.wantStatus {
				t.Errorf("Response status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestLogging_ResponseSizeCapture(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.HandlerFunc
		wantSize int
	}{
		{
			name: "empty response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantSize: 0,
		},
		{
			name: "small response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hello"))
			},
			wantSize: 5,
		},
		{
			name: "multiple writes",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hello"))
				w.Write([]byte(" "))
				w.Write([]byte("World"))
			},
			wantSize: 11,
		},
		{
			name: "large response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				data := make([]byte, 1024)
				w.Write(data)
			},
			wantSize: 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			handler := Logging(logger)(tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Parse log output
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("Failed to parse log output: %v", err)
			}

			// Verify size in log
			size, ok := logEntry["size_bytes"].(float64)
			if !ok {
				t.Fatal("Log entry missing 'size_bytes' field")
			}
			if int(size) != tt.wantSize {
				t.Errorf("Logged size = %d, want %d", int(size), tt.wantSize)
			}

			// Verify actual response size
			if rec.Body.Len() != tt.wantSize {
				t.Errorf("Response size = %d, want %d", rec.Body.Len(), tt.wantSize)
			}
		})
	}
}

func TestLogging_MultipleWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.WriteHeader(http.StatusNotFound)            // Should be ignored
		w.WriteHeader(http.StatusInternalServerError) // Should be ignored
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify only first status code is captured
	status, ok := logEntry["status"].(float64)
	if !ok {
		t.Fatal("Log entry missing 'status' field")
	}
	if int(status) != http.StatusOK {
		t.Errorf("Logged status = %d, want %d (first WriteHeader call)", int(status), http.StatusOK)
	}

	// Verify response status
	if rec.Code != http.StatusOK {
		t.Errorf("Response status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLogging_FlushInterface(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	flushed := false
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test that Flush interface is available
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	}))

	// Use a ResponseWriter that supports Flusher
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !flushed {
		t.Error("Flush interface not propagated through responseWriter")
	}
}

func TestLogging_HijackInterface(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test that Hijack interface is available
		h, ok := w.(http.Hijacker)
		if !ok {
			t.Error("Hijacker interface not available")
			return
		}

		// Try to hijack (will fail with httptest.ResponseRecorder, but interface should exist)
		_, _, err := h.Hijack()
		if err == nil {
			t.Error("Expected hijack to fail with httptest.ResponseRecorder")
		}
		if err.Error() != "hijack not supported" {
			t.Errorf("Hijack error = %v, want 'hijack not supported'", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestLogging_PushInterface(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test that Pusher interface is available
		p, ok := w.(http.Pusher)
		if !ok {
			t.Error("Pusher interface not available")
			return
		}

		// Try to push (will fail with httptest.ResponseRecorder, but interface should exist)
		err := p.Push("/static/style.css", nil)
		if err == nil {
			t.Error("Expected push to fail with httptest.ResponseRecorder")
		}
		if err.Error() != "push not supported" {
			t.Errorf("Push error = %v, want 'push not supported'", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
}

func TestLogging_UnwrapInterface(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	var originalWriter http.ResponseWriter
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Test that Unwrap returns the original ResponseWriter
		if unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
			originalWriter = unwrapper.Unwrap()
		} else {
			t.Error("Unwrap method not available")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if originalWriter != rec {
		t.Error("Unwrap did not return the original ResponseWriter")
	}
}

func TestLogging_AllLogFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Created"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify all expected fields are present
	expectedFields := []string{"method", "path", "status", "duration_ms", "size_bytes", "remote_addr", "msg"}
	for _, field := range expectedFields {
		if _, ok := logEntry[field]; !ok {
			t.Errorf("Log entry missing field %q", field)
		}
	}

	// Verify field values
	if logEntry["method"] != "POST" {
		t.Errorf("method = %v, want POST", logEntry["method"])
	}
	if logEntry["path"] != "/api/users" {
		t.Errorf("path = %v, want /api/users", logEntry["path"])
	}
	if int(logEntry["status"].(float64)) != http.StatusCreated {
		t.Errorf("status = %v, want %d", logEntry["status"], http.StatusCreated)
	}
	if int(logEntry["size_bytes"].(float64)) != 7 {
		t.Errorf("size_bytes = %v, want 7", logEntry["size_bytes"])
	}
	if logEntry["remote_addr"] != "192.168.1.1:12345" {
		t.Errorf("remote_addr = %v, want 192.168.1.1:12345", logEntry["remote_addr"])
	}
	if logEntry["msg"] != "HTTP request" {
		t.Errorf("msg = %v, want 'HTTP request'", logEntry["msg"])
	}

	// Verify duration_ms is a reasonable value (should be >= 0)
	durationMs, ok := logEntry["duration_ms"].(float64)
	if !ok {
		t.Fatal("duration_ms is not a number")
	}
	if durationMs < 0 {
		t.Errorf("duration_ms = %v, want >= 0", durationMs)
	}
}

// mockHijacker is a mock http.ResponseWriter that implements http.Hijacker.
type mockHijacker struct {
	httptest.ResponseRecorder
}

func (m *mockHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("mock hijack")
}

func TestLogging_HijackInterfacePropagation(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	hijackCalled := false
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, ok := w.(http.Hijacker)
		if !ok {
			t.Error("Hijacker interface not available")
			return
		}

		_, _, err := h.Hijack()
		if err == nil {
			t.Error("Expected hijack to return error")
		}
		if err.Error() == "mock hijack" {
			hijackCalled = true
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := &mockHijacker{}

	handler.ServeHTTP(rec, req)

	if !hijackCalled {
		t.Error("Hijack was not called on underlying ResponseWriter")
	}
}

// mockPusher is a mock http.ResponseWriter that implements http.Pusher.
type mockPusher struct {
	httptest.ResponseRecorder
	pushCalled bool
}

func (m *mockPusher) Push(target string, opts *http.PushOptions) error {
	m.pushCalled = true
	return errors.New("mock push")
}

func TestLogging_PushInterfacePropagation(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	rec := &mockPusher{}
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := w.(http.Pusher)
		if !ok {
			t.Error("Pusher interface not available")
			return
		}

		err := p.Push("/static/style.css", nil)
		if err == nil {
			t.Error("Expected push to return error")
		}
		if err.Error() != "mock push" {
			t.Errorf("Push error = %v, want 'mock push'", err)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler.ServeHTTP(rec, req)

	if !rec.pushCalled {
		t.Error("Push was not called on underlying ResponseWriter")
	}
}

// discardWriter is a simple ResponseWriter that discards all writes.
type discardWriter struct {
	header http.Header
}

func (d *discardWriter) Header() http.Header {
	if d.header == nil {
		d.header = make(http.Header)
	}
	return d.header
}

func (d *discardWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (d *discardWriter) WriteHeader(statusCode int) {}

func TestLogging_LargeResponse(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create a large response (10 MB)
	largeData := make([]byte, 10*1024*1024)
	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(largeData)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify size is correct
	size, ok := logEntry["size_bytes"].(float64)
	if !ok {
		t.Fatal("Log entry missing 'size_bytes' field")
	}
	if int(size) != len(largeData) {
		t.Errorf("Logged size = %d, want %d", int(size), len(largeData))
	}
}

func TestLogging_WriteOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only call Write, not WriteHeader
		io.WriteString(w, "test response")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify status defaults to 200
	status, ok := logEntry["status"].(float64)
	if !ok {
		t.Fatal("Log entry missing 'status' field")
	}
	if int(status) != http.StatusOK {
		t.Errorf("Logged status = %d, want %d (default)", int(status), http.StatusOK)
	}

	// Verify size is correct
	size, ok := logEntry["size_bytes"].(float64)
	if !ok {
		t.Fatal("Log entry missing 'size_bytes' field")
	}
	if int(size) != len("test response") {
		t.Errorf("Logged size = %d, want %d", int(size), len("test response"))
	}
}
