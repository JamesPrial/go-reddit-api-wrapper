package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

// mockClock is a test helper that wraps clock.MockClock.
type mockClock struct {
	*clock.MockClock
}

// NewMockClock creates a test clock starting at the given time.
func NewMockClock(t time.Time) *mockClock {
	return &mockClock{clock.NewMockClock(t)}
}

// discardLogger returns a logger that discards all output (useful for tests).
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// TestMemoryCache_GetSet tests basic get/set operations.
func TestMemoryCache_GetSet(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	tokenStr := "test-token-123"
	expiry := clk.Now().Add(1 * time.Hour)

	// Set a token
	err := cache.Set(context.Background(), tokenStr, expiry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the token
	retrievedToken, retrievedExpiry, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("Token should be found")
	}
	if retrievedToken != tokenStr {
		t.Errorf("Token mismatch: got %q, want %q", retrievedToken, tokenStr)
	}
	if !retrievedExpiry.Equal(expiry) {
		t.Errorf("Expiry mismatch: got %v, want %v", retrievedExpiry, expiry)
	}
}

// TestMemoryCache_GetExpired tests that expired tokens are treated as cache misses.
func TestMemoryCache_GetExpired(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	tokenStr := "test-token-123"
	expiry := clk.Now().Add(1 * time.Hour)

	// Set a token
	err := cache.Set(context.Background(), tokenStr, expiry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Advance time past expiry
	clk.Advance(2 * time.Hour)

	// Get should return cache miss
	_, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Expired token should not be found")
	}
}

// TestMemoryCache_GetMiss tests cache miss when no token is set.
func TestMemoryCache_GetMiss(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	_, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Should be cache miss")
	}
}

// TestMemoryCache_Invalidate tests clearing the cache.
func TestMemoryCache_Invalidate(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	tokenStr := "test-token-123"
	expiry := clk.Now().Add(1 * time.Hour)

	// Set a token
	err := cache.Set(context.Background(), tokenStr, expiry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it's there
	_, _, found, _ := cache.Get(context.Background())
	if !found {
		t.Fatal("Token should be found before invalidation")
	}

	// Invalidate
	err = cache.Invalidate(context.Background())
	if err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}

	// Verify it's gone
	_, _, found, _ = cache.Get(context.Background())
	if found {
		t.Fatal("Token should not be found after invalidation")
	}
}

// TestFileCache_CreateDirectory tests that NewFileCache creates parent directories.
func TestFileCache_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "subdir1", "subdir2", "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}
	if cache == nil {
		t.Fatal("Cache should not be nil")
	}

	// Verify directory was created
	dir := filepath.Dir(cachePath)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Cache directory was not created: %v", err)
	}

	// Verify directory has secure permissions (0700)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("Directory permissions: got %o, want 0700", info.Mode().Perm())
	}
}

// TestFileCache_GetSet tests basic get/set with file persistence.
func TestFileCache_GetSet(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	tokenStr := "test-token-123"
	expiry := clk.Now().Add(1 * time.Hour)

	// Set a token
	err = cache.Set(context.Background(), tokenStr, expiry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the token
	retrievedToken, retrievedExpiry, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("Token should be found")
	}
	if retrievedToken != tokenStr {
		t.Errorf("Token mismatch: got %q, want %q", retrievedToken, tokenStr)
	}
	if !retrievedExpiry.Equal(expiry) {
		t.Errorf("Expiry mismatch: got %v, want %v", retrievedExpiry, expiry)
	}

	// Verify file was created with correct permissions
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Cache file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Cache file permissions: got %o, want 0600", info.Mode().Perm())
	}
}

// TestFileCache_LoadFromDisk tests loading a token from a persisted cache file.
func TestFileCache_LoadFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create a cache file manually
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := startTime.Add(1 * time.Hour)

	data := oauthToken{
		Token:  "persisted-token",
		Expiry: expiry,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(cachePath, jsonData, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Create cache instance and verify it loads the persisted token
	clk := NewMockClock(startTime)
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	// Get should return the persisted token
	retrievedToken, retrievedExpiry, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("Persisted token should be found")
	}
	if retrievedToken != "persisted-token" {
		t.Errorf("Token mismatch: got %q, want persisted-token", retrievedToken)
	}
	if !retrievedExpiry.Equal(expiry) {
		t.Errorf("Expiry mismatch: got %v, want %v", retrievedExpiry, expiry)
	}
}

