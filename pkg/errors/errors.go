// Package errors defines common error types used throughout the Reddit API wrapper.
package errors

import (
	"fmt"
	"strings"
)

// joinParts joins error message parts with the specified separator.
func joinParts(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// ConfigError indicates a problem with the client configuration.
type ConfigError struct {
	// Field contains the name of the configuration field that caused the error
	Field string
	// Message contains the detailed error message
	Message string
}

func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config error in field %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("config error: %s", e.Message)
}

// AuthError indicates an authentication failure.
type AuthError struct {
	// StatusCode is the HTTP status code (if from an HTTP response)
	StatusCode int
	// Message contains the detailed error message
	Message string
	// Body contains the raw response body (if available)
	Body string
	// Err contains the underlying error if available
	Err error
}

func (e *AuthError) Error() string {
	// Handle special cases to match legacy format
	if e.StatusCode == 0 && e.Body == "" && e.Message == "" && e.Err != nil {
		return fmt.Sprintf("auth error, err: %v", e.Err)
	}
	if e.StatusCode == 0 && e.Body != "" && e.Message == "" && e.Err == nil {
		return fmt.Sprintf("auth error, body: %q", e.Body)
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

	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + ": " + fmt.Sprintf("%s", joinParts(parts[1:], ", "))
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// StateError indicates an operation was attempted when the client is not ready.
type StateError struct {
	// Operation is the name of the operation that was attempted
	Operation string
	// Message contains the detailed error message
	Message string
}

func (e *StateError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("state error during %s: %s", e.Operation, e.Message)
	}
	return fmt.Sprintf("state error: %s", e.Message)
}

// RequestError indicates a problem with making an API request.
type RequestError struct {
	// Operation is the name of the API operation that failed
	Operation string
	// URL is the URL that was being accessed
	URL string
	// Message contains the detailed error message
	Message string
	// Err contains the underlying error if available
	Err error
}

func (e *RequestError) Error() string {
	// Use Message if available, otherwise use Err.Error()
	msg := e.Message
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}

	if e.Operation != "" && e.URL != "" {
		return fmt.Sprintf("request error during %s to %s: %s", e.Operation, e.URL, msg)
	} else if e.Operation != "" {
		return fmt.Sprintf("request error during %s: %s", e.Operation, msg)
	}
	return fmt.Sprintf("request error: %s", msg)
}

func (e *RequestError) Unwrap() error {
	return e.Err
}

// ParseError indicates a problem parsing the API response.
type ParseError struct {
	// Operation is the name of the API operation where parsing failed
	Operation string
	// Message contains the detailed error message
	Message string
	// Err contains the underlying error if available
	Err error
}

func (e *ParseError) Error() string {
	// Use Message if available, otherwise use Err.Error()
	msg := e.Message
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}

	if e.Operation != "" {
		return fmt.Sprintf("parse error during %s: %s", e.Operation, msg)
	}
	return fmt.Sprintf("parse error: %s", msg)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// APIError represents an error response from the Reddit API.
type APIError struct {
	// StatusCode is the HTTP status code
	StatusCode int
	// ErrorCode is the error code from Reddit (if available)
	ErrorCode string
	// Message is the error message from Reddit
	Message string
	// Details contains any additional error details from the API
	Details interface{}
}

func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("reddit API error (status %d, code %s): %s", e.StatusCode, e.ErrorCode, e.Message)
	}
	// Use the legacy format for backward compatibility
	return fmt.Sprintf("API request failed with status %d: %s", e.StatusCode, e.Message)
}

// ClientError indicates a problem with the HTTP client operations.
type ClientError struct {
	// Operation describes what the client was trying to do
	Operation string
	// Message contains the detailed error message
	Message string
	// Err contains the underlying error if available
	Err error
}

func (e *ClientError) Error() string {
	// For backward compatibility, if only Err is set, return its error message directly
	if e.Err != nil && e.Operation == "" && e.Message == "" {
		return e.Err.Error()
	}
	if e.Err != nil {
		return fmt.Sprintf("client error: %v", e.Err)
	}
	if e.Operation != "" && e.Message != "" {
		return fmt.Sprintf("client error during %s: %s", e.Operation, e.Message)
	}
	if e.Operation != "" {
		return fmt.Sprintf("client error during %s", e.Operation)
	}
	if e.Message != "" {
		return fmt.Sprintf("client error: %s", e.Message)
	}
	return "client error"
}

func (e *ClientError) Unwrap() error {
	return e.Err
}
