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
go run ./cmd/example/main.go

# Run specific examples (see examples/ directory)
go run ./examples/monitor/main.go
go run ./examples/analyzer/main.go
```

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
func robustFetch(ctx context.Context, client *graw.Reddit, subreddit string) {
    resp, err := client.GetHot(ctx, &types.PostsRequest{
        Subreddit:  subreddit,
        Pagination: types.Pagination{Limit: 25},
    })

    if err != nil {
        // Handle specific error types
        switch e := err.(type) {
        case *graw.AuthError:
            log.Printf("Authentication failed: %s", e.Message)
            // Maybe refresh credentials or notify user

        case *graw.RequestError:
            // Check if it's a rate limit issue
            if apiErr, ok := e.Err.(*internal.APIError); ok {
                if apiErr.StatusCode == 429 {
                    log.Printf("Rate limited. Waiting before retry...")
                    time.Sleep(60 * time.Second)
                    // Retry the request
                }
            }

        case *graw.ParseError:
            log.Printf("Failed to parse response: %v", e.Err)
            // Maybe log the raw response for debugging

        default:
            log.Printf("Unexpected error: %v", err)
        }
        return
    }

    // Process posts
    for _, post := range resp.Posts {
        fmt.Printf("%s (%d points)\n", post.Title, post.Score)
    }
}
```

For complete working examples, see the `examples/` directory.

## Error Handling

The library provides structured error handling through specific error types:

- `ConfigError` - Configuration and validation errors
- `AuthError` - Authentication and authorization errors
- `StateError` - Client state errors (e.g., not connected)
- `RequestError` - HTTP request creation/execution errors
- `ParseError` - JSON parsing and response structure errors
- `APIError` - Errors returned by Reddit's API

```go
if err != nil {
    switch e := err.(type) {
    case *graw.ConfigError:
        fmt.Printf("Configuration error: %s\n", e.Message)
    case *graw.AuthError:
        fmt.Printf("Authentication error: %s\n", e.Message)
    case *graw.RequestError:
        fmt.Printf("Request error for %s: %v\n", e.URL, e.Err)
    case *graw.ParseError:
        fmt.Printf("Failed to parse %s: %v\n", e.Operation, e.Err)
    case *graw.APIError:
        fmt.Printf("Reddit API error [%s]: %s\n", e.ErrorCode, e.Message)
    case *graw.StateError:
        fmt.Printf("Client state error: %s\n", e.Message)
    default:
        fmt.Printf("Unexpected error: %v\n", err)
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
