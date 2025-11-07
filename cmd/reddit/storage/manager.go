// Package storage provides storage management for the Reddit CLI.
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// Manager provides a storage management wrapper for the CLI.
// It wraps the storage.Store interface and handles initialization and cleanup.
type Manager struct {
	store  storage.Store
	logger *slog.Logger
}

// NewManager initializes a new storage Manager with the specified database path.
// It performs the following operations:
//   - Expands home directory paths (~/...) to absolute paths
//   - Creates parent directories if they don't exist
//   - Initializes a SQLite store with proper configuration (WAL mode, foreign keys, etc.)
//
// Parameters:
//   - dbPath: Path to the database file. Use ":memory:" for in-memory database. Supports ~/ prefix.
//   - logger: Logger for structured logging. If nil, uses slog.Default().
//
// Returns an error if:
//   - Home directory expansion fails
//   - Parent directory creation fails
//   - Database initialization fails
func NewManager(ctx context.Context, dbPath string, logger *slog.Logger) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Expand home directory if path starts with ~/
	expandedPath, err := expandHomePath(dbPath)
	if err != nil {
		logger.Error("failed to expand home directory", "path", dbPath, "error", err)
		return nil, fmt.Errorf("expand home path: %w", err)
	}

	// Create parent directory if path is not :memory: and parent doesn't exist
	if expandedPath != ":memory:" && !strings.Contains(expandedPath, "mode=memory") {
		// Validate path to prevent dangerous operations
		if err := validateDatabasePath(expandedPath); err != nil {
			logger.Error("invalid database path", "path", expandedPath, "error", err)
			return nil, fmt.Errorf("validate database path: %w", err)
		}

		dir := filepath.Dir(expandedPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				logger.Error("failed to create directory", "path", dir, "error", err)
				return nil, fmt.Errorf("create directory %s: %w", dir, err)
			}
			logger.Debug("database directory created", "path", dir)
		}
	}

	// Initialize storage with SQLite backend
	cfg := storage.Config{
		Driver: "sqlite",
		DSN:    expandedPath,
		Logger: logger,
	}

	store, err := storage.New(ctx, cfg)
	if err != nil {
		logger.Error("failed to initialize storage", "path", expandedPath, "error", err)
		return nil, fmt.Errorf("initialize storage: %w", err)
	}

	logger.Info("storage manager initialized", "path", expandedPath)
	return &Manager{
		store:  store,
		logger: logger,
	}, nil
}

// Close cleanly shuts down the storage, releasing all resources.
// Returns an error if cleanup fails.
func (m *Manager) Close() error {
	if m.store == nil {
		return nil
	}
	if err := m.store.Close(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to close storage", "error", err)
		}
		return fmt.Errorf("close storage: %w", err)
	}
	if m.logger != nil {
		m.logger.Debug("storage manager closed")
	}
	return nil
}

// Ping verifies that the storage is accessible and operational.
// Returns an error if the store cannot be reached or is not functioning.
func (m *Manager) Ping(ctx context.Context) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.Ping(ctx)
}

// PostOperations methods

// UpsertPost inserts a new post or updates an existing post if it already exists.
// The post ID (post.ID) is used as the unique identifier.
// Returns an error if the operation fails.
func (m *Manager) UpsertPost(ctx context.Context, post *types.Post) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.UpsertPost(ctx, post)
}

// GetPost retrieves a post by its ID (without prefix, e.g., "abc123").
// Returns the post if found, or nil with an error if not found.
func (m *Manager) GetPost(ctx context.Context, id string) (*types.Post, error) {
	if m.store == nil {
		return nil, fmt.Errorf("storage manager not initialized")
	}
	return m.store.GetPost(ctx, id)
}

// ListPosts retrieves posts matching the specified criteria.
// Returns an empty slice if no posts match the criteria.
func (m *Manager) ListPosts(ctx context.Context, opts *storage.ListPostsOptions) ([]*types.Post, error) {
	if m.store == nil {
		return nil, fmt.Errorf("storage manager not initialized")
	}
	return m.store.ListPosts(ctx, opts)
}

// CountPosts returns the total number of posts matching the specified criteria.
func (m *Manager) CountPosts(ctx context.Context, opts *storage.ListPostsOptions) (int64, error) {
	if m.store == nil {
		return 0, fmt.Errorf("storage manager not initialized")
	}
	return m.store.CountPosts(ctx, opts)
}