// TestFileCache_LoadExpired tests that expired persisted tokens are not loaded.
func TestFileCache_LoadExpired(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create a cache file with an expired token
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := startTime.Add(-1 * time.Hour) // Expired

	data := oauthToken{
		Token:  "expired-token",
		Expiry: expiry,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(cachePath, jsonData, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Create cache instance - should not load expired token
	clk := NewMockClock(startTime)
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	// Get should return cache miss
	_, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Expired token should not be loaded")
	}
}

// TestFileCache_InsecurePermissions tests that insecurely permissioned files are rejected.
func TestFileCache_InsecurePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := startTime.Add(1 * time.Hour)

	// Create a cache file with valid token data but INSECURE permissions
	data := oauthToken{
		Token:  "test-token",
		Expiry: expiry,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Write with insecure permissions (world-readable 0644)
	if err := os.WriteFile(cachePath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Create cache instance - should reject insecure file
	clk := NewMockClock(startTime)
	_, found, _, _, err := NewFileCache(context.Background(), cachePath, clk)

	// Should return an error for insecure permissions
	if err == nil {
		t.Fatal("NewFileCache should error on insecure permissions")
	}

	var ce *CacheError
	if !errors.As(err, &ce) {
		t.Fatalf("Error should be CacheError, got %T", err)
	}

	if !strings.Contains(ce.Error(), "insecure permissions") {
		t.Errorf("Error should mention insecure permissions, got: %v", err)
	}

	if found {
		t.Error("Should not return found=true for insecurely permissioned file")
	}
}

// TestFileCache_Invalidate tests clearing the cache and removing the file.
func TestFileCache_Invalidate(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	tokenStr := "test-token-123"
	expiry := clk.Now().Add(1 * time.Hour)

	// Set a token
	err = cache.Set(context.Background(), tokenStr, expiry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("Cache file should exist: %v", err)
	}

	// Invalidate
	err = cache.Invalidate(context.Background())
	if err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}

	// Verify cache is empty
	_, _, found, _ := cache.Get(context.Background())
	if found {
		t.Fatal("Token should not be found after invalidation")
	}

	// Verify file is deleted
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("Cache file should be deleted: %v", err)
	}
}

// TestFileCache_InvalidateNonExistent tests invalidating when no file exists.
func TestFileCache_InvalidateNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	// Invalidate when no file exists (should not error)
	err = cache.Invalidate(context.Background())
	if err != nil {
		t.Fatalf("Invalidate should not error on non-existent file: %v", err)
	}
}

// TestFileCache_ParseError tests handling of corrupted cache files.
func TestFileCache_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create a corrupted cache file
	if err := os.WriteFile(cachePath, []byte("invalid json"), 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Create cache instance - should handle parse error gracefully
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache should not error on corrupted file: %v", err)
	}

	// Get should return cache miss
	_, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Corrupted file should not be loaded")
	}
}

// TestFileCache_EmptyToken tests rejection of empty tokens in persisted cache.
func TestFileCache_EmptyToken(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create a cache file with empty token
	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	expiry := startTime.Add(1 * time.Hour)

	data := oauthToken{
		Token:  "", // Empty
		Expiry: expiry,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(cachePath, jsonData, 0600); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Create cache instance - should reject empty token
	clk := NewMockClock(startTime)
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	// Get should return cache miss
	_, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("Empty token should not be loaded")
	}
}

// TestFileCache_AtomicWrite tests that writes are atomic using temp files.
func TestFileCache_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, clk)
	if err != nil {
		t.Fatalf("NewFileCache failed: %v", err)
	}

	expiry := clk.Now().Add(1 * time.Hour)

	// Set a token multiple times
	for i := range 5 {
		tokenStr := fmt.Sprintf("test-token-%d", i)
		err = cache.Set(context.Background(), tokenStr, expiry)
		if err != nil {
			t.Fatalf("Set failed on iteration %d: %v", i, err)
		}
	}

	// Verify the final token is correctly persisted
	retrievedToken, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("Token should be found")
	}
	if retrievedToken != "test-token-4" {
		t.Errorf("Token mismatch: got %q, want test-token-4", retrievedToken)
	}

	// Verify no temp files are left behind
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "cache.json" {
			t.Errorf("Unexpected file in cache directory: %s", entry.Name())
		}
	}
}

