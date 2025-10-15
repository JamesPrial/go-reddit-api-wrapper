package parse

import (
	"fmt"
)

// KindError represents errors that occur during Thing kind validation,
// including nil things, wrong kinds, and unknown kinds.
type KindError struct {
	Operation string // The operation being performed (e.g., "parse_listing", "parse_comment")
	Expected  string // The expected kind (e.g., "Listing", "t3", "non-nil")
	Actual    string // The actual kind received (e.g., "t1", "nil", "unknown")
}

func (e *KindError) Error() string {
	return fmt.Sprintf("kind error during %s: expected %s, got %s", e.Operation, e.Expected, e.Actual)
}

// UnmarshalError represents errors that occur during JSON unmarshaling
// of Thing data into specific types.
type UnmarshalError struct {
	ThingKind string // The kind of Thing being unmarshaled (e.g., "Listing", "Post", "Comment")
	Operation string // The operation being performed (e.g., "unmarshal_listing", "unmarshal_replies")
	Err       error  // The underlying error
}

func (e *UnmarshalError) Error() string {
	return fmt.Sprintf("unmarshal error for %s during %s: %v", e.ThingKind, e.Operation, e.Err)
}

func (e *UnmarshalError) Unwrap() error {
	return e.Err
}

// ValidationError represents errors that occur when validating parsed data,
// such as invalid fullnames, invalid field values, or data structure violations.
type ValidationError struct {
	ThingKind string // The kind of Thing being validated (e.g., "Post", "Comment", "Listing")
	Field     string // The field that failed validation (e.g., "AfterFullname", "ID")
	Value     string // The invalid value (if applicable, empty if not available)
	Err       error  // The underlying validation error (if applicable)
}

func (e *ValidationError) Error() string {
	var msg string
	if e.Value != "" {
		msg = fmt.Sprintf("validation error for %s.%s with value '%s'", e.ThingKind, e.Field, e.Value)
	} else {
		msg = fmt.Sprintf("validation error for %s.%s", e.ThingKind, e.Field)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}

	return msg
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// DepthError represents errors that occur when the comment tree depth
// exceeds the maximum allowed depth (to prevent stack overflow attacks).
type DepthError struct {
	CurrentDepth int // The current depth when the error occurred
	MaxDepth     int // The maximum allowed depth
}

func (e *DepthError) Error() string {
	return fmt.Sprintf("depth error: comment tree depth %d exceeds maximum of %d", e.CurrentDepth, e.MaxDepth)
}

// ExtractionError represents errors that occur during high-level extraction
// operations, such as extracting posts, comments, or post-and-comments from responses.
type ExtractionError struct {
	Operation string // The operation being performed (e.g., "extract_posts", "extract_comments")
	Context   string // Additional context about the error (if applicable)
	Err       error  // The underlying error (if applicable)
}

func (e *ExtractionError) Error() string {
	var msg string
	if e.Context != "" {
		msg = fmt.Sprintf("extraction error during %s (%s)", e.Operation, e.Context)
	} else {
		msg = fmt.Sprintf("extraction error during %s", e.Operation)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}

	return msg
}

func (e *ExtractionError) Unwrap() error {
	return e.Err
}
