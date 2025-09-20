package graw

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	)
	if err != nil {
		return nil, &ClientError{Err: err.Error()}
	}

	return &Client{
		auth:   auth,
		config: config,
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
func (c *Client) Me(ctx context.Context) (*types.Thing, error) {
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

	return &result, nil
}

// GetSubreddit retrieves information about a specific subreddit.
func (c *Client) GetSubreddit(ctx context.Context, name string) (*types.Thing, error) {
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

	return &result, nil
}

// ListingOptions provides options for listing operations.
type ListingOptions struct {
	Limit  int    // Number of items to retrieve (max 100)
	After  string // Get items after this item ID
	Before string // Get items before this item ID
}

// GetHot retrieves hot posts from a subreddit.
// If subreddit is empty, it gets from the front page.
func (c *Client) GetHot(ctx context.Context, subreddit string, opts *ListingOptions) (*types.Thing, error) {
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

	return &result, nil
}

// GetNew retrieves new posts from a subreddit.
// If subreddit is empty, it gets from the front page.
func (c *Client) GetNew(ctx context.Context, subreddit string, opts *ListingOptions) (*types.Thing, error) {
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

	return &result, nil
}

// GetComments retrieves comments for a specific post.
func (c *Client) GetComments(ctx context.Context, subreddit, postID string, opts *ListingOptions) (*types.Thing, error) {
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

	var result types.Thing
	_, err = c.client.Do(req, &result)
	if err != nil {
		return nil, &ClientError{Err: "failed to get comments: " + err.Error()}
	}

	return &result, nil
}

// ClientError represents an error from the Reddit client.
type ClientError struct {
	Err string
}

// Error implements the error interface.
func (e *ClientError) Error() string {
	return "reddit client error: " + e.Err
}
