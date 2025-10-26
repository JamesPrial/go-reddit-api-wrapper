package db

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Repository defines the database operations for the Reddit tracker application.
// It provides CRUD operations for subreddits, posts, and comments with context support
// and transaction handling.
type Repository interface {
	// Subreddit operations

	// CreateSubreddit creates a new subreddit record in the database.
	// Returns an error if a subreddit with the same fullname already exists.
	CreateSubreddit(ctx context.Context, sub *Subreddit) error

	// GetSubreddit retrieves a subreddit by name (e.g., "golang").
	// Returns gorm.ErrRecordNotFound if the subreddit does not exist.
	GetSubreddit(ctx context.Context, name string) (*Subreddit, error)

	// ListSubreddits retrieves all subreddits from the database.
	// Returns an empty slice if no subreddits exist.
	ListSubreddits(ctx context.Context) ([]Subreddit, error)

	// UpdateSubreddit updates an existing subreddit record.
	// Only updates non-zero fields.
	UpdateSubreddit(ctx context.Context, sub *Subreddit) error

	// DeleteSubreddit deletes a subreddit by name.
	// Cascades to delete all associated posts and comments due to foreign key constraints.
	DeleteSubreddit(ctx context.Context, name string) error

	// Post operations

	// CreatePost creates a new post record in the database.
	// Returns an error if a post with the same fullname already exists.
	CreatePost(ctx context.Context, post *Post) error

	// GetPost retrieves a post by fullname (e.g., "t3_abc123").
	// Returns gorm.ErrRecordNotFound if the post does not exist.
	GetPost(ctx context.Context, fullname string) (*Post, error)

	// ListPosts retrieves posts for a specific subreddit with pagination support.
	// Results are ordered by CreatedUTC descending (newest first).
	// Use limit=0 to get all posts (not recommended for large datasets).
	ListPosts(ctx context.Context, subredditName string, limit, offset int) ([]Post, error)

	// UpdatePost updates an existing post record.
	// Only updates non-zero fields.
	UpdatePost(ctx context.Context, post *Post) error

	// Comment operations

	// CreateComments creates multiple comment records in a single transaction.
	// This is more efficient than creating comments one by one.
	// Returns an error if any comment with the same fullname already exists.
	CreateComments(ctx context.Context, comments []Comment) error

	// GetCommentsByPost retrieves all comments for a specific post.
	// Results include the full comment tree with nested replies preloaded.
	// Comments are ordered by CreatedUTC ascending (oldest first).
	GetCommentsByPost(ctx context.Context, postFullname string) ([]Comment, error)

	// Transaction support

	// BeginTx starts a new database transaction and returns a Repository
	// that operates within this transaction context.
	// All operations on the returned Repository will be part of the transaction
	// until Commit or Rollback is called.
	BeginTx(ctx context.Context) (Repository, error)

	// Commit commits the current transaction.
	// Should only be called on a Repository returned by BeginTx.
	Commit() error

	// Rollback rolls back the current transaction.
	// Should only be called on a Repository returned by BeginTx.
	Rollback() error
}

// gormRepository implements the Repository interface using GORM.
type gormRepository struct {
	db *gorm.DB
	tx *gorm.DB // Set when in transaction mode
}

// NewRepository creates a new Repository instance backed by GORM.
// The provided *gorm.DB should be properly initialized with migrations applied.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

// getDB returns the appropriate database connection (transaction or normal).
func (r *gormRepository) getDB() *gorm.DB {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// Subreddit operations

// CreateSubreddit creates a new subreddit record in the database.
func (r *gormRepository) CreateSubreddit(ctx context.Context, sub *Subreddit) error {
	result := r.getDB().WithContext(ctx).Create(sub)
	if result.Error != nil {
		return fmt.Errorf("failed to create subreddit: %w", result.Error)
	}
	return nil
}

// GetSubreddit retrieves a subreddit by name.
func (r *gormRepository) GetSubreddit(ctx context.Context, name string) (*Subreddit, error) {
	var sub Subreddit
	result := r.getDB().WithContext(ctx).Where("name = ?", name).First(&sub)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get subreddit '%s': %w", name, result.Error)
	}
	return &sub, nil
}

