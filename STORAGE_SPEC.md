# Storage Backend Implementation Specification

## Executive Summary

This document provides comprehensive specifications for adding SQLite and PostgreSQL storage backends to the Go Reddit API wrapper. The implementation will enable data persistence, caching, and offline analysis while maintaining complete backward compatibility with existing code.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Database Schema Design](#database-schema-design)
3. [Repository Pattern Implementation](#repository-pattern-implementation)
4. [Integration Strategy](#integration-strategy)
5. [Migration Management](#migration-management)
6. [Caching Strategies](#caching-strategies)
7. [Implementation Phases](#implementation-phases)
8. [Testing Strategy](#testing-strategy)
9. [Code Examples](#code-examples)
10. [Performance Considerations](#performance-considerations)

## Architecture Overview

### Design Principles

1. **Optional Storage**: Storage is completely opt-in, with zero impact on existing users
2. **Interface-Based**: Repository pattern abstracts database implementation details
3. **Backward Compatible**: No breaking changes to existing API
4. **Performance-Focused**: Batch operations, connection pooling, strategic indexing
5. **Testable**: Mock implementations for unit testing, in-memory DB for integration tests

### Package Structure

```
/go-reddit-api-wrapper/
├── pkg/
│   └── storage/
│       ├── repository.go          # Repository interface definition
│       ├── options.go              # Query options and configuration
│       ├── middleware.go           # Storage middleware for caching
│       ├── hooks.go                # Event hooks for custom logic
│       ├── errors.go               # Storage-specific error types
│       ├── migrations/
│       │   ├── sqlite/
│       │   │   ├── 000001_initial_schema.up.sql
│       │   │   ├── 000001_initial_schema.down.sql
│       │   │   ├── 000002_add_indexes.up.sql
│       │   │   └── 000002_add_indexes.down.sql
│       │   └── postgres/
│       │       ├── 000001_initial_schema.up.sql
│       │       ├── 000001_initial_schema.down.sql
│       │       ├── 000002_add_indexes.up.sql
│       │       └── 000002_add_indexes.down.sql
│       ├── sqlite/
│       │   ├── repository.go      # SQLite implementation
│       │   ├── queries.go         # SQLite-specific queries
│       │   └── repository_test.go
│       ├── postgres/
│       │   ├── repository.go      # PostgreSQL implementation
│       │   ├── queries.go         # PostgreSQL-specific queries
│       │   └── repository_test.go
│       └── memory/
│           ├── repository.go      # In-memory implementation for testing
│           └── repository_test.go
├── examples/
│   └── storage/
│       ├── sqlite_example.go
│       ├── postgres_example.go
│       └── migration_example.go
└── reddit.go                       # Modified to include storage configuration
```

## Database Schema Design

### Posts Table

```sql
-- SQLite version
CREATE TABLE posts (
    id TEXT PRIMARY KEY,              -- Reddit ID without t3_ prefix
    name TEXT UNIQUE NOT NULL,        -- Full name with t3_ prefix
    subreddit TEXT NOT NULL,
    subreddit_id TEXT NOT NULL,
    author TEXT NOT NULL,
    title TEXT NOT NULL,
    selftext TEXT,
    url TEXT,
    score INTEGER NOT NULL,
    ups INTEGER NOT NULL,
    downs INTEGER NOT NULL,
    num_comments INTEGER NOT NULL,
    created_utc REAL NOT NULL,
    permalink TEXT NOT NULL,

    -- Boolean flags
    is_self INTEGER NOT NULL,         -- SQLite uses 0/1 for booleans
    over18 INTEGER NOT NULL,
    locked INTEGER NOT NULL,
    stickied INTEGER NOT NULL,
    archived INTEGER NOT NULL DEFAULT 0,
    hidden INTEGER NOT NULL DEFAULT 0,
    saved INTEGER NOT NULL DEFAULT 0,

    -- Nullable fields
    distinguished TEXT,                -- 'moderator', 'admin', null
    edited_timestamp REAL,             -- NULL if not edited

    -- JSON fields stored as TEXT
    media TEXT,                        -- JSON string
    author_flair TEXT,                 -- JSON string
    link_flair TEXT,                   -- JSON string

    -- Storage metadata
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_posts_subreddit ON posts(subreddit);
CREATE INDEX idx_posts_author ON posts(author);
CREATE INDEX idx_posts_created ON posts(created_utc DESC);
CREATE INDEX idx_posts_score ON posts(score DESC);
CREATE INDEX idx_posts_fetched ON posts(fetched_at);
CREATE INDEX idx_posts_subreddit_created ON posts(subreddit, created_utc DESC);

-- PostgreSQL version
CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    subreddit TEXT NOT NULL,
    subreddit_id TEXT NOT NULL,
    author TEXT NOT NULL,
    title TEXT NOT NULL,
    selftext TEXT,
    url TEXT,
    score INTEGER NOT NULL,
    ups INTEGER NOT NULL,
    downs INTEGER NOT NULL,
    num_comments INTEGER NOT NULL,
    created_utc DOUBLE PRECISION NOT NULL,
    permalink TEXT NOT NULL,

    -- Boolean flags (native PostgreSQL boolean)
    is_self BOOLEAN NOT NULL,
    over18 BOOLEAN NOT NULL,
    locked BOOLEAN NOT NULL,
    stickied BOOLEAN NOT NULL,
    archived BOOLEAN NOT NULL DEFAULT false,
    hidden BOOLEAN NOT NULL DEFAULT false,
    saved BOOLEAN NOT NULL DEFAULT false,

    -- Nullable fields
    distinguished TEXT,
    edited_timestamp DOUBLE PRECISION,

    -- JSONB for better query performance
    media JSONB,
    author_flair JSONB,
    link_flair JSONB,

    -- Storage metadata
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- PostgreSQL indexes (same as SQLite plus JSONB indexes)
CREATE INDEX idx_posts_media ON posts USING GIN (media);
CREATE INDEX idx_posts_flair ON posts USING GIN (author_flair, link_flair);
```

### Comments Table (Adjacency List Pattern)

```sql
-- SQLite version
CREATE TABLE comments (
    id TEXT PRIMARY KEY,              -- Reddit ID without t1_ prefix
    name TEXT UNIQUE NOT NULL,        -- Full name with t1_ prefix
    link_id TEXT NOT NULL,            -- Post ID with t3_ prefix
    parent_id TEXT NOT NULL,          -- Parent ID with prefix (t1_ or t3_)
    subreddit TEXT NOT NULL,
    subreddit_id TEXT NOT NULL,
    author TEXT NOT NULL,
    body TEXT NOT NULL,
    body_html TEXT NOT NULL,
    score INTEGER NOT NULL,
    ups INTEGER NOT NULL,
    downs INTEGER NOT NULL,
    gilded INTEGER NOT NULL DEFAULT 0,
    created_utc REAL NOT NULL,

    -- Tree structure metadata
    depth INTEGER NOT NULL DEFAULT 0,  -- Distance from root (post)

    -- Boolean flags
    score_hidden INTEGER NOT NULL,
    saved INTEGER NOT NULL DEFAULT 0,
    removed INTEGER NOT NULL DEFAULT 0,
    approved INTEGER NOT NULL DEFAULT 0,

    -- Nullable fields
    distinguished TEXT,
    edited_timestamp REAL,
    removal_reason TEXT,

    -- Storage metadata
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_comments_link ON comments(link_id);
CREATE INDEX idx_comments_parent ON comments(parent_id);
CREATE INDEX idx_comments_author ON comments(author);
CREATE INDEX idx_comments_created ON comments(created_utc DESC);
CREATE INDEX idx_comments_score ON comments(score DESC);
CREATE INDEX idx_comments_tree ON comments(link_id, parent_id, created_utc);
CREATE INDEX idx_comments_depth ON comments(link_id, depth);

-- PostgreSQL version (similar with BOOLEAN type and TIMESTAMPTZ)
```

### Subreddits Table

```sql
-- SQLite version
CREATE TABLE subreddits (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,        -- Full name with t5_ prefix
    display_name TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    subscribers INTEGER NOT NULL,
    accounts_active INTEGER,
    description TEXT,
    public_description TEXT,
    subreddit_type TEXT NOT NULL,     -- 'public', 'private', 'restricted'
    over18 INTEGER NOT NULL,
    quarantine INTEGER NOT NULL DEFAULT 0,
    url TEXT NOT NULL,
    created_utc REAL NOT NULL,

    -- User relationship fields
    user_is_banned INTEGER DEFAULT 0,
    user_is_moderator INTEGER DEFAULT 0,
    user_is_subscriber INTEGER DEFAULT 0,

    -- JSON fields
    submission_type TEXT,              -- JSON for allowed post types
    rules TEXT,                        -- JSON for subreddit rules

    -- Storage metadata
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_subreddits_name ON subreddits(display_name);
CREATE INDEX idx_subreddits_subscribers ON subreddits(subscribers DESC);
CREATE INDEX idx_subreddits_type ON subreddits(subreddit_type, over18);
```

### Accounts Table

```sql
-- SQLite version
CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,        -- Username
    comment_karma INTEGER NOT NULL,
    link_karma INTEGER NOT NULL,
    total_karma INTEGER GENERATED ALWAYS AS (comment_karma + link_karma) STORED,
    is_gold INTEGER NOT NULL,
    is_mod INTEGER NOT NULL,
    is_employee INTEGER NOT NULL DEFAULT 0,
    is_suspended INTEGER NOT NULL DEFAULT 0,
    verified INTEGER NOT NULL DEFAULT 0,
    over_18 INTEGER NOT NULL,
    created_utc REAL NOT NULL,

    -- Profile fields
    icon_img TEXT,
    banner_img TEXT,

    -- Storage metadata
    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_accounts_name ON accounts(name);
CREATE INDEX idx_accounts_karma ON accounts(total_karma DESC);
```

### More Children Table (for deferred comment loading)

```sql
CREATE TABLE more_children (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    link_id TEXT NOT NULL,
    parent_id TEXT NOT NULL,
    children TEXT NOT NULL,            -- JSON array of comment IDs
    count INTEGER NOT NULL,
    depth INTEGER NOT NULL,

    fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(link_id, parent_id)
);

CREATE INDEX idx_more_children_link ON more_children(link_id);
```

## Repository Pattern Implementation

### Core Interface

```go
// pkg/storage/repository.go
package storage

import (
    "context"
    "time"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Repository defines the storage interface for Reddit data
type Repository interface {
    // Post operations
    SavePost(ctx context.Context, post *types.Post) error
    SavePosts(ctx context.Context, posts []*types.Post) error
    GetPost(ctx context.Context, id string) (*types.Post, error)
    GetPostByName(ctx context.Context, name string) (*types.Post, error)
    GetPostsBySubreddit(ctx context.Context, subreddit string, opts *QueryOptions) ([]*types.Post, error)
    GetPostsByAuthor(ctx context.Context, author string, opts *QueryOptions) ([]*types.Post, error)
    UpdatePostScores(ctx context.Context, id string, score, ups, downs, numComments int) error
    DeletePost(ctx context.Context, id string) error

    // Comment operations
    SaveComment(ctx context.Context, comment *types.Comment) error
    SaveComments(ctx context.Context, comments []*types.Comment) error
    SaveCommentTree(ctx context.Context, postID string, comments []*types.Comment) error
    GetComment(ctx context.Context, id string) (*types.Comment, error)
    GetCommentsByPost(ctx context.Context, postID string, opts *QueryOptions) ([]*types.Comment, error)
    GetCommentsByAuthor(ctx context.Context, author string, opts *QueryOptions) ([]*types.Comment, error)
    GetCommentTree(ctx context.Context, postID string, maxDepth int) ([]*types.Comment, error)
    DeleteComment(ctx context.Context, id string) error

    // Subreddit operations
    SaveSubreddit(ctx context.Context, sub *types.SubredditData) error
    GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error)
    GetSubreddits(ctx context.Context, opts *QueryOptions) ([]*types.SubredditData, error)
    UpdateSubredditStats(ctx context.Context, name string, subscribers int64, active int) error

    // Account operations
    SaveAccount(ctx context.Context, account *types.AccountData) error
    GetAccount(ctx context.Context, name string) (*types.AccountData, error)
    GetAccounts(ctx context.Context, opts *QueryOptions) ([]*types.AccountData, error)

    // More children operations (for deferred comment loading)
    SaveMoreChildren(ctx context.Context, linkID, parentID string, childIDs []string, count int) error
    GetMoreChildren(ctx context.Context, linkID, parentID string) ([]string, error)

    // Query operations
    Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error)
    GetStats(ctx context.Context) (*DatabaseStats, error)

    // Maintenance operations
    Vacuum(ctx context.Context) error
    PruneOldData(ctx context.Context, before time.Time) error

    // Transaction support
    BeginTx(ctx context.Context) (Transaction, error)

    // Lifecycle
    Close() error
}

// Transaction represents a database transaction
type Transaction interface {
    Repository
    Commit() error
    Rollback() error
}

// QueryOptions provides filtering and pagination
type QueryOptions struct {
    // Pagination
    Limit      int
    Offset     int

    // Sorting
    SortBy     SortField
    Order      SortOrder

    // Time filtering
    After      *time.Time
    Before     *time.Time

    // Score filtering
    MinScore   *int
    MaxScore   *int

    // Other filters
    IncludeDeleted bool
    IncludeRemoved bool
}

// SearchOptions for full-text search
type SearchOptions struct {
    QueryOptions

    // Search scope
    SearchIn   []SearchField  // title, selftext, body

    // Result grouping
    GroupBy    string
}

// DatabaseStats provides storage metrics
type DatabaseStats struct {
    PostCount       int64
    CommentCount    int64
    SubredditCount  int64
    AccountCount    int64
    DatabaseSize    int64  // bytes
    OldestPost      *time.Time
    NewestPost      *time.Time
}

// SortField enumeration
type SortField string

const (
    SortByCreated    SortField = "created"
    SortByScore      SortField = "score"
    SortByComments   SortField = "comments"
    SortByAuthor     SortField = "author"
)

// SortOrder enumeration
type SortOrder string

const (
    OrderAsc  SortOrder = "asc"
    OrderDesc SortOrder = "desc"
)

// SearchField enumeration
type SearchField string

const (
    SearchTitle    SearchField = "title"
    SearchSelfText SearchField = "selftext"
    SearchBody     SearchField = "body"
    SearchAuthor   SearchField = "author"
)
```

### SQLite Implementation

```go
// pkg/storage/sqlite/repository.go
package sqlite

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"

    _ "github.com/mattn/go-sqlite3"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

type SQLiteRepository struct {
    db *sql.DB

    // Prepared statements cache
    stmts map[string]*sql.Stmt
}

// NewSQLiteRepository creates a new SQLite storage backend
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
    db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-64000")
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Enable foreign keys
    if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
        return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    }

    // Set connection pool
    db.SetMaxOpenConns(1) // SQLite doesn't handle concurrent writes well
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)

    repo := &SQLiteRepository{
        db:    db,
        stmts: make(map[string]*sql.Stmt),
    }

    // Run migrations
    if err := repo.migrate(); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }

    // Prepare common statements
    if err := repo.prepareStatements(); err != nil {
        return nil, fmt.Errorf("failed to prepare statements: %w", err)
    }

    return repo, nil
}

// SavePost saves or updates a post
func (r *SQLiteRepository) SavePost(ctx context.Context, post *types.Post) error {
    query := `
        INSERT INTO posts (
            id, name, subreddit, subreddit_id, author, title, selftext, url,
            score, ups, downs, num_comments, created_utc, permalink,
            is_self, over18, locked, stickied, archived, hidden, saved,
            distinguished, edited_timestamp, media, author_flair, link_flair
        ) VALUES (
            ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
        )
        ON CONFLICT(id) DO UPDATE SET
            score = excluded.score,
            ups = excluded.ups,
            downs = excluded.downs,
            num_comments = excluded.num_comments,
            edited_timestamp = excluded.edited_timestamp,
            locked = excluded.locked,
            stickied = excluded.stickied,
            archived = excluded.archived,
            updated_at = CURRENT_TIMESTAMP
    `

    var editedTs *float64
    if post.Edited.IsEdited && post.Edited.Timestamp > 0 {
        editedTs = &post.Edited.Timestamp
    }

    mediaJSON, _ := json.Marshal(post.Media)
    authorFlairJSON, _ := json.Marshal(post.AuthorFlairText)
    linkFlairJSON, _ := json.Marshal(post.LinkFlairText)

    _, err := r.db.ExecContext(ctx, query,
        post.ID, post.Name, post.Subreddit, post.SubredditID,
        post.Author, post.Title, post.SelfText, post.URL,
        post.Score, post.Ups, post.Downs, post.NumComments,
        post.CreatedUTC, post.Permalink,
        boolToInt(post.IsSelf), boolToInt(post.Over18),
        boolToInt(post.Locked), boolToInt(post.Stickied),
        boolToInt(post.Archived), boolToInt(post.Hidden), boolToInt(post.Saved),
        post.Distinguished, editedTs,
        string(mediaJSON), string(authorFlairJSON), string(linkFlairJSON),
    )

    return err
}

// SavePosts saves multiple posts in a transaction
func (r *SQLiteRepository) SavePosts(ctx context.Context, posts []*types.Post) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO posts (
            id, name, subreddit, subreddit_id, author, title, selftext, url,
            score, ups, downs, num_comments, created_utc, permalink,
            is_self, over18, locked, stickied
        ) VALUES (
            ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
        )
        ON CONFLICT(id) DO UPDATE SET
            score = excluded.score,
            ups = excluded.ups,
            downs = excluded.downs,
            num_comments = excluded.num_comments,
            updated_at = CURRENT_TIMESTAMP
    `)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, post := range posts {
        _, err := stmt.ExecContext(ctx,
            post.ID, post.Name, post.Subreddit, post.SubredditID,
            post.Author, post.Title, post.SelfText, post.URL,
            post.Score, post.Ups, post.Downs, post.NumComments,
            post.CreatedUTC, post.Permalink,
            boolToInt(post.IsSelf), boolToInt(post.Over18),
            boolToInt(post.Locked), boolToInt(post.Stickied),
        )
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}

// SaveCommentTree saves an entire comment tree efficiently
func (r *SQLiteRepository) SaveCommentTree(ctx context.Context, postID string, comments []*types.Comment) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Recursive function to save comments with depth
    var saveWithDepth func(comment *types.Comment, depth int) error
    saveWithDepth = func(comment *types.Comment, depth int) error {
        // Save the comment
        _, err := tx.ExecContext(ctx, `
            INSERT INTO comments (
                id, name, link_id, parent_id, subreddit, subreddit_id,
                author, body, body_html, score, ups, downs, gilded,
                created_utc, depth, score_hidden, saved, distinguished
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(id) DO UPDATE SET
                score = excluded.score,
                edited_timestamp = excluded.edited_timestamp,
                updated_at = CURRENT_TIMESTAMP
        `,
            comment.ID, comment.Name, comment.LinkID, comment.ParentID,
            comment.Subreddit, comment.SubredditID, comment.Author,
            comment.Body, comment.BodyHTML, comment.Score,
            comment.Ups, comment.Downs, comment.Gilded,
            comment.CreatedUTC, depth,
            boolToInt(comment.ScoreHidden), boolToInt(comment.Saved),
            comment.Distinguished,
        )
        if err != nil {
            return err
        }

        // Save replies recursively
        for _, reply := range comment.Replies {
            if err := saveWithDepth(reply, depth+1); err != nil {
                return err
            }
        }

        // Save "more children" IDs if present
        if len(comment.MoreChildrenIDs) > 0 {
            childrenJSON, _ := json.Marshal(comment.MoreChildrenIDs)
            _, err = tx.ExecContext(ctx, `
                INSERT INTO more_children (link_id, parent_id, children, count, depth)
                VALUES (?, ?, ?, ?, ?)
                ON CONFLICT(link_id, parent_id) DO UPDATE SET
                    children = excluded.children,
                    count = excluded.count
            `,
                postID, comment.ID, string(childrenJSON),
                len(comment.MoreChildrenIDs), depth+1,
            )
            if err != nil {
                return err
            }
        }

        return nil
    }

    // Save all top-level comments
    for _, comment := range comments {
        if err := saveWithDepth(comment, 0); err != nil {
            return err
        }
    }

    return tx.Commit()
}

// GetCommentTree retrieves and reconstructs the comment tree
func (r *SQLiteRepository) GetCommentTree(ctx context.Context, postID string, maxDepth int) ([]*types.Comment, error) {
    query := `
        SELECT id, name, link_id, parent_id, subreddit, subreddit_id,
               author, body, body_html, score, ups, downs, gilded,
               created_utc, depth, score_hidden, saved, distinguished,
               edited_timestamp
        FROM comments
        WHERE link_id = ?
        AND depth <= ?
        ORDER BY parent_id, score DESC, created_utc
    `

    rows, err := r.db.QueryContext(ctx, query, "t3_"+postID, maxDepth)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // Read all comments into a map
    commentMap := make(map[string]*types.Comment)
    var allComments []*types.Comment

    for rows.Next() {
        var c types.Comment
        var depth int
        var editedTs *float64

        err := rows.Scan(
            &c.ID, &c.Name, &c.LinkID, &c.ParentID,
            &c.Subreddit, &c.SubredditID, &c.Author,
            &c.Body, &c.BodyHTML, &c.Score,
            &c.Ups, &c.Downs, &c.Gilded,
            &c.CreatedUTC, &depth,
            &c.ScoreHidden, &c.Saved, &c.Distinguished,
            &editedTs,
        )
        if err != nil {
            return nil, err
        }

        if editedTs != nil && *editedTs > 0 {
            c.Edited = types.Edited{IsEdited: true, Timestamp: *editedTs}
        }

        c.Replies = make([]*types.Comment, 0)
        commentMap[c.ID] = &c
        allComments = append(allComments, &c)
    }

    // Build the tree structure
    var roots []*types.Comment
    for _, comment := range allComments {
        if comment.ParentID == "t3_"+postID {
            // Top-level comment
            roots = append(roots, comment)
        } else {
            // Reply to another comment
            parentID := comment.ParentID[3:] // Strip t1_ prefix
            if parent, exists := commentMap[parentID]; exists {
                parent.Replies = append(parent.Replies, comment)
            }
        }
    }

    return roots, nil
}

// Helper functions
func boolToInt(b bool) int {
    if b {
        return 1
    }
    return 0
}

func intToBool(i int) bool {
    return i != 0
}

// Prepare commonly used statements
func (r *SQLiteRepository) prepareStatements() error {
    queries := map[string]string{
        "getPost": "SELECT * FROM posts WHERE id = ?",
        "getPostByName": "SELECT * FROM posts WHERE name = ?",
        "getComment": "SELECT * FROM comments WHERE id = ?",
        "getSubreddit": "SELECT * FROM subreddits WHERE display_name = ?",
        "getAccount": "SELECT * FROM accounts WHERE name = ?",
    }

    for name, query := range queries {
        stmt, err := r.db.Prepare(query)
        if err != nil {
            return fmt.Errorf("failed to prepare %s: %w", name, err)
        }
        r.stmts[name] = stmt
    }

    return nil
}

// Close closes the database connection
func (r *SQLiteRepository) Close() error {
    for _, stmt := range r.stmts {
        stmt.Close()
    }
    return r.db.Close()
}
```

### PostgreSQL Implementation

```go
// pkg/storage/postgres/repository.go
package postgres

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    _ "github.com/lib/pq"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

type PostgresRepository struct {
    db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL storage backend
func NewPostgresRepository(connStr string) (*PostgresRepository, error) {
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    repo := &PostgresRepository{db: db}

    // Run migrations
    if err := repo.migrate(); err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }

    return repo, nil
}

// SavePost implementation for PostgreSQL
func (r *PostgresRepository) SavePost(ctx context.Context, post *types.Post) error {
    query := `
        INSERT INTO posts (
            id, name, subreddit, subreddit_id, author, title, selftext, url,
            score, ups, downs, num_comments, created_utc, permalink,
            is_self, over18, locked, stickied, archived, hidden, saved,
            distinguished, edited_timestamp, media, author_flair, link_flair
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
            $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26
        )
        ON CONFLICT (id) DO UPDATE SET
            score = EXCLUDED.score,
            ups = EXCLUDED.ups,
            downs = EXCLUDED.downs,
            num_comments = EXCLUDED.num_comments,
            edited_timestamp = EXCLUDED.edited_timestamp,
            locked = EXCLUDED.locked,
            stickied = EXCLUDED.stickied,
            archived = EXCLUDED.archived,
            updated_at = NOW()
    `

    // PostgreSQL handles JSONB natively
    _, err := r.db.ExecContext(ctx, query,
        post.ID, post.Name, post.Subreddit, post.SubredditID,
        post.Author, post.Title, post.SelfText, post.URL,
        post.Score, post.Ups, post.Downs, post.NumComments,
        post.CreatedUTC, post.Permalink,
        post.IsSelf, post.Over18, post.Locked, post.Stickied,
        post.Archived, post.Hidden, post.Saved,
        post.Distinguished, nilIfZero(post.Edited.Timestamp),
        post.Media, post.AuthorFlairText, post.LinkFlairText,
    )

    return err
}

// Additional PostgreSQL-specific features
func (r *PostgresRepository) SearchFullText(ctx context.Context, query string, opts *storage.SearchOptions) (*storage.SearchResults, error) {
    // Use PostgreSQL full-text search
    searchQuery := `
        SELECT id, name, title, selftext, author, score, created_utc
        FROM posts
        WHERE to_tsvector('english', title || ' ' || COALESCE(selftext, ''))
              @@ plainto_tsquery('english', $1)
        ORDER BY ts_rank(to_tsvector('english', title || ' ' || COALESCE(selftext, '')),
                        plainto_tsquery('english', $1)) DESC
        LIMIT $2 OFFSET $3
    `

    // Execute search query...
    return nil, nil
}

func nilIfZero(f float64) *float64 {
    if f == 0 {
        return nil
    }
    return &f
}
```

## Integration Strategy

### Client Modifications

```go
// Modify reddit.go to add storage support

type Config struct {
    // ... existing fields ...

    // Storage configuration (optional)
    Storage *StorageConfig
}

type StorageConfig struct {
    // Enable storage integration
    Enabled bool

    // Repository implementation
    Repository storage.Repository

    // Caching behavior
    CacheMode CacheMode

    // TTL for cached data
    CacheTTL time.Duration

    // Auto-save API responses
    AutoSave bool

    // Save concurrency (for batch operations)
    SaveWorkers int
}

type CacheMode string

const (
    CacheModeNone         CacheMode = "none"          // No caching
    CacheModeReadThrough  CacheMode = "read-through"  // Check cache before API
    CacheModeWriteThrough CacheMode = "write-through" // Save synchronously
    CacheModeWriteBack    CacheMode = "write-back"    // Save asynchronously
)

// Client struct modifications
type Client struct {
    // ... existing fields ...

    storage       storage.Repository
    storageConfig *StorageConfig
    saveQueue     chan interface{} // For async saves
}

// NewClient modifications
func NewClient(cfg *Config) (*Client, error) {
    // ... existing initialization ...

    c := &Client{
        // ... existing fields ...
    }

    // Initialize storage if configured
    if cfg.Storage != nil && cfg.Storage.Enabled {
        c.storage = cfg.Storage.Repository
        c.storageConfig = cfg.Storage

        if cfg.Storage.CacheMode == CacheModeWriteBack {
            c.saveQueue = make(chan interface{}, 1000)
            for i := 0; i < cfg.Storage.SaveWorkers; i++ {
                go c.saveWorker()
            }
        }
    }

    return c, nil
}

// Modified API methods with storage integration
func (c *Client) GetHot(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
    // Check cache if read-through enabled
    if c.shouldReadFromCache() {
        cached, err := c.storage.GetPostsBySubreddit(ctx, req.Subreddit, toQueryOptions(req))
        if err == nil && len(cached) > 0 && !c.isStale(cached) {
            return &types.PostsResponse{Posts: cached}, nil
        }
    }

    // Call original API
    resp, err := c.getHotOriginal(ctx, req)
    if err != nil {
        return nil, err
    }

    // Save to storage if configured
    if c.shouldSaveToStorage() {
        c.savePostsToStorage(ctx, resp.Posts)
    }

    return resp, nil
}

// Storage helper methods
func (c *Client) shouldReadFromCache() bool {
    return c.storageConfig != nil &&
           c.storageConfig.Enabled &&
           c.storageConfig.CacheMode == CacheModeReadThrough
}

func (c *Client) shouldSaveToStorage() bool {
    return c.storageConfig != nil &&
           c.storageConfig.Enabled &&
           c.storageConfig.AutoSave
}

func (c *Client) savePostsToStorage(ctx context.Context, posts []*types.Post) {
    if c.storageConfig.CacheMode == CacheModeWriteBack {
        // Async save
        select {
        case c.saveQueue <- posts:
        default:
            // Queue full, log warning
        }
    } else {
        // Sync save
        _ = c.storage.SavePosts(ctx, posts)
    }
}

func (c *Client) saveWorker() {
    for item := range c.saveQueue {
        ctx := context.Background()
        switch v := item.(type) {
        case []*types.Post:
            _ = c.storage.SavePosts(ctx, v)
        case []*types.Comment:
            _ = c.storage.SaveComments(ctx, v)
        }
    }
}
```

## Migration Management

### Migration Structure

```go
// pkg/storage/migrate.go
package storage

import (
    "database/sql"
    "embed"
    "fmt"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

// RunMigrations executes database migrations
func RunMigrations(db *sql.DB, driver string) error {
    var d database.Driver
    var migrations embed.FS

    switch driver {
    case "sqlite", "sqlite3":
        var err error
        d, err = sqlite3.WithInstance(db, &sqlite3.Config{})
        if err != nil {
            return fmt.Errorf("failed to create sqlite driver: %w", err)
        }
        migrations = sqliteMigrations

    case "postgres", "postgresql":
        var err error
        d, err = postgres.WithInstance(db, &postgres.Config{})
        if err != nil {
            return fmt.Errorf("failed to create postgres driver: %w", err)
        }
        migrations = postgresMigrations

    default:
        return fmt.Errorf("unsupported driver: %s", driver)
    }

    source, err := iofs.New(migrations, fmt.Sprintf("migrations/%s", driver))
    if err != nil {
        return fmt.Errorf("failed to create migration source: %w", err)
    }

    m, err := migrate.NewWithInstance("iofs", source, driver, d)
    if err != nil {
        return fmt.Errorf("failed to create migrator: %w", err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("migration failed: %w", err)
    }

    return nil
}
```

### Migration Files

```sql
-- migrations/sqlite/000001_initial_schema.up.sql
CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    -- ... full schema from above
);

CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    -- ... full schema from above
);

-- Create indexes...

-- migrations/sqlite/000001_initial_schema.down.sql
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS posts;

-- migrations/sqlite/000002_add_fulltext_search.up.sql
CREATE VIRTUAL TABLE posts_fts USING fts5(
    title, selftext, content=posts, content_rowid=rowid
);

-- Triggers to keep FTS index updated
CREATE TRIGGER posts_fts_insert AFTER INSERT ON posts
BEGIN
    INSERT INTO posts_fts(rowid, title, selftext)
    VALUES (new.rowid, new.title, new.selftext);
END;

-- migrations/postgres/000001_initial_schema.up.sql
-- PostgreSQL version with JSONB, proper booleans, etc.

-- migrations/postgres/000002_add_fulltext_search.up.sql
CREATE INDEX idx_posts_fulltext ON posts
USING GIN (to_tsvector('english', title || ' ' || COALESCE(selftext, '')));
```

## Caching Strategies

### Cache Implementation

```go
// pkg/storage/cache.go
package storage

import (
    "context"
    "sync"
    "time"

    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// CacheEntry represents a cached item
type CacheEntry struct {
    Data      interface{}
    FetchedAt time.Time
    TTL       time.Duration
}

// IsStale checks if the cache entry has expired
func (e *CacheEntry) IsStale() bool {
    return time.Since(e.FetchedAt) > e.TTL
}

// MemoryCache provides in-memory caching
type MemoryCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry
    maxSize int
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSize int) *MemoryCache {
    return &MemoryCache{
        entries: make(map[string]*CacheEntry),
        maxSize: maxSize,
    }
}

// Get retrieves a cached item
func (c *MemoryCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    entry, exists := c.entries[key]
    if !exists || entry.IsStale() {
        return nil, false
    }

    return entry.Data, true
}

// Set stores an item in cache
func (c *MemoryCache) Set(key string, data interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Simple LRU eviction if at capacity
    if len(c.entries) >= c.maxSize {
        var oldest string
        var oldestTime time.Time
        for k, v := range c.entries {
            if oldest == "" || v.FetchedAt.Before(oldestTime) {
                oldest = k
                oldestTime = v.FetchedAt
            }
        }
        delete(c.entries, oldest)
    }

    c.entries[key] = &CacheEntry{
        Data:      data,
        FetchedAt: time.Now(),
        TTL:       ttl,
    }
}

// CachingRepository wraps a repository with caching
type CachingRepository struct {
    storage.Repository
    cache *MemoryCache
    ttl   time.Duration
}

// NewCachingRepository creates a repository with caching layer
func NewCachingRepository(repo storage.Repository, cacheSize int, ttl time.Duration) *CachingRepository {
    return &CachingRepository{
        Repository: repo,
        cache:      NewMemoryCache(cacheSize),
        ttl:        ttl,
    }
}

// GetPost checks cache before database
func (r *CachingRepository) GetPost(ctx context.Context, id string) (*types.Post, error) {
    // Check cache
    if cached, ok := r.cache.Get("post:" + id); ok {
        return cached.(*types.Post), nil
    }

    // Load from storage
    post, err := r.Repository.GetPost(ctx, id)
    if err != nil {
        return nil, err
    }

    // Cache the result
    r.cache.Set("post:"+id, post, r.ttl)

    return post, nil
}
```

## Implementation Phases

### Phase 1: Foundation (Week 1-2)
1. Create package structure and interfaces
2. Implement SQLite repository with basic CRUD operations
3. Set up migration system with initial schema
4. Write unit tests for repository interface
5. Create in-memory repository for testing

### Phase 2: PostgreSQL Support (Week 2-3)
1. Implement PostgreSQL repository
2. Add PostgreSQL-specific optimizations (JSONB, full-text search)
3. Create PostgreSQL migrations
4. Write integration tests for both backends
5. Add connection pooling and performance tuning

### Phase 3: Client Integration (Week 3-4)
1. Modify Client struct to support optional storage
2. Add StorageConfig to main Config
3. Implement storage middleware for transparent caching
4. Add hook system for custom storage logic
5. Ensure backward compatibility with tests

### Phase 4: Advanced Features (Week 4-5)
1. Implement caching strategies (read-through, write-through, write-back)
2. Add bulk operations for performance
3. Implement comment tree storage and retrieval
4. Add full-text search capabilities
5. Create maintenance operations (vacuum, prune)

### Phase 5: Polish and Documentation (Week 5-6)
1. Write comprehensive documentation
2. Create example applications
3. Add performance benchmarks
4. Write migration guides for users
5. Create troubleshooting documentation

## Testing Strategy

### Unit Tests

```go
// pkg/storage/repository_test.go
package storage_test

import (
    "context"
    "testing"
    "time"

    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage/memory"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRepositorySaveAndGetPost(t *testing.T) {
    repos := []struct {
        name string
        repo storage.Repository
    }{
        {"memory", memory.NewMemoryRepository()},
        {"sqlite", createTestSQLiteRepo(t)},
        {"postgres", createTestPostgresRepo(t)},
    }

    for _, tc := range repos {
        t.Run(tc.name, func(t *testing.T) {
            ctx := context.Background()

            post := &types.Post{
                ThingData: types.ThingData{
                    ID:   "test123",
                    Name: "t3_test123",
                },
                Title:       "Test Post",
                Author:      "testuser",
                Subreddit:   "golang",
                Score:       42,
                CreatedUTC:  float64(time.Now().Unix()),
            }

            // Save post
            err := tc.repo.SavePost(ctx, post)
            require.NoError(t, err)

            // Retrieve post
            retrieved, err := tc.repo.GetPost(ctx, "test123")
            require.NoError(t, err)
            assert.Equal(t, post.ID, retrieved.ID)
            assert.Equal(t, post.Title, retrieved.Title)
            assert.Equal(t, post.Score, retrieved.Score)
        })
    }
}

func TestRepositoryCommentTree(t *testing.T) {
    repos := []storage.Repository{
        memory.NewMemoryRepository(),
        createTestSQLiteRepo(t),
    }

    for _, repo := range repos {
        ctx := context.Background()

        // Create a comment tree
        root := &types.Comment{
            ThingData: types.ThingData{ID: "c1", Name: "t1_c1"},
            LinkID:    "t3_post123",
            ParentID:  "t3_post123",
            Body:      "Root comment",
            Replies: []*types.Comment{
                {
                    ThingData: types.ThingData{ID: "c2", Name: "t1_c2"},
                    LinkID:    "t3_post123",
                    ParentID:  "t1_c1",
                    Body:      "First reply",
                    Replies: []*types.Comment{
                        {
                            ThingData: types.ThingData{ID: "c3", Name: "t1_c3"},
                            LinkID:    "t3_post123",
                            ParentID:  "t1_c2",
                            Body:      "Nested reply",
                        },
                    },
                },
                {
                    ThingData: types.ThingData{ID: "c4", Name: "t1_c4"},
                    LinkID:    "t3_post123",
                    ParentID:  "t1_c1",
                    Body:      "Second reply",
                },
            },
        }

        // Save the tree
        err := repo.SaveCommentTree(ctx, "post123", []*types.Comment{root})
        require.NoError(t, err)

        // Retrieve the tree
        retrieved, err := repo.GetCommentTree(ctx, "post123", 10)
        require.NoError(t, err)
        require.Len(t, retrieved, 1)

        // Verify structure
        assert.Equal(t, "c1", retrieved[0].ID)
        assert.Len(t, retrieved[0].Replies, 2)
        assert.Equal(t, "c2", retrieved[0].Replies[0].ID)
        assert.Len(t, retrieved[0].Replies[0].Replies, 1)
        assert.Equal(t, "c3", retrieved[0].Replies[0].Replies[0].ID)
    }
}

func BenchmarkSavePostsSQLite(b *testing.B) {
    repo := createTestSQLiteRepo(b)
    ctx := context.Background()

    posts := generateTestPosts(100)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = repo.SavePosts(ctx, posts)
    }
}
```

### Integration Tests

```go
// integration_test.go
// +build integration

package storage_test

import (
    "context"
    "testing"

    "github.com/jamesprial/go-reddit-api-wrapper"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage/sqlite"
)

func TestClientWithStorage(t *testing.T) {
    // Create storage backend
    repo, err := sqlite.NewSQLiteRepository(":memory:")
    require.NoError(t, err)
    defer repo.Close()

    // Create client with storage
    client, err := reddit.NewClient(&reddit.Config{
        ClientID:     getEnv("REDDIT_CLIENT_ID", "test"),
        ClientSecret: getEnv("REDDIT_CLIENT_SECRET", "test"),
        Storage: &reddit.StorageConfig{
            Enabled:    true,
            Repository: repo,
            CacheMode:  reddit.CacheModeWriteThrough,
            AutoSave:   true,
        },
    })
    require.NoError(t, err)

    // Fetch posts (should save to storage)
    ctx := context.Background()
    resp, err := client.GetHot(ctx, &types.PostsRequest{
        Subreddit: "golang",
        Pagination: types.Pagination{Limit: 10},
    })
    require.NoError(t, err)
    require.NotEmpty(t, resp.Posts)

    // Verify posts were saved
    saved, err := repo.GetPostsBySubreddit(ctx, "golang", nil)
    require.NoError(t, err)
    assert.Len(t, saved, len(resp.Posts))
}
```

## Code Examples

### Example 1: Basic Usage with SQLite

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    reddit "github.com/jamesprial/go-reddit-api-wrapper"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage/sqlite"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func main() {
    // Create SQLite storage
    repo, err := sqlite.NewSQLiteRepository("reddit.db")
    if err != nil {
        log.Fatal(err)
    }
    defer repo.Close()

    // Create Reddit client with storage
    client, err := reddit.NewClient(&reddit.Config{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
        Storage: &reddit.StorageConfig{
            Enabled:    true,
            Repository: repo,
            CacheMode:  reddit.CacheModeReadThrough,
            CacheTTL:   15 * time.Minute,
            AutoSave:   true,
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // First call - fetches from Reddit API and saves to DB
    posts1, err := client.GetHot(ctx, &types.PostsRequest{
        Subreddit: "golang",
        Pagination: types.Pagination{Limit: 25},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Fetched %d posts from API\n", len(posts1.Posts))

    // Second call - returns from cache if within TTL
    posts2, err := client.GetHot(ctx, &types.PostsRequest{
        Subreddit: "golang",
        Pagination: types.Pagination{Limit: 25},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Retrieved %d posts (possibly from cache)\n", len(posts2.Posts))

    // Direct database query
    topPosts, err := repo.GetPostsBySubreddit(ctx, "golang", &storage.QueryOptions{
        Limit:  10,
        SortBy: storage.SortByScore,
        Order:  storage.OrderDesc,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("\nTop 10 posts by score:")
    for i, post := range topPosts {
        fmt.Printf("%d. [%d] %s\n", i+1, post.Score, post.Title)
    }
}
```

### Example 2: PostgreSQL with Full-Text Search

```go
package main

import (
    "context"
    "fmt"
    "log"

    reddit "github.com/jamesprial/go-reddit-api-wrapper"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage/postgres"
)

func main() {
    // PostgreSQL connection string
    connStr := "postgres://user:password@localhost/reddit_db?sslmode=disable"

    // Create PostgreSQL storage
    repo, err := postgres.NewPostgresRepository(connStr)
    if err != nil {
        log.Fatal(err)
    }
    defer repo.Close()

    ctx := context.Background()

    // Create client and fetch data
    client, err := reddit.NewClient(&reddit.Config{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
        Storage: &reddit.StorageConfig{
            Enabled:     true,
            Repository:  repo,
            AutoSave:    true,
            SaveWorkers: 4, // Parallel saves
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Fetch posts from multiple subreddits
    subreddits := []string{"golang", "programming", "rust"}
    for _, sub := range subreddits {
        _, err := client.GetHot(ctx, &types.PostsRequest{
            Subreddit: sub,
            Pagination: types.Pagination{Limit: 100},
        })
        if err != nil {
            log.Printf("Error fetching %s: %v", sub, err)
        }
    }

    // Full-text search across all posts
    results, err := repo.Search(ctx, "concurrency goroutines", &storage.SearchOptions{
        QueryOptions: storage.QueryOptions{
            Limit: 20,
        },
        SearchIn: []storage.SearchField{
            storage.SearchTitle,
            storage.SearchSelfText,
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Search Results:")
    for _, post := range results.Posts {
        fmt.Printf("- [%s] %s (Score: %d)\n",
            post.Subreddit, post.Title, post.Score)
    }
}
```

### Example 3: Comment Tree Storage and Retrieval

```go
package main

import (
    "context"
    "fmt"
    "log"

    reddit "github.com/jamesprial/go-reddit-api-wrapper"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/storage/sqlite"
)

func main() {
    repo, err := sqlite.NewSQLiteRepository("comments.db")
    if err != nil {
        log.Fatal(err)
    }
    defer repo.Close()

    client, err := reddit.NewClient(&reddit.Config{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Fetch comments for a post
    comments, err := client.GetComments(ctx, &types.CommentsRequest{
        Subreddit: "golang",
        PostID:    "abc123",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Save the entire comment tree
    err = repo.SaveCommentTree(ctx, "abc123", comments.Comments)
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve comment tree with depth limit
    tree, err := repo.GetCommentTree(ctx, "abc123", 3) // Max depth 3
    if err != nil {
        log.Fatal(err)
    }

    // Display comment tree
    var printTree func(comments []*types.Comment, indent int)
    printTree = func(comments []*types.Comment, indent int) {
        for _, c := range comments {
            fmt.Printf("%s[%d] %s: %s\n",
                strings.Repeat("  ", indent),
                c.Score, c.Author,
                truncate(c.Body, 50))
            printTree(c.Replies, indent+1)
        }
    }

    printTree(tree, 0)
}
```

## Performance Considerations

### Database Optimization

1. **Index Strategy**
   - Primary indexes on ID fields
   - Composite indexes for common queries
   - Full-text indexes for search
   - Partial indexes for filtered queries

2. **Connection Pooling**
   - SQLite: Single writer, multiple readers
   - PostgreSQL: 25 connections default, tunable
   - Connection lifetime management

3. **Batch Operations**
   - Use transactions for bulk inserts
   - Prepared statements for repeated queries
   - Chunk large operations to avoid timeouts

4. **Query Optimization**
   - EXPLAIN ANALYZE for query planning
   - Avoid N+1 queries in comment trees
   - Use CTEs for complex queries

### Caching Strategy

1. **Memory Cache**
   - LRU eviction for bounded memory
   - TTL-based expiration
   - Key-based invalidation

2. **Storage Cache**
   - Read-through for common queries
   - Write-through for consistency
   - Write-back for performance

3. **Cache Invalidation**
   - TTL for time-based expiry
   - Event-based for real-time updates
   - Manual refresh for user control

### Benchmarks

```go
// Benchmark results (example)
// BenchmarkSavePost-8              1000   1,234,567 ns/op
// BenchmarkSavePosts100-8           100  12,345,678 ns/op
// BenchmarkGetCommentTree-8         500   2,345,678 ns/op
// BenchmarkFullTextSearch-8         200   5,678,901 ns/op
```

## Deployment Considerations

### Configuration

```yaml
# config.yaml example
reddit:
  client_id: ${REDDIT_CLIENT_ID}
  client_secret: ${REDDIT_CLIENT_SECRET}

storage:
  enabled: true
  backend: postgres  # or sqlite
  connection: ${DATABASE_URL}
  cache:
    mode: read-through
    ttl: 15m
    memory_size: 1000
  auto_save: true
  save_workers: 4
```

### Environment Variables

```bash
# SQLite
export REDDIT_STORAGE_BACKEND=sqlite
export REDDIT_STORAGE_PATH=/var/lib/reddit/data.db

# PostgreSQL
export REDDIT_STORAGE_BACKEND=postgres
export DATABASE_URL=postgres://user:pass@host:5432/reddit_db

# Cache configuration
export REDDIT_CACHE_MODE=read-through
export REDDIT_CACHE_TTL=15m
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o reddit-app ./cmd/app

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/reddit-app /reddit-app
COPY --from=builder /app/storage/migrations /migrations
CMD ["/reddit-app"]
```

## Troubleshooting Guide

### Common Issues

1. **Migration Failures**
   - Check database permissions
   - Verify migration file syntax
   - Ensure sequential version numbers

2. **Performance Issues**
   - Check index usage with EXPLAIN
   - Monitor connection pool metrics
   - Review cache hit rates

3. **Storage Size**
   - Implement data pruning
   - Use compression for old data
   - Archive to cold storage

4. **Concurrency Problems**
   - SQLite: Use WAL mode
   - PostgreSQL: Check lock contention
   - Review transaction isolation levels

## Conclusion

This specification provides a comprehensive blueprint for implementing SQLite and PostgreSQL storage backends for the Go Reddit API wrapper. The design maintains backward compatibility while adding powerful data persistence and caching capabilities. The repository pattern ensures clean abstraction, making it easy to add additional storage backends in the future.

Key implementation priorities:
1. Start with SQLite for simplicity
2. Use repository pattern for abstraction
3. Maintain backward compatibility
4. Optimize for common use cases
5. Provide comprehensive documentation

The implementation should be completed in phases, with thorough testing at each stage to ensure reliability and performance.