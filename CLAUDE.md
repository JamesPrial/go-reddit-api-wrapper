# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go wrapper for the Reddit API providing OAuth2 authentication and a clean interface for common Reddit operations. The library supports both application-only and user authentication modes.

The project consists of four main components:
1. **Reddit API Client** (`reddit/`, `pkg/types/`) - Core library for Reddit API interaction
2. **Storage Layer** (`storage/`) - Database persistence with SQLite/PostgreSQL backends
3. **HTTP API Server** (`cmd/server/`) - Standalone REST API server exposing Reddit API functionality
4. **Frontend Application** (`frontend/`) - Svelte web UI with Go backend for Reddit authentication

Always use context7 when I need code generation, setup or configuration steps, or library/API documentation. This means you should automatically use the Context7 MCP tools to resolve library id and get library docs without me having to explicitly ask.

## Context Management

Alongside the custom subagents, your Task tool can be used to delegate to subagents, avoiding bloating your context window, as well as creating a fresh one for every task.
Proactively think hard about how problems or requests can be decomposed to utilize the Task tool to manage your context effectively.

## Key Commands

### Testing
**ALWAYS use go-test-runner agent to execute tests** - this agent is optimized for Go test execution and provides focused summaries.

```bash
# Run all tests with race detection (matches CI pipeline)
go test -v -race -cover ./...

# Run tests for a specific package
go test -v ./reddit/internal
go test -v ./storage/sqlite/internal
go test -v ./storage

# Run a specific test by name
go test -v -run TestAuthenticator_GetToken ./reddit/internal/auth
go test -v -run TestSQLiteStore_SavePost ./storage/sqlite/internal

# Run tests with coverage report
go test -cover ./...
go tool cover -func=coverage.out

# Run benchmarks
go test -bench=. ./reddit/internal/client
go test -bench=. ./storage/sqlite/internal
```

### Building
```bash
# Build HTTP API server
go build -o reddit-server ./cmd/server/

# Build all example applications (if they exist)
go build -o reddit-example-basic ./cmd/examples/basic
go build -o reddit-example-monitor ./cmd/examples/monitor
go build -o reddit-example-analyzer ./cmd/examples/analyzer

# Build frontend backend server
cd frontend/server && go build -o reddit-frontend-server .

# Build with race detection
go build -race -o reddit-server ./cmd/server/
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

### Running the HTTP API Server
```bash
# Set required environment variables
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"

# Optional for user authentication:
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"

# Optional server configuration:
export PORT=8080
export RATE_LIMIT=10
export RATE_BURST=5
export CORS_ORIGIN="*"

# Run the server
go run ./cmd/server/

# Server will be available at http://localhost:8080
# API endpoints are under /api/v1/
```

### Running the Examples
```bash
# Set required environment variables
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
# Optional for user auth:
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"

# Run examples (note: these don't exist yet)
go run ./cmd/examples/basic
go run ./cmd/examples/monitor
go run ./cmd/examples/analyzer
```

### Running the Frontend Application
```bash
# Terminal 1: Start the backend server
cd frontend/server
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
go run .

# Terminal 2: Start the frontend dev server
cd frontend/web
npm install
npm run dev

# Access at http://localhost:5173
```

## Context Management

ALWAYS PROACTIVELY use the Task tool and available subagents to break down complex problems and delegate subtasks. This prevents context bloat and ensures focused execution.

### Package Structure

#### Reddit API Client (`reddit/`, `pkg/types/`)

- **`reddit/` Package** (`reddit.go`): Core Reddit client implementation
  - `Reddit` struct: Main client with dependency-injected interfaces
  - `Config` struct: Client configuration including auth credentials and customization
  - Public API methods: `GetHot`, `GetNew`, `GetComments`, `GetMoreComments`, `GetCommentsMultiple`, `Me`, `GetSubreddit`
  - Interface abstractions: `TokenProvider`, `HTTPClient`, `Validator`, `Parser`

- **`reddit/internal/` Package**: Internal implementation details (not exposed in public API)
  - `auth/`: OAuth2 authentication with token caching using atomic pointers
  - `client/`: HTTP client with rate limiting, retry logic, buffer pooling, structured logging
  - `parse/`: Response parsing and Thing/Listing extraction helpers
  - `validator/`: Input validation for subreddit names, post IDs, pagination params
  - `clock/`: Time abstraction (Clock interface) enabling time-dependent testing without delays
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

#### Storage Layer (`storage/`)

- **`storage/` Package**: Database persistence abstraction layer
  - `Store` interface: Composite interface of PostOperations, CommentOperations, and UtilityOperations
  - Factory pattern with auto-detection from DSN or explicit driver specification
  - Backends register themselves via `init()` functions using `storage.RegisterFactory()`
  - Typed errors: `NotFoundError`, `ValidationError`, `IntegrityError`, `TransactionError`, `DatabaseError`, `ConflictError`

- **`storage/internal/` Package**: Shared internal implementation
  - Generic factory registry for thread-safe backend registration
  - Database-agnostic conversion utilities for nullable SQL types
  - Test utilities with fixtures, assertions, and DB helpers

- **`storage/sqlite/` Package**: SQLite backend implementation
  - Lightweight file-based or in-memory storage
  - Comment tree reconstruction using closure table pattern
  - Migrations with `golang-migrate/migrate`
  - Comprehensive test coverage including concurrency, transactions, and edge cases

- **`storage/postgres/` Package**: PostgreSQL backend (stub implementation)
  - Enterprise-grade relational database support
  - Follows same patterns as SQLite backend

#### Frontend Application (`frontend/`)

- **`frontend/server/` Package**: Go backend server
  - JWT-based session management with in-memory storage
  - Reddit OAuth2 password grant authentication proxy
  - Rate limiting (5 req/sec), request size limits (1MB), graceful shutdown
  - API endpoints: `/api/auth/login`, `/api/auth/status`, `/api/auth/logout`, `/health`
  - Environment variables: `JWT_SECRET_KEY`, `REDDIT_CLIENT_ID`, `REDDIT_CLIENT_SECRET`

- **`frontend/web/` Package**: Svelte frontend application
  - Vite + Svelte development stack
  - Responsive login form with validation and error handling
  - Simple dashboard with Reddit karma stats
  - Vite proxy configuration for backend API requests
  - Built with `npm run build`, served with `npm run dev`

### Key Design Patterns

1. **Dependency Injection**:
   - Core client uses interfaces (`HTTPClient`, `TokenProvider`, `Validator`, `Parser`)
   - Enables easy mocking and testing without network calls
   - Clock abstraction (`Clock` interface) allows time-based tests to run instantly
   - Storage factory pattern allows pluggable backends without code changes

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
   - Storage layer has its own typed errors that backends translate to

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
   - Storage layer is thread-safe with transaction support

8. **Storage Factory Pattern**:
   - Backend-agnostic interface with pluggable implementations
   - Blank imports register backends: `import _ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"`
   - Auto-detection from DSN pattern or explicit driver specification
   - Thread-safe registry using read-write mutexes

