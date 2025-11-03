// Package cache provides token caching interfaces and implementations for Reddit API authentication.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

type OAuthToken struct {
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
}

// MemoryCache is a simple in-memory token cache with no persistence.
// Thread-safe using atomic operations for lock-free reads and writes.
type MemoryCache struct {
	token atomic.Pointer[OAuthToken]
	clock clock.Clock
}

// NewMemoryCache creates a new memory-based token cache.
// If clock is nil, a real clock will be used.
func NewMemoryCache(clk clock.Clock) *MemoryCache {
	if clk == nil {
		clk = clock.NewRealClock()
	}
	return &MemoryCache{clock: clk}
	// Note: atomic.Pointer zero value is nil, no explicit initialization needed
}

// Get retrieves the cached token if it exists and is not expired.
func (m *MemoryCache) Get(ctx context.Context) (string, time.Time, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", time.Time{}, false, err
	}

	loaded := m.token.Load()
	if loaded == nil {
		return "", time.Time{}, false, nil
	}

	// Check if token has expired
	if m.clock.Now().After(loaded.Expiry) {
		return "", time.Time{}, false, nil
	}

	return loaded.Token, loaded.Expiry, true, nil

}

// Set stores a token with its expiry time.
func (m *MemoryCache) Set(ctx context.Context, token string, expiry time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.token.Store(&OAuthToken{
		Token:  token,
		Expiry: expiry,
	})
	return nil
}

// Invalidate clears the cached token.
func (m *MemoryCache) Invalidate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.token.Store(nil)
	return nil
}

type FileCache struct {
	token    atomic.Pointer[OAuthToken] // Lock-free reads
	writeMu  sync.Mutex                 // Only for coordinating writes
	filePath string
	clock    clock.Clock
}

func (f *FileCache) Get(ctx context.Context) (string, time.Time, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", time.Time{}, false, err
	}

	loaded := f.token.Load()
	if loaded == nil || f.clock.Now().After(loaded.Expiry) {
		return "", time.Time{}, false, nil
	}
	return loaded.Token, loaded.Expiry, true, nil
}

func (f *FileCache) Set(ctx context.Context, token string, expiry time.Time) error {
	newToken := &OAuthToken{Token: token, Expiry: expiry}

	// Update memory immediately (visible to readers)
	f.token.Store(newToken)

	// Serialize file writes (but don't block readers)
	f.writeMu.Lock()
	defer f.writeMu.Unlock()

	return f.saveToFile()
}

// NewFileCache creates a new file-backed token cache.
// The filePath parameter specifies where to persist tokens.
// Parent directories are created automatically with secure permissions (0700).
// If clock is nil, a real clock will be used.
// Errors are returned only if the path is invalid or directory creation fails.
// Existing cache files are loaded transparently, and load failures are treated
// as cache misses without error.
func NewFileCache(filePath string, clk clock.Clock) (*FileCache, error) {
	if filePath == "" {
		return nil, &CacheError{Operation: "create", Err: fmt.Errorf("cache path cannot be empty")}
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}

	// Validate parent directory exists and create if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, &CacheError{Operation: "create", Path: filePath, Err: fmt.Errorf("failed to create cache directory: %w", err)}
	}

	cache := &FileCache{
		filePath: filePath,
		clock:    clk,
	}

	// Try to load existing token from file (non-fatal failure)
	_ = cache.loadFromFile()

	return cache, nil
}

// Invalidate clears the cached token and removes the persisted file.
func (f *FileCache) Invalidate(ctx context.Context) error {
	// Check context before acquiring lock (Issue 2: Context cancellation)
	if err := ctx.Err(); err != nil {
		return err
	}

	f.writeMu.Lock()
	defer f.writeMu.Unlock()

	// Check again after acquiring lock
	if err := ctx.Err(); err != nil {
		return err
	}

	f.token.Store(nil)

	// Remove persisted file
	return f.deleteFile()
}

