// Package db provides the database layer for the Reddit tracker application.
//
// This package implements a SQLite-based persistence layer using GORM for ORM functionality.
// It provides models, repository interfaces, database initialization, and automatic migrations.
//
// # Architecture
//
// The package follows a repository pattern with clear separation of concerns:
//
//   - Models (models.go): GORM model definitions with proper relationships and indexes
//   - Repository (repository.go): Interface-based data access layer with transaction support
//   - Initialization (sqlite.go): Database setup with optimized SQLite configuration
//   - Migrations (migrations.go): Automatic schema creation and updates
//
// # Database Schema
//
// The schema consists of four main tables:
//
//   - Subreddits: Reddit subreddit metadata (name, description, subscribers)
//   - Posts: Reddit posts/submissions (title, author, score, content)
//   - Comments: Reddit comments with self-referential parent-child relationships
//   - TrackingConfig: Configuration for subreddit polling (optional)
//
// All tables use GORM's standard fields (ID, CreatedAt, UpdatedAt, DeletedAt) for
// automatic timestamp management and soft-delete support.
//
// # Indexes
//
// The schema includes strategic indexes for common query patterns:
//
//   - Unique indexes on Reddit fullnames (t3_abc123, t1_xyz789) to prevent duplicates
//   - Single-column indexes on frequently queried fields (author, score, created_utc)
//   - Composite indexes for efficient time-based queries within subreddits/posts
//
// # SQLite Configuration
//
// The database is configured with optimal settings for the Reddit tracker use case:
//
//   - WAL mode for better concurrency (multiple readers, single writer)
//   - Foreign key constraints enabled for referential integrity
//   - 20MB cache size for better performance
//   - 5 second busy timeout to handle lock contention gracefully
//
// # Usage Example
//
//	// Initialize database
//	db, err := db.InitDB(db.Config{
//	    Path:        "reddit_tracker.db",
//	    EnableDebug: false,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create repository
//	repo := db.NewRepository(db)
//
//	// Create a subreddit
//	subreddit := &db.Subreddit{
//	    Fullname:    "t5_2qh33",
//	    Name:        "golang",
//	    Description: "The Go programming language",
//	    Subscribers: 250000,
//	}
//	if err := repo.CreateSubreddit(context.Background(), subreddit); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create a post
//	post := &db.Post{
//	    Fullname:    "t3_abc123",
//	    SubredditID: subreddit.ID,
//	    Title:       "Why Go is awesome",
//	    Author:      "gopher",
//	    Score:       42,
//	    CreatedUTC:  time.Now().UTC(),
//	}
//	if err := repo.CreatePost(context.Background(), post); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use transactions for atomic operations
//	tx, err := repo.BeginTx(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tx.Rollback() // Rollback if not committed
//
//	// Perform multiple operations in transaction
//	if err := tx.CreatePost(context.Background(), post1); err != nil {
//	    log.Fatal(err)
//	}
//	if err := tx.CreatePost(context.Background(), post2); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Commit transaction
//	if err := tx.Commit(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Thread Safety
//
// The repository is safe for concurrent use. SQLite in WAL mode supports multiple
// concurrent readers and a single writer. The connection pool is configured with
// MaxOpenConns(1) to serialize writes while allowing concurrent reads.
//
// # Error Handling
//
// All repository methods return wrapped errors with context about the operation
// that failed. Use errors.Is() or errors.As() to check for specific GORM errors:
//
//	sub, err := repo.GetSubreddit(ctx, "unknown")
//	if errors.Is(err, gorm.ErrRecordNotFound) {
//	    // Handle not found case
//	}
package db
