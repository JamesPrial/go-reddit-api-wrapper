package testutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/stretchr/testify/require"
)

// AssertNoError fails the test if err is not nil.
// This is a test helper for integration tests to verify no errors occurred.
//
// Example:
//
//	err := someFunction()
//	testutil.AssertNoError(t, err)
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// AssertPostEquals performs a deep field-by-field comparison of two posts.
// Fails the test with a detailed message showing which fields differ.
func AssertPostEquals(t *testing.T, expected, actual *types.Post) {
	t.Helper()

	if expected == nil && actual == nil {
		return
	}

	require.NotNil(t, expected, "expected post should not be nil")
	require.NotNil(t, actual, "actual post should not be nil")

	// Compare ThingData
	require.Equal(t, expected.ThingData.ID, actual.ThingData.ID, "post ID mismatch")
	require.Equal(t, expected.ThingData.Name, actual.ThingData.Name, "post Name mismatch")

	// Compare Votable
	require.Equal(t, expected.Score, actual.Score, "post Score mismatch")
	require.Equal(t, expected.Ups, actual.Ups, "post Ups mismatch")
	require.Equal(t, expected.Downs, actual.Downs, "post Downs mismatch")
	require.Equal(t, expected.Likes, actual.Likes, "post Likes mismatch")

	// Compare Created
	require.Equal(t, expected.Created.Created, actual.Created.Created, "post Created mismatch")
	require.Equal(t, expected.Created.CreatedUTC, actual.Created.CreatedUTC, "post CreatedUTC mismatch")

	// Compare important fields
	require.Equal(t, expected.Author, actual.Author, "post Author mismatch")
	require.Equal(t, expected.Subreddit, actual.Subreddit, "post Subreddit mismatch")
	require.Equal(t, expected.Title, actual.Title, "post Title mismatch")
	require.Equal(t, expected.SelfText, actual.SelfText, "post SelfText mismatch")
	require.Equal(t, expected.URL, actual.URL, "post URL mismatch")
	require.Equal(t, expected.Permalink, actual.Permalink, "post Permalink mismatch")
	require.Equal(t, expected.NumComments, actual.NumComments, "post NumComments mismatch")

	// Compare nullable fields
	require.Equal(t, expected.AuthorFlairText, actual.AuthorFlairText, "post AuthorFlairText mismatch")
	require.Equal(t, expected.LinkFlairText, actual.LinkFlairText, "post LinkFlairText mismatch")
	require.Equal(t, expected.SelfTextHTML, actual.SelfTextHTML, "post SelfTextHTML mismatch")

	// Compare edited field
	require.Equal(t, expected.Edited.IsEdited, actual.Edited.IsEdited, "post Edited.IsEdited mismatch")
	require.Equal(t, expected.Edited.Timestamp, actual.Edited.Timestamp, "post Edited.Timestamp mismatch")

	// Compare other fields
	require.Equal(t, expected.IsSelf, actual.IsSelf, "post IsSelf mismatch")
	require.Equal(t, expected.Over18, actual.Over18, "post Over18 mismatch")
	require.Equal(t, expected.Locked, actual.Locked, "post Locked mismatch")
	require.Equal(t, expected.Stickied, actual.Stickied, "post Stickied mismatch")
	require.Equal(t, expected.UpvoteRatio, actual.UpvoteRatio, "post UpvoteRatio mismatch")
}

