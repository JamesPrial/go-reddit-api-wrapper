# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go wrapper for the Reddit API providing OAuth2 authentication and a clean interface for common Reddit operations. The library supports both application-only and user authentication modes.

Always use context7 when I need code generation, setup or configuration steps, or library/API documentation. This means you should automatically use the Context7 MCP tools to resolve library id and get library docs without me having to explicitly ask.

## Context Management

Alongside the custom subagents, your Task tool can be used to delegate to subagents, avoiding bloating your context window, as well as creating a fresh one for every task.
Proactively think hard about how problems or requests can be decomposed to utilize the Task tool to manage your context effectively.

## Key Commands

### Testing
**ALWAYS use @agent-go-test-runner to execute tests** - this agent is optimized for Go test execution and provides focused summaries.

```bash
# Run all tests with race detection (matches CI pipeline)
go test -v -race -cover ./...

# Run tests for a specific package
go test -v ./reddit/internal

# Run a specific test by name
go test -v -run TestAuthenticator_GetToken ./reddit/internal

# Run tests with coverage report
go test -cover ./...
go tool cover -func=coverage.out

# Run benchmarks
go test -bench=. ./reddit/internal
```

### Building
```bash
# Build all example applications
go build -o reddit-example-basic ./cmd/examples/basic
go build -o reddit-example-monitor ./cmd/examples/monitor
go build -o reddit-example-analyzer ./cmd/examples/analyzer

# Build with race detection
go build -race -o reddit-example ./cmd/examples/basic
```

### Linting & Code Quality
```bash
# Run go vet (always run before committing)
go vet ./...

# Format all code
go fmt ./...

# Verify dependencies
go mod verify

# Tidy dependencies
go mod tidy
```

### Running the Examples
```bash
# Set required environment variables
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
# Optional for user auth:
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"

# Run examples
go run ./cmd/examples/basic
go run ./cmd/examples/monitor
go run ./cmd/examples/analyzer
```

## Architecture

**UTILIZE @agent-codebase-navigator WHEN CONTEXT FROM MULTIPLE FILES IS NEEDED**

### Package Structure
- **`reddit/` Package** (`reddit.go`): Core Reddit client implementation
  - `Reddit` struct: Main client with dependency-injected interfaces
  - `Config` struct: Client configuration including auth credentials and customization
  - Public API methods: `GetHot`, `GetNew`, `GetComments`, `GetMoreComments`, `GetCommentsMultiple`
  - Interface abstractions: `TokenProvider`, `HTTPClient`, `Validator`, `Parser`

- **`reddit/internal/` Package**: Internal implementation details (not exposed in public API)
  - `auth.go`: OAuth2 authentication with token caching using atomic pointers
  - `http.go`: HTTP client with rate limiting, retry logic, buffer pooling, structured logging
  - `parse.go`: Response parsing and Thing/Listing extraction helpers
  - `validator.go`: Input validation for subreddit names, post IDs, pagination params
  - `clock.go`: Time abstraction (Clock interface) enabling time-dependent testing without delays
  - `testutil/`: Test utilities including mock servers, builders, and assertions

- **`pkg/types/` Package**: Public API types
  - Reddit data structures (`Thing`, `Link`, `Comment`, `Subreddit`, `Post`, `AccountData`)
  - Request/Response types (`PostsRequest`, `CommentsRequest`, `MoreCommentsRequest`, etc.)
  - Custom unmarshalers for handling Reddit's mixed-type fields (UnmarshalJSON implementations)

- **`reddit/errors.go`**: Public error types (exported from `graw` package)
  - `ConfigError`, `ValidationError`, `AuthError`, `APIError`, `RateLimitError`, `NetworkError`, `ParseError`
  - All errors that wrap underlying errors implement `Unwrap()` for error chain inspection
  - Internal packages (`auth`, `client`, `parse`, `validator`) have their own error types that get translated to public types via translation layer in `reddit.go`

- **`pkg/validation/` Package**: Validation utilities
  - Input sanitization and format validation
  - Security-focused validators to prevent injection attacks

### Key Design Patterns

1. **Dependency Injection**:
   - Core client uses interfaces (`HTTPClient`, `TokenProvider`, `Validator`, `Parser`)
   - Enables easy mocking and testing without network calls
   - Clock abstraction (`Clock` interface) allows time-based tests to run instantly

