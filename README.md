# Go Reddit API Wrapper

A Go wrapper for the Reddit API that provides a clean, easy-to-use interface for interacting with Reddit.

## Features

- OAuth2 authentication (both app-only and user authentication)
- Clean, typed API for common Reddit operations
- Built-in error handling and rate limiting considerations
- Support for pagination and listing options

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
    
    graw "github.com/jamesprial/go-reddit-api-wrapper"
)

func main() {
    // Create client configuration
    config := &graw.Config{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
        UserAgent:   "my-bot/1.0 by YourUsername",
    }

    // Create the client
    client, err := graw.NewClient(config)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    // Connect to Reddit (authenticate)
    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect to Reddit: %v", err)
    }

    // Get hot posts from r/golang
    posts, err := client.GetHot(ctx, "golang", &graw.ListingOptions{Limit: 10})
    if err != nil {
        log.Fatalf("Failed to get hot posts: %v", err)
    }
    
    fmt.Printf("Retrieved %s posts\n", posts.Kind)
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
}
```

### Available Methods

- `NewClient(config *Config) (*Client, error)` - Create a new Reddit client
- `Connect(ctx context.Context) error` - Authenticate with Reddit
- `Me(ctx context.Context) (*Thing, error)` - Get authenticated user info
- `GetSubreddit(ctx context.Context, name string) (*Thing, error)` - Get subreddit info
- `GetHot(ctx context.Context, subreddit string, opts *ListingOptions) (*Thing, error)` - Get hot posts
- `GetNew(ctx context.Context, subreddit string, opts *ListingOptions) (*Thing, error)` - Get new posts
- `GetComments(ctx context.Context, subreddit, postID string, opts *ListingOptions) (*Thing, error)` - Get post comments

### Listing Options

```go
type ListingOptions struct {
    Limit  int    // Number of items to retrieve (max 100)
    After  string // Get items after this item ID  
    Before string // Get items before this item ID
}
```

## Environment Variables

The example application supports these environment variables:

- `REDDIT_CLIENT_ID` - Your Reddit app client ID
- `REDDIT_CLIENT_SECRET` - Your Reddit app client secret
- `REDDIT_USERNAME` - Your Reddit username (optional)
- `REDDIT_PASSWORD` - Your Reddit password (optional)

## Running the Example

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
# Optional for user authentication:
export REDDIT_USERNAME="your-username"  
export REDDIT_PASSWORD="your-password"

go run example/main.go
```

## Error Handling

The library provides structured error handling through `ClientError` and `AuthError` types:

```go
if err != nil {
    if clientErr, ok := err.(*graw.ClientError); ok {
        fmt.Printf("Client error: %s\n", clientErr.Error())
    }
    // Handle other error types...
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

This project is licensed under the MIT License - see the LICENSE file for details.

## Reddit API Documentation

For more information about the Reddit API, see:
- [Reddit API Documentation](https://www.reddit.com/dev/api/)
- [OAuth2 Documentation](https://github.com/reddit-archive/reddit/wiki/OAuth2)
