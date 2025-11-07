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
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-id",
		RedditClientSecret: "test-secret",
		APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
		AllowedOrigins:     []string{"http://localhost:3000", "https://example.com"},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_MissingClientID(t *testing.T) {
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "", // Missing
		RedditClientSecret: "test-secret",
		APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing client ID")
	}

	if !strings.Contains(err.Error(), "REDDIT_CLIENT_ID is required") {
		t.Errorf("Validate() error = %v, want error containing 'REDDIT_CLIENT_ID is required'", err)
	}
}

func TestValidate_MissingClientSecret(t *testing.T) {
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-id",
		RedditClientSecret: "", // Missing
		APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
	}

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
			cfg := &Config{
				Port:               tt.port,
				ShutdownTimeout:    30 * time.Second,
				RequestTimeout:     30 * time.Second,
				RedditClientID:     "test-id",
				RedditClientSecret: "test-secret",
				APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
			}

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
			cfg := &Config{
				Port:               8080,
				ShutdownTimeout:    tt.shutdownTimeout,
				RequestTimeout:     tt.requestTimeout,
				RedditClientID:     "test-id",
				RedditClientSecret: "test-secret",
				APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
			}

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
			cfg := &Config{
				Port:               8080,
				ShutdownTimeout:    tt.shutdownTimeout,
				RequestTimeout:     tt.requestTimeout,
				RedditClientID:     "test-id",
				RedditClientSecret: "test-secret",
				APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
			}

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
			cfg := &Config{
				Port:               8080,
				ShutdownTimeout:    30 * time.Second,
				RequestTimeout:     30 * time.Second,
				RedditClientID:     "test-id",
				RedditClientSecret: "test-secret",
				AllowedOrigins:     tt.origins,
				APIKeys:            []string{"dGVzdC1rZXktdGhhdC1pcy1sb25nLWVub3VnaC1mb3ItdmFsaWRhdGlvbg"},
			}

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
	cfg := &Config{
		Port:               0,                          // Invalid
		ShutdownTimeout:    -1 * time.Second,           // Invalid
		RequestTimeout:     0,                          // Invalid
		RedditClientID:     "",                         // Missing
		RedditClientSecret: "",                         // Missing
		AllowedOrigins:     []string{"invalid-origin"}, // Invalid
	}

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
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-client-id",
		RedditClientSecret: "test-client-secret",
		RedditUsername:     "test-user",
		RedditPassword:     "test-pass",
		AllowedOrigins:     []string{"http://localhost:3000"},
	}

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
	}

	str := cfg.String()

	// Verify empty credentials show as "<empty>"
	if !strings.Contains(str, "<empty>") {
		t.Error("String() does not contain '<empty>' marker for empty credentials")
	}

	// Verify "<redacted>" is not present since all credentials are empty
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
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-id",
		RedditClientSecret: "test-secret",
		APIKeys:            []string{"short"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for short API key")
	}

	if !strings.Contains(err.Error(), "must be at least 32 characters") {
		t.Errorf("Validate() error = %v, want error containing 'must be at least 32 characters'", err)
	}
}

func TestValidate_APIKeys_InvalidBase64(t *testing.T) {
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-id",
		RedditClientSecret: "test-secret",
		APIKeys:            []string{"this-is-not-valid-base64!!!!!!!"}, // 32 chars but not valid base64
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid base64 API key")
	}

	if !strings.Contains(err.Error(), "not valid base64") {
		t.Errorf("Validate() error = %v, want error containing 'not valid base64'", err)
	}
}

func TestValidate_APIKeys_Missing(t *testing.T) {
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-id",
		RedditClientSecret: "test-secret",
		APIKeys:            []string{}, // Empty slice
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing API keys")
	}

	if !strings.Contains(err.Error(), "at least one API key is required") {
		t.Errorf("Validate() error = %v, want error containing 'at least one API key is required'", err)
	}
}

func TestConfig_String_RedactsAPIKeys(t *testing.T) {
	cfg := &Config{
		Port:               8080,
		ShutdownTimeout:    30 * time.Second,
		RequestTimeout:     30 * time.Second,
		RedditClientID:     "test-id",
		RedditClientSecret: "test-secret",
		APIKeys:            []string{"dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM"},
	}

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
