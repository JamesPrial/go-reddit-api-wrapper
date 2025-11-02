// Package config provides configuration management for sentiment analysis.
// It handles loading and validating sentiment lexicons and modifiers from
// embedded defaults or external files.
package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

//go:embed lexicon_default.json modifier_default.json
var defaultConfigFS embed.FS

// LexiconConfig contains the sentiment lexicons for analysis.
type LexiconConfig struct {
	PositiveWords map[string]float64 `json:"positive_words"`
	NegativeWords map[string]float64 `json:"negative_words"`
	Emoticons     map[string]float64 `json:"emoticons"`
}

// ModifierConfig contains configuration for sentiment modifiers like negation.
type ModifierConfig struct {
	NegationWords []string `json:"negation_words"`
}

// Config combines lexicon and modifier configurations for sentiment analysis.
type Config struct {
	Lexicon  LexiconConfig
	Modifier ModifierConfig
}

// rawConfig is the JSON structure for loading from files or readers.
type rawConfig struct {
	PositiveWords map[string]float64 `json:"positive_words"`
	NegativeWords map[string]float64 `json:"negative_words"`
	Emoticons     map[string]float64 `json:"emoticons"`
	NegationWords []string           `json:"negation_words"`
}

// LoadDefaultConfig loads the default sentiment configuration from embedded files.
// It combines lexicon_default.json and modifier_default.json into a single Config.
//
// Returns an error if the embedded files cannot be read or if validation fails.
func LoadDefaultConfig() (*Config, error) {
	lexiconData, err := defaultConfigFS.ReadFile("lexicon_default.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded lexicon_default.json: %w", err)
	}

	modifierData, err := defaultConfigFS.ReadFile("modifier_default.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded modifier_default.json: %w", err)
	}

	var lexConfig rawConfig
	if err := json.Unmarshal(lexiconData, &lexConfig); err != nil {
		return nil, fmt.Errorf("failed to parse lexicon_default.json: %w", err)
	}

	var modConfig rawConfig
	if err := json.Unmarshal(modifierData, &modConfig); err != nil {
		return nil, fmt.Errorf("failed to parse modifier_default.json: %w", err)
	}

	cfg := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: lexConfig.PositiveWords,
			NegativeWords: lexConfig.NegativeWords,
			Emoticons:     lexConfig.Emoticons,
		},
		Modifier: ModifierConfig{
			NegationWords: modConfig.NegationWords,
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("default config validation failed: %w", err)
	}

	return cfg, nil
}

// LoadLexiconConfigFromFile loads sentiment lexicon and modifier configuration from a JSON file.
// This function loads lexicon data (positive words, negative words, emoticons) and
// modifier configuration (negation words) from an external file.
// The file should contain all four fields: positive_words, negative_words, emoticons,
// and negation_words.
//
// For loading analyzer-specific settings (MinWordCount, EnableEmoticons),
// use LoadAnalyzerConfigFromFile from the sentiment package.
//
// Returns an error if the file cannot be read, parsed, or validated.
func LoadLexiconConfigFromFile(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	return parseConfig(data, fmt.Sprintf("config file %q", path))
}

// LoadConfigFromReader loads sentiment configuration from an io.Reader.
// The reader should provide JSON content with all four fields: positive_words,
// negative_words, emoticons, and negation_words.
//
// Returns an error if the content cannot be parsed or validated.
func LoadConfigFromReader(r io.Reader) (*Config, error) {
	if r == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}

	return parseConfig(data, "reader")
}

// parseConfig parses raw JSON data into a Config struct.
func parseConfig(data []byte, source string) (*Config, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", source)
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", source, err)
	}

	cfg := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: raw.PositiveWords,
			NegativeWords: raw.NegativeWords,
			Emoticons:     raw.Emoticons,
		},
		Modifier: ModifierConfig{
			NegationWords: raw.NegationWords,
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed from %s: %w", source, err)
	}

	return cfg, nil
}

// Validate checks that the configuration has all required fields and they are not nil.
// It also validates that sentiment scores are within valid ranges:
//   - Positive words: (0, MAX_SCORE]
//   - Negative words: [MIN_SCORE, 0)
//   - Emoticons: [MIN_SCORE, MAX_SCORE]
//
// Returns an error if validation fails.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if c.Lexicon.PositiveWords == nil {
		return fmt.Errorf("positive_words map cannot be nil")
	}

	if c.Lexicon.NegativeWords == nil {
		return fmt.Errorf("negative_words map cannot be nil")
	}

	if c.Lexicon.Emoticons == nil {
		return fmt.Errorf("emoticons map cannot be nil")
	}

	if c.Modifier.NegationWords == nil {
		return fmt.Errorf("negation_words slice cannot be nil")
	}

	// Validate positive word scores are in valid range (0, MAX_SCORE]
	for word, score := range c.Lexicon.PositiveWords {
		if score <= 0 || score > MAX_SCORE {
			return fmt.Errorf("positive word %q has invalid score %f (expected range: 0 < score <= %f)", word, score, MAX_SCORE)
		}
	}

	// Validate negative word scores are in valid range [MIN_SCORE, 0)
	for word, score := range c.Lexicon.NegativeWords {
		if score >= 0 || score < MIN_SCORE {
			return fmt.Errorf("negative word %q has invalid score %f (expected range: %f <= score < 0)", word, score, MIN_SCORE)
		}
	}

	// Validate emoticon scores are in valid range [MIN_SCORE, MAX_SCORE]
	for emoticon, score := range c.Lexicon.Emoticons {
		if score < MIN_SCORE || score > MAX_SCORE {
			return fmt.Errorf("emoticon %q has invalid score %f (expected range: %f <= score <= %f)", emoticon, score, MIN_SCORE, MAX_SCORE)
		}
	}

	return nil
}
