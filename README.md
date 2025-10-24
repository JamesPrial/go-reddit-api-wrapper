# Go Reddit API Wrapper

A Go wrapper for the Reddit API that provides a clean, easy-to-use interface for interacting with Reddit.

## Features

- OAuth2 authentication (both app-only and user authentication)
- Clean, typed API for common Reddit operations
- Built-in error handling and rate limiting considerations
- Support for pagination and listing options
- Structured logging via Go's slog with optional response payload dumps

## Installation

```bash
go get github.com/jamesprial/go-reddit-api-wrapper
```

## Quick Start

### 1. Get Reddit API Credentials

1. Go to [Reddit App Preferences](https://www.reddit.com/prefs/apps)
2. Click "Create App" or "Create Another App"
3. Choose "script" for personal use or "web app" for web applications
4. Note your `client_id` and `client_secret`

### 2. Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "log/slog"
    "os"

    graw "github.com/jamesprial/go-reddit-api-wrapper"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func main() {
    // Create client configuration
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

    config := &graw.Config{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
        UserAgent:   "my-bot/1.0 by YourUsername",
        Logger:       logger, // Optional: capture structured logs
    }

    // Create the client (automatically authenticates)
    ctx := context.Background()
    client, err := graw.NewClient(config)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    // Get hot posts from r/golang
    posts, err := client.GetHot(ctx, &types.PostsRequest{
        Subreddit:  "golang",
        Pagination: types.Pagination{Limit: 10},
    })
    if err != nil {
        log.Fatalf("Failed to get hot posts: %v", err)
    }

    fmt.Printf("Retrieved %d posts\n", len(posts.Posts))
}
```

### 3. User Authentication

For operations that require user authentication (like posting, voting, etc.), provide username and password:

```go
config := &graw.Config{
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    Username:     "your-reddit-username",
    Password:     "your-reddit-password",
    UserAgent:   "my-bot/1.0 by YourUsername",
}
```

## API Reference

### Client Configuration

```go
type Config struct {
    Username     string        // Reddit username (optional, for user auth)
    Password     string        // Reddit password (optional, for user auth)  
    ClientID     string        // Reddit app client ID (required)
    ClientSecret string        // Reddit app client secret (required)
    UserAgent   string        // User agent string (required)
    BaseURL      string        // API base URL (optional, defaults to oauth.reddit.com)
    AuthURL      string        // Auth base URL (optional, defaults to www.reddit.com)  
    HTTPClient   *http.Client  // HTTP client (optional, uses default with 30s timeout)
    Logger       *slog.Logger  // Structured logger (optional, defaults to no logging)
    LogBodyLimit int           // Response bytes included in debug logs (optional)
}
```

### Available Methods

- `NewClient(config *Config) (*Client, error)` - Create and authenticate a new Reddit client
- `Me(ctx context.Context) (*types.AccountData, error)` - Get authenticated user info
- `GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error)` - Get subreddit info
- `GetHot(ctx context.Context, request *types.PostsRequest) (*types.PostsResponse, error)` - Get hot posts
- `GetNew(ctx context.Context, request *types.PostsRequest) (*types.PostsResponse, error)` - Get new posts
- `GetComments(ctx context.Context, request *types.CommentsRequest) (*types.CommentsResponse, error)` - Get post comments
- `GetCommentsMultiple(ctx context.Context, requests []*types.CommentsRequest) ([]*types.CommentsResponse, error)` - Batch comment loading
- `GetMoreComments(ctx context.Context, request *types.MoreCommentsRequest) ([]*types.Comment, error)` - Load truncated comments

### Request Types (pkg/types)

```go
type Pagination struct {
    Limit  int    // Number of items to retrieve (max 100)
    After  string // Get items after this item ID
    Before string // Get items before this item ID
}

type PostsRequest struct {
    Subreddit string
    Pagination
}

type CommentsRequest struct {
    Subreddit string
    PostID    string
    Pagination
}

