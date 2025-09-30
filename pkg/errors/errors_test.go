package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ConfigError
		contains []string
	}{
		{
			name: "with field and message",
			err: ConfigError{
				Field:   "username",
				Message: "cannot be empty",
			},
			contains: []string{"config error", "username", "cannot be empty"},
		},
		{
			name: "only message",
			err: ConfigError{
				Message: "invalid configuration",
			},
			contains: []string{"config error", "invalid configuration"},
		},
		{
			name: "empty error",
			err:  ConfigError{},
			contains: []string{"config error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("ConfigError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestAuthError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      AuthError
		contains []string
	}{
		{
			name: "full error with all fields",
			err: AuthError{
				StatusCode: 401,
				Message:    "unauthorized",
				Body:       `{"error": "invalid_grant"}`,
				Err:        errors.New("connection failed"),
			},
			contains: []string{"auth error", "401", "unauthorized", "invalid_grant", "connection failed"},
		},
		{
			name: "only status code and message",
			err: AuthError{
				StatusCode: 403,
				Message:    "forbidden",
			},
			contains: []string{"auth error", "403", "forbidden"},
		},
		{
			name: "only error",
			err: AuthError{
				Err: errors.New("network error"),
			},
			contains: []string{"auth error", "network error"},
		},
		{
			name: "only body (legacy format)",
			err: AuthError{
				Body: `{"error": "invalid_token"}`,
			},
			contains: []string{"auth error", "body", "invalid_token"},
		},
		{
			name: "empty error",
			err:  AuthError{},
			contains: []string{"auth error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("AuthError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &AuthError{Err: innerErr}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("AuthError.Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	nilErr := &AuthError{}
	if unwrapped := nilErr.Unwrap(); unwrapped != nil {
		t.Errorf("AuthError.Unwrap() with nil Err = %v, want nil", unwrapped)
	}
}

func TestStateError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      StateError
		contains []string
	}{
		{
			name: "with operation and message",
			err: StateError{
				Operation: "GetPosts",
				Message:   "client not authenticated",
			},
			contains: []string{"state error", "GetPosts", "client not authenticated"},
		},
		{
			name: "only message",
			err: StateError{
				Message: "invalid state",
			},
			contains: []string{"state error", "invalid state"},
		},
		{
			name: "empty error",
			err:  StateError{},
			contains: []string{"state error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("StateError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestRequestError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      RequestError
		contains []string
	}{
		{
			name: "with operation, URL, and message",
			err: RequestError{
				Operation: "GetPosts",
				URL:       "https://oauth.reddit.com/r/golang/hot",
				Message:   "request failed",
			},
			contains: []string{"request error", "GetPosts", "https://oauth.reddit.com/r/golang/hot", "request failed"},
		},
		{
			name: "with operation and message",
			err: RequestError{
				Operation: "GetPosts",
				Message:   "request failed",
			},
			contains: []string{"request error", "GetPosts", "request failed"},
		},
		{
			name: "only message",
			err: RequestError{
				Message: "request failed",
			},
			contains: []string{"request error", "request failed"},
		},
		{
			name: "empty error",
			err:  RequestError{},
			contains: []string{"request error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("RequestError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestRequestError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &RequestError{Err: innerErr}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("RequestError.Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	nilErr := &RequestError{}
	if unwrapped := nilErr.Unwrap(); unwrapped != nil {
		t.Errorf("RequestError.Unwrap() with nil Err = %v, want nil", unwrapped)
	}
}

func TestParseError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ParseError
		contains []string
	}{
		{
			name: "with operation and message",
			err: ParseError{
				Operation: "ParseComment",
				Message:   "invalid JSON",
			},
			contains: []string{"parse error", "ParseComment", "invalid JSON"},
		},
		{
			name: "only message",
			err: ParseError{
				Message: "invalid JSON",
			},
			contains: []string{"parse error", "invalid JSON"},
		},
		{
			name: "empty error",
			err:  ParseError{},
			contains: []string{"parse error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("ParseError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestParseError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &ParseError{Err: innerErr}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("ParseError.Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	nilErr := &ParseError{}
	if unwrapped := nilErr.Unwrap(); unwrapped != nil {
		t.Errorf("ParseError.Unwrap() with nil Err = %v, want nil", unwrapped)
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      APIError
		contains []string
	}{
		{
			name: "with error code",
			err: APIError{
				StatusCode: 403,
				ErrorCode:  "SUBREDDIT_NOEXIST",
				Message:    "that subreddit doesn't exist",
			},
			contains: []string{"reddit API error", "403", "SUBREDDIT_NOEXIST", "that subreddit doesn't exist"},
		},
		{
			name: "without error code (legacy format)",
			err: APIError{
				StatusCode: 500,
				Message:    "internal server error",
			},
			contains: []string{"API request failed", "500", "internal server error"},
		},
		{
			name: "empty message",
			err: APIError{
				StatusCode: 404,
			},
			contains: []string{"API request failed", "404"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("APIError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestClientError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      ClientError
		contains []string
	}{
		{
			name: "only inner error (backward compatibility)",
			err: ClientError{
				Err: errors.New("connection refused"),
			},
			contains: []string{"connection refused"},
		},
		{
			name: "with operation and message",
			err: ClientError{
				Operation: "Do",
				Message:   "request failed",
			},
			contains: []string{"client error", "Do", "request failed"},
		},
		{
			name: "with operation only",
			err: ClientError{
				Operation: "Do",
			},
			contains: []string{"client error", "Do"},
		},
		{
			name: "with message only",
			err: ClientError{
				Message: "request failed",
			},
			contains: []string{"client error", "request failed"},
		},
		{
			name: "with all fields",
			err: ClientError{
				Operation: "Do",
				Message:   "request failed",
				Err:       errors.New("timeout"),
			},
			contains: []string{"client error", "timeout"},
		},
		{
			name: "empty error",
			err:  ClientError{},
			contains: []string{"client error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("ClientError.Error() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func TestClientError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	err := &ClientError{Err: innerErr}

	if unwrapped := err.Unwrap(); unwrapped != innerErr {
		t.Errorf("ClientError.Unwrap() = %v, want %v", unwrapped, innerErr)
	}

	nilErr := &ClientError{}
	if unwrapped := nilErr.Unwrap(); unwrapped != nil {
		t.Errorf("ClientError.Unwrap() with nil Err = %v, want nil", unwrapped)
	}
}

func TestErrorChaining(t *testing.T) {
	// Test that errors.Is and errors.As work with our error types
	rootErr := errors.New("root cause")

	authErr := &AuthError{Err: rootErr}
	if !errors.Is(authErr, rootErr) {
		t.Error("AuthError should wrap root error for errors.Is")
	}

	requestErr := &RequestError{Err: rootErr}
	if !errors.Is(requestErr, rootErr) {
		t.Error("RequestError should wrap root error for errors.Is")
	}

	parseErr := &ParseError{Err: rootErr}
	if !errors.Is(parseErr, rootErr) {
		t.Error("ParseError should wrap root error for errors.Is")
	}

	clientErr := &ClientError{Err: rootErr}
	if !errors.Is(clientErr, rootErr) {
		t.Error("ClientError should wrap root error for errors.Is")
	}
}

func TestErrorTypeAssertion(t *testing.T) {
	// Test that errors.As works with our error types
	t.Run("AuthError", func(t *testing.T) {
		err := &AuthError{StatusCode: 401}
		var target *AuthError
		if !errors.As(err, &target) {
			t.Error("errors.As should find AuthError")
		}
		if target.StatusCode != 401 {
			t.Errorf("target.StatusCode = %d, want 401", target.StatusCode)
		}
	})

	t.Run("RequestError", func(t *testing.T) {
		err := &RequestError{Operation: "test"}
		var target *RequestError
		if !errors.As(err, &target) {
			t.Error("errors.As should find RequestError")
		}
		if target.Operation != "test" {
			t.Errorf("target.Operation = %q, want %q", target.Operation, "test")
		}
	})

	t.Run("ParseError", func(t *testing.T) {
		err := &ParseError{Operation: "test"}
		var target *ParseError
		if !errors.As(err, &target) {
			t.Error("errors.As should find ParseError")
		}
		if target.Operation != "test" {
			t.Errorf("target.Operation = %q, want %q", target.Operation, "test")
		}
	})

	t.Run("ClientError", func(t *testing.T) {
		err := &ClientError{Message: "test"}
		var target *ClientError
		if !errors.As(err, &target) {
			t.Error("errors.As should find ClientError")
		}
		if target.Message != "test" {
			t.Errorf("target.Message = %q, want %q", target.Message, "test")
		}
	})

	t.Run("APIError", func(t *testing.T) {
		err := &APIError{StatusCode: 404}
		var target *APIError
		if !errors.As(err, &target) {
			t.Error("errors.As should find APIError")
		}
		if target.StatusCode != 404 {
			t.Errorf("target.StatusCode = %d, want 404", target.StatusCode)
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		err := &ConfigError{Field: "test"}
		var target *ConfigError
		if !errors.As(err, &target) {
			t.Error("errors.As should find ConfigError")
		}
		if target.Field != "test" {
			t.Errorf("target.Field = %q, want %q", target.Field, "test")
		}
	})

	t.Run("StateError", func(t *testing.T) {
		err := &StateError{Operation: "test"}
		var target *StateError
		if !errors.As(err, &target) {
			t.Error("errors.As should find StateError")
		}
		if target.Operation != "test" {
			t.Errorf("target.Operation = %q, want %q", target.Operation, "test")
		}
	})
}
