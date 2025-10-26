package storage_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/stretchr/testify/require"
)

// TestErrors_NotFoundErrorImplementsError verifies NotFoundError implements the error interface
// and produces expected error messages.
func TestErrors_NotFoundErrorImplementsError(t *testing.T) {
	err := &storage.NotFoundError{
		ResourceType: "post",
		ResourceID:   "abc123",
	}

	// Verify it implements the error interface
	var _ error = err
	require.Error(t, err)

	// Verify Error() returns expected format
	expectedMsg := `post "abc123" not found in storage`
	require.Equal(t, expectedMsg, err.Error())
}

// TestErrors_ValidationErrorImplementsError verifies ValidationError implements the error interface
// and produces expected error messages.
func TestErrors_ValidationErrorImplementsError(t *testing.T) {
	err := &storage.ValidationError{
		Operation: "UpsertPost",
		Field:     "post",
		Reason:    "cannot be nil",
	}

	// Verify it implements the error interface
	var _ error = err
	require.Error(t, err)

	// Verify Error() message format
	expectedMsg := `validation error in UpsertPost for field "post": cannot be nil`
	require.Equal(t, expectedMsg, err.Error())
}

// TestErrors_AllErrorTypesImplementError uses table-driven test to verify all 6 error types
// implement the error interface.
func TestErrors_AllErrorTypesImplementError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "NotFoundError",
			err: &storage.NotFoundError{
				ResourceType: "post",
				ResourceID:   "123",
			},
		},
		{
			name: "ValidationError",
			err: &storage.ValidationError{
				Field:  "post",
				Reason: "invalid",
			},
		},
		{
			name: "IntegrityError",
			err: &storage.IntegrityError{
				ResourceType: "comment",
				ResourceID:   "456",
				Reason:       "parent not found",
			},
		},
		{
			name: "TransactionError",
			err: &storage.TransactionError{
				Operation: "commit",
				Message:   "database locked",
			},
		},
		{
			name: "DatabaseError",
			err: &storage.DatabaseError{
				Operation: "GetPost",
				Message:   "query failed",
			},
		},
		{
			name: "ConflictError",
			err: &storage.ConflictError{
				ResourceType: "post",
				ResourceID:   "789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All should implement error interface
			require.Error(t, tt.err)
			// Error() should return a non-empty string
			require.NotEmpty(t, tt.err.Error())
		})
	}
}

// TestErrors_ErrorWrapping verifies error wrapping with Unwrap(), errors.Is(), and errors.As().
func TestErrors_ErrorWrapping(t *testing.T) {
	// Create error chain: DatabaseError wrapping sql.ErrNoRows
	underlyingErr := sql.ErrNoRows
	dbErr := &storage.DatabaseError{
		Operation: "GetPost",
		Err:       underlyingErr,
	}

	// Verify Unwrap() returns underlying error
	require.Equal(t, underlyingErr, dbErr.Unwrap())
	require.Equal(t, sql.ErrNoRows, dbErr.Unwrap())

	// Verify errors.Is() works with wrapped errors
	require.True(t, errors.Is(dbErr, sql.ErrNoRows))

	// Verify errors.As() works to extract specific error type
	var extracted *storage.DatabaseError
	require.True(t, errors.As(dbErr, &extracted))
	require.Equal(t, dbErr, extracted)
}

// TestErrors_NotFoundErrorFields verifies NotFoundError fields are properly included
// in the error message and accessible.
func TestErrors_NotFoundErrorFields(t *testing.T) {
	err := &storage.NotFoundError{
		ResourceType: "comment",
		ResourceID:   "xyz789",
	}

	// Verify fields are accessible
	require.Equal(t, "comment", err.ResourceType)
	require.Equal(t, "xyz789", err.ResourceID)

	// Verify Error() message includes both fields
	errMsg := err.Error()
	require.Contains(t, errMsg, "comment")
	require.Contains(t, errMsg, "xyz789")
	require.Contains(t, errMsg, "not found")
}

