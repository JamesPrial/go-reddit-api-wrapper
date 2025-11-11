package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite" // Register SQLite backend
	"github.com/stretchr/testify/require"
)

// buildMonitorState creates a test MonitorState with default values.
// Options can be provided to customize specific fields.
func buildMonitorState(id string, opts ...func(*storage.MonitorState)) *storage.MonitorState {
	now := time.Now().UTC()
	state := &storage.MonitorState{
		ID:                id,
		Subreddits:        []string{"golang", "rust"},
		IntervalSeconds:   60,
		PostLimit:         25,
		FetchComments:     true,
		Status:            "active",
		LastPostIDs:       map[string]string{},
		TotalFetches:      0,
		TotalPosts:        0,
		TotalComments:     0,
		FailedFetches:     0,
		ConsecutiveErrors: 0,
		LastError:         "",
		CreatedAt:         now,
		StartedAt:         now,
		LastFetchTime:     nil,
		StoppedAt:         nil,
	}

	for _, opt := range opts {
		opt(state)
	}

	return state
}

// Test option functions for buildMonitorState
func withSubreddits(subreddits []string) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.Subreddits = subreddits
	}
}

func withStatus(status string) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.Status = status
	}
}

func withLastPostIDs(lastPostIDs map[string]string) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.LastPostIDs = lastPostIDs
	}
}

func withStats(fetches, posts, comments, failed, consecutive uint64, lastError string) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.TotalFetches = fetches
		s.TotalPosts = posts
		s.TotalComments = comments
		s.FailedFetches = failed
		s.ConsecutiveErrors = consecutive
		s.LastError = lastError
	}
}

func withLastFetchTime(t time.Time) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.LastFetchTime = &t
	}
}

func withStoppedAt(t time.Time) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.StoppedAt = &t
	}
}

func withID(id string) func(*storage.MonitorState) {
	return func(s *storage.MonitorState) {
		s.ID = id
	}
}

// ============================================================================
// SaveMonitorState Tests
// ============================================================================

// TestSaveMonitorState_NewMonitor verifies that a new monitor can be saved successfully.
func TestSaveMonitorState_NewMonitor(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-001")

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err, "failed to save new monitor state")

	// Verify it was saved by retrieving it
	retrieved, err := store.GetMonitorState(ctx, "monitor-001")
	require.NoError(t, err, "failed to retrieve monitor state")
	require.NotNil(t, retrieved)
	require.Equal(t, "monitor-001", retrieved.ID)
	require.Equal(t, []string{"golang", "rust"}, retrieved.Subreddits)
	require.Equal(t, 60, retrieved.IntervalSeconds)
	require.Equal(t, 25, retrieved.PostLimit)
	require.True(t, retrieved.FetchComments)
	require.Equal(t, "active", retrieved.Status)
}

// TestSaveMonitorState_UpdateExisting verifies upsert behavior for existing monitors.
func TestSaveMonitorState_UpdateExisting(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Save initial monitor state
	state := buildMonitorState("monitor-002")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err, "failed to save initial monitor state")

	// Update the monitor
	state.IntervalSeconds = 120
	state.PostLimit = 50
	state.Status = "paused"
	state.TotalFetches = 100
	state.TotalPosts = 500
	state.TotalComments = 2000
	state.LastError = "some error"

	err = store.SaveMonitorState(ctx, state)
	require.NoError(t, err, "failed to update monitor state")

	// Verify all fields were updated
	retrieved, err := store.GetMonitorState(ctx, "monitor-002")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, 120, retrieved.IntervalSeconds)
	require.Equal(t, 50, retrieved.PostLimit)
	require.Equal(t, "paused", retrieved.Status)
	require.Equal(t, uint64(100), retrieved.TotalFetches)
	require.Equal(t, uint64(500), retrieved.TotalPosts)
	require.Equal(t, uint64(2000), retrieved.TotalComments)
	require.Equal(t, "some error", retrieved.LastError)
}

// TestSaveMonitorState_EmptyOptionalFields verifies saving with nil/empty optional fields.
func TestSaveMonitorState_EmptyOptionalFields(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-003")
	state.LastPostIDs = map[string]string{}
	state.LastError = ""
	state.LastFetchTime = nil
	state.StoppedAt = nil

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err, "failed to save monitor with empty optional fields")

	retrieved, err := store.GetMonitorState(ctx, "monitor-003")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Empty(t, retrieved.LastPostIDs)
	require.Empty(t, retrieved.LastError)
	require.Nil(t, retrieved.LastFetchTime)
	require.Nil(t, retrieved.StoppedAt)
}