// AssertCommentEquals performs a deep field-by-field comparison of two comments.
// Fails the test with a detailed message showing which fields differ.
func AssertCommentEquals(t *testing.T, expected, actual *types.Comment) {
	t.Helper()

	if expected == nil && actual == nil {
		return
	}

	require.NotNil(t, expected, "expected comment should not be nil")
	require.NotNil(t, actual, "actual comment should not be nil")

	// Compare ThingData
	require.Equal(t, expected.ThingData.ID, actual.ThingData.ID, "comment ID mismatch")
	require.Equal(t, expected.ThingData.Name, actual.ThingData.Name, "comment Name mismatch")

	// Compare Votable
	require.Equal(t, expected.Score, actual.Score, "comment Score mismatch")
	require.Equal(t, expected.Ups, actual.Ups, "comment Ups mismatch")
	require.Equal(t, expected.Downs, actual.Downs, "comment Downs mismatch")
	require.Equal(t, expected.Likes, actual.Likes, "comment Likes mismatch")

	// Compare Created
	require.Equal(t, expected.Created.Created, actual.Created.Created, "comment Created mismatch")
	require.Equal(t, expected.Created.CreatedUTC, actual.Created.CreatedUTC, "comment CreatedUTC mismatch")

	// Compare important fields
	require.Equal(t, expected.Author, actual.Author, "comment Author mismatch")
	require.Equal(t, expected.Body, actual.Body, "comment Body mismatch")
	require.Equal(t, expected.BodyHTML, actual.BodyHTML, "comment BodyHTML mismatch")
	require.Equal(t, expected.Subreddit, actual.Subreddit, "comment Subreddit mismatch")

	// Compare relationship fields
	require.Equal(t, expected.LinkID, actual.LinkID, "comment LinkID mismatch")
	require.Equal(t, expected.ParentID, actual.ParentID, "comment ParentID mismatch")

	// Compare nullable fields
	require.Equal(t, expected.ApprovedBy, actual.ApprovedBy, "comment ApprovedBy mismatch")
	require.Equal(t, expected.BannedBy, actual.BannedBy, "comment BannedBy mismatch")
	require.Equal(t, expected.AuthorFlairText, actual.AuthorFlairText, "comment AuthorFlairText mismatch")
	require.Equal(t, expected.Distinguished, actual.Distinguished, "comment Distinguished mismatch")

	// Compare edited field
	require.Equal(t, expected.Edited.IsEdited, actual.Edited.IsEdited, "comment Edited.IsEdited mismatch")
	require.Equal(t, expected.Edited.Timestamp, actual.Edited.Timestamp, "comment Edited.Timestamp mismatch")

	// Compare other fields
	require.Equal(t, expected.Gilded, actual.Gilded, "comment Gilded mismatch")
	require.Equal(t, expected.Saved, actual.Saved, "comment Saved mismatch")
	require.Equal(t, expected.ScoreHidden, actual.ScoreHidden, "comment ScoreHidden mismatch")
	require.Equal(t, expected.MoreChildrenIDs, actual.MoreChildrenIDs, "comment MoreChildrenIDs mismatch")
}

// AssertCommentTreeStructure validates that a comment slice has the expected hierarchy structure.
// Checks that parent-child relationships are correctly maintained via ParentID.
// Verifies that the tree has the expected maximum depth.
func AssertCommentTreeStructure(t *testing.T, comments []*types.Comment, expectedDepth int) {
	t.Helper()

	require.NotNil(t, comments, "comments slice should not be nil")

	// Build a map of comment IDs for quick lookup
	commentMap := make(map[string]*types.Comment)
	for _, c := range comments {
		require.NotEmpty(t, c.ID, "comment ID should not be empty")
		commentMap[c.ID] = c
	}

	// Calculate actual depth from parent-child relationships
	maxDepth := 0

	for _, comment := range comments {
		depth := 0
		currentParentID := comment.ParentID

		// Traverse up the parent chain
		for currentParentID != "" {
			// Extract the actual ID from the ParentID by removing prefix
			var actualParentID string
			if strings.HasPrefix(currentParentID, "t3_") {
				// Parent is a post (t3_), we've reached the top
				break
			}
			if strings.HasPrefix(currentParentID, "t1_") {
				// Parent is a comment (t1_), extract the ID
				actualParentID = strings.TrimPrefix(currentParentID, "t1_")
			} else {
				// Unexpected format, stop traversal
				break
			}

			parentComment, exists := commentMap[actualParentID]
			if !exists {
				// Parent not in this batch, stop traversal
				break
			}

			depth++
			currentParentID = parentComment.ParentID
		}

		if depth > maxDepth {
			maxDepth = depth
		}
	}

	require.Equal(t, expectedDepth, maxDepth,
		fmt.Sprintf("expected maximum comment depth %d, got %d", expectedDepth, maxDepth))
}

// AssertErrorType extracts and validates that an error is of a specific type.
// Returns the error as the expected type if successful, fails the test otherwise.
// Uses errors.As for proper error chain inspection.
//
// Example:
//
//	var validationErr *ValidationError
//	AssertErrorType[*ValidationError](t, err) // validates err is *ValidationError
func AssertErrorType[T error](t *testing.T, err error) T {
	t.Helper()

	var target T
	if !errors.As(err, &target) {
		t.Fatalf("expected error of type %T, got %T: %v",
			target, err, err)
	}
	return target
}

// AssertErrorIs validates that an error matches a target error using errors.Is.
// Provides a clear error message if the assertion fails.
//
// Example:
//
//	AssertErrorIs(t, err, io.EOF)
func AssertErrorIs(t *testing.T, err, target error) {
	t.Helper()

	if !errors.Is(err, target) {
		t.Fatalf("expected error %v, got %v", target, err)
	}
}
