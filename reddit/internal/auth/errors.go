package auth

import (
	"fmt"
)

// ConfigError represents configuration and validation errors that occur
// before any network requests are made (e.g., invalid URLs, missing credentials).
type ConfigError struct {
	Field string // The configuration field that failed (e.g., "base_url", "token_path")
	Value string // The invalid value (if applicable)
	Err   error  // The underlying error
}

func (e *ConfigError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("config error for field '%s' with value '%s': %v", e.Field, e.Value, e.Err)
	}
	return fmt.Sprintf("config error for field '%s': %v", e.Field, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// TokenError represents errors that occur during token acquisition or refresh.
// This includes network errors, authentication failures, and response parsing errors.
type TokenError struct {
	Operation  string // The operation being performed ("fetch", "refresh")
	HTTPStatus int    // HTTP status code (0 if no HTTP request was made)
	Body       string // Response body for debugging (empty if not applicable)
	RequestID  string // Request ID for tracing (empty if not available)
	Err        error  // The underlying error
}

func (e *TokenError) Error() string {
	var msg string
	if e.HTTPStatus > 0 {
		msg = fmt.Sprintf("token %s error: status %d", e.Operation, e.HTTPStatus)
		if e.Body != "" {
			msg += fmt.Sprintf(", body: %q", e.Body)
		}
	} else {
		msg = fmt.Sprintf("token %s error", e.Operation)
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(", request_id: %s", e.RequestID)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *TokenError) Unwrap() error {
	return e.Err
}
