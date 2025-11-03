package cache

import "fmt"

// CacheError represents errors that occur during cache operations.
type CacheError struct {
	Operation string // The operation being performed ("get", "set", "invalidate")
	Path      string // File path (if applicable, e.g., for persistent caches)
	Err       error  // The underlying error
}

// Error implements the error interface.
func (e *CacheError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("cache %s failed for %s: %v", e.Operation, e.Path, e.Err)
	}
	return fmt.Sprintf("cache %s failed: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying error for error chain inspection.
func (e *CacheError) Unwrap() error {
	return e.Err
}
