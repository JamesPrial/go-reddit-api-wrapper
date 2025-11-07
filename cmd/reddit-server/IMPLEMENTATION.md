# HTTP Server Implementation Summary

## Overview

A production-quality HTTP server that exposes Reddit API CLI commands as REST API endpoints, enabling easy integration with web applications and external services.

## Architecture

The server follows clean architecture principles with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Clients                           │
│         (JavaScript, Python, Go, cURL, etc.)               │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP/JSON
                           ▼
┌─────────────────────────────────────────────────────────────┐
│              Middleware Layer                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ - CORS (github.com/go-chi/cors)                      │  │
│  │ - Recovery (panic recovery with logging)             │  │
│  │ - Logging (structured slog)                          │  │
│  │ - Auth (header/env credentials extraction)           │  │
│  └──────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│              Handler Layer                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ - Health (GET /health)                               │  │
│  │ - User (GET /api/v1/user/me)                         │  │
│  │ - Subreddit (GET /api/v1/subreddit/{name})          │  │
│  │ - Posts (GET /api/v1/posts/hot, new)                │  │
│  │ - Comments (GET/POST comments and more-comments)    │  │
│  └──────────────────────────────────────────────────────┘  │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│              Reddit API Client                              │
│              (github.com/jamesprial/go-reddit-...)         │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│              Reddit OAuth2 API                              │
│              (oauth.reddit.com, api.reddit.com)            │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

```
cmd/reddit-server/
├── main.go                 # Server entry point, configuration loading, graceful shutdown
├── config/
│   └── config.go          # Environment variable parsing, server/reddit/CORS config
├── middleware/
│   ├── auth.go            # Extract credentials from headers/env, context storage
│   ├── logging.go         # Request/response logging with slog
│   └── recovery.go        # Panic recovery with stack traces
├── handlers/
│   ├── handlers.go        # Common utilities, error mapping, response formatting
│   ├── health.go          # GET /health endpoint (no auth)
│   ├── user.go            # GET /api/v1/user/me endpoint
│   ├── subreddit.go       # GET /api/v1/subreddit/{name} endpoint
│   └── posts.go           # Posts and comments endpoints
├── README.md              # User documentation
├── INTEGRATION.md         # Integration examples (JS, Python, Svelte, etc.)
└── IMPLEMENTATION.md      # This file
```

## Key Design Patterns

### 1. Dependency Injection
- Handler receives logger and Reddit client as dependencies
- Credentials extracted via middleware and passed through context
- Easy to mock for testing

### 2. Error Mapping
- Typed errors from Reddit client mapped to HTTP status codes
- Consistent error response format with error type and message
- Support for wrapped errors via `errors.As()`

### 3. Middleware Chain
- CORS applied in Router()
- Auth, Logging, Recovery applied globally in main.go
- Execution order: CORS → Recovery → Logging → Auth → Handler

### 4. Configuration Management
- Environment variables loaded once on startup
- Can be overridden per-request via headers
- CORS, server timeouts, Reddit credentials all configurable

### 5. Response Formatting
- Consistent Response wrapper with Data and Pagination fields
- Error responses use standard ErrorResponse structure
- JSON encoding with proper content-type headers

## API Endpoints

### Public Endpoints
- `GET /health` - Health check (no authentication required)

### Authenticated Endpoints
- `GET /api/v1/user/me` - Current user information
- `GET /api/v1/subreddit/{name}` - Subreddit information
- `GET /api/v1/posts/hot` - Hot posts (with pagination)
- `GET /api/v1/posts/new` - New posts (with pagination)
- `GET /api/v1/posts/{subreddit}/{postID}/comments` - Post comments (with pagination)
- `POST /api/v1/posts/{linkID}/more-comments` - Load more comments

## Configuration Options

### Environment Variables

**Server Configuration:**
- `SERVER_PORT` (default: 8080)
- `SERVER_READ_TIMEOUT` (default: 15s)
- `SERVER_WRITE_TIMEOUT` (default: 15s)
- `SERVER_IDLE_TIMEOUT` (default: 60s)

**Reddit API Configuration:**
- `REDDIT_CLIENT_ID` (required)
- `REDDIT_CLIENT_SECRET` (required)
- `REDDIT_USERNAME` (optional)
- `REDDIT_PASSWORD` (optional)
- `REDDIT_USER_AGENT` (default: reddit-server/1.0)

**CORS Configuration:**
- `CORS_ALLOWED_ORIGINS` (default: *)
- `CORS_ALLOWED_METHODS` (default: GET,OPTIONS)
- `CORS_ALLOWED_HEADERS` (default: Content-Type,Authorization)
- `CORS_MAX_AGE` (default: 300s)

## Authentication Flow

1. **Startup**: Load credentials from environment variables
2. **Request**: AuthFromConfig middleware stores credentials in context
3. **Per-Request Override**: Headers (X-Reddit-*) can override environment credentials
4. **Handler**: Retrieve credentials from context, create new Reddit client
5. **Execution**: Use client to call Reddit API

## Error Handling

The server implements a sophisticated error mapping system:

| Error Type | HTTP Status | Example |
|-----------|------------|---------|
| ValidationError | 400 | Invalid limit or subreddit name |
| AuthError | 401 | Missing or invalid credentials |
| Not Found | 404 | Subreddit or post not found |
| RateLimitError | 429 | Reddit rate limit exceeded |
| APIError, NetworkError, ParseError | 500 | Server-side errors |

Error response format:
```json
{
  "error": {
    "message": "descriptive error message",
    "type": "error_type",
    "code": 400
  }
}
```

