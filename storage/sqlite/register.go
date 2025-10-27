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
// type (DSN, MaxOpenConns, MaxIdleConns, ConnMaxLifetime, Logger).
//
// Migrations are embedded at compile time and automatically applied when the store is created.
package sqlite

import (
	"context"
	"database/sql"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	internalsqlite "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite/internal"
)

// SQLiteStore is the public type alias for the internal SQLiteStore implementation.
// It is exported here to allow testutil packages to access it for testing purposes.
type SQLiteStore = internalsqlite.SQLiteStore

func init() {
	// Set the embedded migrations filesystem in the internal package
	internalsqlite.SetMigrationsFS(migrationsFS)

	// Register factory for both "sqlite" and "sqlite3" driver names
	storage.RegisterFactory("sqlite", newStoreFactory)
	storage.RegisterFactory("sqlite3", newStoreFactory)
}

// newStoreFactory is the factory function that creates a Store from storage.Config.
// It delegates to the internal SQLite implementation.
func newStoreFactory(ctx context.Context, cfg storage.Config) (storage.Store, error) {
	// Delegate to internal SQLite implementation
	return internalsqlite.NewStore(ctx, cfg)
}

// GetDB returns the underlying *sql.DB connection from a SQLiteStore.
// This is exported for testing purposes only and should not be used in production code.
// It allows test helpers to directly query the database for verification purposes.
func GetDB(s *SQLiteStore) *sql.DB {
	return internalsqlite.GetDB((*internalsqlite.SQLiteStore)(s))
}
