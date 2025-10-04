package internal

import (
	"strings"
	"testing"

	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestValidator_ValidateSubredditName(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		input     string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{name: "valid minimum length", input: "abc", wantError: false},
		{name: "valid maximum length", input: "abcdefghijklmnopqrstu", wantError: false},
		{name: "valid with numbers", input: "test123", wantError: false},
		{name: "valid with underscore", input: "test_sub", wantError: false},
		{name: "valid mixed case", input: "TestSub", wantError: false},

		// Invalid cases - empty and length
		{name: "empty string", input: "", wantError: true, errorMsg: "cannot be empty"},
		{name: "too short", input: "ab", wantError: true, errorMsg: "at least 3 characters"},
		{name: "too long", input: "abcdefghijklmnopqrstuv", wantError: true, errorMsg: "cannot exceed 21 characters"},

		// Invalid cases - underscore rules
		{name: "starts with underscore", input: "_test", wantError: true, errorMsg: "cannot start or end with underscore"},
		{name: "ends with underscore", input: "test_", wantError: true, errorMsg: "cannot start or end with underscore"},
		{name: "consecutive underscores", input: "test__sub", wantError: true, errorMsg: "cannot contain consecutive underscores"},

		// Invalid cases - special characters
		{name: "contains dash", input: "test-sub", wantError: true, errorMsg: "invalid character"},
		{name: "contains space", input: "test sub", wantError: true, errorMsg: "invalid character"},
		{name: "contains dot", input: "test.sub", wantError: true, errorMsg: "invalid character"},
		{name: "contains slash", input: "test/sub", wantError: true, errorMsg: "invalid character"},
		{name: "contains special char", input: "test@sub", wantError: true, errorMsg: "invalid character"},
		{name: "contains newline", input: "test\nsub", wantError: true, errorMsg: "invalid character"},
		{name: "contains unicode", input: "test™", wantError: true, errorMsg: "invalid character"},
		{name: "SQL injection attempt", input: "'; DROP TABLE--", wantError: true, errorMsg: "invalid character"},
		{name: "path traversal", input: "../etc", wantError: true, errorMsg: "invalid character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateSubredditName(tt.input)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				// Verify it's a ConfigError
				if _, ok := err.(*pkgerrs.ConfigError); !ok {
					t.Errorf("expected *pkgerrs.ConfigError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidatePagination(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name       string
		pagination *types.Pagination
		wantError  bool
		errorMsg   string
	}{
		// Valid cases
		{name: "nil pagination", pagination: nil, wantError: false},
		{name: "empty pagination", pagination: &types.Pagination{}, wantError: false},
		{name: "valid limit", pagination: &types.Pagination{Limit: 25}, wantError: false},
		{name: "max limit", pagination: &types.Pagination{Limit: 100}, wantError: false},
		{name: "with after", pagination: &types.Pagination{Limit: 25, After: "t3_abc123"}, wantError: false},
		{name: "with before", pagination: &types.Pagination{Limit: 25, Before: "t3_xyz789"}, wantError: false},

		// Invalid cases
		{name: "negative limit", pagination: &types.Pagination{Limit: -1}, wantError: true, errorMsg: "cannot be negative"},
		{name: "limit too high", pagination: &types.Pagination{Limit: 101}, wantError: true, errorMsg: "cannot exceed 100"},
		{name: "both after and before", pagination: &types.Pagination{After: "t3_abc", Before: "t3_xyz"}, wantError: true, errorMsg: "cannot set both"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidatePagination(tt.pagination)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				// Verify it's a ConfigError
				if _, ok := err.(*pkgerrs.ConfigError); !ok {
					t.Errorf("expected *pkgerrs.ConfigError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateCommentIDs(t *testing.T) {
	v := NewValidator()

	// Helper to create a slice of n valid IDs
	makeIDs := func(n int) []string {
		ids := make([]string, n)
		for i := 0; i < n; i++ {
			ids[i] = "abc123"
		}
		return ids
	}

	tests := []struct {
		name      string
		ids       []string
		wantError bool
		errorMsg  string
	}{
		// Valid cases (base36 is lowercase only)
		{name: "empty slice", ids: []string{}, wantError: false},
		{name: "single valid ID", ids: []string{"abc123"}, wantError: false},
		{name: "multiple valid IDs", ids: []string{"abc123", "def456", "ghi789"}, wantError: false},
		{name: "max count", ids: makeIDs(100), wantError: false},

		// Invalid cases - count
		{name: "too many IDs", ids: makeIDs(101), wantError: true, errorMsg: "cannot request more than 100"},

		// Invalid cases - ID format
		{name: "empty ID", ids: []string{""}, wantError: true, errorMsg: "cannot be empty"},
		{name: "mixed case IDs", ids: []string{"AbC123", "XyZ789"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID with space", ids: []string{"abc 123"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID with dash", ids: []string{"abc-123"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID with underscore", ids: []string{"abc_123"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID with special char", ids: []string{"abc@123"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID with slash", ids: []string{"abc/123"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID with newline", ids: []string{"abc\n123"}, wantError: true, errorMsg: "invalid format"},
		{name: "ID too long", ids: []string{strings.Repeat("a", 101)}, wantError: true, errorMsg: "too long"},
		{name: "SQL injection", ids: []string{"'; DROP TABLE--"}, wantError: true, errorMsg: "invalid format"},
		{name: "path traversal", ids: []string{"../etc"}, wantError: true, errorMsg: "invalid format"},

		// Invalid cases - mixed
		{name: "one valid one invalid", ids: []string{"abc123", "invalid!"}, wantError: true, errorMsg: "invalid format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCommentIDs(tt.ids)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				// Verify it's a ConfigError
				if _, ok := err.(*pkgerrs.ConfigError); !ok {
					t.Errorf("expected *pkgerrs.ConfigError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateUserAgent(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		ua        string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{name: "valid simple", ua: "myapp/1.0", wantError: false},
		{name: "valid with username", ua: "web:myapp:1.0 by /u/myuser", wantError: false},
		{name: "valid max length", ua: strings.Repeat("a", 256), wantError: false},

		// Invalid cases
		{name: "empty", ua: "", wantError: true, errorMsg: "cannot be empty"},
		{name: "too long", ua: strings.Repeat("a", 257), wantError: true, errorMsg: "too long"},
		{name: "contains newline", ua: "myapp/1.0\nX-Injected-Header: bad", wantError: true, errorMsg: "cannot contain newline"},
		{name: "contains carriage return", ua: "myapp/1.0\rX-Injected: bad", wantError: true, errorMsg: "cannot contain newline"},
		{name: "header injection attempt", ua: "myapp/1.0\r\nAuthorization: Bearer stolen", wantError: true, errorMsg: "cannot contain newline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateUserAgent(tt.ua)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateLinkID(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name             string
		linkID           string
		wantNormalized   string
		wantError        bool
		errorMsg         string
	}{
		// Valid cases - no prefix
		{name: "valid without prefix", linkID: "abc123", wantNormalized: "t3_abc123", wantError: false},
		{name: "valid alphanumeric", linkID: "xyz789", wantNormalized: "t3_xyz789", wantError: false},

		// Valid cases - with correct prefix
		{name: "valid with t3_ prefix", linkID: "t3_abc123", wantNormalized: "t3_abc123", wantError: false},
		{name: "valid long ID with prefix", linkID: "t3_abcdefghij", wantNormalized: "t3_abcdefghij", wantError: false},

		// Invalid cases - empty
		{name: "empty string", linkID: "", wantError: true, errorMsg: "link ID is required"},

		// Invalid cases - wrong prefix
		{name: "wrong prefix t1_", linkID: "t1_abc123", wantError: true, errorMsg: "wrong type prefix"},
		{name: "wrong prefix t2_", linkID: "t2_abc123", wantError: true, errorMsg: "wrong type prefix"},
		{name: "wrong prefix t4_", linkID: "t4_abc123", wantError: true, errorMsg: "wrong type prefix"},
		{name: "wrong prefix t5_", linkID: "t5_abc123", wantError: true, errorMsg: "wrong type prefix"},

		// Invalid cases - malformed
		{name: "t3_ prefix but no content", linkID: "t3_", wantError: true, errorMsg: "no content after"},
		{name: "just t3_", linkID: "t3_", wantError: true, errorMsg: "no content after"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := v.ValidateLinkID(tt.linkID)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				// Verify it's a ConfigError
				if _, ok := err.(*pkgerrs.ConfigError); !ok {
					t.Errorf("expected *pkgerrs.ConfigError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if normalized != tt.wantNormalized {
					t.Errorf("expected normalized %q, got %q", tt.wantNormalized, normalized)
				}
			}
		})
	}
}

func TestValidateCommentID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantError bool
		errorMsg  string
	}{
		// Valid cases (base36 is lowercase only)
		{name: "valid lowercase", id: "abc123", wantError: false},
		{name: "valid all numbers", id: "123456", wantError: false},
		{name: "valid all letters", id: "abcdef", wantError: false},
		{name: "valid max length", id: strings.Repeat("a", 100), wantError: false},

		// Invalid cases
		{name: "empty", id: "", wantError: true, errorMsg: "cannot be empty"},
		{name: "too long", id: strings.Repeat("a", 101), wantError: true, errorMsg: "too long"},
		{name: "uppercase", id: "ABC123", wantError: true, errorMsg: "invalid format"},
		{name: "mixed case", id: "AbC123", wantError: true, errorMsg: "invalid format"},
		{name: "with space", id: "abc 123", wantError: true, errorMsg: "invalid format"},
		{name: "with underscore", id: "abc_123", wantError: true, errorMsg: "invalid format"},
		{name: "with dash", id: "abc-123", wantError: true, errorMsg: "invalid format"},
		{name: "with dot", id: "abc.123", wantError: true, errorMsg: "invalid format"},
		{name: "with special char", id: "abc!123", wantError: true, errorMsg: "invalid format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCommentID(tt.id)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidator_ValidateSubredditName(b *testing.B) {
	v := NewValidator()
	for i := 0; i < b.N; i++ {
		_ = v.ValidateSubredditName("golang")
	}
}

func BenchmarkValidator_ValidatePagination(b *testing.B) {
	v := NewValidator()
	p := &types.Pagination{Limit: 25, After: "t3_abc123"}
	for i := 0; i < b.N; i++ {
		_ = v.ValidatePagination(p)
	}
}

func BenchmarkValidator_ValidateCommentIDs(b *testing.B) {
	v := NewValidator()
	ids := []string{"abc123", "def456", "ghi789"}
	for i := 0; i < b.N; i++ {
		_ = v.ValidateCommentIDs(ids)
	}
}

func BenchmarkValidator_ValidateUserAgent(b *testing.B) {
	v := NewValidator()
	ua := "web:myapp:1.0 by /u/myuser"
	for i := 0; i < b.N; i++ {
		_ = v.ValidateUserAgent(ua)
	}
}

func BenchmarkValidator_ValidateLinkID(b *testing.B) {
	v := NewValidator()
	linkID := "abc123"
	for i := 0; i < b.N; i++ {
		_, _ = v.ValidateLinkID(linkID)
	}
}

// Tests for new validation methods

func TestValidator_ValidatePostID(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		postID    string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{name: "valid lowercase", postID: "abc123", wantError: false},
		{name: "valid all numbers", postID: "123456", wantError: false},
		{name: "valid all letters", postID: "abcdef", wantError: false},

		// Invalid cases
		{name: "empty", postID: "", wantError: true, errorMsg: "required"},
		{name: "uppercase", postID: "ABC123", wantError: true, errorMsg: "invalid format"},
		{name: "mixed case", postID: "AbC123", wantError: true, errorMsg: "invalid format"},
		{name: "with underscore", postID: "abc_123", wantError: true, errorMsg: "invalid format"},
		{name: "with dash", postID: "abc-123", wantError: true, errorMsg: "invalid format"},
		{name: "with space", postID: "abc 123", wantError: true, errorMsg: "invalid format"},
		{name: "too long", postID: strings.Repeat("a", 101), wantError: true, errorMsg: "too long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidatePostID(tt.postID)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidatePaginationToken(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		token     string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{name: "valid comment", token: "t1_abc123", wantError: false},
		{name: "valid post", token: "t3_def456", wantError: false},
		{name: "valid subreddit", token: "t5_xyz789", wantError: false},

		// Invalid cases
		{name: "empty", token: "", wantError: true, errorMsg: "cannot be empty"},
		{name: "no prefix", token: "abc123", wantError: true, errorMsg: "invalid fullname format"},
		{name: "wrong prefix", token: "t0_abc123", wantError: true, errorMsg: "invalid fullname format"},
		{name: "uppercase ID", token: "t3_ABC123", wantError: true, errorMsg: "invalid fullname format"},
		{name: "no underscore", token: "t3abc123", wantError: true, errorMsg: "invalid fullname format"},
		{name: "missing ID", token: "t3_", wantError: true, errorMsg: "invalid fullname format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidatePaginationToken(tt.token)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidator_ValidateURL(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		url       string
		wantError bool
		errorMsg  string
	}{
		// Valid cases
		{name: "valid http", url: "http://example.com", wantError: false},
		{name: "valid https", url: "https://example.com", wantError: false},
		{name: "valid with path", url: "https://example.com/path", wantError: false},
		{name: "valid with query", url: "https://example.com?key=value", wantError: false},
		{name: "valid with port", url: "https://example.com:8080", wantError: false},

		// Invalid cases
		{name: "empty", url: "", wantError: true, errorMsg: "cannot be empty"},
		{name: "invalid scheme", url: "javascript:alert(1)", wantError: true, errorMsg: "must use http or https scheme"},
		{name: "file scheme", url: "file:///etc/passwd", wantError: true, errorMsg: "must use http or https scheme"},
		{name: "no scheme", url: "example.com", wantError: true, errorMsg: "must use http or https scheme"},
		{name: "no host", url: "https://", wantError: true, errorMsg: "must have a valid host"},
		{name: "with newline", url: "https://example.com\nX-Injected: header", wantError: true, errorMsg: "invalid"},
		{name: "with carriage return", url: "https://example.com\rX-Injected: header", wantError: true, errorMsg: "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateURL(tt.url)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
