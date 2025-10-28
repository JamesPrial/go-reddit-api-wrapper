package sentiment

import "fmt"

// AnalysisError represents an error that occurs during sentiment analysis.
// It provides context about what operation failed and why.
type AnalysisError struct {
	// Operation is the name of the operation that failed (e.g., "analyze_post", "analyze_comment")
	Operation string
	// Message contains a detailed description of what went wrong
	Message string
	// Err is the underlying error, if any
	Err error
}

// Error returns a string representation of the analysis error.
func (e *AnalysisError) Error() string {
	if e.Operation != "" {
		if e.Err != nil {
			return fmt.Sprintf("analysis error during %s: %s, err: %v", e.Operation, e.Message, e.Err)
		}
		return fmt.Sprintf("analysis error during %s: %s", e.Operation, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("analysis error: %s, err: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("analysis error: %s", e.Message)
}

// Unwrap returns the underlying error, if any.
// This allows errors.As and errors.Is to work correctly with error chains.
func (e *AnalysisError) Unwrap() error {
	return e.Err
}

// NewAnalysisError creates a new AnalysisError with the given operation and message.
func NewAnalysisError(operation, message string) *AnalysisError {
	return &AnalysisError{
		Operation: operation,
		Message:   message,
		Err:       nil,
	}
}

// NewAnalysisErrorWithErr creates a new AnalysisError with the given operation,
// message, and underlying error.
func NewAnalysisErrorWithErr(operation, message string, err error) *AnalysisError {
	return &AnalysisError{
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

// NewValidationError creates a validation error for invalid input during analysis.
func NewValidationError(message string) *AnalysisError {
	return &AnalysisError{
		Operation: "validation",
		Message:   message,
		Err:       nil,
	}
}

// NewValidationErrorWithErr creates a validation error with an underlying error.
func NewValidationErrorWithErr(message string, err error) *AnalysisError {
	return &AnalysisError{
		Operation: "validation",
		Message:   message,
		Err:       err,
	}
}
