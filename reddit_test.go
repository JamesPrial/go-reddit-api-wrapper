package graw

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// mockHTTPClient implements the HTTPClient interface for testing
type mockHTTPClient struct {
	newRequestFunc     func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error)
	doFunc             func(req *http.Request, v *types.Thing) error
	doThingArrayFunc   func(req *http.Request) ([]*types.Thing, error)
	doMoreChildrenFunc func(req *http.Request) ([]*types.Thing, error)
}

func (m *mockHTTPClient) NewRequest(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
	if m.newRequestFunc != nil {
		return m.newRequestFunc(ctx, method, path, body, params...)
	}
	req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
	if len(params) > 0 && params[0] != nil {
		req.URL.RawQuery = params[0].Encode()
	}
	return req, nil
}

func (m *mockHTTPClient) Do(req *http.Request, v *types.Thing) error {
	if m.doFunc != nil {
		return m.doFunc(req, v)
	}
	return nil
}

func (m *mockHTTPClient) DoThingArray(req *http.Request) ([]*types.Thing, error) {
	if m.doThingArrayFunc != nil {
		return m.doThingArrayFunc(req)
	}
	return nil, nil
}

func (m *mockHTTPClient) DoMoreChildren(req *http.Request) ([]*types.Thing, error) {
	if m.doMoreChildrenFunc != nil {
		return m.doMoreChildrenFunc(req)
	}
	return nil, nil
}

// mockTokenProvider implements the TokenProvider interface for testing
type mockTokenProvider struct {
	token string
	err   error
}

func (m *mockTokenProvider) GetToken(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

func newTestClient(httpClient HTTPClient, auth TokenProvider) *Reddit {
	if auth == nil {
		auth = &mockTokenProvider{token: "test_token"}
	}
	return &Reddit{
		httpClient: httpClient,
		auth:       auth,
		config: &Config{
			UserAgent: "test/1.0",
			BaseURL:   "https://oauth.reddit.com/",
		},
		parser:    internal.NewParser(),
		validator: internal.NewValidator(),
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorType string
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "missing client ID",
			config: &Config{
				ClientSecret: "secret",
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "missing client secret",
			config: &Config{
				ClientID: "id",
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "valid config",
			config: &Config{
				ClientID:     "test_id",
				ClientSecret: "test_secret",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType == "ConfigError" {
					if _, ok := err.(*pkgerrs.ConfigError); !ok {
						t.Errorf("expected ConfigError, got %T", err)
					}
				}
			} else {
				if err != nil {
					// Auth will fail in tests without proper setup, but we're testing config validation
					t.Logf("got expected auth error: %v", err)
				}
			}
			if !tt.wantError && err == nil && client == nil {
				t.Error("expected client but got nil")
			}
		})
	}
}

func TestNewClient_InvalidUserAgent(t *testing.T) {
	t.Parallel()

	config := &Config{
		ClientID:     "id",
		ClientSecret: "secret",
		UserAgent:    "bad\nagent",
	}

	_, err := NewClientWithContext(context.Background(), config)
	if err == nil {
		t.Fatal("expected error but got none")
	}
	var configErr *pkgerrs.ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if !strings.Contains(configErr.Message, "invalid user agent") {
		t.Fatalf("expected invalid user agent message, got %s", configErr.Message)
	}
}

func TestNewClient_HTTPClientTimeoutHandling(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("timeout zero sanitized", func(t *testing.T) {
		t.Parallel()

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/access_token" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"stub","token_type":"bearer","expires_in":3600}`))
		}))
		t.Cleanup(tokenServer.Close)

		httpClient := tokenServer.Client()
		httpClient.Timeout = 0

		config := &Config{
			ClientID:     "id",
			ClientSecret: "secret",
			UserAgent:    "tester",
			AuthURL:      tokenServer.URL + "/",
			BaseURL:      tokenServer.URL + "/",
			HTTPClient:   httpClient,
			Logger:       logger,
		}

		client, err := NewClientWithContext(context.Background(), config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected client instance")
		}
		if got := config.HTTPClient.Timeout; got != DefaultTimeout {
			t.Fatalf("expected timeout to be set to default (%v), got %v", DefaultTimeout, got)
		}
		if original := httpClient.Timeout; original != 0 {
			t.Fatalf("expected original client timeout to remain 0, got %v", original)
		}
	})

	t.Run("timeout too short rejected", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			ClientID:     "id",
			ClientSecret: "secret",
			UserAgent:    "tester",
			HTTPClient:   &http.Client{Timeout: 500 * time.Millisecond},
		}

		_, err := NewClientWithContext(context.Background(), config)
		if err == nil {
			t.Fatal("expected error but got none")
		}
		var configErr *pkgerrs.ConfigError
		if !errors.As(err, &configErr) {
			t.Fatalf("expected ConfigError, got %T", err)
		}
		if !strings.Contains(configErr.Message, "timeout too short") {
			t.Fatalf("expected timeout too short message, got %s", configErr.Message)
		}
	})

	t.Run("timeout too long warns", func(t *testing.T) {
		t.Parallel()

		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/access_token" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"stub","token_type":"bearer","expires_in":3600}`))
		}))
		t.Cleanup(tokenServer.Close)

		httpClient := tokenServer.Client()
		httpClient.Timeout = 10 * time.Minute

		config := &Config{
			ClientID:     "id",
			ClientSecret: "secret",
			UserAgent:    "tester",
			AuthURL:      tokenServer.URL + "/",
			BaseURL:      tokenServer.URL + "/",
			HTTPClient:   httpClient,
			Logger:       logger,
		}

		client, err := NewClientWithContext(context.Background(), config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("expected client instance")
		}
		if got := config.HTTPClient.Timeout; got != httpClient.Timeout {
			t.Fatalf("expected timeout to remain %v, got %v", httpClient.Timeout, got)
		}
	})
}

