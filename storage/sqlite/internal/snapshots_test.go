package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite" // Register SQLite backend
	"github.com/stretchr/testify/require"
)

// TestSavePostSnapshot verifies that post snapshots can be saved successfully.
func TestSavePostSnapshot(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create a post first (required for foreign key constraint)
	post := testutil.BuildPost("test123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to create post")

	snapshot := &storage.PostSnapshot{
		PostID:      "test123",
		Fullname:    "t3_test123",
		NumComments: 42,
		Score:       100,
	}

	err = store.SavePostSnapshot(ctx, snapshot)
	require.NoError(t, err, "failed to save post snapshot")
}

// TestSavePostSnapshot_NilSnapshot verifies that saving a nil snapshot returns an error.
func TestSavePostSnapshot_NilSnapshot(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.SavePostSnapshot(ctx, nil)
	require.Error(t, err, "should error for nil snapshot")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SavePostSnapshot", validationErr.Operation)
}

// TestSavePostSnapshot_EmptyPostID verifies that saving a snapshot with empty PostID returns an error.
func TestSavePostSnapshot_EmptyPostID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	snapshot := &storage.PostSnapshot{
		PostID:      "",
		Fullname:    "t3_test123",
		NumComments: 42,
		Score:       100,
	}

	err := store.SavePostSnapshot(ctx, snapshot)
	require.Error(t, err, "should error for empty PostID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SavePostSnapshot", validationErr.Operation)
	require.Equal(t, "snapshot.PostID", validationErr.Field)
}

// TestSavePostSnapshot_EmptyFullname verifies that saving a snapshot with empty Fullname returns an error.
func TestSavePostSnapshot_EmptyFullname(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	snapshot := &storage.PostSnapshot{
		PostID:      "test123",
		Fullname:    "",
		NumComments: 42,
		Score:       100,
	}

	err := store.SavePostSnapshot(ctx, snapshot)
	require.Error(t, err, "should error for empty Fullname")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SavePostSnapshot", validationErr.Operation)
	require.Equal(t, "snapshot.Fullname", validationErr.Field)
}

// TestGetLatestSnapshot verifies that the most recent snapshot is retrieved.
func TestGetLatestSnapshot(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	postID := "test123"

	// Create a post first (required for foreign key constraint)
	post := testutil.BuildPost(postID, "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to create post")

	// Save first snapshot
	snapshot1 := &storage.PostSnapshot{
		PostID:      postID,
		Fullname:    "t3_test123",
		NumComments: 10,
		Score:       50,
	}
	err = store.SavePostSnapshot(ctx, snapshot1)
	require.NoError(t, err, "failed to save first snapshot")

	// Wait a bit to ensure different timestamps (at least 1 second for Unix timestamp granularity)
	time.Sleep(1001 * time.Millisecond)

	// Save second snapshot
	snapshot2 := &storage.PostSnapshot{
		PostID:      postID,
		Fullname:    "t3_test123",
		NumComments: 20,
		Score:       75,
	}
	err = store.SavePostSnapshot(ctx, snapshot2)
	require.NoError(t, err, "failed to save second snapshot")

	// Retrieve the latest snapshot
	latest, err := store.GetLatestSnapshot(ctx, postID)
	require.NoError(t, err, "failed to get latest snapshot")
	require.NotNil(t, latest, "latest snapshot should not be nil")
	require.Equal(t, postID, latest.PostID)
	require.Equal(t, "t3_test123", latest.Fullname)
	require.Equal(t, 20, latest.NumComments, "should have latest comment count")
	require.Equal(t, 75, latest.Score, "should have latest score")
}

// TestGetLatestSnapshot_NoSnapshot verifies that GetLatestSnapshot returns nil when no snapshot exists.
func TestGetLatestSnapshot_NoSnapshot(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Try to get a snapshot for a non-existent post
	latest, err := store.GetLatestSnapshot(ctx, "nonexistent")
	require.NoError(t, err, "should not error for non-existent post")
	require.Nil(t, latest, "should return nil for non-existent snapshot")
}

// TestGetLatestSnapshot_EmptyPostID verifies that empty PostID returns an error.
func TestGetLatestSnapshot_EmptyPostID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	_, err := store.GetLatestSnapshot(ctx, "")
	require.Error(t, err, "should error for empty PostID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "GetLatestSnapshot", validationErr.Operation)
}

