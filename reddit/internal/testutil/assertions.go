// Package testutil provides reusable test helpers and assertions for the Reddit API wrapper.
// It includes assertion functions for testing common operations and type-safe helpers
// that reduce boilerplate in test code while providing clear error messages.
//
// Example usage:
//
//	func TestMyFeature(t *testing.T) {
//	    client := testutil.NewTestClient(mockServer)
//	    post := testutil.DefaultPost()
//
//	    response, err := client.GetHot(ctx, nil)
//	    testutil.AssertNoError(t, err)
//	    testutil.AssertPostCount(t, response, 5)
//	}
package testutil

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// AssertNoError fails the test if err is not nil.
// This is used for operations that should succeed without error.
//
// Example:
//
//	err := client.Connect()
//	testutil.AssertNoError(t, err)
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil.
// This is used for operations that should fail with an error.
//
// Example:
//
//	err := client.GetHot(ctx, invalidRequest)
//	testutil.AssertError(t, err)
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// AssertErrorType checks that err can be unwrapped to the expected type using errors.As.
// The expectedType parameter should be a pointer to the error type you expect.
//
// Example:
//
//	err := client.Authenticate()
//	testutil.AssertErrorType(t, err, &pkgerrs.AuthError{})
func AssertErrorType(t *testing.T, err error, expectedType interface{}) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error of type %T, got nil", expectedType)
	}
	if !errors.As(err, expectedType) {
		t.Fatalf("expected error of type %T, got %T: %v", expectedType, err, err)
	}
}

// AssertAPIError checks that err is an APIError with the specified status code.
// This is a convenience wrapper around AssertErrorType for the common case
// of checking API error responses.
//
// Example:
//
//	err := client.GetSubreddit(ctx, "nonexistent")
//	testutil.AssertAPIError(t, err, 404)
func AssertAPIError(t *testing.T, err error, expectedStatus int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected APIError with status %d, got nil", expectedStatus)
	}

	// Try to extract status code from error using reflection
	// This works with any error type that has a StatusCode field
	v := reflect.ValueOf(err)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		t.Fatalf("expected APIError with status %d, got %T (not a struct): %v", expectedStatus, err, err)
	}

	statusCodeField := v.FieldByName("StatusCode")
	if !statusCodeField.IsValid() {
		t.Fatalf("expected APIError with status %d, got %T (does not have StatusCode field): %v", expectedStatus, err, err)
	}

	if statusCodeField.Kind() != reflect.Int {
		t.Fatalf("expected APIError with status %d, got %T (StatusCode is not an int): %v", expectedStatus, err, err)
	}

	statusCode := int(statusCodeField.Int())

	if statusCode != expectedStatus {
		t.Fatalf("expected APIError with status %d, got status %d: %v", expectedStatus, statusCode, err)
	}
}

// AssertPostEqual compares key fields of two Post objects for equality.
// It checks ID, Title, Author, Subreddit, Score, and NumComments.
// This is useful for verifying that posts are correctly parsed and processed.
//
// Example:
//
//	expected := testutil.DefaultPost()
//	actual, err := client.GetPost(ctx, "abc123")
//	testutil.AssertNoError(t, err)
//	testutil.AssertPostEqual(t, expected, actual)
func AssertPostEqual(t *testing.T, expected, actual *types.Post) {
	t.Helper()
	if expected == nil && actual == nil {
		return
	}
	if expected == nil {
		t.Fatal("expected post is nil but actual is not nil")
	}
	if actual == nil {
		t.Fatal("expected post is not nil but actual is nil")
	}

	if expected.ID != actual.ID {
		t.Errorf("post ID mismatch: expected %q, got %q", expected.ID, actual.ID)
	}
	if expected.Name != actual.Name {
		t.Errorf("post Name mismatch: expected %q, got %q", expected.Name, actual.Name)
	}
	if expected.Title != actual.Title {
		t.Errorf("post Title mismatch: expected %q, got %q", expected.Title, actual.Title)
	}
	if expected.Author != actual.Author {
		t.Errorf("post Author mismatch: expected %q, got %q", expected.Author, actual.Author)
	}
	if expected.Subreddit != actual.Subreddit {
		t.Errorf("post Subreddit mismatch: expected %q, got %q", expected.Subreddit, actual.Subreddit)
	}
	if expected.Score != actual.Score {
		t.Errorf("post Score mismatch: expected %d, got %d", expected.Score, actual.Score)
	}
	if expected.NumComments != actual.NumComments {
		t.Errorf("post NumComments mismatch: expected %d, got %d", expected.NumComments, actual.NumComments)
	}
}

