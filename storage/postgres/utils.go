package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// GetStats returns statistics about the stored data.
// It queries the database for post counts, comment counts, oldest/newest entry timestamps,
// and total database size.
// This is a stub implementation - returns "not yet implemented" error.
func (s *PostgresStore) GetStats(ctx context.Context) (*storage.CacheStats, error) {
	return nil, fmt.Errorf("PostgreSQL storage not yet implemented: GetStats")
}

// EvictStale removes entries older than the specified maxAge.
// It deletes posts and comments where the creation timestamp is earlier than the calculated cutoff time.
// Returns the number of entries evicted, or an error if the operation fails.
// The maxAge parameter specifies how old an entry must be to be considered stale,
// measured from its creation time (Created/CreatedUTC fields).
// This is a stub implementation - returns "not yet implemented" error.
func (s *PostgresStore) EvictStale(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, fmt.Errorf("PostgreSQL storage not yet implemented: EvictStale")
}
