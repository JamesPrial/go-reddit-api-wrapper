package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLoadFromFile_ValidYAMLAllFields tests loading a YAML file with all fields populated.
func TestLoadFromFile_ValidYAMLAllFields(t *testing.T) {
	yamlContent := `
port: 9090
shutdown_timeout: 45s
request_timeout: 1m
reddit_client_id: test-client-id
reddit_client_secret: test-client-secret
reddit_username: test-user
reddit_password: test-pass
reddit_user_agent: test-agent/1.0
api_keys:
  - dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM
  - YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw
allowed_origins:
  - http://localhost:3000
  - https://example.com
storage_dsn: /tmp/test.db
storage_max_open_conns: 20
storage_max_idle_conns: 10
log_level: debug
log_format: text
log_file: /tmp/test.log
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	// Verify all fields are correctly loaded
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
	if len(cfg.APIKeys) != 2 {
		t.Errorf("APIKeys length = %d, want 2", len(cfg.APIKeys))
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins length = %d, want 2", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("AllowedOrigins[0] = %q, want %q", cfg.AllowedOrigins[0], "http://localhost:3000")
	}
	if cfg.AllowedOrigins[1] != "https://example.com" {
		t.Errorf("AllowedOrigins[1] = %q, want %q", cfg.AllowedOrigins[1], "https://example.com")
	}
	if cfg.StorageDSN != "/tmp/test.db" {
		t.Errorf("StorageDSN = %q, want %q", cfg.StorageDSN, "/tmp/test.db")
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
	if cfg.LogFile != "/tmp/test.log" {
		t.Errorf("LogFile = %q, want %q", cfg.LogFile, "/tmp/test.log")
	}
}

// TestLoadFromFile_RequiredFieldsOnly tests loading a YAML file with only required fields.
func TestLoadFromFile_RequiredFieldsOnly(t *testing.T) {
	yamlContent := `
reddit_client_id: test-client-id
reddit_client_secret: test-client-secret
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	// Verify required fields are set
	if cfg.RedditClientID != "test-client-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "test-client-id")
	}
	if cfg.RedditClientSecret != "test-client-secret" {
		t.Errorf("RedditClientSecret = %q, want %q", cfg.RedditClientSecret, "test-client-secret")
	}

	// Verify defaults are applied for optional fields
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Port)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s (default)", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s (default)", cfg.RequestTimeout)
	}
	if cfg.StorageMaxOpenConns != 10 {
		t.Errorf("StorageMaxOpenConns = %d, want 10 (default)", cfg.StorageMaxOpenConns)
	}
	if cfg.StorageMaxIdleConns != 5 {
		t.Errorf("StorageMaxIdleConns = %d, want 5 (default)", cfg.StorageMaxIdleConns)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q (default)", cfg.LogFormat, "json")
	}
	if cfg.RedditUsername != "" {
		t.Errorf("RedditUsername = %q, want empty", cfg.RedditUsername)
	}
	if cfg.RedditPassword != "" {
		t.Errorf("RedditPassword = %q, want empty", cfg.RedditPassword)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("AllowedOrigins length = %d, want 0", len(cfg.AllowedOrigins))
	}
}

// TestLoadFromFile_InvalidYAMLSyntax tests loading a file with invalid YAML syntax.
func TestLoadFromFile_InvalidYAMLSyntax(t *testing.T) {
	yamlContent := `
port: 9090
reddit_client_id: test-id
  invalid_indentation: this is wrong
reddit_client_secret: test-secret
`
	configPath := createTempYAMLFile(t, yamlContent)

	_, err := LoadFromFile(configPath)
	if err == nil {
		t.Fatal("LoadFromFile() error = nil, want error for invalid YAML syntax")
	}

	if !strings.Contains(err.Error(), "yaml") && !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("LoadFromFile() error = %v, want error mentioning YAML parsing issue", err)
	}
}

// TestLoadFromFile_NonExistentFile tests loading a file that doesn't exist.
func TestLoadFromFile_NonExistentFile(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("LoadFromFile() error = nil, want error for non-existent file")
	}

	if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "not exist") && !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("LoadFromFile() error = %v, want error mentioning file not found", err)
	}
}

