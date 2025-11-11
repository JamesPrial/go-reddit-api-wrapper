package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// SaveMonitorState inserts a new monitor or updates an existing monitor if it already exists.
// The monitor ID (state.ID) is used as the unique identifier.
// This is a full upsert - all fields are updated.
// Returns an error if the operation fails.
func (s *SQLiteStore) SaveMonitorState(ctx context.Context, state *storage.MonitorState) error {
	if state == nil {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state", Reason: "state cannot be nil"}
	}
	if state.ID == "" {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.ID", Reason: "monitor ID cannot be empty"}
	}
	if len(state.Subreddits) == 0 {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.Subreddits", Reason: "subreddits list cannot be empty"}
	}
	if state.IntervalSeconds < 10 {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.IntervalSeconds", Reason: "interval must be at least 10 seconds"}
	}
	if state.PostLimit < 1 || state.PostLimit > 100 {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.PostLimit", Reason: "post limit must be between 1 and 100"}
	}
	if !isValidMonitorStatus(state.Status) {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.Status", Value: state.Status, Reason: "status must be 'active', 'paused', or 'stopped'"}
	}

	s.logger.Debug("saving monitor state", "monitor_id", state.ID, "status", state.Status)

	// Marshal JSON fields
	subredditsJSON, err := marshalJSON(state.Subreddits, "subreddits")
	if err != nil {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.Subreddits", Reason: fmt.Sprintf("failed to marshal subreddits: %v", err)}
	}

	lastPostIDsJSON, err := marshalJSON(state.LastPostIDs, "last_post_ids")
	if err != nil {
		return &storage.ValidationError{Operation: "SaveMonitorState", Field: "state.LastPostIDs", Reason: fmt.Sprintf("failed to marshal last_post_ids: %v", err)}
	}

	// Convert time.Time to Unix timestamps
	createdAt := toUnixTime(state.CreatedAt)
	startedAt := toUnixTime(state.StartedAt)
	lastFetchTime := toUnixTimeNullable(state.LastFetchTime)
	stoppedAt := toUnixTimeNullable(state.StoppedAt)

	// Convert bool to integer (SQLite doesn't have native boolean type)
	fetchComments := 0
	if state.FetchComments {
		fetchComments = 1
	}

	// Execute upsert
	_, err = s.db.ExecContext(ctx, queryUpsertMonitorState,
		state.ID,
		subredditsJSON,
		state.IntervalSeconds,
		state.PostLimit,
		fetchComments,
		state.Status,
		lastPostIDsJSON,
		state.TotalFetches,
		state.TotalPosts,
		state.TotalComments,
		state.FailedFetches,
		state.ConsecutiveErrors,
		state.LastError,
		createdAt,
		startedAt,
		lastFetchTime,
		stoppedAt,
	)
	if err != nil {
		return &storage.DatabaseError{Operation: "SaveMonitorState", Message: fmt.Sprintf("failed to upsert monitor state %s", state.ID), Err: err}
	}

	s.logger.Debug("successfully saved monitor state", "monitor_id", state.ID)
	return nil
}

// GetMonitorState retrieves a monitor state by its ID.
// Returns the monitor state if found.
// Returns NotFoundError if the monitor doesn't exist.
// Returns an error for other database failures.
func (s *SQLiteStore) GetMonitorState(ctx context.Context, id string) (*storage.MonitorState, error) {
	if id == "" {
		return nil, &storage.ValidationError{Operation: "GetMonitorState", Field: "id", Reason: "monitor ID cannot be empty"}
	}

	s.logger.Debug("getting monitor state", "monitor_id", id)

	row := s.db.QueryRowContext(ctx, queryGetMonitorState, id)

	state, err := scanMonitorState(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &storage.NotFoundError{ResourceType: "monitor_state", ResourceID: id}
		}
		return nil, err
	}

	s.logger.Debug("successfully retrieved monitor state", "monitor_id", id)
	return state, nil
}

