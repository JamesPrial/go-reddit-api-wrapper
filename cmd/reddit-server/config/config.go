// Package config handles HTTP server configuration loading from environment variables.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	// Storage configuration
	StorageDSN          string // Database connection string (from STORAGE_DSN, default: ~/.local/share/reddit-server/reddit.db)
	StorageMaxOpenConns int    // Maximum open database connections (from STORAGE_MAX_OPEN_CONNS, default: 10)
	StorageMaxIdleConns int    // Maximum idle database connections (from STORAGE_MAX_IDLE_CONNS, default: 5)
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
//   - STORAGE_DSN: Database connection string (default: XDG_DATA_HOME/reddit-server/reddit.db or ~/.local/share/reddit-server/reddit.db)
//   - STORAGE_MAX_OPEN_CONNS: Maximum open database connections (default: 10)
//   - STORAGE_MAX_IDLE_CONNS: Maximum idle database connections (default: 5)
//
// Returns the config, a generated API key (if one was auto-generated), and an error if any required fields are missing or invalid.
// If API_KEYS is empty, a secure random API key is generated and stored in Config.APIKeys.
// The storage DSN directory is created if it doesn't exist.
func Load() (*Config, string, error) {
	cfg := &Config{
		Port:                8080,
		ShutdownTimeout:     30 * time.Second,
		RequestTimeout:      30 * time.Second,
		StorageMaxOpenConns: 10,
		StorageMaxIdleConns: 5,
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

	// Parse storage configuration
	if dsnStr := os.Getenv("STORAGE_DSN"); dsnStr != "" {
		cfg.StorageDSN = dsnStr
	} else {
		// Build default DSN using XDG_DATA_HOME or ~/.local/share
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome != "" {
			// XDG spec requires absolute path
			if !filepath.IsAbs(dataHome) {
				return nil, "", fmt.Errorf("XDG_DATA_HOME must be an absolute path, got %q", dataHome)
			}
			// Check for directory traversal
			if strings.Contains(dataHome, "..") {
				return nil, "", fmt.Errorf("XDG_DATA_HOME must not contain '..' sequences, got %q", dataHome)
			}
		}
		if dataHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, "", fmt.Errorf("failed to determine home directory: %w", err)
			}
			dataHome = filepath.Join(homeDir, ".local", "share")
		}

		dbDir := filepath.Join(dataHome, "reddit-server")
		cfg.StorageDSN = filepath.Join(dbDir, "reddit.db")

		// Create directory if it doesn't exist with secure permissions
		if err := os.MkdirAll(dbDir, 0o700); err != nil {
			return nil, "", fmt.Errorf("failed to create storage directory %q: %w", dbDir, err)
		}
	}

	// Parse storage max open connections
	if maxOpenStr := os.Getenv("STORAGE_MAX_OPEN_CONNS"); maxOpenStr != "" {
		maxOpen, err := strconv.Atoi(maxOpenStr)
		if err != nil {
			return nil, "", fmt.Errorf("invalid STORAGE_MAX_OPEN_CONNS: %w", err)
		}
		cfg.StorageMaxOpenConns = maxOpen
	}

	// Parse storage max idle connections
	if maxIdleStr := os.Getenv("STORAGE_MAX_IDLE_CONNS"); maxIdleStr != "" {
		maxIdle, err := strconv.Atoi(maxIdleStr)
		if err != nil {
			return nil, "", fmt.Errorf("invalid STORAGE_MAX_IDLE_CONNS: %w", err)
		}
		cfg.StorageMaxIdleConns = maxIdle
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
//   - Storage DSN (must not be empty)
//   - Storage pool sizes (must be positive)
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

	// Validate storage configuration
	const (
		maxStorageOpenConns = 100
		maxStorageIdleConns = 50
	)

	if c.StorageDSN == "" {
		errs = append(errs, errors.New("storage DSN must not be empty"))
	} else if c.StorageDSN != ":memory:" {
		// Check for directory traversal attempts
		if strings.Contains(c.StorageDSN, "..") {
			errs = append(errs, fmt.Errorf("storage DSN must not contain '..' (directory traversal protection)"))
		}

		// Ensure path is absolute for security
		if !filepath.IsAbs(c.StorageDSN) {
			errs = append(errs, fmt.Errorf("storage DSN must be an absolute path, got %q", c.StorageDSN))
		}

		// Validate parent directory exists and is writable
		dir := filepath.Dir(c.StorageDSN)
		if info, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				errs = append(errs, fmt.Errorf("storage DSN parent directory %q does not exist", dir))
			} else {
				errs = append(errs, fmt.Errorf("cannot access storage DSN parent directory %q: %w", dir, err))
			}
		} else if !info.IsDir() {
			errs = append(errs, fmt.Errorf("storage DSN parent %q is not a directory", dir))
		}

		// Check if DSN points to existing directory (should be file)
		if info, err := os.Stat(c.StorageDSN); err == nil && info.IsDir() {
			errs = append(errs, fmt.Errorf("storage DSN %q is a directory, expected file path", c.StorageDSN))
		}
	}

	if c.StorageMaxOpenConns <= 0 {
		errs = append(errs, fmt.Errorf("storage max open connections must be positive, got %d", c.StorageMaxOpenConns))
	}
	if c.StorageMaxOpenConns > maxStorageOpenConns {
		errs = append(errs, fmt.Errorf("storage max open connections must not exceed %d (resource limit), got %d", maxStorageOpenConns, c.StorageMaxOpenConns))
	}
	if c.StorageMaxIdleConns <= 0 {
		errs = append(errs, fmt.Errorf("storage max idle connections must be positive, got %d", c.StorageMaxIdleConns))
	}
	if c.StorageMaxIdleConns > maxStorageIdleConns {
		errs = append(errs, fmt.Errorf("storage max idle connections must not exceed %d (resource limit), got %d", maxStorageIdleConns, c.StorageMaxIdleConns))
	}
	if c.StorageMaxIdleConns > c.StorageMaxOpenConns {
		errs = append(errs, fmt.Errorf("storage max idle connections (%d) must not exceed max open connections (%d)", c.StorageMaxIdleConns, c.StorageMaxOpenConns))
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
		"Config{Port: %d, ShutdownTimeout: %v, RequestTimeout: %v, RedditClientID: %s, RedditClientSecret: %s, RedditUsername: %s, RedditPassword: %s, APIKeys: %v, AllowedOrigins: %v, StorageDSN: %s, StorageMaxOpenConns: %d, StorageMaxIdleConns: %d}",
		c.Port,
		c.ShutdownTimeout,
		c.RequestTimeout,
		redact(c.RedditClientID),
		redact(c.RedditClientSecret),
		redact(c.RedditUsername),
		redact(c.RedditPassword),
		redactedKeys,
		c.AllowedOrigins,
		redact(c.StorageDSN),
		c.StorageMaxOpenConns,
		c.StorageMaxIdleConns,
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
