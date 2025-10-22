# MockServer Enhanced Features

This document summarizes the enhanced test server utilities added to the `testutil` package. These features standardize common test patterns and make it easier to test edge cases, error scenarios, and pagination logic.

## Overview

Three major enhancements have been added:

1. **Error Scenario Methods** - Easily configure common error conditions
2. **Pagination Helper** - Simplified multi-page response testing
3. **Custom Response Server** - Complete control for edge case testing

## 1. Error Scenario Methods

### WithStatusCode(code int)

Returns a specific HTTP status code for all requests. Useful for testing error handling and edge cases.

```go
server := testutil.NewMockServer().
    WithStatusCode(http.StatusServiceUnavailable).
    Start()
defer server.Close()

// All requests will return 503 Service Unavailable
```

**Use cases:**
- Testing 500-series server errors
- Testing 400-series client errors
- Testing rate limiting (429)
- Testing service unavailability

### WithTimeout(duration time.Duration)

Simulates network latency by delaying responses. Useful for testing timeout handling.

```go
server := testutil.NewMockServer().
    WithTimeout(2 * time.Second).
    Start()
defer server.Close()

// All requests will be delayed by 2 seconds
```

**Use cases:**
- Testing timeout error handling
- Testing client retry logic with delays
- Simulating slow network conditions
- Testing deadline exceeded scenarios

### WithMalformedJSON()

Returns invalid JSON to test parsing error handling.

```go
server := testutil.NewMockServer().
    WithMalformedJSON().
    Start()
defer server.Close()

// Returns: {"kind": "Listing", "data": {"children": [
// (incomplete JSON)
```

**Use cases:**
- Testing JSON parsing error handling
- Testing error recovery from malformed responses
- Validating error messages for parsing failures

### WithEmptyResponse()

Returns 200 OK with an empty response body.

```go
server := testutil.NewMockServer().
    WithEmptyResponse().
    Start()
defer server.Close()

// Returns 200 OK with no body
```

**Use cases:**
- Testing handling of unexpected empty responses
- Testing error recovery from missing data
- Validating assumptions about response content

### Configuration Priority

Error scenario methods have the following priority (highest to lowest):

1. `WithStatusCode()` - Overrides all other configurations
2. `WithMalformedJSON()` - Overrides normal responses
3. `WithEmptyResponse()` - Overrides normal responses
4. `WithError()` - Path-specific errors
5. Normal mock data (`WithPosts`, `WithSubreddit`, etc.)

## 2. Pagination Helper

### WithPaginatedPosts(subreddit, sort string, pages map[string][]*types.Post)

Configures multi-page post responses where the map key is the "after" parameter value.

```go
pages := map[string][]*types.Post{
    "":          {post1, post2},    // First page (no after param)
    "t3_post2": {post3, post4},     // Second page (after=t3_post2)
    "t3_post4": {post5, post6},     // Third page (after=t3_post4)
}

server := testutil.NewMockServer().
    WithPaginatedPosts("golang", "hot", pages).
    Start()
defer server.Close()

// GET /r/golang/hot -> returns post1, post2 with after=t3_post2
// GET /r/golang/hot?after=t3_post2 -> returns post3, post4 with after=t3_post4
// GET /r/golang/hot?after=t3_post4 -> returns post5, post6 with after=""
```

**Features:**
- Automatic "after" field generation based on last post in page
- Automatic "before" field set to current "after" parameter
- Empty "after" field when no next page is configured
- Supports multiple subreddits and sort orders

**Use cases:**
- Testing pagination logic through multiple pages
- Testing end-of-pagination handling
- Testing pagination token consistency
- Testing concurrent page fetching

**Example: Complete Pagination Flow**

```go
post1 := testutil.NewPostBuilder().WithID("post1").WithTitle("First").Build()
post2 := testutil.NewPostBuilder().WithID("post2").WithTitle("Second").Build()
post3 := testutil.NewPostBuilder().WithID("post3").WithTitle("Third").Build()

pages := map[string][]*types.Post{
    "":         {post1, post2},
    "t3_post2": {post3},  // Last page
}

server := testutil.NewMockServer().
    WithPaginatedPosts("golang", "hot", pages).
    Start()
defer server.Close()

// Navigate through all pages
currentAfter := ""
for {
    url := server.URL() + "/r/golang/hot"
    if currentAfter != "" {
        url += "?after=" + currentAfter
    }

    resp, _ := http.Get(url)
    // Parse response and extract next "after" token
    // Break if "after" is empty (last page)
}
```