// TestFileCache_InvalidPathEmpty tests error handling for empty path.
func TestFileCache_InvalidPathEmpty(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	_, _, _, _, err := NewFileCache(context.Background(), "", clk)
	if err == nil {
		t.Fatal("NewFileCache should error on empty path")
	}

	var ce *CacheError
	if !AsError(err, &ce) {
		t.Fatalf("Error should be CacheError, got %T", err)
	}
	if ce.Operation != "create" {
		t.Errorf("Operation mismatch: got %s, want create", ce.Operation)
	}
}

// TestFileCache_NilClock tests that nil clock defaults to real clock.
func TestFileCache_NilClock(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	// Create with nil clock (should default to RealClock)
	cache, _, _, _, err := NewFileCache(context.Background(), cachePath, nil)
	if err != nil {
		t.Fatalf("NewFileCache with nil clock failed: %v", err)
	}

	tokenStr := "test-token"
	expiry := time.Now().Add(1 * time.Hour)

	err = cache.Set(context.Background(), tokenStr, expiry)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	retrievedToken, _, found, err := cache.Get(context.Background())
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("Token should be found")
	}
	if retrievedToken != tokenStr {
		t.Errorf("Token mismatch: got %q, want %q", retrievedToken, tokenStr)
	}
}

// AsError is a helper that mimics errors.As for CacheError.
func AsError(err error, target **CacheError) bool {
	if ce, ok := err.(*CacheError); ok {
		*target = ce
		return true
	}
	return false
}

// ============================================================================
// Context Cancellation Tests for MemoryCache
// ============================================================================

// TestMemoryCache_GetContextCanceled tests that Get respects context cancellation.
func TestMemoryCache_GetContextCanceled(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, _, err := cache.Get(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestMemoryCache_SetContextCanceled tests that Set respects context cancellation.
func TestMemoryCache_SetContextCanceled(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tokenStr := "test-token"
	expiry := clk.Now().Add(1 * time.Hour)
	err := cache.Set(ctx, tokenStr, expiry)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestMemoryCache_InvalidateContextCanceled tests that Invalidate respects context cancellation.
func TestMemoryCache_InvalidateContextCanceled(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := cache.Invalidate(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestMemoryCache_SetContextTimeoutDuring tests context timeout during Set operation.
func TestMemoryCache_SetContextTimeoutDuring(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	// Give timeout a chance to occur
	time.Sleep(10 * time.Millisecond)

	tokenStr := "test-token"
	expiry := clk.Now().Add(1 * time.Hour)
	err := cache.Set(ctx, tokenStr, expiry)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

// ============================================================================
// Context Cancellation Tests for FileCache
// ============================================================================

// TestFileCache_GetContextCanceled tests that FileCache.Get respects context cancellation.
func TestFileCache_GetContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, _, err = cache.Get(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestFileCache_SetContextCanceled tests that FileCache.Set respects context cancellation.
func TestFileCache_SetContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tokenStr := "token"
	expiry := clk.Now().Add(1 * time.Hour)
	err = cache.Set(ctx, tokenStr, expiry)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestFileCache_InvalidateContextCanceled tests that FileCache.Invalidate respects context cancellation.
func TestFileCache_InvalidateContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = cache.Invalidate(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

// TestFileCache_SetContextTimeoutDuring tests context timeout during FileCache.Set operation.
func TestFileCache_SetContextTimeoutDuring(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	// Give timeout a chance to occur
	time.Sleep(10 * time.Millisecond)

	tokenStr := "token"
	expiry := clk.Now().Add(1 * time.Hour)
	err = cache.Set(ctx, tokenStr, expiry)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

// ============================================================================
// Concurrency Tests
// ============================================================================

// TestMemoryCache_ConcurrentAccess tests MemoryCache with concurrent reads, writes, and invalidations.
// This test uses the race detector to ensure thread-safety.
func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // Writers, readers, invalidators

	// Writers: concurrently set tokens
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				tokenStr := fmt.Sprintf("token-%d-%d", id, j)
				expiry := clk.Now().Add(1 * time.Hour)
				_ = cache.Set(context.Background(), tokenStr, expiry)
			}
		}(i)
	}

	// Readers: concurrently read tokens
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				_, _, _, _ = cache.Get(context.Background())
			}
		}()
	}

	// Invalidators: concurrently invalidate cache
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				_ = cache.Invalidate(context.Background())
			}
		}()
	}

	wg.Wait()
	// Test passes if no race detector errors or panics occur
}

