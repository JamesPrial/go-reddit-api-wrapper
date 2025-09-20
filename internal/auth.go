package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultTokenEndpointPath = "api/v1/access_token"

// Authenticator handles retrieving an access token from the Reddit API.
type Authenticator struct {
	client       *http.Client
	clientID     string
	clientSecret string
	userAgent    string
	BaseURL      *url.URL
	tokenURL     *url.URL
	formData     *url.Values
}

// NewAuthenticator creates a new authenticator.
// The tokenPath parameter can be an empty string to use the default Reddit token endpoint.
func NewAuthenticator(httpClient *http.Client, username, password, clientID, clientSecret, userAgent, baseURL, grantType, tokenPath string) (*Authenticator, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, &AuthError{Err: fmt.Errorf("failed to parse base URL: %w", err)}
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}

	if tokenPath == "" {
		tokenPath = defaultTokenEndpointPath
	}

	resolvedTokenURL, err := parsedURL.Parse(tokenPath)
	if err != nil {
		return nil, &AuthError{Err: fmt.Errorf("failed to parse token endpoint path: %w", err)}
	}

	// Prepare form data upfront
	form := &url.Values{}
	form.Add("grant_type", grantType)
	if username != "" && password != "" {
		form.Add("username", username)
		form.Add("password", password)
	}

	return &Authenticator{
		client:       httpClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		userAgent:    userAgent,
		BaseURL:      parsedURL,
		tokenURL:     resolvedTokenURL,
		formData:     form,
	}, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// GetToken performs the password grant flow to get an access token.
func (a *Authenticator) GetToken(ctx context.Context) (string, error) {
	data := a.formData.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL.String(), strings.NewReader(data))
	if err != nil {
		return "", &AuthError{Err: fmt.Errorf("failed to create token request: %w", err)}
	}

	req.SetBasicAuth(a.clientID, a.clientSecret)
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", &AuthError{Err: fmt.Errorf("failed to execute token request: %w", err)}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// Error reading the response body.
		return "", &AuthError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("failed to read response body: %w", err),
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &AuthError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", &AuthError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
			Err:        fmt.Errorf("failed to unmarshal token response: %w", err),
		}
	}

	if tokenResp.AccessToken == "" {
		return "", &AuthError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
			Err:        fmt.Errorf("access token was empty in response"),
		}
	}

	return tokenResp.AccessToken, nil
}

// AuthError represents an error that occurred during authentication.
type AuthError struct {
	StatusCode int
	// Body contains the raw response body from the server, which may hold more details.
	Body string
	// Err is the underlying error that occurred, e.g., a network or JSON parsing error.
	Err error
}

// Error implements the error interface, providing a detailed error message.
func (e *AuthError) Error() string {
	var sb strings.Builder
	sb.WriteString("auth error")

	if e.StatusCode != 0 {
		fmt.Fprintf(&sb, ": status code %d", e.StatusCode)
	}

	if e.Body != "" {
		// Use Fprintf to correctly handle quoting the body string.
		fmt.Fprintf(&sb, ", body: %q", e.Body)
	}

	if e.Err != nil {
		fmt.Fprintf(&sb, ", err: %v", e.Err)
	}

	return sb.String()
}

// Unwrap allows for error chaining with errors.Is and errors.As.
func (e *AuthError) Unwrap() error { return e.Err }