// TestLoadFromFile_InvalidDurationFormat tests loading a file with invalid duration format.
func TestLoadFromFile_InvalidDurationFormat(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		errSubstr string
	}{
		{
			name: "invalid shutdown timeout",
			content: `
reddit_client_id: test-id
reddit_client_secret: test-secret
shutdown_timeout: not-a-duration
`,
			errSubstr: "shutdown_timeout",
		},
		{
			name: "invalid request timeout",
			content: `
reddit_client_id: test-id
reddit_client_secret: test-secret
request_timeout: invalid
`,
			errSubstr: "request_timeout",
		},
		{
			name: "timeout without unit",
			content: `
reddit_client_id: test-id
reddit_client_secret: test-secret
shutdown_timeout: 30
`,
			errSubstr: "shutdown_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := createTempYAMLFile(t, tt.content)

			_, err := LoadFromFile(configPath)
			if err == nil {
				t.Fatalf("LoadFromFile() error = nil, want error for invalid duration format")
			}

			errStr := strings.ToLower(err.Error())
			if !strings.Contains(errStr, "duration") && !strings.Contains(errStr, "timeout") && !strings.Contains(errStr, "parse") {
				t.Errorf("LoadFromFile() error = %v, want error mentioning duration/timeout/parse issue", err)
			}
		})
	}
}

// TestLoadFromFile_EmptyFile tests loading an empty file.
func TestLoadFromFile_EmptyFile(t *testing.T) {
	configPath := createTempYAMLFile(t, "")

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil (empty file should load with defaults)", err)
	}

	// Empty file should result in all defaults being applied
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default)", cfg.Port)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s (default)", cfg.ShutdownTimeout)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s (default)", cfg.RequestTimeout)
	}
	if cfg.RedditClientID != "" {
		t.Errorf("RedditClientID = %q, want empty", cfg.RedditClientID)
	}
	if cfg.RedditClientSecret != "" {
		t.Errorf("RedditClientSecret = %q, want empty", cfg.RedditClientSecret)
	}
}

// TestLoadFromFile_PartialConfig tests loading a file with some sections missing.
func TestLoadFromFile_PartialConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "only server config",
			content: `
port: 9090
shutdown_timeout: 45s
`,
		},
		{
			name: "only reddit config",
			content: `
reddit_client_id: test-id
reddit_client_secret: test-secret
reddit_username: test-user
`,
		},
		{
			name: "only storage config",
			content: `
storage_dsn: /tmp/test.db
storage_max_open_conns: 15
`,
		},
		{
			name: "only logging config",
			content: `
log_level: debug
log_format: text
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := createTempYAMLFile(t, tt.content)

			cfg, err := LoadFromFile(configPath)
			if err != nil {
				t.Fatalf("LoadFromFile() error = %v, want nil", err)
			}

			// Verify that loaded fields are set and missing fields have defaults
			if cfg == nil {
				t.Fatal("LoadFromFile() returned nil config")
			}

			// All partial configs should still have defaults for missing fields
			if cfg.Port == 0 && !strings.Contains(tt.content, "port") {
				t.Errorf("Port = %d, want default 8080", cfg.Port)
			}
		})
	}
}

// TestLoadFromFile_WithComments tests loading a file with YAML comments.
func TestLoadFromFile_WithComments(t *testing.T) {
	yamlContent := `
# Server configuration
port: 9090
shutdown_timeout: 45s  # How long to wait for graceful shutdown
request_timeout: 1m    # Maximum request duration

# Reddit API credentials
reddit_client_id: test-client-id      # From Reddit app registration
reddit_client_secret: test-client-secret
# Optional user authentication
reddit_username: test-user
reddit_password: test-pass

# Storage configuration
storage_dsn: /tmp/test.db  # SQLite database path
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	// Verify fields are correctly loaded despite comments
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.RedditClientID != "test-client-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "test-client-id")
	}
	if cfg.StorageDSN != "/tmp/test.db" {
		t.Errorf("StorageDSN = %q, want %q", cfg.StorageDSN, "/tmp/test.db")
	}
}

