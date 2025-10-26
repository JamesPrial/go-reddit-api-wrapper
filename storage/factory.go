package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal"
)

// registry holds the global factory registry for storage backends.
var registry = internal.NewRegistry[Config, Store]()

// RegisterFactory registers a factory function for a storage driver.
// This is called by backend subpackages (like sqlite) in their init() function.
func RegisterFactory(driver string, factory func(context.Context, Config) (Store, error)) {
	registry.Register(driver, factory)
}

// Config holds configuration for any storage backend.
// All fields have sensible defaults if left as zero values.
type Config struct {
	// Driver specifies the database driver: "sqlite", "sqlite3", "postgres", "postgresql", "pgx"
	// If empty, will be auto-detected from DSN
	Driver string

	// DSN is the data source name / connection string
	// SQLite: ":memory:" or "/path/to/file.db"
	// PostgreSQL: "postgres://user:pass@localhost/dbname" or "host=localhost user=user dbname=dbname"
	DSN string

	// MaxOpenConns sets the maximum number of open connections to the database
	// Default: 25 for file-based DBs, 1 for in-memory SQLite
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections
	// Default: 5 for file-based DBs, 1 for in-memory SQLite
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum lifetime of a connection
	// Default: 0 (connections reused forever)
	ConnMaxLifetime time.Duration

	// MigrationsPath specifies the directory containing migration files
	// If empty, uses default path for the driver
	MigrationsPath string

	// Logger for structured logging
	// If nil, uses slog.Default()
	Logger *slog.Logger
}

// New creates a Store based on the configured driver.
// Supported drivers: sqlite, sqlite3, postgres, postgresql, pgx
// Returns an error if the driver is not supported or if store creation fails.
func New(ctx context.Context, cfg Config) (Store, error) {
	driver := cfg.Driver

	// Auto-detect driver from DSN if not specified
	if driver == "" {
		driver = detectDriver(cfg.DSN)
	}

	// Look up registered factory for the driver
	factory, ok := registry.Get(driver)

	if ok && factory != nil {
		return factory(ctx, cfg)
	}

	// Provide helpful error for known drivers that aren't registered
	switch driver {
	case "sqlite", "sqlite3":
		return nil, fmt.Errorf("SQLite driver not registered; ensure the storage/backends/sqlite subpackage is imported in your main package with 'import _ \"github.com/jamesprial/go-reddit-api-wrapper/storage/backends/sqlite\"'")
	case "postgres", "postgresql", "pgx":
		return nil, fmt.Errorf("PostgreSQL driver not registered; ensure the storage/backends/postgres subpackage is imported in your main package with 'import _ \"github.com/jamesprial/go-reddit-api-wrapper/storage/backends/postgres\"'")
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}

// detectDriver attempts to detect the driver type from the DSN.
// Returns "postgres" for PostgreSQL DSN patterns, "sqlite" as default.
func detectDriver(dsn string) string {
	// PostgreSQL DSN patterns
	if strings.HasPrefix(dsn, "postgres://") ||
		strings.HasPrefix(dsn, "postgresql://") ||
		strings.Contains(dsn, "host=") {
		return "postgres"
	}

	// Default to SQLite for file paths and :memory:
	return "sqlite"
}