// TestErrors_ValidationErrorFields verifies ValidationError fields are properly included
// and accessible.
func TestErrors_ValidationErrorFields(t *testing.T) {
	err := &storage.ValidationError{
		Operation: "UpsertComments",
		Field:     "commentID",
		Value:     "",
		Reason:    "cannot be empty",
		Err:       nil,
	}

	// Verify fields are accessible
	require.Equal(t, "UpsertComments", err.Operation)
	require.Equal(t, "commentID", err.Field)
	require.Equal(t, "", err.Value)
	require.Equal(t, "cannot be empty", err.Reason)
	require.Nil(t, err.Err)

	// Verify Error() includes both Operation and Field
	errMsg := err.Error()
	require.Contains(t, errMsg, "UpsertComments")
	require.Contains(t, errMsg, "commentID")
	require.Contains(t, errMsg, "cannot be empty")
}

// TestErrors_DatabaseErrorFields verifies DatabaseError fields are properly included
// in the error message and Unwrap() returns wrapped error.
func TestErrors_DatabaseErrorFields(t *testing.T) {
	underlyingErr := errors.New("connection timeout")
	dbErr := &storage.DatabaseError{
		Operation: "ListPosts",
		Query:     "SELECT * FROM posts",
		Message:   "timeout occurred",
		Err:       underlyingErr,
	}

	// Verify fields are accessible
	require.Equal(t, "ListPosts", dbErr.Operation)
	require.Equal(t, "SELECT * FROM posts", dbErr.Query)
	require.Equal(t, "timeout occurred", dbErr.Message)
	require.Equal(t, underlyingErr, dbErr.Err)

	// Verify all fields in Error() message
	errMsg := dbErr.Error()
	require.Contains(t, errMsg, "ListPosts")
	require.Contains(t, errMsg, "timeout occurred")
	require.Contains(t, errMsg, "SELECT * FROM posts")
	require.Contains(t, errMsg, "connection timeout")

	// Verify Unwrap() returns the wrapped Err
	require.Equal(t, underlyingErr, dbErr.Unwrap())
}

// TestErrors_ErrorComparison verifies error comparison and errors.Is() behavior.
func TestErrors_ErrorComparison(t *testing.T) {
	// Verify that NotFoundError instances with same fields compare equal by value
	err1 := &storage.NotFoundError{
		ResourceType: "post",
		ResourceID:   "abc123",
	}

	err2 := &storage.NotFoundError{
		ResourceType: "post",
		ResourceID:   "abc123",
	}

	// Go struct comparison is value-based, not pointer-based
	require.Equal(t, err1, err2)

	// Verify they produce identical error messages
	require.Equal(t, err1.Error(), err2.Error())

	// errors.Is() should work with wrapped errors
	wrappedErr := &storage.DatabaseError{
		Err: sql.ErrConnDone,
	}

	require.True(t, errors.Is(wrappedErr, sql.ErrConnDone))
	require.False(t, errors.Is(wrappedErr, sql.ErrNoRows))
}

// TestErrors_ErrorMessageConsistency verifies all error types produce consistent
// and well-formatted error messages.
func TestErrors_ErrorMessageConsistency(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		shouldCheck func(t *testing.T, msg string)
	}{
		{
			name: "NotFoundError has consistent format",
			err: &storage.NotFoundError{
				ResourceType: "post",
				ResourceID:   "id1",
			},
			shouldCheck: func(t *testing.T, msg string) {
				require.Contains(t, msg, `"id1"`)
				require.Contains(t, msg, "not found")
			},
		},
		{
			name: "ValidationError has consistent format",
			err: &storage.ValidationError{
				Operation: "Test",
				Field:     "field1",
				Reason:    "reason1",
			},
			shouldCheck: func(t *testing.T, msg string) {
				require.Contains(t, msg, "validation error")
				require.Contains(t, msg, "field1")
				require.Contains(t, msg, "reason1")
			},
		},
		{
			name: "IntegrityError has consistent format",
			err: &storage.IntegrityError{
				Operation:    "Test",
				ResourceType: "post",
				ResourceID:   "id1",
				Reason:       "invalid",
			},
			shouldCheck: func(t *testing.T, msg string) {
				require.Contains(t, msg, "integrity error")
				require.Contains(t, msg, "post")
				require.Contains(t, msg, `"id1"`)
			},
		},
		{
			name: "TransactionError has consistent format",
			err: &storage.TransactionError{
				Operation: "begin",
				Message:   "locked",
			},
			shouldCheck: func(t *testing.T, msg string) {
				require.Contains(t, msg, "transaction error")
				require.Contains(t, msg, "begin")
				require.Contains(t, msg, "locked")
			},
		},
		{
			name: "DatabaseError has consistent format",
			err: &storage.DatabaseError{
				Operation: "Test",
				Message:   "failed",
			},
			shouldCheck: func(t *testing.T, msg string) {
				require.Contains(t, msg, "database error")
				require.Contains(t, msg, "Test")
				require.Contains(t, msg, "failed")
			},
		},
		{
			name: "ConflictError has consistent format",
			err: &storage.ConflictError{
				ResourceType: "post",
				ResourceID:   "id1",
				Message:      "already exists",
			},
			shouldCheck: func(t *testing.T, msg string) {
				require.Contains(t, msg, "conflict")
				require.Contains(t, msg, "post")
				require.Contains(t, msg, "already exists")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			require.NotEmpty(t, msg)
			tt.shouldCheck(t, msg)
		})
	}
}

