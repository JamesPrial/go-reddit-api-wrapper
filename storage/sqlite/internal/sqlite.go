package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/mattn/go-sqlite3"
)

// migrationsFS holds the embedded migrations. It is set by init() in the public package.
var migrationsFS embed.FS

const (
	DEFAULT_DB_PATH            = "reddit.db"
	DEFAULT_MAX_OPEN_CONNS     = 10
	DEFAULT_MAX_IDLE_CONNS     = 10
	DEFAULT_CONN_MAX_LIFE      = 0
	DEFAULT_CONN_MAX_IDLE_TIME = 5 * time.Minute
)

// Config holds configuration options for SQLiteStore.
// All fields have sensible defaults if left as zero values.
type Config struct {
	// DBPath specifies the path to the SQLite database file.
	// Use ":memory:" for an in-memory database (useful for testing).
	//
	// IMPORTANT: In-memory databases are automatically configured with a single shared connection
	// (MaxOpenConns=1, MaxIdleConns=1) regardless of Config values, because SQLite in-memory
	// databases are isolated per connection. This means:
	//   - No concurrent database operations (all operations are serialized)
	//   - WAL journal mode is not available (SQLite uses default mode instead)
	//   - Suitable for testing but not for production workloads requiring concurrency
	//
	// Supports standard ":memory:" format and URI formats like "file::memory:" or "file:name?mode=memory".
	// If empty, defaults to "reddit.db" in the current directory.
	DBPath string

	// MaxOpenConns sets the maximum number of open connections to the database.
	// If <= 0, defaults to 10.
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections.
	// If <= 0, defaults to 10.
	MaxIdleConns int

	// ConnMaxLife sets the maximum amount of time a connection may be reused.
	// If <= 0, connections are not closed due to age (unlimited lifetime).
	ConnMaxLife time.Duration

	// ConnMaxIdleTime sets the maximum amount of time a connection may sit idle.
	// If <= 0, defaults to 5 minutes.
	ConnMaxIdleTime time.Duration

	// Logger is used for structured logging.
	// If nil, uses slog.Default().
	Logger *slog.Logger
}

// SQLiteStore implements the Store interface using SQLite for persistent storage.
// It provides thread-safe operations for storing and retrieving Reddit posts and comments.
type SQLiteStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewStore creates and initializes a new SQLiteStore from storage.Config.
// It accepts the generic storage.Config type and adapts it to SQLite-specific configuration.
// This function is used by the storage factory pattern and should be preferred for generic code.
// It opens the database, configures the connection pool, runs migrations, and returns the store.
// Returns an error if database initialization or migration fails.
func NewStore(ctx context.Context, cfg storage.Config) (storage.Store, error) {
	// Convert storage.Config to sqlite-specific Config
	sqliteConfig := &Config{
		DBPath:       cfg.DSN,
		MaxOpenConns: cfg.MaxOpenConns,
		MaxIdleConns: cfg.MaxIdleConns,
		ConnMaxLife:  cfg.ConnMaxLifetime,
		Logger:       cfg.Logger,
	}

	return NewSQLiteStore(sqliteConfig)
}

