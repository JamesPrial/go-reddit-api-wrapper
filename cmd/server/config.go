package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// Config holds the server configuration.
type Config struct {
	// Reddit API credentials
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	UserAgent    string

	// Server configuration
	Port       int
	RateLimit  int
	RateBurst  int
	CORSOrigin string

	// Authentication mode
	UseUserAuth bool
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Load required fields
	cfg.ClientID = os.Getenv("REDDIT_CLIENT_ID")
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("REDDIT_CLIENT_ID environment variable is required")
	}

	cfg.ClientSecret = os.Getenv("REDDIT_CLIENT_SECRET")
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("REDDIT_CLIENT_SECRET environment variable is required")
	}

	// Load optional fields with defaults
	cfg.Username = os.Getenv("REDDIT_USERNAME")
	cfg.Password = os.Getenv("REDDIT_PASSWORD")

	// Determine authentication mode
	cfg.UseUserAuth = cfg.Username != "" && cfg.Password != ""

	// User agent with default
	cfg.UserAgent = os.Getenv("USER_AGENT")
	if cfg.UserAgent == "" {
		cfg.UserAgent = "go-reddit-api-wrapper-server/1.0"
	}

	// Port configuration
	portStr := os.Getenv("PORT")
	if portStr == "" {
		cfg.Port = 8080
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT value: %v", err)
		}
		cfg.Port = port
	}

	// Rate limiting configuration
	rateLimitStr := os.Getenv("RATE_LIMIT")
	if rateLimitStr == "" {
		cfg.RateLimit = 10
	} else {
		rateLimit, err := strconv.Atoi(rateLimitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_LIMIT value: %v", err)
		}
		cfg.RateLimit = rateLimit
	}

	rateBurstStr := os.Getenv("RATE_BURST")
	if rateBurstStr == "" {
		cfg.RateBurst = 5
	} else {
		rateBurst, err := strconv.Atoi(rateBurstStr)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_BURST value: %v", err)
		}
		cfg.RateBurst = rateBurst
	}

	// CORS configuration
	cfg.CORSOrigin = os.Getenv("CORS_ORIGIN")
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "*"
	}

	return cfg, nil
}

// CreateRedditClient creates a Reddit API client from the configuration.
func (c *Config) CreateRedditClient() (*graw.Reddit, error) {
	redditConfig := &graw.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		UserAgent:    c.UserAgent,
	}

	// Set up authentication mode
	if c.UseUserAuth {
		redditConfig.Username = c.Username
		redditConfig.Password = c.Password
	}

	client, err := graw.NewClient(redditConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reddit client: %w", err)
	}

	return client, nil
}

// Validate performs validation on the configuration.
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", c.Port)
	}

	if c.RateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive: %d", c.RateLimit)
	}

	if c.RateBurst <= 0 {
		return fmt.Errorf("rate burst must be positive: %d", c.RateBurst)
	}

	// Validate CORS origin format (basic validation)
	if c.CORSOrigin != "*" {
		if !strings.HasPrefix(c.CORSOrigin, "http://") && !strings.HasPrefix(c.CORSOrigin, "https://") {
			return fmt.Errorf("invalid CORS origin format: %s (must be * or start with http:// or https://)", c.CORSOrigin)
		}
	}

	return nil
}
