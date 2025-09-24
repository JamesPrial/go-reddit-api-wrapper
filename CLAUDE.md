# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go wrapper for the Reddit API providing OAuth2 authentication and a clean interface for common Reddit operations. The library supports both application-only and user authentication modes.

## Key Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./internal

# Run a specific test
go test -run TestName ./internal
```

### Building
```bash
# Build the example application
go build -o reddit-example ./cmd/example

# Build with race detection
go build -race -o reddit-example ./cmd/example
```

### Linting & Code Quality
```bash
# Run go vet for static analysis
go vet ./...

# Format all code
go fmt ./...

# Tidy dependencies
go mod tidy
```

### Running the Example
```bash
# Set required environment variables
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
# Optional for user auth:
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"

# Run the example
go run ./cmd/example
```

## Architecture

### Package Structure
- **Main Package (`/`)**: Core Reddit client implementation in `reddit.go`
  - `Client` struct: Main client with OAuth token management
  - `Config` struct: Client configuration including auth credentials and customization
  - Public API methods: `GetHot`, `GetNew`, `GetComments`, etc.

- **`internal/` Package**: Internal implementation details
  - `auth.go`: OAuth2 authentication logic, token management
  - `http.go`: HTTP client with rate limiting, request/response handling, structured logging
  - `parse.go`: Response parsing and Thing/Listing extraction helpers
  - Comprehensive test coverage including benchmarks

- **`pkg/types/` Package**: Public API types
  - Reddit data structures (`Thing`, `Link`, `Comment`, `Subreddit`)
  - Request/Response types for API operations
  - Custom unmarshalers for handling Reddit's mixed-type fields

### Key Design Patterns

1. **Authentication Flow**:
   - Uses OAuth2 password grant for user auth, client credentials for app-only auth
   - Token stored in `Client.token`, automatically refreshed as needed
   - Auth handled by internal `Authenticator` abstraction

2. **HTTP Client Architecture**:
   - Custom `HTTPClient` interface allows testing with mocks
   - Built-in exponential backoff retry logic
   - Structured logging with slog, configurable debug payload capture
   - Respects Reddit rate limit headers

3. **Error Handling**:
   - Typed errors (`ConfigError`, `AuthError`, `StateError`, `RequestError`, `ParseError`, `APIError`)
   - Each error type includes relevant context (URLs, status codes, operations)
   - Errors implement `Unwrap()` for error chain inspection

4. **Response Parsing**:
   - Reddit returns nested `Thing` objects with `kind` and `data` fields
   - Internal parse helpers extract typed data from raw JSON
   - Supports Reddit's listing structure for pagination

5. **Pagination**:
   - Uses Reddit "fullnames" (e.g., "t3_abc123" for posts)
   - `Pagination` struct in requests with `Limit`, `After`, `Before` fields
   - Response includes `AfterFullname`/`BeforeFullname` for next/prev page

## Testing Strategy

- Unit tests in `internal/*_test.go` cover auth, HTTP client, and parsing logic
- Mock HTTP client (`mockHTTPClient`) enables deterministic testing without API calls
- Benchmarks measure performance of HTTP operations with/without logging
- Example application (`cmd/example`) serves as integration test and usage demo

### git commit after finishing anything ###