// NewSQLiteStore creates and initializes a new SQLiteStore.
// It opens the database, configures the connection pool, runs migrations, and returns the store.
// Returns an error if database initialization or migration fails.
func NewSQLiteStore(cfg *Config) (*SQLiteStore, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Apply default values
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = DEFAULT_DB_PATH
	}

	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = DEFAULT_MAX_OPEN_CONNS
	}

	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = DEFAULT_MAX_IDLE_CONNS
	}

	connMaxIdleTime := cfg.ConnMaxIdleTime
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = DEFAULT_CONN_MAX_IDLE_TIME
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// For in-memory databases, force a single connection to ensure schema consistency
	// SQLite in-memory databases are isolated per connection, so we must use exactly one connection
	// to ensure migrations and queries see the same schema
	// Detect all in-memory formats: ":memory:", "file::memory:", and "file:name?mode=memory"
	isMemory := dbPath == ":memory:" ||
		strings.Contains(dbPath, "mode=memory") ||
		strings.Contains(dbPath, "file::memory:")
	if isMemory {
		maxOpenConns = 1
		maxIdleConns = 1
		logger.Info("detected in-memory database, forcing single connection for schema consistency")
	}

	// Build DSN with SQLite pragmas for optimal performance
	var dsn string
	if isMemory {
		// For in-memory databases, handle different formats
		// Note: WAL journal mode is not supported for in-memory databases
		// Note: _busy_timeout is unnecessary with single connection (no lock contention)
		if strings.HasPrefix(dbPath, "file:") {
			// URI format is already provided, just add foreign keys if not present
			if strings.Contains(dbPath, "?") {
				dsn = fmt.Sprintf("%s&_foreign_keys=ON", dbPath)
			} else {
				dsn = fmt.Sprintf("%s?_foreign_keys=ON", dbPath)
			}
		} else if dbPath == ":memory:" {
			// Standard memory format, use shared cache
			dsn = "file::memory:?cache=shared&_foreign_keys=ON"
		} else {
			// Other format, wrap in file: URI
			dsn = fmt.Sprintf("file:%s?cache=shared&mode=memory&_foreign_keys=ON", dbPath)
		}
	} else {
		// For file-based databases, use standard pragmas
		// _journal_mode=WAL: Write-Ahead Logging enables concurrent reads during writes
		// _foreign_keys=ON: Enforce foreign key constraints (important for referential integrity)
		// _busy_timeout=5000: Wait up to 5 seconds if database is locked
		dsn = fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000", dbPath)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "NewSQLiteStore", Message: "failed to open database", Err: err}
	}

	// Configure connection pool
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLife)

	// SetConnMaxIdleTime closes idle connections after the specified duration to free resources.
	// Matching MaxIdleConns to MaxOpenConns with an idle timeout reduces connection churn
	// and improves performance under high-traffic scenarios.
	// However, for in-memory databases, we skip this to keep the single connection alive indefinitely,
	// since in-memory databases disappear when the connection closes.
	if !isMemory {
		db.SetConnMaxIdleTime(connMaxIdleTime)
	}

	logAttrs := []any{
		"db_path", dbPath,
		"max_open_conns", maxOpenConns,
		"max_idle_conns", maxIdleConns,
		"conn_max_life", cfg.ConnMaxLife,
	}
	if !isMemory {
		logAttrs = append(logAttrs, "conn_max_idle_time", connMaxIdleTime)
	}
	logger.Info("database connection opened", logAttrs...)

	// Create store instance
	store := &SQLiteStore{
		db:     db,
		logger: logger,
	}

	// Run database migrations
	if err := store.runMigrations(); err != nil {
		db.Close() // Clean up on migration failure
		return nil, &storage.DatabaseError{Operation: "NewSQLiteStore", Message: "failed to run migrations", Err: err}
	}

	logger.Info("database migrations completed")

	// After migrations run successfully, verify indexes
	if err := store.verifyIndexes(); err != nil {
		db.Close() // Clean up on index verification failure
		return nil, err
	}

	return store, nil
}

// runMigrations applies all pending database migrations using golang-migrate.
// It uses embedded migrations bundled at compile time, so they are always available.
// Returns an error if migration setup or execution fails.
// Returns nil if migrations are already up-to-date (ErrNoChange is not considered an error).
func (s *SQLiteStore) runMigrations() error {
	// Create a database driver instance for golang-migrate
	driver, err := sqlite3.WithInstance(s.db, &sqlite3.Config{})
	if err != nil {
		return &storage.DatabaseError{Operation: "runMigrations", Message: "failed to create migration driver", Err: err}
	}

	// Create source from embedded FS
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return &storage.DatabaseError{Operation: "runMigrations", Message: "failed to create embedded migrations source", Err: err}
	}

	// Create migrate instance with embedded source and database driver
	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", driver)
	if err != nil {
		return &storage.DatabaseError{Operation: "runMigrations", Message: "failed to create migrate instance", Err: err}
	}

	// Apply all pending UP migrations
	if err := m.Up(); err != nil {
		// ErrNoChange is not an error - it means we're already up-to-date
		if err == migrate.ErrNoChange {
			s.logger.Debug("database migrations already up-to-date")
			return nil
		}
		return &storage.DatabaseError{Operation: "runMigrations", Message: "migration failed", Err: err}
	}

	return nil
}

// Close cleanly shuts down the database connection pool.
// It waits for all open connections to be returned or closed.
// Should be called when the store is no longer needed.
// Returns an error if the database fails to close properly.
func (s *SQLiteStore) Close() error {
	s.logger.Info("closing database connection")
	if err := s.db.Close(); err != nil {
		return &storage.DatabaseError{Operation: "Close", Message: "failed to close database", Err: err}
	}
	return nil
}

// Ping verifies that the database is accessible and operational.
// It executes a simple query to test connectivity.
// Returns an error if the database cannot be reached or is not responding.
func (s *SQLiteStore) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return &storage.DatabaseError{Operation: "Ping", Message: "database ping failed", Err: err}
	}
	return nil
}

// SetMigrationsFS sets the embedded migrations filesystem.
// This is called by the public package's init() function.
func SetMigrationsFS(fs embed.FS) {
	migrationsFS = fs
}

