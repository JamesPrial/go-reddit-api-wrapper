package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShutdown_POST_Success(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers with shutdown channel
	h := NewHandlers(nil, nil, shutdownCh)

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Shutdown(w, req)

	// Verify status code (202 Accepted)
	if w.Code != http.StatusAccepted {
		t.Errorf("Shutdown() status = %d, want %d", w.Code, http.StatusAccepted)
	}

	// Verify Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Shutdown() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify response body
	expectedBody := "{\"message\":\"server shutdown initiated\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Shutdown() body = %q, want %q", w.Body.String(), expectedBody)
	}

	// Verify shutdown signal was sent to channel
	select {
	case <-shutdownCh:
		// Success - signal was sent
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown() did not send shutdown signal to channel")
	}
}

func TestShutdown_GET_MethodNotAllowed(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Create GET request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Shutdown(w, req)

	// Verify status code
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Shutdown() status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}

	// Verify Allow header
	allow := w.Header().Get("Allow")
	if allow != "POST" {
		t.Errorf("Shutdown() Allow header = %q, want %q", allow, "POST")
	}

	// Verify error response
	expectedBody := "{\"error\":\"method not allowed\"}\n"
	if w.Body.String() != expectedBody {
		t.Errorf("Shutdown() body = %q, want %q", w.Body.String(), expectedBody)
	}

	// Verify no shutdown signal was sent
	select {
	case <-shutdownCh:
		t.Error("Shutdown() should not send signal for non-POST requests")
	case <-time.After(50 * time.Millisecond):
		// Success - no signal sent
	}
}

func TestShutdown_ResponseFormat(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Shutdown(w, req)

	// Verify the JSON structure is correct
	if w.Code != http.StatusAccepted {
		t.Fatalf("Shutdown() status = %d, want %d", w.Code, http.StatusAccepted)
	}

	// Parse and verify structure
	type response struct {
		Message string `json:"message"`
	}

	var resp response
	if err := parseJSON(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Shutdown() failed to parse JSON response: %v", err)
	}

	if resp.Message != "server shutdown initiated" {
		t.Errorf("Shutdown() response.Message = %q, want %q", resp.Message, "server shutdown initiated")
	}
}

func TestShutdown_DuplicateRequests(t *testing.T) {
	// Create shutdown channel with buffer of 1
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w1 := httptest.NewRecorder()
	h.Shutdown(w1, req1)

	// Verify first request succeeded
	if w1.Code != http.StatusAccepted {
		t.Errorf("First Shutdown() status = %d, want %d", w1.Code, http.StatusAccepted)
	}

	// Second request (channel already has signal)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w2 := httptest.NewRecorder()
	h.Shutdown(w2, req2)

	// Verify second request also returns 202 (handler is idempotent)
	if w2.Code != http.StatusAccepted {
		t.Errorf("Second Shutdown() status = %d, want %d", w2.Code, http.StatusAccepted)
	}

	// Verify only one signal is in the channel
	signalCount := 0
	timeout := time.After(100 * time.Millisecond)

	for {
		select {
		case <-shutdownCh:
			signalCount++
		case <-timeout:
			// Verify exactly one signal was sent
			if signalCount != 1 {
				t.Errorf("Expected 1 shutdown signal in channel, got %d", signalCount)
			}
			return
		}
	}
}

func TestShutdown_NonBlocking(t *testing.T) {
	// Create shutdown channel with buffer of 1
	shutdownCh := make(chan struct{}, 1)

	// Fill the channel
	shutdownCh <- struct{}{}

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Create POST request - should not block even though channel is full
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler - should complete immediately without blocking
	done := make(chan bool, 1)
	go func() {
		h.Shutdown(w, req)
		done <- true
	}()

	// Verify handler completes within reasonable time (100ms)
	select {
	case <-done:
		// Success - handler completed without blocking
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown() blocked when channel was full")
	}

	// Verify response is still 202 Accepted
	if w.Code != http.StatusAccepted {
		t.Errorf("Shutdown() status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestShutdown_ConcurrentRequests(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Send multiple concurrent requests
	const numRequests = 10
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
			w := httptest.NewRecorder()
			h.Shutdown(w, req)
			results <- w.Code
		}()
	}

	// Collect all results
	for i := 0; i < numRequests; i++ {
		select {
		case code := <-results:
			if code != http.StatusAccepted {
				t.Errorf("Concurrent Shutdown() request %d status = %d, want %d", i, code, http.StatusAccepted)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("Concurrent Shutdown() request %d timed out", i)
		}
	}

	// Verify channel has at most 1 signal (non-blocking writes)
	signalCount := 0
	timeout := time.After(100 * time.Millisecond)

	for {
		select {
		case <-shutdownCh:
			signalCount++
		case <-timeout:
			if signalCount > 1 {
				t.Errorf("Expected at most 1 shutdown signal, got %d", signalCount)
			}
			return
		}
	}
}

func TestShutdown_ContentTypeJSON(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Shutdown(w, req)

	// Verify Content-Type is application/json
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Shutdown() Content-Type = %q, want %q", contentType, "application/json")
	}

	// Verify body is valid JSON
	var jsonData map[string]interface{}
	if err := parseJSON(w.Body.Bytes(), &jsonData); err != nil {
		t.Errorf("Shutdown() response is not valid JSON: %v", err)
	}
}

