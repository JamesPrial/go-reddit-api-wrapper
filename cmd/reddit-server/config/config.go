// Package config handles server configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Server contains HTTP server configuration.
type Server struct {
	Port         int           // Server port (default: 8080)
	ReadTimeout  time.Duration // Request read timeout (default: 15s)
	WriteTimeout time.Duration // Response write timeout (default: 15s)
	IdleTimeout  time.Duration // Connection idle timeout (default: 60s)
}

// Reddit contains Reddit API configuration.
type Reddit struct {
	ClientID     string // OAuth2 client ID (required)
	ClientSecret string // OAuth2 client secret (required)
	Username     string // Optional username for user authentication
	Password     string // Optional password for user authentication
	UserAgent    string // Optional custom user agent string
}

// Config represents the complete server configuration.
type Config struct {
	Server Server
	Reddit Reddit
	CORS   CORS
}

// CORS contains CORS configuration.
type CORS struct {
	AllowedOrigins string // Comma-separated list of allowed origins
	AllowedMethods string // Comma-separated list of allowed HTTP methods
	AllowedHeaders string // Comma-separated list of allowed headers
	MaxAge         int    // Max age for preflight cache in seconds
}

// FromEnv loads configuration from environment variables.
// Environment variables:
//   - SERVER_PORT: HTTP server port (default: 8080, valid: 1-65535)
//   - SERVER_READ_TIMEOUT: Read timeout in seconds (default: 15, must be positive)
//   - SERVER_WRITE_TIMEOUT: Write timeout in seconds (default: 15, must be positive)
//   - SERVER_IDLE_TIMEOUT: Idle timeout in seconds (default: 60, must be positive)
//   - REDDIT_CLIENT_ID: OAuth2 client ID (required)
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret (required)
//   - REDDIT_USERNAME: Optional username for user authentication
//   - REDDIT_PASSWORD: Optional password for user authentication
//   - REDDIT_USER_AGENT: Optional custom user agent string
//   - CORS_ALLOWED_ORIGINS: Comma-separated allowed origins (default: *)
//   - CORS_ALLOWED_METHODS: Comma-separated allowed methods (default: GET,OPTIONS)
//   - CORS_ALLOWED_HEADERS: Comma-separated allowed headers (default: Content-Type,Authorization)
//   - CORS_MAX_AGE: Preflight cache max age in seconds (default: 300, must be non-negative)
//
// Returns an error if required fields (REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET) are missing
// or if environment variables cannot be parsed.
func FromEnv() (*Config, error) {
	// Parse integer configuration values with error handling
	port, err := getIntEnv("SERVER_PORT", 8080)
	if err != nil {
		return nil, err
	}

	readTimeout, err := getIntEnv("SERVER_READ_TIMEOUT", 15)
	if err != nil {
		return nil, err
	}

	writeTimeout, err := getIntEnv("SERVER_WRITE_TIMEOUT", 15)
	if err != nil {
		return nil, err
	}

	idleTimeout, err := getIntEnv("SERVER_IDLE_TIMEOUT", 60)
	if err != nil {
		return nil, err
	}

	corsMaxAge, err := getIntEnv("CORS_MAX_AGE", 300)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: Server{
			Port:         port,
			ReadTimeout:  time.Duration(readTimeout) * time.Second,
			WriteTimeout: time.Duration(writeTimeout) * time.Second,
			IdleTimeout:  time.Duration(idleTimeout) * time.Second,
		},
		Reddit: Reddit{
			ClientID:     os.Getenv("REDDIT_CLIENT_ID"),
			ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
			Username:     os.Getenv("REDDIT_USERNAME"),
			Password:     os.Getenv("REDDIT_PASSWORD"),
			UserAgent:    os.Getenv("REDDIT_USER_AGENT"),
		},
		CORS: CORS{
			AllowedOrigins: getStringEnv("CORS_ALLOWED_ORIGINS", "*"),
			AllowedMethods: getStringEnv("CORS_ALLOWED_METHODS", "GET,OPTIONS"),
			AllowedHeaders: getStringEnv("CORS_ALLOWED_HEADERS", "Content-Type,Authorization"),
			MaxAge:         corsMaxAge,
		},
	}

	// Validate required Reddit credentials
	if cfg.Reddit.ClientID == "" {
		return nil, fmt.Errorf("REDDIT_CLIENT_ID environment variable is required")
	}
	if cfg.Reddit.ClientSecret == "" {
		return nil, fmt.Errorf("REDDIT_CLIENT_SECRET environment variable is required")
	}

	// Validate server configuration ranges
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return nil, fmt.Errorf("server port must be between 1 and 65535, got %d", cfg.Server.Port)
	}

	if cfg.Server.ReadTimeout <= 0 {
		return nil, fmt.Errorf("server read timeout must be positive, got %d seconds", readTimeout)
	}

	if cfg.Server.WriteTimeout <= 0 {
		return nil, fmt.Errorf("server write timeout must be positive, got %d seconds", writeTimeout)
	}

	if cfg.Server.IdleTimeout <= 0 {
		return nil, fmt.Errorf("server idle timeout must be positive, got %d seconds", idleTimeout)
	}

	// Validate CORS configuration
	if cfg.CORS.MaxAge < 0 {
		return nil, fmt.Errorf("CORS max age must be non-negative, got %d", cfg.CORS.MaxAge)
	}

	// Set default user agent if not provided
	if cfg.Reddit.UserAgent == "" {
		cfg.Reddit.UserAgent = "reddit-server/1.0 by /u/reddit-server-user"
	}

	return cfg, nil
}

// getIntEnv gets an integer environment variable with a default value.
// Returns an error if the environment variable cannot be parsed as an integer.
func getIntEnv(key string, defaultVal int) (int, error) {
	if val, ok := os.LookupEnv(key); ok {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("invalid integer for %s: %s", key, val)
		}
		return intVal, nil
	}
	return defaultVal, nil
}

// getStringEnv gets a string environment variable with a default value.
func getStringEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