type MoreCommentsRequest struct {
    LinkID     string
    CommentIDs []string
    Sort       string
    Depth      int
    Limit      int
}
```

## Environment Variables

The example application supports these environment variables:

- `REDDIT_CLIENT_ID` - Your Reddit app client ID
- `REDDIT_CLIENT_SECRET` - Your Reddit app client secret
- `REDDIT_USERNAME` - Your Reddit username (optional)
- `REDDIT_PASSWORD` - Your Reddit password (optional)

## Debug Logging

Provide a `*slog.Logger` in `Config.Logger` to capture structured diagnostics. Debug level enables response payload snippets and rate limit metadata:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

config := &graw.Config{
    // ... other fields ...
    Logger:       logger,
    LogBodyLimit: 8 * 1024, // optional override (defaults to 4 KiB)
}
```

The client logs request method, URL, status, duration, and rate limit headers. When debug logging is enabled, response bodies are included up to `LogBodyLimit` bytes.

## Running the Examples

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
# Optional for user authentication:
export REDDIT_USERNAME="your-username"
export REDDIT_PASSWORD="your-password"

# Run basic example
go run ./cmd/examples/basic

# Run specific examples (see cmd/examples/ directory)
go run ./cmd/examples/monitor
go run ./cmd/examples/analyzer
```

## Performance Testing & Benchmarks

This project includes comprehensive benchmark suites to measure performance characteristics:

### Unit Benchmarks

Measure internal component performance using mock HTTP servers:

```bash
# Run all unit benchmarks
go test -bench=. ./...

# Run benchmarks for specific packages
go test -bench=. ./reddit/internal/auth    # Authentication benchmarks
go test -bench=. ./reddit/internal/client  # HTTP client benchmarks
go test -bench=. ./reddit/internal/parse   # Response parsing benchmarks

# Run scenario benchmarks (realistic workflows)
go test -bench=BenchmarkScenario ./reddit
```

### End-to-End Benchmarks

**NEW**: E2E benchmarks test against Reddit's real API infrastructure to measure actual performance characteristics.

#### Prerequisites

E2E benchmarks require Reddit API credentials:

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
```

#### Running E2E Benchmarks

```bash
# Run all E2E benchmarks
go test -bench=. ./benchmarks/e2e -benchmem

# Run specific benchmark categories
go test -bench=BenchmarkE2E_Auth ./benchmarks/e2e          # Authentication flow
go test -bench=BenchmarkE2E_RateLimit ./benchmarks/e2e    # Rate limiting behavior
go test -bench=BenchmarkE2E_Pagination ./benchmarks/e2e   # Pagination patterns
go test -bench=BenchmarkE2E_GetHot ./benchmarks/e2e       # API endpoint performance

# Run with limited iterations to conserve API quota
go test -bench=. -benchtime=5x ./benchmarks/e2e -benchmem
```

#### E2E Benchmark Categories

- **API Endpoints**: Real-world performance of GetHot, GetNew, GetComments, GetSubreddit
- **Authentication**: Token acquisition, caching, concurrent access, thundering herd protection
- **Rate Limiting**: Header processing, throttling behavior, burst vs sustained requests
- **Pagination**: Cursor handling, large datasets, page size efficiency

**Important Notes**:
- E2E benchmarks make real API calls and consume your Reddit API quota
- Reddit's rate limit is 600 requests per 10 minutes for OAuth2 apps
- Some benchmarks may take several minutes to complete
- Results depend on network conditions and Reddit server load

For detailed E2E benchmark documentation, see [`benchmarks/e2e/README.md`](benchmarks/e2e/README.md).

## Real-World Usage Examples

### 1. Monitoring a Subreddit for New Posts

```go
// Monitor r/golang for new posts every 60 seconds
func monitorSubreddit(ctx context.Context, client *graw.Reddit, subreddit string) {
    var lastSeen string
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Get new posts
            resp, err := client.GetNew(ctx, &types.PostsRequest{
                Subreddit:  subreddit,
                Pagination: types.Pagination{Limit: 10, Before: lastSeen},
            })
            if err != nil {
                log.Printf("Error fetching posts: %v", err)
                continue
            }

            // Process new posts
            for _, post := range resp.Posts {
                fmt.Printf("[NEW] %s - %s\n", post.Title, post.URL)
                // Do something with the post (send notification, analyze, etc.)
            }

            // Update last seen
            if len(resp.Posts) > 0 {
                lastSeen = "t3_" + resp.Posts[0].ID
            }
        }
    }
}
```

