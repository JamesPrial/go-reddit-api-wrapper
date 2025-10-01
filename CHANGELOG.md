# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
