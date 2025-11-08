# Reddit API HTTP Server

A production-ready HTTP server that exposes Reddit API operations as RESTful endpoints. This server provides a simple, standardized interface for accessing Reddit's API with built-in rate limiting, authentication, error handling, and graceful shutdown capabilities.

## Features

- **RESTful API** - Clean, intuitive endpoints following REST conventions
- **Authentication** - Supports both application-only and user authentication modes
- **Rate Limiting** - Automatic rate limit handling with intelligent throttling
- **CORS Support** - Configurable cross-origin resource sharing for web applications
- **Graceful Shutdown** - Proper cleanup on SIGINT/SIGTERM with configurable timeout
- **Structured Logging** - JSON-formatted logs using Go's `slog` package
- **Request Validation** - Input sanitization and path traversal protection
- **Panic Recovery** - Middleware prevents server crashes from panics
- **Request Timeouts** - Configurable timeouts to prevent hung requests
- **Security Hardening** - Request size limits, header timeouts, and input validation

## Installation

### Build from Source

```bash
cd cmd/reddit-server
go build -o reddit-server .
```

### Build with Race Detection (for development)

```bash
go build -race -o reddit-server .
```

## Configuration

The server is configured entirely through environment variables. All variables except Reddit credentials have sensible defaults.

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `REDDIT_CLIENT_ID` | Reddit OAuth2 client ID from your app registration | `your-client-id` |
| `REDDIT_CLIENT_SECRET` | Reddit OAuth2 client secret from your app registration | `your-client-secret` |

### Optional Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `PORT` | HTTP server port (1-65535) | `8080` | `3000` |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown timeout (max 5 minutes) | `30s` | `45s`, `1m` |
| `REQUEST_TIMEOUT` | HTTP request timeout (max 5 minutes) | `30s` | `60s`, `2m` |
| `API_KEYS` | Comma-separated API keys for HTTP endpoint authentication | _(auto-generated)_ | `key1,key2,key3` |
| `REDDIT_USERNAME` | Reddit username for user authentication | _(none)_ | `your-username` |
| `REDDIT_PASSWORD` | Reddit password for user authentication | _(none)_ | `your-password` |
| `REDDIT_USER_AGENT` | Custom user agent string | `reddit-api-server/1.0 (host:hostname)` | `MyApp/1.0` |
| `ALLOWED_ORIGINS` | Comma-separated CORS allowed origins | _(none)_ | `http://localhost:5173,https://example.com` |
| `STORAGE_DSN` | SQLite database path or `:memory:` | `~/.local/share/reddit-server/reddit.db` | `/var/lib/reddit-server/reddit.db` or `:memory:` |
| `STORAGE_MAX_OPEN_CONNS` | Maximum open database connections | `10` | `25` |
| `STORAGE_MAX_IDLE_CONNS` | Maximum idle database connections | `5` | `10` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` | `debug` |
| `LOG_FORMAT` | Log output format (json, text) | `json` | `text` |
| `LOG_FILE` | Path to log file (must be absolute path; logs to stderr + file when set) | _(empty)_ | `/var/log/reddit-server/app.log` |

**Notes:**
- Duration values accept Go duration strings (e.g., `30s`, `1m`, `1m30s`)
- If `REDDIT_USERNAME` and `REDDIT_PASSWORD` are provided, the server uses user authentication
- If only client credentials are provided, the server uses application-only authentication
- CORS is disabled if `ALLOWED_ORIGINS` is not set
- Origins in `ALLOWED_ORIGINS` must start with `http://` or `https://`
- Storage DSN respects `XDG_DATA_HOME` environment variable when using default location
- Use `:memory:` for in-memory database (data lost on restart)

## Running the Server

### Basic Usage (Application-Only Authentication)

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
./reddit-server
```

### User Authentication Mode

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"
./reddit-server
```

### Custom Configuration

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export PORT=3000
export SHUTDOWN_TIMEOUT=45s
export REQUEST_TIMEOUT=60s
export ALLOWED_ORIGINS="http://localhost:5173,https://example.com"
./reddit-server
```

### Logging Configuration

The server supports flexible logging configuration through environment variables:

**Basic file logging (info level, JSON format):**

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export LOG_FILE=/var/log/reddit-server/app.log
./reddit-server
```