// TestMemoryCache_ConcurrentContextCancellation tests MemoryCache with concurrent operations and context cancellation.
func TestMemoryCache_ConcurrentContextCancellation(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	const goroutines = 50
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Each goroutine performs operations with cancellable contexts
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				// Randomly use cancelled contexts to test cancellation handling
				var ctx context.Context
				if j%3 == 0 {
					cancelCtx, cancel := context.WithCancel(context.Background())
					cancel()
					ctx = cancelCtx
				} else {
					ctx = context.Background()
				}

				tokenStr := fmt.Sprintf("token-%d-%d", id, j)
				expiry := clk.Now().Add(1 * time.Hour)

				switch j % 3 {
				case 0:
					_ = cache.Set(ctx, tokenStr, expiry)
				case 1:
					_, _, _, _ = cache.Get(ctx)
				default:
					_ = cache.Invalidate(ctx)
				}
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race detector errors or panics occur
}

// TestFileCache_ConcurrentAccess tests FileCache with concurrent reads, writes, and invalidations.
// This test uses the race detector to ensure thread-safety.
func TestFileCache_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	const goroutines = 50 // Fewer for file I/O
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Writers: concurrently write tokens to file
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				tokenStr := fmt.Sprintf("token-%d-%d", id, j)
				expiry := clk.Now().Add(1 * time.Hour)
				_ = cache.Set(context.Background(), tokenStr, expiry)
			}
		}(i)
	}

	// Readers: concurrently read tokens
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				_, _, _, _ = cache.Get(context.Background())
			}
		}()
	}

	// Invalidators: concurrently invalidate cache
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				_ = cache.Invalidate(context.Background())
			}
		}()
	}

	wg.Wait()
	// Test passes if no race detector errors or panics occur
}

// TestFileCache_ConcurrentContextCancellation tests FileCache with concurrent operations and context cancellation.
func TestFileCache_ConcurrentContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	const goroutines = 30
	const iterations = 30

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Each goroutine performs operations with cancellable contexts
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				// Randomly use cancelled contexts to test cancellation handling
				var ctx context.Context
				if j%3 == 0 {
					cancelCtx, cancel := context.WithCancel(context.Background())
					cancel()
					ctx = cancelCtx
				} else {
					ctx = context.Background()
				}

				tokenStr := fmt.Sprintf("token-%d-%d", id, j)
				expiry := clk.Now().Add(1 * time.Hour)

				switch j % 3 {
				case 0:
					_ = cache.Set(ctx, tokenStr, expiry)
				case 1:
					_, _, _, _ = cache.Get(ctx)
				default:
					_ = cache.Invalidate(ctx)
				}
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race detector errors or panics occur
}

// TestMemoryCache_SequentialContextStates tests context behavior across sequential operations.
func TestMemoryCache_SequentialContextStates(t *testing.T) {
	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache := NewMemoryCache(clk, discardLogger())

	// Test with valid context
	ctx := context.Background()
	tokenStr1 := "token"
	expiry := clk.Now().Add(1 * time.Hour)
	err := cache.Set(ctx, tokenStr1, expiry)
	if err != nil {
		t.Errorf("Set with valid context should succeed, got %v", err)
	}

	// Test with cancelled context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	tokenStr2 := "token2"
	err = cache.Set(cancelCtx, tokenStr2, expiry)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Set with cancelled context should return Canceled, got %v", err)
	}

	// Verify original token is still there (operation was rejected)
	retrievedToken, _, found, _ := cache.Get(context.Background())
	if !found || retrievedToken != "token" {
		t.Errorf("Original token should be preserved after cancelled Set, got %q (found=%v)", retrievedToken, found)
	}
}

// TestFileCache_SequentialContextStates tests context behavior across sequential operations for FileCache.
func TestFileCache_SequentialContextStates(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "cache.json")

	clk := NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	cache, _, _, _, err := NewFileCache(context.Background(), filePath, clk)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test with valid context
	ctx := context.Background()
	tokenStr1 := "token"
	expiry := clk.Now().Add(1 * time.Hour)
	err = cache.Set(ctx, tokenStr1, expiry)
	if err != nil {
		t.Errorf("Set with valid context should succeed, got %v", err)
	}

	// Test with cancelled context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	tokenStr2 := "token2"
	err = cache.Set(cancelCtx, tokenStr2, expiry)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Set with cancelled context should return Canceled, got %v", err)
	}

	// Verify original token is still there (operation was rejected)
	retrievedToken, _, found, _ := cache.Get(context.Background())
	if !found || retrievedToken != "token" {
		t.Errorf("Original token should be preserved after cancelled Set, got %q (found=%v)", retrievedToken, found)
	}
}
