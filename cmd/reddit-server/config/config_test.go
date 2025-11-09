package config

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoad_AllEnvironmentVariables(t *testing.T) {
	// Set all environment variables
	setenv(t, "PORT", "9090")
	setenv(t, "SHUTDOWN_TIMEOUT", "45s")
	setenv(t, "REQUEST_TIMEOUT", "1m")
	setenv(t, "REDDIT_CLIENT_ID", "test-client-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-client-secret")
	setenv(t, "REDDIT_USERNAME", "test-user")
	setenv(t, "REDDIT_PASSWORD", "test-pass")
	setenv(t, "REDDIT_USER_AGENT", "test-agent/1.0")
	setenv(t, "ALLOWED_ORIGINS", "http://localhost:3000,https://example.com")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify all fields
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.ShutdownTimeout != 45*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 45s", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 60*time.Second {
		t.Errorf("RequestTimeout = %v, want 1m", cfg.RequestTimeout)
	}
	if cfg.RedditClientID != "test-client-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "test-client-id")
	}
	if cfg.RedditClientSecret != "test-client-secret" {
		t.Errorf("RedditClientSecret = %q, want %q", cfg.RedditClientSecret, "test-client-secret")
	}
	if cfg.RedditUsername != "test-user" {
		t.Errorf("RedditUsername = %q, want %q", cfg.RedditUsername, "test-user")
	}
	if cfg.RedditPassword != "test-pass" {
		t.Errorf("RedditPassword = %q, want %q", cfg.RedditPassword, "test-pass")
	}
	if cfg.RedditUserAgent != "test-agent/1.0" {
		t.Errorf("RedditUserAgent = %q, want %q", cfg.RedditUserAgent, "test-agent/1.0")
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins length = %d, want 2", len(cfg.AllowedOrigins))
	}
	if len(cfg.AllowedOrigins) == 2 {
		if cfg.AllowedOrigins[0] != "http://localhost:3000" {
			t.Errorf("AllowedOrigins[0] = %q, want %q", cfg.AllowedOrigins[0], "http://localhost:3000")
		}
		if cfg.AllowedOrigins[1] != "https://example.com" {
			t.Errorf("AllowedOrigins[1] = %q, want %q", cfg.AllowedOrigins[1], "https://example.com")
		}
	}
}

func TestLoad_RequiredOnly(t *testing.T) {
	// Set only required environment variables
	setenv(t, "REDDIT_CLIENT_ID", "test-client-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-client-secret")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify defaults
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Port)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s (default)", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s (default)", cfg.RequestTimeout)
	}
	if cfg.RedditClientID != "test-client-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "test-client-id")
	}
	if cfg.RedditClientSecret != "test-client-secret" {
		t.Errorf("RedditClientSecret = %q, want %q", cfg.RedditClientSecret, "test-client-secret")
	}
	if cfg.RedditUsername != "" {
		t.Errorf("RedditUsername = %q, want empty", cfg.RedditUsername)
	}
	if cfg.RedditPassword != "" {
		t.Errorf("RedditPassword = %q, want empty", cfg.RedditPassword)
	}
	if cfg.RedditUserAgent != "" {
		t.Errorf("RedditUserAgent = %q, want empty", cfg.RedditUserAgent)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("AllowedOrigins length = %d, want 0", len(cfg.AllowedOrigins))
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	setenv(t, "PORT", "not-a-number")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid PORT")
	}

	if !strings.Contains(err.Error(), "invalid PORT") {
		t.Errorf("Load() error = %v, want error containing 'invalid PORT'", err)
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		value  string
	}{
		{
			name:   "invalid shutdown timeout",
			envVar: "SHUTDOWN_TIMEOUT",
			value:  "not-a-duration",
		},
		{
			name:   "invalid request timeout",
			envVar: "REQUEST_TIMEOUT",
			value:  "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setenv(t, tt.envVar, tt.value)
			setenv(t, "REDDIT_CLIENT_ID", "test-id")
			setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

			_, _, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error for invalid %s", tt.envVar)
			}

			if !strings.Contains(err.Error(), "invalid "+tt.envVar) {
				t.Errorf("Load() error = %v, want error containing 'invalid %s'", err, tt.envVar)
			}
		})
	}
}

func TestLoad_UsesDefaults(t *testing.T) {
	// Clear all environment variables (don't set any)
	// This tests that defaults are applied correctly

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify defaults are applied
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Port)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s (default)", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s (default)", cfg.RequestTimeout)
	}

	// Required fields should be empty but Load() should still succeed
	// Validation happens separately in Validate()
	if cfg.RedditClientID != "" {
		t.Errorf("RedditClientID = %q, want empty", cfg.RedditClientID)
	}
	if cfg.RedditClientSecret != "" {
		t.Errorf("RedditClientSecret = %q, want empty", cfg.RedditClientSecret)
	}
}

func TestLoad_ThenValidate_RejectsRequiredFields(t *testing.T) {
	// Load with defaults (missing required fields)
	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Validate should reject missing required fields
	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing required fields")
	}

	// Should contain both required field errors
	errStr := err.Error()
	if !strings.Contains(errStr, "REDDIT_CLIENT_ID is required") {
		t.Errorf("Validate() error = %v, want error containing 'REDDIT_CLIENT_ID is required'", err)
	}
	if !strings.Contains(errStr, "REDDIT_CLIENT_SECRET is required") {
		t.Errorf("Validate() error = %v, want error containing 'REDDIT_CLIENT_SECRET is required'", err)
	}
}