**Debug logging to file:**

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export LOG_LEVEL=debug
export LOG_FILE=/tmp/reddit-server-debug.log
./reddit-server
```

**Text format logging:**

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
export LOG_FORMAT=text
export LOG_LEVEL=info
./reddit-server
```

**Notes:**
- When `LOG_FILE` is set, logs are written to both stderr and the specified file
- Parent directories are created automatically if they don't exist (server will fail to start if creation fails due to permissions)
- Parent directories are created with 0700 permissions (owner read/write/execute only)
- Log files are created with 0600 permissions (owner read/write only)
- Log file path must not contain '..' (directory traversal protection)
- Log levels are case-insensitive (debug, info, warn, error)
- Text format is human-readable, JSON format is structured for log aggregation

### Verify Server is Running

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "ok",
  "service": "reddit-api-server"
}
```

## API Endpoints

All endpoints return JSON responses. Error responses follow a standard format with an `error` field containing a human-readable message.

### Health Check

Check if the server is running and responsive.

**Endpoint:** `GET /health`

**Authentication:** Not required

**Response:** `200 OK`

```json
{
  "status": "ok",
  "service": "reddit-api-server"
}
```

**Example:**

```bash
curl http://localhost:8080/health
```

---

### Get Authenticated User

Retrieve information about the currently authenticated user.

**Endpoint:** `GET /api/v1/user/me`

**Authentication:** Required (user credentials must be configured)

**Response:** `200 OK`

```json
{
  "id": "abc123",
  "name": "t2_abc123",
  "is_gold": false,
  "is_mod": true,
  "link_karma": 12345,
  "comment_karma": 67890,
  "created_utc": 1234567890.0,
  "has_verified_email": true,
  "icon_img": "https://www.redditstatic.com/avatars/avatar_default_02.png"
}
```

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| `401 Unauthorized` | Authentication required (user credentials not configured) |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server error |

**Example:**

```bash
curl http://localhost:8080/api/v1/user/me
```

---

### Get Subreddit Information

Retrieve information about a specific subreddit.

**Endpoint:** `GET /api/v1/subreddit/{name}`

**URL Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Subreddit name (e.g., "golang", "programming") |

**Response:** `200 OK`

```json
{
  "id": "2qh1i",
  "name": "t5_2qh1i",
  "display_name": "golang",
  "display_name_prefixed": "r/golang",
  "title": "The Go Programming Language",
  "public_description": "Ask questions and post articles about the Go programming language...",
  "subscribers": 250000,
  "active_user_count": 500,
  "created_utc": 1283396419.0,
  "over18": false,
  "icon_img": "https://styles.redditmedia.com/...",
  "banner_img": "https://styles.redditmedia.com/..."
}
```

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid subreddit name or missing parameter |
| `401 Unauthorized` | Authentication failed |
| `404 Not Found` | Subreddit does not exist |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server error |

**Example:**

```bash
curl http://localhost:8080/api/v1/subreddit/golang
```

---

### Get Hot Posts

Retrieve hot posts from a subreddit or the frontpage.

**Endpoint:** `GET /api/v1/posts/hot`

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `subreddit` | string | No | _(frontpage)_ | Subreddit name (omit for frontpage) |
| `limit` | integer | No | `25` | Number of posts (1-100) |
| `after` | string | No | _(none)_ | Pagination cursor for next page |
| `before` | string | No | _(none)_ | Pagination cursor for previous page |

**Response:** `200 OK`

```json
{
  "posts": [
    {
      "id": "abc123",
      "name": "t3_abc123",
      "title": "Introducing Go 1.22",
      "author": "gopher",
      "subreddit": "golang",
      "score": 1234,
      "num_comments": 56,
      "created_utc": 1234567890.0,
      "url": "https://go.dev/blog/go1.22",
      "permalink": "/r/golang/comments/abc123/introducing_go_122/",
      "is_self": false,
      "selftext": "",
      "thumbnail": "https://..."
    }
  ],
  "after": "t3_xyz789",
  "before": ""
}
```

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid query parameters |
| `401 Unauthorized` | Authentication failed |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server error |

**Examples:**

```bash
# Get hot posts from frontpage
curl http://localhost:8080/api/v1/posts/hot

