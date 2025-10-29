package sentiment

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// Config contains configuration options for the sentiment Analyzer.
// All fields are optional and sensible defaults will be used if not provided.
type Config struct {
	// Logger is an optional slog.Logger for structured logging of analysis operations.
	// If nil, no logging will be performed.
	Logger *slog.Logger
	// MinWordCount specifies the minimum number of words required to perform sentiment analysis.
	// Content with fewer words than this threshold will be treated as neutral.
	// Default is 3.
	MinWordCount int
	// EnableEmoticons specifies whether to include emoticon analysis in sentiment scoring.
	// When enabled, emoticons like :) and :( will contribute to the sentiment score.
	// Default is true.
	EnableEmoticons bool
}

// DefaultConfig returns a Config with sensible default values.
// The returned configuration:
//   - Does not include a logger (logging disabled)
//   - Requires a minimum of 3 words for analysis
//   - Enables emoticon analysis
func DefaultConfig() *Config {
	return &Config{
		Logger:          nil,
		MinWordCount:    3,
		EnableEmoticons: true,
	}
}

// LoadAnalyzerConfigFromFile loads analyzer configuration from a JSON file.
// This function loads analyzer-specific settings only (MinWordCount, EnableEmoticons).
// The file should contain a JSON object with optional fields:
//   - "minWordCount": integer (default: 3)
//   - "enableEmoticons": boolean (default: true)
//
// Logger is not configurable via file and will always be nil.
// Unknown fields in the JSON are ignored.
//
// For loading lexicon and modifier data, use config.LoadLexiconConfigFromFile.
//
// Returns an error if the file cannot be read or if the JSON is invalid.
func LoadAnalyzerConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file as JSON: %w", err)
	}

	return &cfg, nil
}
