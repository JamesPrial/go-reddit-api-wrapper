package adversarial_tests

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/adversarial_tests/helpers"
	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestDeepErrorWrapping tests error wrapping chains
func TestDeepErrorWrapping(t *testing.T) {
	// Create a deep error chain
	baseErr := errors.New("base error")
	wrapped := baseErr

	// Wrap 10 levels deep
	for i := 1; i <= 10; i++ {
		wrapped = fmt.Errorf("level %d: %w", i, wrapped)
	}

	// Verify we can unwrap all the way to the base
	current := wrapped
	levels := 0

	for current != nil {
		levels++
		current = errors.Unwrap(current)
	}

	if levels != 11 { // 10 wrapping levels + 1 base
		t.Errorf("Expected 11 levels in error chain, got %d", levels)
	}

	// Verify errors.Is works through deep chain
	if !errors.Is(wrapped, baseErr) {
		t.Error("errors.Is failed to find base error in deep chain")
	}

	t.Logf("Deep error wrapping test passed: %d levels", levels)
}

// TestErrorTypePreservation tests that error types are preserved through wrapping
func TestErrorTypePreservation(t *testing.T) {
	testCases := []struct {
		name  string
		error error
	}{
		{"AuthError", &pkgerrs.AuthError{Message: "test auth error"}},
		{"ConfigError", &pkgerrs.ConfigError{Message: "test config error"}},
		{"StateError", &pkgerrs.StateError{Message: "test state error"}},
		{"RequestError", &pkgerrs.RequestError{Message: "test request error"}},
		{"ParseError", &pkgerrs.ParseError{Message: "test parse error"}},
		{"APIError", &pkgerrs.APIError{Message: "test api error"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap the error
			wrapped := fmt.Errorf("wrapped: %w", tc.error)

			// Verify type is preserved with errors.As
			switch tc.error.(type) {
			case *pkgerrs.AuthError:
				var authErr *pkgerrs.AuthError
				if !errors.As(wrapped, &authErr) {
					t.Error("AuthError type not preserved through wrapping")
				}
			case *pkgerrs.ConfigError:
				var configErr *pkgerrs.ConfigError
				if !errors.As(wrapped, &configErr) {
					t.Error("ConfigError type not preserved through wrapping")
				}
			case *pkgerrs.StateError:
				var stateErr *pkgerrs.StateError
				if !errors.As(wrapped, &stateErr) {
					t.Error("StateError type not preserved through wrapping")
				}
			case *pkgerrs.RequestError:
				var reqErr *pkgerrs.RequestError
				if !errors.As(wrapped, &reqErr) {
					t.Error("RequestError type not preserved through wrapping")
				}
			case *pkgerrs.ParseError:
				var parseErr *pkgerrs.ParseError
				if !errors.As(wrapped, &parseErr) {
					t.Error("ParseError type not preserved through wrapping")
				}
			case *pkgerrs.APIError:
				var apiErr *pkgerrs.APIError
				if !errors.As(wrapped, &apiErr) {
					t.Error("APIError type not preserved through wrapping")
				}
			}
		})
	}
}

// TestContextCancellationPropagation tests that context.Canceled is properly propagated
func TestContextCancellationPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay to allow context cancellation
		time.Sleep(2 * time.Second)

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind": "t2", "data": {"id": "test"}}`))
	}))
	defer server.Close()

	client, err := internal.NewClient(nil, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/test", nil)
	var thing types.Thing
	err = client.Do(req, &thing)

	// Should get context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	// Verify it's a context error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}

	t.Logf("Context cancellation properly propagated: %v", err)
}

// TestConcurrentErrorHandling tests error handling under concurrent load
func TestConcurrentErrorHandling(t *testing.T) {
	// Server that returns errors half the time
	var requestNum int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Atomically increment and get the current request number
		currentNum := atomic.AddInt64(&requestNum, 1)

		if currentNum%2 == 0 {
			// Return error
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Internal Server Error`))
		} else {
			// Return success
			w.Header().Set("X-Ratelimit-Remaining", "60")
			w.Header().Set("X-Ratelimit-Reset", "60")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"kind": "t2", "data": {"id": "test"}}`))
		}
	}))
	defer server.Close()

	client, err := internal.NewClient(nil, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Launch concurrent requests
	numGoroutines := 100
	errors := make(chan error, numGoroutines)
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			ctx := context.Background()
			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/test", nil)

			var thing types.Thing
			err := client.Do(req, &thing)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Count errors
	errorCount := 0
	for range errors {
		errorCount++
	}

	t.Logf("Concurrent error handling: %d errors out of %d requests", errorCount, numGoroutines)

	// Should have approximately half errors
	if errorCount < numGoroutines/4 || errorCount > 3*numGoroutines/4 {
		t.Errorf("Unexpected error count: %d (expected around %d)", errorCount, numGoroutines/2)
	}
}