// TestSaveCommentChangeEvent verifies that comment change events can be saved successfully.
func TestSaveCommentChangeEvent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Create a post first (required for foreign key constraint)
	post := testutil.BuildPost("test123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to create post")

	event := &storage.CommentChangeEvent{
		PostID:        "test123",
		Fullname:      "t3_test123",
		PreviousCount: 10,
		NewCount:      15,
		CommentsAdded: 5,
	}

	err = store.SaveCommentChangeEvent(ctx, event)
	require.NoError(t, err, "failed to save comment change event")
}

// TestSaveCommentChangeEvent_NilEvent verifies that saving a nil event returns an error.
func TestSaveCommentChangeEvent_NilEvent(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	err := store.SaveCommentChangeEvent(ctx, nil)
	require.Error(t, err, "should error for nil event")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SaveCommentChangeEvent", validationErr.Operation)
}

// TestSaveCommentChangeEvent_EmptyPostID verifies that saving an event with empty PostID returns an error.
func TestSaveCommentChangeEvent_EmptyPostID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	event := &storage.CommentChangeEvent{
		PostID:        "",
		Fullname:      "t3_test123",
		PreviousCount: 10,
		NewCount:      15,
		CommentsAdded: 5,
	}

	err := store.SaveCommentChangeEvent(ctx, event)
	require.Error(t, err, "should error for empty PostID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SaveCommentChangeEvent", validationErr.Operation)
	require.Equal(t, "event.PostID", validationErr.Field)
}

// TestSaveCommentChangeEvent_EmptyFullname verifies that saving an event with empty Fullname returns an error.
func TestSaveCommentChangeEvent_EmptyFullname(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	event := &storage.CommentChangeEvent{
		PostID:        "test123",
		Fullname:      "",
		PreviousCount: 10,
		NewCount:      15,
		CommentsAdded: 5,
	}

	err := store.SaveCommentChangeEvent(ctx, event)
	require.Error(t, err, "should error for empty Fullname")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "SaveCommentChangeEvent", validationErr.Operation)
	require.Equal(t, "event.Fullname", validationErr.Field)
}

// TestGetCommentChangeEvents verifies that comment change events can be retrieved.
func TestGetCommentChangeEvents(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	postID := "test123"

	// Create a post first (required for foreign key constraint)
	post := testutil.BuildPost(postID, "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to create post")

	// Save multiple events
	for i := 0; i < 5; i++ {
		event := &storage.CommentChangeEvent{
			PostID:        postID,
			Fullname:      "t3_test123",
			PreviousCount: i * 10,
			NewCount:      (i + 1) * 10,
			CommentsAdded: 10,
		}
		err := store.SaveCommentChangeEvent(ctx, event)
		require.NoError(t, err, "failed to save event %d", i)
		time.Sleep(5 * time.Millisecond) // Ensure different timestamps
	}

	// Retrieve events
	events, err := store.GetCommentChangeEvents(ctx, postID, 10)
	require.NoError(t, err, "failed to get comment change events")
	require.NotNil(t, events, "events should not be nil")
	require.Len(t, events, 5, "should have 5 events")

	// Verify events are in descending order (most recent first)
	for i := 0; i < len(events)-1; i++ {
		require.True(t, events[i].DetectedAt.After(events[i+1].DetectedAt) || events[i].DetectedAt.Equal(events[i+1].DetectedAt),
			"events should be ordered by most recent first")
	}

	// Verify event data
	require.Equal(t, postID, events[0].PostID)
	require.Equal(t, "t3_test123", events[0].Fullname)
}

// TestGetCommentChangeEvents_WithLimit verifies that the limit parameter works correctly.
func TestGetCommentChangeEvents_WithLimit(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	postID := "test123"

	// Create a post first (required for foreign key constraint)
	post := testutil.BuildPost(postID, "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to create post")

	// Save 10 events
	for i := 0; i < 10; i++ {
		event := &storage.CommentChangeEvent{
			PostID:        postID,
			Fullname:      "t3_test123",
			PreviousCount: i * 10,
			NewCount:      (i + 1) * 10,
			CommentsAdded: 10,
		}
		err := store.SaveCommentChangeEvent(ctx, event)
		require.NoError(t, err, "failed to save event")
		time.Sleep(5 * time.Millisecond)
	}

	// Retrieve only 3 events
	events, err := store.GetCommentChangeEvents(ctx, postID, 3)
	require.NoError(t, err, "failed to get comment change events")
	require.Len(t, events, 3, "should return only 3 events")
}

// TestGetCommentChangeEvents_NoEvents verifies that an empty slice is returned when no events exist.
func TestGetCommentChangeEvents_NoEvents(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	events, err := store.GetCommentChangeEvents(ctx, "nonexistent", 10)
	require.NoError(t, err, "should not error for non-existent post")
	require.Equal(t, 0, len(events), "should return empty slice")
}

