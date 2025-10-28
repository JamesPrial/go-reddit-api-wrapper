package storage_test

import (
	"context"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// TestSQLiteStoreImplementsInterface verifies that SQLite store implements storage.Store interface.
func TestSQLiteStoreImplementsInterface(t *testing.T) {
	cfg := storage.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Verify store implements the Store interface
	var _ storage.Store = store
}

// TestStorageFactoryCreatesImplementer verifies that storage.New returns a valid Store implementation.
func TestStorageFactoryCreatesImplementer(t *testing.T) {
	cfg := storage.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create store via factory: %v", err)
	}
	defer store.Close()

	// Verify store implements the Store interface
	var _ storage.Store = store

	// Verify basic operations work
	if err := store.Ping(context.Background()); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}
