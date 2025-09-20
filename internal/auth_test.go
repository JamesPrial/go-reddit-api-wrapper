package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockResponse defines the response from the mock server.
type mockResponse struct {
	statusCode int
	body       string
}

// mockAuthServer is a mock HTTP server for testing the authenticator.
type mockAuthServer struct {
	t            *testing.T
	mockResponse *mockResponse
	grantType    string
	expectedUser string
	expectedPass string
	username     string
	password     string
}

// ServeHTTP handles incoming requests to the mock server.
func (s *mockAuthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.t.Errorf("expected POST request, got %s", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	user, pass, ok := r.BasicAuth()
	if !ok || user != s.expectedUser || pass != s.expectedPass {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "invalid_client"}`)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.t.Fatalf("failed to parse form: %v", err)
	}
	if r.Form.Get("grant_type") != s.grantType {
		s.t.Errorf("expected grant_type %q, got %q", s.grantType, r.Form.Get("grant_type"))
	}

	// Validate username and password if they are expected
	if s.username != "" {
		if r.Form.Get("username") != s.username {
			s.t.Errorf("expected username %q, got %q", s.username, r.Form.Get("username"))
		}
	}
	if s.password != "" {
		if r.Form.Get("password") != s.password {
			s.t.Errorf("expected password %q, got %q", s.password, r.Form.Get("password"))
		}
	}

	if s.mockResponse == nil {
		s.t.Error("mockResponse is nil but auth succeeded, this is likely a test setup error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(s.mockResponse.statusCode)
	fmt.Fprint(w, s.mockResponse.body)
}

func TestNewAuthenticator(t *testing.T) {
	t.Parallel()

	customClient := &http.Client{}

	testCases := []struct {
		name       string
		httpClient *http.Client
		baseURL    string
		tokenPath  string
		username   string
		password   string
		grantType  string
		wantErr    bool
		checkFunc  func(t *testing.T, a *Authenticator, err error)
	}{
		{
			name:       "success with nil client",
			httpClient: nil,
			baseURL:    "https://www.reddit.com/",
			tokenPath:  "api/v1/access_token",
			grantType:  "password",
			wantErr:    false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
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
			name:       "success with custom client",
			httpClient: customClient,
			baseURL:    "https://www.reddit.com/",
			grantType:  "password",
			wantErr:    false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				if a.client != customClient {
					t.Error("expected client to be the custom client")
				}
			},
		},
		{
			name:      "success with base url missing trailing slash",
			baseURL:   "https://www.reddit.com",
			tokenPath: "api/v1/access_token",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
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
			name:      "success with empty token path",
			baseURL:   "https://www.reddit.com/",
			tokenPath: "",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				expected := "https://www.reddit.com/" + defaultTokenEndpointPath
				if a.tokenURL.String() != expected {
					t.Errorf("expected tokenURL to be %q, got %q", expected, a.tokenURL.String())
				}
			},
		},
		{
			name:      "error with invalid base url",
			baseURL:   "::invalid-url",
			grantType: "password",
			wantErr:   true,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Errorf("expected AuthError, got %T", err)
				}
			},
		},
		{
			name:      "error with invalid token path",
			baseURL:   "https://www.reddit.com/",
			tokenPath: ":", // invalid path segment
			grantType: "password",
			wantErr:   true,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Errorf("expected AuthError, got %T", err)
				}
			},
		},
		{
			name:      "success with username and password",
			baseURL:   "https://www.reddit.com/",
			tokenPath: "",
			username:  "testuser",
			password:  "testpass",
			grantType: "password",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				// Check that form data contains username and password
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
			name:      "success with empty username and password",
			baseURL:   "https://www.reddit.com/",
			tokenPath: "",
			grantType: "client_credentials",
			wantErr:   false,
			checkFunc: func(t *testing.T, a *Authenticator, err error) {
				// Check that form data does not contain username and password when empty
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := NewAuthenticator(tc.httpClient, tc.username, tc.password, "id", "secret", "agent", tc.baseURL, tc.grantType, tc.tokenPath)

			if (err != nil) != tc.wantErr {
				t.Fatalf("NewAuthenticator() error = %v, wantErr %v", err, tc.wantErr)
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t, a, err)
			}
		})
	}
}

func TestAuthenticator_GetToken(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		clientID     string
		clientSecret string
		username     string
		password     string
		// expectedClientID and expectedClientSecret are for the mock server to expect.
		expectedClientID     string
		expectedClientSecret string
		mockResponse         *mockResponse
		serverDown           bool
		grantType            string
		expectedToken        string
		wantErr              bool
		checkErr             func(t *testing.T, err error)
	}{
		{
			name:                 "success",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			mockResponse: &mockResponse{
				statusCode: http.StatusOK,
				body:       `{"access_token": "test-token", "token_type": "bearer", "expires_in": 3600, "scope": "*"}`,
			},
			grantType:     "password",
			expectedToken: "test-token",
			wantErr:       false,
		},
		{
			name:                 "success with username and password",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			username:             "reddit_user",
			password:             "reddit_pass",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			mockResponse: &mockResponse{
				statusCode: http.StatusOK,
				body:       `{"access_token": "user-token", "token_type": "bearer", "expires_in": 3600, "scope": "*"}`,
			},
			grantType:     "password",
			expectedToken: "user-token",
			wantErr:       false,
		},
		{
			name:                 "invalid credentials",
			clientID:             "wrong-id",
			clientSecret:         "wrong-secret",
			expectedClientID:     "correct-id",
			expectedClientSecret: "correct-secret",
			mockResponse:         nil, // Not used as auth fails
			grantType:            "password",
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Fatalf("expected AuthError, got %T", err)
				}
				if authErr.StatusCode != http.StatusUnauthorized {
					t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, authErr.StatusCode)
				}
				if authErr.Body != `{"error": "invalid_client"}` {
					t.Errorf("unexpected body in error: %q", authErr.Body)
				}
			},
		},
		{
			name:                 "api error",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			mockResponse: &mockResponse{
				statusCode: http.StatusUnauthorized,
				body:       `{"error": "unauthorized"}`,
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Fatalf("expected AuthError, got %T", err)
				}
				if authErr.StatusCode != http.StatusUnauthorized {
					t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, authErr.StatusCode)
				}
				if authErr.Body != `{"error": "unauthorized"}` {
					t.Errorf("unexpected body in error: %q", authErr.Body)
				}
			},
		},
		{
			name:                 "network error",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			serverDown:           true,
			wantErr:              true,
			checkErr: func(t *testing.T, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Fatalf("expected AuthError, got %T", err)
				}
				if authErr.Err == nil {
					t.Error("expected underlying network error, but was nil")
				}
			},
		},
		{
			name:                 "bad json response",
			clientID:             "test-id",
			clientSecret:         "test-secret",
			expectedClientID:     "test-id",
			expectedClientSecret: "test-secret",
			mockResponse: &mockResponse{
				statusCode: http.StatusOK,
				body:       `{not-json}`,
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Fatalf("expected AuthError, got %T", err)
				}
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
			mockResponse: &mockResponse{
				statusCode: http.StatusOK,
				body:       `{"access_token": "", "token_type": "bearer"}`,
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				var authErr *AuthError
				if !errors.As(err, &authErr) {
					t.Fatalf("expected AuthError, got %T", err)
				}
				if !strings.Contains(authErr.Err.Error(), "access token was empty") {
					t.Errorf("expected error about empty access token, got %v", authErr.Err)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockServerHandler := &mockAuthServer{
				t:            t,
				mockResponse: tc.mockResponse,
				grantType:    tc.grantType,
				expectedUser: tc.expectedClientID,
				expectedPass: tc.expectedClientSecret,
				username:     tc.username,
				password:     tc.password,
			}

			server := httptest.NewServer(mockServerHandler)

			serverURL := server.URL
			if tc.serverDown {
				server.Close()
			} else {
				defer server.Close()
			}

			a, err := NewAuthenticator(server.Client(), tc.username, tc.password, tc.clientID, tc.clientSecret, "test-agent", serverURL, tc.grantType, "")
			if err != nil {
				t.Fatalf("failed to create authenticator: %v", err)
			}

			token, err := a.GetToken(context.Background())

			if (err != nil) != tc.wantErr {
				t.Fatalf("GetToken() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				if token != tc.expectedToken {
					t.Errorf("GetToken() token = %q, want %q", token, tc.expectedToken)
				}
			}

			if tc.wantErr && tc.checkErr != nil {
				tc.checkErr(t, err)
			}
		})
	}

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("server should not have been called")
		}))
		defer server.Close()

		a, err := NewAuthenticator(http.DefaultClient, "", "", "id", "secret", "agent", server.URL, "creds", "")
		if err != nil {
			t.Fatalf("failed to create authenticator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel context immediately

		_, err = a.GetToken(ctx)
		if err == nil {
			t.Fatal("expected an error for canceled context, got nil")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected error to be or wrap context.Canceled, got %v", err)
		}
	})
}

func TestAuthError_Error(t *testing.T) {
	t.Parallel()

	testErr := errors.New("underlying error")

	testCases := []struct {
		name     string
		err      AuthError
		expected string
	}{
		{
			name:     "full error",
			err:      AuthError{StatusCode: 401, Body: `{"error":"invalid"}`, Err: testErr},
			expected: `auth error: status code 401, body: "{\"error\":\"invalid\"}", err: underlying error`,
		},
		{
			name:     "status and body",
			err:      AuthError{StatusCode: 400, Body: "bad request"},
			expected: `auth error: status code 400, body: "bad request"`,
		},
		{
			name:     "status and err",
			err:      AuthError{StatusCode: 500, Err: testErr},
			expected: `auth error: status code 500, err: underlying error`,
		},
		{
			name:     "only status",
			err:      AuthError{StatusCode: 404},
			expected: "auth error: status code 404",
		},
		{
			name:     "only body",
			err:      AuthError{Body: "some body"},
			expected: `auth error, body: "some body"`,
		},
		{
			name:     "only err",
			err:      AuthError{Err: testErr},
			expected: "auth error, err: underlying error",
		},
		{
			name:     "empty error",
			err:      AuthError{},
			expected: "auth error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.expected {
				t.Errorf("Error() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	t.Parallel()

	baseErr := io.EOF
	authErr := &AuthError{Err: fmt.Errorf("wrapped: %w", baseErr)}

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

	emptyErr := &AuthError{}
	if errors.Unwrap(emptyErr) != nil {
		t.Error("Unwrap should return nil for an error with no inner Err")
	}
}
