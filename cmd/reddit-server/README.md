# Reddit HTTP Server

A production-quality HTTP server that exposes the Reddit API as REST API endpoints, enabling easy integration with web applications and external services.

## Features

- **RESTful API Endpoints**: Clean JSON-based endpoints for Reddit operations
- **Authentication**: Flexible authentication via environment variables or HTTP headers
- **CORS Support**: Built-in CORS middleware for frontend integration
- **Rate Limiting**: Respects Reddit's rate limits automatically
- **Structured Logging**: Comprehensive request/response logging with slog
- **Error Handling**: Typed errors mapped to appropriate HTTP status codes
- **Graceful Shutdown**: Proper cleanup on server termination
- **Pagination**: Full pagination support for post and comment endpoints

## Building

```bash
cd cmd/reddit-server
go build -o reddit-server
```

## Running

### Basic Usage

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
./reddit-server
```

### Custom Configuration

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"
export SERVER_PORT=3000
export SERVER_READ_TIMEOUT=20
export SERVER_WRITE_TIMEOUT=20
export CORS_ALLOWED_ORIGINS="http://localhost:5173,https://myapp.com"
./reddit-server -debug
```

## Environment Variables

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | HTTP server port |
| `SERVER_READ_TIMEOUT` | 15s | Request read timeout in seconds |
| `SERVER_WRITE_TIMEOUT` | 15s | Response write timeout in seconds |
| `SERVER_IDLE_TIMEOUT` | 60s | Connection idle timeout in seconds |

### Reddit API Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `REDDIT_CLIENT_ID` | **required** | OAuth2 client ID |
| `REDDIT_CLIENT_SECRET` | **required** | OAuth2 client secret |
| `REDDIT_USERNAME` | - | Username for user authentication (optional) |
| `REDDIT_PASSWORD` | - | Password for user authentication (optional) |
| `REDDIT_USER_AGENT` | reddit-server/1.0 | Custom user agent string |

### CORS Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CORS_ALLOWED_ORIGINS` | * | Comma-separated allowed origins |
| `CORS_ALLOWED_METHODS` | GET,OPTIONS | Comma-separated allowed HTTP methods |
| `CORS_ALLOWED_HEADERS` | Content-Type,Authorization | Comma-separated allowed headers |
| `CORS_MAX_AGE` | 300 | Preflight cache max age in seconds |

## Endpoints

### Health Check

```
GET /health
```

Health check endpoint that requires no authentication.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0"
}
```

### User Operations

#### Get Current User Information

```
GET /api/v1/user/me
```

Returns information about the authenticated user.

**Authentication:** Required (via headers or environment)

**Response:**
```json
{
  "data": {
    "name": "reddit_username",
    "link_karma": 1000,
    "comment_karma": 5000,
    "is_gold": false,
    "created": 1234567890
  },
  "pagination": {}
}
```

### Subreddit Operations

#### Get Subreddit Information

```
GET /api/v1/subreddit/{name}
```

Returns information about a specific subreddit.

**Path Parameters:**
- `name`: Subreddit name (e.g., "golang", "programming")

**Authentication:** Required

**Response:**
```json
{
  "data": {
    "name": "golang",
    "title": "The Go Programming Language",
    "public_description": "...",
    "subscribers": 150000,
    "created": 1234567890
  },
  "pagination": {}
}
```

### Post Operations

#### Get Hot Posts

```
GET /api/v1/posts/hot
```

Returns hot (trending) posts.

**Query Parameters:**
- `limit`: Number of posts to return (1-100, default: 25)
- `after`: Pagination token for next page
- `before`: Pagination token for previous page
- `subreddit`: Subreddit name (optional, empty = frontpage)

**Authentication:** Required

**Response:**
```json
{
  "data": [
    {
      "id": "abc123",
      "title": "Post Title",
      "author": "username",
      "score": 1000,
      "num_comments": 50,
      "created": 1234567890,
      "url": "https://...",
      "selftext": "Post content..."
    }
  ],
  "pagination": {
    "after": "t3_def456",
    "before": ""
  }
}
```

#### Get New Posts

```
GET /api/v1/posts/new
```

Returns new (recently submitted) posts.

**Query Parameters:** Same as `/posts/hot`

**Response:** Same as `/posts/hot`

### Comment Operations

#### Get Comments for a Post

```
GET /api/v1/posts/{subreddit}/{postID}/comments
```

Returns comments for a specific post.

**Path Parameters:**
- `subreddit`: Subreddit name
- `postID`: Post ID (without "t3_" prefix)

**Query Parameters:**
- `limit`: Number of comments (1-100, default: 25)
- `after`: Pagination token for next page
- `before`: Pagination token for previous page

**Authentication:** Required

**Response:**
```json
{
  "data": {
    "post": {
      "id": "abc123",
      "title": "Post Title",
      "author": "username",
      "score": 1000,
      "num_comments": 50
    },
    "comments": [
      {
        "id": "xyz789",
        "body": "Comment text",
        "author": "username",
        "score": 100,
        "created": 1234567890
      }
    ]
  },
  "pagination": {
    "after": "t1_def456",
    "before": ""
  }
}
```

#### Get More Comments

```
POST /api/v1/posts/{linkID}/more-comments
```

Loads additional comments referenced in a post's comment tree.

**Path Parameters:**
- `linkID`: Full name or ID of the post (with or without "t3_" prefix)

**Request Body:**
```json
{
  "link_id": "t3_abc123",
  "comment_ids": ["id1", "id2", "id3"]
}
```

**Authentication:** Required

**Response:**
```json
{
  "data": [
    {
      "id": "id1",
      "body": "Comment text",
      "author": "username",
      "score": 100
    }
  ],
  "pagination": {}
}
```

## Authentication

The server supports authentication via two methods:

### 1. Environment Variables

Set credentials as environment variables (loaded on startup):

```bash
export REDDIT_CLIENT_ID="your-id"
export REDDIT_CLIENT_SECRET="your-secret"
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"
export REDDIT_USER_AGENT="myapp/1.0"
```

### 2. HTTP Headers (Per-Request)

Override credentials via headers on each request:

```bash
curl -H "X-Reddit-Client-ID: your-id" \
     -H "X-Reddit-Client-Secret: your-secret" \
     -H "X-Reddit-Username: your-username" \
     -H "X-Reddit-Password: your-password" \
     http://localhost:8080/api/v1/user/me
