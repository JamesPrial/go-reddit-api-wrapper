// Package graw provides a Go wrapper for the Reddit API with OAuth2 authentication support.
// It supports both application-only authentication and user password authentication.
//
// The package provides a simple interface for common Reddit operations like fetching posts,
// comments, and subreddit information. It handles authentication, rate limiting, and
// proper request formatting automatically.
//
// Basic usage:
//
//	config := &graw.Config{
//		ClientID:     "your-client-id",
//		ClientSecret: "your-client-secret",
//		UserAgent:    "myapp/1.0",
//	}
//
//	client, err := graw.NewClient(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang"})
//	if err != nil {
//		log.Fatal(err)
//	}
package graw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

const (
	// DefaultBaseURL is the default Reddit API base URL
	DefaultBaseURL = "https://oauth.reddit.com/"
	// DefaultAuthURL is the default Reddit OAuth base URL
	DefaultAuthURL = "https://www.reddit.com/"
	// DefaultUserAgent is the default user agent string
	DefaultUserAgent = "go-reddit-api-wrapper/0.01"
	// MoreChildrenURL is the endpoint for loading more comments
	MoreChildrenURL = "api/morechildren"
	// MeURL is the endpoint for fetching the authenticated user's info
	MeURL = "api/v1/me"

	SubPrefixURL = "r/"
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
)

// Config holds the configuration for the Reddit client.
// It provides all necessary authentication credentials and optional customization settings.
//
// For application-only authentication (script apps), provide ClientID and ClientSecret.
// For user authentication, additionally provide Username and Password.
//
// Example for app-only auth:
//
//	config := &Config{
//		ClientID:     "your-client-id",
//		ClientSecret: "your-client-secret",
//		UserAgent:    "myapp/1.0 by /u/yourusername",
//	}
//
// Example for user auth:
//
//	config := &Config{
//		Username:     "your-username",
//		Password:     "your-password",
//		ClientID:     "your-client-id",
//		ClientSecret: "your-client-secret",
//		UserAgent:    "myapp/1.0 by /u/yourusername",
//	}
type Config struct {
	// Username and Password for password grant flow.
	// Required only for user authentication. Leave empty for app-only authentication.
	Username string
	Password string

	// ClientID and ClientSecret for OAuth2 authentication.
	// Required for all authentication types. Obtain these from Reddit's app preferences.
	ClientID     string
	ClientSecret string

	// UserAgent string to identify your application to Reddit.
	// Should follow format: "platform:app-name:version by /u/username"
	// Example: "web:myapp:1.0 by /u/myusername"
	UserAgent string

	// BaseURL for the Reddit API.
	// Defaults to DefaultBaseURL if not specified. Usually doesn't need to be changed.
	BaseURL string

	// AuthURL for Reddit OAuth authentication.
	// Defaults to DefaultAuthURL if not specified. Usually doesn't need to be changed.
	AuthURL string

	// HTTPClient to use for requests.
	// Defaults to a client with DefaultTimeout if not specified.
	// Customize this to set custom timeouts, proxies, or other HTTP behavior.
	HTTPClient *http.Client

	// Logger for structured diagnostics.
	// Optional. If provided, debug information will be logged during API calls.
	Logger *slog.Logger
}

// TokenProvider defines the interface for retrieving an access token.
// Implementations should handle token caching, renewal, and error handling.
// The internal authenticator implements this interface.
type TokenProvider interface {
	// GetToken returns a valid access token for making authenticated requests.
	// It should handle token refresh automatically if the token is expired.
	GetToken(ctx context.Context) (string, error)
}

// HTTPClient defines the behavior required from the internal HTTP client.
// This interface allows for easy testing and customization of HTTP behavior.
type HTTPClient interface {
	// NewRequest creates a new HTTP request with proper authentication headers.
	// The path is relative to the configured base URL.
	// Optional query parameters can be provided as url.Values.
	NewRequest(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error)

	// Do executes an HTTP request and unmarshals the response into a Reddit Thing object.
	// This is used for most Reddit API endpoints that return structured data.
	Do(req *http.Request, v *types.Thing) error

	// DoThingArray executes an HTTP request and returns either an array of Things or a single Thing.
	// This is used for the comments endpoint which can return [post, comments] or a single Listing.
	DoThingArray(req *http.Request) ([]*types.Thing, error)

	// DoMoreChildren executes an HTTP request for the morechildren endpoint.
	// Returns the Things array from the nested json.data structure.
	DoMoreChildren(req *http.Request) ([]*types.Thing, error)
}

