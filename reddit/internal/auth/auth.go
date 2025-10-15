package auth

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
	"sync/atomic"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

const (
	defaultTokenEndpointPath = "api/v1/access_token"
	// maxResponseBodySize limits the size of HTTP response bodies to prevent DoS
	maxResponseBodySize   = 10 * 1024 * 1024   // 10MB
	maxTokenExpirySeconds = 365 * 24 * 60 * 60 // 1 year in seconds
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
	clock        clock.Clock // Time abstraction for testing

	// Token cache using atomic pointer for lock-free reads
	cachedToken atomic.Pointer[tokenCache]
	// Mutex to prevent concurrent token refreshes
	tokenMu sync.Mutex
}

// NewAuthenticator creates a new authenticator.
// If a nil clock is provided, a real clock will be used.
func NewAuthenticator(httpClient *http.Client, username, password, clientID, clientSecret, userAgent, baseURL, grantType string, logger *slog.Logger, clk clock.Clock) (*Authenticator, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, &ConfigError{Field: "base_url", Value: baseURL, Err: err}
	}
	if !strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path += "/"
	}
	tokenPath := defaultTokenEndpointPath

	resolvedTokenURL, err := parsedURL.Parse(tokenPath)
	if err != nil {
		return nil, &ConfigError{Field: "token_path", Value: tokenPath, Err: err}
	}

	// Prepare form data upfront
	form := url.Values{}
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
		formData:     &form,
		logger:       logger,
		clock:        clk,
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
		// Capture the current time once for consistent comparison
		now := a.clock.Now()
		if now.Before(cached.expiry) {
			if a.logger != nil {
				a.logger.LogAttrs(ctx, slog.LevelDebug, "using cached reddit token",
					slog.Time("expires_at", cached.expiry))
			}
			return cached.token, nil
		}
	}

	// Cache miss or expired, need to refresh
	// Use mutex to prevent concurrent refreshes
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// Double-check cache after acquiring lock - another goroutine might have refreshed
	if cached := a.cachedToken.Load(); cached != nil {
		now := a.clock.Now()
		if now.Before(cached.expiry) {
			if a.logger != nil {
				a.logger.LogAttrs(ctx, slog.LevelDebug, "using cached reddit token (after lock)",
					slog.Time("expires_at", cached.expiry))
			}
			return cached.token, nil
		}
	}

	// Definitely need to fetch new token
	data := a.formData.Encode()
	start := a.clock.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL.String(), strings.NewReader(data))
	if err != nil {
		a.logAuthError(ctx, "failed to create token request", err)
		return "", &TokenError{Operation: "fetch", Err: err}
	}

	req.SetBasicAuth(a.clientID, a.clientSecret)
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	a.logAuthRequest(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logAuthError(ctx, "failed to execute token request", err)
		return "", &TokenError{Operation: "fetch", Err: err}
	}
	defer resp.Body.Close()

	// Limit response body size to prevent DoS attacks
	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		a.logAuthError(ctx, "failed to read token response", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Err: err}
	}
	// Check if we hit the size limit
	if int64(len(bodyBytes)) == maxResponseBodySize {
		// Try reading one more byte to see if there's more data
		var extraByte [1]byte
		if n, _ := resp.Body.Read(extraByte[:]); n > 0 {
			err := fmt.Errorf("response body exceeded max size of %d bytes", maxResponseBodySize)
			a.logAuthError(ctx, "response body too large", err)
			return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes[:1000]), Err: err}
		}
	}

	duration := a.clock.Since(start)
	a.logAuthHTTPResult(ctx, resp.StatusCode, duration, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes)}
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		a.logAuthError(ctx, "failed to decode token response", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), Err: err}
	}

	if tokenResp.AccessToken == "" {
		err := fmt.Errorf("access token was empty in response")
		a.logAuthError(ctx, "received empty access token", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), Err: err}
	}

	// Validate ExpiresIn bounds to prevent integer overflow and invalid values

	if tokenResp.ExpiresIn < 0 {
		err := fmt.Errorf("invalid expires_in value: %d (cannot be negative)", tokenResp.ExpiresIn)
		a.logAuthError(ctx, "received negative expires_in", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), Err: err}
	}
	if tokenResp.ExpiresIn > maxTokenExpirySeconds {
		err := fmt.Errorf("invalid expires_in value: %d (exceeds maximum of %d seconds)", tokenResp.ExpiresIn, maxTokenExpirySeconds)
		a.logAuthError(ctx, "received expires_in exceeding maximum", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), Err: err}
	}

	// Cache the token with tiered expiry thresholds based on token lifetime
	// This ensures tokens refresh proactively before they actually expire
	actualExpiry := time.Duration(tokenResp.ExpiresIn) * time.Second
	var cacheRatio float64

	// Tiered thresholds for different token lifetimes:
	if actualExpiry > 60*time.Second {
		// Long-lived tokens (>60s): 80% threshold (refresh with 20% lifetime remaining)
		cacheRatio = 0.80
	} else if actualExpiry >= 10*time.Second {
		// Medium-lived tokens (10-60s): 50% threshold (refresh at half-life)
		cacheRatio = 0.50
	} else {
		// Very short-lived tokens (<10s): 90% threshold (minimal margin)
		cacheRatio = 0.90
	}

	expiryDuration := time.Duration(float64(actualExpiry) * cacheRatio)

	a.cachedToken.Store(&tokenCache{
		token:  tokenResp.AccessToken,
		expiry: a.clock.Now().Add(expiryDuration),
	})

	a.logAuthSuccess(ctx, duration, tokenResp)

	return tokenResp.AccessToken, nil
}

// InvalidateToken clears the cached token, forcing a fresh token fetch on next GetToken call
func (a *Authenticator) InvalidateToken() {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	a.cachedToken.Store(nil)
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

// contextOrBackground returns the provided context or Background as fallback.
// Used to ensure logging functions always have a valid context.
func contextOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