# Get hot posts from a specific subreddit
curl http://localhost:8080/api/v1/posts/hot?subreddit=golang

# Get hot posts with pagination
curl http://localhost:8080/api/v1/posts/hot?subreddit=golang&limit=10

# Get next page
curl "http://localhost:8080/api/v1/posts/hot?subreddit=golang&limit=10&after=t3_xyz789"
```

---

### Get New Posts

Retrieve new posts from a subreddit or the frontpage.

**Endpoint:** `GET /api/v1/posts/new`

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `subreddit` | string | No | _(frontpage)_ | Subreddit name (omit for frontpage) |
| `limit` | integer | No | `25` | Number of posts (1-100) |
| `after` | string | No | _(none)_ | Pagination cursor for next page |
| `before` | string | No | _(none)_ | Pagination cursor for previous page |

**Response:** `200 OK`

```json
{
  "posts": [
    {
      "id": "def456",
      "name": "t3_def456",
      "title": "Help: How to use goroutines?",
      "author": "newgopher",
      "subreddit": "golang",
      "score": 5,
      "num_comments": 2,
      "created_utc": 1234567899.0,
      "url": "https://reddit.com/r/golang/comments/def456/...",
      "permalink": "/r/golang/comments/def456/help_how_to_use_goroutines/",
      "is_self": true,
      "selftext": "I'm new to Go and trying to understand concurrency...",
      "thumbnail": "self"
    }
  ],
  "after": "t3_ghi789",
  "before": ""
}
```

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid query parameters |
| `401 Unauthorized` | Authentication failed |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server error |

**Examples:**

```bash
# Get new posts from frontpage
curl http://localhost:8080/api/v1/posts/new

# Get new posts from a specific subreddit
curl http://localhost:8080/api/v1/posts/new?subreddit=golang&limit=50
```

---

### Get Post Comments

Retrieve comments for a specific post.

**Endpoint:** `GET /api/v1/posts/{subreddit}/{postID}/comments`

**URL Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `subreddit` | string | Subreddit name |
| `postID` | string | Post ID without prefix (e.g., "abc123", not "t3_abc123") |

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `limit` | integer | No | `25` | Number of comments (1-100) |
| `after` | string | No | _(none)_ | Pagination cursor for next page |
| `before` | string | No | _(none)_ | Pagination cursor for previous page |

**Response:** `200 OK`

```json
{
  "post": {
    "id": "abc123",
    "name": "t3_abc123",
    "title": "Introducing Go 1.22",
    "author": "gopher",
    "subreddit": "golang",
    "score": 1234,
    "num_comments": 56,
    "created_utc": 1234567890.0,
    "selftext": "Check out the new features..."
  },
  "comments": [
    {
      "id": "comment1",
      "name": "t1_comment1",
      "author": "commenter1",
      "body": "This is amazing! Great work on the new version.",
      "score": 42,
      "created_utc": 1234567900.0,
      "parent_id": "t3_abc123",
      "link_id": "t3_abc123",
      "depth": 0,
      "replies": [
        {
          "id": "comment2",
          "name": "t1_comment2",
          "author": "commenter2",
          "body": "I agree! The generics support is fantastic.",
          "score": 15,
          "created_utc": 1234567910.0,
          "parent_id": "t1_comment1",
          "link_id": "t3_abc123",
          "depth": 1,
          "replies": []
        }
      ]
    }
  ],
  "after": "",
  "before": ""
}
```

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid subreddit name, post ID, or query parameters |
| `401 Unauthorized` | Authentication failed |
| `404 Not Found` | Post does not exist |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server error |

**Example:**

```bash
curl http://localhost:8080/api/v1/posts/golang/abc123/comments