// TestGetCommentChangeEvents_EmptyPostID verifies that empty PostID returns an error.
func TestGetCommentChangeEvents_EmptyPostID(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	_, err := store.GetCommentChangeEvents(ctx, "", 10)
	require.Error(t, err, "should error for empty PostID")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "GetCommentChangeEvents", validationErr.Operation)
}

// TestGetCommentChangeEvents_InvalidLimit verifies that invalid limit returns an error.
func TestGetCommentChangeEvents_InvalidLimit(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	_, err := store.GetCommentChangeEvents(ctx, "test123", 0)
	require.Error(t, err, "should error for limit < 1")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
	require.Equal(t, "GetCommentChangeEvents", validationErr.Operation)
	require.Equal(t, "limit", validationErr.Field)
}

// TestGetCommentChangeEvents_NegativeLimit verifies that negative limit returns an error.
func TestGetCommentChangeEvents_NegativeLimit(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	_, err := store.GetCommentChangeEvents(ctx, "test123", -1)
	require.Error(t, err, "should error for negative limit")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr, "should return ValidationError")
}

// TestSnapshotAndChangeEventsIntegration verifies the full flow of snapshots and change events.
func TestSnapshotAndChangeEventsIntegration(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	postID := "post123"
	fullname := "t3_post123"

	// Create a post first (required for foreign key constraint)
	post := testutil.BuildPost(postID, "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err, "failed to create post")

	// Save initial snapshot
	snapshot1 := &storage.PostSnapshot{
		PostID:      postID,
		Fullname:    fullname,
		NumComments: 50,
		Score:       100,
	}
	err = store.SavePostSnapshot(ctx, snapshot1)
	require.NoError(t, err)

	// Save a change event
	event1 := &storage.CommentChangeEvent{
		PostID:        postID,
		Fullname:      fullname,
		PreviousCount: 50,
		NewCount:      55,
		CommentsAdded: 5,
	}
	err = store.SaveCommentChangeEvent(ctx, event1)
	require.NoError(t, err)

	time.Sleep(1001 * time.Millisecond)

	// Save second snapshot
	snapshot2 := &storage.PostSnapshot{
		PostID:      postID,
		Fullname:    fullname,
		NumComments: 55,
		Score:       105,
	}
	err = store.SavePostSnapshot(ctx, snapshot2)
	require.NoError(t, err)

	// Save another change event
	event2 := &storage.CommentChangeEvent{
		PostID:        postID,
		Fullname:      fullname,
		PreviousCount: 55,
		NewCount:      60,
		CommentsAdded: 5,
	}
	err = store.SaveCommentChangeEvent(ctx, event2)
	require.NoError(t, err)

	// Verify we can retrieve the latest snapshot
	latest, err := store.GetLatestSnapshot(ctx, postID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, 55, latest.NumComments)
	require.Equal(t, 105, latest.Score)

	// Verify we can retrieve all change events
	events, err := store.GetCommentChangeEvents(ctx, postID, 10)
	require.NoError(t, err)
	require.Len(t, events, 2, "should have 2 change events")

	// Verify events are in descending order
	require.Equal(t, 60, events[0].NewCount, "most recent event should be first")
	require.Equal(t, 55, events[1].NewCount, "older event should be second")
}

// TestMultiplePostSnapshots verifies that snapshots for different posts are stored independently.
func TestMultiplePostSnapshots(t *testing.T) {
	store := NewTestDB(t)
	ctx := context.Background()

	// Save snapshots for multiple posts
	for i := 1; i <= 3; i++ {
		postID := "post" + string(rune('0'+i))

		// Create a post first (required for foreign key constraint)
		post := testutil.BuildPost(postID, "golang")
		err := store.UpsertPost(ctx, post)
		require.NoError(t, err, "failed to create post %s", postID)

		snapshot := &storage.PostSnapshot{
			PostID:      postID,
			Fullname:    "t3_" + postID,
			NumComments: i * 10,
			Score:       i * 100,
		}
		err = store.SavePostSnapshot(ctx, snapshot)
		require.NoError(t, err, "failed to save snapshot for post %s", postID)
	}

	// Verify each post has the correct latest snapshot
	for i := 1; i <= 3; i++ {
		postID := "post" + string(rune('0'+i))
		latest, err := store.GetLatestSnapshot(ctx, postID)
		require.NoError(t, err)
		require.NotNil(t, latest)
		require.Equal(t, postID, latest.PostID)
		require.Equal(t, i*10, latest.NumComments)
		require.Equal(t, i*100, latest.Score)
	}
}