## 3. Custom Response Server Helper

### NewCustomResponseServer(handler http.HandlerFunc)

Creates a test server with complete control over HTTP responses. Automatically adds standard Reddit API rate limit headers.

```go
server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"custom": "response"}`))
})
defer server.Close()
```

**Features:**
- Full control over response status, headers, and body
- Automatic rate limit headers (`X-Ratelimit-Remaining`, `X-Ratelimit-Reset`)
- Access to request details (URL, method, headers, body)
- Can simulate connection issues by hijacking the connection

**Use cases:**
- Testing edge cases not covered by MockServer
- Simulating connection interruptions
- Testing custom response headers
- Testing request validation logic
- Simulating intermittent failures

**Example: Simulating Network Interruption**

```go
server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    // Write partial response
    w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))

    // Hijack connection to simulate abrupt disconnect
    if hj, ok := w.(http.Hijacker); ok {
        conn, _, _ := hj.Hijack()
        conn.Close()
    }
})
defer server.Close()
```

**Example: Intermittent Failures**

```go
var requestCount int

server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
    requestCount++

    // Fail every third request
    if requestCount%3 == 0 {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status": "ok"}`))
})
defer server.Close()
```

## Method Chaining

All MockServer methods support fluent chaining:

```go
server := testutil.NewMockServer().
    WithSubreddit("golang", subreddit).
    WithPosts("golang", "hot", post1, post2).
    WithPaginatedPosts("rust", "hot", rustPages).
    WithError("/r/private", http.StatusForbidden, "Private").
    WithTimeout(100 * time.Millisecond).
    Start()
defer server.Close()
```

## Migration Guide

### Before: Custom Pagination Server

Old pattern from `pagination_logic_test.go`:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("X-Ratelimit-Remaining", "60")
    w.Header().Set("X-Ratelimit-Reset", "60")
    w.Header().Set("Content-Type", "application/json")

    after := r.URL.Query().Get("after")

    // Complex pagination logic...
}))
```

### After: WithPaginatedPosts

New pattern:

```go
pages := map[string][]*types.Post{
    "": {post1, post2},
    "t3_post2": {post3, post4},
}

server := testutil.NewMockServer().
    WithPaginatedPosts("golang", "hot", pages).
    Start()
```

### Before: Custom Error Responses

Old pattern from `response_edge_cases_test.go`:

```go
type customResponseServer struct {
    server  *httptest.Server
    handler http.HandlerFunc
}

func newCustomResponseServer(handler http.HandlerFunc) *customResponseServer {
    // ...
}

server := newCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(""))  // Empty response
})
```

### After: WithEmptyResponse or NewCustomResponseServer

New pattern:

```go
// For simple empty response
server := testutil.NewMockServer().
    WithEmptyResponse().
    Start()

// For custom logic
server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(""))
})
```

## Testing Best Practices

1. **Use the most specific method** - Prefer `WithStatusCode` over custom handlers for simple status codes
2. **Combine methods** - Use multiple error scenario methods together for complex test cases
3. **Test error recovery** - Use timeout and status code methods to test retry logic
4. **Test edge cases** - Use malformed JSON and empty responses to test error handling
5. **Test pagination thoroughly** - Use `WithPaginatedPosts` to test forward/backward navigation, empty results, and token consistency

## Thread Safety

- `MockServer` is safe to use from multiple goroutines after `Start()` is called
- Individual request handlers run concurrently
- `NewCustomResponseServer` handlers must be thread-safe if they access shared state

## Performance Considerations

- `WithTimeout` adds actual delay - use short durations in tests (100ms-500ms)
- Pagination creates multiple response objects - limit page count in tests
- Custom response servers can simulate slow responses without blocking test execution

## Examples

See the following files for comprehensive examples:

- `mockserver_features_test.go` - Unit tests demonstrating all features
- `mockserver_features_example_test.go` - Example tests with output verification
- `mockserver_example_test.go` - Original MockServer examples

## Summary

These enhancements make it easier to:

1. **Test error scenarios** - Status codes, timeouts, malformed responses
2. **Test pagination** - Multi-page navigation without complex server setup
3. **Test edge cases** - Complete control over responses with custom handlers
4. **Write cleaner tests** - Fluent API with method chaining
5. **Maintain backward compatibility** - All existing MockServer functionality remains unchanged