// AssertPostsEqual compares two slices of posts for equality.
// It checks that both slices have the same length and that each corresponding
// pair of posts matches using AssertPostEqual.
//
// Example:
//
//	expected := []*types.Post{testutil.DefaultPost()}
//	actual := response.Posts
//	testutil.AssertPostsEqual(t, expected, actual)
func AssertPostsEqual(t *testing.T, expected, actual []*types.Post) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("post count mismatch: expected %d posts, got %d", len(expected), len(actual))
	}

	for i := range expected {
		AssertPostEqual(t, expected[i], actual[i])
	}
}

// AssertCommentEqual compares key fields of two Comment objects for equality.
// It checks ID, Body, Author, ParentID, LinkID, and Score.
// This is useful for verifying that comments are correctly parsed and processed.
//
// Example:
//
//	expected := testutil.DefaultComment()
//	actual := response.Comments[0]
//	testutil.AssertCommentEqual(t, expected, actual)
func AssertCommentEqual(t *testing.T, expected, actual *types.Comment) {
	t.Helper()
	if expected == nil && actual == nil {
		return
	}
	if expected == nil {
		t.Fatal("expected comment is nil but actual is not nil")
	}
	if actual == nil {
		t.Fatal("expected comment is not nil but actual is nil")
	}

	if expected.ID != actual.ID {
		t.Errorf("comment ID mismatch: expected %q, got %q", expected.ID, actual.ID)
	}
	if expected.Name != actual.Name {
		t.Errorf("comment Name mismatch: expected %q, got %q", expected.Name, actual.Name)
	}
	if expected.Body != actual.Body {
		t.Errorf("comment Body mismatch: expected %q, got %q", expected.Body, actual.Body)
	}
	if expected.Author != actual.Author {
		t.Errorf("comment Author mismatch: expected %q, got %q", expected.Author, actual.Author)
	}
	if expected.ParentID != actual.ParentID {
		t.Errorf("comment ParentID mismatch: expected %q, got %q", expected.ParentID, actual.ParentID)
	}
	if expected.LinkID != actual.LinkID {
		t.Errorf("comment LinkID mismatch: expected %q, got %q", expected.LinkID, actual.LinkID)
	}
	if expected.Score != actual.Score {
		t.Errorf("comment Score mismatch: expected %d, got %d", expected.Score, actual.Score)
	}
}

// AssertStringContains checks that str contains substr.
// This is useful for checking error messages or log output.
//
// Example:
//
//	err := client.Connect()
//	testutil.AssertError(t, err)
//	testutil.AssertStringContains(t, err.Error(), "connection refused")
func AssertStringContains(t *testing.T, str, substr string) {
	t.Helper()
	if !strings.Contains(str, substr) {
		t.Errorf("expected string to contain %q, got: %q", substr, str)
	}
}

// AssertPostCount verifies that a PostsResponse contains the expected number of posts.
// This is a common check after fetching posts from the API.
//
// Example:
//
//	response, err := client.GetHot(ctx, &types.PostsRequest{Limit: 10})
//	testutil.AssertNoError(t, err)
//	testutil.AssertPostCount(t, response, 10)
func AssertPostCount(t *testing.T, response *types.PostsResponse, expected int) {
	t.Helper()
	if response == nil {
		t.Fatal("response is nil")
	}
	if len(response.Posts) != expected {
		t.Errorf("expected %d posts, got %d", expected, len(response.Posts))
	}
}

// AssertCommentCount verifies that a CommentsResponse contains the expected number of comments.
// This is a common check after fetching comments from the API.
//
// Example:
//
//	response, err := client.GetComments(ctx, request)
//	testutil.AssertNoError(t, err)
//	testutil.AssertCommentCount(t, response, 5)
func AssertCommentCount(t *testing.T, response *types.CommentsResponse, expected int) {
	t.Helper()
	if response == nil {
		t.Fatal("response is nil")
	}
	if len(response.Comments) != expected {
		t.Errorf("expected %d comments, got %d", expected, len(response.Comments))
	}
}
