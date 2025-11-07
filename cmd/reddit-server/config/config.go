// Package config handles HTTP server configuration loading from environment variables.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the HTTP server configuration loaded from environment variables.
// All fields have sensible defaults except Reddit API credentials which are required.
type Config struct {
	// Server configuration
	Port            int           // HTTP server port (default: 8080, from PORT)
	ShutdownTimeout time.Duration // Graceful shutdown timeout (default: 30s, from SHUTDOWN_TIMEOUT)
	RequestTimeout  time.Duration // HTTP request timeout (default: 30s, from REQUEST_TIMEOUT)

	// Reddit API credentials - required
	RedditClientID     string // Reddit OAuth2 client ID (from REDDIT_CLIENT_ID)
	RedditClientSecret string // Reddit OAuth2 client secret (from REDDIT_CLIENT_SECRET)

	// Optional Reddit user authentication
	RedditUsername string // Reddit username (from REDDIT_USERNAME)
	RedditPassword string // Reddit password (from REDDIT_PASSWORD)

	// Optional Reddit configuration
	RedditUserAgent string // Custom user agent (from REDDIT_USER_AGENT)

	// API key authentication
	APIKeys []string // API keys for request authentication (from API_KEYS, comma-separated, or auto-generated)

	// CORS configuration
	AllowedOrigins []string // CORS allowed origins (from ALLOWED_ORIGINS, comma-separated)
}