func TestLoad_AllowedOriginsCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{
			name:     "single origin",
			value:    "http://localhost:3000",
			expected: []string{"http://localhost:3000"},
		},
		{
			name:     "multiple origins",
			value:    "http://localhost:3000,https://example.com,https://app.example.com",
			expected: []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
		},
		{
			name:     "origins with whitespace",
			value:    "http://localhost:3000 , https://example.com , https://app.example.com",
			expected: []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
		},
		{
			name:     "origins with empty entries",
			value:    "http://localhost:3000,,https://example.com",
			expected: []string{"http://localhost:3000", "https://example.com"},
		},
		{
			name:     "empty string",
			value:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setenv(t, "ALLOWED_ORIGINS", tt.value)
			setenv(t, "REDDIT_CLIENT_ID", "test-id")
			setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

			cfg, _, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}

			if len(cfg.AllowedOrigins) != len(tt.expected) {
				t.Errorf("AllowedOrigins length = %d, want %d", len(cfg.AllowedOrigins), len(tt.expected))
			}

			for i, expected := range tt.expected {
				if i >= len(cfg.AllowedOrigins) {
					break
				}
				if cfg.AllowedOrigins[i] != expected {
					t.Errorf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], expected)
				}
			}
		})
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := newTestConfig()

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_MissingClientID(t *testing.T) {
	cfg := newTestConfig()
	cfg.RedditClientID = "" // Missing

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing client ID")
	}

	if !strings.Contains(err.Error(), "REDDIT_CLIENT_ID is required") {
		t.Errorf("Validate() error = %v, want error containing 'REDDIT_CLIENT_ID is required'", err)
	}
}

func TestValidate_MissingClientSecret(t *testing.T) {
	cfg := newTestConfig()
	cfg.RedditClientSecret = "" // Missing

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing client secret")
	}

	if !strings.Contains(err.Error(), "REDDIT_CLIENT_SECRET is required") {
		t.Errorf("Validate() error = %v, want error containing 'REDDIT_CLIENT_SECRET is required'", err)
	}
}

func TestValidate_InvalidPortRange(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{name: "port zero", port: 0},
		{name: "negative port", port: -1},
		{name: "port too high", port: 70000},
		{name: "port 65536", port: 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			cfg.Port = tt.port

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want error for port %d", tt.port)
			}

			if !strings.Contains(err.Error(), "port must be between 1 and 65535") {
				t.Errorf("Validate() error = %v, want error about port range", err)
			}
		})
	}
}

func TestValidate_NegativeTimeout(t *testing.T) {
	tests := []struct {
		name            string
		shutdownTimeout time.Duration
		requestTimeout  time.Duration
		errorContains   string
	}{
		{
			name:            "negative shutdown timeout",
			shutdownTimeout: -1 * time.Second,
			requestTimeout:  30 * time.Second,
			errorContains:   "shutdown timeout must be positive",
		},
		{
			name:            "negative request timeout",
			shutdownTimeout: 30 * time.Second,
			requestTimeout:  -1 * time.Second,
			errorContains:   "request timeout must be positive",
		},
		{
			name:            "zero shutdown timeout",
			shutdownTimeout: 0,
			requestTimeout:  30 * time.Second,
			errorContains:   "shutdown timeout must be positive",
		},
		{
			name:            "zero request timeout",
			shutdownTimeout: 30 * time.Second,
			requestTimeout:  0,
			errorContains:   "request timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			cfg.ShutdownTimeout = tt.shutdownTimeout
			cfg.RequestTimeout = tt.requestTimeout

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error for negative/zero timeout")
			}

			if !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errorContains)
			}
		})
	}
}

func TestValidate_ExcessiveTimeout(t *testing.T) {
	tests := []struct {
		name            string
		shutdownTimeout time.Duration
		requestTimeout  time.Duration
		errorContains   string
	}{
		{
			name:            "excessive shutdown timeout",
			shutdownTimeout: 6 * time.Minute,
			requestTimeout:  30 * time.Second,
			errorContains:   "shutdown timeout must not exceed 5 minutes",
		},
		{
			name:            "excessive request timeout",
			shutdownTimeout: 30 * time.Second,
			requestTimeout:  6 * time.Minute,
			errorContains:   "request timeout must not exceed 5 minutes",
		},
		{
			name:            "exactly 5 minutes is valid",
			shutdownTimeout: 5 * time.Minute,
			requestTimeout:  5 * time.Minute,
			errorContains:   "", // Should not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			cfg.ShutdownTimeout = tt.shutdownTimeout
			cfg.RequestTimeout = tt.requestTimeout

			err := cfg.Validate()
			if tt.errorContains == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil for 5 minute timeout", err)
				}
			} else {
				if err == nil {
					t.Fatal("Validate() error = nil, want error for excessive timeout")
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errorContains)
				}
			}
		})
	}
}

