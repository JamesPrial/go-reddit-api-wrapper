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
}

// Client is the main Reddit API client.
type Client struct {
	client HTTPClient
	auth   TokenProvider
	config *Config
	parser Parser
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
	// Get access token
	token, err := c.auth.GetToken(ctx)
	if err != nil {
		return &ClientError{Err: "failed to authenticate: " + err.Error()}
	}

	// Create internal HTTP client
	client, err := internal.NewClient(
		c.config.HTTPClient,
		token,
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
func (c *Client) Me(ctx context.Context) (*UserResponse, error) {
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

	return &UserResponse{User: account}, nil
}

// GetSubreddit retrieves information about a specific subreddit.
func (c *Client) GetSubreddit(ctx context.Context, name string) (*SubredditResponse, error) {
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

	return &SubredditResponse{Subreddit: subreddit}, nil
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
	var result []*types.Thing
	if err := c.getJSON(ctx, req, &result); err != nil {
		return nil, &ClientError{Err: "failed to get comments: " + err.Error()}
	}

	// Parse the post and comments
	post, comments, moreIDs, err := c.parser.ExtractPostAndComments(result)
	if err != nil {
		return nil, &ClientError{Err: "failed to parse comments: " + err.Error()}
	}

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

	if err := c.getJSON(ctx, req, &response); err != nil {
		return nil, &ClientError{Err: "failed to get more comments: " + err.Error()}
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

// getJSON is a helper method to handle JSON responses that aren't Thing objects
func (c *Client) getJSON(ctx context.Context, req *http.Request, v interface{}) error {
	resp, err := c.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// ClientError represents an error from the Reddit client.
type ClientError struct {
	Err string
}

// Error implements the error interface.
func (e *ClientError) Error() string {
	return "reddit client error: " + e.Err
}