// Client is the main Reddit API client.
// It provides methods for common Reddit operations like fetching posts, comments,
// and subreddit information. The client is ready to use immediately after creation.
//
// Example usage:
//
//	client, err := NewClient(config)
//	if err != nil {
//		return err
//	}
//
//	// The client is ready to make API calls
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang", Limit: 25})
type Client struct {
	client HTTPClient
	auth   TokenProvider
	config *Config
	parser *internal.Parser
}

// NewClient creates a new Reddit client with the provided configuration.
// It validates the configuration, authenticates with Reddit, and returns a ready-to-use client.
//
// The function will:
//   - Validate that required configuration fields are present
//   - Set default values for optional fields
//   - Create an appropriate authenticator based on the provided credentials
//   - Authenticate with Reddit and obtain an access token
//   - Initialize the internal HTTP client with authentication
//   - Return a client ready for making API calls
//
// Returns an error if:
//   - config is nil
//   - ClientID or ClientSecret are missing
//   - Authentication fails (invalid credentials, network issues, etc.)
//   - HTTP client initialization fails
//
// After successful creation, the client is immediately ready to use for API calls.
func NewClient(config *Config) (*Client, error) {
	return NewClientWithContext(context.Background(), config)
}

// NewClientWithContext creates a new Reddit client with the provided context and configuration.
// This allows cancellation of the authentication process if needed.
func NewClientWithContext(ctx context.Context, config *Config) (*Client, error) {
	if config == nil {
		return nil, &pkgerrs.ConfigError{Message: "config cannot be nil"}
	}

	// Validate required fields
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, &pkgerrs.ConfigError{Message: "ClientID and ClientSecret are required"}
	}

	// Set defaults
	if config.UserAgent == "" {
		config.UserAgent = DefaultUserAgent
	}
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if config.AuthURL == "" {
		config.AuthURL = DefaultAuthURL
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: DefaultTimeout}
	} else if config.HTTPClient.Timeout == 0 {
		// Warn about missing timeout and set a default to prevent indefinite hangs
		config.HTTPClient.Timeout = DefaultTimeout
		if config.Logger != nil {
			config.Logger.Warn("HTTPClient timeout was 0, setting to default",
				slog.Duration("timeout", DefaultTimeout))
		}
	} else if config.HTTPClient.Timeout < time.Second {
		// Validate that timeout is not unreasonably short
		return nil, &pkgerrs.ConfigError{
			Field:   "HTTPClient.Timeout",
			Message: fmt.Sprintf("timeout too short: %v (minimum 1 second)", config.HTTPClient.Timeout),
		}
	} else if config.HTTPClient.Timeout > 5*time.Minute {
		// Warn about very long timeouts
		if config.Logger != nil {
			config.Logger.Warn("HTTPClient timeout may be too long",
				slog.Duration("timeout", config.HTTPClient.Timeout))
		}
	}

	// Create authenticator
	grantType := "client_credentials" // Default to app-only auth
	if config.Username != "" && config.Password != "" {
		grantType = "password" // Use password grant if credentials provided
	}

	auth, err := internal.NewAuthenticator(
		config.HTTPClient,
		config.Username,
		config.Password,
		config.ClientID,
		config.ClientSecret,
		config.UserAgent,
		config.AuthURL,
		grantType,
		config.Logger,
	)
	if err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to create authenticator", Err: err}
	}

	// Validate that we can get a token before creating the client
	_, err = auth.GetToken(ctx)
	if err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to authenticate", Err: err}
	}

	// Create internal HTTP client
	httpClient, err := internal.NewClient(
		config.HTTPClient,
		config.BaseURL,
		config.UserAgent,
		config.Logger,
	)
	if err != nil {
		return nil, &pkgerrs.RequestError{
			Message:   "failed to create HTTP client",
			Operation: "create HTTP client",
			Err:       err,
		}
	}

	return &Client{
		client: httpClient,
		auth:   auth,
		config: config,
		parser: internal.NewParser(),
	}, nil
}