2. **Authentication Flow**:
   - Uses OAuth2 password grant for user auth, client credentials for app-only auth
   - Token cached with atomic pointer for lock-free reads, mutex for refresh coordination
   - Automatic token refresh with tiered expiry thresholds (80% for long-lived, 50% for medium, 90% for short-lived)
   - 401 error detection triggers token invalidation and automatic retry

3. **HTTP Client Architecture**:
   - Rate limiting with `golang.org/x/time/rate` limiter
   - Proactive throttling when approaching Reddit's rate limits
   - Buffer pooling (`sync.Pool`) for response bodies to reduce allocations
   - Maximum response size limits (10MB) to prevent DoS
   - Structured logging with slog, configurable debug payload capture
   - Reddit rate limit headers (`X-Ratelimit-Remaining`, `X-Ratelimit-Reset`, `Retry-After`) automatically respected

4. **Error Handling**:
   - Typed errors with context (URLs, status codes, operations)
   - All errors implement `Unwrap()` for error chain inspection
   - Request errors preserve underlying API errors without wrapping
   - Use `errors.As()` to extract specific error types for handling

5. **Response Parsing**:
   - Reddit returns nested `Thing` objects with `kind` and `data` fields
   - Internal parse helpers extract typed data from raw JSON
   - Supports Reddit's listing structure for pagination
   - Handles both array responses `[post, comments]` and single object responses

6. **Pagination**:
   - Uses Reddit "fullnames" (e.g., "t3_abc123" for posts, "t1_xyz789" for comments)
   - `Pagination` struct in requests with `Limit`, `After`, `Before` fields
   - Response includes `AfterFullname`/`BeforeFullname` for next/prev page
   - Maximum limit is 100 per Reddit's API constraints

7. **Concurrency Patterns**:
   - `GetCommentsMultiple` uses worker pool pattern (max 10 concurrent) to prevent resource exhaustion
   - Semaphore-based concurrency control with context cancellation support
   - Panic recovery in goroutines to prevent deadlocks

## Testing Strategy

- **Unit tests**: `reddit/*_test.go` and `internal/*_test.go` cover all core logic
- **Test organization**: Tests are grouped by concern (auth lifecycle, rate limiting, concurrency, parsing, etc.)
- **Mocking**: Mock HTTP clients and mock clock enable deterministic testing without network calls or time delays
- **Benchmarks**: `internal/*_bench_test.go` measure performance of hot paths (HTTP operations, buffer pooling)
- **Test utilities**: `internal/testutil/` provides builders, assertions, and mock servers for complex test scenarios
- **CI Pipeline**: GitHub Actions runs `go vet`, tests with `-race` and `-cover` flags, and builds all examples
- **Coverage**: Aim for comprehensive coverage of error paths and edge cases

### Important Testing Notes
- Use `MockClock` from `internal/clock.go` for time-dependent tests (rate limiting, token expiry)
- Mock HTTP client in tests to avoid real API calls and enable error scenario testing
- Test utilities in `internal/testutil/` provide builders for creating test data
- Race detector is enabled in CI, so all concurrent code must be race-free

## Development Workflow

1. **Before coding**: Use @agent-codebase-navigator to understand related code across multiple files
2. **Coding**: Use @agent-go-code-writer to write all code
2. **After coding**: Use @agent-go-test-runner to verify tests pass
3. **Before committing**: Always run `go vet ./...` and ensure tests pass
4. **After finishing work**: Use @agent-git-ops to commit and push changes

### Common Development Tasks

**Adding a new API endpoint:**
1. Add request/response types to `pkg/types/types.go`
2. Add validation logic to `reddit/internal/validator.go` if needed
3. Implement method in `reddit/reddit.go` following existing patterns (auth headers, error wrapping)
4. Add parsing logic to `reddit/internal/parse.go` if needed
5. Write tests using mock HTTP client
6. Update README.md with usage examples

**Modifying authentication:**
- Authentication logic is in `reddit/internal/auth.go`
- Token cache uses atomic pointer for lock-free reads
- Always use `Clock` interface for time operations (enables testing)
- Test token refresh, expiry, and 401 retry scenarios

**Modifying HTTP client behavior:**
- HTTP client is in `reddit/internal/http.go`
- Rate limiting uses `golang.org/x/time/rate` limiter
- Buffer pooling in `bodyBufferPool` reduces allocations
- Always use `Clock` interface for time operations
- Test rate limiting, retries, and error handling