// GetActiveMonitors retrieves all monitors with status="active".
// Returns an empty slice if no active monitors exist.
// Returns an error if the operation fails.
func (s *SQLiteStore) GetActiveMonitors(ctx context.Context) ([]*storage.MonitorState, error) {
	s.logger.Debug("getting active monitors")

	rows, err := s.db.QueryContext(ctx, queryGetActiveMonitors)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "GetActiveMonitors", Message: "failed to query active monitors", Err: err}
	}
	defer rows.Close()

	monitors := make([]*storage.MonitorState, 0)
	for rows.Next() {
		state, err := scanMonitorState(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, state)
	}

	if err := rows.Err(); err != nil {
		return nil, &storage.DatabaseError{Operation: "GetActiveMonitors", Message: "error iterating over monitor rows", Err: err}
	}

	s.logger.Debug("successfully retrieved active monitors", "count", len(monitors))
	return monitors, nil
}

// GetPausedMonitors retrieves all monitors with status="paused".
// Returns an empty slice if no paused monitors exist.
// Returns an error if the operation fails.
func (s *SQLiteStore) GetPausedMonitors(ctx context.Context) ([]*storage.MonitorState, error) {
	s.logger.Debug("getting paused monitors")

	rows, err := s.db.QueryContext(ctx, queryGetPausedMonitors)
	if err != nil {
		return nil, &storage.DatabaseError{Operation: "GetPausedMonitors", Message: "failed to query paused monitors", Err: err}
	}
	defer rows.Close()

	monitors := make([]*storage.MonitorState, 0)
	for rows.Next() {
		state, err := scanMonitorState(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, state)
	}

	if err := rows.Err(); err != nil {
		return nil, &storage.DatabaseError{Operation: "GetPausedMonitors", Message: "error iterating over monitor rows", Err: err}
	}

	s.logger.Debug("successfully retrieved paused monitors", "count", len(monitors))
	return monitors, nil
}

// UpdateMonitorStatus updates only the status field of a monitor.
// Returns NotFoundError if the monitor doesn't exist.
// Returns an error if the operation fails.
func (s *SQLiteStore) UpdateMonitorStatus(ctx context.Context, id string, status string) error {
	if id == "" {
		return &storage.ValidationError{Operation: "UpdateMonitorStatus", Field: "id", Reason: "monitor ID cannot be empty"}
	}
	if !isValidMonitorStatus(status) {
		return &storage.ValidationError{Operation: "UpdateMonitorStatus", Field: "status", Value: status, Reason: "status must be 'active', 'paused', or 'stopped'"}
	}

	s.logger.Debug("updating monitor status", "monitor_id", id, "status", status)

	result, err := s.db.ExecContext(ctx, queryUpdateMonitorStatus, status, id)
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateMonitorStatus", Message: fmt.Sprintf("failed to update monitor status for %s", id), Err: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateMonitorStatus", Message: "failed to get rows affected", Err: err}
	}

	if rowsAffected == 0 {
		return &storage.NotFoundError{ResourceType: "monitor_state", ResourceID: id}
	}

	s.logger.Debug("successfully updated monitor status", "monitor_id", id)
	return nil
}

// UpdateMonitorStats updates the statistics fields of a monitor.
// This is a partial update - only statistics and LastFetchTime are modified.
// Returns NotFoundError if the monitor doesn't exist.
// Returns an error if the operation fails.
func (s *SQLiteStore) UpdateMonitorStats(ctx context.Context, id string, stats *storage.MonitorStats) error {
	if id == "" {
		return &storage.ValidationError{Operation: "UpdateMonitorStats", Field: "id", Reason: "monitor ID cannot be empty"}
	}
	if stats == nil {
		return &storage.ValidationError{Operation: "UpdateMonitorStats", Field: "stats", Reason: "stats cannot be nil"}
	}

	s.logger.Debug("updating monitor stats", "monitor_id", id, "total_fetches", stats.TotalFetches)

	lastFetchTime := toUnixTime(stats.LastFetchTime)

	result, err := s.db.ExecContext(ctx, queryUpdateMonitorStats,
		stats.TotalFetches,
		stats.TotalPosts,
		stats.TotalComments,
		stats.FailedFetches,
		stats.ConsecutiveErrors,
		stats.LastError,
		lastFetchTime,
		id,
	)
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateMonitorStats", Message: fmt.Sprintf("failed to update monitor stats for %s", id), Err: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateMonitorStats", Message: "failed to get rows affected", Err: err}
	}

	if rowsAffected == 0 {
		return &storage.NotFoundError{ResourceType: "monitor_state", ResourceID: id}
	}

	s.logger.Debug("successfully updated monitor stats", "monitor_id", id)
	return nil
}