func TestValidate_InvalidCORSOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
	}{
		{
			name:    "no protocol",
			origins: []string{"localhost:3000"},
		},
		{
			name:    "ftp protocol",
			origins: []string{"ftp://example.com"},
		},
		{
			name:    "ws protocol",
			origins: []string{"ws://localhost:8080"},
		},
		{
			name:    "mixed valid and invalid",
			origins: []string{"http://localhost:3000", "invalid-origin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig()
			cfg.AllowedOrigins = tt.origins

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error for invalid CORS origin")
			}

			if !strings.Contains(err.Error(), "allowed origin must start with http:// or https://") {
				t.Errorf("Validate() error = %v, want error about invalid origin", err)
			}
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := newTestConfig()
	cfg.Port = 0                                    // Invalid
	cfg.ShutdownTimeout = -1 * time.Second          // Invalid
	cfg.RequestTimeout = 0                          // Invalid
	cfg.RedditClientID = ""                         // Missing
	cfg.RedditClientSecret = ""                     // Missing
	cfg.AllowedOrigins = []string{"invalid-origin"} // Invalid

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want multiple errors")
	}

	// errors.Join creates an error that unwraps to multiple errors
	var joinedErr interface{ Unwrap() []error }
	if !errors.As(err, &joinedErr) {
		t.Fatal("Validate() error is not a joined error")
	}

	errs := joinedErr.Unwrap()
	if len(errs) < 5 {
		t.Errorf("Validate() returned %d errors, want at least 5", len(errs))
	}

	// Verify all expected errors are present
	errStr := err.Error()
	expectedSubstrings := []string{
		"REDDIT_CLIENT_ID is required",
		"REDDIT_CLIENT_SECRET is required",
		"port must be between 1 and 65535",
		"shutdown timeout must be positive",
		"request timeout must be positive",
		"allowed origin must start with http:// or https://",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(errStr, substr) {
			t.Errorf("Validate() error = %v, want error containing %q", err, substr)
		}
	}
}

func TestString_Redaction(t *testing.T) {
	cfg := newTestConfig()
	cfg.RedditUsername = "test-user"
	cfg.RedditPassword = "test-pass"

	str := cfg.String()

	// Verify sensitive fields are redacted
	if strings.Contains(str, "test-client-id") {
		t.Error("String() contains unredacted client ID")
	}
	if strings.Contains(str, "test-client-secret") {
		t.Error("String() contains unredacted client secret")
	}
	if strings.Contains(str, "test-user") {
		t.Error("String() contains unredacted username")
	}
	if strings.Contains(str, "test-pass") {
		t.Error("String() contains unredacted password")
	}

	// Verify redacted marker is present
	if !strings.Contains(str, "<redacted>") {
		t.Error("String() does not contain '<redacted>' marker")
	}

	// Verify non-sensitive fields are present
	if !strings.Contains(str, "8080") {
		t.Error("String() does not contain port")
	}
	if !strings.Contains(str, "30s") {
		t.Error("String() does not contain timeout values")
	}
	if !strings.Contains(str, "http://localhost:3000") {
		t.Error("String() does not contain allowed origins")
	}
}

func TestString_EmptyCredentials(t *testing.T) {
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "",
		RedditClientSecret: "",
		RedditUsername:     "",
		RedditPassword:     "",
		LogLevel:           "info",
		LogFormat:          "json",
	}

	str := cfg.String()

	// Verify empty credentials show as "<empty>"
	if !strings.Contains(str, "<empty>") {
		t.Error("String() does not contain '<empty>' marker for empty credentials")
	}

	// Verify "<redacted>" is not present since all credentials are empty and no APIKeys
	if strings.Contains(str, "<redacted>") {
		t.Error("String() contains '<redacted>' marker when all credentials are empty")
	}
}

