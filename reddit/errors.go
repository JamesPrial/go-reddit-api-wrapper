package graw

import (
	"fmt"
	"strings"
	"time"
)

// joinParts joins error message parts with the specified separator.
func joinParts(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// ConfigError indicates a problem with the client configuration.
// This error is returned when required configuration fields are missing,
// invalid, or when the configuration is inconsistent.
type ConfigError struct {
	// Field contains the name of the configuration field that caused the error
	Field string
	// Message contains the detailed error message
	Message string
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *ConfigError) Error() string {
	var msg string
	if e.Field != "" {
		msg = fmt.Sprintf("config error in field %s: %s", e.Field, e.Message)
	} else {
		msg = fmt.Sprintf("config error: %s", e.Message)
	}
	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	return msg
}

// ValidationError represents errors that occur during input validation.
// This includes validation of subreddit names, post IDs, comment IDs,
// pagination parameters, user agents, URLs, and client configuration.
type ValidationError struct {
	// Field is the field/parameter being validated (e.g., "subreddit", "pagination.Limit", "CommentIDs[0]")
	Field string
	// Value is the invalid value (may be empty if value shouldn't be logged for security reasons)
	Value string
	// Reason is a description of why validation failed
	Reason string
	// Err is the underlying error (if applicable)
	Err error
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *ValidationError) Error() string {
	var msg string
	if e.Value != "" {
		msg = fmt.Sprintf("validation error for %s with value '%s': %s", e.Field, e.Value, e.Reason)
	} else {
		msg = fmt.Sprintf("validation error for %s: %s", e.Field, e.Reason)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}

	return msg
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// AuthError indicates an authentication failure.
// This error is returned when OAuth2 token requests fail or when
// authentication credentials are invalid.
type AuthError struct {
	// StatusCode is the HTTP status code (if from an HTTP response)
	StatusCode int
	// Message contains the detailed error message
	Message string
	// Body contains the raw response body (if available)
	Body string
	// Err contains the underlying error if available
	Err error
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *AuthError) Error() string {
	// Handle special cases to match legacy format
	if e.StatusCode == 0 && e.Body == "" && e.Message == "" && e.Err != nil {
		msg := fmt.Sprintf("auth error, err: %v", e.Err)
		if e.RequestID != "" {
			msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
		}
		return msg
	}
	if e.StatusCode == 0 && e.Body != "" && e.Message == "" && e.Err == nil {
		msg := fmt.Sprintf("auth error, body: %q", e.Body)
		if e.RequestID != "" {
			msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
		}
		return msg
	}

	var parts []string
	parts = append(parts, "auth error")

	if e.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("status code %d", e.StatusCode))
	}

	if e.Body != "" {
		parts = append(parts, fmt.Sprintf("body: %q", e.Body))
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("err: %v", e.Err))
	}

	var msg string
	if len(parts) == 1 {
		msg = parts[0]
	} else {
		msg = parts[0] + ": " + fmt.Sprintf("%s", joinParts(parts[1:], ", "))
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}

	return msg
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// APIError represents an error response from the Reddit API.
// This error is returned when Reddit returns a non-2xx status code
// or when the API returns an error in the response body.
type APIError struct {
	// StatusCode is the HTTP status code
	StatusCode int
	// ErrorCode is the error code from Reddit (if available)
	ErrorCode string
	// Message is the error message from Reddit
	Message string
	// Details contains any additional error details from the API
	Details interface{}
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *APIError) Error() string {
	var msg string
	if e.ErrorCode != "" {
		msg = fmt.Sprintf("reddit API error (status %d, code %s): %s", e.StatusCode, e.ErrorCode, e.Message)
	} else {
		// Use the legacy format for backward compatibility
		msg = fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Message)
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}

	return msg
}

// RateLimitError represents errors that occur during rate limiting operations.
// This includes context cancellation while waiting for rate limit availability,
// and errors related to Reddit's rate limit enforcement.
type RateLimitError struct {
	// Reason is the reason for the error (e.g., "context_cancelled", "limiter_wait_failed")
	Reason string
	// WaitDuration is how long we were trying to wait (if applicable)
	WaitDuration time.Duration
	// Err is the underlying error
	Err error
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *RateLimitError) Error() string {
	var msg string
	if e.WaitDuration > 0 {
		msg = fmt.Sprintf("rate limit error (%s) after waiting %v: %v", e.Reason, e.WaitDuration, e.Err)
	} else {
		msg = fmt.Sprintf("rate limit error (%s): %v", e.Reason, e.Err)
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}

	return msg
}

func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// NetworkError represents errors that occur during HTTP request execution.
// This includes network failures, connection timeouts, DNS resolution failures,
// and other transport-level errors.
type NetworkError struct {
	// Method is the HTTP method (GET, POST, etc.)
	Method string
	// URL is the URL being requested
	URL string
	// Duration is how long the request took before failing (if applicable)
	Duration time.Duration
	// Err is the underlying error
	Err error
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *NetworkError) Error() string {
	var msg string
	if e.Duration > 0 {
		msg = fmt.Sprintf("network error for %s %s after %v: %v", e.Method, e.URL, e.Duration, e.Err)
	} else {
		msg = fmt.Sprintf("network error for %s %s: %v", e.Method, e.URL, e.Err)
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}

	return msg
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// ParseError indicates a problem parsing the API response.
// This error is returned when JSON unmarshaling fails, when the response
// structure is unexpected, or when required fields are missing.
type ParseError struct {
	// Operation is the name of the API operation where parsing failed
	Operation string
	// Message contains the detailed error message
	Message string
	// Err contains the underlying error if available
	Err error
	// RequestID is the unique identifier for the request (if available)
	RequestID string
}

func (e *ParseError) Error() string {
	// Use Message if available, otherwise use Err.Error()
	msg := e.Message
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}

	var errMsg string
	if e.Operation != "" {
		errMsg = fmt.Sprintf("parse error during %s: %s", e.Operation, msg)
	} else {
		errMsg = fmt.Sprintf("parse error: %s", msg)
	}

	if e.RequestID != "" {
		errMsg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}

	return errMsg
}

func (e *ParseError) Unwrap() error {
	return e.Err
}
