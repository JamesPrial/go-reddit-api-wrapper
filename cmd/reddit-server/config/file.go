package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// fileConfig represents the structure of the YAML configuration file.
// All fields are optional. The file provides base values that can be overridden by environment variables.
// This uses a flat structure (no nesting) for simplicity.
// Numeric fields use pointers to distinguish between "not set" (nil) and "explicitly set to zero" (non-nil).
type fileConfig struct {
	Port                *int            `yaml:"port"`
	ShutdownTimeout     string          `yaml:"shutdown_timeout"` // Duration string (e.g., "30s", "1m")
	RequestTimeout      string          `yaml:"request_timeout"`  // Duration string (e.g., "30s", "1m")
	RedditClientID      string          `yaml:"reddit_client_id"`
	RedditClientSecret  string          `yaml:"reddit_client_secret"`
	RedditUsername      string          `yaml:"reddit_username"`
	RedditPassword      string          `yaml:"reddit_password"`
	RedditUserAgent     string          `yaml:"reddit_user_agent"`
	APIKeys             []string        `yaml:"api_keys"`
	AllowedOrigins      []string        `yaml:"allowed_origins"`
	StorageDSN          string          `yaml:"storage_dsn"`
	StorageMaxOpenConns *int            `yaml:"storage_max_open_conns"`
	StorageMaxIdleConns *int            `yaml:"storage_max_idle_conns"`
	LogLevel            string          `yaml:"log_level"`
	LogFormat           string          `yaml:"log_format"`
	LogFile             string          `yaml:"log_file"`
	Auth                *fileAuthConfig `yaml:"auth"`
}

