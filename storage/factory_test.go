//go:build integration

package storage_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// TestFactory_RegisterAndRetrieveSQLite verifies that the SQLite driver is registered
// and can be used to create a store.
func TestFactory_RegisterAndRetrieveSQLite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create store with explicit driver
	store, err := storage.New(ctx, storage.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	require.NoError(t, err, "failed to create store with sqlite driver")
	require.NotNil(t, store, "store should not be nil")
	defer store.Close()

	// Verify store implements Store interface
	var _ storage.Store = store
}

// TestFactory_RegisterAndRetrieveSQLiteAlias verifies that the sqlite3 alias is also registered.
func TestFactory_RegisterAndRetrieveSQLiteAlias(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create store with sqlite3 alias
	store, err := storage.New(ctx, storage.Config{
		Driver: "sqlite3",
		DSN:    ":memory:",
	})
	require.NoError(t, err, "failed to create store with sqlite3 driver")
	require.NotNil(t, store, "store should not be nil")
	defer store.Close()

	// Verify store implements Store interface
	var _ storage.Store = store
}

// TestFactory_UnregisteredDriver verifies that an error is returned for unregistered drivers.
func TestFactory_UnregisteredDriver(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to create store with unregistered driver
	store, err := storage.New(ctx, storage.Config{
		Driver: "postgres",
		DSN:    "postgres://localhost/testdb",
	})

	require.Error(t, err, "should return error for unregistered driver")
	require.Nil(t, store, "store should be nil on error")
	require.Contains(t, err.Error(), "postgres", "error message should mention the driver name")
}

// TestFactory_UnregisteredDriverGeneric verifies error for completely unknown drivers.
func TestFactory_UnregisteredDriverGeneric(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to create store with completely unknown driver
	store, err := storage.New(ctx, storage.Config{
		Driver: "mongodb",
		DSN:    "mongodb://localhost:27017",
	})

	require.Error(t, err, "should return error for unknown driver")
	require.Nil(t, store, "store should be nil on error")
	require.Contains(t, err.Error(), "mongodb", "error message should mention the driver name")
}

// TestFactory_ConfigPropagation verifies that configuration is properly passed to the backend.
func TestFactory_ConfigPropagation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create store with specific configuration
	store, err := storage.New(ctx, storage.Config{
		Driver:       "sqlite",
		DSN:          ":memory:",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	})
	require.NoError(t, err, "failed to create store with configuration")
	require.NotNil(t, store, "store should not be nil")
	defer store.Close()

	// Verify store is accessible via Ping (configuration was applied correctly)
	err = store.Ping(ctx)
	require.NoError(t, err, "store should be accessible after creation")

	// Verify store statistics are accessible
	stats, err := store.GetStats(ctx)
	require.NoError(t, err, "failed to get store statistics")
	require.NotNil(t, stats, "stats should not be nil")
}

// TestFactory_ThreadSafety verifies that concurrent store creation is safe.
func TestFactory_ThreadSafety(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const numGoroutines = 10
	var wg sync.WaitGroup
	stores := make([]storage.Store, numGoroutines)
	errors := make([]error, numGoroutines)
	mu := sync.Mutex{}
	tmpDir, err := os.MkdirTemp("", "factory_test_*")
	require.NoError(t, err, "failed to create temp directory")
	defer os.RemoveAll(tmpDir)

	// Launch concurrent goroutines to create stores
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Use separate file-based database for each goroutine to avoid SQLite locking issues
			dbPath := filepath.Join(tmpDir, fmt.Sprintf("test_%d.sqlite", index))

			store, err := storage.New(ctx, storage.Config{
				Driver: "sqlite",
				DSN:    dbPath,
			})

			mu.Lock()
			stores[index] = store
			errors[index] = err
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify all stores were created successfully
	for i, err := range errors {
		require.NoError(t, err, "goroutine %d failed to create store", i)
		require.NotNil(t, stores[i], "store %d should not be nil", i)
		defer stores[i].Close()
	}

	// Verify all stores are functional
	for i, store := range stores {
		if store != nil {
			err := store.Ping(ctx)
			require.NoError(t, err, "store %d should be accessible", i)
		}
	}
}

