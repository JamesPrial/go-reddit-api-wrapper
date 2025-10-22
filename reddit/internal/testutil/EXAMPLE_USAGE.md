# Mock Usage Examples

This document demonstrates how to use the new `MockHTTPClient` and `MockParser` utilities in tests.

## MockHTTPClient Examples

### Basic Usage with Default Behavior

```go
// Create a mock that succeeds with empty responses
mock := testutil.NewMockHTTPClient()

// Use in tests
req, _ := mock.NewRequest(ctx, "GET", "/r/golang/hot", nil)
err := mock.Do(req, &thing)

// Verify calls
if mock.DoCalls() != 1 {
    t.Errorf("expected 1 Do call, got %d", mock.DoCalls())
}
```

### Error Testing

```go
// Test error handling
mock := testutil.NewMockHTTPClient().WithError(errors.New("network timeout"))

err := mock.Do(req, &thing)
// err will be "network timeout"
```

### Success with Data

```go
// Test successful response with data
expectedThing := &types.Thing{Kind: "Listing"}
mock := testutil.NewMockHTTPClient().WithSuccess(expectedThing)

var thing types.Thing
mock.Do(req, &thing)
// thing will contain the expected data
```

### Custom Behavior

```go
// Full control over behavior
mock := testutil.NewMockHTTPClient()
mock.DoFunc = func(req *http.Request, v *types.Thing) error {
    // Custom logic
    if req.URL.Path == "/error" {
        return errors.New("custom error")
    }
    v.Kind = "Listing"
    return nil
}
```

### Thread-Safe Call Tracking

```go
mock := testutil.NewMockHTTPClient()

// Make concurrent calls
for i := 0; i < 10; i++ {
    go func() {
        req, _ := mock.NewRequest(ctx, "GET", "/test", nil)
        mock.Do(req, &types.Thing{})
    }()
}

// Atomic counters ensure accurate counts
if mock.DoCalls() != 10 {
    t.Error("expected 10 calls")
}
```

## MockParser Examples

### Basic Usage

```go
// Create a parser that returns empty results
mock := testutil.NewMockParser()

result, err := mock.ParseThing(ctx, thing)
// result will be nil, err will be nil
```

### Return Test Data

```go
// Return specific posts
expectedPosts := []*types.Post{testutil.DefaultPost()}
mock := testutil.NewMockParser().WithPosts(expectedPosts)

posts, err := mock.ExtractPosts(ctx, thing)
// posts will contain the expected posts
```

### Return Comments Response

```go
// Return specific comments
expectedResp := &types.CommentsResponse{
    Post: testutil.DefaultPost(),
    Comments: []*types.Comment{testutil.DefaultComment()},
}
mock := testutil.NewMockParser().WithCommentsResponse(expectedResp)

resp, err := mock.ExtractPostAndComments(ctx, things)
// resp will contain the expected data
```

### Custom Parsing Logic

```go
mock := testutil.NewMockParser()
mock.ExtractPostsFunc = func(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
    // Custom logic
    if thing.Kind != "Listing" {
        return nil, errors.New("invalid kind")
    }
    return []*types.Post{testutil.DefaultPost()}, nil
}
```

## Replacing reddit_test.go Inline Mocks

### Before (inline mock)

```go
type mockHTTPClient struct {
    doFunc func(req *http.Request, v *types.Thing) error
}

func (m *mockHTTPClient) NewRequest(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
    req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
    if len(params) > 0 && params[0] != nil {
        req.URL.RawQuery = params[0].Encode()
    }
    return req, nil
}

func (m *mockHTTPClient) Do(req *http.Request, v *types.Thing) error {
    if m.doFunc != nil {
        return m.doFunc(req, v)
    }
    return nil
}

// ... more methods
```

### After (using testutil)

```go
import "github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"

mock := testutil.NewMockHTTPClient()
mock.DoFunc = func(req *http.Request, v *types.Thing) error {
    // Custom behavior
    return nil
}

// Or use helpers
mock := testutil.NewMockHTTPClient().WithError(someError)
```

## Benefits

1. **Less Boilerplate**: No need to define mock structs in every test file
2. **Thread-Safe**: Atomic counters for concurrent testing
3. **Consistent**: Same mock across all tests
4. **Helper Methods**: Common scenarios (error, success) are one-liners
5. **Call Tracking**: Built-in verification of call counts
6. **Reusable**: Import once, use everywhere
