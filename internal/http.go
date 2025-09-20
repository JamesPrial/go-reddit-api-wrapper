package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Client manages communication with the Reddit API.
type Client struct {
	client    *http.Client
	BaseURL   *url.URL
	UserAgent string
	token     string
}

// NewClient returns a new Reddit API client.
// If a nil httpClient is provided, http.DefaultClient will be used.
func NewClient(httpClient *http.Client, authToken string, baseURL string, userAgent string) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}

	c := &Client{
		client:    httpClient,
		BaseURL:   parsedURL,
		UserAgent: userAgent,
		token:     authToken,
	}

	return c, nil
}

// NewRequest creates an API request. A relative URL can be provided in path,
// in which case it is resolved relative to the BaseURL of the Client.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u, err := c.BaseURL.Parse(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.UserAgent)

	return req, nil
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred.
func (c *Client) Do(req *http.Request, v *types.Thing) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, fmt.Errorf("API request failed with status %s", resp.Status)
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return resp, err
		}
	}

	return resp, nil
}