func TestLoad_APIKeys_Single(t *testing.T) {
	setenv(t, "API_KEYS", "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// When API_KEYS is explicitly provided, no key should be generated
	if generatedKey != "" {
		t.Errorf("generatedKey = %q, want empty string when API_KEYS is explicitly provided", generatedKey)
	}

	if len(cfg.APIKeys) != 1 {
		t.Errorf("APIKeys length = %d, want 1", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] != "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM" {
		t.Errorf("APIKeys[0] = %q, want %q", cfg.APIKeys[0], "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM")
	}
}

func TestLoad_APIKeys_Multiple(t *testing.T) {
	setenv(t, "API_KEYS", "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM,YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// When API_KEYS is explicitly provided, no key should be generated
	if generatedKey != "" {
		t.Errorf("generatedKey = %q, want empty string when API_KEYS is explicitly provided", generatedKey)
	}

	if len(cfg.APIKeys) != 2 {
		t.Errorf("APIKeys length = %d, want 2", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] != "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM" {
		t.Errorf("APIKeys[0] = %q, want %q", cfg.APIKeys[0], "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM")
	}
	if cfg.APIKeys[1] != "YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw" {
		t.Errorf("APIKeys[1] = %q, want %q", cfg.APIKeys[1], "YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw")
	}
}

func TestLoad_APIKeys_Empty(t *testing.T) {
	// Don't set API_KEYS - should trigger auto-generation
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// When API_KEYS is empty, a key should be auto-generated
	if generatedKey == "" {
		t.Error("generatedKey is empty, expected auto-generated key")
	}

	if len(cfg.APIKeys) != 1 {
		t.Errorf("APIKeys length = %d, want 1 (auto-generated)", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] == "" {
		t.Error("APIKeys[0] is empty, expected auto-generated key")
	}
	// Verify the config's key matches the returned generated key
	if cfg.APIKeys[0] != generatedKey {
		t.Errorf("APIKeys[0] = %q, want to match generatedKey = %q", cfg.APIKeys[0], generatedKey)
	}
}

func TestLoad_APIKeys_GeneratedFormat(t *testing.T) {
	// Don't set API_KEYS - should trigger auto-generation
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify generated key is returned
	if generatedKey == "" {
		t.Fatal("generatedKey is empty, expected auto-generated key")
	}

	// Verify key is at least 32 characters
	if len(generatedKey) < 32 {
		t.Errorf("Generated key length = %d, want at least 32", len(generatedKey))
	}

	// Verify key is valid base64url (RawURLEncoding without padding)
	_, err = base64.RawURLEncoding.DecodeString(generatedKey)
	if err != nil {
		t.Errorf("Generated key is not valid base64url: %v", err)
	}

	// Verify it's stored in the config
	if len(cfg.APIKeys) != 1 {
		t.Errorf("APIKeys length = %d, want 1", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] != generatedKey {
		t.Errorf("APIKeys[0] = %q, want to match generatedKey = %q", cfg.APIKeys[0], generatedKey)
	}
}

func TestValidate_APIKeys_TooShort(t *testing.T) {
	cfg := newTestConfig()
	cfg.APIKeys = []string{"short"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for short API key")
	}

	if !strings.Contains(err.Error(), "must be at least 32 characters") {
		t.Errorf("Validate() error = %v, want error containing 'must be at least 32 characters'", err)
	}
}

func TestValidate_APIKeys_InvalidBase64(t *testing.T) {
	cfg := newTestConfig()
	cfg.APIKeys = []string{"this-is-not-valid-base64!!!!!!!"} // 32 chars but not valid base64

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid base64 API key")
	}

	if !strings.Contains(err.Error(), "not valid base64") {
		t.Errorf("Validate() error = %v, want error containing 'not valid base64'", err)
	}
}

func TestValidate_APIKeys_Missing(t *testing.T) {
	cfg := newTestConfig()
	cfg.APIKeys = []string{} // Empty slice

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing API keys")
	}

	if !strings.Contains(err.Error(), "at least one API key is required") {
		t.Errorf("Validate() error = %v, want error containing 'at least one API key is required'", err)
	}
}

func TestConfig_String_RedactsAPIKeys(t *testing.T) {
	cfg := newTestConfig()
	cfg.APIKeys = []string{"dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM"}

	str := cfg.String()

	// Verify API key is redacted in output
	if strings.Contains(str, "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM") {
		t.Error("String() contains unredacted API key")
	}

	// Verify redacted marker is present for API keys
	if !strings.Contains(str, "<redacted>") {
		t.Error("String() does not contain '<redacted>' marker for API keys")
	}
}

func TestLoad_APIKeys_WhitespaceAndEmpty(t *testing.T) {
	// Test that whitespace is trimmed and empty entries are filtered
	setenv(t, "API_KEYS", "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM , , YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// When API_KEYS is explicitly provided (even with empty entries), no key should be generated
	if generatedKey != "" {
		t.Errorf("generatedKey = %q, want empty string when API_KEYS is explicitly provided", generatedKey)
	}

	// Should have exactly 2 keys (empty entries and whitespace filtered)
	if len(cfg.APIKeys) != 2 {
		t.Errorf("APIKeys length = %d, want 2 (with whitespace trimmed and empty entries removed)", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] != "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM" {
		t.Errorf("APIKeys[0] = %q, want %q (whitespace should be trimmed)", cfg.APIKeys[0], "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM")
	}
	if cfg.APIKeys[1] != "YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw" {
		t.Errorf("APIKeys[1] = %q, want %q (whitespace should be trimmed)", cfg.APIKeys[1], "YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw")
	}
}

func TestLoad_StorageDefaults(t *testing.T) {
	// Set only required environment variables
	setenv(t, "REDDIT_CLIENT_ID", "test-client-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-client-secret")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify storage defaults
	if cfg.StorageMaxOpenConns != 10 {
		t.Errorf("StorageMaxOpenConns = %d, want 10 (default)", cfg.StorageMaxOpenConns)
	}
	if cfg.StorageMaxIdleConns != 5 {
		t.Errorf("StorageMaxIdleConns = %d, want 5 (default)", cfg.StorageMaxIdleConns)
	}
	if cfg.StorageDSN == "" {
		t.Error("StorageDSN is empty, want default path")
	}
	// Verify default path contains expected components
	if !strings.Contains(cfg.StorageDSN, "reddit.db") {
		t.Errorf("StorageDSN = %q, want path containing 'reddit.db'", cfg.StorageDSN)
	}
}

func TestLoad_StorageEnvironmentVariables(t *testing.T) {
	setenv(t, "STORAGE_DSN", "/custom/path/db.sqlite")
	setenv(t, "STORAGE_MAX_OPEN_CONNS", "20")
	setenv(t, "STORAGE_MAX_IDLE_CONNS", "10")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.StorageDSN != "/custom/path/db.sqlite" {
		t.Errorf("StorageDSN = %q, want %q", cfg.StorageDSN, "/custom/path/db.sqlite")
	}
	if cfg.StorageMaxOpenConns != 20 {
		t.Errorf("StorageMaxOpenConns = %d, want 20", cfg.StorageMaxOpenConns)
	}
	if cfg.StorageMaxIdleConns != 10 {
		t.Errorf("StorageMaxIdleConns = %d, want 10", cfg.StorageMaxIdleConns)
	}
}

func TestLoad_InvalidStorageMaxOpenConns(t *testing.T) {
	setenv(t, "STORAGE_MAX_OPEN_CONNS", "not-a-number")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid STORAGE_MAX_OPEN_CONNS")
	}

	if !strings.Contains(err.Error(), "invalid STORAGE_MAX_OPEN_CONNS") {
		t.Errorf("Load() error = %v, want error containing 'invalid STORAGE_MAX_OPEN_CONNS'", err)
	}
}

func TestLoad_InvalidStorageMaxIdleConns(t *testing.T) {
	setenv(t, "STORAGE_MAX_IDLE_CONNS", "not-a-number")
	setenv(t, "REDDIT_CLIENT_ID", "test-id")
	setenv(t, "REDDIT_CLIENT_SECRET", "test-secret")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid STORAGE_MAX_IDLE_CONNS")
	}

	if !strings.Contains(err.Error(), "invalid STORAGE_MAX_IDLE_CONNS") {
		t.Errorf("Load() error = %v, want error containing 'invalid STORAGE_MAX_IDLE_CONNS'", err)
	}
}

func TestValidate_StorageEmptyDSN(t *testing.T) {
	cfg := newTestConfig()
	cfg.StorageDSN = "" // Empty

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty storage DSN")
	}

	if !strings.Contains(err.Error(), "storage DSN must not be empty") {
		t.Errorf("Validate() error = %v, want error containing 'storage DSN must not be empty'", err)
	}
}

func TestValidate_StorageNegativeMaxOpenConns(t *testing.T) {
	cfg := newTestConfig()
	cfg.StorageMaxOpenConns = -1 // Invalid

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for negative storage max open conns")
	}

	if !strings.Contains(err.Error(), "storage max open connections must be positive") {
		t.Errorf("Validate() error = %v, want error containing 'storage max open connections must be positive'", err)
	}
}

func TestValidate_StorageNegativeMaxIdleConns(t *testing.T) {
	cfg := newTestConfig()
	cfg.StorageMaxIdleConns = -1 // Invalid

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for negative storage max idle conns")
	}

	if !strings.Contains(err.Error(), "storage max idle connections must be positive") {
		t.Errorf("Validate() error = %v, want error containing 'storage max idle connections must be positive'", err)
	}
}

func TestValidate_StorageIdleExceedsOpen(t *testing.T) {
	cfg := newTestConfig()
	cfg.StorageMaxOpenConns = 5
	cfg.StorageMaxIdleConns = 10 // Exceeds max open

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error when idle exceeds open")
	}

	if !strings.Contains(err.Error(), "storage max idle connections") || !strings.Contains(err.Error(), "must not exceed max open connections") {
		t.Errorf("Validate() error = %v, want error about idle exceeding open", err)
	}
}