### 2. Analyzing Comment Sentiment

```go
// Fetch comments from a post and analyze them
func analyzePostComments(ctx context.Context, client *graw.Reddit, subreddit, postID string) {
    // Get all comments for the post
    resp, err := client.GetComments(ctx, &types.CommentsRequest{
        Subreddit:  subreddit,
        PostID:     postID,
        Pagination: types.Pagination{Limit: 100},
    })
    if err != nil {
        log.Fatalf("Failed to get comments: %v", err)
    }

    // Analyze comments
    var totalScore int
    authorStats := make(map[string]int)

    for _, comment := range resp.Comments {
        totalScore += comment.Score
        authorStats[comment.Author]++
    }

    fmt.Printf("Post: %s\n", resp.Post.Title)
    fmt.Printf("Total comments: %d\n", len(resp.Comments))
    fmt.Printf("Average score: %.2f\n", float64(totalScore)/float64(len(resp.Comments)))
    fmt.Printf("Unique authors: %d\n", len(authorStats))

    // Load more comments if truncated
    if len(resp.MoreIDs) > 0 {
        moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
            LinkID:     postID,
            CommentIDs: resp.MoreIDs[:min(100, len(resp.MoreIDs))],
        })
        if err == nil {
            fmt.Printf("Loaded %d additional comments\n", len(moreComments))
        }
    }
}
```

### 3. Paginating Through Top Posts

```go
// Get top posts from multiple pages
func getTopPosts(ctx context.Context, client *graw.Reddit, subreddit string, count int) []*types.Post {
    var allPosts []*types.Post
    after := ""

    for len(allPosts) < count {
        limit := min(100, count-len(allPosts))

        resp, err := client.GetHot(ctx, &types.PostsRequest{
            Subreddit:  subreddit,
            Pagination: types.Pagination{
                Limit: limit,
                After: after,
            },
        })
        if err != nil {
            log.Printf("Error fetching posts: %v", err)
            break
        }

        allPosts = append(allPosts, resp.Posts...)

        // Check if there are more posts
        if resp.AfterFullname == "" {
            break
        }
        after = resp.AfterFullname
    }

    return allPosts
}
```

### 4. Batch Processing Multiple Subreddits

```go
// Fetch hot posts from multiple subreddits concurrently
func getMultiSubredditPosts(ctx context.Context, client *graw.Reddit, subreddits []string) map[string][]*types.Post {
    results := make(map[string][]*types.Post)
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, sub := range subreddits {
        wg.Add(1)
        go func(subreddit string) {
            defer wg.Done()

            resp, err := client.GetHot(ctx, &types.PostsRequest{
                Subreddit:  subreddit,
                Pagination: types.Pagination{Limit: 25},
            })
            if err != nil {
                log.Printf("Error fetching r/%s: %v", subreddit, err)
                return
            }

            mu.Lock()
            results[subreddit] = resp.Posts
            mu.Unlock()
        }(sub)
    }

    wg.Wait()
    return results
}
```

### 5. Building a Comment Thread Tree

```go
// Build a hierarchical comment tree
type CommentNode struct {
    Comment  *types.Comment
    Children []*CommentNode
}

func buildCommentTree(comments []*types.Comment) []*CommentNode {
    // Create lookup map
    nodeMap := make(map[string]*CommentNode)
    var roots []*CommentNode

    // First pass: create all nodes
    for _, comment := range comments {
        nodeMap[comment.ID] = &CommentNode{
            Comment:  comment,
            Children: []*CommentNode{},
        }
    }

    // Second pass: build tree structure
    for _, comment := range comments {
        node := nodeMap[comment.ID]

        if comment.ParentID == "" || comment.ParentID[:3] == "t3_" {
            // Top-level comment
            roots = append(roots, node)
        } else {
            // Child comment - extract parent ID
            parentID := comment.ParentID[3:] // Remove "t1_" prefix
            if parent, exists := nodeMap[parentID]; exists {
                parent.Children = append(parent.Children, node)
            }
        }
    }

    return roots
}

// Print comment tree
func printTree(node *CommentNode, depth int) {
    indent := strings.Repeat("  ", depth)
    fmt.Printf("%s- %s: %s (score: %d)\n",
        indent, node.Comment.Author,
        truncate(node.Comment.Body, 60),
        node.Comment.Score)

    for _, child := range node.Children {
        printTree(child, depth+1)
    }
}
```