// Me returns information about the authenticated user.
// This is useful for testing authentication and getting user details.
//
// For application-only authentication, this will return basic account information.
// For user authentication, this will return detailed information about the authenticated user.
//
// Returns an error if:
//   - The API request fails
//   - The response cannot be parsed
//
// This method requires the client to have 'read' scope for the authenticated user.
func (c *Client) Me(ctx context.Context) (*types.AccountData, error) {
	req, err := c.client.NewRequest(ctx, http.MethodGet, MeURL, nil)
	if err != nil {
		return nil, &pkgerrs.RequestError{Operation: "create request", URL: MeURL, Err: err}
	}

	// Add authentication headers
	if err := c.addAuthHeaders(ctx, req); err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to add auth headers", Err: err}
	}

	var result types.Thing
	err = c.client.Do(req, &result)
	if err != nil {
		if apiErr, ok := mapAPIError(err); ok {
			return nil, apiErr
		}
		return nil, &pkgerrs.RequestError{Operation: "get user info", URL: MeURL, Err: err}
	}

	// Parse the account data
	parsed, err := c.parser.ParseThing(&result)
	if err != nil {
		return nil, &pkgerrs.ParseError{Operation: "parse user info", Err: err}
	}

	account, ok := parsed.(*types.AccountData)
	if !ok {
		return nil, &pkgerrs.ParseError{Operation: "user info response", Err: fmt.Errorf("unexpected response type")}
	}

	return account, nil
}

// GetSubreddit retrieves information about a specific subreddit.
// This includes subscriber count, description, rules, and other metadata.
//
// Parameters:
//   - name: The subreddit name without the "r/" prefix (e.g., "golang", "programming")
//
// Returns detailed subreddit information including:
//   - Subscriber count and active user count
//   - Description and public description
//   - Subreddit type and submission settings
//   - User permissions (if authenticated with user credentials)
//
// Returns an error if:
//   - The subreddit doesn't exist or is private/banned
//   - The API request fails
//   - The response cannot be parsed
//
// This method works with both application-only and user authentication.
func (c *Client) GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error) {
	path := SubPrefixURL + name + "/about"
	req, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &pkgerrs.RequestError{Operation: "create request", URL: path, Err: err}
	}

	// Add authentication headers
	if err := c.addAuthHeaders(ctx, req); err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to add auth headers", Err: err}
	}

	var result types.Thing
	err = c.client.Do(req, &result)
	if err != nil {
		if apiErr, ok := mapAPIError(err); ok {
			return nil, apiErr
		}
		return nil, &pkgerrs.RequestError{Operation: "get subreddit", URL: SubPrefixURL + name + "/about", Err: err}
	}

	// Parse the subreddit data
	parsed, err := c.parser.ParseThing(&result)
	if err != nil {
		return nil, &pkgerrs.ParseError{Operation: "parse subreddit", Err: err}
	}

	subreddit, ok := parsed.(*types.SubredditData)
	if !ok {
		return nil, &pkgerrs.ParseError{Operation: "subreddit response", Err: fmt.Errorf("unexpected response type")}
	}

	return subreddit, nil
}

// GetHot retrieves hot posts from a subreddit or the Reddit front page.
// Hot posts are determined by Reddit's algorithm based on recent activity and votes.
//
// Provide a nil request to fetch the front page with default pagination. To target a
// specific subreddit, set PostsRequest.Subreddit and adjust pagination via the embedded
// Pagination fields.
//
// Returns:
//   - PostsResponse containing the posts and pagination information
//   - Error if the request fails
//
// The returned PostsResponse includes AfterFullname and BeforeFullname fields
// that can be used in subsequent calls for pagination.
func (c *Client) GetHot(ctx context.Context, request *types.PostsRequest) (*types.PostsResponse, error) {
	return c.getPosts(ctx, request, "hot")
}

// GetNew retrieves new posts from a subreddit or the Reddit front page.
// New posts are sorted by submission time, with the most recent first.
//
// Provide a nil request to fetch the front page with default pagination. To target a
// specific subreddit, set PostsRequest.Subreddit and adjust pagination via the embedded
// Pagination fields.
//
// Returns:
//   - PostsResponse containing the posts and pagination information
//   - Error if the request fails
func (c *Client) GetNew(ctx context.Context, request *types.PostsRequest) (*types.PostsResponse, error) {
	return c.getPosts(ctx, request, "new")
}

// getPosts is the common implementation for fetching posts from different sort endpoints.
func (c *Client) getPosts(ctx context.Context, request *types.PostsRequest, sort string) (*types.PostsResponse, error) {
	subreddit := ""
	var pagination *types.Pagination
	if request != nil {
		subreddit = request.Subreddit
		pagination = &request.Pagination
	}

	path := sort
	if subreddit != "" {
		path = SubPrefixURL + subreddit + "/" + sort
	}

	// Build query parameters
	params := buildPaginationParams(pagination)

	httpReq, err := c.client.NewRequest(ctx, http.MethodGet, path, nil, params)
	if err != nil {
		return nil, &pkgerrs.RequestError{Operation: "create request", URL: path, Err: err}
	}

	// Add authentication headers
	if err := c.addAuthHeaders(ctx, httpReq); err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to add auth headers", Err: err}
	}

	var result types.Thing
	err = c.client.Do(httpReq, &result)
	if err != nil {
		if apiErr, ok := mapAPIError(err); ok {
			return nil, apiErr
		}
		return nil, &pkgerrs.RequestError{Operation: "get " + sort + " posts", URL: path, Err: err}
	}

	posts, err := c.parser.ExtractPosts(&result)
	if err != nil {
		return nil, &pkgerrs.ParseError{Operation: "parse posts", Err: err}
	}

	var after, before string
	listing, err := c.parser.ParseThing(&result)
	if err == nil {
		if listingData, ok := listing.(*types.ListingData); ok {
			after = listingData.AfterFullname
			before = listingData.BeforeFullname
		}
	}

	return &types.PostsResponse{
		Posts:          posts,
		AfterFullname:  after,
		BeforeFullname: before,
	}, nil
}