func TestString_StorageConfiguration(t *testing.T) {
	cfg := newTestConfig()
	cfg.StorageDSN = "/home/user/.local/share/reddit-server/reddit.db"
	cfg.StorageMaxOpenConns = 15
	cfg.StorageMaxIdleConns = 8

	str := cfg.String()

	// Verify storage config is included
	if !strings.Contains(str, "StorageDSN") {
		t.Error("String() does not contain 'StorageDSN'")
	}
	if !strings.Contains(str, "15") {
		t.Error("String() does not contain StorageMaxOpenConns value")
	}
	if !strings.Contains(str, "8") {
		t.Error("String() does not contain StorageMaxIdleConns value")
	}

	// Verify DSN path is redacted
	if strings.Contains(str, "/home/user/") {
		t.Error("String() contains unredacted DSN path")
	}
	if !strings.Contains(str, "<redacted>") {
		t.Error("String() does not contain '<redacted>' marker for DSN")
	}
}

// setenv sets an environment variable and registers a cleanup function to restore the previous value after the test.
func setenv(t *testing.T, key, value string) {
	t.Helper()
	oldValue, hadValue := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed to set environment variable %s: %v", key, err)
	}
	t.Cleanup(func() {
		if hadValue {
			os.Setenv(key, oldValue)
		} else {
			os.Unsetenv(key)
		}
	})
}

