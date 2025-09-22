package graw

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
type Config struct {
	// Username and Password for password grant flow
	Username string
	Password string
	// ClientID and ClientSecret for OAuth2 authentication
	ClientID     string
	ClientSecret string
	// UserAgent string to identify your application
	UserAgent string
	// BaseURL for the Reddit API (defaults to DefaultBaseURL)
	BaseURL string
	// AuthURL for Reddit OAuth (defaults to DefaultAuthURL)
	AuthURL string
	// HTTPClient to use for requests (defaults to a client with DefaultTimeout)
	HTTPClient *http.Client
	// Logger for structured diagnostics (optional)
	Logger *slog.Logger
}

// TokenProvider defines the interface for retrieving an access token.
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

// HTTPClient defines the behavior required from the internal HTTP client.
type HTTPClient interface {
	NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error)
	Do(req *http.Request, v *types.Thing) (*http.Response, error)
	DoRaw(req *http.Request) ([]byte, error)
}

// Client is the main Reddit API client.
type Client struct {
	client HTTPClient
	auth   TokenProvider
	config *Config
	parser *internal.Parser
}

// NewClient creates a new Reddit client with the provided configuration.
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
		"", // Use default token path
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
// This must be called before making any API requests.
func (c *Client) Connect(ctx context.Context) error {
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

// IsConnected returns true if the client is authenticated and ready to make requests.
func (c *Client) IsConnected() bool {
	return c.client != nil
}

// Me returns information about the authenticated user.
// This is useful for testing authentication and getting user details.
func (c *Client) Me(ctx context.Context) (*types.AccountData, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
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
func (c *Client) GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
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

// ListingOptions provides options for listing operations.
type ListingOptions struct {
	Limit  int    // Number of items to retrieve (max 100)
	After  string // Get items after this item ID
	Before string // Get items before this item ID
}

// MoreCommentsOptions provides options for loading additional comments.
type MoreCommentsOptions struct {
	Sort  string // Sort order: "confidence", "new", "top", "controversial", "old", "qa"
	Depth int    // Maximum depth of replies to retrieve (0 for no limit)
	Limit int    // Maximum number of comments to retrieve (default 100)
}

// GetHot retrieves hot posts from a subreddit.
// If subreddit is empty, it gets from the front page.
func (c *Client) GetHot(ctx context.Context, subreddit string, opts *ListingOptions) (*PostsResponse, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
	}

	var path string
	if subreddit == "" {
		path = "hot"
	} else {
		path = "r/" + subreddit + "/hot"
	}

	req, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	// Add query parameters if options provided
	if opts != nil {
		q := req.URL.Query()
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.After != "" {
			q.Set("after", opts.After)
		}
		if opts.Before != "" {
			q.Set("before", opts.Before)
		}
		req.URL.RawQuery = q.Encode()
	}

	var result types.Thing
	_, err = c.client.Do(req, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get hot posts: " + err.Error()}
	}

	// Extract posts from the listing
	posts, err := c.parser.ExtractPosts(&result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse posts: " + err.Error()}
	}

	// Get pagination info
	listing, _ := c.parser.ParseThing(&result)
	listingData, _ := listing.(*types.ListingData)

	return &PostsResponse{
		Posts:  posts,
		After:  listingData.After,
		Before: listingData.Before,
	}, nil
}

// GetNew retrieves new posts from a subreddit.
// If subreddit is empty, it gets from the front page.
func (c *Client) GetNew(ctx context.Context, subreddit string, opts *ListingOptions) (*PostsResponse, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
	}

	var path string
	if subreddit == "" {
		path = "new"
	} else {
		path = "r/" + subreddit + "/new"
	}

	req, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	// Add query parameters if options provided
	if opts != nil {
		q := req.URL.Query()
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		if opts.After != "" {
			q.Set("after", opts.After)
		}
		if opts.Before != "" {
			q.Set("before", opts.Before)
		}
		req.URL.RawQuery = q.Encode()
	}

	var result types.Thing
	_, err = c.client.Do(req, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get new posts: " + err.Error()}
	}

	// Extract posts from the listing
	posts, err := c.parser.ExtractPosts(&result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse posts: " + err.Error()}
	}

	// Get pagination info
	listing, _ := c.parser.ParseThing(&result)
	listingData, _ := listing.(*types.ListingData)

	return &PostsResponse{
		Posts:  posts,
		After:  listingData.After,
		Before: listingData.Before,
	}, nil
}