// GetComments retrieves comments for a specific post.
// This fetches both the post information and all available comments in a single request.
//
// Provide a CommentsRequest with Subreddit and PostID populated. Pagination controls from the
// embedded Pagination struct are applied to the comment listing.
//
// Returns:
//   - CommentsResponse containing the post, comments, and IDs for loading more comments
//   - Error if the request fails
//
// Reddit may truncate the comment tree if there are too many comments. The MoreIDs
// field in the response contains comment IDs that can be loaded using GetMoreComments().
//
// The comments are returned in a flat slice, but each comment contains information
// about its parent and can be organized into a tree structure if needed.
//
// Returns an error if:
//   - The client is not connected
//   - The post doesn't exist or is in a private subreddit
//   - The API request fails
func (c *Client) GetComments(ctx context.Context, request *types.CommentsRequest) (*types.CommentsResponse, error) {
	if request == nil {
		return nil, &pkgerrs.ConfigError{Message: "comments request cannot be nil"}
	}
	if request.Subreddit == "" || request.PostID == "" {
		return nil, &pkgerrs.ConfigError{Message: "subreddit and postID are required"}
	}

	path := SubPrefixURL + request.Subreddit + "/comments/" + request.PostID

	// Build query parameters
	params := buildPaginationParams(&request.Pagination)
	httpReq, err := c.client.NewRequest(ctx, http.MethodGet, path, nil, params)
	if err != nil {
		return nil, &pkgerrs.RequestError{Operation: "create request", URL: path, Err: err}
	}

	// Add authentication headers
	if err := c.addAuthHeaders(ctx, httpReq); err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to add auth headers", Err: err}
	}

	result, err := c.client.DoThingArray(httpReq)
	if err != nil {
		if apiErr, ok := mapAPIError(err); ok {
			return nil, apiErr
		}
		return nil, &pkgerrs.RequestError{Operation: "get comments", URL: path, Err: err}
	}

	// Parse the post and comments
	post, comments, moreIDs, err := c.parser.ExtractPostAndComments(result)
	if err != nil {
		return nil, &pkgerrs.ParseError{Operation: "parse comments", Err: err}
	}

	// Note: post may be nil if Reddit only returned comments without the post
	return &types.CommentsResponse{
		Post:     post,
		Comments: comments,
		MoreIDs:  moreIDs,
	}, nil
}

// GetCommentsMultiple loads comments for multiple posts in parallel.
// This is more efficient than calling GetComments multiple times sequentially,
// especially when you need to fetch comments for many posts.
//
// Parameters:
//   - requests: Slice of pointers to types.CommentsRequest describing each post to fetch
//
// Returns:
//   - Slice of CommentsResponse in the same order as the input requests
//   - Error if any of the requests fail (the first error encountered)
//
// The function launches goroutines to fetch all comments in parallel, then
// collects the results in the original order. If any request fails, the error
// is returned but successful responses are still included in the result slice.
//
// Returns an error if any individual request fails.
func (c *Client) GetCommentsMultiple(ctx context.Context, requests []*types.CommentsRequest) ([]*types.CommentsResponse, error) {
	if len(requests) == 0 {
		return []*types.CommentsResponse{}, nil
	}

	// Create channels for results
	type result struct {
		index    int
		response *types.CommentsResponse
		err      error
	}
	resultChan := make(chan result, len(requests))

	// Launch goroutines for parallel fetching
	for i, req := range requests {
		go func(index int, r *types.CommentsRequest) {
			// Check if context is already cancelled before starting
			select {
			case <-ctx.Done():
				resultChan <- result{index: index, response: nil, err: ctx.Err()}
				return
			default:
			}

			resp, err := c.GetComments(ctx, r)
			resultChan <- result{index: index, response: resp, err: err}
		}(i, req)
	}

	// Collect results
	results := make([]*types.CommentsResponse, len(requests))
	var firstError error
	collected := 0
	for collected < len(requests) {
		select {
		case res := <-resultChan:
			if res.err != nil && firstError == nil {
				firstError = res.err
			}
			results[res.index] = res.response
			collected++
		case <-ctx.Done():
			// Context cancelled, collect remaining results but set error
			if firstError == nil {
				firstError = ctx.Err()
			}
			// Drain remaining results to prevent goroutine leaks
			remaining := len(requests) - collected
			for j := 0; j < remaining; j++ {
				<-resultChan
			}
			return results, firstError
		}
	}

	if firstError != nil {
		return results, firstError
	}
	return results, nil
}

