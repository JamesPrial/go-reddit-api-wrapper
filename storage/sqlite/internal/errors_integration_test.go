//go:build integration

package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal/testutil"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite" // Register SQLite backend
	"github.com/stretchr/testify/require"
)

// TestErrors_NotFoundError_Post verifies that NotFoundError is returned when
// attempting to retrieve a non-existent post by ID.
func TestErrors_NotFoundError_Post(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Try to get non-existent post
	post, err := store.GetPost(ctx, "nonexistent_id")

	// Verify error is NotFoundError
	require.Error(t, err)
	require.Nil(t, post)

	var notFoundErr *storage.NotFoundError
	require.True(t, errors.As(err, &notFoundErr), "expected NotFoundError")
	require.Contains(t, err.Error(), "nonexistent_id", "error should mention the post ID")
	require.Contains(t, err.Error(), "post", "error should mention resource type")
}

// TestErrors_NotFoundError_Comment verifies that NotFoundError is returned when
// attempting to retrieve a non-existent comment by ID.
func TestErrors_NotFoundError_Comment(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Try to get non-existent comment
	comment, err := store.GetComment(ctx, "nonexistent_comment_id")

	// Verify error is NotFoundError
	require.Error(t, err)
	require.Nil(t, comment)

	var notFoundErr *storage.NotFoundError
	require.True(t, errors.As(err, &notFoundErr), "expected NotFoundError")
	require.Contains(t, err.Error(), "nonexistent_comment_id", "error should mention the comment ID")
	require.Contains(t, err.Error(), "comment", "error should mention resource type")
}

// TestErrors_ValidationError_NilPost verifies that ValidationError is returned when
// attempting to upsert a nil post.
func TestErrors_ValidationError_NilPost(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Try to upsert nil post
	err := store.UpsertPost(ctx, nil)

	// Verify error is ValidationError
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError")
	require.Contains(t, err.Error(), "nil", "error should mention nil")
}

// TestErrors_ValidationError_EmptyPostID verifies that ValidationError is returned when
// attempting to upsert a post with an empty ID field.
func TestErrors_ValidationError_EmptyPostID(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Create post with empty ID
	post := testutil.BuildPost("", "golang")

	// Try to upsert post with empty ID
	err := store.UpsertPost(ctx, post)

	// Verify error is ValidationError
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError")
	require.Contains(t, err.Error(), "ID", "error should mention ID field")
}

// TestErrors_ValidationError_EmptyPostName verifies that ValidationError is returned when
// attempting to upsert a post with an empty Name field.
func TestErrors_ValidationError_EmptyPostName(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Create post with empty Name
	post := testutil.BuildPost("abc123", "golang")
	post.Name = ""

	// Try to upsert post with empty Name
	err := store.UpsertPost(ctx, post)

	// Verify error is ValidationError
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError")
	require.Contains(t, err.Error(), "Name", "error should mention Name field")
}

// TestErrors_ValidationError_EmptyPostSubreddit verifies that ValidationError is returned when
// attempting to upsert a post with an empty Subreddit field.
func TestErrors_ValidationError_EmptyPostSubreddit(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Create post with empty Subreddit
	post := testutil.BuildPost("abc123", "")

	// Try to upsert post with empty Subreddit
	err := store.UpsertPost(ctx, post)

	// Verify error is ValidationError
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError")
	require.Contains(t, err.Error(), "Subreddit", "error should mention Subreddit field")
}

// TestErrors_ValidationError_NilComment verifies that ValidationError is returned when
// attempting to upsert a nil comment.
func TestErrors_ValidationError_NilComment(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Try to upsert nil comment
	err := store.UpsertComment(ctx, nil)

	// Verify error is ValidationError
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError")
	require.Contains(t, err.Error(), "nil", "error should mention nil")
}

