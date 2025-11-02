// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/config"
)

const (
	// maskedValue is the placeholder for sensitive fields that should not be logged.
	maskedValue = "***REDACTED***"
)

// ShowConfig displays the current configuration with sensitive fields redacted.
//
// This function pretty-prints the configuration to stdout, masking all sensitive
// fields (ClientSecret, Password) to prevent accidental exposure of credentials.
// This is safe to use in logs and terminal output.
//
// Parameters:
//   - cfg: the configuration to display
//
// Returns an error if output fails (unlikely in normal operation).
func ShowConfig(cfg *config.Config) error {
	return showConfigToWriter(os.Stdout, cfg)
}

// showConfigToWriter displays the configuration to the provided writer.
// This is the internal implementation that allows testing with custom writers.
func showConfigToWriter(w io.Writer, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if w == nil {
		return fmt.Errorf("writer cannot be nil")
	}

	// Build configuration display lines
	lines := []string{
		"Reddit CLI Configuration:",
		"",
		fmt.Sprintf("  Client ID:        %s", maskSensitive(cfg.ClientID, false)),
		fmt.Sprintf("  Client Secret:    %s", maskSensitive(cfg.ClientSecret, true)),
		fmt.Sprintf("  Username:         %s", maskSensitive(cfg.Username, false)),
		fmt.Sprintf("  Password:         %s", maskSensitive(cfg.Password, true)),
		fmt.Sprintf("  User Agent:       %s", cfg.UserAgent),
		"",
		fmt.Sprintf("  Output Format:    %s", cfg.Output),
		fmt.Sprintf("  Default Limit:    %d", cfg.Limit),
		fmt.Sprintf("  Pagination After: %s", maskSensitive(cfg.After, false)),
		fmt.Sprintf("  Pagination Before:%s", maskSensitive(cfg.Before, false)),
		"",
		fmt.Sprintf("  Verbose:          %v", cfg.Verbose),
		fmt.Sprintf("  Debug:            %v", cfg.Debug),
	}

	output := strings.Join(lines, "\n")
	_, err := fmt.Fprintf(w, "%s\n", output)
	return err
}

// ValidateConfig validates the configuration without making API calls.
//
// This function checks that all required fields are present and in valid format
// (e.g., output format is one of json/table/text, limit is in valid range).
// Validation is performed without making any network requests.
//
// Parameters:
//   - cfg: the configuration to validate
//
// Returns an error if validation fails, nil if configuration is valid.
func ValidateConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Check required authentication fields
	if cfg.ClientID == "" {
		return fmt.Errorf("validation failed: client_id is required")
	}
	if cfg.ClientSecret == "" {
		return fmt.Errorf("validation failed: client_secret is required")
	}

	// Validate output format
	validFormats := map[string]bool{
		"json":  true,
		"table": true,
		"text":  true,
	}
	if !validFormats[cfg.Output] {
		return fmt.Errorf("validation failed: invalid output format %q (must be json, table, or text)", cfg.Output)
	}

	// Validate limit is in acceptable range (1-100 per Reddit API)
	if cfg.Limit < 1 || cfg.Limit > 100 {
		return fmt.Errorf("validation failed: limit must be between 1 and 100, got %d", cfg.Limit)
	}

	// Validate user agent if provided
	if cfg.UserAgent != "" {
		if len(cfg.UserAgent) > 500 {
			return fmt.Errorf("validation failed: user_agent is too long (max 500 characters)")
		}
		// Check for potential header injection
		if strings.ContainsAny(cfg.UserAgent, "\r\n") {
			return fmt.Errorf("validation failed: user_agent contains invalid characters")
		}
	}

	// If user credentials are provided, both must be present
	if (cfg.Username == "" && cfg.Password != "") || (cfg.Username != "" && cfg.Password == "") {
		return fmt.Errorf("validation failed: both username and password must be provided for user authentication, or neither")
	}

	return nil
}

// maskSensitive masks sensitive values for safe display.
//
// For non-secret fields (e.g., ClientID), empty values are shown as "(not set)".
// For secret fields (e.g., ClientSecret), non-empty values are masked.
//
// Parameters:
//   - value: the value to mask
//   - isSecret: true if this is a sensitive field that should be masked when present
//
// Returns:
//   - "(not set)" if value is empty
//   - "***REDACTED***" if isSecret is true and value is non-empty
//   - the original value if isSecret is false and value is non-empty
func maskSensitive(value string, isSecret bool) string {
	if value == "" {
		return "(not set)"
	}

	if isSecret {
		return maskedValue
	}

	// For non-secret fields, show a preview of longer values
	if len(value) > 20 {
		return value[:20] + "..."
	}
	return value
}
