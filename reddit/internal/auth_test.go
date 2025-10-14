package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
)

// oauthServerConfig holds configuration for the mock OAuth server.
type oauthServerConfig struct {
	expectedClientID     string
	expectedClientSecret string
	username             string
	password             string
	grantType            string
	statusCode           int
	responseBody         string
}

// mockOAuthServer creates a simple OAuth token endpoint server for auth testing.
// This is separate from testutil.MockServer since it handles OAuth-specific endpoints.
func mockOAuthServer(t *testing.T, config *oauthServerConfig) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Check basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != config.expectedClientID || pass != config.expectedClientSecret {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error": "invalid_client"}`)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		// Validate grant type
		if r.Form.Get("grant_type") != config.grantType {
			t.Errorf("expected grant_type %q, got %q", config.grantType, r.Form.Get("grant_type"))
		}

		// Validate username/password if provided
		if config.username != "" && r.Form.Get("username") != config.username {
			t.Errorf("expected username %q, got %q", config.username, r.Form.Get("username"))
		}
		if config.password != "" && r.Form.Get("password") != config.password {
			t.Errorf("expected password %q, got %q", config.password, r.Form.Get("password"))
		}

		// Return configured response
		w.WriteHeader(config.statusCode)
		fmt.Fprint(w, config.responseBody)
	}))
}

func TestNewAuthenticator(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{}

	tests := []struct {
		name      string
		client    *http.Client
		baseURL   string
		username  string
		password  string
		grantType string
		wantErr   bool
		checkFunc func(t *testing.T, a *Authenticator, err error)
	}{
		{
			name:      "success with nil client uses default",
			client:    nil,
			baseURL:   "https://www.reddit.com/",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				t.Helper()
				if a.client != http.DefaultClient {
					t.Error("expected client to be http.DefaultClient")
				}
				expectedURL := "https://www.reddit.com/api/v1/access_token"
				if a.tokenURL.String() != expectedURL {
					t.Errorf("expected tokenURL %q, got %q", expectedURL, a.tokenURL.String())
				}
			},
		},
		{
			name:      "success with custom client",
			client:    customClient,
			baseURL:   "https://www.reddit.com/",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				t.Helper()
				if a.client != customClient {
					t.Error("expected client to be the custom client")
				}
			},
		},
		{
			name:      "adds trailing slash to base URL",
			baseURL:   "https://www.reddit.com",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				t.Helper()
				if a.BaseURL.String() != "https://www.reddit.com/" {
					t.Errorf("expected base URL to have trailing slash, got %q", a.BaseURL.String())
				}
				expectedURL := "https://www.reddit.com/api/v1/access_token"
				if a.tokenURL.String() != expectedURL {
					t.Errorf("expected tokenURL %q, got %q", expectedURL, a.tokenURL.String())
				}
			},
		},
		{
			name:      "error with invalid base URL",
			baseURL:   "::invalid-url",
			grantType: "password",
			wantErr:   true,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
			},
		},
		{
			name:      "stores username and password for user auth",
			baseURL:   "https://www.reddit.com/",
			username:  "testuser",
			password:  "testpass",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				t.Helper()
				if a.formData.Get("username") != "testuser" {
					t.Errorf("expected username 'testuser', got %q", a.formData.Get("username"))
				}
				if a.formData.Get("password") != "testpass" {
					t.Errorf("expected password 'testpass', got %q", a.formData.Get("password"))
				}
				if a.formData.Get("grant_type") != "password" {
					t.Errorf("expected grant_type 'password', got %q", a.formData.Get("grant_type"))
				}
			},
		},
		{
			name:      "client credentials with no username/password",
			baseURL:   "https://www.reddit.com/",
			grantType: "client_credentials",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				t.Helper()
				if a.formData.Get("username") != "" {
					t.Errorf("expected empty username, got %q", a.formData.Get("username"))
				}
				if a.formData.Get("password") != "" {
					t.Errorf("expected empty password, got %q", a.formData.Get("password"))
				}
				if a.formData.Get("grant_type") != "client_credentials" {
					t.Errorf("expected grant_type 'client_credentials', got %q", a.formData.Get("grant_type"))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a, err := NewAuthenticator(
				tt.client,
				tt.username,
				tt.password,
				"id",
				"secret",
				"agent",
				tt.baseURL,
				tt.grantType,
				nil,
				nil, // Use real clock
			)

			if tt.wantErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, a, err)
			}
		})
	}
}

