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
