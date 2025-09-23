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
//	if err := client.Connect(ctx); err != nil {
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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

const (
	// DefaultBaseURL is the default Reddit API base URL
	DefaultBaseURL = "https://oauth.reddit.com/"
	// DefaultAuthURL is the default Reddit OAuth base URL
	DefaultAuthURL = "https://www.reddit.com/"
	// DefaultUserAgent is the default user agent string
	DefaultUserAgent = "go-reddit-api-wrapper/0.01"
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
	NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error)

	// Do executes an HTTP request and unmarshals the response into a Reddit Thing object.
	// This is used for most Reddit API endpoints that return structured data.
	Do(req *http.Request, v *types.Thing) (*http.Response, error)

	// DoRaw executes an HTTP request and returns the raw response bytes.
	// This is used for endpoints that return non-standard JSON structures.
	DoRaw(req *http.Request) ([]byte, error)
}

// Client is the main Reddit API client.
// It provides methods for common Reddit operations like fetching posts, comments,
// and subreddit information. All methods require the client to be connected first.
//
// Example usage:
//
//	client, err := NewClient(config)
//	if err != nil {
//		return err
//	}
//
//	if err := client.Connect(ctx); err != nil {
//		return err
//	}
//
//	// Now the client is ready to make API calls
//	posts, err := client.GetHot(ctx, &types.PostsRequest{Subreddit: "golang", Limit: 25})
type Client struct {
	client HTTPClient
	auth   TokenProvider
	config *Config
	parser *internal.Parser

	connectOnce sync.Once
	connectErr  error
}

// NewClient creates a new Reddit client with the provided configuration.
// It validates the configuration and sets up the authentication mechanism.
//
// The function will:
//   - Validate that required configuration fields are present
//   - Set default values for optional fields
//   - Create an appropriate authenticator based on the provided credentials
//   - Return a client ready to be connected
//
// Returns an error if:
//   - config is nil
//   - ClientID or ClientSecret are missing
//   - Authentication setup fails
//
// Note: This function does not perform authentication. Call Connect() after
// creating the client to authenticate and prepare it for API calls.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, &ClientError{Err: "config cannot be nil"}
	}

	// Validate required fields
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, &ClientError{Err: "ClientID and ClientSecret are required"}
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
		return nil, &ClientError{Err: err.Error()}
	}

	return &Client{
		auth:   auth,
		config: config,
		parser: internal.NewParser(),
	}, nil
}

// Connect authenticates with Reddit and initializes the internal HTTP client.
// It is safe to call Connect multiple times; initialization will only occur once.
//
// The function will:
//   - Authenticate with Reddit using the provided credentials
//   - Set up the internal HTTP client with proper authentication
//   - Prepare the client for making API requests
//
// Returns an error if:
//   - Authentication fails (invalid credentials, network issues, etc.)
//   - HTTP client initialization fails
//
// After successful connection, IsConnected() will return true and all API
// methods will be available for use.
func (c *Client) Connect(ctx context.Context) error {
	c.connectOnce.Do(func() {
		c.connectErr = c.initialize(ctx)
	})

	return c.connectErr
}

// initialize performs the underlying connection setup work.
func (c *Client) initialize(ctx context.Context) error {
	// Validate that we can get a token before creating the client
	_, err := c.auth.GetToken(ctx)
	if err != nil {
		return &ClientError{Err: "failed to authenticate: " + err.Error()}
	}

	// Create internal HTTP client with token provider
	client, err := internal.NewClient(
		c.config.HTTPClient,
		c.auth,
		c.config.BaseURL,
		c.config.UserAgent,
		c.config.Logger,
	)
	if err != nil {
		return &ClientError{Err: "failed to create HTTP client: " + err.Error()}
	}

	c.client = client
	return nil
}

