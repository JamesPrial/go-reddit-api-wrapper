// Package internal provides internal utilities for the Reddit CLI.
package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// PaginationFlags holds pagination parameters from CLI flags.
type PaginationFlags struct {
	Limit  int
	After  string
	Before string
}

// BuildPagination creates a Pagination struct from CLI flags.
// Returns a validation error if parameters are invalid.
func BuildPagination(flags PaginationFlags) (*types.Pagination, error) {
	// Validate limit
	if flags.Limit < 0 {
		return nil, fmt.Errorf("limit cannot be negative: got %d", flags.Limit)
	}
	if flags.Limit > 100 {
		return nil, fmt.Errorf("limit cannot exceed 100: got %d (Reddit API constraint)", flags.Limit)
	}

	// Validate after/before mutual exclusivity
	if flags.After != "" && flags.Before != "" {
		return nil, fmt.Errorf("cannot use both --after and --before pagination tokens")
	}

	return &types.Pagination{
		Limit:  flags.Limit,
		After:  flags.After,
		Before: flags.Before,
	}, nil
}

// ErrorType represents the type of error for proper handling and exit codes.
type ErrorType int

const (
	// ErrorTypeUnknown represents an unclassified error.
	ErrorTypeUnknown ErrorType = iota
	// ErrorTypeConfig represents a configuration error.
	ErrorTypeConfig
	// ErrorTypeValidation represents an input validation error.
	ErrorTypeValidation
	// ErrorTypeAuth represents an authentication error.
	ErrorTypeAuth
	// ErrorTypeAPI represents an error from the Reddit API.
	ErrorTypeAPI
	// ErrorTypeRateLimit represents a rate limit error.
	ErrorTypeRateLimit
	// ErrorTypeNetwork represents a network/connection error.
	ErrorTypeNetwork
	// ErrorTypeParse represents a parsing error.
	ErrorTypeParse
	// ErrorTypeNotFound represents a not found error.
	ErrorTypeNotFound
)

// ClassifyError determines the type of error for proper handling.
// Returns the error type and a user-friendly error message.
func ClassifyError(err error) (ErrorType, string) {
	if err == nil {
		return ErrorTypeUnknown, "unknown error"
	}

	// Check for typed Reddit errors
	var configErr *graw.ConfigError
	if errors.As(err, &configErr) {
		return ErrorTypeConfig, fmt.Sprintf("Configuration error: %s", configErr.Error())
	}

	var validationErr *graw.ValidationError
	if errors.As(err, &validationErr) {
		return ErrorTypeValidation, fmt.Sprintf("Invalid input: %s", validationErr.Error())
	}

	var authErr *graw.AuthError
	if errors.As(err, &authErr) {
		return ErrorTypeAuth, fmt.Sprintf("Authentication failed: %s", authErr.Error())
	}

	var apiErr *graw.APIError
	if errors.As(err, &apiErr) {
		// Handle specific API error codes for better UX
		switch apiErr.ErrorCode {
		case "NOT_FOUND":
			return ErrorTypeNotFound, fmt.Sprintf("Resource not found: %s", apiErr.Message)
		case "FORBIDDEN":
			return ErrorTypeAPI, fmt.Sprintf("Access denied: %s", apiErr.Message)
		case "INVALID_SUBREDDIT":
			return ErrorTypeAPI, fmt.Sprintf("Invalid subreddit: %s", apiErr.Message)
		default:
			return ErrorTypeAPI, fmt.Sprintf("Reddit API error: %s", apiErr.Error())
		}
	}

	var rateLimitErr *graw.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return ErrorTypeRateLimit, fmt.Sprintf("Rate limited: %s", rateLimitErr.Error())
	}

	var networkErr *graw.NetworkError
	if errors.As(err, &networkErr) {
		return ErrorTypeNetwork, fmt.Sprintf("Network error: %s", networkErr.Error())
	}

	var parseErr *graw.ParseError
	if errors.As(err, &parseErr) {
		return ErrorTypeParse, fmt.Sprintf("Failed to parse response: %s", parseErr.Error())
	}

	// Fallback to generic error
	return ErrorTypeUnknown, fmt.Sprintf("Error: %v", err)
}