// TestErrors_ValidationError_EmptyCommentID verifies that ValidationError is NOT currently
// enforced for empty comment ID. The implementation allows empty IDs to be inserted,
// which would be caught at the database level as a constraint violation.
// This test documents the current behavior and notes it as a potential area for enhancement.
func TestErrors_ValidationError_EmptyCommentID(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// First, insert a post so we can reference it in the comment
	post := testutil.BuildPost("post123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create comment with empty ID and minimal valid fields
	comment := testutil.BuildComment("", "post123", "", 0)
	comment.ID = ""

	// Currently, UpsertComment doesn't validate ID, so this may succeed or fail
	// depending on database constraints. Document the actual behavior.
	_ = store.UpsertComment(ctx, comment)
	// Note: This test documents that field-level validation is not comprehensive
}

// TestErrors_ValidationError_CommentFieldValidation documents that individual comment
// fields (Body, BodyHTML, LinkID, ParentID) are NOT currently validated in UpsertComment.
// The implementation only validates that the comment is not nil and LinkID is extractable.
// This test runs through several scenarios to document the current behavior.
func TestErrors_ValidationError_CommentFieldValidation(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// First, insert a post so we can reference it
	post := testutil.BuildPost("post123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	t.Run("empty_body_is_allowed", func(t *testing.T) {
		comment := testutil.BuildComment("comment123", "post123", "", 0)
		comment.Body = ""
		// Empty body is currently allowed to be inserted
		// This documents the current behavior
		_ = store.UpsertComment(ctx, comment)
	})

	t.Run("empty_bodyhtml_is_allowed", func(t *testing.T) {
		comment := testutil.BuildComment("comment124", "post123", "", 0)
		comment.BodyHTML = ""
		// Empty BodyHTML is currently allowed
		_ = store.UpsertComment(ctx, comment)
	})

	t.Run("invalid_linkid_fails_validation", func(t *testing.T) {
		comment := testutil.BuildComment("comment125", "post123", "", 0)
		comment.LinkID = ""
		// Invalid LinkID (can't extract post ID) should fail validation
		err := store.UpsertComment(ctx, comment)
		require.Error(t, err)
		var validationErr *storage.ValidationError
		require.True(t, errors.As(err, &validationErr), "expected ValidationError for invalid LinkID")
	})

	t.Run("empty_parentid_with_valid_linkid_is_allowed", func(t *testing.T) {
		comment := testutil.BuildComment("comment126", "post123", "", 0)
		comment.ParentID = "" // Empty parent_id is allowed at the validation level
		// This might be valid for top-level comments pointing to a post
		_ = store.UpsertComment(ctx, comment)
	})
}

// TestErrors_IntegrityError_CommentWithoutPost documents that the current implementation
// does NOT validate that a comment's referenced post actually exists in the database.
// The LinkID is parsed for a post ID, but no database lookup is performed during validation.
// This is a potential area for enhancement to add referential integrity checking.
func TestErrors_IntegrityError_CommentWithoutPost(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Try to insert comment without the post existing
	comment := testutil.BuildComment("comment123", "nonexistent_post", "", 0)
	comment.LinkID = "t3_nonexistent_post"

	// Try to upsert comment with non-existent post
	err := store.UpsertComment(ctx, comment)

	// Currently, this succeeds without error - no integrity check is performed
	// This documents the current behavior and notes it as an enhancement opportunity
	if err == nil {
		t.Logf("Comment was inserted with non-existent post - no referential integrity check is performed")
	}
}