// TestSaveMonitorState_WithLastPostIDs verifies JSON serialization of LastPostIDs.
func TestSaveMonitorState_WithLastPostIDs(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	lastPostIDs := map[string]string{
		"golang":      "t3_abc123",
		"rust":        "t3_def456",
		"programming": "t3_ghi789",
	}

	state := buildMonitorState("monitor-004", withLastPostIDs(lastPostIDs))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	retrieved, err := store.GetMonitorState(ctx, "monitor-004")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, lastPostIDs, retrieved.LastPostIDs)
	require.Equal(t, "t3_abc123", retrieved.LastPostIDs["golang"])
	require.Equal(t, "t3_def456", retrieved.LastPostIDs["rust"])
	require.Equal(t, "t3_ghi789", retrieved.LastPostIDs["programming"])
}

// TestSaveMonitorState_WithTimestamps verifies timestamp persistence.
func TestSaveMonitorState_WithTimestamps(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	lastFetch := now.Add(-1 * time.Hour)
	stopped := now.Add(-30 * time.Minute)

	state := buildMonitorState("monitor-005",
		withLastFetchTime(lastFetch),
		withStoppedAt(stopped))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	retrieved, err := store.GetMonitorState(ctx, "monitor-005")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.NotNil(t, retrieved.LastFetchTime)
	require.NotNil(t, retrieved.StoppedAt)

	// Compare Unix timestamps to handle precision differences
	require.Equal(t, lastFetch.Unix(), retrieved.LastFetchTime.Unix())
	require.Equal(t, stopped.Unix(), retrieved.StoppedAt.Unix())
}

// TestSaveMonitorState_NilState verifies that saving a nil state returns an error.
func TestSaveMonitorState_NilState(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.SaveMonitorState(ctx, nil)
	require.Error(t, err, "should error for nil state")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SaveMonitorState", validationErr.Operation)
}

// TestSaveMonitorState_EmptyID verifies that empty ID returns an error.
func TestSaveMonitorState_EmptyID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("")

	err := store.SaveMonitorState(ctx, state)
	require.Error(t, err, "should error for empty ID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SaveMonitorState", validationErr.Operation)
	require.Contains(t, validationErr.Field, "ID")
}

// TestSaveMonitorState_EmptySubreddits verifies that empty subreddits returns an error.
func TestSaveMonitorState_EmptySubreddits(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-006", withSubreddits([]string{}))

	err := store.SaveMonitorState(ctx, state)
	require.Error(t, err, "should error for empty subreddits")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SaveMonitorState", validationErr.Operation)
}

// TestSaveMonitorState_InvalidIntervalSeconds verifies constraint validation.
func TestSaveMonitorState_InvalidIntervalSeconds(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-007")
	state.IntervalSeconds = 5 // Less than minimum of 10

	err := store.SaveMonitorState(ctx, state)
	require.Error(t, err, "should error for interval_seconds < 10")
}

// TestSaveMonitorState_InvalidPostLimit verifies post limit constraints.
func TestSaveMonitorState_InvalidPostLimit(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	tests := []struct {
		name      string
		postLimit int
	}{
		{"below minimum", 0},
		{"above maximum", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := buildMonitorState("monitor-008-" + tt.name)
			state.PostLimit = tt.postLimit

			err := store.SaveMonitorState(ctx, state)
			require.Error(t, err, "should error for invalid post_limit")
		})
	}
}

// TestSaveMonitorState_InvalidStatus verifies status constraint validation.
func TestSaveMonitorState_InvalidStatus(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-009")
	state.Status = "invalid-status"

	err := store.SaveMonitorState(ctx, state)
	require.Error(t, err, "should error for invalid status")
}

// ============================================================================
// GetMonitorState Tests
// ============================================================================

// TestGetMonitorState_Existing verifies retrieving an existing monitor.
func TestGetMonitorState_Existing(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create and save a monitor
	state := buildMonitorState("monitor-010",
		withSubreddits([]string{"golang", "programming"}),
		withStats(50, 250, 1000, 5, 0, ""))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Retrieve it
	retrieved, err := store.GetMonitorState(ctx, "monitor-010")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	require.Equal(t, "monitor-010", retrieved.ID)
	require.Equal(t, []string{"golang", "programming"}, retrieved.Subreddits)
	require.Equal(t, uint64(50), retrieved.TotalFetches)
	require.Equal(t, uint64(250), retrieved.TotalPosts)
	require.Equal(t, uint64(1000), retrieved.TotalComments)
	require.Equal(t, uint64(5), retrieved.FailedFetches)
}

