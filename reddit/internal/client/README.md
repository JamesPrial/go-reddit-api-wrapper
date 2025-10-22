# client

Internal HTTP client package for Reddit API communication.

## Overview

This package provides a production-ready HTTP client for the Reddit API with built-in rate limiting, buffer pooling, structured logging, and comprehensive error handling. It handles Reddit-specific response formats and proactively manages rate limits based on Reddit's response headers.

## Key Components

### Client

The `Client` struct manages all HTTP communication with Reddit's API:

```go
type Client struct {
    client          *http.Client
    BaseURL         *url.URL
    UserAgent       string
    logger          *slog.Logger
    limiter         *rate.Limiter
    clock           clock.Clock
    // ... internal fields for rate limiting and logging
}
```

**Configuration:**
- `NewClient()` - Create with default rate limiting (100 req/min, burst of 10)
- `NewClientWithRateLimit()` - Create with custom rate limiting and clock injection for testing

**Rate Limiting:**
- Local rate limiter (`golang.org/x/time/rate`) for client-side throttling
- Proactive rate limiting based on Reddit's `X-Ratelimit-Remaining` and `X-Ratelimit-Reset` headers
- Automatic handling of `Retry-After` headers (e.g., when approaching limits)
- Concurrent-safe forced delay mechanism using atomic operations

### Request Methods

**`Do(req *http.Request, v *types.Thing) error`**
- General-purpose method for Reddit API requests expecting a single Thing response
- Handles authentication headers, rate limiting, and response decoding
- Returns typed errors for different failure scenarios

**`DoThingArray(req *http.Request) ([]*types.Thing, error)`**
- Specialized method for Reddit's comments endpoint
- Handles both array responses `[post, comments]` and single Listing responses
- Validates response structure and extracts error details

**`DoMoreChildren(req *http.Request) ([]*types.Thing, error)`**
- Specialized method for the `/api/morechildren` endpoint
- Parses Reddit's nested `{"json":{"data":{"things":[...]}}}` structure
- Handles API error arrays in the response

### Error Types

All errors implement clear messages and context for debugging:

- **`RequestBuildError`** - URL parsing or request construction failures
- **`RateLimitError`** - Rate limiting wait failures (e.g., context cancellation)
- **`TransportError`** - Network-level failures (timeouts, connection errors)
- **`ResponseReadError`** - I/O errors reading response body, or size limit violations
- **`DecodeError`** - JSON parsing failures with body snippets for debugging
- **`ResponseValidationError`** - Unexpected response structure
- **`APIError`** - Reddit API error responses with status code and error details

All errors that wrap underlying errors implement `Unwrap()` for use with `errors.Is()` and `errors.As()`.

## Design Patterns

### Buffer Pooling

Response bodies are read using pooled `bytes.Buffer` instances to reduce allocations:

```go
buf := getBuffer()
defer putBuffer(buf)
// ... use buffer
```

- Buffers larger than `MAX_BUFFER_SIZE` (256KB) are discarded to prevent memory bloat
- Initial buffer size is 8KB to handle most Reddit responses without reallocation
- Thread-safe with automatic statistics tracking

### Proactive Rate Limiting

Reddit's rate limit headers are used to calculate optimal delays:

```go
// When remaining requests drop below threshold (default: 5)
// Calculate delay: (resetSeconds * 1.1) / remaining
// This spreads remaining requests over the reset period
```

- Adds 10% safety buffer (`RateLimitBufferMultiplier`)
- Only updates if new delay is longer than current delay (CAS loop)
- Context-aware to handle cancellation during rate limit waits

### Clock Abstraction

Time operations use the `clock.Clock` interface:
- Enables instant testing of time-dependent behavior (no `time.Sleep` in tests)
- Real clock used in production, mock clock in tests
- All timer operations go through `clock.After()` for cancellation support

### Structured Logging

Optional structured logging with `log/slog`:
- Request/response info at INFO level
- Errors at WARN/ERROR levels with full context
- Optional DEBUG logging with configurable body capture limits
- Rate limit header values logged for monitoring

## Implementation Notes

### Thread Safety

- Rate limiter is thread-safe (provided by `golang.org/x/time/rate`)
- Forced delay (`forceWaitUntil`) uses atomic operations for lock-free reads
- Delay updates use CAS (Compare-And-Swap) loop to prevent shorter delays from overriding longer ones

### Response Size Limits

- Maximum response body size: 10MB (`MAX_RESPONSE_BODY_SIZE`)
- Enforced using `io.LimitReader` to prevent DoS attacks
- Errors clearly indicate when size limit is exceeded

### Reddit API Specifics

- **Base URL**: Must end with `/` (automatically added if missing)
- **User-Agent**: Required by Reddit, set on all requests
- **Rate Limit Headers**:
  - `X-Ratelimit-Remaining`: Requests remaining in current window (float)
  - `X-Ratelimit-Reset`: Seconds until reset (delta time, not Unix timestamp)
  - `Retry-After`: Seconds to wait before retrying (optional)

### Error Extraction

The `extractAPIErrorDetails()` function handles Reddit's various error formats:
- Standard `{"error": "CODE", "message": "text"}` format
- Nested `{"json": {"errors": [["CODE", "message", details]]}}` format
- Fallback to `reason`, `explanation`, or `error_description` fields

## Testing

Tests use dependency injection for deterministic behavior:
- Mock HTTP client (no real network calls)
- Mock clock (instant time-dependent tests)
- Test server for integration scenarios
- Comprehensive coverage of error paths and edge cases

Example test setup:
```go
mockClock := clock.NewMockClock(time.Time{})
client, _ := NewClientWithRateLimit(httpClient, baseURL, userAgent, logger, RateLimitConfig{}, mockClock)
```

## Relationship to Other Packages

- **`reddit/`** - Public client uses this package internally via dependency injection
- **`reddit/internal/auth`** - Sets `Authorization` headers on requests before passing to this client
- **`pkg/types`** - Provides `Thing` and other response types decoded by this client
- **`reddit/internal/clock`** - Provides time abstraction for testing
- **`reddit/internal/testutil`** - Test helpers for assertions and builders

## Maintenance Notes

- This is an **internal package** - breaking changes do not affect public API
- Always use the `clock.Clock` interface for time operations (enables testing)
- Preserve buffer pool statistics for future monitoring/telemetry
- Consider increasing `ProactiveRateLimitThreshold` if hitting rate limits frequently
- Error types should include all context needed for debugging (URLs, status codes, snippets)