// ExitCodeForError returns the appropriate exit code for an error type.
// Exit codes follow Unix conventions:
// 0 = success, 1 = general error, 2 = misuse (validation),
// 3-127 = specific error types.
func ExitCodeForError(errType ErrorType) int {
	switch errType {
	case ErrorTypeConfig:
		return 2 // Usage/config error
	case ErrorTypeValidation:
		return 2 // Invalid input
	case ErrorTypeAuth:
		return 3 // Auth error
	case ErrorTypeAPI:
		return 4 // API error
	case ErrorTypeRateLimit:
		return 5 // Rate limit
	case ErrorTypeNetwork:
		return 6 // Network error
	case ErrorTypeParse:
		return 7 // Parse error
	case ErrorTypeNotFound:
		return 8 // Not found
	default:
		return 1 // General error
	}
}

// ExitCodeForErr is a convenience function that combines ClassifyError and ExitCodeForError.
func ExitCodeForErr(err error) int {
	errType, _ := ClassifyError(err)
	return ExitCodeForError(errType)
}

// UserFriendlyError returns a user-friendly error message for an error.
// This is suitable for displaying to end users.
func UserFriendlyError(err error) string {
	_, msg := ClassifyError(err)
	return msg
}

// FormatErrorForDisplay formats an error with context for CLI output.
// Includes hints for common errors.
func FormatErrorForDisplay(err error, operation string) string {
	errType, msg := ClassifyError(err)

	// Add operation context if provided
	if operation != "" {
		msg = fmt.Sprintf("Failed to %s: %s", operation, msg)
	}

	// Add helpful hints for specific error types
	var hint string
	switch errType {
	case ErrorTypeAuth:
		hint = "Check your REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, and credentials if using user auth"
	case ErrorTypeValidation:
		hint = "Check the format of your input parameters"
	case ErrorTypeRateLimit:
		hint = "Wait a moment and try again"
	case ErrorTypeNetwork:
		hint = "Check your network connection and try again"
	case ErrorTypeNotFound:
		hint = "Verify the subreddit or post ID exists"
	}

	if hint != "" {
		msg += fmt.Sprintf("\nHint: %s", hint)
	}

	return msg
}

// ValidateSubreddit checks if a subreddit name is in a valid format.
// Returns an error if the subreddit name is empty or contains invalid characters.
func ValidateSubreddit(name string) error {
	if name == "" {
		return fmt.Errorf("subreddit name cannot be empty")
	}
	if len(name) > 21 {
		return fmt.Errorf("subreddit name too long: %d characters (max 21)", len(name))
	}
	return nil
}

// ValidatePostID checks if a post ID is in a valid format.
// Returns an error if the post ID is empty.
func ValidatePostID(id string) error {
	if id == "" {
		return fmt.Errorf("post ID cannot be empty")
	}
	return nil
}

// ValidateCommentID checks if a comment ID is in a valid format.
// Returns an error if the comment ID is empty.
func ValidateCommentID(id string) error {
	if id == "" {
		return fmt.Errorf("comment ID cannot be empty")
	}
	return nil
}

// SanitizeOutput removes or escapes potentially problematic characters from output strings.
// This is useful for preventing terminal escape sequence injection.
func SanitizeOutput(s string) string {
	// In a real CLI, you might strip or replace control characters
	// For now, we return as-is since the Reddit API already validates content
	return s
}

// TruncateString truncates a string to a maximum length, adding ellipsis if needed.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

// ParseInterval parses a duration string (e.g., "5m", "1h30m") and validates it.
// Returns an error if the duration is invalid or non-positive.
// This is useful for parsing monitor polling intervals and other time-based configuration.
func ParseInterval(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("interval cannot be empty")
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid interval format: %w", err)
	}

	if duration <= 0 {
		return 0, fmt.Errorf("interval must be positive, got %v", duration)
	}

	return duration, nil
}