// TestGetMonitorState_NonExistent verifies NotFoundError for non-existent monitor.
func TestGetMonitorState_NonExistent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	retrieved, err := store.GetMonitorState(ctx, "nonexistent")
	require.Error(t, err, "should error for non-existent monitor")

	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr, "should return NotFoundError")
	require.Equal(t, "monitor_state", notFoundErr.ResourceType)
	require.Equal(t, "nonexistent", notFoundErr.ResourceID)
	require.Nil(t, retrieved)
}

// TestGetMonitorState_EmptyID verifies that empty ID returns an error.
func TestGetMonitorState_EmptyID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	_, err := store.GetMonitorState(ctx, "")
	require.Error(t, err, "should error for empty ID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "GetMonitorState", validationErr.Operation)
}

// TestGetMonitorState_JSONDecoding verifies proper JSON decoding of arrays and maps.
func TestGetMonitorState_JSONDecoding(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	subreddits := []string{"golang", "rust", "python", "javascript"}
	lastPostIDs := map[string]string{
		"golang":     "t3_aaa111",
		"rust":       "t3_bbb222",
		"python":     "t3_ccc333",
		"javascript": "t3_ddd444",
	}

	state := buildMonitorState("monitor-011",
		withSubreddits(subreddits),
		withLastPostIDs(lastPostIDs))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	retrieved, err := store.GetMonitorState(ctx, "monitor-011")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify array decoding
	require.Len(t, retrieved.Subreddits, 4)
	require.Equal(t, subreddits, retrieved.Subreddits)

	// Verify map decoding
	require.Len(t, retrieved.LastPostIDs, 4)
	require.Equal(t, lastPostIDs, retrieved.LastPostIDs)
}

// ============================================================================
// GetActiveMonitors Tests
// ============================================================================

// TestGetActiveMonitors_NoMonitors verifies empty slice when no monitors exist.
func TestGetActiveMonitors_NoMonitors(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	monitors, err := store.GetActiveMonitors(ctx)
	require.NoError(t, err)
	require.NotNil(t, monitors, "should return empty slice, not nil")
	require.Len(t, monitors, 0)
}

// TestGetActiveMonitors_OnlyActive verifies only active monitors are returned.
func TestGetActiveMonitors_OnlyActive(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create monitors with different statuses
	monitors := []*storage.MonitorState{
		buildMonitorState("monitor-012", withStatus("active")),
		buildMonitorState("monitor-013", withStatus("paused")),
		buildMonitorState("monitor-014", withStatus("active")),
		buildMonitorState("monitor-015", withStatus("stopped")),
		buildMonitorState("monitor-016", withStatus("active")),
	}

	for _, m := range monitors {
		err := store.SaveMonitorState(ctx, m)
		require.NoError(t, err, "failed to save monitor %s", m.ID)
	}

	// Get active monitors
	active, err := store.GetActiveMonitors(ctx)
	require.NoError(t, err)
	require.Len(t, active, 3, "should return only 3 active monitors")

	// Verify all returned monitors are active
	for _, m := range active {
		require.Equal(t, "active", m.Status)
	}

	// Verify correct IDs
	ids := make([]string, len(active))
	for i, m := range active {
		ids[i] = m.ID
	}
	require.Contains(t, ids, "monitor-012")
	require.Contains(t, ids, "monitor-014")
	require.Contains(t, ids, "monitor-016")
}

// TestGetActiveMonitors_MultipleMonitors verifies multiple active monitors.
func TestGetActiveMonitors_MultipleMonitors(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create 10 active monitors
	for i := 1; i <= 10; i++ {
		state := buildMonitorState("monitor-active-"+string(rune('0'+i)), withStatus("active"))
		err := store.SaveMonitorState(ctx, state)
		require.NoError(t, err)
	}

	active, err := store.GetActiveMonitors(ctx)
	require.NoError(t, err)
	require.Len(t, active, 10)

	for _, m := range active {
		require.Equal(t, "active", m.Status)
	}
}

// ============================================================================
// UpdateMonitorStatus Tests
// ============================================================================

// ============================================================================
// GetPausedMonitors Tests
// ============================================================================

// TestGetPausedMonitors_NoMonitors verifies empty slice when no monitors exist.
func TestGetPausedMonitors_NoMonitors(t *testing.T) {
	store := NewTestDB(t)

	monitors, err := store.GetPausedMonitors(context.Background())
	require.NoError(t, err)
	require.Empty(t, monitors, "should return empty slice when no monitors exist")
}

