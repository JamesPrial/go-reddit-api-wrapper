# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2025-10-27

### Changed
- **BREAKING**: Renamed `Client` struct to `Reddit` (66ddbbb)
  - All instances of `*graw.Client` must be replaced with `*graw.Reddit`
  - `NewClient()` function name remains the same
  - All methods unchanged, only type name changed
- Reorganized examples into `cmd/examples/` directory following Go conventions (c26a9a7)
  - Moved `cmd/example` to `cmd/examples/basic`
  - Moved `examples/monitor` to `cmd/examples/monitor`
  - Moved `examples/analyzer` to `cmd/examples/analyzer`
  - Updated all documentation and CI workflows with new paths
- Storage backend architecture improvements
  - Refactored package structure for cleaner architecture (28db635, f05fdee)
  - Moved converters to internal package (9ecc5cc)
  - Consolidated validation in posts layer (efad706)
  - Removed deprecated postToScanDest function (cae45a7)
  - Used constants for migrations path (e789b07)
- Error handling improvements
  - Exported error types from reddit package (d805e3a)
  - Replaced ConfigError with ValidationError (c7a12bb)
  - Structured error types for parsing (aa455ea, d7a3f0e)
  - ConfigError and TokenError introduction (12d2688)
  - Improved auth retry flow and error reporting (9e5026b)
- Internal package reorganization into subdirectories (de9153e)
- CI/CD improvements
  - Removed gh-pages deployment workflow (865a9a3)
  - Removed obsolete benchmark step (149de14)

### Added

#### Storage Layer (Major Feature - PR #8)
- Complete SQLite storage backend implementation (2bf30df, 90d2e4f)
  - SQLite backend with in-memory and file-based persistence
  - Hierarchical comment storage using closure table pattern
  - Full CRUD operations for posts and comments
  - Transaction support with rollback capabilities
  - Database migrations with golang-migrate
  - 93.5% test coverage (bf83f36)
- Storage abstraction layer with factory pattern (2f8341b, 6b0eb23)
  - Backend-agnostic Store interface
  - Generic factory registry with auto-detection from DSN
  - PostgreSQL backend preparation (stub implementation)
  - Custom error types: NotFoundError, ValidationError, IntegrityError, TransactionError, DatabaseError, ConflictError (987a603, ab3568a)
- CountPosts method for pagination support (6077d36)
- Embedded SQLite migrations (a68f2f0) - Removed external MigrationsPath requirement

#### Frontend Application (Major Feature - PR #12)
- Complete Svelte 5 + Vite web application (e266b6e, 5b987ec)
  - Modern Svelte 5 + Vite frontend stack
  - Responsive login screen with validation
  - Subreddit search and browsing interface (2470928)
  - Post viewing with comment threads
  - Saved posts management UI (f6fa6ad)
  - Auto-cache posts/comments to SQLite backend (f6fa6ad)
- Go backend authentication server (e266b6e)
  - JWT-based session management with in-memory storage
  - Reddit OAuth2 password grant authentication proxy
  - Rate limiting (5 req/sec) and request size limits (1MB)
  - Graceful shutdown support
  - Auto-generated JWT secrets at runtime (abb9f8f)
  - App-only authentication mode (55d1683)
  - Health check endpoint
- Svelte/Vite development agents (5c6277d)
  - Code writer agent for Svelte component development
  - Code reviewer agent for frontend changes

#### Testing & Development Infrastructure
- Comprehensive benchmark suite (a0ba1f5, f1058a2)
  - End-to-end benchmark suite for Reddit API client
  - GitHub Pages benchmark tracking (6eff626)
  - Performance measurement for hot paths
- Enhanced test utilities (c942087, 841648e, f8b0965)
  - Standardized test suite using internal/testutil package
  - Fluent builders for test data (Post, Comment, Subreddit, Account, More) (ea007b1, afaffe9)
  - Comprehensive mock server utilities
  - 10/10 validation test coverage (9350ebd)
- Mock clock system for time-based testing (d4d82f8)
  - Dependency injection for time operations
  - Eliminates delays in rate limit and token expiry tests
  - Converted all tests to use mock clock (c2fd07c, 3fd77a7, d4bfd7f, fc111c4)
- Development agents (f64adc8)
  - Codebase navigator agent
  - Go code reviewer agent
  - Go test runner agent

