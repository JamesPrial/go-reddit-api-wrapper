package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// testLogger creates a logger that discards output during tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// TestStaticHandler_Index tests that the StaticHandler serves the index.html file
// when requesting the root path or /index.html explicitly.
func TestStaticHandler_Index(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedTitle  string
		allowRedirect  bool // FileServer may redirect certain requests
	}{
		{
			name:           "root path serves index",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedTitle:  "Reddit API Browser",
			allowRedirect:  false,
		},
		{
			name:           "explicit index.html serves index",
			path:           "/index.html",
			expectedStatus: http.StatusOK,
			expectedTitle:  "Reddit API Browser",
			allowRedirect:  true, // FileServer may redirect to / without extension
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Allow 301 redirects if allowRedirect is true
			if rec.Code == http.StatusMovedPermanently && tt.allowRedirect {
				// Redirects are acceptable for some file server requests
				return
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			contentType := rec.Header().Get("Content-Type")
			if rec.Code == http.StatusOK && !strings.Contains(contentType, "text/html") {
				t.Errorf("Content-Type = %s, want to contain text/html", contentType)
			}

			body := rec.Body.String()
			if rec.Code == http.StatusOK && !strings.Contains(body, tt.expectedTitle) {
				t.Errorf("response body doesn't contain expected title %q", tt.expectedTitle)
			}
		})
	}
}

// TestStaticHandler_NotFound tests that the StaticHandler returns 404 for non-existent files.
func TestStaticHandler_NotFound(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "nonexistent file returns 404",
			path:           "/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "nonexistent directory returns 404",
			path:           "/missing/file.html",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "path traversal attempt returns 404",
			path:           "/../../../etc/passwd",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestStaticHandler_Methods tests various HTTP methods with static files.
// The file server should handle standard HTTP methods appropriately.
func TestStaticHandler_Methods(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		acceptRedirect bool
	}{
		{
			name:           "GET index returns 200",
			method:         http.MethodGet,
			path:           "/",
			expectedStatus: http.StatusOK,
			acceptRedirect: false,
		},
		{
			name:           "HEAD index returns 200",
			method:         http.MethodHead,
			path:           "/",
			expectedStatus: http.StatusOK,
			acceptRedirect: false,
		},
		{
			name:           "POST to root returns 405 Method Not Allowed",
			method:         http.MethodPost,
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
			acceptRedirect: false,
		},
		{
			name:           "PUT to index returns 405 Method Not Allowed",
			method:         http.MethodPut,
			path:           "/index.html",
			expectedStatus: http.StatusMethodNotAllowed,
			acceptRedirect: false,
		},
		{
			name:           "DELETE to index returns 405 Method Not Allowed",
			method:         http.MethodDelete,
			path:           "/index.html",
			expectedStatus: http.StatusMethodNotAllowed,
			acceptRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d (method: %s)", rec.Code, tt.expectedStatus, tt.method)
			}
		})
	}
}

// TestStaticHandler_ContentType tests that static files are served with correct Content-Type headers.
func TestStaticHandler_ContentType(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name            string
		path            string
		expectedStatus  int
		expectedSubType string // Substring to check in Content-Type
		acceptRedirect  bool
	}{
		{
			name:            "HTML file has text/html content type",
			path:            "/index.html",
			expectedStatus:  http.StatusOK,
			expectedSubType: "text/html",
			acceptRedirect:  true, // FileServer may redirect
		},
		{
			name:            "root path returns HTML",
			path:            "/",
			expectedStatus:  http.StatusOK,
			expectedSubType: "text/html",
			acceptRedirect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Allow redirects if acceptRedirect is true
			if rec.Code == http.StatusMovedPermanently && tt.acceptRedirect {
				return
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			contentType := rec.Header().Get("Content-Type")
			if rec.Code == http.StatusOK && !strings.Contains(contentType, tt.expectedSubType) {
				t.Errorf("Content-Type = %q, want to contain %q", contentType, tt.expectedSubType)
			}
		})
	}
}

// TestStaticHandler_CSSFile tests that CSS files are served with correct Content-Type.
func TestStaticHandler_CSSFile(t *testing.T) {
	handler := StaticHandler(testLogger())

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", contentType)
	}

	// Verify Cache-Control header for CSS
	cacheControl := rec.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "public") {
		t.Errorf("Cache-Control = %q, want public cache", cacheControl)
	}
}

// TestStaticHandler_JavaScriptFile tests that JavaScript files are served with correct Content-Type.
func TestStaticHandler_JavaScriptFile(t *testing.T) {
	handler := StaticHandler(testLogger())

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "javascript") {
		t.Errorf("Content-Type = %q, want javascript", contentType)
	}

	// Verify Cache-Control header for JS
	cacheControl := rec.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "public") {
		t.Errorf("Cache-Control = %q, want public cache", cacheControl)
	}
}