func TestShutdown_MethodNotAllowed_AllMethods(t *testing.T) {
	// Test all HTTP methods except POST
	methods := []string{
		http.MethodGet,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
		http.MethodConnect,
		http.MethodTrace,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			shutdownCh := make(chan struct{}, 1)
			h := NewHandlers(nil, nil, shutdownCh)

			req := httptest.NewRequest(method, "/api/v1/shutdown", nil)
			w := httptest.NewRecorder()

			h.Shutdown(w, req)

			// Verify 405 status
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Shutdown() with %s status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
			}

			// Verify Allow header
			allow := w.Header().Get("Allow")
			if allow != "POST" {
				t.Errorf("Shutdown() with %s Allow header = %q, want %q", method, allow, "POST")
			}

			// Verify no signal sent
			select {
			case <-shutdownCh:
				t.Errorf("Shutdown() with %s should not send signal", method)
			case <-time.After(50 * time.Millisecond):
				// Expected - no signal sent
			}
		})
	}
}

func TestShutdown_ErrorResponseFormat(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Create invalid request (GET instead of POST)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Shutdown(w, req)

	// Parse error response
	type errorResponse struct {
		Error string `json:"error"`
	}

	var errResp errorResponse
	if err := parseJSON(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Shutdown() failed to parse error response: %v", err)
	}

	// Verify error message
	if errResp.Error != "method not allowed" {
		t.Errorf("Shutdown() error message = %q, want %q", errResp.Error, "method not allowed")
	}
}

func TestShutdown_ChannelNotNil(t *testing.T) {
	// Verify that handlers require non-nil shutdown channel
	// This is a defensive test to ensure nil channels are handled

	// Create handlers with valid shutdown channel
	shutdownCh := make(chan struct{}, 1)
	h := NewHandlers(nil, nil, shutdownCh)

	// Verify handlers were created successfully
	if h == nil {
		t.Fatal("NewHandlers() returned nil")
	}

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Should not panic
	h.Shutdown(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Shutdown() status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestShutdown_EmptyRequestBody(t *testing.T) {
	// Create shutdown channel
	shutdownCh := make(chan struct{}, 1)

	// Create handlers
	h := NewHandlers(nil, nil, shutdownCh)

	// Create POST request with explicitly nil body
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler
	h.Shutdown(w, req)

	// Should succeed with empty body
	if w.Code != http.StatusAccepted {
		t.Errorf("Shutdown() with nil body status = %d, want %d", w.Code, http.StatusAccepted)
	}

	// Verify signal was sent
	select {
	case <-shutdownCh:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Shutdown() with nil body did not send signal")
	}
}

func TestShutdown_ImmediateResponse(t *testing.T) {
	// Verify handler returns immediately without waiting for shutdown
	shutdownCh := make(chan struct{}, 1)
	h := NewHandlers(nil, nil, shutdownCh)

	// Measure handler execution time
	start := time.Now()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	h.Shutdown(w, req)

	elapsed := time.Since(start)

	// Handler should complete very quickly (< 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Shutdown() took %v, expected < 100ms", elapsed)
	}

	// Verify response
	if w.Code != http.StatusAccepted {
		t.Errorf("Shutdown() status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestShutdown_NilChannel(t *testing.T) {
	// Test that nil shutdown channel is handled gracefully
	// This verifies the defensive check at the beginning of the handler

	// Create handlers with nil shutdown channel
	h := NewHandlers(nil, nil, nil)

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()

	// Call handler - should not panic
	h.Shutdown(w, req)

	// Verify status code is 503 Service Unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Shutdown() with nil channel status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	// Verify error response format
	var errResp map[string]interface{}
	if err := parseJSON(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp["error"] != "shutdown service not available" {
		t.Errorf("Shutdown() with nil channel error = %q, want %q", errResp["error"], "shutdown service not available")
	}
}
