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
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/reqid"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/cache"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

const (
	defaultTokenEndpointPath = "api/v1/access_token"
	// maxResponseBodySize limits the size of HTTP response bodies to prevent DoS
	maxResponseBodySize   = 10 * 1024 * 1024   // 10MB
	maxTokenExpirySeconds = 365 * 24 * 60 * 60 // 1 year in seconds
)

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

	cache cache.Cache // Token cache

	// Mutex to prevent concurrent token refreshes
	tokenMu sync.Mutex
}

// NewAuthenticator creates a new authenticator.
// If a nil clock is provided, a real clock will be used.
// If a nil tokenCache is provided, a memory cache will be used.
func NewAuthenticator(httpClient *http.Client, username, password, clientID, clientSecret, userAgent, baseURL, grantType string, logger *slog.Logger, clk clock.Clock, tokenCache cache.Cache) (*Authenticator, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}

	if tokenCache == nil {
		tokenCache = cache.NewMemoryCache(clk)
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
		cache:        tokenCache,
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
	// Extract request ID at method entry for tracing
	requestID := reqid.FromContext(ctx)

	// Check cache first - lock-free read
	token, expiry, found, err := a.cache.Get(ctx)
	if err == nil && found {
		if a.logger != nil {
			attrs := []slog.Attr{slog.Time("expires_at", expiry)}
			if requestID != "" {
				attrs = append(attrs, slog.String("request_id", requestID))
			}
			a.logger.LogAttrs(ctx, slog.LevelDebug, "using cached reddit token", attrs...)
		}
		return token, nil
	}

	// Cache miss or expired, need to refresh
	// Use mutex to prevent concurrent refreshes
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// Double-check cache after acquiring lock - another goroutine might have refreshed
	token, expiry, found, err = a.cache.Get(ctx)
	if err == nil && found {
		if a.logger != nil {
			attrs := []slog.Attr{slog.Time("expires_at", expiry)}
			if requestID != "" {
				attrs = append(attrs, slog.String("request_id", requestID))
			}
			a.logger.LogAttrs(ctx, slog.LevelDebug, "using cached reddit token (after lock)", attrs...)
		}
		return token, nil
	}

	// Definitely need to fetch new token
	data := a.formData.Encode()
	start := a.clock.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL.String(), strings.NewReader(data))
	if err != nil {
		a.logAuthError(ctx, requestID, "failed to create token request", err)
		return "", &TokenError{Operation: "fetch", RequestID: requestID, Err: err}
	}

	req.SetBasicAuth(a.clientID, a.clientSecret)
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	a.logAuthRequest(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logAuthError(ctx, requestID, "failed to execute token request", err)
		return "", &TokenError{Operation: "fetch", RequestID: requestID, Err: err}
	}
	defer resp.Body.Close()

	// Limit response body size to prevent DoS attacks
	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		a.logAuthError(ctx, requestID, "failed to read token response", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, RequestID: requestID, Err: err}
	}
	// Check if we hit the size limit
	if int64(len(bodyBytes)) == maxResponseBodySize {
		// Try reading one more byte to see if there's more data
		var extraByte [1]byte
		if n, _ := resp.Body.Read(extraByte[:]); n > 0 {
			err := fmt.Errorf("response body exceeded max size of %d bytes", maxResponseBodySize)
			a.logAuthError(ctx, requestID, "response body too large", err)
			return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes[:1000]), RequestID: requestID, Err: err}
		}
	}

	duration := a.clock.Since(start)
	a.logAuthHTTPResult(ctx, requestID, resp.StatusCode, duration, bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), RequestID: requestID}
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		a.logAuthError(ctx, requestID, "failed to decode token response", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), RequestID: requestID, Err: err}
	}

	if tokenResp.AccessToken == "" {
		err := fmt.Errorf("access token was empty in response")
		a.logAuthError(ctx, requestID, "received empty access token", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), RequestID: requestID, Err: err}
	}

	// Validate ExpiresIn bounds to prevent integer overflow and invalid values

	if tokenResp.ExpiresIn < 0 {
		err := fmt.Errorf("invalid expires_in value: %d (cannot be negative)", tokenResp.ExpiresIn)
		a.logAuthError(ctx, requestID, "received negative expires_in", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), RequestID: requestID, Err: err}
	}
	if tokenResp.ExpiresIn > maxTokenExpirySeconds {
		err := fmt.Errorf("invalid expires_in value: %d (exceeds maximum of %d seconds)", tokenResp.ExpiresIn, maxTokenExpirySeconds)
		a.logAuthError(ctx, requestID, "received expires_in exceeding maximum", err)
		return "", &TokenError{Operation: "fetch", HTTPStatus: resp.StatusCode, Body: string(bodyBytes), RequestID: requestID, Err: err}
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
	cacheExpiry := a.clock.Now().Add(expiryDuration)

	// Store in cache
	if err := a.cache.Set(ctx, tokenResp.AccessToken, cacheExpiry); err != nil {
		// Log warning but don't fail the request - we have the token
		if a.logger != nil {
			attrs := []slog.Attr{slog.String("error", err.Error())}
			if requestID != "" {
				attrs = append(attrs, slog.String("request_id", requestID))
			}
			a.logger.LogAttrs(ctx, slog.LevelWarn, "failed to cache token", attrs...)
		}
	}

	a.logAuthSuccess(ctx, requestID, duration, tokenResp)

	return tokenResp.AccessToken, nil
}

// InvalidateToken clears the cached token, forcing a fresh token fetch on next GetToken call
func (a *Authenticator) InvalidateToken(ctx context.Context) {
	if err := a.cache.Invalidate(ctx); err != nil {
		if a.logger != nil {
			requestID := reqid.FromContext(ctx)
			attrs := []slog.Attr{slog.String("error", err.Error())}
			if requestID != "" {
				attrs = append(attrs, slog.String("request_id", requestID))
			}
			a.logger.LogAttrs(ctx, slog.LevelWarn, "failed to invalidate token cache", attrs...)
		}
	}
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

func (a *Authenticator) logAuthHTTPResult(ctx context.Context, requestID string, status int, duration time.Duration, body []byte) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{
		slog.Int("status", status),
		slog.Duration("duration", duration),
		slog.Int("response_bytes", len(body)),
	}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
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

func (a *Authenticator) logAuthError(ctx context.Context, requestID string, message string, err error) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{slog.String("error", err.Error())}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if a.tokenURL != nil {
		attrs = append(attrs, slog.String("url", a.tokenURL.String()))
	}

	a.logger.LogAttrs(ctx, slog.LevelError, message, attrs...)
}

func (a *Authenticator) logAuthSuccess(ctx context.Context, requestID string, duration time.Duration, token tokenResponse) {
	if a.logger == nil {
		return
	}

	ctx = contextOrBackground(ctx)
	attrs := []slog.Attr{slog.Duration("duration", duration)}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
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
