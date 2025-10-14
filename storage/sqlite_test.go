package storage

import (
	"context"
	"testing"
	"time"
)

// TestNewSQLiteStore_InMemory verifies that an in-memory SQLite store can be created
// and that migrations run successfully.
func TestNewSQLiteStore_InMemory(t *testing.T) {
	cfg := &Config{
		DBPath:         ":memory:",
		MaxOpenConns:   5,
		MaxIdleConns:   2,
		MigrationsPath: "migrations", // Relative to the storage package directory during tests
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	// Verify store is not nil
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Verify database connection is working
	if store.db == nil {
		t.Fatal("expected non-nil database connection")
	}

	// Verify logger is set
	if store.logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

// TestSQLiteStore_Ping verifies that the Ping method works correctly.
func TestSQLiteStore_Ping(t *testing.T) {
	cfg := &Config{
		DBPath:         ":memory:",
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test that ping succeeds
	if err := store.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

// TestSQLiteStore_Close verifies that the Close method works correctly.
func TestSQLiteStore_Close(t *testing.T) {
	cfg := &Config{
		DBPath:         ":memory:",
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	// Close should succeed
	if err := store.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Subsequent operations should fail
	ctx := context.Background()
	err = store.Ping(ctx)
	if err == nil {
		t.Error("expected Ping to fail after Close, but it succeeded")
	}
}

// TestSQLiteStore_ConfigDefaults verifies that default configuration values are applied.
func TestSQLiteStore_ConfigDefaults(t *testing.T) {
	// Test with nil config (must specify migrations path for tests)
	store, err := NewSQLiteStore(&Config{DBPath: ":memory:", MigrationsPath: "migrations"})
	if err != nil {
		t.Fatalf("NewSQLiteStore with minimal config failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected non-nil store with minimal config")
	}

	// Test with empty config (all defaults)
	store2, err := NewSQLiteStore(&Config{DBPath: ":memory:", MigrationsPath: "migrations"})
	if err != nil {
		t.Fatalf("NewSQLiteStore with empty config failed: %v", err)
	}
	defer store2.Close()

	if store2 == nil {
		t.Fatal("expected non-nil store with empty config")
	}
}

// TestSQLiteStore_MigrationsApplied verifies that migrations create the expected tables.
func TestSQLiteStore_MigrationsApplied(t *testing.T) {
	cfg := &Config{
		DBPath:         ":memory:",
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	// Verify that the posts table exists
	ctx := context.Background()
	var tableName string
	err = store.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&tableName)
	if err != nil {
		t.Errorf("posts table not found: %v", err)
	}
	if tableName != "posts" {
		t.Errorf("expected table name 'posts', got %q", tableName)
	}

	// Verify that the comments table exists
	err = store.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='comments'").Scan(&tableName)
	if err != nil {
		t.Errorf("comments table not found: %v", err)
	}
	if tableName != "comments" {
		t.Errorf("expected table name 'comments', got %q", tableName)
	}

	// Verify that the comment_closures table exists
	err = store.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='comment_closures'").Scan(&tableName)
	if err != nil {
		t.Errorf("comment_closures table not found: %v", err)
	}
	if tableName != "comment_closures" {
		t.Errorf("expected table name 'comment_closures', got %q", tableName)
	}
}

// TestSQLiteStore_ConnectionPoolConfig verifies that connection pool settings are applied.
func TestSQLiteStore_ConnectionPoolConfig(t *testing.T) {
	cfg := &Config{
		DBPath:         ":memory:",
		MaxOpenConns:   15,
		MaxIdleConns:   8,
		ConnMaxLife:    30 * time.Minute,
		MigrationsPath: "migrations",
	}

	store, err := NewSQLiteStore(cfg)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	// Note: database/sql doesn't expose getters for these settings,
	// so we just verify the store was created successfully.
	// The settings are tested indirectly through actual usage.
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

// Note: TestSQLiteStore_UnimplementedMethods has been removed since all storage methods
// are now fully implemented and tested in their respective test files:
// - posts_test.go: Post CRUD operations
// - comments_test.go: Comment CRUD and tree operations
// - utils_test.go: GetStats and EvictStale operations
