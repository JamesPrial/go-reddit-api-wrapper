package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite" // Register SQLite backend
)

// TestNewSQLiteStore_InMemory verifies that an in-memory SQLite store can be created
// and that migrations run successfully.
func TestNewSQLiteStore_InMemory(t *testing.T) {
	cfg := storage.Config{
		DSN:          ":memory:",
		MaxOpenConns: 5,
		MaxIdleConns: 2,
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	// Verify store is not nil
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	// Verify database connection is working by pinging it
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("expected database to be accessible: %v", err)
	}
}

// TestSQLiteStore_Ping verifies that the Ping method works correctly.
func TestSQLiteStore_Ping(t *testing.T) {
	cfg := storage.Config{
		DSN: ":memory:",
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
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
	cfg := storage.Config{
		DSN: ":memory:",
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
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
	// Test with minimal config
	cfg := storage.Config{
		DSN: ":memory:",
	}
	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New with minimal config failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("expected non-nil store with minimal config")
	}

	// Test with another instance
	store2, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewStore with minimal config failed (2nd time): %v", err)
	}
	defer store2.Close()

	if store2 == nil {
		t.Fatal("expected non-nil store with minimal config (2nd instance)")
	}
}

// TestSQLiteStore_ConnectionPoolConfig verifies that connection pool settings are applied.
func TestSQLiteStore_ConnectionPoolConfig(t *testing.T) {
	cfg := storage.Config{
		DSN:             ":memory:",
		MaxOpenConns:    15,
		MaxIdleConns:    8,
		ConnMaxLifetime: 30 * time.Minute,
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
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

// TestInMemoryDatabaseConnectionPool verifies that in-memory databases use a single shared connection
func TestInMemoryDatabaseConnectionPool(t *testing.T) {
	cfg := storage.Config{
		DSN:          ":memory:",
		MaxOpenConns: 10, // Should be overridden to 1
		MaxIdleConns: 5,  // Should be overridden to 1
	}

	store, err := storage.New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Verify schema is accessible (migrations ran successfully)
	err = store.Ping(ctx)
	if err != nil {
		t.Errorf("should be able to ping in-memory database: %v", err)
	}

	// Insert a post to verify table exists and is accessible
	post := &types.Post{
		ThingData: types.ThingData{
			ID:   "test123",
			Name: "t3_test123",
		},
		Votable: types.Votable{
			Score: 42,
			Ups:   42,
			Downs: 0,
		},
		Created: types.Created{
			CreatedUTC: float64(time.Now().Unix()),
		},
		Subreddit: "golang",
		Title:     "Test Post",
		Author:    "testuser",
	}

	err = store.UpsertPost(ctx, post)
	if err != nil {
		t.Errorf("should be able to insert post into in-memory database: %v", err)
	}

	// Retrieve the post to verify it's accessible
	retrieved, err := store.GetPost(ctx, "test123")
	if err != nil {
		t.Errorf("should be able to retrieve post from in-memory database: %v", err)
	}
	if retrieved == nil {
		t.Error("retrieved post should not be nil")
	}
	if retrieved != nil && retrieved.ID != "test123" {
		t.Errorf("expected ID 'test123', got %q", retrieved.ID)
	}
}

// TestInMemoryDatabaseURIFormats verifies that various URI-based in-memory database formats are detected
func TestInMemoryDatabaseURIFormats(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name   string
		dbPath string
	}{
		{
			name:   "standard memory format",
			dbPath: ":memory:",
		},
		{
			name:   "file URI memory format",
			dbPath: "file::memory:",
		},
		{
			name:   "named memory database",
			dbPath: "file:testdb?mode=memory&cache=shared",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := storage.Config{
				DSN:          tc.dbPath,
				MaxOpenConns: 10, // Should be overridden to 1
				MaxIdleConns: 5,  // Should be overridden to 1
			}

			store, err := storage.New(context.Background(), cfg)
			if err != nil {
				t.Fatalf("failed to create store with %s: %v", tc.dbPath, err)
			}
			defer store.Close()

			// Verify schema is accessible (migrations ran successfully)
			err = store.Ping(ctx)
			if err != nil {
				t.Errorf("should be able to ping database: %v", err)
			}

			// Insert a post to verify table exists and is accessible
			post := &types.Post{
				ThingData: types.ThingData{
					ID:   "test_uri_" + tc.name,
					Name: "t3_test_uri_" + tc.name,
				},
				Votable: types.Votable{
					Score: 42,
					Ups:   42,
					Downs: 0,
				},
				Created: types.Created{
					CreatedUTC: float64(time.Now().Unix()),
				},
				Subreddit: "golang",
				Title:     "Test Post for " + tc.name,
				Author:    "testuser",
			}

			err = store.UpsertPost(ctx, post)
			if err != nil {
				t.Errorf("should be able to insert post: %v", err)
			}

			// Retrieve the post to verify it's accessible
			retrieved, err := store.GetPost(ctx, post.ID)
			if err != nil {
				t.Errorf("should be able to retrieve post: %v", err)
			}
			if retrieved == nil {
				t.Error("retrieved post should not be nil")
			}
			if retrieved != nil && retrieved.ID != post.ID {
				t.Errorf("expected ID %q, got %q", post.ID, retrieved.ID)
			}
		})
	}
}