// UpdateLastPostID updates the last fetched post ID for a specific subreddit.
// This tracks the position in the subreddit's feed to prevent duplicate fetches.
// Returns NotFoundError if the monitor doesn't exist.
// Returns an error if the operation fails.
func (s *SQLiteStore) UpdateLastPostID(ctx context.Context, monitorID string, subreddit string, postID string) error {
	if monitorID == "" {
		return &storage.ValidationError{Operation: "UpdateLastPostID", Field: "monitorID", Reason: "monitor ID cannot be empty"}
	}
	if subreddit == "" {
		return &storage.ValidationError{Operation: "UpdateLastPostID", Field: "subreddit", Reason: "subreddit cannot be empty"}
	}
	if postID == "" {
		return &storage.ValidationError{Operation: "UpdateLastPostID", Field: "postID", Reason: "post ID cannot be empty"}
	}

	s.logger.Debug("updating last post ID", "monitor_id", monitorID, "subreddit", subreddit, "post_id", postID)

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return &storage.TransactionError{Operation: "begin", Message: "UpdateLastPostID", Err: err}
	}
	defer tx.Rollback()

	// Retrieve current last_post_ids JSON
	var lastPostIDsJSON string
	err = tx.QueryRowContext(ctx, queryGetMonitorLastPostIDs, monitorID).Scan(&lastPostIDsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return &storage.NotFoundError{ResourceType: "monitor_state", ResourceID: monitorID}
		}
		return &storage.DatabaseError{Operation: "UpdateLastPostID", Message: fmt.Sprintf("failed to get last_post_ids for monitor %s", monitorID), Err: err}
	}

	// Unmarshal current map
	var lastPostIDs map[string]string
	if err := json.Unmarshal([]byte(lastPostIDsJSON), &lastPostIDs); err != nil {
		return &storage.DatabaseError{Operation: "UpdateLastPostID", Message: "failed to unmarshal last_post_ids", Err: err}
	}

	// Update the specific subreddit entry
	if lastPostIDs == nil {
		lastPostIDs = make(map[string]string)
	}
	lastPostIDs[subreddit] = postID

	// Marshal back to JSON
	updatedJSON, err := marshalJSON(lastPostIDs, "last_post_ids")
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateLastPostID", Message: "failed to marshal updated last_post_ids", Err: err}
	}

	// Update the database
	result, err := tx.ExecContext(ctx, queryUpdateMonitorLastPostIDs, updatedJSON, monitorID)
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateLastPostID", Message: fmt.Sprintf("failed to update last_post_ids for monitor %s", monitorID), Err: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &storage.DatabaseError{Operation: "UpdateLastPostID", Message: "failed to get rows affected", Err: err}
	}

	if rowsAffected == 0 {
		return &storage.NotFoundError{ResourceType: "monitor_state", ResourceID: monitorID}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return &storage.TransactionError{Operation: "commit", Message: "UpdateLastPostID", Err: err}
	}

	s.logger.Debug("successfully updated last post ID", "monitor_id", monitorID, "subreddit", subreddit)
	return nil
}

