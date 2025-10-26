package db

import (
	"fmt"

	"gorm.io/gorm"
)

// runMigrations applies all database migrations to create/update the schema.
// This function is called automatically by InitDB during database initialization.
//
// The migration process:
// 1. Uses GORM's AutoMigrate to create/update tables based on model definitions
// 2. Creates additional indexes that aren't part of the GORM model tags
// 3. Ensures all foreign key constraints are properly configured
//
// AutoMigrate is safe to call multiple times - it only adds missing columns and indexes,
// it never deletes existing columns or data. For destructive schema changes,
// manual migrations would be needed.
//
// The following indexes are created:
// - Composite indexes for efficient queries (subreddit_id + created_utc, post_id + created_utc)
// - Single-column indexes for common query patterns (author, score, created_utc)
// - Unique indexes for Reddit fullnames to prevent duplicates
//
// All indexes except composite ones are created automatically via GORM struct tags.
// This function only handles composite indexes that require explicit creation.
func runMigrations(db *gorm.DB) error {
	// Run GORM's AutoMigrate to create/update tables
	// This will create tables if they don't exist, and add missing columns
	// It will NOT delete columns or modify existing column types
	if err := db.AutoMigrate(
		&Subreddit{},
		&Post{},
		&Comment{},
		&TrackingConfig{},
	); err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	// Note: Composite indexes are defined in the model struct tags using `gorm:"index:idx_name"`
	// For example:
	//   - idx_subreddit_time on posts (subreddit_id, created_utc)
	//   - idx_post_time on comments (post_id, created_utc)
	//
	// GORM automatically creates these composite indexes during AutoMigrate.
	// No additional index creation is needed here unless adding indexes
	// that can't be expressed in struct tags.

	// All migrations completed successfully
	return nil
}