```

**Supported Headers:**
- `X-Reddit-Client-ID`: OAuth2 client ID
- `X-Reddit-Client-Secret`: OAuth2 client secret
- `X-Reddit-Username`: Username (optional)
- `X-Reddit-Password`: Password (optional)
- `X-Reddit-User-Agent`: Custom user agent (optional)

Headers take precedence over environment variables.

## Error Handling

The server returns standard error responses with appropriate HTTP status codes:

### 400 Bad Request
```json
{
  "error": {
    "message": "limit must be between 1 and 100",
    "type": "validation_error",
    "code": 400
  }
}
```

### 401 Unauthorized
```json
{
  "error": {
    "message": "Missing authentication credentials",
    "type": "auth_error",
    "code": 401
  }
}
```

### 404 Not Found
```json
{
  "error": {
    "message": "Subreddit not found",
    "type": "not_found",
    "code": 404
  }
}
```

### 429 Too Many Requests
```json
{
  "error": {
    "message": "rate limit exceeded",
    "type": "rate_limit_error",
    "code": 429
  }
}
```

### 500 Internal Server Error
```json
{
  "error": {
    "message": "Internal server error",
    "type": "server_error",
    "code": 500
  }
}
```

## Examples

### Get User Information

```bash
curl http://localhost:8080/api/v1/user/me \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret" | jq .
```

### Get Hot Posts from /r/golang

```bash
curl "http://localhost:8080/api/v1/posts/hot?subreddit=golang&limit=10" \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret" | jq .
```

### Get Comments for a Post

```bash
curl "http://localhost:8080/api/v1/posts/golang/abc123/comments?limit=20" \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret" | jq .
```

### Load More Comments

```bash
curl -X POST http://localhost:8080/api/v1/posts/t3_abc123/more-comments \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret" \
  -H "Content-Type: application/json" \
  -d '{
    "link_id": "t3_abc123",
    "comment_ids": ["id1", "id2", "id3"]
  }' | jq .
```

### Paginated Results

```bash
# Get first page
curl "http://localhost:8080/api/v1/posts/hot?limit=25" \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret" | jq .

# Get next page using the 'after' token from previous response
curl "http://localhost:8080/api/v1/posts/hot?limit=25&after=t3_xyz789" \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret" | jq .
```

## Logging

The server logs all requests with structured logging. Run with `-debug` flag for debug-level logging:

```bash
./reddit-server -debug
```

Log output includes:
- Request method and path
- Response status code
- Request duration in milliseconds
- Response size in bytes

Example:
```
time=2024-01-01T12:00:00.123Z level=INFO msg="GET /api/v1/user/me" status=200 duration=150ms size=1024
time=2024-01-01T12:00:01.456Z level=ERROR msg="failed to fetch user info" error="authentication failed" status_code=401
```

## Development

### File Structure

```
cmd/reddit-server/
├── main.go              # Server entry point
├── config/
│   └── config.go       # Configuration management
├── middleware/
│   ├── auth.go         # Authentication middleware
│   ├── logging.go      # Request logging
│   └── recovery.go     # Panic recovery
├── handlers/
│   ├── handlers.go     # Common handler utilities
│   ├── health.go       # Health check
│   ├── user.go         # User endpoints
│   ├── subreddit.go    # Subreddit endpoints
│   └── posts.go        # Posts and comments endpoints
└── README.md           # This file
```

### Testing

To test the server locally:

```bash
# Terminal 1: Start the server
export REDDIT_CLIENT_ID="test-id"
export REDDIT_CLIENT_SECRET="test-secret"
./reddit-server -debug

# Terminal 2: Test endpoints
curl http://localhost:8080/health | jq .
```

## Performance Considerations

- **Rate Limiting**: The server respects Reddit's rate limits automatically
- **Timeouts**: Default 30-second timeout per request (configurable)
- **Connection Pool**: Uses HTTP connection pooling internally
- **Request Size Limits**: Maximum 10MB response size
- **Pagination**: Use appropriate `limit` values (25-50 recommended for best performance)

## Security

- **Credentials**: Never log credentials; use header injection for sensitive data
- **CORS**: Configure `CORS_ALLOWED_ORIGINS` for your domain
- **User Agent**: Always provide a meaningful user agent to Reddit
- **Rate Limiting**: Respect Reddit's rate limits to avoid account suspension

## Troubleshooting

### "Missing Reddit Client ID"
```bash
# Solution: Set environment variables or HTTP headers
export REDDIT_CLIENT_ID="your-id"
export REDDIT_CLIENT_SECRET="your-secret"
```

### "Authorization failed"
- Verify credentials are correct
- Check Reddit credentials are configured as OAuth app
- Ensure user credentials are valid (if using user authentication)

### "Subreddit not found"
- Verify the subreddit name is correct
- Check that the subreddit is public and accessible
- Some subreddits may require special permissions

### Slow Responses
- Check pagination `limit` parameter (smaller values = faster)
- Monitor Reddit rate limits
- Verify network connectivity
- Check server logs for errors

## License

This is part of the go-reddit-api-wrapper project.