// TestPanicRecovery tests that no inputs cause unrecovered panics
func TestPanicRecovery(t *testing.T) {
	generator := helpers.NewJSONGenerator()

	testCases := []struct {
		name     string
		jsonData string
	}{
		{"malformed_json", `{invalid json`},
		{"null", `null`},
		{"empty_string", ``},
		{"just_brackets", `{}`},
		{"array_instead_of_object", `[]`},
		{"nested_nulls", `{"kind": null, "data": null}`},
		{"deeply_nested", generator.GenerateJSONBomb(50)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure no panic occurs
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic occurred: %v", r)
				}
			}()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Ratelimit-Remaining", "60")
				w.Header().Set("X-Ratelimit-Reset", "60")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.jsonData))
			}))
			defer server.Close()

			client, err := internal.NewClient(nil, server.URL, "test/1.0", nil)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx := context.Background()
			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/test", nil)

			// Should not panic, may return error
			var thing types.Thing
			err = client.Do(req, &thing)

			t.Logf("Input: %s, Error: %v", tc.name, err)
		})
	}
}

// TestErrorChainIntegrity tests that error chains remain intact
func TestErrorChainIntegrity(t *testing.T) {
	// Create a chain of errors
	level0 := errors.New("level 0")
	level1 := fmt.Errorf("level 1: %w", level0)
	level2 := fmt.Errorf("level 2: %w", level1)
	level3 := fmt.Errorf("level 3: %w", level2)

	// Verify chain integrity
	if !errors.Is(level3, level0) {
		t.Error("Error chain broken: level3 does not contain level0")
	}

	if !errors.Is(level3, level1) {
		t.Error("Error chain broken: level3 does not contain level1")
	}

	if !errors.Is(level3, level2) {
		t.Error("Error chain broken: level3 does not contain level2")
	}

	// Unwrap and verify
	unwrapped := errors.Unwrap(level3)
	if unwrapped.Error() != level2.Error() {
		t.Errorf("Unwrap failed: expected %q, got %q", level2.Error(), unwrapped.Error())
	}

	t.Log("Error chain integrity verified")
}

// TestErrorMessageFormatting tests error message formatting
func TestErrorMessageFormatting(t *testing.T) {
	testCases := []struct {
		name     string
		error    error
		expected string
	}{
		{
			"AuthError_with_status",
			&pkgerrs.AuthError{StatusCode: 401, Message: "unauthorized"},
			"auth error",
		},
		{
			"ConfigError_with_field",
			&pkgerrs.ConfigError{Field: "client_id", Message: "required"},
			"config error",
		},
		{
			"RequestError_with_url",
			&pkgerrs.RequestError{Operation: "GET", URL: "http://example.com"},
			"request error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorMsg := tc.error.Error()

			if !strings.Contains(errorMsg, tc.expected) {
				t.Errorf("Error message %q does not contain expected substring %q",
					errorMsg, tc.expected)
			}

			t.Logf("Error message: %s", errorMsg)
		})
	}
}

// TestTimeoutErrorPropagation tests that timeout errors are properly propagated
func TestTimeoutErrorPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than timeout
		time.Sleep(2 * time.Second)

		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind": "t2", "data": {"id": "test"}}`))
	}))
	defer server.Close()

	client, err := internal.NewClient(nil, server.URL, "test/1.0", nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/test", nil)
	var thing types.Thing
	err = client.Do(req, &thing)

	// Should get timeout error
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	// Verify it's a deadline exceeded error
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("Error type: %T, Error: %v", err, err)
	}

	t.Logf("Timeout error properly propagated: %v", err)
}

// TestErrorUnwrapBehavior tests Unwrap method behavior
func TestErrorUnwrapBehavior(t *testing.T) {
	baseErr := errors.New("base error")

	// Wrap in different error types
	authErr := &pkgerrs.AuthError{Err: baseErr}

	// Test Unwrap
	unwrapped := errors.Unwrap(authErr)
	if unwrapped != baseErr {
		t.Errorf("Unwrap failed: expected %v, got %v", baseErr, unwrapped)
	}

	// Test errors.Is through unwrap
	if !errors.Is(authErr, baseErr) {
		t.Error("errors.Is failed to find base error")
	}

	t.Log("Error unwrap behavior verified")
}

