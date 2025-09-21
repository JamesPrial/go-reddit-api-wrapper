# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This is a Go wrapper for the Reddit API that provides OAuth2 authentication and clean interfaces for Reddit operations. The library uses structured logging via slog and includes built-in rate limiting.

## Common Commands

### Testing
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run a specific test
go test -run TestFunctionName ./...

# Run tests in verbose mode
go test -v ./...
```

### Building
```bash
# Build the library
go build ./...

# Build the example application
go build -o reddit-example ./cmd/example

# Run the example
go run cmd/example/main.go
```

### Code Quality
```bash
# Format code
go fmt ./...

# Vet code for common issues
go vet ./...

# Check for module issues
go mod verify

# Update dependencies
go mod tidy
```

## Architecture

### Package Structure
- **`reddit.go`**: Main client implementation and public API surface
  - `Client` struct: Main Reddit API client
  - `Config` struct: Client configuration including auth credentials, rate limiting
  - Public methods: `NewClient()`, `Connect()`, `GetHot()`, `GetNew()`, `GetComments()`, etc.

- **`internal/`**: Internal implementation details (not exposed to library users)
  - `auth.go`: OAuth2 authentication handling via `Authenticator` struct
    - Handles both app-only and user authentication flows
    - Token management and refresh logic
  - `http.go`: HTTP client wrapper with rate limiting via `Client` struct
    - Uses `golang.org/x/time/rate` for rate limiting
    - Handles Reddit's rate limit headers (X-Ratelimit-*)
    - Structured logging of requests/responses

- **`pkg/types/`**: Public type definitions
  - `types.go`: Reddit API object models (`Thing`, `Votable`, `Created`, `Edited`)
  - Implements custom unmarshalers for Reddit's mixed-type fields

### Key Design Patterns

1. **Token Provider Interface**: Abstraction for token retrieval, allowing different auth strategies
2. **HTTP Client Interface**: Abstraction over internal HTTP client for testing
3. **Rate Limiting**: Dual approach using local rate limiter and respecting Reddit's headers
4. **Structured Logging**: Optional slog integration with configurable body limits for debugging

### Authentication Flow
1. Client creation with credentials (`NewClient`)
2. Connection establishment (`Connect`) - obtains OAuth2 token
3. Authenticated requests using bearer token
4. Automatic token refresh on expiry

### Rate Limiting Strategy
- Default: 60 requests/minute with burst of 10
- Respects Reddit's X-Ratelimit-Reset header
- Local rate limiter prevents exceeding limits
- Configurable via `RateLimitConfig`

## Environment Variables

The example application (`cmd/example/main.go`) uses:
- `REDDIT_CLIENT_ID`: OAuth2 client ID
- `REDDIT_CLIENT_SECRET`: OAuth2 client secret
- `REDDIT_USERNAME`: Reddit username (optional, for user auth)
- `REDDIT_PASSWORD`: Reddit password (optional, for user auth)