// DeletePost removes a post by its ID (without prefix, e.g., "abc123").
// Returns an error if the operation fails.
func (m *Manager) DeletePost(ctx context.Context, id string) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.DeletePost(ctx, id)
}

// UpsertPosts performs a batch upsert of multiple posts.
// Each post is inserted or updated based on its ID.
// Returns an error if any operation fails.
func (m *Manager) UpsertPosts(ctx context.Context, posts []*types.Post) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.UpsertPosts(ctx, posts)
}

// CommentOperations methods

// UpsertComment inserts a new comment or updates an existing comment if it already exists.
// The comment ID (comment.ID) is used as the unique identifier.
// Returns an error if the operation fails.
func (m *Manager) UpsertComment(ctx context.Context, comment *types.Comment) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.UpsertComment(ctx, comment)
}

// GetComment retrieves a comment by its ID (without prefix, e.g., "xyz789").
// Returns the comment if found, or nil with an error if not found.
func (m *Manager) GetComment(ctx context.Context, id string) (*types.Comment, error) {
	if m.store == nil {
		return nil, fmt.Errorf("storage manager not initialized")
	}
	return m.store.GetComment(ctx, id)
}

// GetCommentTree retrieves all comments for a specific post, optionally filtered
// and sorted according to the provided options.
// The postID should be without prefix (e.g., "abc123").
// Returns an empty slice if no comments exist for the post.
func (m *Manager) GetCommentTree(ctx context.Context, postID string, opts *storage.CommentTreeOptions) ([]*types.Comment, error) {
	if m.store == nil {
		return nil, fmt.Errorf("storage manager not initialized")
	}
	return m.store.GetCommentTree(ctx, postID, opts)
}

// DeleteComment removes a comment by its ID (without prefix, e.g., "xyz789").
// Returns an error if the operation fails.
func (m *Manager) DeleteComment(ctx context.Context, id string) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.DeleteComment(ctx, id)
}

// UpsertComments performs a batch upsert of multiple comments.
// Each comment is inserted or updated based on its ID.
// Returns an error if any operation fails.
func (m *Manager) UpsertComments(ctx context.Context, comments []*types.Comment) error {
	if m.store == nil {
		return fmt.Errorf("storage manager not initialized")
	}
	return m.store.UpsertComments(ctx, comments)
}

// UtilityOperations methods

// GetStats returns statistics about the stored data.
// Returns an error if the operation fails.
func (m *Manager) GetStats(ctx context.Context) (*storage.CacheStats, error) {
	if m.store == nil {
		return nil, fmt.Errorf("storage manager not initialized")
	}
	return m.store.GetStats(ctx)
}

// EvictStale removes entries older than the specified maxAge.
// Returns the number of entries evicted, or an error if the operation fails.
func (m *Manager) EvictStale(ctx context.Context, maxAge time.Duration) (int64, error) {
	if m.store == nil {
		return 0, fmt.Errorf("storage manager not initialized")
	}
	return m.store.EvictStale(ctx, maxAge)
}

// expandHomePath expands ~ to the user's home directory with path traversal prevention.
// If the path doesn't start with ~, it is returned unchanged.
// Returns an error if the expanded path attempts to escape the home directory.
func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	// Join the home directory with the path suffix
	expanded := filepath.Join(home, path[1:])

	// Convert to absolute paths and check for traversal attacks
	absHome, err := filepath.Abs(home)
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	absExpanded, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve expanded path: %w", err)
	}

	// Ensure the expanded path is within the home directory
	rel, err := filepath.Rel(absHome, absExpanded)
	if err != nil {
		return "", fmt.Errorf("compute relative path: %w", err)
	}

	// If the relative path starts with "..", it escapes the home directory
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path traversal detected: expanded path escapes home directory")
	}

	return absExpanded, nil
}

// validateDatabasePath ensures the database path is safe to use.
// It prevents attempts to access system directories and validates against databases in root directory.
func validateDatabasePath(path string) error {
	// Convert to absolute path for validation
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Get the parent directory
	dir := filepath.Dir(abs)

	// Prevent creating database in root directory (Unix) or Windows volume roots
	if dir == "/" {
		return fmt.Errorf("database path cannot be in root directory")
	}

	// Prevent paths in Windows volume roots (e.g., parent is "C:\\")
	if len(dir) == 3 && dir[1] == ':' && (dir[2] == '\\' || dir[2] == '/') {
		return fmt.Errorf("database path cannot be in root directory")
	}

	return nil
}
