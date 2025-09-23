// Package graw provides a comprehensive Go wrapper for the Reddit API with OAuth2 authentication.
//
// # Overview
//
// This package enables Go applications to interact with Reddit's API through a clean,
// type-safe interface. It supports both application-only authentication (for script apps)
// and user authentication (for accessing user-specific data).
//
// # Features
//
//   - OAuth2 authentication with automatic token refresh
//   - Type-safe API methods with proper error handling
//   - Built-in rate limiting to respect Reddit's API guidelines
//   - Structured logging support via Go's slog package
//   - Pagination support for large result sets
//   - Parallel request capabilities for bulk operations
//   - Comment tree parsing and "load more" functionality
//
// # Quick Start
//
// Basic setup requires Reddit API credentials:
//
//	config := &graw.Config{
//		ClientID:     "your-client-id",
//		ClientSecret: "your-client-secret",
//		UserAgent:    "myapp/1.0 by /u/yourusername",
//	}
//
//	client, err := graw.NewClient(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Optional: call Connect now to surface authentication errors early.
//	if err := client.Connect(ctx); err != nil {
//		log.Fatal(err)
//	}
//
// # Connection Lifecycle
//
// The client lazily establishes its authenticated HTTP session. Every public method calls
// Connect() under the hood, and a sync.Once gate ensures the initialization logic only runs
// once. You can still call Connect() manually during startup if you prefer to detect
// authentication issues before issuing requests.
//
// # Authentication Types
//
// Application-Only Authentication (script apps):
//   - Requires only ClientID and ClientSecret
//   - Good for read-only operations and public data
//   - No user-specific permissions
//
// User Authentication:
//   - Requires ClientID, ClientSecret, Username, and Password
//   - Enables access to user-specific data and actions
//   - Required for accessing private subreddits or user preferences
//
// # Common Operations
//
// Fetch hot posts from a subreddit:
//
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang", Pagination: types.Pagination{Limit: 25}})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	for _, post := range posts.Posts {
//		fmt.Printf("%s (score: %d)\n", post.Title, post.Score)
//	}
//
// Get subreddit information:
//
//	subreddit, err := client.GetSubreddit(ctx, "golang")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("r/%s has %d subscribers\n",
//		subreddit.DisplayName, subreddit.Subscribers)
//
// Retrieve comments for a post:
//
//	comments, err := client.GetComments(ctx, &types.CommentsRequest{Subreddit: "golang", PostID: "abc123"})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Post: %s\n", comments.Post.Title)
//	for _, comment := range comments.Comments {
//		fmt.Printf("- %s: %s\n", comment.Author, comment.Body)
//	}
//
// # Pagination
//
// Reddit's API uses cursor-based pagination with "fullnames" (e.g., "t3_abc123"):
//
//	req := &types.PostsRequest{Subreddit: "golang", Pagination: types.Pagination{Limit: 25}}
//	for {
//		posts, err := client.GetHot(ctx, req)
//		if err != nil {
//			break
//		}
//
//		// Process posts...
//
//		if posts.AfterFullname == "" {
//			break // No more pages
//		}
//		req.After = posts.AfterFullname
//	}
//
// # Rate Limiting
//
// The client automatically handles rate limiting according to Reddit's guidelines.
// It includes built-in delays and respects HTTP 429 responses from Reddit's servers.
//
// # Error Handling
//
// All methods return *ClientError for consistent error handling:
//
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "nonexistent"})
//	if err != nil {
//		var clientErr *graw.ClientError
//		if errors.As(err, &clientErr) {
//			fmt.Printf("Reddit API error: %s\n", clientErr.Error())
//		}
//	}
//
// # Logging
//
// Enable debug logging by providing a logger in the config:
//
//	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
//		Level: slog.LevelDebug,
//	}))
//
//	config := &graw.Config{
//		// ... other config ...
//		Logger: logger,
//	}
//
// # Best Practices
//
//   - Call Connect() during startup if you want authentication errors surfaced eagerly
//   - Lazy connection is enabled by default; the first API call will connect automatically
//   - Use IsConnected() when you need to know whether initialization has already succeeded
//   - Use appropriate user agents that identify your app
//   - Respect Reddit's API guidelines and rate limits
//   - Handle errors gracefully, especially for private/deleted content
//   - Use batch operations (GetCommentsMultiple) when fetching multiple items
//   - Cache results when appropriate to minimize API calls
//
// # Reddit API Documentation
//
// For detailed information about Reddit's API endpoints, parameters, and responses,
// refer to Reddit's official API documentation at https://www.reddit.com/dev/api/.
package graw