// newTestConfig returns a Config with all required fields set to valid defaults for testing.
func newTestConfig() *Config {
	return &Config{
		Port:                8080,
		ShutdownTimeout:     30 * time.Second,
		RequestTimeout:      30 * time.Second,
		RedditClientID:      "test-id",
		RedditClientSecret:  "test-secret",
		APIKeys:             []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
		AllowedOrigins:      []string{"http://localhost:3000", "https://example.com"},
		StorageDSN:          "/tmp/test.db",
		StorageMaxOpenConns: 10,
		StorageMaxIdleConns: 5,
		LogLevel:            "info",
		LogFormat:           "json",
		LogFile:             "",
	}
}

// writeTempConfigFile writes a YAML configuration file to a temporary location
// and returns the absolute path. The file is automatically cleaned up after the test.
func writeTempConfigFile(t *testing.T, content string) string {
	t.Helper()

	// Create temp file
	tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	// Write content
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	// Close file
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to close temp config file: %v", err)
	}

	// Register cleanup
	path := tmpFile.Name()
	t.Cleanup(func() {
		os.Remove(path)
	})

	return path
}

// Tests for file-based configuration

func TestLoad_FromFile_Complete(t *testing.T) {
	// Create a complete config file
	configContent := `
port: 9090
shutdown_timeout: "45s"
request_timeout: "1m"
reddit_client_id: "file-client-id"
reddit_client_secret: "file-client-secret"
reddit_username: "file-user"
reddit_password: "file-pass"
reddit_user_agent: "file-agent/1.0"
api_keys:
  - "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM"
allowed_origins:
  - "http://localhost:4000"
  - "https://file-example.com"
storage_dsn: "/tmp/file-test.db"
storage_max_open_conns: 20
storage_max_idle_conns: 10
log_level: "debug"
log_format: "text"
log_file: "/tmp/file-test.log"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	// Create parent directory for log file
	os.MkdirAll("/tmp", 0o755)

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// No key should be generated when API keys are in file
	if generatedKey != "" {
		t.Errorf("generatedKey = %q, want empty (keys provided in file)", generatedKey)
	}

	// Verify all fields from file
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.ShutdownTimeout != 45*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 45s", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 60*time.Second {
		t.Errorf("RequestTimeout = %v, want 1m", cfg.RequestTimeout)
	}
	if cfg.RedditClientID != "file-client-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "file-client-id")
	}
	if cfg.RedditClientSecret != "file-client-secret" {
		t.Errorf("RedditClientSecret = %q, want %q", cfg.RedditClientSecret, "file-client-secret")
	}
	if cfg.RedditUsername != "file-user" {
		t.Errorf("RedditUsername = %q, want %q", cfg.RedditUsername, "file-user")
	}
	if cfg.RedditPassword != "file-pass" {
		t.Errorf("RedditPassword = %q, want %q", cfg.RedditPassword, "file-pass")
	}
	if cfg.RedditUserAgent != "file-agent/1.0" {
		t.Errorf("RedditUserAgent = %q, want %q", cfg.RedditUserAgent, "file-agent/1.0")
	}
	if len(cfg.APIKeys) != 1 {
		t.Errorf("APIKeys length = %d, want 1", len(cfg.APIKeys))
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins length = %d, want 2", len(cfg.AllowedOrigins))
	}
	if cfg.StorageDSN != "/tmp/file-test.db" {
		t.Errorf("StorageDSN = %q, want %q", cfg.StorageDSN, "/tmp/file-test.db")
	}
	if cfg.StorageMaxOpenConns != 20 {
		t.Errorf("StorageMaxOpenConns = %d, want 20", cfg.StorageMaxOpenConns)
	}
	if cfg.StorageMaxIdleConns != 10 {
		t.Errorf("StorageMaxIdleConns = %d, want 10", cfg.StorageMaxIdleConns)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "text")
	}
	if cfg.LogFile != "/tmp/file-test.log" {
		t.Errorf("LogFile = %q, want %q", cfg.LogFile, "/tmp/file-test.log")
	}
	if cfg.ConfigFile != configPath {
		t.Errorf("ConfigFile = %q, want %q", cfg.ConfigFile, configPath)
	}
}

func TestLoad_FromFile_EnvOverrides(t *testing.T) {
	// Create config file with some values
	configContent := `
port: 9090
reddit_client_id: "file-client-id"
reddit_client_secret: "file-client-secret"
reddit_username: "file-user"
log_level: "debug"
storage_dsn: "/tmp/file-test.db"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	// Override some fields with environment variables
	t.Setenv("PORT", "8888")
	t.Setenv("REDDIT_CLIENT_ID", "env-client-id")
	t.Setenv("REDDIT_PASSWORD", "env-pass")
	t.Setenv("LOG_LEVEL", "warn")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Environment variables should win
	if cfg.Port != 8888 {
		t.Errorf("Port = %d, want 8888 (from env, not file)", cfg.Port)
	}
	if cfg.RedditClientID != "env-client-id" {
		t.Errorf("RedditClientID = %q, want %q (from env, not file)", cfg.RedditClientID, "env-client-id")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q (from env, not file)", cfg.LogLevel, "warn")
	}

	// File values should be used when env var is not set
	if cfg.RedditClientSecret != "file-client-secret" {
		t.Errorf("RedditClientSecret = %q, want %q (from file)", cfg.RedditClientSecret, "file-client-secret")
	}
	if cfg.RedditUsername != "file-user" {
		t.Errorf("RedditUsername = %q, want %q (from file)", cfg.RedditUsername, "file-user")
	}

	// Env var overrides empty file value
	if cfg.RedditPassword != "env-pass" {
		t.Errorf("RedditPassword = %q, want %q (from env)", cfg.RedditPassword, "env-pass")
	}
}

