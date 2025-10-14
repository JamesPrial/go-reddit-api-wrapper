# Test Utilities Usage Guide

This directory contains reusable test helpers for the Reddit API wrapper. The utilities are organized into two main files:

## Files Created

1. **assertions.go** - Test assertion helpers that fail tests with clear messages
2. **helpers.go** - Helper functions for creating test data and mock objects

## Quick Start

### Using Assertions

```go
import "github.com/jamesprial/go-reddit-api-wrapper/internal/testutil"

func TestGetHotPosts(t *testing.T) {
    client := setupTestClient()

    response, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang"})

    // Use assertions for cleaner test code
    testutil.AssertNoError(t, err)
    testutil.AssertPostCount(t, response, 10)
    testutil.AssertStringContains(t, response.Posts[0].Title, "Go")
}
```

### Using Default Data Helpers

```go
func TestPostProcessing(t *testing.T) {
    // Get a fully-populated default post
    post := testutil.DefaultPost()

    // Customize only the fields you care about
    post.Title = "Custom Title"
    post.Score = 500

    // Use in your test
    result := processPost(post)
    testutil.AssertStringContains(t, result, "Custom Title")
}
```

### Using MockTokenProvider

```go
func TestAuthenticationError(t *testing.T) {
    mockAuth := &testutil.MockTokenProvider{
        Err: errors.New("authentication failed"),
    }

    token, err := mockAuth.GetToken(ctx)
    testutil.AssertError(t, err)
    testutil.AssertStringContains(t, err.Error(), "authentication failed")
}
```

## Assertion Functions

### Error Assertions

- `AssertNoError(t, err)` - Fail if error is not nil
- `AssertError(t, err)` - Fail if error is nil
- `AssertErrorType(t, err, &expectedType)` - Check error type with errors.As
- `AssertAPIError(t, err, statusCode)` - Check for APIError with specific status

### Data Comparison

- `AssertPostEqual(t, expected, actual)` - Compare key post fields
- `AssertPostsEqual(t, expected, actual)` - Compare post slices
- `AssertCommentEqual(t, expected, actual)` - Compare key comment fields

### String Assertions

- `AssertStringContains(t, str, substr)` - Check substring presence

### Count Assertions

- `AssertPostCount(t, response, expected)` - Verify post count
- `AssertCommentCount(t, response, expected)` - Verify comment count

## Helper Functions

### Default Data Generators

Create fully-populated test objects with realistic default values:

- `DefaultPost()` - Returns a complete Post with realistic data
- `DefaultComment()` - Returns a complete Comment with realistic data
- `DefaultSubreddit()` - Returns a complete SubredditData with realistic data
- `DefaultAccount()` - Returns a complete AccountData with realistic data

All defaults include:
- Proper ID format (e.g., "t3_abc123" for posts)
- Realistic timestamps
- Valid relationships (subreddit IDs, parent IDs, etc.)
- Sensible default values for all fields

### Mock Objects

- `MockTokenProvider` - Mock implementation of TokenProvider interface
  - Set `Token` field for successful auth
  - Set `Err` field for auth failures
  - Track invalidation calls with `InvalidateCount`

## Example: Complete Test Function

```go
func TestFetchAndProcessComments(t *testing.T) {
    // Create test data
    comment1 := testutil.DefaultComment()
    comment1.Body = "First comment"
    comment1.Score = 100

    comment2 := testutil.DefaultComment()
    comment2.ID = "comment2"
    comment2.Name = "t1_comment2"
    comment2.Body = "Second comment"
    comment2.Score = 50

    // Create mock response
    response := &types.CommentsResponse{
        Post: testutil.DefaultPost(),
        Comments: []*types.Comment{comment1, comment2},
    }

    // Test your function
    result := ProcessComments(response)

    // Use assertions
    testutil.AssertNoError(t, result.Err)
    testutil.AssertCommentCount(t, response, 2)
    testutil.AssertStringContains(t, result.Summary, "2 comments")

    // Verify specific comment
    testutil.AssertCommentEqual(t, comment1, result.TopComment)
}
```

## Integration with Existing Builders

This package also includes fluent builders for creating more complex test data:

- `NewPostBuilder()` - Build posts with fluent API
- `NewCommentBuilder()` - Build comments with nested replies
- `NewSubreddit(name)` - Build subreddit data
- `NewAccount(username)` - Build account data
- `NewMore()` - Build "more comments" objects

See the existing builder test files for examples of these.

## Benefits

1. **Consistent Error Messages**: All assertions provide clear, detailed failure messages
2. **Less Boilerplate**: Reduce repetitive test code
3. **Type Safety**: Assertion helpers are type-safe and catch errors at compile time
4. **Maintainability**: Centralized test utilities are easier to update
5. **Readability**: Tests read more like specifications

## Notes

- All assertion functions call `t.Helper()` to provide accurate line numbers in failure messages
- Default helpers create realistic data that passes validation
- Mock objects are simple and easy to customize for specific test scenarios
