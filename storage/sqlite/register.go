// Package sqlite registers the SQLite backend with the storage factory.
//
// This package provides transparent registration of the SQLite backend, allowing
// users to use SQLite storage by simply importing this package with a blank import
// in their main package:
//
//	import _ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
//
// Once imported, storage.New() will automatically support "sqlite" and "sqlite3"
// driver names. All database configuration is done through the standard storage.Config
// type (DSN, MaxOpenConns, MaxIdleConns, ConnMaxLifetime, MigrationsPath, Logger).
//
// The default migrations path is "storage/sqlite/migrations" if not specified.
package sqlite

import (
	"context"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	internalsqlite "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite/internal"
)

func init() {
	// Register factory for both "sqlite" and "sqlite3" driver names
	storage.RegisterFactory("sqlite", newStoreFactory)
	storage.RegisterFactory("sqlite3", newStoreFactory)
}

// newStoreFactory is the factory function that creates a Store from storage.Config.
// It sets the default migrations path to the backend's migrations directory if not specified,
// then delegates to the internal SQLite implementation.
func newStoreFactory(ctx context.Context, cfg storage.Config) (storage.Store, error) {
	// Update migrations path default to point to the backend's migrations directory
	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "storage/sqlite/migrations"
	}

	// Delegate to internal SQLite implementation
	return internalsqlite.NewStore(ctx, cfg)
}
