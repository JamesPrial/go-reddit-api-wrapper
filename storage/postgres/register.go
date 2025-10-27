// Package postgres registers the PostgreSQL backend with the storage factory.
//
// This package provides transparent registration of the PostgreSQL backend, allowing
// users to use PostgreSQL storage by simply importing this package with a blank import
// in their main package:
//
//	import _ "github.com/jamesprial/go-reddit-api-wrapper/storage/postgres"
//
// Once imported, storage.New() will automatically support "postgres", "postgresql", and "pgx"
// driver names. All database configuration is done through the standard storage.Config
// type (DSN, MaxOpenConns, MaxIdleConns, ConnMaxLifetime, Logger).
package postgres

import (
	"context"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	internalpostgres "github.com/jamesprial/go-reddit-api-wrapper/storage/postgres/internal"
)

func init() {
	// Register factory for "postgres", "postgresql", and "pgx" driver names
	storage.RegisterFactory("postgres", newStoreFactory)
	storage.RegisterFactory("postgresql", newStoreFactory)
	storage.RegisterFactory("pgx", newStoreFactory)
}

// newStoreFactory is the factory function that creates a Store from storage.Config.
// It delegates to the internal PostgreSQL implementation.
func newStoreFactory(ctx context.Context, cfg storage.Config) (storage.Store, error) {
	// Delegate to internal PostgreSQL implementation
	return internalpostgres.NewStore(ctx, cfg)
}