// DeleteMonitorState removes a monitor state by its ID.
// Returns an error if the operation fails.
// Succeeds silently if the monitor doesn't exist (idempotent delete).
func (s *SQLiteStore) DeleteMonitorState(ctx context.Context, id string) error {
	if id == "" {
		return &storage.ValidationError{Operation: "DeleteMonitorState", Field: "id", Reason: "monitor ID cannot be empty"}
	}

	s.logger.Debug("deleting monitor state", "monitor_id", id)

	_, err := s.db.ExecContext(ctx, queryDeleteMonitorState, id)
	if err != nil {
		return &storage.DatabaseError{Operation: "DeleteMonitorState", Message: fmt.Sprintf("failed to delete monitor state %s", id), Err: err}
	}

	s.logger.Debug("successfully deleted monitor state", "monitor_id", id)
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// isValidMonitorStatus checks if a status string is valid.
func isValidMonitorStatus(status string) bool {
	switch status {
	case "active", "paused", "stopped":
		return true
	default:
		return false
	}
}

// marshalJSON marshals a value to JSON and returns it as a string.
// Returns an error if marshaling fails.
func marshalJSON(v interface{}, fieldName string) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal %s: %w", fieldName, err)
	}
	return string(data), nil
}

// toUnixTime converts a time.Time to a Unix timestamp (seconds since epoch).
// Zero time returns 0.
func toUnixTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// toUnixTimeNullable converts a *time.Time to a nullable Unix timestamp.
// Nil pointer returns NULL (sql.NullInt64{Valid: false}).
// Zero time is preserved as 0 (Unix epoch) in the database.
func toUnixTimeNullable(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

// fromUnixTime converts a Unix timestamp to time.Time.
// Zero timestamp returns zero time.
func fromUnixTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}

// fromUnixTimeNullable converts a nullable Unix timestamp to *time.Time.
// Invalid (NULL) returns nil.
// Zero timestamp returns pointer to zero time (Unix epoch).
func fromUnixTimeNullable(ts sql.NullInt64) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := time.Unix(ts.Int64, 0).UTC()
	return &t
}

// scanMonitorState is a helper interface for scanning monitor state from a row.
// It works with both sql.Row and sql.Rows.
type scannable interface {
	Scan(dest ...interface{}) error
}

// scanMonitorState scans a monitor state from a database row.
// This works with both sql.Row and sql.Rows.
func scanMonitorState(row scannable) (*storage.MonitorState, error) {
	var state storage.MonitorState
	var subredditsJSON string
	var lastPostIDsJSON string
	var fetchCommentsInt int
	var createdAt int64
	var startedAt int64
	var lastFetchTime sql.NullInt64
	var stoppedAt sql.NullInt64

	err := row.Scan(
		&state.ID,
		&subredditsJSON,
		&state.IntervalSeconds,
		&state.PostLimit,
		&fetchCommentsInt,
		&state.Status,
		&lastPostIDsJSON,
		&state.TotalFetches,
		&state.TotalPosts,
		&state.TotalComments,
		&state.FailedFetches,
		&state.ConsecutiveErrors,
		&state.LastError,
		&createdAt,
		&startedAt,
		&lastFetchTime,
		&stoppedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, &storage.DatabaseError{Operation: "scanMonitorState", Message: "failed to scan monitor state row", Err: err}
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal([]byte(subredditsJSON), &state.Subreddits); err != nil {
		return nil, &storage.DatabaseError{Operation: "scanMonitorState", Message: "failed to unmarshal subreddits", Err: err}
	}

	if err := json.Unmarshal([]byte(lastPostIDsJSON), &state.LastPostIDs); err != nil {
		return nil, &storage.DatabaseError{Operation: "scanMonitorState", Message: "failed to unmarshal last_post_ids", Err: err}
	}

	// Convert integer to bool
	state.FetchComments = fetchCommentsInt != 0

	// Convert Unix timestamps to time.Time
	state.CreatedAt = fromUnixTime(createdAt)
	state.StartedAt = fromUnixTime(startedAt)
	state.LastFetchTime = fromUnixTimeNullable(lastFetchTime)
	state.StoppedAt = fromUnixTimeNullable(stoppedAt)

	return &state, nil
}