// TestLoadFromFile_ExtraUnknownFields tests that extra unknown fields are ignored.
func TestLoadFromFile_ExtraUnknownFields(t *testing.T) {
	yamlContent := `
reddit_client_id: test-id
reddit_client_secret: test-secret
port: 9090
unknown_field: this-should-be-ignored
another_unknown: also-ignored
nested:
  unknown: ignored
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil (unknown fields should be ignored)", err)
	}

	// Verify known fields are still loaded correctly
	if cfg.RedditClientID != "test-id" {
		t.Errorf("RedditClientID = %q, want %q", cfg.RedditClientID, "test-id")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

// TestLoadFromFile_WrongTypeForPort tests loading a file with wrong type for port (string instead of int).
func TestLoadFromFile_WrongTypeForPort(t *testing.T) {
	yamlContent := `
reddit_client_id: test-id
reddit_client_secret: test-secret
port: "not-a-number"
`
	configPath := createTempYAMLFile(t, yamlContent)

	_, err := LoadFromFile(configPath)
	if err == nil {
		t.Fatal("LoadFromFile() error = nil, want error for wrong type")
	}

	// Error should be about YAML unmarshal failure or type mismatch
	// Since Port is a pointer now, error message might not contain "port"
	if !strings.Contains(err.Error(), "unmarshal") && !strings.Contains(err.Error(), "cannot") && !strings.Contains(err.Error(), "type") {
		t.Errorf("LoadFromFile() error = %v, want error about type mismatch or unmarshal failure", err)
	}
}

// TestLoadFromFile_InvalidPortRange tests loading a file with invalid port values.
func TestLoadFromFile_InvalidPortRange(t *testing.T) {
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
			yamlContent := `
reddit_client_id: test-id
reddit_client_secret: test-secret
port: ` + strconv.Itoa(tt.port)
			configPath := createTempYAMLFile(t, yamlContent)

			cfg, err := LoadFromFile(configPath)
			// Loading should succeed - validation happens separately
			if err != nil {
				t.Fatalf("LoadFromFile() error = %v, want nil (validation is separate)", err)
			}

			// But validation should fail
			err = cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want error for invalid port %d", tt.port)
			}

			if !strings.Contains(err.Error(), "port") {
				t.Errorf("Validate() error = %v, want error about port", err)
			}
		})
	}
}

// TestLoadFromFile_NegativeTimeouts tests loading a file with negative timeout values.
func TestLoadFromFile_NegativeTimeouts(t *testing.T) {
	yamlContent := `
reddit_client_id: test-id
reddit_client_secret: test-secret
shutdown_timeout: -30s
request_timeout: -1m
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	// Loading should succeed - validation happens separately
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil (validation is separate)", err)
	}

	// But validation should fail
	err = cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for negative timeouts")
	}

	if !strings.Contains(err.Error(), "timeout") || !strings.Contains(err.Error(), "positive") {
		t.Errorf("Validate() error = %v, want error about positive timeout", err)
	}
}

// TestLoadFromFile_WrongTypeStorageConns tests loading a file with wrong types for storage connection counts.
func TestLoadFromFile_WrongTypeStorageConns(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "string for max_open_conns",
			content: `
reddit_client_id: test-id
reddit_client_secret: test-secret
storage_max_open_conns: "not-a-number"
`,
		},
		{
			name: "string for max_idle_conns",
			content: `
reddit_client_id: test-id
reddit_client_secret: test-secret
storage_max_idle_conns: "not-a-number"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := createTempYAMLFile(t, tt.content)

			_, err := LoadFromFile(configPath)
			if err == nil {
				t.Fatal("LoadFromFile() error = nil, want error for wrong type")
			}

			// Error should be about YAML unmarshal failure or type mismatch
			// Since storage fields are pointers now, error message might not contain "storage"
			if !strings.Contains(err.Error(), "unmarshal") && !strings.Contains(err.Error(), "cannot") && !strings.Contains(err.Error(), "type") {
				t.Errorf("LoadFromFile() error = %v, want error about type mismatch or unmarshal failure", err)
			}
		})
	}
}

// TestLoadFromFile_APIKeysArray tests loading API keys as a YAML array.
func TestLoadFromFile_APIKeysArray(t *testing.T) {
	yamlContent := `
reddit_client_id: test-id
reddit_client_secret: test-secret
api_keys:
  - dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM
  - YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	if len(cfg.APIKeys) != 2 {
		t.Errorf("APIKeys length = %d, want 2", len(cfg.APIKeys))
	}
	if cfg.APIKeys[0] != "dGVzdC1hcGkta2V5LXRoYXQtaXMtYXQtbGVhc3QtMzItY2hhcnM" {
		t.Errorf("APIKeys[0] = %q, want first key", cfg.APIKeys[0])
	}
	if cfg.APIKeys[1] != "YW5vdGhlci10ZXN0LWFwaS1rZXktdGhhdC1pcy0zMi1jaGFycw" {
		t.Errorf("APIKeys[1] = %q, want second key", cfg.APIKeys[1])
	}
}

