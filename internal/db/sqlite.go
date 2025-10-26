package db

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds configuration options for database initialization.
type Config struct {
	// Path is the file path to the SQLite database file.
	// Use ":memory:" for an in-memory database (useful for testing).
	Path string

	// EnableDebug enables verbose SQL query logging.
	// When true, all SQL queries will be logged at INFO level.
	// When false, only errors are logged.
	EnableDebug bool
}

// InitDB initializes a SQLite database connection with optimal settings for the Reddit tracker.
// It configures SQLite pragmas for performance and reliability, sets up connection pooling,
// and automatically runs migrations to create/update the schema.
//
// The function configures the following SQLite pragmas:
//   - journal_mode=WAL: Write-Ahead Logging for better concurrency
//   - busy_timeout=5000: Wait up to 5 seconds for locks to clear
//   - foreign_keys=ON: Enable foreign key constraints
//   - synchronous=NORMAL: Balance between safety and performance
//   - cache_size=-20000: Use 20MB cache (negative = KB, positive = pages)
//   - temp_store=MEMORY: Store temporary tables in memory
//
// Connection pool is configured with SetMaxOpenConns(1) because SQLite
// only supports a single writer at a time (WAL mode allows concurrent readers).
//
// After successful initialization, the function automatically runs migrations
// to create or update the database schema. See migrations.go for details.
func InitDB(cfg Config) (*gorm.DB, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("database path cannot be empty")
	}

	// Configure SQLite DSN with optimal pragmas for the Reddit tracker use case
	dsn := cfg.Path + "?" +
		"_journal_mode=WAL&" + // Write-Ahead Logging for better concurrency
		"_busy_timeout=5000&" + // Wait up to 5 seconds for locks
		"_foreign_keys=on&" + // Enable foreign key constraints
		"_synchronous=NORMAL&" + // Balance between safety and performance
		"_cache_size=-20000&" + // 20MB cache (negative = KB)
		"_temp_store=MEMORY" // Temp tables in memory

	// Configure GORM logger based on debug setting
	var gormLogger logger.Interface
	if cfg.EnableDebug {
		// Debug mode: log all SQL queries with detailed info
		gormLogger = logger.New(
			slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo),
			logger.Config{
				SlowThreshold:             200 * time.Millisecond, // Log slow queries
				LogLevel:                  logger.Info,            // Log all queries
				IgnoreRecordNotFoundError: false,                  // Log "record not found" errors
				Colorful:                  false,                  // No color in production logs
			},
		)
	} else {
		// Production mode: only log errors
		gormLogger = logger.New(
			slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
			logger.Config{
				SlowThreshold:             time.Second,  // Only log very slow queries
				LogLevel:                  logger.Error, // Only log errors
				IgnoreRecordNotFoundError: true,         // Don't log "record not found" as errors
				Colorful:                  false,
			},
		)
	}

	// Open database connection with GORM
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		// PrepareStmt caches prepared statements for better performance
		PrepareStmt: true,
		// NowFunc allows overriding time.Now for testing (if needed)
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying SQL database for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying database: %w", err)
	}

	// Configure connection pool
	// SQLite only supports one writer at a time (even with WAL mode),
	// so we limit max open connections to 1 to avoid contention.
	// WAL mode allows multiple concurrent readers, but writes are serialized.
	sqlDB.SetMaxOpenConns(1)
	// Keep connection alive (SQLite is file-based, so connection reuse is cheap)
	sqlDB.SetMaxIdleConns(1)
	// Don't close idle connections (they're cheap to keep open for SQLite)
	sqlDB.SetConnMaxIdleTime(0)
	// No max lifetime (SQLite connections don't need rotation like network DB connections)
	sqlDB.SetConnMaxLifetime(0)

	// Run migrations to create/update schema
	if err := runMigrations(db); err != nil {
		// Close the database connection on migration failure
		if closeErr := sqlDB.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to run migrations: %w (also failed to close db: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}
