package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// GetStats returns statistics about the stored data.
// It queries the database for post counts, comment counts, oldest/newest entry timestamps,
// and total database size. Returns an error if any query fails.
// For an empty database, count fields are 0 and timestamp fields are zero time.Time values.
func (s *SQLiteStore) GetStats(ctx context.Context) (*storage.CacheStats, error) {
	stats := &storage.CacheStats{}

	// Query post count
	var postCount sql.NullInt64
	err := s.db.QueryRowContext(ctx, queryGetPostCount).Scan(&postCount)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "GetStats", Message: "failed to query post count", Err: err}
	}
	if postCount.Valid {
		stats.PostCount = postCount.Int64
	}

	// Query comment count
	var commentCount sql.NullInt64
	err = s.db.QueryRowContext(ctx, queryGetCommentCount).Scan(&commentCount)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "GetStats", Message: "failed to query comment count", Err: err}
	}
	if commentCount.Valid {
		stats.CommentCount = commentCount.Int64
	}

	// Query oldest entry timestamp
	// Use UNION ALL to combine posts and comments, then find minimum timestamp
	var oldestTS sql.NullInt64
	err = s.db.QueryRowContext(ctx, queryGetOldestEntry).Scan(&oldestTS)
	if err != nil {
		return nil, &storage.DatabaseError{
			Operation: "GetStats",
			Message:   "failed to query oldest entry",
			Err:       err,
		}
	}
	if oldestTS.Valid && oldestTS.Int64 > 0 {
		stats.OldestEntry = time.Unix(oldestTS.Int64, 0)
	}

	// Query newest entry timestamp
	var newestTS sql.NullInt64
	err = s.db.QueryRowContext(ctx, queryGetNewestEntry).Scan(&newestTS)
	if err != nil {
		return nil, &storage.DatabaseError{
			Operation: "GetStats",
			Message:   "failed to query newest entry",
			Err:       err,
		}
	}
	if newestTS.Valid && newestTS.Int64 > 0 {
		stats.NewestEntry = time.Unix(newestTS.Int64, 0)
	}

	// Query database size
	// SQLite stores this as page_count * page_size
	var sizeBytes sql.NullInt64
	err = s.db.QueryRowContext(ctx, queryGetDatabaseSize).Scan(&sizeBytes)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "GetStats", Message: "failed to get database size", Err: err}
	}
	if sizeBytes.Valid {
		stats.TotalSizeBytes = sizeBytes.Int64
	}

	s.logger.Debug("retrieved cache statistics",
		"post_count", stats.PostCount,
		"comment_count", stats.CommentCount,
		"oldest_entry", stats.OldestEntry,
		"newest_entry", stats.NewestEntry,
		"total_size_bytes", stats.TotalSizeBytes,
	)

	return stats, nil
}

// EvictStale removes entries older than the specified maxAge.
// It deletes posts and comments where fetched_at is earlier than the calculated cutoff time.
// The operation is performed within a transaction for atomicity.
// Returns the total number of entries evicted (posts + comments), or an error if the operation fails.
// Comments are automatically cleaned up from the closure table via CASCADE constraints.
// A maxAge of 0 or negative will delete all entries.
func (s *SQLiteStore) EvictStale(ctx context.Context, maxAge time.Duration) (int64, error) {
	// Calculate cutoff timestamp
	cutoff := time.Now().Add(-maxAge)
	cutoffUnix := cutoff.Unix()

	// Begin transaction for atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, &storage.TransactionError{Operation: "begin", Message: "EvictStale", Err: err}
	}
	defer tx.Rollback() // Safe to call even after commit

	// Delete stale posts
	// Use <= to handle edge case where maxAge=0 (delete everything)
	resultPosts, err := tx.ExecContext(ctx, queryDeleteStalePosts, cutoffUnix)
	if err != nil {
		return 0, &storage.DatabaseError{Operation: "EvictStale", Message: "failed to delete stale posts", Err: err}
	}
	postsDeleted, err := resultPosts.RowsAffected()
	if err != nil {
		return 0, &storage.DatabaseError{
			Operation: "EvictStale",
			Message:   "failed to get posts rows affected",
			Err:       err,
		}
	}

	// Delete stale comments
	// Closure table entries will be automatically deleted via CASCADE
	// Use <= to handle edge case where maxAge=0 (delete everything)
	resultComments, err := tx.ExecContext(ctx, queryDeleteStaleComments, cutoffUnix)
	if err != nil {
		return 0, &storage.DatabaseError{Operation: "EvictStale", Message: "failed to delete stale comments", Err: err}
	}
	commentsDeleted, err := resultComments.RowsAffected()
	if err != nil {
		return 0, &storage.DatabaseError{
			Operation: "EvictStale",
			Message:   "failed to get comments rows affected",
			Err:       err,
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, &storage.TransactionError{Operation: "commit", Message: "EvictStale", Err: err}
	}

	totalDeleted := postsDeleted + commentsDeleted

	s.logger.Info("evicted stale entries",
		"cutoff", cutoff,
		"deleted_posts", postsDeleted,
		"deleted_comments", commentsDeleted,
		"total", totalDeleted,
	)

	return totalDeleted, nil
}