// TestGetPausedMonitors_OnlyPaused verifies only paused monitors are returned.
func TestGetPausedMonitors_OnlyPaused(t *testing.T) {
	store := NewTestDB(t)

	// Create monitors with different statuses
	activeMonitor := buildMonitorState("monitor-active", withStatus("active"))
	pausedMonitor1 := buildMonitorState("monitor-paused-1", withStatus("paused"))
	pausedMonitor2 := buildMonitorState("monitor-paused-2", withStatus("paused"))
	stoppedMonitor := buildMonitorState("monitor-stopped", withStatus("stopped"))

	require.NoError(t, store.SaveMonitorState(context.Background(), activeMonitor))
	require.NoError(t, store.SaveMonitorState(context.Background(), pausedMonitor1))
	require.NoError(t, store.SaveMonitorState(context.Background(), pausedMonitor2))
	require.NoError(t, store.SaveMonitorState(context.Background(), stoppedMonitor))

	// Get paused monitors
	monitors, err := store.GetPausedMonitors(context.Background())
	require.NoError(t, err)
	require.Len(t, monitors, 2, "should return only paused monitors")

	// Verify all returned monitors have "paused" status
	for _, m := range monitors {
		require.Equal(t, "paused", m.Status)
	}
}

// TestGetPausedMonitors_OrderedByStoppedAt verifies monitors are ordered by stopped_at DESC.
func TestGetPausedMonitors_OrderedByStoppedAt(t *testing.T) {
	store := NewTestDB(t)

	now := time.Now()

	// Create paused monitors with different stopped_at times
	pausedMonitor1 := buildMonitorState(
		"monitor-1",
		withStatus("paused"),
		func(s *storage.MonitorState) {
			stoppedAt := now.Add(-2 * time.Hour) // Oldest
			s.StoppedAt = &stoppedAt
		},
	)
	pausedMonitor2 := buildMonitorState(
		"monitor-2",
		withStatus("paused"),
		func(s *storage.MonitorState) {
			stoppedAt := now.Add(-1 * time.Hour) // Middle
			s.StoppedAt = &stoppedAt
		},
	)
	pausedMonitor3 := buildMonitorState(
		"monitor-3",
		withStatus("paused"),
		func(s *storage.MonitorState) {
			stoppedAt := now // Most recent
			s.StoppedAt = &stoppedAt
		},
	)

	require.NoError(t, store.SaveMonitorState(context.Background(), pausedMonitor1))
	require.NoError(t, store.SaveMonitorState(context.Background(), pausedMonitor2))
	require.NoError(t, store.SaveMonitorState(context.Background(), pausedMonitor3))

	// Get paused monitors (should be ordered by stopped_at DESC)
	monitors, err := store.GetPausedMonitors(context.Background())
	require.NoError(t, err)
	require.Len(t, monitors, 3)

	// Verify ordering: most recent first
	require.Equal(t, "monitor-3", monitors[0].ID, "most recently paused should be first")
	require.Equal(t, "monitor-2", monitors[1].ID, "middle should be second")
	require.Equal(t, "monitor-1", monitors[2].ID, "oldest should be last")
}

// ============================================================================
// UpdateMonitorStatus Tests
// ============================================================================

// TestUpdateMonitorStatus_ValidStatuses verifies updating to each valid status.
func TestUpdateMonitorStatus_ValidStatuses(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create initial monitor
	state := buildMonitorState("monitor-017", withStatus("active"))
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	validStatuses := []string{"active", "paused", "stopped"}

	for _, newStatus := range validStatuses {
		t.Run(newStatus, func(t *testing.T) {
			err := store.UpdateMonitorStatus(ctx, "monitor-017", newStatus)
			require.NoError(t, err, "failed to update status to %s", newStatus)

			// Verify status was updated
			retrieved, err := store.GetMonitorState(ctx, "monitor-017")
			require.NoError(t, err)
			require.Equal(t, newStatus, retrieved.Status)
		})
	}
}

// TestUpdateMonitorStatus_NonExistent verifies behavior for non-existent monitor.
func TestUpdateMonitorStatus_NonExistent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.UpdateMonitorStatus(ctx, "nonexistent", "active")
	require.Error(t, err, "should error for non-existent monitor")

	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr, "should return NotFoundError")
}

// TestUpdateMonitorStatus_OnlyStatusChanges verifies only status field is modified.
func TestUpdateMonitorStatus_OnlyStatusChanges(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create monitor with specific values
	state := buildMonitorState("monitor-018",
		withStatus("active"),
		withStats(100, 500, 2000, 10, 2, "some error"))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Update only status
	err = store.UpdateMonitorStatus(ctx, "monitor-018", "paused")
	require.NoError(t, err)

	// Verify only status changed
	retrieved, err := store.GetMonitorState(ctx, "monitor-018")
	require.NoError(t, err)
	require.Equal(t, "paused", retrieved.Status)
	require.Equal(t, uint64(100), retrieved.TotalFetches)
	require.Equal(t, uint64(500), retrieved.TotalPosts)
	require.Equal(t, uint64(2000), retrieved.TotalComments)
	require.Equal(t, uint64(10), retrieved.FailedFetches)
	require.Equal(t, uint64(2), retrieved.ConsecutiveErrors)
	require.Equal(t, "some error", retrieved.LastError)
}