// ensureConnected lazily initializes the client before handling a request.
func (c *Client) ensureConnected(ctx context.Context) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}

	if !c.IsConnected() {
		return &ClientError{Err: "client not connected, call Connect() first"}
	}

	return nil
}

// IsConnected returns true if the client is authenticated and ready to make requests.
func (c *Client) IsConnected() bool {
	return c.client != nil
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	req, err := c.client.NewRequest(ctx, http.MethodGet, "api/v1/me", nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	var result types.Thing
	_, err = c.client.Do(req, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get user info: " + err.Error()}
	}

	// Parse the account data
	parsed, err := c.parser.ParseThing(&result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse user info: " + err.Error()}
	}

	account, ok := parsed.(*types.AccountData)
	if !ok {
		return nil, &ClientError{Err: "unexpected response type"}
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	path := "r/" + name + "/about"
	req, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	var result types.Thing
	_, err = c.client.Do(req, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get subreddit: " + err.Error()}
	}

	// Parse the subreddit data
	parsed, err := c.parser.ParseThing(&result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse subreddit: " + err.Error()}
	}

	subreddit, ok := parsed.(*types.SubredditData)
	if !ok {
		return nil, &ClientError{Err: "unexpected response type"}
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	subreddit := ""
	pagination := types.Pagination{}
	if request != nil {
		subreddit = request.Subreddit
		pagination = request.Pagination
	}

	path := "hot"
	if subreddit != "" {
		path = "r/" + subreddit + "/hot"
	}

	httpReq, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	q := httpReq.URL.Query()
	if pagination.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", pagination.Limit))
	}
	if pagination.After != "" {
		q.Set("after", pagination.After)
	}
	if pagination.Before != "" {
		q.Set("before", pagination.Before)
	}
	httpReq.URL.RawQuery = q.Encode()

	var result types.Thing
	_, err = c.client.Do(httpReq, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get hot posts: " + err.Error()}
	}

	posts, err := c.parser.ExtractPosts(&result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse posts: " + err.Error()}
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	subreddit := ""
	pagination := types.Pagination{}
	if request != nil {
		subreddit = request.Subreddit
		pagination = request.Pagination
	}

	path := "new"
	if subreddit != "" {
		path = "r/" + subreddit + "/new"
	}

	httpReq, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	q := httpReq.URL.Query()
	if pagination.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", pagination.Limit))
	}
	if pagination.After != "" {
		q.Set("after", pagination.After)
	}
	if pagination.Before != "" {
		q.Set("before", pagination.Before)
	}
	httpReq.URL.RawQuery = q.Encode()

	var result types.Thing
	_, err = c.client.Do(httpReq, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get new posts: " + err.Error()}
	}

	posts, err := c.parser.ExtractPosts(&result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse posts: " + err.Error()}
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &ClientError{Err: "comments request cannot be nil"}
	}
	if request.Subreddit == "" || request.PostID == "" {
		return nil, &ClientError{Err: "subreddit and postID are required"}
	}

	path := "r/" + request.Subreddit + "/comments/" + request.PostID
	httpReq, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	q := httpReq.URL.Query()
	if request.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", request.Limit))
	}
	if request.After != "" {
		q.Set("after", request.After)
	}
	if request.Before != "" {
		q.Set("before", request.Before)
	}
	httpReq.URL.RawQuery = q.Encode()

	resp, err := c.client.DoRaw(httpReq)
	if err != nil {
		return nil, &ClientError{Err: "failed to get comments: " + err.Error()}
	}

	// Reddit can return either an array [post, comments] or a single Listing object
	var result []*types.Thing

	// Log the raw response for debugging (if logger is available)
	if c.config.Logger != nil {
		previewLen := len(resp)
		if previewLen > 500 {
			previewLen = 500
		}
		c.config.Logger.Debug("Reddit API raw response", "path", path, "response_preview", string(resp[:previewLen]))
	}

	// First check if it's an array response
	if len(resp) > 0 && resp[0] == '[' {
		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, &ClientError{Err: "failed to parse comments array response: " + err.Error()}
		}
	} else if len(resp) > 0 && resp[0] == '{' {
		// It's a single object - could be a Listing or an error
		var singleThing types.Thing
		if err := json.Unmarshal(resp, &singleThing); err != nil {
			// Check if it's an error response
			var errObj struct {
				Error   string `json:"error"`
				Message string `json:"message"`
				Reason  string `json:"reason"`
			}
			if err := json.Unmarshal(resp, &errObj); err == nil && errObj.Error != "" {
				return nil, &ClientError{Err: fmt.Sprintf("reddit API error: %s - %s", errObj.Error, errObj.Message)}
			}
			return nil, &ClientError{Err: "failed to parse comments response: " + err.Error()}
		}

		// If it's a Listing with comments, wrap it in an array
		// Some endpoints return just the comments listing without the post
		if singleThing.Kind == "Listing" {
			result = []*types.Thing{&singleThing}
		} else {
			return nil, &ClientError{Err: fmt.Sprintf("unexpected response kind: %s", singleThing.Kind)}
		}
	} else {
		return nil, &ClientError{Err: "empty or invalid response from Reddit"}
	}

	// Log the parsed result structure for debugging
	if c.config.Logger != nil {
		c.config.Logger.Debug("Parsed result structure",
			"path", path,
			"result_count", len(result),
			"first_kind", func() string {
				if len(result) > 0 && result[0] != nil {
					return result[0].Kind
				}
				return "none"
			}(),
			"second_kind", func() string {
				if len(result) > 1 && result[1] != nil {
					return result[1].Kind
				}
				return "none"
			}(),
		)
	}

	// Parse the post and comments
	post, comments, moreIDs, err := c.parser.ExtractPostAndComments(result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse comments: " + err.Error()}
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

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
			resp, err := c.GetComments(ctx, r)
			resultChan <- result{index: index, response: resp, err: err}
		}(i, req)
	}

	// Collect results
	results := make([]*types.CommentsResponse, len(requests))
	var firstError error
	for i := 0; i < len(requests); i++ {
		res := <-resultChan
		if res.err != nil && firstError == nil {
			firstError = res.err
		}
		results[res.index] = res.response
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
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, &ClientError{Err: "more comments request cannot be nil"}
	}
	if request.LinkID == "" {
		return nil, &ClientError{Err: "linkID is required"}
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
	req, err := c.client.NewRequest(ctx, http.MethodPost, "api/morechildren", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	// Set Content-Type header for form data
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// The morechildren endpoint returns a different structure
	var response struct {
		JSON struct {
			Errors [][]string `json:"errors"`
			Data   struct {
				Things []*types.Thing `json:"things"`
			} `json:"data"`
		} `json:"json"`
	}

	// Make authenticated request
	respBody, err := c.client.DoRaw(req)
	if err != nil {
		return nil, &ClientError{Err: "failed to get more comments: " + err.Error()}
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, &ClientError{Err: "failed to parse more comments response: " + err.Error()}
	}

	// Check for API errors
	if len(response.JSON.Errors) > 0 {
		return nil, &ClientError{Err: fmt.Sprintf("API error: %v", response.JSON.Errors[0])}
	}

	// Extract comments from the response
	var comments []*types.Comment
	for _, thing := range response.JSON.Data.Things {
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

// ClientError represents an error from the Reddit client.
// It wraps various types of errors that can occur during API operations,
// including authentication errors, network errors, and API-specific errors.
//
// ClientError provides a consistent error interface for all client operations.
// The underlying error details are stored in the Err field.
type ClientError struct {
	// Err contains the detailed error message describing what went wrong
	Err string
}

// Error implements the error interface for ClientError.
// It returns a formatted error message prefixed with "reddit client error: ".
func (e *ClientError) Error() string {
	return "reddit client error: " + e.Err
}