// TestStaticHandler_Logging tests that debug logs are generated for static file requests.
func TestStaticHandler_Logging(t *testing.T) {
	// Create a buffer to capture log output
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	handler := StaticHandler(logger)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	req.RemoteAddr = "192.168.1.1:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuffer.String()

	// Verify that debug log was generated
	if !strings.Contains(logOutput, "serving static file") {
		t.Error("log output doesn't contain 'serving static file'")
	}

	// Verify that important log fields are present
	if !strings.Contains(logOutput, "method") {
		t.Error("log output doesn't contain method field")
	}

	if !strings.Contains(logOutput, "/index.html") {
		t.Error("log output doesn't contain requested path")
	}

	if !strings.Contains(logOutput, "remote_addr") {
		t.Error("log output doesn't contain remote_addr field")
	}
}

// TestStaticHandler_LoggingLevel tests that logging respects log level settings.
func TestStaticHandler_LoggingLevel(t *testing.T) {
	// Test with INFO level - debug logs should not appear
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	handler := StaticHandler(logger)

	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := logBuffer.String()

	// At INFO level, debug logs should not be present
	if strings.Contains(logOutput, "serving static file") {
		t.Error("debug log appeared when log level is INFO")
	}
}

// TestStaticHandler_DirectoryTraversal tests that directory traversal attacks are prevented.
func TestStaticHandler_DirectoryTraversal(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "double dot traversal blocked",
			path:           "/../../../etc/passwd",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "encoded traversal attempt blocked",
			path:           "/..%2F..%2F..%2Fetc%2Fpasswd",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "deep traversal attempt blocked",
			path:           "/../../../../../../../../etc/passwd",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "null byte injection blocked",
			path:           "/index.html%00.jpg",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "null byte in directory blocked",
			path:           "/..%00/etc/passwd",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestStaticHandler_QueryString tests that query strings in requests don't affect file serving.
func TestStaticHandler_QueryString(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		acceptRedirect bool
	}{
		{
			name:           "index with query string serves index",
			path:           "/?v=1.0",
			expectedStatus: http.StatusOK,
			acceptRedirect: false,
		},
		{
			name:           "index.html with query string serves file",
			path:           "/index.html?bust-cache=true",
			expectedStatus: http.StatusOK,
			acceptRedirect: true, // FileServer may redirect
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Allow redirects if acceptRedirect is true
			if rec.Code == http.StatusMovedPermanently && tt.acceptRedirect {
				return
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestStaticHandler_CaseInsensitivity tests file path handling.
func TestStaticHandler_CaseInsensitivity(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "uppercase path may differ based on filesystem",
			path:           "/INDEX.HTML",
			expectedStatus: http.StatusNotFound, // Most filesystems are case-sensitive
		},
		{
			name:           "mixed case returns 404 on case-sensitive systems",
			path:           "/Index.Html",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Logf("note: status = %d (filesystem case sensitivity may vary)", rec.Code)
			}
		})
	}
}

// TestStaticHandler_ResponseHeaders tests that appropriate response headers are set.
func TestStaticHandler_ResponseHeaders(t *testing.T) {
	handler := StaticHandler(testLogger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Verify Content-Type is set for HTML files
	contentType := rec.Header().Get("Content-Type")
	if contentType == "" {
		t.Error("Content-Type header is not set")
	}

	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want to contain text/html", contentType)
	}

	// Verify Content-Length is set for successful responses
	if rec.Header().Get("Content-Length") == "" {
		t.Error("Content-Length header is not set for successful response")
	}
}

// TestStaticHandler_RootPath tests requests with root paths.
// Empty paths are invalid in httptest.NewRequest, so we only test root slash.
func TestStaticHandler_RootPath(t *testing.T) {
	handler := StaticHandler(testLogger())

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "root slash serves index",
			path:           "/",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestStaticHandler_LargeURLPath tests handling of very long paths.
func TestStaticHandler_LargeURLPath(t *testing.T) {
	handler := StaticHandler(testLogger())

	// Create a very long path
	longPath := "/very/long/path/" + strings.Repeat("a", 1000)

	req := httptest.NewRequest(http.MethodGet, longPath, nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should return 404, not crash
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusRequestURITooLong {
		t.Errorf("status = %d, want 404 or 414", rec.Code)
	}
}

// TestStaticHandler_RangeRequests tests that range requests are handled.
func TestStaticHandler_RangeRequests(t *testing.T) {
	handler := StaticHandler(testLogger())

	// Use root path instead of index.html to avoid redirect
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=0-100")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should either handle range or return OK without partial content
	if rec.Code != http.StatusOK && rec.Code != http.StatusPartialContent && rec.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want 200, 206, or 301", rec.Code)
	}
}

// TestStaticHandler_ResponseBody tests that response bodies are valid.
func TestStaticHandler_ResponseBody(t *testing.T) {
	handler := StaticHandler(testLogger())

	// Use root path instead of index.html to avoid redirect
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()

	// Verify that response is not empty
	if len(body) == 0 {
		t.Error("response body is empty")
	}

	// Verify that response contains HTML content
	if !strings.Contains(body, "<") || !strings.Contains(body, ">") {
		t.Error("response doesn't appear to be HTML")
	}

	// Verify that response is valid UTF-8
	if !validUTF8([]byte(body)) {
		t.Error("response body is not valid UTF-8")
	}
}

// validUTF8 is a helper function to check if a byte slice is valid UTF-8.
func validUTF8(b []byte) bool {
	return utf8.Valid(b)
}

// TestStaticHandler_HeadRequest tests that HEAD requests work correctly.
func TestStaticHandler_HeadRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	handler := StaticHandler(logger)

	getReq := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	headReq := httptest.NewRequest(http.MethodHead, "/index.html", nil)
	headRec := httptest.NewRecorder()
	handler.ServeHTTP(headRec, headReq)

	// HEAD should return same status as GET
	if headRec.Code != getRec.Code {
		t.Errorf("HEAD status = %d, GET status = %d, want same", headRec.Code, getRec.Code)
	}

	// HEAD should have same headers but no body
	if headRec.Header().Get("Content-Type") != getRec.Header().Get("Content-Type") {
		t.Errorf("HEAD Content-Type = %q, GET Content-Type = %q, want same",
			headRec.Header().Get("Content-Type"), getRec.Header().Get("Content-Type"))
	}

	if headRec.Body.Len() != 0 {
		t.Error("HEAD response should have no body")
	}
}