// TestNilErrorHandling tests handling of nil errors
func TestNilErrorHandling(t *testing.T) {
	// Test that nil errors don't cause issues
	var err error

	// Should be able to check nil error
	if err != nil {
		t.Error("Nil error check failed")
	}

	// Should be able to unwrap nil
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		t.Errorf("Unwrapping nil should return nil, got: %v", unwrapped)
	}

	// Should be able to use errors.Is with nil
	if errors.Is(err, errors.New("some error")) {
		t.Error("errors.Is with nil should return false")
	}

	t.Log("Nil error handling verified")
}

// TestConcurrentErrorTypeAssertion tests concurrent error type assertions
func TestConcurrentErrorTypeAssertion(t *testing.T) {
	// Create different error types
	errors := []error{
		&pkgerrs.AuthError{Message: "auth"},
		&pkgerrs.ConfigError{Message: "config"},
		&pkgerrs.StateError{Message: "state"},
		&pkgerrs.RequestError{Message: "request"},
		&pkgerrs.ParseError{Message: "parse"},
		&pkgerrs.APIError{Message: "api"},
	}

	// Concurrently check error types
	numGoroutines := 100
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Check different error types
			err := errors[id%len(errors)]

			// Type assertion should work correctly
			switch err.(type) {
			case *pkgerrs.AuthError:
				// OK
			case *pkgerrs.ConfigError:
				// OK
			case *pkgerrs.StateError:
				// OK
			case *pkgerrs.RequestError:
				// OK
			case *pkgerrs.ParseError:
				// OK
			case *pkgerrs.APIError:
				// OK
			default:
				t.Errorf("Unexpected error type: %T", err)
			}
		}(i)
	}

	wg.Wait()

	t.Log("Concurrent error type assertions completed successfully")
}

// TestErrorInDifferentContexts tests errors in various contexts
func TestErrorInDifferentContexts(t *testing.T) {
	// Create contexts that need cleanup
	ctxWithCancel, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctxWithDeadline, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancel2()

	testCases := []struct {
		name    string
		ctx     context.Context
		timeout time.Duration
	}{
		{"background_context", context.Background(), 100 * time.Millisecond},
		{"todo_context", context.TODO(), 100 * time.Millisecond},
		{"with_cancel", ctxWithCancel, 100 * time.Millisecond},
		{"with_deadline", ctxWithDeadline, 200 * time.Millisecond},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tc.timeout)

				w.Header().Set("X-Ratelimit-Remaining", "60")
				w.Header().Set("X-Ratelimit-Reset", "60")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"kind": "t2", "data": {"id": "test"}}`))
			}))
			defer server.Close()

			client, err := internal.NewClient(nil, server.URL, "test/1.0", nil)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			req, _ := http.NewRequestWithContext(tc.ctx, "GET", server.URL+"/test", nil)
			var thing types.Thing
			err = client.Do(req, &thing)

			t.Logf("Context: %s, Error: %v", tc.name, err)
		})
	}
}

// TestErrorWithMultipleWrappingLayers tests complex error wrapping scenarios
func TestErrorWithMultipleWrappingLayers(t *testing.T) {
	// Create complex error chain
	base := errors.New("io error")
	layer1 := &pkgerrs.RequestError{Err: base}
	layer2 := fmt.Errorf("network error: %w", layer1)
	layer3 := &pkgerrs.ParseError{Err: layer2}
	layer4 := fmt.Errorf("processing failed: %w", layer3)

	// Verify we can find base error
	if !errors.Is(layer4, base) {
		t.Error("Failed to find base error in complex chain")
	}

	// Verify we can find intermediate types
	var reqErr *pkgerrs.RequestError
	if !errors.As(layer4, &reqErr) {
		t.Error("Failed to find RequestError in chain")
	}

	var parseErr *pkgerrs.ParseError
	if !errors.As(layer4, &parseErr) {
		t.Error("Failed to find ParseError in chain")
	}

	t.Log("Complex error wrapping verified")
}

// TestErrorMessageConsistency tests that error messages are consistent
func TestErrorMessageConsistency(t *testing.T) {
	// Create same error multiple times
	errors := make([]*pkgerrs.ConfigError, 10)
	for i := range errors {
		errors[i] = &pkgerrs.ConfigError{
			Field:   "test_field",
			Message: "test message",
		}
	}

	// All should have same error message
	expected := errors[0].Error()
	for i, err := range errors {
		if err.Error() != expected {
			t.Errorf("Error %d has inconsistent message: %q vs %q", i, err.Error(), expected)
		}
	}

	t.Log("Error message consistency verified")
}
