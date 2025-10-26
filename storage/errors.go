package storage

import (
	"fmt"
)

// NotFoundError indicates that a requested resource (post or comment) does not exist
// in the storage system. This error is returned when attempting to retrieve, update,
// or delete a resource by ID that cannot be found in the database.
type NotFoundError struct {
	// ResourceType specifies the type of resource that was not found ("post" or "comment")
	ResourceType string
	// ResourceID is the ID of the resource that was not found
	ResourceID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found in storage", e.ResourceType, e.ResourceID)
}

// ValidationError represents errors that occur during input validation in storage operations.
// This includes validation of nil inputs, empty IDs, invalid formats, and missing required fields
// before attempting to store or retrieve data.
type ValidationError struct {
	// Operation is the name of the storage operation being performed (e.g., "UpsertPost", "UpsertComments")
	Operation string
	// Field is the name of the field/parameter being validated (e.g., "post", "commentID", "author")
	Field string
	// Value is the invalid value (may be empty if the value shouldn't be logged for security reasons)
	Value string
	// Reason is a description of why validation failed
	Reason string
	// Err is the underlying error (if applicable)
	Err error
}

func (e *ValidationError) Error() string {
	var msg string

	// Build message based on whether Operation is set
	if e.Operation != "" {
		if e.Value != "" {
			msg = fmt.Sprintf("validation error in %s for field %q with value %q: %s", e.Operation, e.Field, e.Value, e.Reason)
		} else {
			msg = fmt.Sprintf("validation error in %s for field %q: %s", e.Operation, e.Field, e.Reason)
		}
	} else {
		if e.Value != "" {
			msg = fmt.Sprintf("validation error for field %q with value %q: %s", e.Field, e.Value, e.Reason)
		} else {
			msg = fmt.Sprintf("validation error for field %q: %s", e.Field, e.Reason)
		}
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// IntegrityError indicates a violation of database integrity constraints.
// This includes missing parent references, self-referential comments, orphaned comments,
// and other data consistency violations that would result in an invalid state.
type IntegrityError struct {
	// Operation is the name of the storage operation that caused the integrity violation
	Operation string
	// ResourceType specifies the type of resource involved in the integrity violation ("post" or "comment")
	ResourceType string
	// ResourceID is the ID of the resource involved in the integrity violation
	ResourceID string
	// Reason is a description of the integrity constraint violation
	Reason string
	// Err is the underlying error (if applicable)
	Err error
}

func (e *IntegrityError) Error() string {
	var msg string
	if e.Operation != "" {
		msg = fmt.Sprintf("integrity error in %s: %s %q %s", e.Operation, e.ResourceType, e.ResourceID, e.Reason)
	} else {
		msg = fmt.Sprintf("integrity error: %s %q %s", e.ResourceType, e.ResourceID, e.Reason)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *IntegrityError) Unwrap() error {
	return e.Err
}

// TransactionError represents errors that occur during database transaction operations.
// This includes failures to begin, commit, or rollback transactions, and can indicate
// database locking issues, connection failures, or other transaction-related problems.
type TransactionError struct {
	// Operation specifies the transaction operation that failed ("begin", "commit", or "rollback")
	Operation string
	// Message contains the detailed error message describing what went wrong
	Message string
	// Err is the underlying error from the database
	Err error
}

func (e *TransactionError) Error() string {
	var msg string
	if e.Operation != "" {
		msg = fmt.Sprintf("transaction error during %s", e.Operation)
	} else {
		msg = "transaction error"
	}

	if e.Message != "" {
		msg += fmt.Sprintf(": %s", e.Message)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *TransactionError) Unwrap() error {
	return e.Err
}

// DatabaseError represents errors that occur during database query execution or scanning.
// This includes SQL syntax errors, query execution failures, row scanning failures,
// and other low-level database operation errors.
type DatabaseError struct {
	// Operation is the name of the storage operation being performed (e.g., "ListPosts", "GetComment")
	Operation string
	// Query is the SQL query that failed (may be omitted for security reasons)
	Query string
	// Message contains the detailed error message
	Message string
	// Err is the underlying database error
	Err error
}

func (e *DatabaseError) Error() string {
	var msg string
	if e.Operation != "" {
		msg = fmt.Sprintf("database error during %s", e.Operation)
	} else {
		msg = "database error"
	}

	if e.Message != "" {
		msg += fmt.Sprintf(": %s", e.Message)
	}

	if e.Query != "" {
		msg += fmt.Sprintf(" (query: %s)", e.Query)
	}

	if e.Err != nil {
		msg += fmt.Sprintf(", err: %v", e.Err)
	}

	return msg
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// ConflictError indicates a duplicate resource conflict during an upsert operation.
// This error is used when a resource with the same ID already exists and the operation
// cannot be completed due to the duplicate. While SQLite handles conflicts with ON CONFLICT clauses,
// this error type can be used in future implementations or other storage backends.
type ConflictError struct {
	// ResourceType specifies the type of resource that has a conflict ("post" or "comment")
	ResourceType string
	// ResourceID is the ID of the resource that caused the conflict
	ResourceID string
	// Message contains additional context about the conflict
	Message string
}

func (e *ConflictError) Error() string {
	var msg string
	if e.Message != "" {
		msg = fmt.Sprintf("conflict: %s %q %s", e.ResourceType, e.ResourceID, e.Message)
	} else {
		msg = fmt.Sprintf("conflict: %s %q already exists", e.ResourceType, e.ResourceID)
	}
	return msg
}

// DriverError indicates an issue with storage driver registration or support.
// This includes cases where a driver is not registered (missing import of backend package)
// or when an unsupported driver name is provided.
type DriverError struct {
	// Driver is the name of the driver that caused the error
	Driver string
	// Backend is the name of the backend that caused the error
	Backend string
	// Reason describes why the driver error occurred
	Reason string
}

func (e *DriverError) Error() string {
	msg := fmt.Sprintf("driver error for %q", e.Driver)
	if e.Backend != "" {
		msg += fmt.Sprintf(" - using backend: %q", e.Backend)
	}
	if e.Reason != "" {
		msg += fmt.Sprintf(" - %s", e.Reason)
	}

	return msg
}