// LoadFromFile reads a YAML configuration file and returns a Config with defaults applied.
// It parses the file, converts YAML duration strings to time.Duration, but does NOT validate.
// Validation should be done separately by calling Config.Validate().
//
// Parameters:
//   - path: Absolute or relative path to the YAML configuration file
//
// Returns:
//   - *Config: Parsed configuration with defaults applied where not specified
//   - error: Error if file cannot be read, YAML is invalid, or duration parsing fails
//
// The function handles:
//   - Missing files with clear error messages
//   - YAML syntax errors with context
//   - Duration parsing (e.g., "15s", "1m30s")
//   - Empty/missing optional fields with sensible defaults
//
// Example YAML structure (flat, no nesting):
//
//	port: 8080
//	shutdown_timeout: 30s
//	request_timeout: 30s
//	reddit_client_id: "your-client-id"
//	reddit_client_secret: "your-client-secret"
//	reddit_username: "your-username"  # optional
//	reddit_password: "your-password"  # optional
//	reddit_user_agent: "custom-agent" # optional
//	api_keys:
//	  - "key1"
//	  - "key2"
//	allowed_origins:
//	  - "http://localhost:3000"
//	storage_dsn: "/path/to/database.db"
//	storage_max_open_conns: 10
//	storage_max_idle_conns: 5
//	log_level: "info"
//	log_format: "json"
//	log_file: "/var/log/reddit-server.log"
func LoadFromFile(path string) (*Config, error) {
	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to read configuration file %q: %w", path, err)
	}

	// Parse YAML
	var fileCfg fileConfig
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}

	// Convert fileConfig to Config
	cfg, err := fileConfigToConfig(&fileCfg, path)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create parent directories if needed
	if err := ensureDirectories(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// fileConfigToConfig converts a fileConfig (parsed from YAML) to a Config.
// It applies defaults for missing fields and parses duration strings.
// It does NOT validate - validation should be done separately.
func fileConfigToConfig(fileCfg *fileConfig, configPath string) (*Config, error) {
	var errs []error

	// Start with defaults
	cfg := &Config{
		Port:                8080,
		ShutdownTimeout:     30 * time.Second,
		RequestTimeout:      30 * time.Second,
		StorageMaxOpenConns: 10,
		StorageMaxIdleConns: 5,
		LogLevel:            "info",
		LogFormat:           "json",
		LogFile:             "",
		ConfigFile:          configPath,
	}

	// Server configuration
	if fileCfg.Port != nil {
		cfg.Port = *fileCfg.Port
	}

	if fileCfg.ShutdownTimeout != "" {
		timeout, err := time.ParseDuration(fileCfg.ShutdownTimeout)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid shutdown_timeout %q: %w", fileCfg.ShutdownTimeout, err))
		} else {
			cfg.ShutdownTimeout = timeout
		}
	}

	if fileCfg.RequestTimeout != "" {
		timeout, err := time.ParseDuration(fileCfg.RequestTimeout)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid request_timeout %q: %w", fileCfg.RequestTimeout, err))
		} else {
			cfg.RequestTimeout = timeout
		}
	}

	// Reddit configuration
	cfg.RedditClientID = fileCfg.RedditClientID
	cfg.RedditClientSecret = fileCfg.RedditClientSecret
	cfg.RedditUsername = fileCfg.RedditUsername
	cfg.RedditPassword = fileCfg.RedditPassword
	cfg.RedditUserAgent = fileCfg.RedditUserAgent

	// API configuration
	if len(fileCfg.APIKeys) > 0 {
		cfg.APIKeys = fileCfg.APIKeys
	} else {
		// Generate API key if not provided
		key, err := generateAPIKey()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate API key: %w", err))
		} else {
			cfg.APIKeys = []string{key}
		}
	}

	// CORS configuration
	cfg.AllowedOrigins = fileCfg.AllowedOrigins

	// Storage configuration
	if fileCfg.StorageDSN != "" {
		// Check for directory traversal attempts
		if strings.Contains(fileCfg.StorageDSN, "..") {
			errs = append(errs, fmt.Errorf("storage DSN must not contain '..' (directory traversal protection)"))
		} else if fileCfg.StorageDSN != ":memory:" && !filepath.IsAbs(fileCfg.StorageDSN) {
			errs = append(errs, fmt.Errorf("storage DSN must be an absolute path, got %q", fileCfg.StorageDSN))
		} else {
			cfg.StorageDSN = fileCfg.StorageDSN
		}
	} else {
		// Use default DSN logic (will be set by ensureDirectories)
		cfg.StorageDSN = ""
	}

	if fileCfg.StorageMaxOpenConns != nil {
		cfg.StorageMaxOpenConns = *fileCfg.StorageMaxOpenConns
	}

	if fileCfg.StorageMaxIdleConns != nil {
		cfg.StorageMaxIdleConns = *fileCfg.StorageMaxIdleConns
	}

	// Logging configuration
	if fileCfg.LogLevel != "" {
		cfg.LogLevel = strings.ToLower(fileCfg.LogLevel)
	}

	if fileCfg.LogFormat != "" {
		cfg.LogFormat = strings.ToLower(fileCfg.LogFormat)
	}

	if fileCfg.LogFile != "" {
		// Validate path is clean
		cleanPath := filepath.Clean(fileCfg.LogFile)
		if cleanPath != fileCfg.LogFile {
			errs = append(errs, fmt.Errorf("log file path must be clean (no ., .., or duplicate slashes), got %q, expected %q", fileCfg.LogFile, cleanPath))
		} else if !filepath.IsAbs(fileCfg.LogFile) {
			errs = append(errs, fmt.Errorf("log file must be an absolute path, got %q", fileCfg.LogFile))
		} else {
			cfg.LogFile = fileCfg.LogFile
		}
	}

	// Authentication configuration
	authCfg, err := LoadAuthConfigFromFile(fileCfg.Auth)
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid auth configuration: %w", err))
	} else {
		cfg.Auth = authCfg
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return cfg, nil
}

// ensureDirectories creates necessary directories for storage and logging if they don't exist.
// This matches the behavior of Load() in config.go.
func ensureDirectories(cfg *Config) error {
	// Set default storage DSN if not provided (matches Load() behavior)
	if cfg.StorageDSN == "" {
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome != "" {
			// XDG spec requires absolute path
			if !filepath.IsAbs(dataHome) {
				return fmt.Errorf("XDG_DATA_HOME must be an absolute path, got %q", dataHome)
			}
			// Check for directory traversal
			if strings.Contains(dataHome, "..") {
				return fmt.Errorf("XDG_DATA_HOME must not contain '..' sequences, got %q", dataHome)
			}
		}
		if dataHome == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to determine home directory: %w", err)
			}
			dataHome = filepath.Join(homeDir, ".local", "share")
		}

		dbDir := filepath.Join(dataHome, "reddit-server")
		cfg.StorageDSN = filepath.Join(dbDir, "reddit.db")

		// Create directory if it doesn't exist with secure permissions
		if err := os.MkdirAll(dbDir, 0o700); err != nil {
			return fmt.Errorf("failed to create storage directory %q: %w", dbDir, err)
		}
	}

	// Create log file parent directory if needed
	if cfg.LogFile != "" {
		dir := filepath.Dir(cfg.LogFile)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create log file directory %q: %w", dir, err)
		}
	}

	return nil
}