// TestLoadFromFile_AllowedOriginsArray tests loading allowed origins as a YAML array.
func TestLoadFromFile_AllowedOriginsArray(t *testing.T) {
	yamlContent := `
reddit_client_id: test-id
reddit_client_secret: test-secret
allowed_origins:
  - http://localhost:3000
  - https://example.com
  - https://app.example.com
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	if len(cfg.AllowedOrigins) != 3 {
		t.Errorf("AllowedOrigins length = %d, want 3", len(cfg.AllowedOrigins))
	}
	if cfg.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("AllowedOrigins[0] = %q, want %q", cfg.AllowedOrigins[0], "http://localhost:3000")
	}
	if cfg.AllowedOrigins[1] != "https://example.com" {
		t.Errorf("AllowedOrigins[1] = %q, want %q", cfg.AllowedOrigins[1], "https://example.com")
	}
	if cfg.AllowedOrigins[2] != "https://app.example.com" {
		t.Errorf("AllowedOrigins[2] = %q, want %q", cfg.AllowedOrigins[2], "https://app.example.com")
	}
}

// TestLoadFromFile_CaseSensitivity tests that YAML keys are case-sensitive.
func TestLoadFromFile_CaseSensitivity(t *testing.T) {
	yamlContent := `
Port: 9090  # Wrong case - should be ignored
REDDIT_CLIENT_ID: test-id  # Wrong case - should be ignored
reddit_client_secret: test-secret
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	// Wrong case fields should be ignored, defaults should apply
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080 (default, since 'Port' with capital P should be ignored)", cfg.Port)
	}
	if cfg.RedditClientID != "" {
		t.Errorf("RedditClientID = %q, want empty (since 'REDDIT_CLIENT_ID' in caps should be ignored)", cfg.RedditClientID)
	}
	if cfg.RedditClientSecret != "test-secret" {
		t.Errorf("RedditClientSecret = %q, want %q", cfg.RedditClientSecret, "test-secret")
	}
}

// TestLoadFromFile_WhitespaceInValues tests that whitespace in values is preserved/trimmed correctly.
func TestLoadFromFile_WhitespaceInValues(t *testing.T) {
	yamlContent := `
reddit_client_id: "  test-id  "
reddit_client_secret: test-secret
reddit_user_agent: "  My Agent/1.0  "
`
	configPath := createTempYAMLFile(t, yamlContent)

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v, want nil", err)
	}

	// YAML should preserve quoted strings exactly
	if cfg.RedditClientID != "  test-id  " {
		t.Errorf("RedditClientID = %q, want %q (whitespace should be preserved in quotes)", cfg.RedditClientID, "  test-id  ")
	}
	if cfg.RedditUserAgent != "  My Agent/1.0  " {
		t.Errorf("RedditUserAgent = %q, want %q (whitespace should be preserved in quotes)", cfg.RedditUserAgent, "  My Agent/1.0  ")
	}
}

// createTempYAMLFile creates a temporary YAML file with the given content.
// The file is automatically cleaned up after the test completes.
func createTempYAMLFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir() // Automatically cleaned up by testing framework
	configPath := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configPath, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("Failed to create temp YAML file: %v", err)
	}

	return configPath
}