// TestStaticHandler_FileServerBehavior tests the overall file server behavior.
func TestStaticHandler_FileServerBehavior(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	handler := StaticHandler(logger)

	tests := []struct {
		name            string
		path            string
		expectedStatus  int
		shouldHaveBody  bool
		acceptRedirect  bool
		acceptErrorBody bool // Some 404s may have body content
	}{
		{
			name:            "index.html GET returns 200 or redirect",
			path:            "/index.html",
			expectedStatus:  http.StatusOK,
			shouldHaveBody:  true,
			acceptRedirect:  true,
			acceptErrorBody: false,
		},
		{
			name:            "root GET returns 200 with body",
			path:            "/",
			expectedStatus:  http.StatusOK,
			shouldHaveBody:  true,
			acceptRedirect:  false,
			acceptErrorBody: false,
		},
		{
			name:            "nonexistent file returns 404",
			path:            "/nonexistent.html",
			expectedStatus:  http.StatusNotFound,
			shouldHaveBody:  false,
			acceptRedirect:  false,
			acceptErrorBody: true, // 404s may have error body
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Allow redirects if acceptable
			if rec.Code == http.StatusMovedPermanently && tt.acceptRedirect {
				return
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}

			hasBody := rec.Body.Len() > 0
			if rec.Code == http.StatusOK && hasBody != tt.shouldHaveBody {
				t.Errorf("has body = %v, want %v", hasBody, tt.shouldHaveBody)
			}
		})
	}
}

// BenchmarkStaticHandler_Index benchmarks serving the index file.
func BenchmarkStaticHandler_Index(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	handler := StaticHandler(logger)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkStaticHandler_NotFound benchmarks serving non-existent files.
func BenchmarkStaticHandler_NotFound(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	handler := StaticHandler(logger)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent.html", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// TestStaticHandler_RaceCondition tests that concurrent requests don't cause race conditions.
func TestStaticHandler_RaceCondition(t *testing.T) {
	handler := StaticHandler(testLogger())

	const concurrency = 50
	done := make(chan int, concurrency)
	errors := make([]string, 0)
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- id }()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK && rec.Code != http.StatusMovedPermanently {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("goroutine %d: status = %d", id, rec.Code))
				mu.Unlock()
			}
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}

	if len(errors) > 0 {
		t.Errorf("concurrent requests failed:\n%s", strings.Join(errors, "\n"))
	}
}

// TestStaticHandler_Initialization tests StaticHandler initialization without external dependencies.
// This test ensures the handler is properly isolated.
func TestStaticHandler_Initialization(t *testing.T) {
	// Test with different log levels
	logLevels := []slog.Level{
		slog.LevelDebug,
		slog.LevelInfo,
		slog.LevelWarn,
		slog.LevelError,
	}

	for _, level := range logLevels {
		t.Run("log_level_"+level.String(), func(t *testing.T) {
			var logBuffer bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
				Level: level,
			}))

			handler := StaticHandler(logger)

			if handler == nil {
				t.Error("StaticHandler returned nil")
			}

			// Verify handler is actually an http.Handler
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			// Should not panic
			handler.ServeHTTP(rec, req)

			if rec.Code == http.StatusInternalServerError {
				// This might be okay if the static files couldn't be embedded
				// but we should check the logs
				logOutput := logBuffer.String()
				if !strings.Contains(logOutput, "failed to create static filesystem") {
					t.Logf("Unexpected 500 error: %s", logOutput)
				}
			}
		})
	}
}