// SavePostSnapshot stores a snapshot of a post's current state.
// The snapshot contains immutable data about the post at a specific point in time.
// Returns an error if the operation fails.
func (s *SQLiteStore) SavePostSnapshot(ctx context.Context, snapshot *storage.PostSnapshot) error {
	if snapshot == nil {
		return &storage.ValidationError{Operation: "SavePostSnapshot", Field: "snapshot", Reason: "snapshot cannot be nil"}
	}
	if snapshot.PostID == "" {
		return &storage.ValidationError{Operation: "SavePostSnapshot", Field: "snapshot.PostID", Reason: "post ID cannot be empty"}
	}
	if snapshot.Fullname == "" {
		return &storage.ValidationError{Operation: "SavePostSnapshot", Field: "snapshot.Fullname", Reason: "fullname cannot be empty"}
	}

	// Validate numeric fields
	if snapshot.NumComments < 0 {
		return &storage.ValidationError{
			Operation: "SavePostSnapshot",
			Field:     "snapshot.NumComments",
			Reason:    "comment count cannot be negative",
		}
	}
	// Note: Score can be negative on Reddit (downvoted posts), so no validation

	s.logger.Debug("saving post snapshot", "post_id", snapshot.PostID, "num_comments", snapshot.NumComments, "score", snapshot.Score)

	result, err := s.db.ExecContext(ctx, insertPostSnapshotQuery, snapshot.PostID, snapshot.Fullname, snapshot.NumComments, snapshot.Score)
	if err != nil {
		return &storage.DatabaseError{Operation: "SavePostSnapshot", Message: fmt.Sprintf("failed to insert snapshot for post %s", snapshot.PostID), Err: err}
	}

	// Get the last inserted ID for logging purposes
	id, err := result.LastInsertId()
	if err != nil {
		s.logger.Warn("failed to retrieve snapshot ID", "post_id", snapshot.PostID, "error", err)
	} else {
		s.logger.Debug("successfully saved post snapshot", "post_id", snapshot.PostID, "snapshot_id", id)
	}

	return nil
}

// GetLatestSnapshot retrieves the most recent snapshot for a post.
// The postID should be without prefix (e.g., "abc123").
// Returns nil if no snapshot exists for the post (not an error).
// Returns an error if the operation fails.
func (s *SQLiteStore) GetLatestSnapshot(ctx context.Context, postID string) (*storage.PostSnapshot, error) {
	if postID == "" {
		return nil, &storage.ValidationError{Operation: "GetLatestSnapshot", Field: "postID", Reason: "post ID cannot be empty"}
	}

	s.logger.Debug("getting latest snapshot", "post_id", postID)

	row := s.db.QueryRowContext(ctx, selectLatestSnapshotQuery, postID)

	var snapshot storage.PostSnapshot
	var createdAtUnix int64

	err := row.Scan(&snapshot.ID, &snapshot.PostID, &snapshot.Fullname, &snapshot.NumComments, &snapshot.Score, &createdAtUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			s.logger.Debug("no snapshot found for post", "post_id", postID)
			return nil, nil
		}
		return nil, &storage.DatabaseError{Operation: "GetLatestSnapshot", Message: fmt.Sprintf("failed to query snapshot for post %s", postID), Err: err}
	}

	// Convert Unix timestamp (seconds) to time.Time
	if createdAtUnix < 0 {
		return nil, &storage.DatabaseError{
			Operation: "GetLatestSnapshot",
			Message:   fmt.Sprintf("invalid snapshot timestamp for post %s: negative Unix timestamp %d", postID, createdAtUnix),
			Err:       nil,
		}
	}
	snapshot.CreatedAt = time.Unix(createdAtUnix, 0).UTC()

	s.logger.Debug("successfully retrieved latest snapshot", "post_id", postID, "snapshot_id", snapshot.ID)
	return &snapshot, nil
}