// TestUpdateMonitorStatus_InvalidStatus verifies validation of status values.
func TestUpdateMonitorStatus_InvalidStatus(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-019")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	err = store.UpdateMonitorStatus(ctx, "monitor-019", "invalid-status")
	require.Error(t, err, "should error for invalid status")
}

// TestUpdateMonitorStatus_EmptyID verifies empty ID validation.
func TestUpdateMonitorStatus_EmptyID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.UpdateMonitorStatus(ctx, "", "active")
	require.Error(t, err, "should error for empty ID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// ============================================================================
// UpdateMonitorStats Tests
// ============================================================================

// TestUpdateMonitorStats_AllStatistics verifies updating all statistics.
func TestUpdateMonitorStats_AllStatistics(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create initial monitor
	state := buildMonitorState("monitor-020")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Update statistics
	now := time.Now().UTC()
	stats := &storage.MonitorStats{
		TotalFetches:      100,
		TotalPosts:        500,
		TotalComments:     2500,
		FailedFetches:     10,
		ConsecutiveErrors: 0,
		LastError:         "",
		LastFetchTime:     now,
	}

	err = store.UpdateMonitorStats(ctx, "monitor-020", stats)
	require.NoError(t, err)

	// Verify all stats were updated
	retrieved, err := store.GetMonitorState(ctx, "monitor-020")
	require.NoError(t, err)
	require.Equal(t, uint64(100), retrieved.TotalFetches)
	require.Equal(t, uint64(500), retrieved.TotalPosts)
	require.Equal(t, uint64(2500), retrieved.TotalComments)
	require.Equal(t, uint64(10), retrieved.FailedFetches)
	require.Equal(t, uint64(0), retrieved.ConsecutiveErrors)
	require.Equal(t, "", retrieved.LastError)
	require.NotNil(t, retrieved.LastFetchTime)
	require.Equal(t, now.Unix(), retrieved.LastFetchTime.Unix())
}

// TestUpdateMonitorStats_IncrementedValues verifies incremental stat updates.
func TestUpdateMonitorStats_IncrementedValues(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create monitor with initial stats
	state := buildMonitorState("monitor-021",
		withStats(50, 200, 1000, 5, 0, ""))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Increment statistics
	now := time.Now().UTC()
	stats := &storage.MonitorStats{
		TotalFetches:      51,
		TotalPosts:        210,
		TotalComments:     1050,
		FailedFetches:     5,
		ConsecutiveErrors: 0,
		LastError:         "",
		LastFetchTime:     now,
	}

	err = store.UpdateMonitorStats(ctx, "monitor-021", stats)
	require.NoError(t, err)

	retrieved, err := store.GetMonitorState(ctx, "monitor-021")
	require.NoError(t, err)
	require.Equal(t, uint64(51), retrieved.TotalFetches)
	require.Equal(t, uint64(210), retrieved.TotalPosts)
	require.Equal(t, uint64(1050), retrieved.TotalComments)
}

// TestUpdateMonitorStats_WithError verifies updating stats with error information.
func TestUpdateMonitorStats_WithError(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-022")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	now := time.Now().UTC()
	stats := &storage.MonitorStats{
		TotalFetches:      10,
		TotalPosts:        50,
		TotalComments:     200,
		FailedFetches:     3,
		ConsecutiveErrors: 3,
		LastError:         "rate limit exceeded",
		LastFetchTime:     now,
	}

	err = store.UpdateMonitorStats(ctx, "monitor-022", stats)
	require.NoError(t, err)

	retrieved, err := store.GetMonitorState(ctx, "monitor-022")
	require.NoError(t, err)
	require.Equal(t, uint64(3), retrieved.FailedFetches)
	require.Equal(t, uint64(3), retrieved.ConsecutiveErrors)
	require.Equal(t, "rate limit exceeded", retrieved.LastError)
}

// TestUpdateMonitorStats_NonExistent verifies error for non-existent monitor.
func TestUpdateMonitorStats_NonExistent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	stats := &storage.MonitorStats{
		TotalFetches:  1,
		LastFetchTime: time.Now().UTC(),
	}

	err := store.UpdateMonitorStats(ctx, "nonexistent", stats)
	require.Error(t, err, "should error for non-existent monitor")

	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr, "should return NotFoundError")
}