func TestAuthenticator_GetToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		clientID             string
		clientSecret         string
		username             string
		password             string
		expectedClientID     string
		expectedClientSecret string
		grantType            string
		statusCode           int
		responseBody         string
		serverDown           bool
		expectedToken        string
		wantErr              bool
		checkErr             func(t *testing.T, err error)
		logger               *slog.Logger
	}{
		{
			name:                 "success with valid credentials",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         `{"access_token": "test-token", "token_type": "bearer", "expires_in": 3600, "scope": "*"}`,
			expectedToken:        "test-token",
			wantErr:              false,
			logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
		{
			name:                 "success with username and password",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			username:             "reddit_user",
			password:             "reddit_pass",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         `{"access_token": "user-token", "token_type": "bearer", "expires_in": 3600, "scope": "*"}`,
			expectedToken:        "user-token",
			wantErr:              false,
		},
		{
			name:                 "invalid credentials return auth error",
			clientID:             "wrong-id",
			clientSecret:         "wrong-secret",
			expectedClientID:     "correct-id",
			expectedClientSecret: "correct-secret",
			grantType:            "password",
			statusCode:           0, // Not used - auth fails before response
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				if authErr.StatusCode != http.StatusUnauthorized {
					t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, authErr.StatusCode)
				}
				testutil.AssertStringContains(t, authErr.Body, "invalid_client")
			},
		},
		{
			name:                 "API error with 401 status",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusUnauthorized,
			responseBody:         `{"error": "unauthorized"}`,
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				if authErr.StatusCode != http.StatusUnauthorized {
					t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, authErr.StatusCode)
				}
				testutil.AssertStringContains(t, authErr.Body, "unauthorized")
			},
		},
		{
			name:                 "network error when server down",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			serverDown:           true,
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				if authErr.Err == nil {
					t.Error("expected underlying network error, but was nil")
				}
			},
		},
		{
			name:                 "bad JSON response",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         `{not-json}`,
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				var jsonErr *json.SyntaxError
				if !errors.As(err, &jsonErr) {
					t.Errorf("expected underlying error to be json.SyntaxError, got %T", errors.Unwrap(err))
				}
			},
		},
		{
			name:                 "empty access token in response",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         `{"access_token": "", "token_type": "bearer"}`,
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				testutil.AssertStringContains(t, err.Error(), "access token was empty")
			},
		},
		{
			name:                 "negative expires_in value",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         `{"access_token": "tok", "token_type": "bearer", "expires_in": -10}`,
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				testutil.AssertStringContains(t, err.Error(), "cannot be negative")
			},
		},
		{
			name:                 "expires_in exceeds maximum allowed",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         fmt.Sprintf(`{"access_token": "tok", "token_type": "bearer", "expires_in": %d}`, 400*24*60*60),
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				testutil.AssertStringContains(t, err.Error(), "exceeds maximum")
			},
		},
		{
			name:                 "response body exceeds max size",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			grantType:            "password",
			statusCode:           http.StatusOK,
			responseBody:         strings.Repeat("a", maxResponseBodySize+1),
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var authErr *pkgerrs.AuthError
				testutil.AssertErrorType(t, err, &authErr)
				testutil.AssertStringContains(t, err.Error(), "exceeded max size")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &oauthServerConfig{
				expectedClientID:     tt.expectedClientID,
				expectedClientSecret: tt.expectedClientSecret,
				username:             tt.username,
				password:             tt.password,
				grantType:            tt.grantType,
				statusCode:           tt.statusCode,
				responseBody:         tt.responseBody,
			}

			server := mockOAuthServer(t, config)
			serverURL := server.URL
			if tt.serverDown {
				server.Close()
			} else {
				defer server.Close()
			}

			a, err := NewAuthenticator(
				server.Client(),
				tt.username,
				tt.password,
				tt.clientID,
				tt.clientSecret,
				"test-agent",
				serverURL,
				tt.grantType,
				tt.logger,
				nil, // Use real clock
			)
			testutil.AssertNoError(t, err)

			token, err := a.GetToken(context.Background())

			if tt.wantErr {
				testutil.AssertError(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				testutil.AssertNoError(t, err)
				if token != tt.expectedToken {
					t.Errorf("GetToken() token = %q, want %q", token, tt.expectedToken)
				}
			}
		})
	}

	t.Run("context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("server should not have been called")
		}))
		defer server.Close()

		a, err := NewAuthenticator(http.DefaultClient, "", "", "id", "secret", "agent", server.URL, "creds", nil, nil) // Use real clock
		testutil.AssertNoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately

		_, err = a.GetToken(ctx)
		testutil.AssertError(t, err)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected error to be or wrap context.Canceled, got %v", err)
		}
	})
}

func TestAuthError_Error(t *testing.T) {
	t.Parallel()

	testErr := errors.New("underlying error")

	tests := []struct {
		name     string
		err      pkgerrs.AuthError
		expected string
	}{
		{
			name:     "full error with status, body, and underlying error",
			err:      pkgerrs.AuthError{StatusCode: 401, Body: `{"error":"invalid"}`, Err: testErr},
			expected: `auth error: status code 401, body: "{\"error\":\"invalid\"}", err: underlying error`,
		},
		{
			name:     "status and body only",
			err:      pkgerrs.AuthError{StatusCode: 400, Body: "bad request"},
			expected: `auth error: status code 400, body: "bad request"`,
		},
		{
			name:     "status and underlying error",
			err:      pkgerrs.AuthError{StatusCode: 500, Err: testErr},
			expected: `auth error: status code 500, err: underlying error`,
		},
		{
			name:     "status code only",
			err:      pkgerrs.AuthError{StatusCode: 404},
			expected: "auth error: status code 404",
		},
		{
			name:     "body only",
			err:      pkgerrs.AuthError{Body: "some body"},
			expected: `auth error, body: "some body"`,
		},
		{
			name:     "underlying error only",
			err:      pkgerrs.AuthError{Err: testErr},
			expected: "auth error, err: underlying error",
		},
		{
			name:     "empty error with no fields",
			err:      pkgerrs.AuthError{},
			expected: "auth error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("unwraps nested error correctly", func(t *testing.T) {
		t.Parallel()

		baseErr := io.EOF
		authErr := &pkgerrs.AuthError{Err: fmt.Errorf("wrapped: %w", baseErr)}

		if !errors.Is(authErr, baseErr) {
			t.Errorf("errors.Is failed, expected to find %v in %v", baseErr, authErr)
		}

		unwrapped := errors.Unwrap(authErr)
		if unwrapped == nil {
			t.Fatal("Unwrap() returned nil")
		}

		if !errors.Is(unwrapped, baseErr) {
			t.Errorf("unwrapped error is not the base error")
		}
	})

	t.Run("returns nil for error with no inner Err", func(t *testing.T) {
		t.Parallel()

		emptyErr := &pkgerrs.AuthError{}
		if errors.Unwrap(emptyErr) != nil {
			t.Error("Unwrap should return nil for an error with no inner Err")
		}
	})
}
