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
// // # Connection Lifecycle
//
// The client establishes its authenticated HTTP session immediately during creation.
// NewClient() performs authentication and returns a ready-to-use client.
// Authentication errors are surfaced immediately when creating the client.
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
// Reddit's API uses cursor-based pagination with "fullnames" (e.g., "t3_abc123" for posts,
// "t1_def456" for comments). Responses include AfterFullname and BeforeFullname fields
// that point to the next and previous pages respectively.
//
// Forward Pagination (most common - getting newer content):
//
//	req := &types.PostsRequest{
//		Subreddit: "golang",
//		Pagination: types.Pagination{Limit: 25},
//	}
//
//	var allPosts []*types.Post
//	for {
//		posts, err := client.GetHot(ctx, req)
//		if err != nil {
//			break
//		}
//
//		allPosts = append(allPosts, posts.Posts...)
//		fmt.Printf("Fetched %d posts (total: %d)\n", len(posts.Posts), len(allPosts))
//
//		// Check if there are more pages
//		if posts.AfterFullname == "" {
//			break // No more pages
//		}
//
//		// Set After to the AfterFullname for next page
//		req.After = posts.AfterFullname
//	}
//
// Backward Pagination (getting older content):
//
//	req := &types.PostsRequest{
//		Subreddit: "golang",
//		Pagination: types.Pagination{
//			Limit:  25,
//			Before: "t3_xyz789", // Start from this post and go backward
//		},
//	}
//
//	posts, err := client.GetNew(ctx, req)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Use BeforeFullname for next page going backward
//	if posts.BeforeFullname != "" {
//		req.Before = posts.BeforeFullname
//		// Fetch previous page...
//	}
//
// Fetching Specific Number of Items:
//
//	func fetchNPosts(client *graw.Client, ctx context.Context, subreddit string, n int) ([]*types.Post, error) {
//		var allPosts []*types.Post
//		req := &types.PostsRequest{
//			Subreddit: subreddit,
//			Pagination: types.Pagination{Limit: 100}, // Max per request
//		}
//
//		for len(allPosts) < n {
//			posts, err := client.GetHot(ctx, req)
//			if err != nil {
//				return allPosts, err
//			}
//
//			allPosts = append(allPosts, posts.Posts...)
//
//			if posts.AfterFullname == "" || len(posts.Posts) == 0 {
//				break // No more posts available
//			}
//
//			req.After = posts.AfterFullname
//		}
//
//		// Trim to exactly n posts if we got more
//		if len(allPosts) > n {
//			allPosts = allPosts[:n]
//		}
//
//		return allPosts, nil
//	}
//
// Important Notes:
//   - After and Before cannot be used together in the same request
//   - AfterFullname and BeforeFullname may both be empty on the first/last page
//   - Maximum Limit per request is 100 items
//   - Use AfterFullname for chronological traversal (newer → older)
//   - Use BeforeFullname for reverse chronological traversal (older → newer)
//
// # Rate Limiting
//
// The client automatically handles rate limiting according to Reddit's guidelines.
// It includes built-in delays and respects HTTP 429 responses from Reddit's servers.
//
// # Error Handling
//
// The library uses specific error types for different failure scenarios:
//
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "private"})
//	if err != nil {
//		switch e := err.(type) {
//		case *graw.ConfigError:
//			// Configuration or validation error
//		case *graw.AuthError:
//			// Authentication failed
//		case *graw.RequestError:
//			// HTTP request failed
//		case *graw.ParseError:
//			// Response parsing failed
//		case *graw.APIError:
//			// Reddit API returned an error
//		case *graw.StateError:
//			// Client not in correct state
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
// # Security Considerations
//
// When using custom HTTP clients, ensure proper security configuration:
//
// TLS Certificate Verification:
//   - Never disable TLS verification in production environments
//   - Always verify Reddit's SSL certificates to prevent man-in-the-middle attacks
//   - Use the default http.Client unless you have specific requirements
//
// Timeout Configuration:
//   - Always set reasonable timeouts to prevent resource exhaustion
//   - The library enforces a minimum timeout of 1 second
//   - Default timeout is 30 seconds if not specified
//   - Very long timeouts (>5 minutes) will generate warnings
//
// Proxy Configuration:
//   - Be cautious when routing traffic through proxies
//   - Ensure proxy connections use HTTPS to protect credentials
//   - Verify proxy certificate chains if using custom CAs
//
// Custom HTTP Transports:
//   - If providing a custom http.Transport, ensure it maintains security defaults
//   - Keep TLSClientConfig.InsecureSkipVerify set to false
//   - Configure appropriate TLS minimum version (TLS 1.2 or higher)
//
// Example of secure custom HTTP client:
//
//	httpClient := &http.Client{
//		Timeout: 30 * time.Second,
//		Transport: &http.Transport{
//			TLSClientConfig: &tls.Config{
//				MinVersion: tls.VersionTLS12,
//				// InsecureSkipVerify: false (default, do not change)
//			},
//		},
//	}
//
//	config := &graw.Config{
//		// ... other config ...
//		HTTPClient: httpClient,
//	}
//
// Credential Management:
//   - Never hardcode credentials in source code
//   - Use environment variables or secure configuration management
//   - Rotate credentials regularly, especially if compromised
//   - Use application-only auth when user credentials aren't needed
//
// # Best Practices
//
//   - Authentication happens during client creation; errors are surfaced immediately
//   - Use appropriate user agents that identify your app
//   - Respect Reddit's API guidelines and rate limits
//   - Handle errors gracefully, especially for private/deleted content
//   - Use batch operations (GetCommentsMultiple) when fetching multiple items
//   - Cache results when appropriate to minimize API calls
//   - Always use contexts for cancellation and timeout control
//   - Enable structured logging for production debugging
//
// # Reddit API Documentation
//
// For detailed information about Reddit's API endpoints, parameters, and responses,
// refer to Reddit's official API documentation at https://www.reddit.com/dev/api/.
package graw
