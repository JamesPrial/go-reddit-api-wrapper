// Package cache provides token caching interfaces and implementations for Reddit API authentication.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/reqid"
)

const (
	CACHE_FILE_PERMISSIONS           = 0600
	CACHE_FILE_DIRECTORY_PERMISSIONS = 0700
)

type oauthToken struct {
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
}

// MemoryCache is a simple in-memory token cache with no persistence.
// Thread-safe using atomic operations for lock-free reads and writes.
type MemoryCache struct {
	token  atomic.Pointer[oauthToken]
	logger *slog.Logger
	clock  clock.Clock
}

// NewMemoryCache creates a new memory-based token cache.
// If clock is nil, a real clock will be used.
// If logger is nil, a discard logger will be used.
func NewMemoryCache(clk clock.Clock, logger *slog.Logger) *MemoryCache {
	if clk == nil {
		clk = clock.NewRealClock()
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &MemoryCache{clock: clk, logger: logger}
	// Note: atomic.Pointer zero value is nil, no explicit initialization needed
}

// Get retrieves the cached token if it exists and is not expired.
func (m *MemoryCache) Get(ctx context.Context) (token string, expiry time.Time, found bool, err error) {
	if err := ctx.Err(); err != nil {
		m.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while getting token from memory cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return "", time.Time{}, false, err
	}

	loaded := m.token.Load()
	if loaded == nil {
		m.logger.LogAttrs(ctx, slog.LevelDebug, "no token found in memory cache",
			slog.String("request_id", reqid.FromContext(ctx)))
		return "", time.Time{}, false, nil
	}

	// Check if token has expired
	if m.clock.Now().After(loaded.Expiry) {
		m.logger.LogAttrs(ctx, slog.LevelDebug, "token has expired in memory cache",
			slog.String("request_id", reqid.FromContext(ctx)))
		return "", time.Time{}, false, nil
	}

	return loaded.Token, loaded.Expiry, true, nil

}

// Set stores a token with its expiry time.
func (m *MemoryCache) Set(ctx context.Context, token string, expiry time.Time) error {
	if err := ctx.Err(); err != nil {
		m.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while setting token in memory cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return &CacheError{Operation: "set", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	m.token.Store(&oauthToken{
		Token:  token,
		Expiry: expiry,
	})
	return nil
}

// Invalidate clears the cached token.
func (m *MemoryCache) Invalidate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		m.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while invalidating token in memory cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return &CacheError{Operation: "invalidate", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	m.token.Store(nil)
	m.logger.LogAttrs(ctx, slog.LevelDebug, "token invalidated in memory cache",
		slog.String("request_id", reqid.FromContext(ctx)))
	return nil
}

type FileCache struct {
	token    atomic.Pointer[oauthToken] // Lock-free reads
	writeMu  sync.Mutex                 // Only for coordinating writes
	filePath string
	clock    clock.Clock
	logger   *slog.Logger
}

func (f *FileCache) Get(ctx context.Context) (token string, expiry time.Time, found bool, err error) {
	if err := ctx.Err(); err != nil {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while getting token from file cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return "", time.Time{}, false, &CacheError{Operation: "get", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	loaded := f.token.Load()
	if loaded == nil || f.clock.Now().After(loaded.Expiry) {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "no token found in file cache",
			slog.String("request_id", reqid.FromContext(ctx)))
		return "", time.Time{}, false, nil
	}
	f.logger.LogAttrs(ctx, slog.LevelDebug, "token found in file cache",
		slog.String("request_id", reqid.FromContext(ctx)))
	return loaded.Token, loaded.Expiry, true, nil
}

func (f *FileCache) Set(ctx context.Context, token string, expiry time.Time) error {
	if err := ctx.Err(); err != nil {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while setting token in file cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return &CacheError{Operation: "set", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	newToken := &oauthToken{Token: token, Expiry: expiry}

	// Update memory immediately (visible to readers)
	f.token.Store(newToken)

	// Serialize file writes (but don't block readers)
	f.writeMu.Lock()
	defer f.writeMu.Unlock()

	// Check again after acquiring lock
	if err := ctx.Err(); err != nil {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled after acquiring write lock",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return &CacheError{Operation: "set", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	f.logger.LogAttrs(ctx, slog.LevelDebug, "writeMu Acquired",
		slog.String("request_id", reqid.FromContext(ctx)))
	err := f.saveToFile(ctx)
	return err
}

// NewFileCache creates a new file-backed token cache.
// The filePath parameter specifies where to persist tokens.
// Parent directories are created automatically with secure permissions (CACHE_FILE_DIRECTORY_PERMISSIONS).
// If clock is nil, a real clock will be used.
// Errors are returned only if the path is invalid or directory creation fails.
// Existing cache files are loaded transparently, and load failures are treated
// as cache misses without error.
func NewFileCache(ctx context.Context, filePath string, clk clock.Clock) (cache *FileCache, found bool, token string, expiry time.Time, err error) {
	if filePath == "" {
		return nil, false, "", time.Time{}, &CacheError{Operation: "create", Err: fmt.Errorf("cache path cannot be empty")}
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}

	// Validate parent directory exists and create if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, CACHE_FILE_DIRECTORY_PERMISSIONS); err != nil {
		return nil, false, "", time.Time{}, &CacheError{Operation: "create", Path: filePath, Err: fmt.Errorf("failed to create cache directory: %w", err)}
	}

	cache = &FileCache{
		filePath: filePath,
		clock:    clk,
		logger:   slog.New(slog.DiscardHandler),
	}

	// Try to load existing token from file (non-fatal failure)
	found, token, expiry, err = cache.loadFromFile(ctx)
	return cache, found, token, expiry, err
}

// Invalidate clears the cached token and removes the persisted file.
func (f *FileCache) Invalidate(ctx context.Context) error {
	// Check context before acquiring lock (Issue 2: Context cancellation)
	if err := ctx.Err(); err != nil {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while invalidating token in file cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return &CacheError{Operation: "invalidate", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	f.writeMu.Lock()
	defer f.writeMu.Unlock()

	// Check again after acquiring lock
	if err := ctx.Err(); err != nil {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "context cancelled while invalidating token in file cache",
			slog.String("error", err.Error()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return &CacheError{Operation: "invalidate", Err: err, RequestID: reqid.FromContext(ctx)}
	}

	f.token.Store(nil)

	// Remove persisted file
	return f.deleteFile(ctx)
}

// loadFromFile attempts to load a token from the cache file.
// It validates:
//   - The file exists and is readable
//   - The file contains valid JSON
//   - The cached token has not expired
//   - File permissions are secure (owned by current user, no world access)
//   - File ownership is validated on Unix systems (Issue 1: File ownership validation)
//
// Validation errors (expired token, empty token, parse errors) are treated as cache misses
// and logged but not returned as errors.
// Only truly fatal conditions return errors:
//   - File stat fails (permission issues)
//   - File has insecure permissions
//   - File has wrong ownership
//   - File read fails
//
// This method should only be called during initialization.
func (f *FileCache) loadFromFile(ctx context.Context) (found bool, token string, expiry time.Time, err error) {
	// Check if file exists
	fileInfo, err := os.Lstat(f.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", time.Time{}, nil // No cache file, this is normal for first use
		}
		return false, "", time.Time{}, &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("failed to stat cache file: %w", err)}
	}
	f.logger.LogAttrs(ctx, slog.LevelDebug, "fileInfo",
		slog.String("name", fileInfo.Name()),
		slog.String("size", strconv.FormatInt(fileInfo.Size(), 10)),
		slog.String("mode", fileInfo.Mode().String()),
		slog.String("modTime", fileInfo.ModTime().String()),
		slog.String("request_id", reqid.FromContext(ctx)))
	// Validate file permissions (0600 = owner can read/write, no one else)
	// Only check permission bits (0777), not file type bits
	if mode := fileInfo.Mode().Perm(); mode != CACHE_FILE_PERMISSIONS {
		return false, "", time.Time{}, &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("cache file has insecure permissions: %o (expected 0600)", mode)}
	}

	// Validate file ownership on Unix systems
	if runtime.GOOS != "windows" {
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			currentUser, err := user.Current()
			if err != nil {
				return false, "", time.Time{}, &CacheError{
					Operation: "load",
					Path:      f.filePath,
					Err:       fmt.Errorf("failed to get current user for ownership check: %w", err),
				}
			}

			currentUID, err := strconv.Atoi(currentUser.Uid)
			if err != nil {
				return false, "", time.Time{}, &CacheError{
					Operation: "load",
					Path:      f.filePath,
					Err:       fmt.Errorf("failed to parse current user UID: %w", err),
				}
			}

			if stat.Uid != uint32(currentUID) {
				return false, "", time.Time{}, &CacheError{
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
		return false, "", time.Time{}, &CacheError{Operation: "load", Path: f.filePath, Err: fmt.Errorf("failed to read cache file: %w", err)}
	}

	// Parse JSON - treat parse errors as cache misses (non-fatal)
	var cd oauthToken
	if err := json.Unmarshal(data, &cd); err != nil {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "failed to parse cache file, treating as cache miss",
			slog.String("error", err.Error()),
			slog.String("path", f.filePath),
			slog.String("request_id", reqid.FromContext(ctx)))
		return false, "", time.Time{}, nil
	}

	// Validate token is not empty - treat as cache miss (non-fatal)
	if cd.Token == "" {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "cached token is empty, treating as cache miss",
			slog.String("path", f.filePath),
			slog.String("request_id", reqid.FromContext(ctx)))
		return false, "", time.Time{}, nil
	}

	// Validate token has not expired - treat as cache miss (non-fatal)
	if f.clock.Now().After(cd.Expiry) {
		f.logger.LogAttrs(ctx, slog.LevelDebug, "cached token has expired, treating as cache miss",
			slog.String("path", f.filePath),
			slog.String("expiry", cd.Expiry.String()),
			slog.String("request_id", reqid.FromContext(ctx)))
		return false, "", time.Time{}, nil
	}

	// Load into cache
	f.token.Store(&oauthToken{
		Token:  cd.Token,
		Expiry: cd.Expiry,
	})

	return true, cd.Token, cd.Expiry, nil
}

// saveToFile persists the current token to the cache file atomically.
// It creates a temporary file, writes data to it with secure permissions (0600),
// syncs to disk, and then atomically renames it to the cache file.
// This ensures the cache file is never in a partially-written state.
// Issue 3: Use a flag to prevent unnecessary cleanup after successful rename.
func (f *FileCache) saveToFile(ctx context.Context) error {
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
	f.logger.LogAttrs(ctx, slog.LevelDebug, "tmpName",
		slog.String("name", tmpName),
		slog.String("request_id", reqid.FromContext(ctx)))
	shouldCleanup := true

	// Issue 3: Defer cleanup with a flag to prevent removing successfully renamed files
	defer func() {
		if shouldCleanup {
			os.Remove(tmpName)
		}
	}()

	// Set secure permissions on temp file before writing
	if err := os.Chmod(tmpName, CACHE_FILE_PERMISSIONS); err != nil {
		tmpFile.Close()
		return &CacheError{Operation: "save", Path: f.filePath, Err: fmt.Errorf("failed to set temp file permissions: %w", err)}
	}

	// Write JSON to temp file
	encoder := json.NewEncoder(tmpFile)
	if err := encoder.Encode(oauthToken{
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

	f.logger.LogAttrs(ctx, slog.LevelDebug, "cache file created",
		slog.String("name", f.filePath),
		slog.String("request_id", reqid.FromContext(ctx)))
	shouldCleanup = false // Don't clean up after successful rename
	return nil
}

// deleteFile removes the cache file.
// Returns nil if the file doesn't exist (non-fatal).
func (f *FileCache) deleteFile(ctx context.Context) error {
	if err := os.Remove(f.filePath); err != nil && !os.IsNotExist(err) {
		return &CacheError{Operation: "delete", Path: f.filePath, Err: fmt.Errorf("failed to delete cache file: %w", err)}
	}
	f.logger.LogAttrs(ctx, slog.LevelDebug, "cache file deleted",
		slog.String("name", f.filePath),
		slog.String("request_id", reqid.FromContext(ctx)))
	return nil
}