// TestUpdateMonitorStats_OtherFieldsUnchanged verifies non-stat fields remain unchanged.
func TestUpdateMonitorStats_OtherFieldsUnchanged(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create monitor with specific configuration
	state := buildMonitorState("monitor-023",
		withSubreddits([]string{"golang", "rust"}))
	state.IntervalSeconds = 60
	state.PostLimit = 25

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Update only statistics
	now := time.Now().UTC()
	stats := &storage.MonitorStats{
		TotalFetches:  100,
		TotalPosts:    500,
		LastFetchTime: now,
	}

	err = store.UpdateMonitorStats(ctx, "monitor-023", stats)
	require.NoError(t, err)

	// Verify configuration fields unchanged
	retrieved, err := store.GetMonitorState(ctx, "monitor-023")
	require.NoError(t, err)
	require.Equal(t, []string{"golang", "rust"}, retrieved.Subreddits)
	require.Equal(t, 60, retrieved.IntervalSeconds)
	require.Equal(t, 25, retrieved.PostLimit)
	require.Equal(t, uint64(100), retrieved.TotalFetches)
	require.Equal(t, uint64(500), retrieved.TotalPosts)
}

// TestUpdateMonitorStats_NilStats verifies nil stats returns an error.
func TestUpdateMonitorStats_NilStats(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-024")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	err = store.UpdateMonitorStats(ctx, "monitor-024", nil)
	require.Error(t, err, "should error for nil stats")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// ============================================================================
// UpdateLastPostID Tests
// ============================================================================

// TestUpdateLastPostID_FirstPostID verifies adding first post ID for a subreddit.
func TestUpdateLastPostID_FirstPostID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create monitor with empty LastPostIDs
	state := buildMonitorState("monitor-025", withLastPostIDs(map[string]string{}))
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Add first post ID
	err = store.UpdateLastPostID(ctx, "monitor-025", "golang", "t3_abc123")
	require.NoError(t, err)

	// Verify it was added
	retrieved, err := store.GetMonitorState(ctx, "monitor-025")
	require.NoError(t, err)
	require.Len(t, retrieved.LastPostIDs, 1)
	require.Equal(t, "t3_abc123", retrieved.LastPostIDs["golang"])
}

// TestUpdateLastPostID_UpdateExisting verifies updating existing post ID.
func TestUpdateLastPostID_UpdateExisting(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create monitor with existing LastPostIDs
	lastPostIDs := map[string]string{
		"golang": "t3_old123",
		"rust":   "t3_old456",
	}
	state := buildMonitorState("monitor-026", withLastPostIDs(lastPostIDs))
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Update existing post ID
	err = store.UpdateLastPostID(ctx, "monitor-026", "golang", "t3_new789")
	require.NoError(t, err)

	// Verify it was updated
	retrieved, err := store.GetMonitorState(ctx, "monitor-026")
	require.NoError(t, err)
	require.Len(t, retrieved.LastPostIDs, 2)
	require.Equal(t, "t3_new789", retrieved.LastPostIDs["golang"])
	require.Equal(t, "t3_old456", retrieved.LastPostIDs["rust"])
}

// TestUpdateLastPostID_MultipleSubreddits verifies multiple subreddits in same monitor.
func TestUpdateLastPostID_MultipleSubreddits(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-027", withLastPostIDs(map[string]string{}))
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Add post IDs for multiple subreddits
	subreddits := map[string]string{
		"golang":      "t3_aaa111",
		"rust":        "t3_bbb222",
		"python":      "t3_ccc333",
		"javascript":  "t3_ddd444",
		"programming": "t3_eee555",
	}

	for sub, postID := range subreddits {
		err := store.UpdateLastPostID(ctx, "monitor-027", sub, postID)
		require.NoError(t, err, "failed to update post ID for %s", sub)
	}

	// Verify all were added
	retrieved, err := store.GetMonitorState(ctx, "monitor-027")
	require.NoError(t, err)
	require.Len(t, retrieved.LastPostIDs, 5)
	require.Equal(t, subreddits, retrieved.LastPostIDs)
}

// TestUpdateLastPostID_NonExistent verifies error for non-existent monitor.
func TestUpdateLastPostID_NonExistent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.UpdateLastPostID(ctx, "nonexistent", "golang", "t3_abc123")
	require.Error(t, err, "should error for non-existent monitor")

	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr, "should return NotFoundError")
}

