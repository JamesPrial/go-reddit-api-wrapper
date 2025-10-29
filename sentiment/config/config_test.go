package config

import (
	"strings"
	"testing"
)

func TestValidate_NilConfig(t *testing.T) {
	var c *Config
	err := c.Validate()
	if err == nil {
		t.Error("expected error for nil config, got nil")
	}
	if !strings.Contains(err.Error(), "config cannot be nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NilPositiveWords(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: nil,
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for nil positive_words, got nil")
	}
	if !strings.Contains(err.Error(), "positive_words map cannot be nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NilNegativeWords(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: nil,
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for nil negative_words, got nil")
	}
	if !strings.Contains(err.Error(), "negative_words map cannot be nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NilEmoticons(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     nil,
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for nil emoticons, got nil")
	}
	if !strings.Contains(err.Error(), "emoticons map cannot be nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NilNegationWords(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: nil,
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for nil negation_words, got nil")
	}
	if !strings.Contains(err.Error(), "negation_words slice cannot be nil") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// Positive word score validation tests
func TestValidate_PositiveWordScoreZero(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{"good": 0},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for positive word with score 0, got nil")
	}
	if !strings.Contains(err.Error(), "positive word \"good\" has invalid score 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PositiveWordScoreNegative(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{"great": -0.5},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for positive word with negative score, got nil")
	}
	if !strings.Contains(err.Error(), "positive word \"great\" has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PositiveWordScoreTooHigh(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{"excellent": 2.5},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for positive word with score > 2.0, got nil")
	}
	if !strings.Contains(err.Error(), "positive word \"excellent\" has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_PositiveWordScoreAtUpperBound(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{"good": 2.0},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for positive word with score 2.0, got: %v", err)
	}
}

func TestValidate_PositiveWordScoreJustAboveZero(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{"ok": 0.1},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for positive word with score 0.1, got: %v", err)
	}
}

// Negative word score validation tests
func TestValidate_NegativeWordScoreZero(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{"bad": 0},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for negative word with score 0, got nil")
	}
	if !strings.Contains(err.Error(), "negative word \"bad\" has invalid score 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NegativeWordScorePositive(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{"terrible": 0.5},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for negative word with positive score, got nil")
	}
	if !strings.Contains(err.Error(), "negative word \"terrible\" has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NegativeWordScoreTooLow(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{"awful": -2.5},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for negative word with score < -2.0, got nil")
	}
	if !strings.Contains(err.Error(), "negative word \"awful\" has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_NegativeWordScoreAtLowerBound(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{"bad": -2.0},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for negative word with score -2.0, got: %v", err)
	}
}

func TestValidate_NegativeWordScoreJustBelowZero(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{"meh": -0.1},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for negative word with score -0.1, got: %v", err)
	}
}

// Emoticon score validation tests
func TestValidate_EmoticonScoreTooHigh(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{":)": 2.5},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for emoticon with score > 2.0, got nil")
	}
	if !strings.Contains(err.Error(), "emoticon \":)\" has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_EmoticonScoreTooLow(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{":(": -2.5},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for emoticon with score < -2.0, got nil")
	}
	if !strings.Contains(err.Error(), "emoticon \":(\" has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_EmoticonScoreAtPositiveUpperBound(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{":)": 2.0},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for emoticon with score 2.0, got: %v", err)
	}
}

func TestValidate_EmoticonScoreAtNegativeLowerBound(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{":(": -2.0},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for emoticon with score -2.0, got: %v", err)
	}
}

func TestValidate_EmoticonScoreAtZero(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{"|": 0},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for emoticon with score 0, got: %v", err)
	}
}

// Valid configurations
func TestValidate_ValidMinimalConfig(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for valid minimal config, got: %v", err)
	}
}

func TestValidate_ValidFullConfig(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{
				"good":      0.5,
				"great":     1.5,
				"excellent": 2.0,
			},
			NegativeWords: map[string]float64{
				"bad":      -0.5,
				"terrible": -1.5,
				"awful":    -2.0,
			},
			Emoticons: map[string]float64{
				":)":  1.0,
				":(":  -1.0,
				"|":   0.0,
				":D":  2.0,
				":'(": -2.0,
			},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{"not", "no", "never"},
		},
	}
	err := c.Validate()
	if err != nil {
		t.Errorf("expected no error for valid full config, got: %v", err)
	}
}

func TestValidate_MultipleInvalidPositiveWords(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{
				"good": 0.5,
				"bad":  -0.5, // invalid
			},
			NegativeWords: map[string]float64{},
			Emoticons:     map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for invalid positive word, got nil")
	}
	if !strings.Contains(err.Error(), "has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_MultipleInvalidNegativeWords(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{
				"bad":      -0.5,
				"terrible": 1.5, // invalid
			},
			Emoticons: map[string]float64{},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for invalid negative word, got nil")
	}
	if !strings.Contains(err.Error(), "has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidate_MultipleInvalidEmoticons(t *testing.T) {
	c := &Config{
		Lexicon: LexiconConfig{
			PositiveWords: map[string]float64{},
			NegativeWords: map[string]float64{},
			Emoticons: map[string]float64{
				":)": 1.0,
				":D": 3.0, // invalid
			},
		},
		Modifier: ModifierConfig{
			NegationWords: []string{},
		},
	}
	err := c.Validate()
	if err == nil {
		t.Error("expected error for invalid emoticon, got nil")
	}
	if !strings.Contains(err.Error(), "has invalid score") {
		t.Errorf("unexpected error message: %v", err)
	}
}