# With pagination
curl "http://localhost:8080/api/v1/posts/golang/abc123/comments?limit=50"
```

---

### Load More Comments

Expand previously truncated comment trees (e.g., "load more comments" links).

**Endpoint:** `POST /api/v1/posts/{linkID}/more-comments`

**URL Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `linkID` | string | Post link ID with prefix (e.g., "t3_abc123") |

**Request Body:**

```json
{
  "children": ["comment_id_1", "comment_id_2", "comment_id_3"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `children` | array of strings | Yes | Comment IDs to load (1-100 IDs, no duplicates, max 100 chars per ID) |

**Request Constraints:**
- Maximum request body size: 1MB
- Children array must contain 1-100 IDs
- Each ID must be unique (no duplicates)
- Each ID must be non-empty and max 100 characters

**Response:** `200 OK`

```json
{
  "comments": [
    {
      "id": "comment_id_1",
      "name": "t1_comment_id_1",
      "author": "user1",
      "body": "This is a previously hidden comment.",
      "score": 10,
      "created_utc": 1234567920.0,
      "parent_id": "t1_parent",
      "link_id": "t3_abc123",
      "depth": 2,
      "replies": []
    }
  ]
}
```

**Error Responses:**

| Status Code | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid link ID, empty children array, array too large (>100), duplicate IDs, or invalid JSON |
| `401 Unauthorized` | Authentication failed |
| `404 Not Found` | Post does not exist |
| `429 Too Many Requests` | Rate limit exceeded |
| `500 Internal Server Error` | Server error |

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/posts/t3_abc123/more-comments \
  -H "Content-Type: application/json" \
  -d '{"children": ["comment_id_1", "comment_id_2"]}'
```

---

### Monitor Endpoints

The monitor endpoints allow you to start, stop, and check the status of background monitoring for one or more subreddits. Only one monitor can run at a time.

#### Start Monitor

`POST /api/v1/monitor/start`

Starts monitoring one or more subreddits for new posts. Posts (and optionally comments) are automatically saved to the database at the specified interval.

**Request Headers:**
- `X-API-Key: <your-api-key>` (required)
- `Content-Type: application/json`

**Request Body:**
```json
{
  "subreddits": ["golang", "programming"],
  "interval": "30s",
  "limit": 25,
  "fetch_comments": true
}
```

**Request Fields:**
- `subreddits` (array of strings, required): List of subreddit names to monitor (1-10 subreddits)
- `interval` (string, required): Polling interval (minimum 10s, e.g., "30s", "1m", "5m")
- `limit` (integer, required): Number of posts to fetch per request (1-100)
- `fetch_comments` (boolean, required): Whether to fetch and save comments for each post

**Response (201 Created):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "running",
  "started_at": "2025-11-07T10:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request`: Invalid configuration (empty subreddits, invalid interval, limit out of range)
- `409 Conflict`: Monitor already running
- `500 Internal Server Error`: Server error

**Example:**
```bash
curl -X POST http://localhost:8080/api/v1/monitor/start \
  -H "X-API-Key: your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "subreddits": ["golang", "programming"],
    "interval": "30s",
    "limit": 25,
    "fetch_comments": true
  }'
```

---

#### Stop Monitor

`POST /api/v1/monitor/stop`

Stops the currently running monitor and returns final statistics.

**Request Headers:**
- `X-API-Key: <your-api-key>` (required)

**Response (200 OK):**
```json
{
  "status": "stopped",
  "stats": {
    "total_fetches": 120,
    "total_posts": 450,
    "total_comments": 3200,
    "last_fetch_time": "2025-11-07T11:00:00Z",
    "last_error": ""
  }
}
```

**Error Responses:**
- `404 Not Found`: No monitor currently running
- `500 Internal Server Error`: Server error

**Example:**
```bash
curl -X POST http://localhost:8080/api/v1/monitor/stop \
  -H "X-API-Key: your-api-key-here"
```

---

#### Get Monitor Status

`GET /api/v1/monitor/status`

Returns the current status of the monitor, including real-time statistics if running.

**Request Headers:**
- `X-API-Key: <your-api-key>` (required)

**Response (200 OK) - When Running:**
```json
{
  "status": "running",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "subreddits": ["golang", "programming"],
  "interval": "30s",
  "started_at": "2025-11-07T10:30:00Z",
  "stats": {
    "total_fetches": 120,
    "total_posts": 450,
    "total_comments": 3200,
    "last_fetch_time": "2025-11-07T11:00:00Z",
    "last_error": ""
  }
}
```

**Response (200 OK) - When Stopped:**
```json
{
  "status": "stopped"
}
```

**Error Responses:**
- `500 Internal Server Error`: Server error

**Example:**
```bash
curl http://localhost:8080/api/v1/monitor/status \
  -H "X-API-Key: your-api-key-here"
```

---

#### Monitor Usage Example

Here's a complete example of starting a monitor, checking its status, and stopping it:

```bash
# 1. Start monitoring r/golang and r/programming
curl -X POST http://localhost:8080/api/v1/monitor/start \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "subreddits": ["golang", "programming"],
    "interval": "30s",
    "limit": 25,
    "fetch_comments": true
  }'

# Response:
# {
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "status": "running",
#   "started_at": "2025-11-07T10:30:00Z"
# }

# 2. Check monitor status (wait a minute for some data)
curl http://localhost:8080/api/v1/monitor/status \
  -H "X-API-Key: your-api-key"

# Response:
# {
#   "status": "running",
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "subreddits": ["golang", "programming"],
#   "interval": "30s",
#   "started_at": "2025-11-07T10:30:00Z",
#   "stats": {
#     "total_fetches": 4,
#     "total_posts": 100,
#     "total_comments": 850,
#     "last_fetch_time": "2025-11-07T10:32:00Z"
#   }
# }

# 3. Stop the monitor
curl -X POST http://localhost:8080/api/v1/monitor/stop \
  -H "X-API-Key: your-api-key"

# Response:
# {
#   "status": "stopped",
#   "stats": {
#     "total_fetches": 4,
#     "total_posts": 100,
#     "total_comments": 850,
#     "last_fetch_time": "2025-11-07T10:32:00Z"
#   }
# }
```

**Notes:**
- Only one monitor can run at a time. Starting a second monitor while one is running returns a 409 Conflict error.
- The monitor runs in the background and does not block other API requests.
- Monitoring continues until explicitly stopped via the API or the server shuts down.
- All monitored posts and comments are saved to the configured storage backend.
- Use intervals of at least 30 seconds to avoid hitting Reddit's rate limits.

---

## Storage API Endpoints

The server includes built-in SQLite storage for saving Reddit posts and comments locally.

### Save Operations

**POST /api/v1/storage/posts**
- Saves a single post to local storage
- Authentication: Required (API key)
- Request body: `{"post": {...}}` (post object from Reddit API)
- Response: `{"success": true, "id": "abc123"}`

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/storage/posts \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"post": {"id": "abc123", "name": "t3_abc123", "title": "Example Post", "subreddit": "golang"}}'
```

---

**POST /api/v1/storage/posts/{id}/comments**
- Saves comments for a specific post
- Authentication: Required (API key)
- URL Parameters:
  - `id` - Post ID (e.g., "abc123")
- Request body: `{"comments": [...]}`
- Response: `{"success": true, "count": 42}`

---

**POST /api/v1/storage/bulk-save**
- Bulk downloads and saves posts from a subreddit
- Authentication: Required (API key)
- Request body:
  ```json
  {
    "subreddit": "golang",
    "sort": "hot",
    "limit": 25
  }
  ```
  - `subreddit` - Subreddit name (required)
  - `sort` - Sort order: `hot`, `new`, `top`, `controversial` (default: `hot`)
  - `limit` - Number of posts to download (1-100, default: 25)
- Response: `{"success": true, "saved": 25, "posts": [...]}`

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/storage/bulk-save \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"subreddit": "golang", "sort": "hot", "limit": 25}'
```

---

### Retrieval Operations

**GET /api/v1/storage/posts**
- Lists saved posts with filtering and pagination
- Authentication: Required (API key)
- Query Parameters:
  - `subreddit` - Filter by subreddit name
  - `author` - Filter by author username
  - `min_score` - Minimum post score (supports negative values)
  - `max_age` - Maximum age in seconds
  - `sort_by` - Sort field: `created_utc` (default), `score`, `num_comments`, `title`
  - `sort_dir` - Sort direction: `desc` (default), `asc`
  - `limit` - Number of results (default: 25, max: 100)
  - `offset` - Pagination offset (default: 0)
- Response: `{"posts": [...], "total": 123}`

**Example:**

```bash
curl -X GET "http://localhost:8080/api/v1/storage/posts?subreddit=golang&limit=10&sort_by=score" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

**GET /api/v1/storage/posts/{id}**
- Retrieves a specific saved post by ID
- Authentication: Required (API key)
- URL Parameters:
  - `id` - Post ID (e.g., "abc123")
- Response: Post object or 404 if not found

---

**GET /api/v1/storage/posts/{id}/comments**
- Retrieves saved comments for a post as a tree structure
- Authentication: Required (API key)
- URL Parameters:
  - `id` - Post ID (e.g., "abc123")
- Query Parameters:
  - `max_depth` - Maximum tree depth (0 = unlimited)
  - `sort_by` - Sort field: `score` (default), `created_utc`
  - `sort_dir` - Sort direction: `desc` (default), `asc`
- Response: `{"comments": [...], "count": 42}`

---

**GET /api/v1/storage/stats**
- Returns storage statistics
- Authentication: Required (API key)
- Response: `{"post_count": 100, "comment_count": 500, ...}`

**Example:**

```bash
curl -X GET http://localhost:8080/api/v1/storage/stats \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

### Delete Operations

**DELETE /api/v1/storage/posts/{id}**
- Deletes a saved post and its associated comments
- Authentication: Required (API key)
- URL Parameters:
  - `id` - Post ID (e.g., "abc123")
- Response: `{"success": true}`
- Returns 200 even if post doesn't exist (idempotent DELETE)

**Example:**

```bash
curl -X DELETE http://localhost:8080/api/v1/storage/posts/abc123 \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

### Storage Configuration

**Database Location:**
- Default: `~/.local/share/reddit-server/reddit.db`
- Respects `XDG_DATA_HOME` environment variable
- Use `:memory:` for in-memory database (data lost on restart)

**Connection Pool:**
- Default: 10 open connections, 5 idle connections
- Suitable for most use cases with SQLite
- Increase for high-concurrency scenarios

**Configuration Examples:**

```bash
# Use in-memory database (testing/development)
export STORAGE_DSN=:memory:

# Custom database location
export STORAGE_DSN=/var/lib/reddit-server/reddit.db

# Increase connection pool for high concurrency
export STORAGE_MAX_OPEN_CONNS=25
export STORAGE_MAX_IDLE_CONNS=10

# Run server with custom storage configuration
./reddit-server
```

---

## Static Frontend

The server includes a built-in web interface for browsing Reddit content.

### Accessing the Frontend

Once the server is running, open your browser to:

```
http://localhost:8080/app/
```

The frontend is embedded in the server binary and requires no additional setup.

### Features

- **API Key Management**: Enter and save the server's API key (displayed at startup) for authenticated requests to the backend
- **Browse Posts**: View hot or new posts from any subreddit with pagination
- **View Comments**: Display threaded comments for posts
- **Save Posts**: Individual save button on each post, or batch save selected posts
- **Bulk Download**: Input a subreddit and quantity to bulk download and save posts
- **Saved Posts View**: Browse, filter, and search saved posts with pagination
- **View Saved Comments**: Access saved comment trees for each saved post
- **Storage Stats**: Real-time display of post and comment counts in the header
- **Delete Posts**: Remove saved posts from storage
- **Responsive Design**: Works on desktop, tablet, and mobile devices
- **Dark Mode**: Automatically adapts to your system theme preference

**Storage Navigation:** Navigate between "Browse Reddit", "Saved Posts", and "Bulk Download" tabs in the UI.

**Note:** The "API key" refers to the server-side API key (auto-generated on first run or set via `API_KEYS` environment variable) that protects the HTTP API endpoints. This is different from Reddit's client credentials (`REDDIT_CLIENT_ID` and `REDDIT_CLIENT_SECRET`).

### Technology Stack

- **Alpine.js**: Lightweight reactive framework (loaded via CDN: `cdn.jsdelivr.net`)
- **Water.css**: Classless CSS baseline (loaded via CDN: `cdn.jsdelivr.net`)
- **Vanilla JavaScript**: No build step required
- **Embedded Files**: All assets bundled in the server binary

**Note:** The frontend requires internet access to load Alpine.js and Water.css from CDN. It will not function offline or in air-gapped environments.

### Browser Requirements

- Modern browsers with ES6 support (Chrome 90+, Firefox 88+, Safari 14+, Edge 90+)
- JavaScript enabled
- LocalStorage enabled (for API key persistence)

**Security Note:** API keys are stored in your browser's LocalStorage for convenience. Only use this on trusted devices, and clear your browser data when using shared computers.

### Development

Static files are located in `cmd/reddit-server/static/`:
- `index.html` - Main HTML structure
- `style.css` - Custom CSS styles
- `app.js` - API client and application logic

**Testing Changes:**
1. Modify files in `cmd/reddit-server/static/`
2. Rebuild the server: `cd cmd/reddit-server && go build -o reddit-server`
3. Restart the server: `./reddit-server`
4. Hard refresh your browser: Ctrl+Shift+R (Linux/Windows) or Cmd+Shift+R (Mac)

**Debugging:**
- Use browser DevTools (F12) to inspect network requests and console errors
- Check server logs for backend API errors
- Verify files are embedded: `go list -f '{{.EmbedFiles}}' .`

**Note:** Files are embedded at compile time using Go's `embed` package. Changes require a full rebuild to take effect.

### Troubleshooting

**Frontend not loading (404 error)**
- Ensure the server is running: `ps aux | grep reddit-server`
- Check you're accessing the correct URL: `http://localhost:8080/app/`
- Verify static files are embedded: `go list -f '{{.EmbedFiles}}' .`

**API calls failing from frontend**
- Check the browser console (F12) for error messages
- Verify the API key is correctly entered and saved
- Ensure the server is configured with valid Reddit credentials
- Check server logs for backend errors

**CORS errors**
- CORS errors typically occur when accessing from a different domain
- For local development at localhost:8080, CORS should not be an issue
- If needed, configure `ALLOWED_ORIGINS` environment variable

**JavaScript console errors**
- Ensure internet access for CDN resources (Alpine.js, Water.css)
- Hard refresh the page: Ctrl+Shift+R (Linux/Windows) or Cmd+Shift+R (Mac)
- Clear browser cache and reload

**Dark mode not working**
- Verify your system has a dark mode preference set
- Some browsers may require enabling dark mode detection in settings

## Error Handling

All error responses follow a consistent JSON format:

```json
{
  "error": "human-readable error message"
}
```

### HTTP Status Codes

| Status Code | Meaning | Common Causes |
|-------------|---------|---------------|
| `200 OK` | Success | Request completed successfully |
| `400 Bad Request` | Invalid request | Missing required parameters, invalid format, validation failure |
| `401 Unauthorized` | Authentication required | Missing credentials, expired token, invalid credentials |
| `404 Not Found` | Resource not found | Subreddit/post/user doesn't exist, invalid ID |
| `405 Method Not Allowed` | Wrong HTTP method | Using POST when GET is required, etc. |
| `429 Too Many Requests` | Rate limit exceeded | Too many requests in a short time period |
| `500 Internal Server Error` | Server error | Unexpected server-side error |
| `503 Service Unavailable` | Service temporarily unavailable | Reddit API is down or unreachable |

### Error Messages

Error messages are sanitized to prevent exposing internal implementation details:

- **Validation errors:** Return specific validation failure messages
- **Authentication errors:** Return generic "authentication required" message
- **Not found errors:** Return generic "resource not found" message
- **Rate limit errors:** Return "rate limit exceeded" message
- **Server errors:** Return generic "internal server error" message

## Security

The server implements multiple security measures:

### Input Validation
- All path parameters are validated to prevent path traversal attacks
- Query parameters are sanitized and bounded
- Request body JSON is validated against expected schemas
- Comment ID arrays are checked for duplicates, length limits, and empty strings

### Request Size Limits
- Maximum request body size: 1MB (enforced via `http.MaxBytesReader`)
- Maximum header size: 1MB
- Maximum response size from Reddit API: 10MB (configured in client)

### Timeouts
- Request timeout: 30 seconds (configurable, max 5 minutes)
- Shutdown timeout: 30 seconds (configurable, max 5 minutes)
- Read header timeout: 5 seconds
- Idle timeout: 60 seconds

### Path Traversal Protection
- All path parameters reject `..`, `./`, and `/.` sequences
- Empty strings and `.` are rejected as path parameters

### Panic Recovery
- Middleware catches and logs panics to prevent server crashes
- Stack traces are logged for debugging but not exposed to clients

### Rate Limiting
- Reddit API rate limits are automatically respected
- Rate limit headers (`X-Ratelimit-Remaining`, `X-Ratelimit-Reset`) are parsed and honored
- Proactive throttling prevents hitting rate limits

### CORS
- CORS is disabled by default (no `Access-Control-Allow-*` headers)
- When enabled, only explicitly allowed origins are permitted
- Origins must start with `http://` or `https://`
- Preflight OPTIONS requests are validated against allowed origins

### Logging
- All requests are logged with method, path, status, duration, and response size
- Error logs include context (subreddit, post ID, etc.) without exposing credentials
- Credentials in configuration logs are automatically redacted

## Development

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with race detection (matches CI)
go test -v -race -cover ./...

# Run tests for a specific package
go test -v ./handlers
go test -v ./middleware
go test -v ./config

# Run a specific test
go test -v -run TestHealth ./handlers
```

### Test Coverage

The server has comprehensive test coverage:

- **86 total tests** across handlers, middleware, and config packages
- **Handlers:** 37.8% coverage (focused on integration patterns)
- **Middleware:** 97.1% coverage (comprehensive unit tests)
- **Config:** High coverage for validation and parsing logic

### Code Quality

```bash
# Run go vet (required before committing)
go vet ./...

# Format code
go fmt ./...

# Verify dependencies
go mod verify

# Tidy dependencies
go mod tidy
```

### Building

```bash
# Standard build
go build -o reddit-server .

# Build with race detection (for testing)
go build -race -o reddit-server .

# Build for production (optimized)
go build -ldflags="-s -w" -o reddit-server .
```

## Architecture

The server follows a clean, layered architecture:

### Package Structure

```
cmd/reddit-server/
├── main.go              # Server initialization and routing
├── config/              # Configuration management
│   ├── config.go        # Environment variable parsing
│   └── config_test.go   # Configuration tests
├── handlers/            # HTTP request handlers
│   ├── handlers.go      # Common utilities (error mapping, pagination)
│   ├── health.go        # Health check endpoint
│   ├── user.go          # User endpoints
│   ├── subreddit.go     # Subreddit endpoints
│   ├── posts.go         # Post endpoints
│   └── *_test.go        # Handler tests
└── middleware/          # HTTP middleware
    ├── cors.go          # CORS headers
    ├── logging.go       # Request/response logging
    ├── recovery.go      # Panic recovery
    └── *_test.go        # Middleware tests
```

### Key Design Patterns

1. **Dependency Injection**
   - Handlers receive a shared Reddit client via constructor
   - Client is reused across requests for proper token caching and rate limiting
   - Interface-based design enables easy testing with mock clients

2. **Middleware Stack**
   - Applied in order: CORS → Logging → Recovery
   - Each middleware wraps the next handler in the chain
   - Clean separation of cross-cutting concerns

3. **Error Handling**
   - Typed errors from Reddit client are mapped to HTTP status codes
   - Error messages are sanitized before being sent to clients
   - All errors are logged with relevant context

4. **Graceful Shutdown**
   - Signal handling for SIGINT and SIGTERM
   - In-flight requests are allowed to complete within timeout
   - Server logs shutdown progress and completion

5. **Structured Logging**
   - JSON-formatted logs using Go's `slog` package
   - Request logs include method, path, status, duration, and response size
   - Error logs include context without exposing sensitive data

### Dependencies

- **Reddit API Client:** `github.com/jamesprial/go-reddit-api-wrapper/reddit`
- **Types:** `github.com/jamesprial/go-reddit-api-wrapper/pkg/types`
- **Standard Library:** Uses only Go standard library for HTTP, JSON, logging

### Configuration Flow

1. Load environment variables via `config.Load()`
2. Validate configuration via `config.Validate()`
3. Create Reddit client with credentials
4. Initialize handlers with client
5. Setup router and middleware stack
6. Start HTTP server with timeouts configured

### Request Flow

1. Request arrives at server
2. CORS middleware adds headers (if enabled)
3. Logging middleware captures request details
4. Recovery middleware wraps handler (catches panics)
5. Router dispatches to appropriate handler
6. Handler validates input and extracts parameters
7. Handler calls Reddit client method
8. Response is formatted as JSON and sent to client
9. Logging middleware captures response details

## License

This server is part of the go-reddit-api-wrapper project. See the repository root for license information.
