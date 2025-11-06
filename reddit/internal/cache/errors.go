package cache

import "fmt"

// CacheError represents errors that occur during cache operations.
type CacheError struct {
	Operation string // The operation being performed ("get", "set", "invalidate")
	Path      string // File path (if applicable, e.g., for persistent caches)
	Err       error  // The underlying error
	RequestID string // Request ID for tracing (empty if not available)
}

// Error implements the error interface.
func (e *CacheError) Error() string {
	var msg string
	if e.Path != "" {
		msg = fmt.Sprintf("cache %s failed for %s: %v", e.Operation, e.Path, e.Err)
	} else {
		msg = fmt.Sprintf("cache %s failed: %v", e.Operation, e.Err)
	}
	if e.RequestID != "" {
		msg += fmt.Sprintf(" (request_id: %s)", e.RequestID)
	}
	return msg
}

// Unwrap returns the underlying error for error chain inspection.
func (e *CacheError) Unwrap() error {
	return e.Err
}