// ListSubreddits retrieves all subreddits from the database.
func (r *gormRepository) ListSubreddits(ctx context.Context) ([]Subreddit, error) {
	var subs []Subreddit
	result := r.getDB().WithContext(ctx).Order("name ASC").Find(&subs)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list subreddits: %w", result.Error)
	}
	return subs, nil
}

// UpdateSubreddit updates an existing subreddit record.
func (r *gormRepository) UpdateSubreddit(ctx context.Context, sub *Subreddit) error {
	result := r.getDB().WithContext(ctx).Save(sub)
	if result.Error != nil {
		return fmt.Errorf("failed to update subreddit: %w", result.Error)
	}
	return nil
}

// DeleteSubreddit deletes a subreddit by name.
func (r *gormRepository) DeleteSubreddit(ctx context.Context, name string) error {
	result := r.getDB().WithContext(ctx).Where("name = ?", name).Delete(&Subreddit{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete subreddit '%s': %w", name, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("subreddit '%s' not found", name)
	}
	return nil
}

// Post operations

// CreatePost creates a new post record in the database.
func (r *gormRepository) CreatePost(ctx context.Context, post *Post) error {
	result := r.getDB().WithContext(ctx).Create(post)
	if result.Error != nil {
		return fmt.Errorf("failed to create post: %w", result.Error)
	}
	return nil
}

// GetPost retrieves a post by fullname.
func (r *gormRepository) GetPost(ctx context.Context, fullname string) (*Post, error) {
	var post Post
	result := r.getDB().WithContext(ctx).
		Preload("Subreddit").
		Where("fullname = ?", fullname).
		First(&post)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get post '%s': %w", fullname, result.Error)
	}
	return &post, nil
}

// ListPosts retrieves posts for a specific subreddit with pagination support.
func (r *gormRepository) ListPosts(ctx context.Context, subredditName string, limit, offset int) ([]Post, error) {
	var posts []Post

	query := r.getDB().WithContext(ctx).
		Joins("JOIN subreddits ON subreddits.id = posts.subreddit_id").
		Where("subreddits.name = ?", subredditName).
		Order("posts.created_utc DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	result := query.Find(&posts)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list posts for subreddit '%s': %w", subredditName, result.Error)
	}

	return posts, nil
}

// UpdatePost updates an existing post record.
func (r *gormRepository) UpdatePost(ctx context.Context, post *Post) error {
	result := r.getDB().WithContext(ctx).Save(post)
	if result.Error != nil {
		return fmt.Errorf("failed to update post: %w", result.Error)
	}
	return nil
}

// Comment operations

// CreateComments creates multiple comment records in a single transaction.
func (r *gormRepository) CreateComments(ctx context.Context, comments []Comment) error {
	if len(comments) == 0 {
		return nil
	}

	// Use CreateInBatches for large comment sets to avoid overwhelming the database
	const batchSize = 100
	result := r.getDB().WithContext(ctx).CreateInBatches(comments, batchSize)
	if result.Error != nil {
		return fmt.Errorf("failed to create comments: %w", result.Error)
	}

	return nil
}

// GetCommentsByPost retrieves all comments for a specific post.
func (r *gormRepository) GetCommentsByPost(ctx context.Context, postFullname string) ([]Comment, error) {
	var comments []Comment

	// First, get the post ID
	var post Post
	result := r.getDB().WithContext(ctx).
		Where("fullname = ?", postFullname).
		First(&post)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get post '%s': %w", postFullname, result.Error)
	}

	// Get all comments for this post, ordered by creation time
	result = r.getDB().WithContext(ctx).
		Where("post_id = ?", post.ID).
		Order("created_utc ASC").
		Find(&comments)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get comments for post '%s': %w", postFullname, result.Error)
	}

	return comments, nil
}

// Transaction support

// BeginTx starts a new database transaction.
func (r *gormRepository) BeginTx(ctx context.Context) (Repository, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	return &gormRepository{
		db: r.db,
		tx: tx,
	}, nil
}

// Commit commits the current transaction.
func (r *gormRepository) Commit() error {
	if r.tx == nil {
		return fmt.Errorf("no active transaction to commit")
	}
	if err := r.tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Rollback rolls back the current transaction.
func (r *gormRepository) Rollback() error {
	if r.tx == nil {
		return fmt.Errorf("no active transaction to rollback")
	}
	if err := r.tx.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	return nil
}