// loadFromFile attempts to load a token from the cache file.
// It validates:
//   - The file exists and is readable
//   - The file contains valid JSON
//   - The cached token has not expired
//   - File permissions are secure (owned by current user, no world access)
//   - File ownership is validated on Unix systems (Issue 1: File ownership validation)
//
// If any validation fails, the cache is treated as a miss.
// This method should only be called during initialization.
func (f *FileCache) loadFromFile() error {
	// Check if file exists
	fileInfo, err := os.Lstat(f.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file, this is normal for first use
		}
		return &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("failed to stat cache file: %w", err)}
	}

	// Validate file permissions (0600 = owner can read/write, no one else)
	// Check that group and other bits are not set
	if mode := fileInfo.Mode(); (mode & 0077) != 0 {
		return &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("cache file has insecure permissions: %o (expected 0600)", mode.Perm())}
	}

	// Validate file ownership on Unix systems
	if runtime.GOOS != "windows" {
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			currentUser, err := user.Current()
			if err != nil {
				return &CacheError{
					Operation: "load",
					Path:      f.filePath,
					Err:       fmt.Errorf("failed to get current user for ownership check: %w", err),
				}
			}

			currentUID, err := strconv.Atoi(currentUser.Uid)
			if err != nil {
				return &CacheError{
					Operation: "load",
					Path:      f.filePath,
					Err:       fmt.Errorf("failed to parse current user UID: %w", err),
				}
			}

			if stat.Uid != uint32(currentUID) {
				return &CacheError{
					Operation: "load",
					Path:      f.filePath,
					Err:       fmt.Errorf("cache file is owned by UID %d, expected %d", stat.Uid, currentUID),
				}
			}
		}
	}

	// Read file contents
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("failed to read cache file: %w", err)}
	}

	// Parse JSON
	var cd OAuthToken
	if err := json.Unmarshal(data, &cd); err != nil {
		return &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("failed to parse cache file: %w", err)}
	}

	// Validate token is not empty
	if cd.Token == "" {
		return &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("cached token is empty")}
	}

	// Validate token has not expired
	if f.clock.Now().After(cd.Expiry) {
		return &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("cached token has expired")}
	}

	// Load into cache
	f.token.Store(&OAuthToken{
		Token:  cd.Token,
		Expiry: cd.Expiry,
	})

	return nil
}

// saveToFile persists the current token to the cache file atomically.
// It creates a temporary file, writes data to it with secure permissions (0600),
// syncs to disk, and then atomically renames it to the cache file.
// This ensures the cache file is never in a partially-written state.
// Issue 3: Use a flag to prevent unnecessary cleanup after successful rename.
func (f *FileCache) saveToFile() error {
	// If token is nil, nothing to save
	token := f.token.Load()
	if token == nil {
		return nil
	}

	// Create temporary file in the same directory as the cache file
	// to ensure atomic rename works
	dir := filepath.Dir(f.filePath)
	tmpFile, err := os.CreateTemp(dir, ".cache-tmp-")
	if err != nil {
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to create temp file: %w", err)}
	}

	tmpName := tmpFile.Name()
	shouldCleanup := true

	// Issue 3: Defer cleanup with a flag to prevent removing successfully renamed files
	defer func() {
		if shouldCleanup {
			os.Remove(tmpName)
		}
	}()

	// Set secure permissions on temp file before writing
	if err := os.Chmod(tmpName, 0600); err != nil {
		tmpFile.Close()
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to set temp file permissions: %w", err)}
	}

	// Write JSON to temp file
	encoder := json.NewEncoder(tmpFile)
	if err := encoder.Encode(OAuthToken{
		Token:  token.Token,
		Expiry: token.Expiry,
	}); err != nil {
		tmpFile.Close()
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to write cache file: %w", err)}
	}

	// Flush to ensure all data is written
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to sync cache file: %w", err)}
	}

	// Close temp file before rename (required on Windows)
	if err := tmpFile.Close(); err != nil {
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to close temp file: %w", err)}
	}

	// Atomically rename temp file to cache file
	if err := os.Rename(tmpName, f.filePath); err != nil {
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to rename cache file: %w", err)}
	}

	shouldCleanup = false // Don't clean up after successful rename
	return nil
}

// deleteFile removes the cache file.
// Returns nil if the file doesn't exist (non-fatal).
func (f *FileCache) deleteFile() error {
	if err := os.Remove(f.filePath); err != nil && !os.IsNotExist(err) {
		return &CacheError{Operation: "delete", Path: f.filePath, Err: fmt.Errorf("failed to delete cache file: %w", err)}
	}
	return nil
}