func TestNewClientWithContext_InvalidAuthURL(t *testing.T) {
	t.Parallel()

	config := &Config{
		ClientID:     "id",
		ClientSecret: "secret",
		UserAgent:    "tester",
		AuthURL:      "::://bad",
	}

	_, err := NewClientWithContext(context.Background(), config)
	if err == nil {
		t.Fatal("expected error but got none")
	}
	var authErr *pkgerrs.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T", err)
	}
}

func TestNewClientWithContext_AuthenticationFailure(t *testing.T) {
	t.Parallel()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/access_token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"error"}`))
	}))
	t.Cleanup(tokenServer.Close)

	config := &Config{
		ClientID:     "id",
		ClientSecret: "secret",
		UserAgent:    "tester",
		AuthURL:      tokenServer.URL + "/",
		BaseURL:      tokenServer.URL + "/",
		HTTPClient:   tokenServer.Client(),
	}

	_, err := NewClientWithContext(context.Background(), config)
	if err == nil {
		t.Fatal("expected error but got none")
	}
	var authErr *pkgerrs.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T", err)
	}
	if !strings.Contains(authErr.Error(), "failed to authenticate") {
		t.Fatalf("expected authenticate failure message, got %v", authErr)
	}
}

func TestNewClientWithContext_RateLimitConfig(t *testing.T) {
	t.Parallel()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/access_token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"stub","token_type":"bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenServer.Close)

	config := &Config{
		ClientID:     "id",
		ClientSecret: "secret",
		UserAgent:    "tester",
		AuthURL:      tokenServer.URL + "/",
		BaseURL:      tokenServer.URL + "/",
		HTTPClient:   tokenServer.Client(),
		RateLimitConfig: &RateLimitConfig{
			RequestsPerMinute:  120,
			Burst:              15,
			ProactiveThreshold: 8,
		},
	}

	client, err := NewClientWithContext(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client instance")
	}
}

func TestClient_Me(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() HTTPClient
		setupAuth func() TokenProvider
		wantError bool
		errorType string
	}{
		{
			name: "successful request",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						accountData := `{"id":"abc123","name":"testuser","link_karma":100,"comment_karma":50}`
						*v = types.Thing{
							Kind: "t2",
							Data: json.RawMessage(accountData),
						}
						return nil
					},
				}
			},
			setupAuth: func() TokenProvider {
				return &mockTokenProvider{token: "valid_token"}
			},
			wantError: false,
		},
		{
			name: "auth error",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			setupAuth: func() TokenProvider {
				return &mockTokenProvider{err: errors.New("auth failed")}
			},
			wantError: true,
			errorType: "AuthError",
		},
		{
			name: "request creation error",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						return nil, errors.New("request creation failed")
					},
				}
			},
			setupAuth: nil,
			wantError: true,
			errorType: "RequestError",
		},
		{
			name: "API error",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						return &pkgerrs.APIError{StatusCode: http.StatusForbidden, Message: "API error"}
					},
				}
			},
			setupAuth: nil,
			wantError: true,
			errorType: "APIError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var auth TokenProvider
			if tt.setupAuth != nil {
				auth = tt.setupAuth()
			}
			client := newTestClient(tt.setupMock(), auth)
			account, err := client.Me(context.Background())

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != "" {
					switch tt.errorType {
					case "AuthError":
						if _, ok := err.(*pkgerrs.AuthError); !ok {
							t.Errorf("expected AuthError, got %T: %v", err, err)
						}
					case "RequestError":
						if _, ok := err.(*pkgerrs.RequestError); !ok {
							t.Errorf("expected RequestError, got %T: %v", err, err)
						}
					case "APIError":
						if _, ok := err.(*pkgerrs.APIError); !ok {
							t.Errorf("expected APIError, got %T: %v", err, err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if account == nil {
					t.Error("expected account but got nil")
				}
			}
		})
	}
}

func TestClient_GetSubreddit(t *testing.T) {
	tests := []struct {
		name      string
		subreddit string
		setupMock func() HTTPClient
		wantError bool
		checkPath bool
		errorType string
	}{
		{
			name:      "successful request",
			subreddit: "golang",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						subredditData := `{"id":"sub123","display_name":"golang","subscribers":100000,"public_description":"Go programming"}`
						*v = types.Thing{
							Kind: "t5",
							Data: json.RawMessage(subredditData),
						}
						return nil
					},
				}
			},
			wantError: false,
			checkPath: true,
		},
		{
			name:      "API error",
			subreddit: "nonexistent",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						return errors.New("subreddit not found")
					},
				}
			},
			wantError: true,
		},
		{
			name:      "invalid subreddit name",
			subreddit: "ab",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name:      "request creation error",
			subreddit: "golang",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						return nil, errors.New("failed to create request")
					},
				}
			},
			wantError: true,
			errorType: "RequestError",
		},
		{
			name:      "unexpected response type",
			subreddit: "golang",
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						listingData := `{"children":[]}`
						*v = types.Thing{
							Kind: "Listing",
							Data: json.RawMessage(listingData),
						}
						return nil
					},
				}
			},
			wantError: true,
			errorType: "ParseError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			mock := tt.setupMock()
			if tt.checkPath {
				originalMock := mock.(*mockHTTPClient)
				originalDo := originalMock.doFunc
				originalMock.newRequestFunc = func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
					capturedPath = path
					req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
					return req, nil
				}
				originalMock.doFunc = originalDo
			}

			client := newTestClient(mock, nil)
			subreddit, err := client.GetSubreddit(context.Background(), tt.subreddit)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != "" {
					switch tt.errorType {
					case "ConfigError":
						if _, ok := err.(*pkgerrs.ConfigError); !ok {
							t.Errorf("expected ConfigError, got %T", err)
						}
					case "RequestError":
						if _, ok := err.(*pkgerrs.RequestError); !ok {
							t.Errorf("expected RequestError, got %T", err)
						}
					case "ParseError":
						if _, ok := err.(*pkgerrs.ParseError); !ok {
							t.Errorf("expected ParseError, got %T", err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if subreddit == nil {
					t.Error("expected subreddit but got nil")
				}
				if tt.checkPath && !strings.Contains(capturedPath, tt.subreddit) {
					t.Errorf("expected path to contain %s, got %s", tt.subreddit, capturedPath)
				}
			}
		})
	}
}

func TestClient_GetHot(t *testing.T) {
	tests := []struct {
		name       string
		request    *types.PostsRequest
		setupMock  func() HTTPClient
		wantError  bool
		wantPosts  int
		checkQuery bool
	}{
		{
			name: "successful request with subreddit",
			request: &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 5},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						children := make([]json.RawMessage, 3)
						for i := range children {
							postData := map[string]interface{}{
								"id":    "post" + string(rune('1'+i)),
								"title": "Test Post",
								"score": 100,
							}
							data, _ := json.Marshal(postData)
							child := map[string]interface{}{
								"kind": "t3",
								"data": json.RawMessage(data),
							}
							children[i], _ = json.Marshal(child)
						}
						listingData := map[string]interface{}{
							"after":    "t3_abc",
							"before":   "",
							"children": children,
						}
						data, _ := json.Marshal(listingData)
						*v = types.Thing{
							Kind: "Listing",
							Data: data,
						}
						return nil
					},
				}
			},
			wantError:  false,
			wantPosts:  3,
			checkQuery: true,
		},
		{
			name:    "nil request (front page)",
			request: nil,
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						listingData := `{"after":"","before":"","children":[]}`
						*v = types.Thing{
							Kind: "Listing",
							Data: json.RawMessage(listingData),
						}
						return nil
					},
				}
			},
			wantError: false,
			wantPosts: 0,
		},
		{
			name: "API error",
			request: &types.PostsRequest{
				Subreddit: "private",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						return errors.New("forbidden")
					},
				}
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedURL *url.URL
			mock := tt.setupMock()
			if tt.checkQuery {
				originalMock := mock.(*mockHTTPClient)
				originalDo := originalMock.doFunc
				originalMock.newRequestFunc = func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
					req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
					if len(params) > 0 && params[0] != nil {
						req.URL.RawQuery = params[0].Encode()
					}
					capturedURL = req.URL
					return req, nil
				}
				originalMock.doFunc = originalDo
			}

			client := newTestClient(mock, nil)
			posts, err := client.GetHot(context.Background(), tt.request)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if posts == nil {
					t.Error("expected posts response but got nil")
				} else if len(posts.Posts) != tt.wantPosts {
					t.Errorf("expected %d posts, got %d", tt.wantPosts, len(posts.Posts))
				}
				if tt.checkQuery && tt.request != nil && tt.request.Limit > 0 {
					if !strings.Contains(capturedURL.RawQuery, "limit=5") {
						t.Errorf("expected query to contain limit=5, got %s", capturedURL.RawQuery)
					}
				}
			}
		})
	}
}

func TestClient_getPostsErrors(t *testing.T) {
	tests := []struct {
		name        string
		request     *types.PostsRequest
		httpClient  HTTPClient
		auth        TokenProvider
		wantErrType string
	}{
		{
			name: "invalid subreddit",
			request: &types.PostsRequest{
				Subreddit: "ab",
			},
			httpClient:  &mockHTTPClient{},
			wantErrType: "ConfigError",
		},
		{
			name: "invalid pagination",
			request: &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{After: "t3_a", Before: "t3_b"},
			},
			httpClient:  &mockHTTPClient{},
			wantErrType: "ConfigError",
		},
		{
			name:    "request creation error",
			request: &types.PostsRequest{},
			httpClient: &mockHTTPClient{
				newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
					return nil, errors.New("boom")
				},
			},
			wantErrType: "RequestError",
		},
		{
			name:        "auth token error",
			request:     nil,
			httpClient:  &mockHTTPClient{},
			auth:        &mockTokenProvider{err: errors.New("token failure")},
			wantErrType: "AuthError",
		},
		{
			name:    "parse error",
			request: nil,
			httpClient: &mockHTTPClient{
				doFunc: func(req *http.Request, v *types.Thing) error {
					*v = types.Thing{Kind: "t3"}
					return nil
				},
			},
			wantErrType: "ParseError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(tt.httpClient, tt.auth)
			_, err := client.getPosts(context.Background(), tt.request, "hot")
			if err == nil {
				t.Fatal("expected error but got none")
			}
			switch tt.wantErrType {
			case "ConfigError":
				if _, ok := err.(*pkgerrs.ConfigError); !ok {
					t.Fatalf("expected ConfigError, got %T", err)
				}
			case "RequestError":
				if _, ok := err.(*pkgerrs.RequestError); !ok {
					t.Fatalf("expected RequestError, got %T", err)
				}
			case "AuthError":
				if _, ok := err.(*pkgerrs.AuthError); !ok {
					t.Fatalf("expected AuthError, got %T", err)
				}
			case "ParseError":
				if _, ok := err.(*pkgerrs.ParseError); !ok {
					t.Fatalf("expected ParseError, got %T", err)
				}
			default:
				t.Fatalf("unhandled error type %q", tt.wantErrType)
			}
		})
	}
}

func TestClient_GetNew(t *testing.T) {
	mock := &mockHTTPClient{
		doFunc: func(req *http.Request, v *types.Thing) error {
			listingData := `{"after":"","before":"","children":[]}`
			*v = types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(listingData),
			}
			return nil
		},
	}

	client := newTestClient(mock, nil)
	posts, err := client.GetNew(context.Background(), &types.PostsRequest{Subreddit: "golang"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if posts == nil {
		t.Error("expected posts response but got nil")
	}
}

func TestClient_GetComments(t *testing.T) {
	tests := []struct {
		name         string
		request      *types.CommentsRequest
		setupMock    func() HTTPClient
		setupAuth    func() TokenProvider
		wantError    bool
		errorType    string
		wantComments int
		wantMoreIDs  []string
	}{
		{
			name: "successful request",
			request: &types.CommentsRequest{
				Subreddit:  "golang",
				PostID:     "abc123",
				Pagination: types.Pagination{Limit: 5},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						// Post listing
						postData := `{"id":"abc123","title":"Test Post","score":100}`
						postChild := map[string]interface{}{
							"kind": "t3",
							"data": json.RawMessage(postData),
						}
						postChildJSON, _ := json.Marshal(postChild)
						postListing := map[string]interface{}{
							"children": []json.RawMessage{postChildJSON},
						}
						postListingData, _ := json.Marshal(postListing)

						// Comments listing
						commentData := `{"id":"com1","body":"Test comment","author":"user1","link_id":"t3_abc123","parent_id":"t3_abc123"}`
						commentChild := map[string]interface{}{
							"kind": "t1",
							"data": json.RawMessage(commentData),
						}
						commentChildJSON, _ := json.Marshal(commentChild)
						commentListing := map[string]interface{}{
							"children": []json.RawMessage{commentChildJSON},
						}
						commentListingData, _ := json.Marshal(commentListing)

						return []*types.Thing{
							{Kind: "Listing", Data: postListingData},
							{Kind: "Listing", Data: commentListingData},
						}, nil
					},
				}
			},
			wantError:    false,
			wantComments: 1,
		},
		{
			name: "captures nested more IDs",
			request: &types.CommentsRequest{
				Subreddit: "golang",
				PostID:    "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						postListingData, _ := json.Marshal(map[string]interface{}{
							"children": []interface{}{
								map[string]interface{}{
									"kind": "t3",
									"data": map[string]interface{}{
										"id":    "abc123",
										"title": "Test Post",
										"score": 100,
									},
								},
							},
						})

						commentListingData, _ := json.Marshal(map[string]interface{}{
							"children": []interface{}{
								map[string]interface{}{
									"kind": "t1",
									"data": map[string]interface{}{
										"id":        "c_nested",
										"body":      "Test comment",
										"author":    "user1",
										"link_id":   "t3_abc123",
										"parent_id": "t3_abc123",
										"replies": map[string]interface{}{
											"kind": "Listing",
											"data": map[string]interface{}{
												"children": []interface{}{
													map[string]interface{}{
														"kind": "more",
														"data": map[string]interface{}{
															"children": []string{"more1", "more2"},
														},
													},
												},
											},
										},
									},
								},
							},
						})

						return []*types.Thing{
							{Kind: "Listing", Data: postListingData},
							{Kind: "Listing", Data: commentListingData},
						}, nil
					},
				}
			},
			wantError:    false,
			wantComments: 1,
			wantMoreIDs:  []string{"more1", "more2"},
		},
		{
			name:    "nil request",
			request: nil,
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "missing subreddit",
			request: &types.CommentsRequest{
				PostID: "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "invalid subreddit",
			request: &types.CommentsRequest{
				Subreddit: "ab",
				PostID:    "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "missing post ID",
			request: &types.CommentsRequest{
				Subreddit: "golang",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "invalid pagination",
			request: &types.CommentsRequest{
				Subreddit:  "golang",
				PostID:     "abc123",
				Pagination: types.Pagination{After: "t1_a", Before: "t1_b"},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "API error",
			request: &types.CommentsRequest{
				Subreddit: "golang",
				PostID:    "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						return nil, &pkgerrs.APIError{StatusCode: http.StatusNotFound, Message: "post not found"}
					},
				}
			},
			wantError: true,
			errorType: "APIError",
		},
		{
			name: "request creation error",
			request: &types.CommentsRequest{
				Subreddit: "golang",
				PostID:    "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						return nil, errors.New("request failure")
					},
				}
			},
			wantError: true,
			errorType: "RequestError",
		},
		{
			name: "auth token error",
			request: &types.CommentsRequest{
				Subreddit: "golang",
				PostID:    "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			setupAuth: func() TokenProvider {
				return &mockTokenProvider{err: errors.New("token fail")}
			},
			wantError: true,
			errorType: "AuthError",
		},
		{
			name: "parse error",
			request: &types.CommentsRequest{
				Subreddit: "golang",
				PostID:    "abc123",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						return []*types.Thing{
							{Kind: "t3", Data: json.RawMessage(`{"id":"abc123"}`)},
						}, nil
					},
				}
			},
			wantError: true,
			errorType: "ParseError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var auth TokenProvider
			if tt.setupAuth != nil {
				auth = tt.setupAuth()
			}
			client := newTestClient(tt.setupMock(), auth)
			comments, err := client.GetComments(context.Background(), tt.request)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != "" {
					switch tt.errorType {
					case "ConfigError":
						if _, ok := err.(*pkgerrs.ConfigError); !ok {
							t.Errorf("expected ConfigError, got %T: %v", err, err)
						}
					case "RequestError":
						if _, ok := err.(*pkgerrs.RequestError); !ok {
							t.Errorf("expected RequestError, got %T: %v", err, err)
						}
					case "APIError":
						if _, ok := err.(*pkgerrs.APIError); !ok {
							t.Errorf("expected APIError, got %T: %v", err, err)
						}
					case "AuthError":
						if _, ok := err.(*pkgerrs.AuthError); !ok {
							t.Errorf("expected AuthError, got %T: %v", err, err)
						}
					case "ParseError":
						if _, ok := err.(*pkgerrs.ParseError); !ok {
							t.Errorf("expected ParseError, got %T: %v", err, err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if comments == nil {
					t.Error("expected comments response but got nil")
				} else if len(comments.Comments) != tt.wantComments {
					t.Errorf("expected %d comments, got %d", tt.wantComments, len(comments.Comments))
				}
				if tt.wantMoreIDs != nil {
					if !reflect.DeepEqual(comments.MoreIDs, tt.wantMoreIDs) {
						t.Errorf("expected more IDs %v, got %v", tt.wantMoreIDs, comments.MoreIDs)
					}
				}
			}
		})
	}
}

func TestClient_GetCommentsMultiple(t *testing.T) {
	tests := []struct {
		name      string
		requests  []*types.CommentsRequest
		setupMock func() HTTPClient
		wantError bool
		wantCount int
		errorType string
		ctxFunc   func() context.Context
	}{
		{
			name:      "empty requests",
			requests:  []*types.CommentsRequest{},
			setupMock: func() HTTPClient { return &mockHTTPClient{} },
			wantError: false,
			wantCount: 0,
		},
		{
			name: "nil request entry",
			requests: []*types.CommentsRequest{
				nil,
			},
			setupMock: func() HTTPClient { return &mockHTTPClient{} },
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "multiple successful requests",
			requests: []*types.CommentsRequest{
				{Subreddit: "golang", PostID: "post1"},
				{Subreddit: "golang", PostID: "post2"},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						postData := `{"id":"abc","title":"Test"}`
						postChild, _ := json.Marshal(map[string]interface{}{"kind": "t3", "data": json.RawMessage(postData)})
						postListing, _ := json.Marshal(map[string]interface{}{"children": []json.RawMessage{postChild}})

						commentData := `{"id":"c1","body":"Test","author":"u1","link_id":"t3_abc","parent_id":"t3_abc"}`
						commentChild, _ := json.Marshal(map[string]interface{}{"kind": "t1", "data": json.RawMessage(commentData)})
						commentListing, _ := json.Marshal(map[string]interface{}{"children": []json.RawMessage{commentChild}})

						return []*types.Thing{
							{Kind: "Listing", Data: postListing},
							{Kind: "Listing", Data: commentListing},
						}, nil
					},
				}
			},
			wantError: false,
			wantCount: 2,
		},
		{
			name: "one request fails",
			requests: []*types.CommentsRequest{
				{Subreddit: "golang", PostID: "post1"},
				{Subreddit: "golang", PostID: "invalid"},
			},
			setupMock: func() HTTPClient {
				var callCount atomic.Int32
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						count := callCount.Add(1)
						if count == 2 {
							return nil, errors.New("post not found")
						}
						postData := `{"id":"abc","title":"Test"}`
						postChild, _ := json.Marshal(map[string]interface{}{"kind": "t3", "data": json.RawMessage(postData)})
						postListing, _ := json.Marshal(map[string]interface{}{"children": []json.RawMessage{postChild}})
						commentListing, _ := json.Marshal(map[string]interface{}{"children": []json.RawMessage{}})

						return []*types.Thing{
							{Kind: "Listing", Data: postListing},
							{Kind: "Listing", Data: commentListing},
						}, nil
					},
				}
			},
			wantError: true,
			wantCount: 2, // Still returns all results, but with error
		},
		{
			name: "context cancelled",
			requests: []*types.CommentsRequest{
				{Subreddit: "golang", PostID: "post1"},
				{Subreddit: "golang", PostID: "post2"},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
						postListing, _ := json.Marshal(map[string]interface{}{"children": []interface{}{map[string]interface{}{"kind": "t3", "data": map[string]interface{}{"id": "abc"}}}})
						commentListing, _ := json.Marshal(map[string]interface{}{"children": []interface{}{map[string]interface{}{"kind": "t1", "data": map[string]interface{}{"id": "c1", "body": "Test", "author": "u1"}}}})
						return []*types.Thing{
							{Kind: "Listing", Data: postListing},
							{Kind: "Listing", Data: commentListing},
						}, nil
					},
				}
			},
			wantError: true,
			errorType: "ContextError",
			wantCount: 2,
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(tt.setupMock(), nil)

			ctx := context.Background()
			if tt.ctxFunc != nil {
				ctx = tt.ctxFunc()
			}

			results, err := client.GetCommentsMultiple(ctx, tt.requests)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != "" {
					switch tt.errorType {
					case "ConfigError":
						if _, ok := err.(*pkgerrs.ConfigError); !ok {
							t.Errorf("expected ConfigError, got %T", err)
						}
					case "ContextError":
						if !errors.Is(err, context.Canceled) {
							t.Errorf("expected context.Canceled, got %v", err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if len(results) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(results))
			}
		})
	}
}

func TestClient_GetMoreComments(t *testing.T) {
	tests := []struct {
		name         string
		request      *types.MoreCommentsRequest
		setupMock    func() HTTPClient
		setupAuth    func() TokenProvider
		wantError    bool
		errorType    string
		wantComments int
	}{
		{
			name: "successful request",
			request: &types.MoreCommentsRequest{
				LinkID:     "abc123",
				CommentIDs: []string{"comment1", "comment2"},
				Sort:       "best",
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					doMoreChildrenFunc: func(req *http.Request) ([]*types.Thing, error) {
						comment1 := `{"id":"comment1","body":"First comment","author":"user1","link_id":"t3_abc123","parent_id":"t3_abc123"}`
						comment2 := `{"id":"comment2","body":"Second comment","author":"user2","link_id":"t3_abc123","parent_id":"t3_abc123"}`
						return []*types.Thing{
							{Kind: "t1", Data: json.RawMessage(comment1)},
							{Kind: "t1", Data: json.RawMessage(comment2)},
						}, nil
					},
				}
			},
			wantError:    false,
			wantComments: 2,
		},
		{
			name:    "nil request",
			request: nil,
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "missing link ID",
			request: &types.MoreCommentsRequest{
				CommentIDs: []string{"comment1"},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "empty comment IDs",
			request: &types.MoreCommentsRequest{
				LinkID:     "abc123",
				CommentIDs: []string{},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{}
			},
			wantError:    false,
			wantComments: 0,
		},
		{
			name: "link ID without prefix",
			request: &types.MoreCommentsRequest{
				LinkID:     "abc123", // No t3_ prefix
				CommentIDs: []string{"comment1"},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						// Verify the body contains t3_ prefix
						if body != nil {
							bodyBytes, _ := io.ReadAll(body)
							bodyStr := string(bodyBytes)
							if !strings.Contains(bodyStr, "t3_abc123") {
								t.Error("expected body to contain t3_ prefix")
							}
						}
						req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
						return req, nil
					},
					doMoreChildrenFunc: func(req *http.Request) ([]*types.Thing, error) {
						return []*types.Thing{}, nil
					},
				}
			},
			wantError:    false,
			wantComments: 0,
		},
		{
			name: "with LimitChildren true",
			request: &types.MoreCommentsRequest{
				LinkID:        "abc123",
				CommentIDs:    []string{"comment1"},
				LimitChildren: true,
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						// Verify the body contains limit_children=true
						if body != nil {
							bodyBytes, _ := io.ReadAll(body)
							bodyStr := string(bodyBytes)
							if !strings.Contains(bodyStr, "limit_children=true") {
								t.Errorf("expected body to contain 'limit_children=true', got: %s", bodyStr)
							}
							// Verify it doesn't contain a numeric value
							if strings.Contains(bodyStr, "limit_children=1") || strings.Contains(bodyStr, "limit_children=10") {
								t.Errorf("limit_children should be boolean 'true', not a number, got: %s", bodyStr)
							}
						}
						req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
						return req, nil
					},
					doMoreChildrenFunc: func(req *http.Request) ([]*types.Thing, error) {
						return []*types.Thing{}, nil
					},
				}
			},
			wantError:    false,
			wantComments: 0,
		},
		{
			name: "with LimitChildren false",
			request: &types.MoreCommentsRequest{
				LinkID:        "abc123",
				CommentIDs:    []string{"comment1"},
				LimitChildren: false,
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						// Verify the body does NOT contain limit_children when false
						if body != nil {
							bodyBytes, _ := io.ReadAll(body)
							bodyStr := string(bodyBytes)
							if strings.Contains(bodyStr, "limit_children") {
								t.Errorf("expected body to NOT contain 'limit_children' when false, got: %s", bodyStr)
							}
						}
						req, _ := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
						return req, nil
					},
					doMoreChildrenFunc: func(req *http.Request) ([]*types.Thing, error) {
						return []*types.Thing{}, nil
					},
				}
			},
			wantError:    false,
			wantComments: 0,
		},
		{
			name: "invalid comment id",
			request: &types.MoreCommentsRequest{
				LinkID:     "t3_abc123",
				CommentIDs: []string{"bad!"},
			},
			setupMock: func() HTTPClient { return &mockHTTPClient{} },
			wantError: true,
			errorType: "ConfigError",
		},
		{
			name: "request creation failure",
			request: &types.MoreCommentsRequest{
				LinkID:     "t3_abc123",
				CommentIDs: []string{"comment1"},
			},
			setupMock: func() HTTPClient {
				return &mockHTTPClient{
					newRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
						return nil, errors.New("request failure")
					},
				}
			},
			wantError: true,
			errorType: "RequestError",
		},
		{
			name: "auth token failure",
			request: &types.MoreCommentsRequest{
				LinkID:     "t3_abc123",
				CommentIDs: []string{"comment1"},
			},
			setupMock: func() HTTPClient { return &mockHTTPClient{} },
			setupAuth: func() TokenProvider {
				return &mockTokenProvider{err: errors.New("token fail")}
			},
			wantError: true,
			errorType: "AuthError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var auth TokenProvider
			if tt.setupAuth != nil {
				auth = tt.setupAuth()
			}
			client := newTestClient(tt.setupMock(), auth)
			comments, err := client.GetMoreComments(context.Background(), tt.request)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != "" {
					switch tt.errorType {
					case "ConfigError":
						if _, ok := err.(*pkgerrs.ConfigError); !ok {
							t.Errorf("expected ConfigError, got %T: %v", err, err)
						}
					case "RequestError":
						if _, ok := err.(*pkgerrs.RequestError); !ok {
							t.Errorf("expected RequestError, got %T: %v", err, err)
						}
					case "AuthError":
						if _, ok := err.(*pkgerrs.AuthError); !ok {
							t.Errorf("expected AuthError, got %T: %v", err, err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(comments) != tt.wantComments {
					t.Errorf("expected %d comments, got %d", tt.wantComments, len(comments))
				}
			}
		})
	}
}

func TestBuildPaginationParams(t *testing.T) {
	tests := []struct {
		name       string
		pagination *types.Pagination
		wantLimit  string
		wantAfter  string
		wantBefore string
	}{
		{
			name:       "nil pagination",
			pagination: nil,
			wantLimit:  "",
			wantAfter:  "",
			wantBefore: "",
		},
		{
			name:       "empty pagination",
			pagination: &types.Pagination{},
			wantLimit:  "",
			wantAfter:  "",
			wantBefore: "",
		},
		{
			name: "with limit",
			pagination: &types.Pagination{
				Limit: 50,
			},
			wantLimit:  "50",
			wantAfter:  "",
			wantBefore: "",
		},
		{
			name: "with after",
			pagination: &types.Pagination{
				After: "t3_abc123",
			},
			wantLimit:  "",
			wantAfter:  "t3_abc123",
			wantBefore: "",
		},
		{
			name: "with before",
			pagination: &types.Pagination{
				Before: "t3_xyz789",
			},
			wantLimit:  "",
			wantAfter:  "",
			wantBefore: "t3_xyz789",
		},
		{
			name: "with all fields",
			pagination: &types.Pagination{
				Limit:  25,
				After:  "t3_abc123",
				Before: "t3_xyz789",
			},
			wantLimit:  "25",
			wantAfter:  "t3_abc123",
			wantBefore: "t3_xyz789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := buildPaginationParams(tt.pagination)

			if got := params.Get("limit"); got != tt.wantLimit {
				t.Errorf("limit = %q, want %q", got, tt.wantLimit)
			}
			if got := params.Get("after"); got != tt.wantAfter {
				t.Errorf("after = %q, want %q", got, tt.wantAfter)
			}
			if got := params.Get("before"); got != tt.wantBefore {
				t.Errorf("before = %q, want %q", got, tt.wantBefore)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	t.Run("ConfigError", func(t *testing.T) {
		err := &pkgerrs.ConfigError{Message: "test error"}
		if !strings.Contains(err.Error(), "test error") {
			t.Errorf("expected error message to contain 'test error', got %s", err.Error())
		}
	})

	t.Run("AuthError", func(t *testing.T) {
		err := &pkgerrs.AuthError{Message: "auth failed", Err: errors.New("underlying")}
		errStr := err.Error()
		if !strings.Contains(errStr, "auth failed") {
			t.Errorf("expected error message to contain 'auth failed', got %s", errStr)
		}
		// Check that underlying error is accessible via Unwrap
		if err.Unwrap() == nil {
			t.Error("expected Unwrap to return underlying error")
		}
		if err.Unwrap().Error() != "underlying" {
			t.Errorf("expected underlying error to be 'underlying', got %s", err.Unwrap().Error())
		}
	})

	t.Run("StateError", func(t *testing.T) {
		err := &pkgerrs.StateError{Message: "not connected"}
		if !strings.Contains(err.Error(), "not connected") {
			t.Errorf("expected error message to contain 'not connected', got %s", err.Error())
		}
	})

	t.Run("RequestError", func(t *testing.T) {
		err := &pkgerrs.RequestError{
			Operation: "get posts",
			URL:       "https://oauth.reddit.com/hot",
			Err:       errors.New("network error"),
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "get posts") {
			t.Errorf("expected error message to contain 'get posts', got %s", errStr)
		}
		if !strings.Contains(errStr, "https://oauth.reddit.com/hot") {
			t.Errorf("expected error message to contain URL, got %s", errStr)
		}
		if err.Unwrap() == nil {
			t.Error("expected Unwrap to return underlying error")
		}
	})

	t.Run("ParseError", func(t *testing.T) {
		err := &pkgerrs.ParseError{
			Operation: "parse posts",
			Err:       errors.New("invalid JSON"),
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "parse posts") {
			t.Errorf("expected error message to contain 'parse posts', got %s", errStr)
		}
		if err.Unwrap() == nil {
			t.Error("expected Unwrap to return underlying error")
		}
	})

	t.Run("APIError", func(t *testing.T) {
		err := &pkgerrs.APIError{
			ErrorCode: "403",
			Message:   "Forbidden",
			Details:   "private subreddit",
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "403") {
			t.Errorf("expected error message to contain '403', got %s", errStr)
		}
		if !strings.Contains(errStr, "Forbidden") {
			t.Errorf("expected error message to contain 'Forbidden', got %s", errStr)
		}
	})
}

func TestClient_addAuthHeaders(t *testing.T) {
	tests := []struct {
		name      string
		auth      TokenProvider
		wantError bool
	}{
		{
			name:      "successful auth",
			auth:      &mockTokenProvider{token: "test_token"},
			wantError: false,
		},
		{
			name:      "auth error",
			auth:      &mockTokenProvider{err: errors.New("token expired")},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Reddit{
				auth: tt.auth,
			}

			req, _ := http.NewRequest(http.MethodGet, "https://oauth.reddit.com/hot", nil)
			err := client.addAuthHeaders(context.Background(), req)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				authHeader := req.Header.Get("Authorization")
				if !strings.HasPrefix(authHeader, "Bearer ") {
					t.Errorf("expected Authorization header to start with 'Bearer ', got %s", authHeader)
				}
			}
		})
	}
}
