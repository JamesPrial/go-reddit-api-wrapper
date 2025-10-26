package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// getMigrationsPath returns the absolute path to the migrations directory
func getMigrationsPath(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	// Navigate from project root to migrations directory
	projectRoot := cwd
	for projectRoot != "/" && !fileExists(filepath.Join(projectRoot, "storage")) {
		projectRoot = filepath.Dir(projectRoot)
	}
	return filepath.Join(projectRoot, "storage", "sqlite", "migrations")
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TestSQLiteStoreImplementsInterface verifies that SQLite store implements storage.Store interface.
func TestSQLiteStoreImplementsInterface(t *testing.T) {
	cfg := storage.Config{
		Driver:         "sqlite",
		DSN:            ":memory:",
		MigrationsPath: getMigrationsPath(t),
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
		Driver:         "sqlite",
		DSN:            ":memory:",
		MigrationsPath: getMigrationsPath(t),
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