func TestLoad_FromFile_PartialConfig(t *testing.T) {
	// Create config file with only required fields
	configContent := `
reddit_client_id: "file-client-id"
reddit_client_secret: "file-client-secret"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// When file doesn't have API keys, LoadFromFile generates one internally
	// So Load() returns empty string (no key generated by Load itself)
	if generatedKey != "" {
		t.Errorf("generatedKey = %q, want empty (key generated by LoadFromFile, not Load)", generatedKey)
	}

	// But the config should have an API key
	if len(cfg.APIKeys) == 0 {
		t.Error("APIKeys is empty, expected auto-generated key from file loader")
	}

	// Required fields from file
	if cfg.RedditClientID != "file-client-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "file-client-id")
	}
	if cfg.RedditClientSecret != "file-client-secret" {
		t.Errorf("RedditClientSecret = %q, want %q", cfg.RedditClientSecret, "file-client-secret")
	}

	// Defaults should be applied for missing fields
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Port)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s (default)", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
	}
}

func TestLoad_FromFile_NotFound(t *testing.T) {
	// Point to a non-existent file
	t.Setenv("CONFIG_FILE", "/tmp/nonexistent-config-file-12345.yaml")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for non-existent file")
	}

	if !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Load() error = %v, want error containing 'does not exist' or 'not found'", err)
	}
}

func TestLoad_FromFile_InvalidYAML(t *testing.T) {
	// Create file with invalid YAML
	configContent := `
port: not-a-number-but-should-be
reddit_client_id: "test"
  invalid-indentation: "bad"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid YAML")
	}

	if !strings.Contains(err.Error(), "failed to parse YAML") {
		t.Errorf("Load() error = %v, want error containing 'failed to parse YAML'", err)
	}
}

func TestLoad_FromFile_RelativePath(t *testing.T) {
	// LoadFromFile accepts relative paths (they are resolved by os.ReadFile)
	// Create a relative path that doesn't exist to test error handling
	t.Setenv("CONFIG_FILE", "./nonexistent-config-test-12345.yaml")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for non-existent file")
	}

	// Should get "does not exist" or "not found" error
	if !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Load() error = %v, want error containing 'does not exist' or 'not found'", err)
	}
}

func TestLoad_FromFile_DirectoryTraversal(t *testing.T) {
	// LoadFromFile doesn't prevent directory traversal - it relies on file system permissions
	// This test ensures that attempting to read a system file fails gracefully
	// (Either file won't exist, won't be readable, or will fail YAML parsing)
	t.Setenv("CONFIG_FILE", "/etc/passwd")

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for system file")
	}

	// Should fail during YAML parsing (passwd is not valid YAML)
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "YAML") {
		// Or if file doesn't exist on some systems
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Load() error = %v, want error containing 'parse' or 'YAML' or 'not found'", err)
		}
	}
}

func TestLoad_FromFile_DirectoryNotFile(t *testing.T) {
	// Try to use a directory as config file
	tmpDir := t.TempDir()
	t.Setenv("CONFIG_FILE", tmpDir)

	_, _, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for directory path")
	}

	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("Load() error = %v, want error containing 'is a directory'", err)
	}
}

func TestLoad_FromFile_InvalidDuration(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError string
	}{
		{
			name: "invalid shutdown timeout",
			content: `
reddit_client_id: "test"
reddit_client_secret: "secret"
shutdown_timeout: "not-a-duration"
`,
			wantError: "invalid shutdown_timeout",
		},
		{
			name: "invalid request timeout",
			content: `
reddit_client_id: "test"
reddit_client_secret: "secret"
request_timeout: "bad-duration"
`,
			wantError: "invalid request_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := writeTempConfigFile(t, tt.content)
			t.Setenv("CONFIG_FILE", configPath)

			_, _, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error for %s", tt.name)
			}

			if !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Load() error = %v, want error containing %q", err, tt.wantError)
			}
		})
	}
}

func TestLoad_WithoutConfigFile_StillWorks(t *testing.T) {
	// Ensure loading without CONFIG_FILE still works (backward compatibility)
	t.Setenv("REDDIT_CLIENT_ID", "test-id")
	t.Setenv("REDDIT_CLIENT_SECRET", "test-secret")
	t.Setenv("PORT", "9000")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (backward compatibility)", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.RedditClientID != "test-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "test-id")
	}
	if cfg.ConfigFile != "" {
		t.Errorf("ConfigFile = %q, want empty (no file loaded)", cfg.ConfigFile)
	}
}