// GetMoreComments loads additional comments that were truncated from the initial response.
// This uses Reddit's /api/morechildren endpoint to fetch comments by their IDs.
//
// When Reddit returns a large comment thread, it may truncate some comments and instead
// provide their IDs in a "more" object. This method allows you to fetch those additional
// comments.
//
// Parameters:
//   - request: MoreCommentsRequest containing the link ID, comment IDs, and optional controls
//
// Returns:
//   - Slice of Comment objects for the requested IDs
//   - Error if the request fails
//
// The function automatically adds the "t3_" prefix to LinkID if not present. The returned
// comments are in Reddit's API order, not necessarily the order of the input IDs.
//
// Note: Reddit has limits on how many comment IDs can be requested at once.
// If you have many IDs, consider splitting them into multiple requests.
//
// Returns an error if:
//   - The client is not connected
//   - The post doesn't exist
//   - The comment IDs are invalid
//   - The API request fails
func (c *Client) GetMoreComments(ctx context.Context, request *types.MoreCommentsRequest) ([]*types.Comment, error) {
	if request == nil {
		return nil, &pkgerrs.ConfigError{Message: "more comments request cannot be nil"}
	}
	if request.LinkID == "" {
		return nil, &pkgerrs.ConfigError{Message: "linkID is required"}
	}
	if len(request.CommentIDs) == 0 {
		return []*types.Comment{}, nil
	}

	// Reddit's link_id format requires the type prefix (t3_)
	linkID := request.LinkID
	if !strings.HasPrefix(linkID, "t3_") {
		linkID = "t3_" + linkID
	}

	// Build form data for POST request
	formData := url.Values{}
	formData.Set("link_id", linkID)
	formData.Set("children", strings.Join(request.CommentIDs, ","))
	formData.Set("api_type", "json")

	if request.Sort != "" {
		formData.Set("sort", request.Sort)
	}
	if request.Depth > 0 {
		formData.Set("depth", fmt.Sprintf("%d", request.Depth))
	}
	if request.Limit > 0 {
		formData.Set("limit_children", fmt.Sprintf("%d", request.Limit))
	}

	// Create POST request with form data
	req, err := c.client.NewRequest(ctx, http.MethodPost, MoreChildrenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, &pkgerrs.RequestError{Operation: "create request", URL: MoreChildrenURL, Err: err}
	}

	// Add authentication headers
	if err := c.addAuthHeaders(ctx, req); err != nil {
		return nil, &pkgerrs.AuthError{Message: "failed to add auth headers", Err: err}
	}

	// Set Content-Type header for form data
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make authenticated request to morechildren endpoint
	things, err := c.client.DoMoreChildren(req)
	if err != nil {
		if apiErr, ok := mapAPIError(err); ok {
			return nil, apiErr
		}
		return nil, &pkgerrs.RequestError{Operation: "get more comments", URL: MoreChildrenURL, Err: err}
	}

	// Extract comments from the response
	var comments []*types.Comment
	for _, thing := range things {
		if thing.Kind == "t1" {
			comment, err := c.parser.ParseComment(thing)
			if err != nil {
				continue // Skip if we can't parse
			}
			comments = append(comments, comment)
		}
	}

	return comments, nil
}

// buildPaginationParams creates url.Values for pagination.
func buildPaginationParams(pagination *types.Pagination) url.Values {
	params := url.Values{}
	if pagination == nil {
		return params
	}
	if pagination.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", pagination.Limit))
	}
	if pagination.After != "" {
		params.Set("after", pagination.After)
	}
	if pagination.Before != "" {
		params.Set("before", pagination.Before)
	}
	return params
}

// addAuthHeaders adds authentication headers to a request.
// This is called internally before each API request.
func (c *Client) addAuthHeaders(ctx context.Context, req *http.Request) error {
	token, err := c.auth.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func mapAPIError(err error) (*pkgerrs.APIError, bool) {
	var apiErr *pkgerrs.APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
