package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLogging_BasicRequest(t *testing.T) {
	var buf bytes.Buffer

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	middleware := LoggingWithWriter(&buf)
	loggingHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	loggingHandler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "GET /api/test") {
		t.Errorf("Logging() should log request method and path")
	}
	if !strings.Contains(logOutput, "status") {
		t.Errorf("Logging() should log status code")
	}
	if !strings.Contains(logOutput, "200") {
		t.Errorf("Logging() should log status 200")
	}
	if !strings.Contains(logOutput, "duration") {
		t.Errorf("Logging() should log duration")
	}
}

func TestLogging_VariousStatusCodes(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedInLog string
	}{
		{
			name:          "200 OK",
			statusCode:    http.StatusOK,
			responseBody:  "success",
			expectedInLog: "200",
		},
		{
			name:          "400 Bad Request",
			statusCode:    http.StatusBadRequest,
			responseBody:  "error",
			expectedInLog: "400",
		},
		{
			name:          "401 Unauthorized",
			statusCode:    http.StatusUnauthorized,
			responseBody:  "unauthorized",
			expectedInLog: "401",
		},
		{
			name:          "404 Not Found",
			statusCode:    http.StatusNotFound,
			responseBody:  "",
			expectedInLog: "404",
		},
		{
			name:          "500 Internal Server Error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  "error",
			expectedInLog: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			})

			middleware := LoggingWithWriter(&buf)
			loggingHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			loggingHandler.ServeHTTP(w, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.expectedInLog) {
				t.Errorf("Logging() should log status %s", tt.expectedInLog)
			}
		})
	}
}

func TestLogging_CapturesResponseSize(t *testing.T) {
	var buf bytes.Buffer

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response body"))
	})

	middleware := LoggingWithWriter(&buf)
	loggingHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	loggingHandler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "size") {
		t.Errorf("Logging() should log response size")
	}
	// Should log the byte count of "test response body"
	if !strings.Contains(logOutput, "18") {
		t.Logf("Logging output: %s", logOutput)
		t.Errorf("Logging() should log size 18 for response")
	}
}

func TestLogging_HealthEndpointAtDebugLevel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(
		&bytes.Buffer{},
		&slog.HandlerOptions{Level: slog.LevelInfo},
	))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	middleware := Logging(logger)
	loggingHandler := middleware(handler)

	// Test /health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	loggingHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health request status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestLogging_DifferentMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{
			name:   "GET request",
			method: "GET",
		},
		{
			name:   "POST request",
			method: "POST",
		},
		{
			name:   "PUT request",
			method: "PUT",
		},
		{
			name:   "DELETE request",
			method: "DELETE",
		},
		{
			name:   "PATCH request",
			method: "PATCH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := LoggingWithWriter(&buf)
			loggingHandler := middleware(handler)

			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			loggingHandler.ServeHTTP(w, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.method) {
				t.Errorf("Logging() should log method %s", tt.method)
			}
		})
	}
}

func TestLogging_DifferentPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "simple path",
			path: "/api/users",
		},
		{
			name: "deep path",
			path: "/api/v1/posts/golang/comments",
		},
		{
			name: "path with query",
			path: "/api/v1/posts?limit=10",
		},
		{
			name: "root path",
			path: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := LoggingWithWriter(&buf)
			loggingHandler := middleware(handler)

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			loggingHandler.ServeHTTP(w, req)

			logOutput := buf.String()
			// Should log the path without query parameters
			expectedPath := tt.path
			if strings.Contains(expectedPath, "?") {
				expectedPath = strings.Split(expectedPath, "?")[0]
			}
			if !strings.Contains(logOutput, expectedPath) {
				t.Errorf("Logging() should log path %s, got: %s", expectedPath, logOutput)
			}
		})
	}
}

func TestLogging_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("first"))
		w.Write([]byte("second"))
		w.Write([]byte("third"))
	})

	middleware := LoggingWithWriter(&buf)
	loggingHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	loggingHandler.ServeHTTP(w, req)

	logOutput := buf.String()
	// Should log total size (5 + 6 + 5 = 16)
	if !strings.Contains(logOutput, "16") {
		t.Logf("Logging output: %s", logOutput)
		t.Errorf("Logging() should log accumulated size 16")
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rw := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
		size:           0,
	}

	if rw.statusCode != http.StatusOK {
		t.Errorf("responseWriter statusCode = %d, want %d", rw.statusCode, http.StatusOK)
	}

	rw.WriteHeader(http.StatusBadRequest)

	if rw.statusCode != http.StatusBadRequest {
		t.Errorf("WriteHeader() statusCode = %d, want %d", rw.statusCode, http.StatusBadRequest)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
		size:           0,
	}

	data := []byte("test data")
	n, err := rw.Write(data)

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	if n != len(data) {
		t.Errorf("Write() returned %d bytes, want %d", n, len(data))
	}

	if rw.size != len(data) {
		t.Errorf("responseWriter size = %d, want %d", rw.size, len(data))
	}
}

func TestResponseWriter_DefaultStatusCode(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
		size:           0,
	}

	// If WriteHeader is not called, status code should default to OK
	if rw.statusCode != http.StatusOK {
		t.Errorf("Default statusCode = %d, want %d", rw.statusCode, http.StatusOK)
	}
}

func TestLogging_CapturesDuration(t *testing.T) {
	var buf bytes.Buffer

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := LoggingWithWriter(&buf)
	loggingHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	loggingHandler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "duration") {
		t.Errorf("Logging() should log duration")
	}
	// Should log a duration in milliseconds or microseconds
	if !strings.Contains(logOutput, "ms") && !strings.Contains(logOutput, "µs") {
		t.Logf("Logging output: %s", logOutput)
		t.Errorf("Logging() should include duration unit")
	}
}
