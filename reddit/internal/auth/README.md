# Auth Package

Internal package providing OAuth2 authentication for Reddit API access.

## Overview

The `auth` package handles OAuth2 token acquisition, caching, and refresh for both user authentication (password grant) and application-only authentication (client credentials grant). It implements efficient lock-free token caching with automatic refresh based on tiered expiry thresholds.

## Key Features

- **Lock-Free Token Caching**: Uses `atomic.Pointer` for lock-free reads with mutex-based refresh coordination
- **Tiered Expiry Thresholds**: Proactive token refresh based on token lifetime:
  - Long-lived tokens (>60s): 80% threshold
  - Medium-lived tokens (10-60s): 50% threshold
  - Short-lived tokens (<10s): 90% threshold
- **Double-Check Pattern**: Prevents redundant token refreshes under concurrent access
- **Clock Abstraction**: `clock.Clock` interface enables time-based tests without delays
- **Structured Logging**: Configurable debug logging with `slog`
- **DoS Protection**: Maximum response body size limit (10MB)
- **OAuth2 Grant Types**: Supports both password grant (user auth) and client credentials (app-only auth)

## Architecture

### Token Caching Strategy

```
GetToken() → Check cache (lock-free)
             ↓
             Cache hit & valid? → Return token
             ↓
             Acquire mutex
             ↓
             Double-check cache
             ↓
             Fetch new token from Reddit
             ↓
             Cache with tiered expiry
             ↓
             Return token
```

### Error Types

- **`ConfigError`**: Configuration validation errors (invalid URLs, missing credentials)
- **`TokenError`**: Token acquisition/refresh errors (network, auth, parsing)

Both error types implement `Unwrap()` for error chain inspection.

## Usage

```go
authenticator, err := auth.NewAuthenticator(
    httpClient,
    username,      // Empty for client credentials
    password,      // Empty for client credentials
    clientID,
    clientSecret,
    userAgent,
    baseURL,
    grantType,     // "password" or "client_credentials"
    logger,        // Optional slog.Logger
    clock,         // Optional clock.Clock (nil for production)
)
if err != nil {
    // Handle ConfigError
}

// Get token (uses cache if valid)
token, err := authenticator.GetToken(ctx)
if err != nil {
    var tokenErr *auth.TokenError
    if errors.As(err, &tokenErr) {
        // Handle token acquisition error
    }
}

// Invalidate cache (e.g., after 401 error)
authenticator.InvalidateToken()
```

## Testing Considerations

- Use `clock.MockClock` for deterministic time-based tests
- Mock HTTP client to avoid real API calls
- Test utilities in `auth_test.go` include `mockOAuthServer` for OAuth endpoint simulation
- Tests cover:
  - Token caching and expiry
  - Concurrent access and refresh coordination
  - Error scenarios (network, auth, malformed responses)
  - DoS protection (response size limits, expiry bounds)

## Implementation Notes

- Token cache is updated atomically to ensure consistency
- Context cancellation is respected throughout
- All errors preserve underlying causes via `Unwrap()`
- Logging is optional and controlled by the provided `slog.Logger`
- Clock abstraction allows tests to run instantly without time-based delays