## Pagination Support

All list endpoints support pagination with query parameters:
- `limit` (1-100, default: 25) - Items per page
- `after` - Token for next page
- `before` - Token for previous page

Response includes pagination metadata:
```json
{
  "data": [...],
  "pagination": {
    "after": "t3_xyz789",
    "before": ""
  }
}
```

## Logging

### Configuration
- Default: INFO level
- With `-debug` flag: DEBUG level
- Uses structured logging (slog)

### Log Format
```
time=<timestamp> level=<LEVEL> msg="<METHOD> <PATH>" status=<STATUS> duration=<MS>ms size=<BYTES>
```

### Examples
```
time=2024-01-01T12:00:00.123Z level=INFO msg="GET /api/v1/user/me" status=200 duration=150ms size=1024
time=2024-01-01T12:00:01.456Z level=ERROR msg="panic recovered" panic="..."  stack="..."
time=2024-01-01T12:00:02.789Z level=DEBUG msg="GET /health" status=200 duration=1ms size=64
```

## Performance Characteristics

- **Request Handling**: ~100-150ms typical for Reddit API calls
- **Pagination**: Efficient with Reddit's fullname-based pagination
- **Rate Limiting**: Automatically respects Reddit's rate limits
- **Connection Pooling**: HTTP keep-alive enabled by default
- **Memory**: Minimal allocations using standard library features

## Security Considerations

1. **Credentials**: Never logged; handled via headers or environment
2. **CORS**: Configurable; default allows all origins (should be restricted)
3. **User Agent**: Required by Reddit; customizable per deployment
4. **Request Size**: Capped at 10MB by Reddit client
5. **Timeouts**: Configurable to prevent hanging connections
6. **Panic Recovery**: Prevents server crashes from unhandled panics

## Testing

The server can be tested with standard HTTP clients:

```bash
# Health check
curl http://localhost:8080/health

# With authentication headers
curl http://localhost:8080/api/v1/user/me \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret"

# With pagination
curl "http://localhost:8080/api/v1/posts/hot?limit=10&after=t3_xyz"

# With POST request
curl -X POST http://localhost:8080/api/v1/posts/t3_abc/more-comments \
  -H "Content-Type: application/json" \
  -d '{"comment_ids": ["id1", "id2"]}'
```

## Building and Running

### Build
```bash
cd cmd/reddit-server
go build -o reddit-server
```

### Run
```bash
export REDDIT_CLIENT_ID="your-id"
export REDDIT_CLIENT_SECRET="your-secret"
./reddit-server
```

### Run with Custom Configuration
```bash
export SERVER_PORT=3000
export CORS_ALLOWED_ORIGINS="http://localhost:5173"
./reddit-server -debug
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/go-chi/chi/v5` | HTTP router and utilities |
| `github.com/go-chi/cors` | CORS middleware |
| `log/slog` | Structured logging (standard library) |
| `net/http` | HTTP server (standard library) |
| `context` | Request context (standard library) |
| `encoding/json` | JSON encoding (standard library) |

## Future Enhancements

Potential improvements for future versions:

1. **Caching Layer**: In-memory cache for frequently accessed resources
2. **WebSocket Support**: Real-time updates via WebSocket
3. **Database Persistence**: Store user data and cache in SQLite/PostgreSQL
4. **OAuth2 Server**: Allow users to authenticate via their own Reddit accounts
5. **Rate Limiting**: Server-side rate limiting per client
6. **Metrics**: Prometheus metrics for monitoring
7. **GraphQL API**: GraphQL endpoint as alternative to REST
8. **API Versioning**: Support multiple API versions
9. **Request Signing**: Request signatures for enhanced security
10. **Bulk Operations**: Batch endpoints for multiple requests

## Troubleshooting

### Build Issues
- Ensure Go 1.24+ is installed
- Run `go mod tidy` to update dependencies
- Check that all imports are available

### Runtime Issues
- Verify credentials are correct
- Check Reddit API credentials are configured as OAuth app
- Ensure REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET are set
- Review server logs for detailed error information

### Performance Issues
- Check network connectivity to Reddit API
- Verify pagination parameters are reasonable
- Monitor server resource usage
- Check Reddit rate limit status

## Code Quality

The implementation follows Go best practices:

- ✅ All code formatted with `go fmt`
- ✅ All code verified with `go vet`
- ✅ Comprehensive error handling
- ✅ Clear function documentation
- ✅ Minimal nesting and early returns
- ✅ Interface-based design
- ✅ Proper resource cleanup (defer, context)
- ✅ No global mutable state
- ✅ No panics in library code
- ✅ Proper HTTP status codes
- ✅ Structured logging throughout

## Integration Examples

Complete integration examples are provided for:

- **JavaScript/Fetch API** - Client implementation with error handling
- **TypeScript/Axios** - Type-safe client with interface definitions
- **Python** - Requests-based client library
- **Svelte** - Frontend component with reactive updates
- **Docker** - Containerized deployment
- **Docker Compose** - Multi-container orchestration

See `INTEGRATION.md` for detailed examples.

## Maintenance

### Regular Tasks
1. Update dependencies: `go get -u ./...`
2. Run tests: `go test ./...`
3. Check for security issues: `go list -m all`
4. Monitor logs for errors

### Version Updates
When updating dependencies:
1. Run `go get -u github.com/package@latest`
2. Run `go mod tidy`
3. Test thoroughly
4. Verify no breaking changes in imports

## License

Part of the go-reddit-api-wrapper project.
