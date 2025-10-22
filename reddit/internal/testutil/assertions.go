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

// assertErrorByTypeName is a helper that checks if an error (or any error in its chain)
// matches the expected type name using reflection.
func assertErrorByTypeName(err error, expectedTypeName string) bool {
	if err == nil {
		return false
	}

	// Check the current error
	errType := reflect.TypeOf(err)
	if errType != nil {
		typeName := errType.String()
		if strings.HasSuffix(typeName, expectedTypeName) {
			return true
		}
	}

	// Check wrapped errors
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		return assertErrorByTypeName(unwrapped, expectedTypeName)
	}

	return false
}

// AssertAuthError checks that err is of type *AuthError and optionally
// validates that the error message contains the expected message string.
// If expectedMsg is empty, only the type is checked.
//
// This is useful for testing authentication failures and credential validation.
//
// Example:
//
//	err := client.Authenticate()
//	testutil.AssertAuthError(t, err, "invalid credentials")
//
//	// Or just check the type:
//	testutil.AssertAuthError(t, err, "")
func AssertAuthError(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected AuthError, got nil")
	}

	if !assertErrorByTypeName(err, "AuthError") {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}

	if expectedMsg != "" && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected AuthError message to contain %q, got: %q", expectedMsg, err.Error())
	}
}

// AssertValidationError checks that err is of type *ValidationError and
// optionally validates that the error message contains the expected message string.
// If expectedMsg is empty, only the type is checked.
//
// This is useful for testing input validation failures for subreddit names,
// post IDs, pagination parameters, and other user inputs.
//
// Example:
//
//	err := client.GetHot(ctx, &types.PostsRequest{
//	    Subreddit: "invalid name!",
//	})
//	testutil.AssertValidationError(t, err, "subreddit")
//
//	// Or just check the type:
//	testutil.AssertValidationError(t, err, "")
func AssertValidationError(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected ValidationError, got nil")
	}

	if !assertErrorByTypeName(err, "ValidationError") {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}

	if expectedMsg != "" && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected ValidationError message to contain %q, got: %q", expectedMsg, err.Error())
	}
}

// AssertParseError checks that err is of type *ParseError and optionally
// validates that the error message contains the expected message string.
// If expectedMsg is empty, only the type is checked.
//
// This is useful for testing response parsing failures when the API returns
// unexpected data structures or malformed JSON.
//
// Example:
//
//	response, err := client.GetHot(ctx, request)
//	testutil.AssertParseError(t, err, "invalid JSON")
//
//	// Or just check the type:
//	testutil.AssertParseError(t, err, "")
func AssertParseError(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected ParseError, got nil")
	}

	if !assertErrorByTypeName(err, "ParseError") {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}

	if expectedMsg != "" && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected ParseError message to contain %q, got: %q", expectedMsg, err.Error())
	}
}

// AssertNetworkError checks that err is of type *NetworkError and optionally
// validates that the error message contains the expected message string.
// If expectedMsg is empty, only the type is checked.
//
// This is useful for testing network-level failures such as connection timeouts,
// DNS resolution failures, and other transport-level errors.
//
// Example:
//
//	response, err := client.GetHot(ctx, request)
//	testutil.AssertNetworkError(t, err, "connection refused")
//
//	// Or just check the type:
//	testutil.AssertNetworkError(t, err, "")
func AssertNetworkError(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected NetworkError, got nil")
	}

	if !assertErrorByTypeName(err, "NetworkError") {
		t.Fatalf("expected *NetworkError, got %T: %v", err, err)
	}

	if expectedMsg != "" && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected NetworkError message to contain %q, got: %q", expectedMsg, err.Error())
	}
}

// AssertRateLimitError checks that err is of type *RateLimitError.
// This assertion does not validate the error message, but ensures the error
// is properly typed as a rate limit error.
//
// This is useful for testing rate limiting behavior, including context
// cancellation while waiting for rate limit availability.
//
// Example:
//
//	// Simulate hitting rate limit
//	for i := 0; i < 100; i++ {
//	    _, err := client.GetHot(ctx, request)
//	    if err != nil {
//	        testutil.AssertRateLimitError(t, err)
//	        break
//	    }
//	}
func AssertRateLimitError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected RateLimitError, got nil")
	}

	if !assertErrorByTypeName(err, "RateLimitError") {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
}

// AssertErrorChain validates that an error wraps other errors in a specific order
// by checking type names using reflection. This is useful for testing error wrapping
// and ensuring that error context is properly preserved through the error chain.
//
// The expectedTypeNames parameter should be a slice of error type names (e.g., "AuthError",
// "NetworkError") representing the expected error types in the chain. The order matters:
// types are checked from outermost to innermost in the error chain.
//
// Example:
//
//	// Test that an error chain contains both ParseError and NetworkError
//	err := client.GetHot(ctx, request)
//	testutil.AssertErrorChain(t, err, "ParseError", "NetworkError")
//
//	// Test a more complex chain
//	testutil.AssertErrorChain(t, err, "APIError", "AuthError", "NetworkError")
func AssertErrorChain(t *testing.T, err error, expectedTypeNames ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error with chain, got nil")
	}

	for i, expectedTypeName := range expectedTypeNames {
		if !assertErrorByTypeName(err, expectedTypeName) {
			// Build a helpful error message showing what we found
			var foundTypes []string
			for j := 0; j < i; j++ {
				foundTypes = append(foundTypes, expectedTypeNames[j])
			}
			if len(foundTypes) > 0 {
				t.Fatalf("error chain broken at position %d: expected *%s but it was not found in chain. Found types so far: %v. Error type: %T, Error: %v",
					i, expectedTypeName, foundTypes, err, err)
			} else {
				t.Fatalf("error chain broken at position %d: expected *%s but it was not found in chain. Error type: %T, Error: %v",
					i, expectedTypeName, err, err)
			}
		}
	}
}
