/*
Package storage provides an abstraction layer for persisting Reddit API data
to various database backends.

# Architecture

The storage package defines interfaces that all storage backends must implement:

  - Store: Composite interface of PostOperations, CommentOperations, and UtilityOperations
  - PostOperations: CRUD operations for Reddit posts
  - CommentOperations: CRUD operations for Reddit comments with hierarchical support
  - UtilityOperations: Database management (health checks, statistics, eviction)

# Supported Backends

Currently supported storage backends:

  - SQLite (storage/sqlite): Lightweight, file-based or in-memory storage
  - PostgreSQL (storage/postgres): Enterprise-grade relational database (stub implementation)

# Usage

There are two ways to create a storage backend:

## Factory Pattern (Recommended)

Use the factory function to automatically select the backend based on configuration.
Note: You must import the desired backend subpackage (even with blank import) to register it:

	import (
		"github.com/jamesprial/go-reddit-api-wrapper/storage"
		_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"  // Register SQLite backend
	)

	cfg := storage.Config{
		Driver: "sqlite",  // or "sqlite3"
		DSN:    ":memory:", // or "/path/to/file.db"
		Logger: slog.Default(),
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	// Use the store
	post, err := store.GetPost(context.Background(), "abc123")

## Direct Import (Explicit Control)

Import a specific backend package directly for explicit control:

	import (
		"github.com/jamesprial/go-reddit-api-wrapper/storage"
		"github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
	)

	cfg := storage.Config{
		DSN:    "reddit.db",
		Logger: slog.Default(),
	}

	store, err := sqlite.NewStore(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

# Error Handling

The package defines typed errors for different failure scenarios:

  - NotFoundError: Resource not in storage
  - ValidationError: Invalid input
  - IntegrityError: Database constraint violation
  - TransactionError: Transaction failure
  - DatabaseError: Low-level database error
  - ConflictError: Duplicate resource

Use errors.As() to check for specific error types:

	var notFoundErr *storage.NotFoundError
	post, err := store.GetPost(ctx, postID)
	if errors.As(err, &notFoundErr) {
		// Handle not found - post doesn't exist in storage
		log.Printf("post %s not found", notFoundErr.ResourceID)
	}

# Features

  - Thread-safe concurrent access to storage
  - Transaction support for batch operations
  - Comment tree reconstruction with parent-child relationships
  - Pagination and filtering for queries
  - Idempotent operations (upsert pattern)
  - Stale data eviction with configurable time thresholds
  - Database statistics and monitoring

# Conversion Utilities

The package provides database-agnostic conversion utilities for working with nullable SQL types:

  - stringToNullString / nullStringToString
  - int64ToNullInt64 / nullInt64ToInt64
  - intToNullInt64 / nullInt64ToInt
  - boolToNullBool / nullBoolToBool
  - float64ToNullFloat64 / nullFloat64ToFloat64
  - timeToNullTime / nullTimeToTime

These utilities handle the conversion between Go types and SQL nullable types,
treating zero values as NULL in the database.

# Backend Implementation Notes

Backend implementations in subpackages (storage/sqlite, storage/postgres) must:

 1. Implement the Store interface
 2. Provide a NewStore(ctx context.Context, cfg storage.Config) (storage.Store, error) constructor
    that accepts storage.Config and adapts it to backend-specific needs
 3. Translate internal errors to storage package error types
 4. Handle migrations for schema versioning
 5. Support graceful shutdown via Close()

See subpackage documentation for backend-specific details.
*/
package storage