## Testing Strategy

- **Unit tests**: `reddit/*_test.go`, `storage/*_test.go`, and `internal/*_test.go` cover all core logic
- **Test organization**: Tests are grouped by concern (auth lifecycle, rate limiting, concurrency, parsing, storage operations, etc.)
- **Mocking**: Mock HTTP clients and mock clock enable deterministic testing without network calls or time delays
- **Benchmarks**: `internal/*_bench_test.go` measure performance of hot paths (HTTP operations, buffer pooling, DB queries)
- **Test utilities**: `internal/testutil/` provides builders, assertions, and mock servers for complex test scenarios
- **CI Pipeline**: GitHub Actions runs `go vet`, tests with `-race` and `-cover` flags, builds all examples, and runs benchmarks
- **Coverage**: Aim for comprehensive coverage of error paths and edge cases

### Important Testing Notes
- Use `MockClock` from `reddit/internal/clock/clock.go` for time-dependent tests (rate limiting, token expiry)
- Mock HTTP client in tests to avoid real API calls and enable error scenario testing
- Test utilities in `reddit/internal/testutil/` provide builders for creating test data
- Storage tests use in-memory SQLite (`:memory:`) or temporary files
- Race detector is enabled in CI, so all concurrent code must be race-free

## Development Workflow

1. **Before coding**: Use Explore agent (via Task tool) to understand related code across multiple files
2. **Coding**: Use go-code-writer agent (via Task tool) to write Go code following project patterns
3. **Frontend coding**: Use svelte-vite-code-writer agent (via Task tool) for Svelte/Vite work
4. **After coding**: Use go-test-runner agent (via Task tool) to verify tests pass
5. **Code review**: Use go-code-reviewer agent (via Task tool) to review changes before committing
6. **Before committing**: Always run `go vet ./...` and ensure tests pass
7. **After finishing work**: Use git-ops agent (via Task tool) to commit and push changes

### Common Development Tasks

**Adding a new Reddit API endpoint:**
1. Add request/response types to `pkg/types/types.go`
2. Add validation logic to `reddit/internal/validator/validator.go` if needed
3. Implement method in `reddit/reddit.go` following existing patterns (auth headers, error wrapping)
4. Add parsing logic to `reddit/internal/parse/parse.go` if needed
5. Write tests using mock HTTP client in `reddit/reddit_test.go`
6. Use go-test-runner agent to verify tests pass
7. Use go-code-reviewer agent to review implementation
8. Update README.md with usage examples

**Adding storage functionality:**
1. Add methods to `Store` interface in `storage/store.go`
2. Implement in SQLite backend at `storage/sqlite/internal/sqlite.go`
3. Add queries to `storage/sqlite/internal/queries.go`
4. Add converters if needed in `storage/sqlite/internal/converters_*.go`
5. Write comprehensive tests in `storage/sqlite/internal/*_test.go`
6. Use go-test-runner agent to verify tests pass
7. Update `storage/doc.go` if interface changes

**Modifying authentication:**
- Authentication logic is in `reddit/internal/auth/auth.go`
- Token cache uses atomic pointer for lock-free reads
- Always use `Clock` interface for time operations (enables testing)
- Test token refresh, expiry, and 401 retry scenarios
- Frontend authentication proxy is in `frontend/server/handlers.go`

**Modifying HTTP client behavior:**
- HTTP client is in `reddit/internal/client/client.go`
- Rate limiting uses `golang.org/x/time/rate` limiter
- Buffer pooling in `bodyBufferPool` reduces allocations
- Always use `Clock` interface for time operations
- Test rate limiting, retries, and error handling

**Working with the frontend:**
- Use svelte-vite-code-writer agent for Svelte component work
- Backend server code in `frontend/server/` follows standard Go patterns
- Frontend API client in `frontend/web/src/api.js` handles backend communication
- Vite proxy config in `frontend/web/vite.config.js` routes `/api/*` to backend
- Use svelte-vite-code-reviewer agent to review frontend changes

**Database migrations:**
- SQLite migrations are in `storage/sqlite/migrations/`
- Use `golang-migrate/migrate` for schema versioning
- Test migrations both up and down
- Ensure idempotent operations where possible
