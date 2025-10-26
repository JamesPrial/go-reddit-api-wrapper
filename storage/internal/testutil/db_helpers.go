package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// NewInMemoryDB creates an in-memory SQLite database for testing.
// It automatically determines the migrations path and registers cleanup with t.Cleanup().
// The database is ready to use immediately with all migrations applied.
// Fails the test if initialization fails.
func NewInMemoryDB(t *testing.T) storage.Store {
	t.Helper()

	cfg := storage.Config{
		DSN:            ":memory:",
		MigrationsPath: GetMigrationsPath(),
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create in-memory database: %v", err)
	}

	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close in-memory database: %v", err)
		}
	})

	return store
}

// NewFileBasedDB creates a temporary file-based SQLite database for testing.
// It uses t.TempDir() for the database file location and sets appropriate DSN with WAL mode pragmas.
// The database is automatically cleaned up when the test completes.
// Fails the test if initialization fails.
func NewFileBasedDB(t *testing.T) storage.Store {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := storage.Config{
		DSN:            dbPath,
		MigrationsPath: GetMigrationsPath(),
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create file-based database: %v", err)
	}

	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Logf("failed to close file-based database: %v", err)
		}
	})

	return store
}

// SeedDatabase populates the database with test posts and comments.
// It upserts all posts first, then all comments.
// Fails the test if any operation fails.
func SeedDatabase(t *testing.T, store storage.Store, posts []*types.Post, comments []*types.Comment) {
	t.Helper()

	ctx := context.Background()

	// Upsert all posts
	if len(posts) > 0 {
		if err := store.UpsertPosts(ctx, posts); err != nil {
			t.Fatalf("failed to upsert posts: %v", err)
		}
	}

	// Upsert all comments
	if len(comments) > 0 {
		if err := store.UpsertComments(ctx, comments); err != nil {
			t.Fatalf("failed to upsert comments: %v", err)
		}
	}
}

// AssertRowCount verifies that a table contains the expected number of rows.
// It uses direct SQL query to count rows, which is useful for verifying data was actually stored.
// Fails the test if the row count doesn't match or if the query fails.
func AssertRowCount(t *testing.T, store storage.Store, table string, expected int64) {
	t.Helper()

	// Type assert to SQLiteStore to access the underlying database
	sqliteStore, ok := store.(*sqlite.SQLiteStore)
	if !ok {
		t.Fatalf("store is not a *sqlite.SQLiteStore, got %T", store)
	}

	// Get the underlying database connection using the testing helper
	db := sqlite.GetDB(sqliteStore)

	ctx := context.Background()
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count rows in %s: %v", table, err)
	}

	if count != expected {
		t.Errorf("expected %d rows in %s, got %d", expected, table, count)
	}
}

// GetMigrationsPath returns the absolute path to the migrations directory.
// It searches upward from the current working directory to find the storage/sqlite/migrations directory.
// Panics with a helpful message if the migrations directory cannot be found.
func GetMigrationsPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic("failed to get current working directory: " + err.Error())
	}

	// Search upward from the current directory until we find the storage directory
	projectRoot := cwd
	for projectRoot != "/" {
		migrationsPath := filepath.Join(projectRoot, "storage", "sqlite", "migrations")
		if fileExists(migrationsPath) {
			return migrationsPath
		}
		projectRoot = filepath.Dir(projectRoot)
	}

	panic("could not find storage/sqlite/migrations directory. " +
		"Make sure you're running tests from within the project repository.")
}

// fileExists checks if a file or directory exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
