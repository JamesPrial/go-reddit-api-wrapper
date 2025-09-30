package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
)

const (
	defaultTokenEndpointPath = "api/v1/access_token"
)

// tokenCache holds cached token data immutably
type tokenCache struct {
	token  string
	expiry time.Time
}

// Authenticator handles retrieving an access token from the Reddit API.
type Authenticator struct {
	client       *http.Client
	clientID     string
	clientSecret string
	userAgent    string
	BaseURL      *url.URL
	tokenURL     *url.URL
	formData     *url.Values
	logger       *slog.Logger

	// Token cache using atomic pointer for lock-free access
	cachedToken atomic.Pointer[tokenCache]
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(httpClient *http.Client, username, password, clientID, clientSecret, userAgent, baseURL, grantType string, logger *slog.Logger) (*Authenticator, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, &pkgerrs.AuthError{Err: fmt.Errorf("failed to parse base URL: %w", err)}
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}
	tokenPath := defaultTokenEndpointPath

	resolvedTokenURL, err := parsedURL.Parse(tokenPath)
	if err != nil {
		return nil, &pkgerrs.AuthError{Err: fmt.Errorf("failed to parse token endpoint path: %w", err)}
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
		logger:       logger,
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
	// Check cache first - lock-free read
	if cached := a.cachedToken.Load(); cached != nil {
		if time.Now().Before(cached.expiry) {
			if a.logger != nil {
				a.logger.LogAttrs(ctx, slog.LevelDebug, "using cached reddit token",
					slog.Time("expires_at", cached.expiry))
			}
			return cached.token, nil
		}
	}

	// Cache miss or expired, fetch new token
	data := a.formData.Encode()
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL.String(), strings.NewReader(data))
	if err != nil {
		a.logAuthError(ctx, "failed to create token request", err)
		return "", &pkgerrs.AuthError{Err: fmt.Errorf("failed to create token request: %w", err)}
	}

	req.SetBasicAuth(a.clientID, a.clientSecret)
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	a.logAuthRequest(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logAuthError(ctx, "failed to execute token request", err)
		return "", &pkgerrs.AuthError{Err: fmt.Errorf("failed to execute token request: %w", err)}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		a.logAuthError(ctx, "failed to read token response", err)
		// Error reading the response body.
		return "", &pkgerrs.AuthError{
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("failed to read response body: %w", err),
		}
	}

	duration := time.Since(start)
	a.logAuthHTTPResult(ctx, resp.StatusCode, duration, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return "", &pkgerrs.AuthError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
		}
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		a.logAuthError(ctx, "failed to decode token response", err)
		return "", &pkgerrs.AuthError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
			Err:        fmt.Errorf("failed to unmarshal token response: %w", err),
		}
	}

	if tokenResp.AccessToken == "" {
		emptyErr := fmt.Errorf("access token was empty in response")
		a.logAuthError(ctx, "received empty access token", emptyErr)
		return "", &pkgerrs.AuthError{
			StatusCode: resp.StatusCode,
			Body:       string(bodyBytes),
			Err:        emptyErr,
		}
	}

	// Cache the token with expiry - atomic store
	// Use 90% of the expiry time to ensure we refresh before it actually expires
	expiryDuration := time.Duration(float64(tokenResp.ExpiresIn) * 0.9 * float64(time.Second))
	a.cachedToken.Store(&tokenCache{
		token:  tokenResp.AccessToken,
		expiry: time.Now().Add(expiryDuration),
	})

	a.logAuthSuccess(ctx, duration, tokenResp)

	return tokenResp.AccessToken, nil
}

func (a *Authenticator) logAuthRequest(ctx context.Context) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{}
	if a.tokenURL != nil {
		attrs = append(attrs, slog.String("url", a.tokenURL.String()))
	}
	if a.formData != nil {
		attrs = append(attrs, slog.String("grant_type", a.formData.Get("grant_type")))
	}

	a.logger.LogAttrs(ctx, slog.LevelDebug, "requesting reddit access token", attrs...)
}

func (a *Authenticator) logAuthHTTPResult(ctx context.Context, status int, duration time.Duration, body []byte) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{
		slog.Int("status", status),
		slog.Duration("duration", duration),
		slog.Int("response_bytes", len(body)),
	}
	if a.tokenURL != nil {
		attrs = append(attrs, slog.String("url", a.tokenURL.String()))
	}

	level := slog.LevelInfo
	msg := "reddit auth token requested"
	if status != http.StatusOK {
		level = slog.LevelWarn
		msg = "reddit auth token request failed"
	}

	a.logger.LogAttrs(ctx, level, msg, attrs...)
}

func (a *Authenticator) logAuthError(ctx context.Context, message string, err error) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{slog.String("error", err.Error())}
	if a.tokenURL != nil {
		attrs = append(attrs, slog.String("url", a.tokenURL.String()))
	}

	a.logger.LogAttrs(ctx, slog.LevelError, message, attrs...)
}

func (a *Authenticator) logAuthSuccess(ctx context.Context, duration time.Duration, token tokenResponse) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{slog.Duration("duration", duration)}
	if token.ExpiresIn > 0 {
		attrs = append(attrs, slog.Int("expires_in", token.ExpiresIn))
	}
	if token.Scope != "" {
		attrs = append(attrs, slog.String("scope", token.Scope))
	}
	if token.TokenType != "" {
		attrs = append(attrs, slog.String("token_type", token.TokenType))
	}

	a.logger.LogAttrs(ctx, slog.LevelInfo, "reddit token acquired", attrs...)
}