// TestErrors_NilUnwrap verifies that Unwrap() returns nil when no underlying error is set.
func TestErrors_NilUnwrap(t *testing.T) {
	tests := []struct {
		name string
		err  interface {
			error
			Unwrap() error
		}
	}{
		{
			name: "ValidationError with nil Err",
			err: &storage.ValidationError{
				Field:  "test",
				Reason: "test",
				Err:    nil,
			},
		},
		{
			name: "IntegrityError with nil Err",
			err: &storage.IntegrityError{
				ResourceType: "post",
				ResourceID:   "1",
				Reason:       "test",
				Err:          nil,
			},
		},
		{
			name: "TransactionError with nil Err",
			err: &storage.TransactionError{
				Operation: "test",
				Err:       nil,
			},
		},
		{
			name: "DatabaseError with nil Err",
			err: &storage.DatabaseError{
				Operation: "test",
				Err:       nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			unwrapped := tt.err.Unwrap()
			require.Nil(t, unwrapped)
		})
	}
}

// TestErrors_ValidationErrorWithValue verifies ValidationError message formats
// correctly when Value field is populated.
func TestErrors_ValidationErrorWithValue(t *testing.T) {
	err := &storage.ValidationError{
		Operation: "UpsertPost",
		Field:     "author",
		Value:     "invalid@value",
		Reason:    "invalid format",
	}

	msg := err.Error()
	require.Contains(t, msg, "invalid@value")
	require.Contains(t, msg, "UpsertPost")
	require.Contains(t, msg, "author")
	require.Contains(t, msg, "invalid format")
}

// TestErrors_ValidationErrorWithoutOperation verifies ValidationError message format
// when Operation field is empty.
func TestErrors_ValidationErrorWithoutOperation(t *testing.T) {
	err := &storage.ValidationError{
		Field:  "commentID",
		Reason: "required",
	}

	msg := err.Error()
	require.Contains(t, msg, "commentID")
	require.Contains(t, msg, "required")
	require.NotContains(t, msg, "in ")
}

// TestErrors_IntegrityErrorWithoutOperation verifies IntegrityError message format
// when Operation field is empty.
func TestErrors_IntegrityErrorWithoutOperation(t *testing.T) {
	err := &storage.IntegrityError{
		ResourceType: "comment",
		ResourceID:   "xyz",
		Reason:       "parent post not found",
	}

	msg := err.Error()
	require.Contains(t, msg, "comment")
	require.Contains(t, msg, "xyz")
	require.Contains(t, msg, "parent post not found")
	require.NotContains(t, msg, "in ")
}

// TestErrors_TransactionErrorWithoutOperation verifies TransactionError message format
// when Operation field is empty.
func TestErrors_TransactionErrorWithoutOperation(t *testing.T) {
	err := &storage.TransactionError{
		Message: "connection lost",
	}

	msg := err.Error()
	require.Equal(t, "transaction error: connection lost", msg)
}

// TestErrors_DatabaseErrorWithoutOperation verifies DatabaseError message format
// when Operation field is empty.
func TestErrors_DatabaseErrorWithoutOperation(t *testing.T) {
	err := &storage.DatabaseError{
		Message: "constraint violation",
	}

	msg := err.Error()
	require.Equal(t, "database error: constraint violation", msg)
}

