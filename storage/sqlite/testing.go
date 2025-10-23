package sqlite

import (
	"context"
	"database/sql"
)

// testingDB is only used by tests in the sqlite_test package to access unexported fields.
// It allows tests to query internal database state.

// GetDB returns the underlying database connection.
// This is exported for testing purposes only and should not be used in production code.
func GetDB(s *SQLiteStore) *sql.DB {
	return s.db
}

// QueryRowContext executes a query that returns at most one row using the store's database.
// This is exported for testing purposes only.
func QueryRowContext(s *SQLiteStore, ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// QueryContext executes a query using the store's database.
// This is exported for testing purposes only.
func QueryContext(s *SQLiteStore, ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

// ExecContext executes a statement using the store's database.
// This is exported for testing purposes only.
func ExecContext(s *SQLiteStore, ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}
