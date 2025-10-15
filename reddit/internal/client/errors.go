package client

import (
	"fmt"
	"time"
)

// RequestBuildError represents errors that occur during request construction,
// such as URL parsing failures or request creation failures.
type RequestBuildError struct {
	Operation string // The operation being performed (e.g., "parse_url", "create_request")
	URL       string // The URL being constructed (if applicable)
	Err       error  // The underlying error
}

func (e *RequestBuildError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("request build error during %s for URL '%s': %v", e.Operation, e.URL, e.Err)
	}
	return fmt.Sprintf("request build error during %s: %v", e.Operation, e.Err)
}

func (e *RequestBuildError) Unwrap() error {
	return e.Err
}

// RateLimitError represents errors that occur during rate limiting operations,
// such as context cancellation while waiting for rate limit availability.
type RateLimitError struct {
	Reason       string        // The reason for the error (e.g., "context_cancelled", "limiter_wait_failed")
	WaitDuration time.Duration // How long we were trying to wait (if applicable)
	Err          error         // The underlying error
}

func (e *RateLimitError) Error() string {
	if e.WaitDuration > 0 {
		return fmt.Sprintf("rate limit error (%s) after waiting %v: %v", e.Reason, e.WaitDuration, e.Err)
	}
	return fmt.Sprintf("rate limit error (%s): %v", e.Reason, e.Err)
}

func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// TransportError represents errors that occur during HTTP request execution,
// such as network failures, connection timeouts, or DNS resolution failures.
type TransportError struct {
	Method   string        // The HTTP method (GET, POST, etc.)
	URL      string        // The URL being requested
	Duration time.Duration // How long the request took before failing (if applicable)
	Err      error         // The underlying error
}

func (e *TransportError) Error() string {
	if e.Duration > 0 {
		return fmt.Sprintf("transport error for %s %s after %v: %v", e.Method, e.URL, e.Duration, e.Err)
	}
	return fmt.Sprintf("transport error for %s %s: %v", e.Method, e.URL, e.Err)
}

func (e *TransportError) Unwrap() error {
	return e.Err
}

// ResponseReadError represents errors that occur while reading the response body,
// including I/O errors and size limit violations.
type ResponseReadError struct {
	URL       string // The URL of the request
	BytesRead int64  // How many bytes were read before the error
	MaxSize   int64  // The maximum allowed size (0 if not a size error)
	Err       error  // The underlying error
}

func (e *ResponseReadError) Error() string {
	if e.MaxSize > 0 && e.BytesRead >= e.MaxSize {
		return fmt.Sprintf("response read error for %s: exceeded max size of %d bytes", e.URL, e.MaxSize)
	}
	if e.BytesRead > 0 {
		return fmt.Sprintf("response read error for %s after reading %d bytes: %v", e.URL, e.BytesRead, e.Err)
	}
	return fmt.Sprintf("response read error for %s: %v", e.URL, e.Err)
}

func (e *ResponseReadError) Unwrap() error {
	return e.Err
}

// DecodeError represents errors that occur during JSON parsing or unmarshaling
// of response data.
type DecodeError struct {
	Operation   string // The operation being performed (e.g., "unmarshal_thing", "unmarshal_array")
	BodySnippet string // First N bytes of the body for debugging (empty if not available)
	Err         error  // The underlying error
}

func (e *DecodeError) Error() string {
	var msg string
	if e.BodySnippet != "" {
		msg = fmt.Sprintf("decode error during %s, body: %q", e.Operation, e.BodySnippet)
	} else {
		msg = fmt.Sprintf("decode error during %s", e.Operation)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *DecodeError) Unwrap() error {
	return e.Err
}

// ResponseValidationError represents errors that occur when the response structure
// is invalid or unexpected (e.g., wrong type, missing required fields, unexpected format).
type ResponseValidationError struct {
	Issue       string // Description of the issue (e.g., "unexpected_kind", "empty_response")
	Expected    string // What was expected (if applicable)
	Actual      string // What was actually received (if applicable)
	BodySnippet string // First N bytes of the body for debugging (empty if not available)
}

func (e *ResponseValidationError) Error() string {
	var msg string
	if e.Expected != "" && e.Actual != "" {
		msg = fmt.Sprintf("response validation error: %s (expected %s, got %s)", e.Issue, e.Expected, e.Actual)
	} else {
		msg = fmt.Sprintf("response validation error: %s", e.Issue)
	}

	if e.BodySnippet != "" {
		msg += fmt.Sprintf(", body: %q", e.BodySnippet)
	}

	return msg
}

// Unwrap is not implemented for ResponseValidationError because it doesn't wrap another error
// (it represents a validation issue, not an underlying error).

// APIError represents an error response from the Reddit API.
// This is an internal type that gets translated to the public graw.APIError.
type APIError struct {
	StatusCode int         // The HTTP status code
	ErrorCode  string      // The error code from Reddit (if available)
	Message    string      // The error message from Reddit
	Details    interface{} // Any additional error details from the API
}

func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("reddit API error (status %d, code %s): %s", e.StatusCode, e.ErrorCode, e.Message)
	}
	return fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Message)
}

// Unwrap is not implemented for APIError because it doesn't wrap another error
// (it represents an API-level error response).