#### Documentation & Developer Experience
- Comprehensive internal package documentation (713ee4f)
- CLAUDE.md enhancements (50c6e7f)
  - Context Management section replacing Architecture
  - Agent usage instructions
  - Go test runner instructions (af115d5)
  - Go code writer workflow (f20137e)

#### API & Type Enhancements
- Public validation package (f9dd856)
  - Post ID validation
  - Pagination token validation
  - URL validation (c9d5cef)
  - Security-focused input sanitization (742db89)
- Enhanced type system (af8076c, b338ea7)
  - KindPrefix constants and validation
  - Votable struct for shared Score field (8f065c7)
  - upvote_ratio added to Post type (113f44b)
- Support for race detector in Go test runner instructions (3e5e5fb)

### Fixed
- Storage layer fixes
  - Single connection enforcement for in-memory SQLite (e7d3e77)
  - Timestamp helpers for eviction tests (6cad8e7)
  - Defensive nil input validation in comments storage (817954c)
- Frontend fixes
  - Removed mobile-style constraints for desktop (d2de145)
  - Removed credentials file, added to gitignore (b6b90dc)
- Test infrastructure fixes
  - Comprehensive test failure fixes in validation, auth, and error handling (25b9315)
  - Relaxed validation rules for test data compatibility (83efa9a)
  - Code review findings from bug report (b7a452d)
  - Infinite loop in TestContextCancellationDuringRecovery (845c14b)
  - Race condition in context cancellation with mock clock (fc111c4)
  - Updated error type assertions (9c62bde)
  - Removed unused test helper directories (9754c8e)
- Rate limiting fixes
  - Fixed rate limit header parsing treating X-Ratelimit-Reset as delta time instead of Unix timestamp (20fbe2b)
- Build system
  - Added *.db to .gitignore (46d2ff0)
  - Added *.bak to .gitignore (0f5d6dd)
- Documentation fixes
  - Corrected error type imports in README.md to use `pkg/errors` package
  - Added missing type imports to code examples

### Security
- Input validation hardening (742db89)
  - Comprehensive security measures in validation layer
  - Protection against injection attacks
- Credentials management (b6b90dc)
  - Removed committed credentials file
  - Added to gitignore
- Frontend security (2470928)
  - Security fixes in subreddit search
  - Request sanitization

## [0.11.2] - 2025-10-01

### Added
- ValidateLinkID to validator for GetMoreComments (c6ad01c)
- Logging and error handling tests for client configuration and request creation (d20e30a)

### Changed
- Refactored validation logic to internal package with interface (b61c3b9)
- Updated DefaultUserAgent to version 0.11.2 with author attribution (5eddf21)

## [0.11.1] - 2025-10-01

### Fixed
- Data race in TestConcurrentErrorHandling using atomic operations (9f56f47)
- Context leaks in error adversarial tests (7c3af72)

## [0.11.0] - 2025-10-01

### Added
- Comprehensive adversarial test suite and security hardening (10e88bb)
- Storage backend implementation specification (bc08137)
- Enhanced comment parsing with nested more IDs capture (94ef8f9)
- MoreChildrenIDs field to Comment struct for deferred loading (94ef8f9)
- APIError type with status codes for improved error handling (94ef8f9)
- Comprehensive error tests for API responses (9bd41ac)
- Security improvements and bug fixes (06d351f)
- Rate limit configuration support (00155ef)

### Fixed
- HTTP keepalives disabled in authentication stress test to prevent goroutine leaks (1c3fd90)
- Adversarial test authentication and rate limit configuration (00155ef)
- Critical bugs in authentication and concurrent operations (26c2e4c, c69dc6e)
- Context cancellation handling (da893e9)
- Pagination response fields for CommentsResponse (2b7a970)
- Critical bugs and improved validation (d31747a)
- Integration test API signatures and type names (5d638f1)
- Critical issues with comprehensive test coverage additions (7771053)
- Data race in GetCommentsMultiple test (ab3941b)

### Changed
- Refactored authentication form data handling (6b8525b)
- Adjusted rate limit constants (6b8525b)
- Refactored error handling patterns to reduce repetition (de44c18)
- Preserved error codes in API responses (9bd41ac)
- Performance optimizations and documentation improvements (1f13af9)
- Major improvements to codebase (bd4a6f5)

### Improved
- Test coverage across authentication, error handling, and core functionality
- Error handling with better context and typed errors
- Authentication reliability and concurrent operation safety
- Performance optimizations throughout the codebase

## [0.1.0] - Initial Release

Initial release of the Go Reddit API wrapper with OAuth2 authentication support.
