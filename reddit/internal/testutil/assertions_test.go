package testutil

import (
	"errors"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// testConfigError is a mock error type for testing
type testConfigError struct {
	Field   string
	Message string
}

func (e *testConfigError) Error() string {
	return e.Message
}

// testAPIError is a mock error type for testing
type testAPIError struct {
	StatusCode int
	Message    string
}

func (e *testAPIError) Error() string {
	return e.Message
}

// TestAssertNoError verifies AssertNoError helper works correctly
func TestAssertNoError(t *testing.T) {
	// This test should pass
	AssertNoError(t, nil)
}

// TestAssertError verifies AssertError helper works correctly
func TestAssertError(t *testing.T) {
	// This test should pass when given an error
	AssertError(t, errors.New("test error"))
}

// TestAssertErrorType verifies error type checking
func TestAssertErrorType(t *testing.T) {
	err := &testConfigError{Field: "test", Message: "test error"}
	var configErr *testConfigError
	AssertErrorType(t, err, &configErr)

	// Verify the error was properly unwrapped
	if configErr.Field != "test" {
		t.Errorf("Expected field 'test', got %q", configErr.Field)
	}
}

// TestAssertAPIError verifies API error checking
func TestAssertAPIError(t *testing.T) {
	err := &testAPIError{StatusCode: 404, Message: "Not Found"}
	AssertAPIError(t, err, 404)
}

// TestAssertPostEqual verifies post comparison
func TestAssertPostEqual(t *testing.T) {
	post1 := DefaultPost()
	post2 := DefaultPost()

	// Should pass - both posts are identical
	AssertPostEqual(t, post1, post2)

	// Modify a field
	post2.Title = "Different Title"

	// This would fail if we called AssertPostEqual again
	// but we can't test that without a sub-test framework
}

// TestAssertPostsEqual verifies post slice comparison
func TestAssertPostsEqual(t *testing.T) {
	posts1 := []*types.Post{DefaultPost(), DefaultPost()}
	posts2 := []*types.Post{DefaultPost(), DefaultPost()}

	AssertPostsEqual(t, posts1, posts2)
}

// TestAssertCommentEqual verifies comment comparison
func TestAssertCommentEqual(t *testing.T) {
	comment1 := DefaultComment()
	comment2 := DefaultComment()

	AssertCommentEqual(t, comment1, comment2)
}

// TestAssertStringContains verifies substring checking
func TestAssertStringContains(t *testing.T) {
	AssertStringContains(t, "hello world", "world")
	AssertStringContains(t, "error: something failed", "failed")
}

// TestAssertPostCount verifies post count checking
func TestAssertPostCount(t *testing.T) {
	response := &types.PostsResponse{
		Posts: []*types.Post{
			DefaultPost(),
			DefaultPost(),
			DefaultPost(),
		},
	}

	AssertPostCount(t, response, 3)
}

// TestAssertCommentCount verifies comment count checking
func TestAssertCommentCount(t *testing.T) {
	response := &types.CommentsResponse{
		Post: DefaultPost(),
		Comments: []*types.Comment{
			DefaultComment(),
			DefaultComment(),
		},
	}

	AssertCommentCount(t, response, 2)
}

// TestDefaultPost verifies DefaultPost helper
func TestDefaultPost(t *testing.T) {
	post := DefaultPost()

	if post == nil {
		t.Fatal("DefaultPost returned nil")
	}

	if post.ID != "abc123" {
		t.Errorf("Expected ID 'abc123', got %q", post.ID)
	}

	if post.Name != "t3_abc123" {
		t.Errorf("Expected Name 't3_abc123', got %q", post.Name)
	}

	if post.Title == "" {
		t.Error("Expected Title to be set")
	}

	if post.Subreddit == "" {
		t.Error("Expected Subreddit to be set")
	}
}

// TestDefaultComment verifies DefaultComment helper
func TestDefaultComment(t *testing.T) {
	comment := DefaultComment()

	if comment == nil {
		t.Fatal("DefaultComment returned nil")
	}

	if comment.ID != "def456" {
		t.Errorf("Expected ID 'def456', got %q", comment.ID)
	}

	if comment.Name != "t1_def456" {
		t.Errorf("Expected Name 't1_def456', got %q", comment.Name)
	}

	if comment.Body == "" {
		t.Error("Expected Body to be set")
	}

	if comment.Author == "" {
		t.Error("Expected Author to be set")
	}
}

// TestDefaultSubreddit verifies DefaultSubreddit helper
func TestDefaultSubreddit(t *testing.T) {
	subreddit := DefaultSubreddit()

	if subreddit == nil {
		t.Fatal("DefaultSubreddit returned nil")
	}

	if subreddit.DisplayName == "" {
		t.Error("Expected DisplayName to be set")
	}

	if subreddit.Subscribers <= 0 {
		t.Error("Expected Subscribers to be > 0")
	}
}

// TestDefaultAccount verifies DefaultAccount helper
func TestDefaultAccount(t *testing.T) {
	account := DefaultAccount()

	if account == nil {
		t.Fatal("DefaultAccount returned nil")
	}

	if account.ID != "test123" {
		t.Errorf("Expected ID 'test123', got %q", account.ID)
	}

	if account.Name != "t2_test123" {
		t.Errorf("Expected Name 't2_test123', got %q", account.Name)
	}
}

// TestMockTokenProvider verifies MockTokenProvider implementation
func TestMockTokenProvider(t *testing.T) {
	// Test successful token retrieval
	mock := &MockTokenProvider{Token: "test-token"}
	token, err := mock.GetToken(nil)

	AssertNoError(t, err)
	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got %q", token)
	}

	// Test error case
	mockErr := &MockTokenProvider{Err: errors.New("auth failed")}
	_, err = mockErr.GetToken(nil)

	AssertError(t, err)
	AssertStringContains(t, err.Error(), "auth failed")

	// Test InvalidateToken tracking
	mock.InvalidateToken()
	if mock.InvalidateCount != 1 {
		t.Errorf("Expected InvalidateCount 1, got %d", mock.InvalidateCount)
	}

	mock.InvalidateToken()
	if mock.InvalidateCount != 2 {
		t.Errorf("Expected InvalidateCount 2, got %d", mock.InvalidateCount)
	}
}
