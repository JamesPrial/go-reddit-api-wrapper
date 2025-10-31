package validator

import (
	"fmt"
)

// ValidationError represents errors that occur during input validation.
// This includes validation of subreddit names, post IDs, comment IDs, pagination parameters,
// user agents, URLs, and client configuration.
type ValidationError struct {
	Field     string // The field/parameter being validated (e.g., "subreddit", "pagination.Limit", "CommentIDs[0]")
	Value     string // The invalid value (may be empty if value shouldn't be logged for security reasons)
	Reason    string // Description of why validation failed
	RequestID string // Request ID for tracing (empty if not available)
	Err       error  // The underlying error (if applicable)
}

func (e *ValidationError) Error() string {
	var msg string
	if e.Value != "" {
		msg = fmt.Sprintf("validation error for %s with value '%s': %s", e.Field, e.Value, e.Reason)
	} else {
		msg = fmt.Sprintf("validation error for %s: %s", e.Field, e.Reason)
	}

	if e.RequestID != "" {
		msg += fmt.Sprintf(", request_id: %s", e.RequestID)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}