// TestUpdateLastPostID_EmptyMonitorID verifies validation of monitor ID.
func TestUpdateLastPostID_EmptyMonitorID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.UpdateLastPostID(ctx, "", "golang", "t3_abc123")
	require.Error(t, err, "should error for empty monitor ID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// TestUpdateLastPostID_EmptySubreddit verifies validation of subreddit.
func TestUpdateLastPostID_EmptySubreddit(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-028")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	err = store.UpdateLastPostID(ctx, "monitor-028", "", "t3_abc123")
	require.Error(t, err, "should error for empty subreddit")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// TestUpdateLastPostID_EmptyPostID verifies validation of post ID.
func TestUpdateLastPostID_EmptyPostID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-029")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	err = store.UpdateLastPostID(ctx, "monitor-029", "golang", "")
	require.Error(t, err, "should error for empty post ID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// ============================================================================
// DeleteMonitorState Tests
// ============================================================================

// TestDeleteMonitorState_Existing verifies deleting an existing monitor.
func TestDeleteMonitorState_Existing(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create a monitor
	state := buildMonitorState("monitor-030")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Verify it exists
	retrieved, err := store.GetMonitorState(ctx, "monitor-030")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Delete it
	err = store.DeleteMonitorState(ctx, "monitor-030")
	require.NoError(t, err)

	// Verify it's gone
	_, err = store.GetMonitorState(ctx, "monitor-030")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

// TestDeleteMonitorState_NonExistent verifies idempotent delete behavior.
func TestDeleteMonitorState_NonExistent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Delete non-existent monitor (should succeed silently or return NotFoundError)
	err := store.DeleteMonitorState(ctx, "nonexistent")
	// Based on existing patterns, this should either succeed or return NotFoundError
	if err != nil {
		var notFoundErr *storage.NotFoundError
		require.ErrorAs(t, err, &notFoundErr, "if error returned, should be NotFoundError")
	}
}

// TestDeleteMonitorState_EmptyID verifies empty ID validation.
func TestDeleteMonitorState_EmptyID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.DeleteMonitorState(ctx, "")
	require.Error(t, err, "should error for empty ID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// ============================================================================
// Edge Cases and Error Scenarios
// ============================================================================

// TestMonitorState_ConcurrentUpdates verifies thread-safe concurrent operations.
func TestMonitorState_ConcurrentUpdates(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create initial monitor
	state := buildMonitorState("monitor-concurrent")
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Update from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			now := time.Now().UTC()
			stats := &storage.MonitorStats{
				TotalFetches:  uint64(n),
				TotalPosts:    uint64(n * 10),
				LastFetchTime: now,
			}
			err := store.UpdateMonitorStats(ctx, "monitor-concurrent", stats)
			require.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify monitor still exists and is valid
	retrieved, err := store.GetMonitorState(ctx, "monitor-concurrent")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
}

// TestMonitorState_LargeJSONPayloads verifies handling of large subreddit lists and post ID maps.
func TestMonitorState_LargeJSONPayloads(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create large subreddit list
	subreddits := make([]string, 100)
	for i := 0; i < 100; i++ {
		subreddits[i] = "subreddit" + string(rune('0'+i%10)) + string(rune('0'+i/10))
	}

	// Create large LastPostIDs map
	lastPostIDs := make(map[string]string)
	for i := 0; i < 100; i++ {
		subName := "subreddit" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		lastPostIDs[subName] = "t3_" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + "123"
	}

	state := buildMonitorState("monitor-large",
		withSubreddits(subreddits),
		withLastPostIDs(lastPostIDs))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// Verify retrieval
	retrieved, err := store.GetMonitorState(ctx, "monitor-large")
	require.NoError(t, err)
	require.Len(t, retrieved.Subreddits, 100)
	require.Len(t, retrieved.LastPostIDs, 100)
}

// TestMonitorState_SpecialCharacters verifies handling of special characters in subreddit names.
func TestMonitorState_SpecialCharacters(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Subreddit names with underscores and numbers (valid Reddit format)
	subreddits := []string{"golang_news", "rust_programming", "python3", "node_js"}
	lastPostIDs := map[string]string{
		"golang_news":      "t3_abc123",
		"rust_programming": "t3_def456",
		"python3":          "t3_ghi789",
		"node_js":          "t3_jkl012",
	}

	state := buildMonitorState("monitor-special",
		withSubreddits(subreddits),
		withLastPostIDs(lastPostIDs))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	retrieved, err := store.GetMonitorState(ctx, "monitor-special")
	require.NoError(t, err)
	require.Equal(t, subreddits, retrieved.Subreddits)
	require.Equal(t, lastPostIDs, retrieved.LastPostIDs)
}

// TestMonitorState_TimestampBoundaries verifies handling of nil vs zero time.
func TestMonitorState_TimestampBoundaries(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Test nil timestamps
	t.Run("nil timestamps", func(t *testing.T) {
		state := buildMonitorState("monitor-nil-times")
		state.LastFetchTime = nil
		state.StoppedAt = nil

		err := store.SaveMonitorState(ctx, state)
		require.NoError(t, err)

		retrieved, err := store.GetMonitorState(ctx, "monitor-nil-times")
		require.NoError(t, err)
		require.Nil(t, retrieved.LastFetchTime)
		require.Nil(t, retrieved.StoppedAt)
	})

	// Test zero time (should be treated as valid)
	t.Run("zero timestamps", func(t *testing.T) {
		var zeroTime time.Time
		state := buildMonitorState("monitor-zero-times",
			withLastFetchTime(zeroTime),
			withStoppedAt(zeroTime))

		err := store.SaveMonitorState(ctx, state)
		require.NoError(t, err)

		retrieved, err := store.GetMonitorState(ctx, "monitor-zero-times")
		require.NoError(t, err)
		require.NotNil(t, retrieved.LastFetchTime)
		require.NotNil(t, retrieved.StoppedAt)
		require.True(t, retrieved.LastFetchTime.IsZero())
		require.True(t, retrieved.StoppedAt.IsZero())
	})
}

// TestMonitorState_StatusTransitions verifies valid status transitions.
func TestMonitorState_StatusTransitions(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	state := buildMonitorState("monitor-transitions", withStatus("active"))
	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	transitions := []struct {
		from string
		to   string
	}{
		{"active", "paused"},
		{"paused", "active"},
		{"active", "stopped"},
		{"stopped", "active"},
		{"paused", "stopped"},
		{"stopped", "paused"},
	}

	for _, tr := range transitions {
		t.Run(tr.from+"_to_"+tr.to, func(t *testing.T) {
			// Set initial status
			err := store.UpdateMonitorStatus(ctx, "monitor-transitions", tr.from)
			require.NoError(t, err)

			// Transition to new status
			err = store.UpdateMonitorStatus(ctx, "monitor-transitions", tr.to)
			require.NoError(t, err)

			// Verify
			retrieved, err := store.GetMonitorState(ctx, "monitor-transitions")
			require.NoError(t, err)
			require.Equal(t, tr.to, retrieved.Status)
		})
	}
}

// TestMonitorState_FullIntegration verifies complete monitor lifecycle.
func TestMonitorState_FullIntegration(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// 1. Create new monitor
	state := buildMonitorState("monitor-integration",
		withSubreddits([]string{"golang", "rust"}),
		withStatus("active"))

	err := store.SaveMonitorState(ctx, state)
	require.NoError(t, err)

	// 2. Verify it's in active monitors
	active, err := store.GetActiveMonitors(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, "monitor-integration", active[0].ID)

	// 3. Update post IDs as monitor runs
	err = store.UpdateLastPostID(ctx, "monitor-integration", "golang", "t3_abc123")
	require.NoError(t, err)
	err = store.UpdateLastPostID(ctx, "monitor-integration", "rust", "t3_def456")
	require.NoError(t, err)

	// 4. Update statistics
	now := time.Now().UTC()
	stats := &storage.MonitorStats{
		TotalFetches:  50,
		TotalPosts:    250,
		TotalComments: 1000,
		LastFetchTime: now,
	}
	err = store.UpdateMonitorStats(ctx, "monitor-integration", stats)
	require.NoError(t, err)

	// 5. Pause monitor
	err = store.UpdateMonitorStatus(ctx, "monitor-integration", "paused")
	require.NoError(t, err)

	// 6. Verify no longer in active monitors
	active, err = store.GetActiveMonitors(ctx)
	require.NoError(t, err)
	require.Len(t, active, 0)

	// 7. Resume monitor
	err = store.UpdateMonitorStatus(ctx, "monitor-integration", "active")
	require.NoError(t, err)

	// 8. Stop monitor
	err = store.UpdateMonitorStatus(ctx, "monitor-integration", "stopped")
	require.NoError(t, err)

	// 9. Verify final state
	retrieved, err := store.GetMonitorState(ctx, "monitor-integration")
	require.NoError(t, err)
	require.Equal(t, "stopped", retrieved.Status)
	require.Equal(t, uint64(50), retrieved.TotalFetches)
	require.Equal(t, "t3_abc123", retrieved.LastPostIDs["golang"])
	require.Equal(t, "t3_def456", retrieved.LastPostIDs["rust"])

	// 10. Delete monitor
	err = store.DeleteMonitorState(ctx, "monitor-integration")
	require.NoError(t, err)

	// 11. Verify deleted
	_, err = store.GetMonitorState(ctx, "monitor-integration")
	require.Error(t, err)
	var notFoundErr *storage.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}
