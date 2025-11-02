// Package config handles CLI configuration loading from environment variables and flags.
package config

import (
	"fmt"
	"os"

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
//
// Returns an error if required fields are missing.
func FromEnv() (*Config, error) {
	cfg := &Config{
		ClientID:     os.Getenv("REDDIT_CLIENT_ID"),
		ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
		Username:     os.Getenv("REDDIT_USERNAME"),
		Password:     os.Getenv("REDDIT_PASSWORD"),
		UserAgent:    os.Getenv("REDDIT_USER_AGENT"),
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

// Validate checks that all required configuration fields are set.
// Returns an error if any required fields are missing or invalid.
func (c *Config) Validate() error {
	if c.ClientID == "" {
		return fmt.Errorf("client ID is required")
	}
	if c.ClientSecret == "" {
		return fmt.Errorf("client secret is required")
	}

	if c.Limit < 1 || c.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100, got %d", c.Limit)
	}

	if c.Output != "json" && c.Output != "table" && c.Output != "text" {
		return fmt.Errorf("output format must be json, table, or text, got %q", c.Output)
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
