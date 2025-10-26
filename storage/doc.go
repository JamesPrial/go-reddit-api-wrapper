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

  - SQLite (storage/backends/sqlite): Lightweight, file-based or in-memory storage
  - PostgreSQL (storage/backends/postgres): Enterprise-grade relational database (stub implementation)

# Usage

There are two ways to create a storage backend:

## Factory Pattern (Recommended)

Use the factory function to automatically select the backend based on configuration.
Note: You must import the desired backend subpackage (even with blank import) to register it:

	import (
		"github.com/jamesprial/go-reddit-api-wrapper/storage"
		_ "github.com/jamesprial/go-reddit-api-wrapper/storage/backends/sqlite"  // Register SQLite backend
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

## Auto-Detection vs Explicit Driver

You can either specify the driver explicitly or let the factory auto-detect from the DSN pattern:

	import (
		"github.com/jamesprial/go-reddit-api-wrapper/storage"
		_ "github.com/jamesprial/go-reddit-api-wrapper/storage/backends/sqlite"
	)

	// Auto-detection (DSN pattern determines driver)
	cfg := storage.Config{
		DSN:    ":memory:",  // Detected as sqlite
		Logger: slog.Default(),
	}

	// Explicit driver (recommended for clarity)
	cfg = storage.Config{
		Driver: "sqlite",  // Explicitly specify driver
		DSN:    "reddit.db",
		Logger: slog.Default(),
	}

	store, err := storage.New(context.Background(), cfg)
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

# Internal Package

The storage/internal package provides internal implementation details that are shared
across storage backends but not exposed to external packages:

## Factory Registry

The factory pattern uses a generic registry (storage/internal.Registry) for type-safe
backend registration. Backends register themselves in their init() functions using
storage.RegisterFactory():

	// In storage/backends/sqlite/register.go
	func init() {
		storage.RegisterFactory("sqlite", newStoreFactory)
		storage.RegisterFactory("sqlite3", newStoreFactory)
	}

The registry is thread-safe and uses read-write mutexes to allow concurrent reads
while protecting writes during registration.

## Conversion Utilities

Database-agnostic conversion utilities for working with nullable SQL types:

  - StringToNullString / NullStringToString
  - Int64ToNullInt64 / NullInt64ToInt64
  - IntToNullInt64 / NullInt64ToInt
  - BoolToNullBool / NullBoolToBool
  - Float64ToNullFloat64 / NullFloat64ToFloat64
  - TimeToNullTime / NullTimeToTime

These utilities handle the conversion between Go types and SQL nullable types,
treating zero values as NULL in the database. Backend implementations can import
these converters from storage/internal to maintain consistency across backends.

# Backend Implementation Notes

Backend implementations are internal and registered transparently:

 1. Backends implement the Store interface in storage/internal/{driver}/ packages
 2. Public registration packages (storage/backends/{driver}/) handle factory registration
 3. Users import the public registration package to enable backend support
 4. Backend implementations translate internal errors to storage package error types
 5. Backends handle migrations for schema versioning
 6. All backends support graceful shutdown via Close()

See subpackage documentation for backend-specific details.
*/
package storage