### 6. Error Handling Best Practices

```go
import (
    "context"
    "errors"
    "log"
    "time"

    graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func robustFetch(ctx context.Context, client *graw.Reddit, subreddit string) {
    resp, err := client.GetHot(ctx, &types.PostsRequest{
        Subreddit:  subreddit,
        Pagination: types.Pagination{Limit: 25},
    })

    if err != nil {
        // Handle specific error types
        var rateLimitErr *graw.RateLimitError
        if errors.As(err, &rateLimitErr) {
            log.Printf("Rate limited. Waiting %v before retry...", rateLimitErr.WaitDuration)
            time.Sleep(rateLimitErr.WaitDuration)
            // Retry the request
            return
        }

        var authErr *graw.AuthError
        if errors.As(err, &authErr) {
            log.Printf("Authentication failed: %s", authErr.Message)
            // Maybe refresh credentials or notify user
            return
        }

        var apiErr *graw.APIError
        if errors.As(err, &apiErr) {
            log.Printf("API error (status %d): %s", apiErr.StatusCode, apiErr.Message)
            // Handle specific status codes
            return
        }

        var parseErr *graw.ParseError
        if errors.As(err, &parseErr) {
            log.Printf("Failed to parse response: %v", parseErr.Err)
            // Maybe log the raw response for debugging
            return
        }

        var validationErr *graw.ValidationError
        if errors.As(err, &validationErr) {
            log.Printf("Validation error: %s", validationErr.Message)
            return
        }

        log.Printf("Unexpected error: %v", err)
        return
    }

    // Process posts
    for _, post := range resp.Posts {
        fmt.Printf("%s (%d points)\n", post.Title, post.Score)
    }
}
```

For complete working examples, see the `cmd/examples/` directory.

## Error Handling

The library provides structured error handling through specific error types exported from the `reddit` package:

- `graw.ConfigError` - Configuration and validation errors
- `graw.ValidationError` - Input validation errors (subreddit names, post IDs, etc.)
- `graw.AuthError` - Authentication and authorization errors
- `graw.APIError` - Reddit API errors (rate limits, not found, etc.)
- `graw.RateLimitError` - Rate limiting errors with retry information
- `graw.NetworkError` - Network and transport errors
- `graw.ParseError` - Response parsing errors

```go
import (
    "errors"
    graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

if err != nil {
    var apiErr *graw.APIError
    if errors.As(err, &apiErr) {
        // Handle API error (check apiErr.StatusCode)
        fmt.Printf("API error: %s (status: %d)\n", apiErr.Message, apiErr.StatusCode)
    }

    var rateLimitErr *graw.RateLimitError
    if errors.As(err, &rateLimitErr) {
        // Handle rate limit (implement backoff using rateLimitErr.WaitDuration)
        fmt.Printf("Rate limited. Wait duration: %v\n", rateLimitErr.WaitDuration)
    }

    var validationErr *graw.ValidationError
    if errors.As(err, &validationErr) {
        // Handle validation error
        fmt.Printf("Validation error: %s\n", validationErr.Message)
    }

    var authErr *graw.AuthError
    if errors.As(err, &authErr) {
        // Handle authentication error
        fmt.Printf("Authentication error: %s\n", authErr.Message)
    }

    var networkErr *graw.NetworkError
    if errors.As(err, &networkErr) {
        // Handle network error
        fmt.Printf("Network error: %v\n", networkErr.Err)
    }

    var parseErr *graw.ParseError
    if errors.As(err, &parseErr) {
        // Handle parse error
        fmt.Printf("Parse error: %s - %v\n", parseErr.Operation, parseErr.Err)
    }
}
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Commit your changes (`git commit -am 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE.md file for details.

## Reddit API Documentation

For more information about the Reddit API, see:
- [Reddit API Documentation](https://www.reddit.com/dev/api/)
- [OAuth2 Documentation](https://github.com/reddit-archive/reddit/wiki/OAuth2)
