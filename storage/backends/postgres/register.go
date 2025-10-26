// Package postgres registers the PostgreSQL backend with the storage factory.
//
// This package provides transparent registration of the PostgreSQL backend, allowing
// users to use PostgreSQL storage by simply importing this package with a blank import
// in their main package:
//
//	import _ "github.com/jamesprial/go-reddit-api-wrapper/storage/backends/postgres"
//
// Once imported, storage.New() will automatically support "postgres", "postgresql", and "pgx"
// driver names. All database configuration is done through the standard storage.Config
// type (DSN, MaxOpenConns, MaxIdleConns, ConnMaxLifetime, MigrationsPath, Logger).
//
// The default migrations path is "storage/backends/postgres/migrations" if not specified.
package postgres

import (
	"context"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	internalpostgres "github.com/jamesprial/go-reddit-api-wrapper/storage/internal/postgres"
)

func init() {
	// Register factory for "postgres", "postgresql", and "pgx" driver names
	storage.RegisterFactory("postgres", newStoreFactory)
	storage.RegisterFactory("postgresql", newStoreFactory)
	storage.RegisterFactory("pgx", newStoreFactory)
}

// newStoreFactory is the factory function that creates a Store from storage.Config.
// It sets the default migrations path to the backend's migrations directory if not specified,
// then delegates to the internal PostgreSQL implementation.
func newStoreFactory(ctx context.Context, cfg storage.Config) (storage.Store, error) {
	// Update migrations path default to point to the backend's migrations directory
	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "storage/backends/postgres/migrations"
	}

	// Delegate to internal PostgreSQL implementation
	return internalpostgres.NewStore(ctx, cfg)
}
