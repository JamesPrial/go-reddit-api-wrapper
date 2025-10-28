package sentiment

import "log/slog"

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
