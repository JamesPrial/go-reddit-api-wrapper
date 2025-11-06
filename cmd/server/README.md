# Reddit API HTTP Server

This directory contains a standalone HTTP API server that exposes the core Reddit API wrapper functionality via REST endpoints.

## Overview

The server provides a simple JSON REST API for common Reddit operations like fetching posts, comments, and subreddit information. It handles authentication (both user and application-only), rate limiting, CORS, and request logging automatically.

## Building

```bash
go build -o reddit-server ./cmd/server/
```

## Running

Set the required environment variables and start the server:

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export PORT=8080  # optional, default is 8080

./reddit-server
```

### User Authentication

For user authentication (password grant flow), also set:

```bash
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"
```

When both username and password are provided, the server uses user authentication. Otherwise, it uses application-only authentication.

### Configuration

All configuration is done via environment variables:

- `REDDIT_CLIENT_ID` (required) - Reddit API client ID
- `REDDIT_CLIENT_SECRET` (required) - Reddit API client secret
- `REDDIT_USERNAME` (optional) - Reddit username for user auth
- `REDDIT_PASSWORD` (optional) - Reddit password for user auth
- `PORT` (default: 8080) - Server port
- `USER_AGENT` (default: "go-reddit-api-wrapper-server/1.0") - User agent string
- `RATE_LIMIT` (default: 10) - Requests per second rate limit
- `RATE_BURST` (default: 5) - Rate limit burst size
- `CORS_ORIGIN` (default: "*") - CORS origin to allow (use "*" for any origin)

## API Endpoints

### Health Check

```
GET /health
```

Returns server health status.

**Response:**
```json
{
  "status": "ok"
}
```

### Get Hot Posts

```
GET /api/v1/r/{subreddit}/hot
```

Get hot posts from a subreddit.

**Query Parameters:**
- `limit` (optional, default: 25, max: 100) - Number of posts to return
- `after` (optional) - Fullname cursor for pagination
- `before` (optional) - Fullname cursor for backward pagination

**Response:**
```json
{
  "posts": [
    {
      "id": "abc123",
      "title": "Post Title",
      "author": "username",
      "score": 1234,
      "num_comments": 56,
      "url": "https://example.com",
      "subreddit": "golang",
      "created_utc": 1234567890,
      "upvote_ratio": 0.95,
      ...
    }
  ],
  "after": "t3_next_cursor",
  "before": "t3_prev_cursor",
  "count": 25
}
```

### Get New Posts

```
GET /api/v1/r/{subreddit}/new
```

Get new posts from a subreddit.

**Query Parameters:**
- `limit` (optional, default: 25, max: 100) - Number of posts to return
- `after` (optional) - Fullname cursor for pagination
- `before` (optional) - Fullname cursor for backward pagination

**Response:** Same as `GET /api/v1/r/{subreddit}/hot`

### Get Subreddit Information

```
GET /api/v1/r/{subreddit}/about
```

Get information about a subreddit.

**Response:**
```json
{
  "name": "golang",
  "display_name": "r/golang",
  "title": "The Go Programming Language",
  "description": "...",
  "subscribers": 150000,
  "created_utc": 1234567890,
  "over_18": false,
  "public": true
}
```

### Get Comments

```
GET /api/v1/r/{subreddit}/posts/{postId}/comments
```

Get comments on a specific post. Both subreddit and postId are required path parameters.

**Query Parameters:**
- `limit` (optional, default: 25, max: 100) - Number of comments to return
- `after` (optional) - Fullname cursor for pagination
- `before` (optional) - Fullname cursor for backward pagination
- `sort` (optional, default: "best") - Comment sort order (best, top, new, controversial, old, qa)
- `depth` (optional) - Comment nesting depth

**Response:**
```json
{
  "post": {
    "id": "abc123",
    "title": "Post Title",
    ...
  },
  "comments": [
    {
      "id": "xyz789",
      "author": "username",
      "body": "Comment text",
      "score": 42,
      "created_utc": 1234567890,
      "depth": 0,
      ...
    }
  ],
  "after": "t1_next_cursor",
  "before": "t1_prev_cursor",
  "count": 25
}
```

### Get Authenticated User

```
GET /api/v1/me
```

Get information about the authenticated user (requires user authentication).

**Response:**
```json
{
  "id": "user123",
  "name": "username",
  "link_karma": 1000,
  "comment_karma": 5000,
  "created_utc": 1234567890,
  "is_moderator": false,
  "is_gold": false
}
```

## Error Handling

All errors are returned as JSON with appropriate HTTP status codes:

```json
{
  "error": "error message",
  "request_id": "unique-request-id",
  "details": "additional details (optional)"
}
```

Common HTTP status codes:

- `200 OK` - Successful request
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Authentication failed
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limited
- `500 Internal Server Error` - Server error

## Middleware

The server includes several built-in middleware:

### Request Logging
All requests are logged with:
- Request ID (for tracing)
- HTTP method and path
- Client IP address
- Response status code
- Response time

### CORS Support
Configurable CORS headers via `CORS_ORIGIN` environment variable.

### Rate Limiting
Server-side rate limiting to prevent abuse. Default: 10 requests per second with a burst of 5.

### Error Recovery
Panics are caught and returned as 500 errors to prevent server crashes.

## Example Usage

```bash
# Set credentials
export REDDIT_CLIENT_ID="your_client_id"
export REDDIT_CLIENT_SECRET="your_client_secret"

# Run server
go run ./cmd/server/

# In another terminal, test the API
curl http://localhost:8080/health

# Get hot posts from r/golang
curl http://localhost:8080/api/v1/r/golang/hot?limit=5

# Get subreddit info
curl http://localhost:8080/api/v1/r/golang/about

# Get comments on a post (replace with real subreddit and post ID)
curl http://localhost:8080/api/v1/r/golang/posts/abc123/comments?limit=10
```

## Development

The server consists of:

- `main.go` - Server bootstrap, configuration, middleware setup, graceful shutdown
- `config.go` - Configuration parsing from environment variables
- `handlers.go` - HTTP request handlers for all endpoints
- `middleware.go` - Logging, CORS, rate limiting, and recovery middleware
- `types.go` - Request/response data structures

All files follow Go best practices and include comprehensive comments.