// TestErrors_ConflictErrorWithoutMessage verifies ConflictError message format
// when Message field is empty.
func TestErrors_ConflictErrorWithoutMessage(t *testing.T) {
	err := &storage.ConflictError{
		ResourceType: "post",
		ResourceID:   "dup123",
	}

	msg := err.Error()
	require.Contains(t, msg, "post")
	require.Contains(t, msg, "dup123")
	require.Contains(t, msg, "already exists")
}

// TestErrors_ComplexErrorChain verifies error chains with multiple levels of wrapping.
func TestErrors_ComplexErrorChain(t *testing.T) {
	// Create a chain: DatabaseError -> TransactionError -> sql.ErrNoRows
	baseErr := sql.ErrNoRows
	transErr := &storage.TransactionError{
		Operation: "rollback",
		Err:       baseErr,
	}
	dbErr := &storage.DatabaseError{
		Operation: "DeletePost",
		Err:       transErr,
	}

	// Verify errors.Is() can find baseErr through the chain
	require.True(t, errors.Is(dbErr, sql.ErrNoRows))

	// Verify errors.As() can extract TransactionError
	var extracted *storage.TransactionError
	require.True(t, errors.As(dbErr, &extracted))
	require.Equal(t, transErr, extracted)

	// Verify direct Unwrap() returns immediate wrapper
	require.Equal(t, transErr, dbErr.Unwrap())
	require.Equal(t, baseErr, transErr.Unwrap())
}

// TestErrors_ValidationErrorWithUnderlying verifies ValidationError properly
// wraps and reports underlying errors.
func TestErrors_ValidationErrorWithUnderlying(t *testing.T) {
	underlying := errors.New("field validation failed")
	err := &storage.ValidationError{
		Operation: "UpsertPost",
		Field:     "score",
		Reason:    "must be positive",
		Err:       underlying,
	}

	// Verify message includes underlying error
	msg := err.Error()
	require.Contains(t, msg, "field validation failed")

	// Verify Unwrap() returns underlying error
	require.Equal(t, underlying, err.Unwrap())
	require.True(t, errors.Is(err, underlying))
}

// TestErrors_IntegrityErrorWithUnderlying verifies IntegrityError properly
// wraps and reports underlying errors.
func TestErrors_IntegrityErrorWithUnderlying(t *testing.T) {
	underlying := errors.New("foreign key constraint failed")
	err := &storage.IntegrityError{
		Operation:    "InsertComment",
		ResourceType: "comment",
		ResourceID:   "cmt123",
		Reason:       "parent post deleted",
		Err:          underlying,
	}

	// Verify message includes underlying error
	msg := err.Error()
	require.Contains(t, msg, "foreign key constraint failed")

	// Verify Unwrap() returns underlying error
	require.Equal(t, underlying, err.Unwrap())
	require.True(t, errors.Is(err, underlying))
}

// TestErrors_AllFieldsCombinations verifies error messages with various field combinations.
func TestErrors_AllFieldsCombinations(t *testing.T) {
	t.Run("DatabaseError with all fields", func(t *testing.T) {
		err := &storage.DatabaseError{
			Operation: "GetPost",
			Query:     "SELECT * FROM posts WHERE id = ?",
			Message:   "scan error",
			Err:       errors.New("invalid column type"),
		}
		msg := err.Error()
		require.Contains(t, msg, "GetPost")
		require.Contains(t, msg, "scan error")
		require.Contains(t, msg, "SELECT * FROM posts WHERE id = ?")
		require.Contains(t, msg, "invalid column type")
	})

	t.Run("DatabaseError with only Operation", func(t *testing.T) {
		err := &storage.DatabaseError{
			Operation: "GetPost",
		}
		msg := err.Error()
		require.Equal(t, "database error during GetPost", msg)
	})

	t.Run("DatabaseError with Operation and Query", func(t *testing.T) {
		err := &storage.DatabaseError{
			Operation: "GetPost",
			Query:     "SELECT * FROM posts",
		}
		msg := err.Error()
		require.Contains(t, msg, "GetPost")
		require.Contains(t, msg, "SELECT * FROM posts")
	})
}