// Load reads configuration from environment variables and returns a Config with defaults applied.
// Required environment variables:
//   - REDDIT_CLIENT_ID: OAuth2 client ID
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret
//
// Optional environment variables:
//   - PORT: HTTP server port (default: 8080)
//   - SHUTDOWN_TIMEOUT: Graceful shutdown timeout (default: 30s, accepts duration strings like "45s", "1m")
//   - REQUEST_TIMEOUT: HTTP request timeout (default: 30s, accepts duration strings)
//   - REDDIT_USERNAME: Reddit username for user authentication
//   - REDDIT_PASSWORD: Reddit password for user authentication
//   - REDDIT_USER_AGENT: Custom user agent string
//   - API_KEYS: Comma-separated list of API keys for authentication (auto-generated if empty)
//   - ALLOWED_ORIGINS: Comma-separated list of CORS allowed origins
//
// Returns the config, a generated API key (if one was auto-generated), and an error if any required fields are missing or invalid.
// If API_KEYS is empty, a secure random API key is generated and stored in Config.APIKeys.
func Load() (*Config, string, error) {
	cfg := &Config{
		Port:            8080,
		ShutdownTimeout: 30 * time.Second,
		RequestTimeout:  30 * time.Second,
	}

	// Parse port
	if portStr := os.Getenv("PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, "", fmt.Errorf("invalid PORT: %w", err)
		}
		cfg.Port = port
	}

	// Parse shutdown timeout
	if timeoutStr := os.Getenv("SHUTDOWN_TIMEOUT"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, "", fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = timeout
	}

	// Parse request timeout
	if timeoutStr := os.Getenv("REQUEST_TIMEOUT"); timeoutStr != "" {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, "", fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
		}
		cfg.RequestTimeout = timeout
	}

	// Load Reddit credentials
	cfg.RedditClientID = os.Getenv("REDDIT_CLIENT_ID")
	cfg.RedditClientSecret = os.Getenv("REDDIT_CLIENT_SECRET")
	cfg.RedditUsername = os.Getenv("REDDIT_USERNAME")
	cfg.RedditPassword = os.Getenv("REDDIT_PASSWORD")
	cfg.RedditUserAgent = os.Getenv("REDDIT_USER_AGENT")

	// Parse API keys
	if keysStr := os.Getenv("API_KEYS"); keysStr != "" {
		keys := strings.Split(keysStr, ",")
		cfg.APIKeys = make([]string, 0, len(keys))
		for _, key := range keys {
			trimmed := strings.TrimSpace(key)
			if trimmed != "" {
				cfg.APIKeys = append(cfg.APIKeys, trimmed)
			}
		}
	}

	// Generate API key if not provided
	generatedKey := ""
	if len(cfg.APIKeys) == 0 {
		key, err := generateAPIKey()
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate API key: %w", err)
		}
		cfg.APIKeys = []string{key}
		generatedKey = key
	}

	// Parse allowed origins
	if originsStr := os.Getenv("ALLOWED_ORIGINS"); originsStr != "" {
		origins := strings.Split(originsStr, ",")
		cfg.AllowedOrigins = make([]string, 0, len(origins))
		for _, origin := range origins {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	return cfg, generatedKey, nil
}

// Validate checks that all required configuration fields are present and valid.
// It validates:
//   - Reddit API credentials (client ID and secret are required)
//   - Port range (1-65535)
//   - Timeout values (must be positive and not exceed 5 minutes)
//   - API keys (must be at least 32 characters and valid base64)
//   - CORS origins (must start with http:// or https://)
//
// Returns an error if any validation fails.
func (c *Config) Validate() error {
	var errs []error

	// Validate required Reddit credentials
	if c.RedditClientID == "" {
		errs = append(errs, errors.New("REDDIT_CLIENT_ID is required"))
	}
	if c.RedditClientSecret == "" {
		errs = append(errs, errors.New("REDDIT_CLIENT_SECRET is required"))
	}

	// Validate port range
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("port must be between 1 and 65535, got %d", c.Port))
	}

	// Validate timeouts
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("shutdown timeout must be positive, got %v", c.ShutdownTimeout))
	}
	if c.ShutdownTimeout > 5*time.Minute {
		errs = append(errs, fmt.Errorf("shutdown timeout must not exceed 5 minutes, got %v", c.ShutdownTimeout))
	}
	if c.RequestTimeout <= 0 {
		errs = append(errs, fmt.Errorf("request timeout must be positive, got %v", c.RequestTimeout))
	}
	if c.RequestTimeout > 5*time.Minute {
		errs = append(errs, fmt.Errorf("request timeout must not exceed 5 minutes, got %v", c.RequestTimeout))
	}

	// Validate API keys
	if len(c.APIKeys) == 0 {
		errs = append(errs, errors.New("at least one API key is required"))
	}
	seen := make(map[string]bool)
	for i, key := range c.APIKeys {
		if len(key) < 32 {
			errs = append(errs, fmt.Errorf("API key %d must be at least 32 characters, got %d", i+1, len(key)))
		}
		if seen[key] {
			errs = append(errs, fmt.Errorf("API key %d is a duplicate", i+1))
		}
		seen[key] = true
		if _, err := base64.RawURLEncoding.DecodeString(key); err != nil {
			errs = append(errs, fmt.Errorf("API key %d is not valid base64: %w", i+1, err))
		}
		// Check for standard base64 characters that aren't URL-safe
		if strings.ContainsAny(key, "+/") {
			errs = append(errs, fmt.Errorf("API key %d must use URL-safe base64 (no + or / characters)", i+1))
		}
	}

	// Validate CORS origins
	for _, origin := range c.AllowedOrigins {
		if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
			errs = append(errs, fmt.Errorf("allowed origin must start with http:// or https://, got %q", origin))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// String returns a string representation of the configuration for logging.
// It redacts sensitive credentials for security.
func (c *Config) String() string {
	redactedKeys := make([]string, len(c.APIKeys))
	for i := range c.APIKeys {
		redactedKeys[i] = redact(c.APIKeys[i])
	}
	return fmt.Sprintf(
		"Config{Port: %d, ShutdownTimeout: %v, RequestTimeout: %v, RedditClientID: %s, RedditClientSecret: %s, RedditUsername: %s, RedditPassword: %s, APIKeys: %v, AllowedOrigins: %v}",
		c.Port,
		c.ShutdownTimeout,
		c.RequestTimeout,
		redact(c.RedditClientID),
		redact(c.RedditClientSecret),
		redact(c.RedditUsername),
		redact(c.RedditPassword),
		redactedKeys,
		c.AllowedOrigins,
	)
}

// redact returns a redacted version of a string for logging.
// Empty strings are shown as "<empty>", non-empty strings as "<redacted>".
func redact(s string) string {
	if s == "" {
		return "<empty>"
	}
	return "<redacted>"
}

// generateAPIKey generates a secure random API key using 32 random bytes encoded with URL-safe base64.
func generateAPIKey() (string, error) {
	buf := make([]byte, 32)
	n, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	if n != len(buf) {
		return "", fmt.Errorf("insufficient random bytes: got %d, want %d", n, len(buf))
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