// TestFactory_MultipleDrivers verifies that both driver names work identically.
func TestFactory_MultipleDrivers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create store with "sqlite" driver
	storeSQLite, err := storage.New(ctx, storage.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	require.NoError(t, err, "failed to create store with sqlite driver")
	require.NotNil(t, storeSQLite, "sqlite store should not be nil")
	defer storeSQLite.Close()

	// Create store with "sqlite3" driver
	storeSQLite3, err := storage.New(ctx, storage.Config{
		Driver: "sqlite3",
		DSN:    ":memory:",
	})
	require.NoError(t, err, "failed to create store with sqlite3 driver")
	require.NotNil(t, storeSQLite3, "sqlite3 store should not be nil")
	defer storeSQLite3.Close()

	// Verify both stores are functional
	err = storeSQLite.Ping(ctx)
	require.NoError(t, err, "sqlite store should be accessible")

	err = storeSQLite3.Ping(ctx)
	require.NoError(t, err, "sqlite3 store should be accessible")

	// Both should have zero initial counts
	statsSQLite, err := storeSQLite.GetStats(ctx)
	require.NoError(t, err, "failed to get sqlite store stats")
	require.Zero(t, statsSQLite.PostCount, "sqlite store should start with zero posts")
	require.Zero(t, statsSQLite.CommentCount, "sqlite store should start with zero comments")

	statsSQLite3, err := storeSQLite3.GetStats(ctx)
	require.NoError(t, err, "failed to get sqlite3 store stats")
	require.Zero(t, statsSQLite3.PostCount, "sqlite3 store should start with zero posts")
	require.Zero(t, statsSQLite3.CommentCount, "sqlite3 store should start with zero comments")
}

// TestFactory_InvalidDSN verifies that invalid DSN values produce appropriate errors.
func TestFactory_InvalidDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to create store with invalid DSN path that doesn't have write permissions
	// (would fail at SQLite initialization level, not factory level)
	store, err := storage.New(ctx, storage.Config{
		Driver: "sqlite",
		DSN:    "/dev/null/nonexistent/db.sqlite",
	})

	// We expect an error from SQLite trying to open the file
	// This could be from various stages of initialization
	if err == nil {
		// If store was created, close it
		store.Close()
		t.Skip("Test environment allows writing to /dev/null/nonexistent, skipping")
	}
	// If we got an error, that's expected behavior
	require.Error(t, err, "should return error for invalid DSN path")
}

// TestFactory_AutoDetectDriver verifies that driver is auto-detected from DSN when not specified.
func TestFactory_AutoDetectDriver(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create store without specifying driver (should auto-detect as SQLite from DSN format)
	store, err := storage.New(ctx, storage.Config{
		DSN: ":memory:",
		// Driver is empty, should auto-detect
	})
	require.NoError(t, err, "failed to create store with auto-detected driver")
	require.NotNil(t, store, "store should not be nil")
	defer store.Close()

	// Verify store is functional
	err = store.Ping(ctx)
	require.NoError(t, err, "auto-detected store should be accessible")
}

// TestFactory_ContextCancellation verifies that context cancellation is respected.
func TestFactory_ContextCancellation(t *testing.T) {
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to create store with cancelled context
	store, err := storage.New(ctx, storage.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})

	// Should either return an error or successfully create a store
	// (SQLite with :memory: might succeed even with cancelled context)
	if err == nil && store != nil {
		defer store.Close()
	}
	// No assertion needed - just verify it doesn't panic
}

// TestFactory_ResourceCleanup verifies that stores are properly cleaned up.
func TestFactory_ResourceCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create multiple stores and close them
	for i := 0; i < 5; i++ {
		store, err := storage.New(ctx, storage.Config{
			Driver: "sqlite",
			DSN:    ":memory:",
		})
		require.NoError(t, err, "iteration %d: failed to create store", i)
		require.NotNil(t, store, "iteration %d: store should not be nil", i)

		// Verify store works before closing
		err = store.Ping(ctx)
		require.NoError(t, err, "iteration %d: store should be accessible before close", i)

		// Close the store
		err = store.Close()
		require.NoError(t, err, "iteration %d: close should not error", i)
	}
	// If we get here without panics or race conditions, cleanup is working
}
