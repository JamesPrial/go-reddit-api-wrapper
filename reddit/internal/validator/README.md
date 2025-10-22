# Validator Package

The `validator` package provides comprehensive input validation for Reddit API parameters with a strong focus on security and data integrity.

## Overview

This package validates all user-supplied inputs before they're used in API requests, preventing injection attacks and ensuring compliance with Reddit's API constraints. The validator is stateless and can be reused across requests.

## Key Features

### Security-Focused Validation
- **Header Injection Prevention**: Validates user agents and URLs to prevent CRLF injection attacks
- **SQL Injection Protection**: Ensures input formats match expected patterns (base36, alphanumeric, etc.)
- **Protocol Injection Prevention**: Restricts URLs to HTTP/HTTPS schemes only
- **Path Traversal Protection**: Validates all inputs against malicious patterns

### Reddit API Compliance
- Enforces Reddit's naming rules for subreddits (length, character restrictions, underscore placement)
- Validates pagination parameters (limit ranges, token formats)
- Ensures comment ID batches don't exceed API limits (max 100)
- Normalizes link IDs with proper type prefixes (t3_ for posts)

## Validation Methods

| Method | Purpose | Key Constraints |
|--------|---------|-----------------|
| `ValidateSubredditName` | Validates subreddit name format | 3-21 chars, alphanumeric + underscore, no consecutive/leading/trailing underscores |
| `ValidatePagination` | Validates pagination parameters | Max limit 100, cannot use both after/before |
| `ValidateCommentIDs` | Validates comment ID arrays | Max 100 IDs, base36 format only |
| `ValidateUserAgent` | Validates user agent strings | Max 256 chars, no newline characters |
| `ValidateLinkID` | Validates and normalizes post IDs | Adds t3_ prefix if missing, validates format |
| `ValidatePostID` | Validates raw post IDs | Base36 format, no prefix allowed |
| `ValidatePaginationToken` | Validates fullname tokens | Must match t[1-6]_[base36] format |
| `ValidateURL` | Validates HTTP/HTTPS URLs | HTTP/HTTPS only, prevents injection |
| `ValidateConfig` | Validates client configuration | Checks credentials, timeouts, user agent |

## Error Handling

All validation errors return `*ValidationError` which includes:
- `Field`: The parameter being validated (e.g., "subreddit", "pagination.Limit")
- `Value`: The invalid value (when safe to log)
- `Reason`: Human-readable explanation of why validation failed
- `Err`: Underlying error (if applicable)

The error type implements `Unwrap()` for error chain inspection.

## Usage

```go
v := validator.NewValidator()

// Validate subreddit name
if err := v.ValidateSubredditName("golang"); err != nil {
    // Handle validation error
    var valErr *validator.ValidationError
    if errors.As(err, &valErr) {
        log.Printf("Invalid %s: %s", valErr.Field, valErr.Reason)
    }
}

// Validate and normalize link ID
linkID, err := v.ValidateLinkID("abc123")
// linkID is now "t3_abc123"

// Validate pagination
pagination := &types.Pagination{Limit: 25, After: "t3_xyz789"}
if err := v.ValidatePagination(pagination); err != nil {
    // Handle error
}
```

## Design Notes

- **Stateless**: Validator has no internal state and is safe for concurrent use
- **Reusable**: Single instance can be shared across requests
- **Defensive**: Uses regex validation from `pkg/validation` plus additional business logic checks
- **Performance**: Benchmarked for common operations (see `validator_test.go`)

## Constants

```go
const (
    minSubredditLength = 3
    maxSubredditLength = 21
    maxPaginationLimit = 100
    maxCommentIDs      = 100
    maxCommentIDLength = 100
    maxUserAgentLength = 256
    MinimumTimeout                 = 1 * time.Second
    MaximumTimeoutWarningThreshold = 5 * time.Minute
)
```

## Testing

The package includes comprehensive tests covering:
- Valid input scenarios
- Invalid input scenarios (format violations, length violations)
- Security attack vectors (SQL injection, path traversal, header injection)
- Edge cases (empty strings, boundary values)
- Performance benchmarks

Run tests with:
```bash
go test -v ./reddit/internal/validator
go test -bench=. ./reddit/internal/validator
```