func TestLoad_FromFile_ValidationStillWorks(t *testing.T) {
	// Create file with required fields
	configContent := `
reddit_client_id: "file-client-id"
reddit_client_secret: "file-client-secret"
port: 99999
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Load should succeed, but Validate should fail
	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid port")
	}

	if !strings.Contains(err.Error(), "port must be between") {
		t.Errorf("Validate() error = %v, want error about port range", err)
	}
}

func TestLoad_FromFile_ComplexPrecedence(t *testing.T) {
	// Create file with multiple fields
	configContent := `
port: 9090
shutdown_timeout: "45s"
request_timeout: "1m"
reddit_client_id: "file-id"
reddit_client_secret: "file-secret"
reddit_username: "file-user"
log_level: "debug"
storage_dsn: "/tmp/file.db"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	// Override only some fields
	t.Setenv("PORT", "8888")
	t.Setenv("REDDIT_CLIENT_ID", "env-id")
	t.Setenv("LOG_LEVEL", "warn")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Env overrides
	if cfg.Port != 8888 {
		t.Errorf("Port = %d, want 8888 (env override)", cfg.Port)
	}
	if cfg.RedditClientID != "env-id" {
		t.Errorf("RedditClientID = %q, want %q (env override)", cfg.RedditClientID, "env-id")
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q (env override)", cfg.LogLevel, "warn")
	}

	// File values (no env override)
	if cfg.ShutdownTimeout != 45*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 45s (from file)", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 60*time.Second {
		t.Errorf("RequestTimeout = %v, want 1m (from file)", cfg.RequestTimeout)
	}
	if cfg.RedditClientSecret != "file-secret" {
		t.Errorf("RedditClientSecret = %q, want %q (from file)", cfg.RedditClientSecret, "file-secret")
	}
	if cfg.RedditUsername != "file-user" {
		t.Errorf("RedditUsername = %q, want %q (from file)", cfg.RedditUsername, "file-user")
	}
	if cfg.StorageDSN != "/tmp/file.db" {
		t.Errorf("StorageDSN = %q, want %q (from file)", cfg.StorageDSN, "/tmp/file.db")
	}
}

func TestLoad_FromFile_EnvOverridesAPIKeys(t *testing.T) {
	// Create config file with API keys
	configContent := `
reddit_client_id: "file-id"
reddit_client_secret: "file-secret"
api_keys:
  - "dGVzdC1maWxlLWFwaS1rZXktdGhhdC1pcy1hdC1sZWFzdC0zMi1jaGFycw"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	// Override API keys with environment variable
	t.Setenv("API_KEYS", "dGVzdC1lbnYtYXBpLWtleS10aGF0LWlzLWF0LWxlYXN0LTMyLWNoYXJzMTIz")

	cfg, generatedKey, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// No key should be generated when explicitly provided
	if generatedKey != "" {
		t.Errorf("generatedKey = %q, want empty (keys provided via env)", generatedKey)
	}

	// Should use env var, NOT file value
	if len(cfg.APIKeys) != 1 {
		t.Errorf("APIKeys length = %d, want 1", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] != "dGVzdC1lbnYtYXBpLWtleS10aGF0LWlzLWF0LWxlYXN0LTMyLWNoYXJzMTIz" {
		t.Errorf("APIKeys[0] = %q, want env value (not file value)", cfg.APIKeys[0])
	}
}

func TestLoad_FromFile_EnvOverridesAllowedOrigins(t *testing.T) {
	// Create config file with allowed origins
	configContent := `
reddit_client_id: "file-id"
reddit_client_secret: "file-secret"
allowed_origins:
  - "http://localhost:4000"
  - "https://file-example.com"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	// Override with environment variable
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5000,https://env-example.com")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Should use env var, NOT file values
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins length = %d, want 2 (from env)", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "http://localhost:5000" {
		t.Errorf("AllowedOrigins[0] = %q, want env value (not file)", cfg.AllowedOrigins[0])
	}
	if cfg.AllowedOrigins[1] != "https://env-example.com" {
		t.Errorf("AllowedOrigins[1] = %q, want env value (not file)", cfg.AllowedOrigins[1])
	}
}

func TestLoad_FromFile_EnvOverridesStorageDSN(t *testing.T) {
	configContent := `
reddit_client_id: "file-id"
reddit_client_secret: "file-secret"
storage_dsn: "/tmp/file-db.db"
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)
	t.Setenv("STORAGE_DSN", "/tmp/env-db.db")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Env should override file
	if cfg.StorageDSN != "/tmp/env-db.db" {
		t.Errorf("StorageDSN = %q, want %q (env override)", cfg.StorageDSN, "/tmp/env-db.db")
	}
}

func TestLoad_FromFile_EnvOverridesStorageConnections(t *testing.T) {
	configContent := `
reddit_client_id: "file-id"
reddit_client_secret: "file-secret"
storage_max_open_conns: 15
storage_max_idle_conns: 8
`
	configPath := writeTempConfigFile(t, configContent)
	t.Setenv("CONFIG_FILE", configPath)

	// Override with env vars
	t.Setenv("STORAGE_MAX_OPEN_CONNS", "25")
	t.Setenv("STORAGE_MAX_IDLE_CONNS", "12")

	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Env should override file
	if cfg.StorageMaxOpenConns != 25 {
		t.Errorf("StorageMaxOpenConns = %d, want 25 (env override)", cfg.StorageMaxOpenConns)
	}
	if cfg.StorageMaxIdleConns != 12 {
		t.Errorf("StorageMaxIdleConns = %d, want 12 (env override)", cfg.StorageMaxIdleConns)
	}
}