// GetComments retrieves comments for a specific post.
func (c *Client) GetComments(ctx context.Context, subreddit, postID string, opts *ListingOptions) (*CommentsResponse, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
	}

	path := "r/" + subreddit + "/comments/" + postID
	req, err := c.client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	// Add query parameters if options provided
	if opts != nil {
		q := req.URL.Query()
		if opts.Limit > 0 {
			q.Set("limit", fmt.Sprintf("%d", opts.Limit))
		}
		req.URL.RawQuery = q.Encode()
	}

	// Reddit returns an array of listings for comments endpoint
	// We can't use c.client.Do because it expects a single Thing, not an array
	// So we need to use DoRaw to get the raw JSON response
	resp, err := c.client.DoRaw(req)
	if err != nil {
		return nil, &ClientError{Err: "failed to get comments: " + err.Error()}
	}

	// Reddit can return either an array [post, comments] or a single Listing object
	var result []*types.Thing

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

	// Parse the post and comments
	post, comments, moreIDs, err := c.parser.ExtractPostAndComments(result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse comments: " + err.Error()}
	}

	// Note: post may be nil if Reddit only returned comments without the post
	return &CommentsResponse{
		Post:     post,
		Comments: comments,
		MoreIDs:  moreIDs,
	}, nil
}

// CommentRequest represents a request for loading comments for a specific post.
type CommentRequest struct {
	Subreddit string
	PostID    string
	Options   *ListingOptions
}

// GetCommentsMultiple loads comments for multiple posts in parallel.
// This is more efficient than calling GetComments multiple times sequentially.
func (c *Client) GetCommentsMultiple(ctx context.Context, requests []CommentRequest) ([]*CommentsResponse, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
	}

	if len(requests) == 0 {
		return []*CommentsResponse{}, nil
	}

	// Create channels for results
	type result struct {
		index    int
		response *CommentsResponse
		err      error
	}
	resultChan := make(chan result, len(requests))

	// Launch goroutines for parallel fetching
	for i, req := range requests {
		go func(index int, r CommentRequest) {
			resp, err := c.GetComments(ctx, r.Subreddit, r.PostID, r.Options)
			resultChan <- result{index: index, response: resp, err: err}
		}(i, req)
	}

	// Collect results
	results := make([]*CommentsResponse, len(requests))
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
func (c *Client) GetMoreComments(ctx context.Context, linkID string, commentIDs []string, opts *MoreCommentsOptions) ([]*types.Comment, error) {
	if !c.IsConnected() {
		return nil, &ClientError{Err: "client not connected, call Connect() first"}
	}

	if len(commentIDs) == 0 {
		return []*types.Comment{}, nil
	}

	// Reddit's link_id format requires the type prefix (t3_)
	if !strings.HasPrefix(linkID, "t3_") {
		linkID = "t3_" + linkID
	}

	req, err := c.client.NewRequest(ctx, http.MethodGet, "api/morechildren", nil)
	if err != nil {
		return nil, &ClientError{Err: "failed to create request: " + err.Error()}
	}

	// Build query parameters
	q := req.URL.Query()
	q.Set("link_id", linkID)
	q.Set("children", strings.Join(commentIDs, ","))
	q.Set("api_type", "json")

	if opts != nil {
		if opts.Sort != "" {
			q.Set("sort", opts.Sort)
		}
		if opts.Depth > 0 {
			q.Set("depth", fmt.Sprintf("%d", opts.Depth))
		}
		if opts.Limit > 0 {
			q.Set("limit_children", fmt.Sprintf("%d", opts.Limit))
		}
	}

	req.URL.RawQuery = q.Encode()

	// The morechildren endpoint returns a different structure
	var response struct {
		JSON struct {
			Errors [][]string   `json:"errors"`
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
			var comment types.Comment
			if err := json.Unmarshal(thing.Data, &comment); err != nil {
				continue // Skip if we can't unmarshal
			}
			comments = append(comments, &comment)
		}
	}

	return comments, nil
}

// ClientError represents an error from the Reddit client.
type ClientError struct {
	Err string
}

// Error implements the error interface.
func (e *ClientError) Error() string {
	return "reddit client error: " + e.Err
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