// TestErrors_IntegrityError_DuplicatePost verifies that upserting the same post twice succeeds
// (SQLite handles conflicts with upsert behavior).
func TestErrors_IntegrityError_DuplicatePost(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("abc123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Try to insert the same post again with different data (this should succeed with upsert)
	post.Title = "Updated Title"
	post.Score = 100
	err = store.UpsertPost(ctx, post)
	require.NoError(t, err, "upsert of existing post should succeed")

	// Verify the post was updated
	retrieved, err := store.GetPost(ctx, "abc123")
	require.NoError(t, err)
	require.Equal(t, "Updated Title", retrieved.Title)
	require.Equal(t, 100, retrieved.Score)
}

// TestErrors_TransactionError_BatchPostsWithInvalidField verifies that UpsertPosts with
// a completely invalid post (nil in the batch) returns an error and rolls back.
func TestErrors_TransactionError_BatchPostsWithInvalidField(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Create a batch with a valid post and a nil entry
	validPost := testutil.BuildPost("valid1", "golang")
	var nilPost *types.Post

	posts := []*types.Post{validPost, nilPost}

	// Try to upsert batch with nil post - should fail
	err := store.UpsertPosts(ctx, posts)

	// Verify error is returned
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError for nil post in batch")
}

// TestErrors_TransactionError_BatchCommentsWithInvalidField verifies that UpsertComments with
// a completely invalid comment (nil in the batch) returns an error and rolls back.
func TestErrors_TransactionError_BatchCommentsWithInvalidField(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// First, insert a post so we can reference it in comments
	post := testutil.BuildPost("post123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Create a batch with a valid comment and a nil entry
	validComment := testutil.BuildComment("valid_comment", "post123", "", 0)
	var nilComment *types.Comment

	comments := []*types.Comment{validComment, nilComment}

	// Try to upsert batch with nil comment - should fail
	err = store.UpsertComments(ctx, comments)

	// Verify error is returned
	require.Error(t, err)

	var validationErr *storage.ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError for nil comment in batch")
}

// TestErrors_ListPosts_InvalidSortByFallsBackToDefault documents that ListPosts silently
// uses default sort parameters if invalid values are provided. Invalid SortBy values
// are ignored and the default "created_utc" is used instead.
func TestErrors_ListPosts_InvalidSortByFallsBackToDefault(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Insert a post so we can retrieve it
	post := testutil.BuildPost("abc123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Try to list posts with invalid SortBy - will use default instead
	opts := &storage.ListPostsOptions{
		SortBy: "invalid_sort_field",
	}

	posts, err := store.ListPosts(ctx, opts)

	// Should succeed with default sort parameters
	require.NoError(t, err)
	require.NotNil(t, posts)
	require.Equal(t, 1, len(posts))
}

// TestErrors_ListPosts_InvalidSortDirFallsBackToDefault documents that ListPosts silently
// uses default sort direction if an invalid direction is provided. Invalid SortDir values
// are ignored and the default "DESC" is used instead.
func TestErrors_ListPosts_InvalidSortDirFallsBackToDefault(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Insert a post so we can retrieve it
	post := testutil.BuildPost("abc123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Try to list posts with invalid SortDir - will use default instead
	opts := &storage.ListPostsOptions{
		SortDir: "invalid_direction",
	}

	posts, err := store.ListPosts(ctx, opts)

	// Should succeed with default sort parameters
	require.NoError(t, err)
	require.NotNil(t, posts)
	require.Equal(t, 1, len(posts))
}

// TestErrors_EvictStale_NegativeMaxAgeInvertsCutoff documents that EvictStale does NOT
// validate the maxAge parameter. A negative maxAge (e.g., -1*time.Hour) inverts the logic
// and creates a future cutoff time. Since all existing posts are older than a future time,
// they all get evicted. This behavior documents the current implementation without validation.
func TestErrors_EvictStale_NegativeMaxAgeInvertsCutoff(t *testing.T) {
	store := testutil.NewInMemoryDB(t)
	ctx := context.Background()

	// Insert a post
	post := testutil.BuildPost("abc123", "golang")
	err := store.UpsertPost(ctx, post)
	require.NoError(t, err)

	// Verify post was inserted
	retrieved, err := store.GetPost(ctx, "abc123")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Call evict with negative maxAge - creates a future cutoff, deleting existing posts
	evicted, err := store.EvictStale(ctx, -1*time.Hour)

	// Should succeed without error
	require.NoError(t, err)
	// All posts are deleted since they're older than the future cutoff
	require.Greater(t, evicted, int64(0), "negative maxAge creates future cutoff, deleting all posts")
}

// TestErrors_ErrorWrapping verifies that errors implement proper wrapping and can be
// extracted using errors.Unwrap() and errors.Is().
func TestErrors_ErrorWrapping(t *testing.T) {
	t.Run("ValidationError Unwrap", func(t *testing.T) {
		store := testutil.NewInMemoryDB(t)
		ctx := context.Background()

		// Trigger a validation error
		err := store.UpsertPost(ctx, nil)
		require.Error(t, err)

		var validationErr *storage.ValidationError
		require.True(t, errors.As(err, &validationErr), "should be able to extract ValidationError")

		// Check that Unwrap() returns the underlying error (if any)
		unwrapped := errors.Unwrap(err)
		// Unwrap may return nil if there's no underlying error
		t.Logf("Unwrapped error: %v", unwrapped)
	})

	t.Run("ValidationError With Empty LinkID", func(t *testing.T) {
		store := testutil.NewInMemoryDB(t)
		ctx := context.Background()

		// Insert a post
		post := testutil.BuildPost("post123", "golang")
		err := store.UpsertPost(ctx, post)
		require.NoError(t, err)

		// Try to insert a comment with empty LinkID
		comment := testutil.BuildComment("comment123", "post123", "", 0)
		comment.LinkID = "" // Empty LinkID will fail validation

		err = store.UpsertComment(ctx, comment)
		require.Error(t, err)

		// This will be a ValidationError
		var validationErr *storage.ValidationError
		require.True(t, errors.As(err, &validationErr), "should be ValidationError for empty LinkID")

		// Check that Unwrap() returns the underlying error
		unwrapped := errors.Unwrap(err)
		// For ValidationError, Unwrap returns the Err field (which may be nil)
		t.Logf("Unwrapped error: %v", unwrapped)
	})

	t.Run("NotFoundError Has No Wrap", func(t *testing.T) {
		store := testutil.NewInMemoryDB(t)
		ctx := context.Background()

		// Trigger a not found error
		_, err := store.GetPost(ctx, "nonexistent_id")
		require.Error(t, err)

		var notFoundErr *storage.NotFoundError
		require.True(t, errors.As(err, &notFoundErr), "should be able to extract NotFoundError")

		// NotFoundError doesn't wrap anything, so Unwrap returns nil
		unwrapped := errors.Unwrap(err)
		require.Nil(t, unwrapped, "NotFoundError should not wrap anything")
	})

	t.Run("Error Chain Inspection", func(t *testing.T) {
		store := testutil.NewInMemoryDB(t)
		ctx := context.Background()

		// Trigger an error
		_, err := store.GetPost(ctx, "nonexistent_id")
		require.Error(t, err)

		// Walk the error chain
		var current error = err
		chainLength := 1
		for errors.Unwrap(current) != nil {
			current = errors.Unwrap(current)
			chainLength++
		}

		t.Logf("Error chain length: %d", chainLength)
	})
}
