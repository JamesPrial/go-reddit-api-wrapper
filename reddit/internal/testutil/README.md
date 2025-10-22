# testutil

Internal test utilities package providing builders, assertions, and mock objects for testing the Reddit API wrapper.

## Overview

The `testutil` package provides a comprehensive suite of testing tools to simplify test writing and reduce boilerplate:

- **Fluent Builders** - Construct test data with sensible defaults and chainable methods
- **Assertions** - Type-safe helpers with clear error messages
- **Mock Server** - Full-featured HTTP server for integration testing
- **Default Helpers** - Quick access to realistic test data

## Quick Start

```go
import "github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"

func TestGetHotPosts(t *testing.T) {
    // Create test data using builders
    post := testutil.NewPostBuilder().
        WithTitle("Test Post").
        WithScore(100).
        Build()

    // Set up mock server
    server := testutil.NewMockServer().
        WithPosts("golang", "hot", post).
        Start()
    defer server.Close()

    // Test your code
    response, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang"})

    // Use assertions
    testutil.AssertNoError(t, err)
    testutil.AssertPostCount(t, response, 1)
}
```

## Components

### Fluent Builders

Type-safe builders with sensible defaults and chainable methods:

- **`NewPostBuilder()`** - Build posts with customizable fields
- **`NewCommentBuilder()`** - Build comments with nested reply support
- **`NewSubreddit(name)`** - Build subreddit metadata
- **`NewAccount(username)`** - Build account data
- **`NewMore()`** - Build "more comments" continuation objects
- **`NewListingBuilder()`** - Build Reddit listing responses
- **`NewMessageBuilder()`** - Build private messages

All builders support `.Build()`, `.ToThing()`, and `.ToJSON()` methods.

### Assertions

Clear, type-safe test assertions that call `t.Helper()` for accurate error locations:

- **Error checks**: `AssertNoError`, `AssertError`, `AssertErrorType`, `AssertAPIError`
- **Data comparison**: `AssertPostEqual`, `AssertPostsEqual`, `AssertCommentEqual`
- **String checks**: `AssertStringContains`
- **Count checks**: `AssertPostCount`, `AssertCommentCount`

### Default Helpers

Quick access to fully-populated test objects:

- `DefaultPost()` - Complete post with realistic defaults
- `DefaultComment()` - Complete comment with realistic defaults
- `DefaultSubreddit()` - Complete subreddit metadata
- `DefaultAccount()` - Complete account data

### MockServer

Full-featured HTTP test server that simulates Reddit's API:

```go
server := testutil.NewMockServer().
    WithSubreddit("golang", subredditData).
    WithPosts("golang", "hot", post1, post2).
    WithComments("golang", "abc123", post, comment1, comment2).
    WithAccount(accountData).
    WithError("/r/private", http.StatusForbidden, "Forbidden").
    Start()
```

Supports all common endpoints (hot, new, top, comments, about, /api/v1/me) with realistic rate limit headers.

### MockTokenProvider

Simple mock for authentication testing:

```go
// Success case
mockAuth := &testutil.MockTokenProvider{Token: "valid-token"}

// Failure case
mockAuth := &testutil.MockTokenProvider{Err: errors.New("auth failed")}
```

## Documentation

See **[USAGE.md](USAGE.md)** for detailed examples and comprehensive usage guide.

## Notes

- All builders use Reddit's proper fullname formats (e.g., "t3_" for posts, "t1_" for comments)
- Builders automatically set related fields (e.g., `WithID` updates both `ID` and `Name`)
- MockServer includes rate limit headers and proper Reddit API response structure
- This package is internal to avoid import cycles with the main `reddit` package
