// Package config handles CLI configuration loading from environment variables and flags.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// Config represents the CLI configuration combining environment variables and CLI flags.
// All fields are optional except those required for Reddit API authentication.
type Config struct {
	// Reddit API credentials - required
	ClientID     string
	ClientSecret string

	// Optional user authentication (for user-specific operations)
	Username string
	Password string

	// Output and formatting options
	Output string // json, table, text (default: text)
	Limit  int    // Number of items to fetch (default: 25, max: 100)
	After  string // Pagination token for next page
	Before string // Pagination token for previous page

	// Behavior options
	Verbose bool // Enable verbose output
	Debug   bool // Enable debug logging

	// Storage configuration
	Store  bool   // Enable storage of posts and comments
	DBPath string // Path to SQLite database file (e.g., ~/.reddit/data.db)

	// Query filters for stored data
	SubredditFilter string // Filter posts by subreddit name
	MinScoreFilter  int    // Filter posts with minimum score threshold (can be negative)

	// User agent string (auto-generated if not provided)
	UserAgent string
}

// FromEnv loads configuration from environment variables.
// It looks for the following environment variables:
//   - REDDIT_CLIENT_ID: OAuth2 client ID (required)
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret (required)
//   - REDDIT_USERNAME: Optional username for user authentication
//   - REDDIT_PASSWORD: Optional password for user authentication
//   - REDDIT_USER_AGENT: Optional custom user agent string
//   - REDDIT_STORE: Optional boolean to enable storage (default: false)
//   - REDDIT_DB_PATH: Optional path to SQLite database file (default: ~/.reddit/data.db)
//
// Returns an error if required fields are missing.
func FromEnv() (*Config, error) {
	dbPath := os.Getenv("REDDIT_DB_PATH")

	if dbPath == "" {
		// Default path - expand tilde
		homeDir, err := os.UserHomeDir()
		if err == nil {
			dbPath = filepath.Join(homeDir, ".reddit", "data.db")
		} else {
			// Fall back to literal path if home directory cannot be determined
			dbPath = "~/.reddit/data.db"
		}
	} else {
		// User-provided path - expand tilde if present
		if strings.HasPrefix(dbPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				dbPath = filepath.Join(homeDir, dbPath[2:])
			}
		} else if dbPath == "~" {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				dbPath = filepath.Join(homeDir, ".reddit", "data.db")
			}
		}
	}

	cfg := &Config{
		ClientID:     os.Getenv("REDDIT_CLIENT_ID"),
		ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
		Username:     os.Getenv("REDDIT_USERNAME"),
		Password:     os.Getenv("REDDIT_PASSWORD"),
		UserAgent:    os.Getenv("REDDIT_USER_AGENT"),
		Store:        os.Getenv("REDDIT_STORE") == "true",
		DBPath:       dbPath,
		Output:       "text",
		Limit:        25,
	}

	if cfg.ClientID == "" {
		return nil, fmt.Errorf("REDDIT_CLIENT_ID environment variable is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("REDDIT_CLIENT_SECRET environment variable is required")
	}

	return cfg, nil
}

// ValidateCredentials checks if required Reddit API authentication credentials are configured.
// This must be called before creating a Reddit API client.
// Returns an error if ClientID or ClientSecret is empty.
func (c *Config) ValidateCredentials() error {
	if c.ClientID == "" {
		return fmt.Errorf("client ID is required (set REDDIT_CLIENT_ID or use -client-id)")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("client secret is required (set REDDIT_CLIENT_SECRET or use -client-secret)")
	}
	return nil
}

// Validate checks that non-credential configuration fields are valid.
// It validates output format, pagination limits, and storage configuration.
// For credential validation, use ValidateCredentials() instead.
// Returns an error if any fields are invalid.
func (c *Config) Validate() error {
	if c.Limit < 1 || c.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100, got %d", c.Limit)
	}

	if c.Output != "json" && c.Output != "table" && c.Output != "text" {
		return fmt.Errorf("output format must be json, table, or text, got %q", c.Output)
	}

	if c.Store && c.DBPath == "" {
		return fmt.Errorf("database path is required when storage is enabled (set REDDIT_DB_PATH or use -db-path)")
	}

	return nil
}

// ToRedditConfig converts the CLI config to a graw.Config suitable for creating a client.
// This handles setting default user agent and authentication mode selection.
func (c *Config) ToRedditConfig() *graw.Config {
	cfg := &graw.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Username:     c.Username,
		Password:     c.Password,
	}

	// Set user agent
	if c.UserAgent != "" {
		cfg.UserAgent = c.UserAgent
	} else {
		cfg.UserAgent = "reddit-cli/1.0 by /u/reddit-cli-user"
	}

	return cfg
}

// String returns a string representation of the configuration for logging/debugging.
// It includes the database path but does not include sensitive credentials.
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{ClientID: %s, Username: %s, Output: %s, Limit: %d, Verbose: %v, Debug: %v, Store: %v, DBPath: %s, SubredditFilter: %s, MinScoreFilter: %d}",
		c.ClientID, c.Username, c.Output, c.Limit, c.Verbose, c.Debug, c.Store, c.DBPath, c.SubredditFilter, c.MinScoreFilter,
	)
}