// SaveCommentChangeEvent records when new comments are detected for a post.
// The event captures the detected change in comment count between snapshots.
// Returns an error if the operation fails.
func (s *SQLiteStore) SaveCommentChangeEvent(ctx context.Context, event *storage.CommentChangeEvent) error {
	if event == nil {
		return &storage.ValidationError{Operation: "SaveCommentChangeEvent", Field: "event", Reason: "event cannot be nil"}
	}
	if event.PostID == "" {
		return &storage.ValidationError{Operation: "SaveCommentChangeEvent", Field: "event.PostID", Reason: "post ID cannot be empty"}
	}
	if event.Fullname == "" {
		return &storage.ValidationError{Operation: "SaveCommentChangeEvent", Field: "event.Fullname", Reason: "fullname cannot be empty"}
	}

	// Validate numeric fields
	if event.PreviousCount < 0 {
		return &storage.ValidationError{
			Operation: "SaveCommentChangeEvent",
			Field:     "event.PreviousCount",
			Reason:    "previous count cannot be negative",
		}
	}
	if event.NewCount < 0 {
		return &storage.ValidationError{
			Operation: "SaveCommentChangeEvent",
			Field:     "event.NewCount",
			Reason:    "new count cannot be negative",
		}
	}
	// Validate that the delta is consistent
	if event.CommentsAdded != (event.NewCount - event.PreviousCount) {
		return &storage.ValidationError{
			Operation: "SaveCommentChangeEvent",
			Field:     "event.CommentsAdded",
			Reason:    fmt.Sprintf("comments_added (%d) does not match delta between new_count (%d) and previous_count (%d)", event.CommentsAdded, event.NewCount, event.PreviousCount),
		}
	}

	s.logger.Debug("saving comment change event", "post_id", event.PostID, "previous_count", event.PreviousCount, "new_count", event.NewCount, "comments_added", event.CommentsAdded)

	result, err := s.db.ExecContext(ctx, insertCommentChangeEventQuery, event.PostID, event.Fullname, event.PreviousCount, event.NewCount, event.CommentsAdded)
	if err != nil {
		return &storage.DatabaseError{Operation: "SaveCommentChangeEvent", Message: fmt.Sprintf("failed to insert change event for post %s", event.PostID), Err: err}
	}

	// Get the last inserted ID for logging purposes
	id, err := result.LastInsertId()
	if err != nil {
		s.logger.Warn("failed to retrieve change event ID", "post_id", event.PostID, "error", err)
	} else {
		s.logger.Debug("successfully saved comment change event", "post_id", event.PostID, "event_id", id)
	}

	return nil
}

// GetCommentChangeEvents retrieves all change events for a post, ordered by most recent first.
// The postID should be without prefix (e.g., "abc123").
// The limit parameter specifies the maximum number of events to return.
// Returns an empty slice if no events exist for the post (not an error).
// Returns an error if the operation fails.
func (s *SQLiteStore) GetCommentChangeEvents(ctx context.Context, postID string, limit int) ([]*storage.CommentChangeEvent, error) {
	if postID == "" {
		return nil, &storage.ValidationError{Operation: "GetCommentChangeEvents", Field: "postID", Reason: "post ID cannot be empty"}
	}
	if limit < 1 {
		return nil, &storage.ValidationError{Operation: "GetCommentChangeEvents", Field: "limit", Reason: "limit must be greater than 0"}
	}

	s.logger.Debug("getting comment change events", "post_id", postID, "limit", limit)

	rows, err := s.db.QueryContext(ctx, selectCommentChangeEventsQuery, postID, limit)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "GetCommentChangeEvents", Message: fmt.Sprintf("failed to query change events for post %s", postID), Err: err}
	}
	defer rows.Close()

	// Initialize as empty slice to return [] instead of nil when no rows
	events := make([]*storage.CommentChangeEvent, 0)

	for rows.Next() {
		var event storage.CommentChangeEvent
		var detectedAtUnix int64

		err := rows.Scan(&event.ID, &event.PostID, &event.Fullname, &detectedAtUnix, &event.PreviousCount, &event.NewCount, &event.CommentsAdded)
		if err != nil {
			return nil, &storage.DatabaseError{Operation: "GetCommentChangeEvents", Message: "failed to scan change event row", Err: err}
		}

		// Convert Unix timestamp (seconds) to time.Time
		if detectedAtUnix < 0 {
			return nil, &storage.DatabaseError{
				Operation: "GetCommentChangeEvents",
				Message:   fmt.Sprintf("invalid change event timestamp for post %s: negative Unix timestamp %d", postID, detectedAtUnix),
				Err:       nil,
			}
		}
		event.DetectedAt = time.Unix(detectedAtUnix, 0).UTC()

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, &storage.DatabaseError{Operation: "GetCommentChangeEvents", Message: "error iterating over change event rows", Err: err}
	}

	s.logger.Debug("successfully retrieved comment change events", "post_id", postID, "count", len(events))
	return events, nil
}

// verifyIndexes checks that required indexes exist in the database.
// This ensures migrations completed successfully and queries will perform well.
func (s *SQLiteStore) verifyIndexes() error {
	requiredIndexes := []string{
		"idx_post_snapshots_post_created",
		"idx_comment_change_events_post_detected",
	}

	for _, idx := range requiredIndexes {
		var exists int
		query := "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?"
		err := s.db.QueryRow(query, idx).Scan(&exists)
		if err != nil || exists == 0 {
			return &storage.DatabaseError{
				Operation: "verifyIndexes",
				Message:   fmt.Sprintf("required index %s is missing", idx),
				Err:       err,
			}
		}
	}
	return nil
}
