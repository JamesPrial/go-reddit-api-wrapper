// Package lexicon provides sentiment analysis lexicon data and modifier functions.
// It combines word sentiment scores (positive/negative words, emoticons) with
// sentiment modifiers (negation detection, punctuation emphasis, capitalization emphasis).
//
// The package uses a constructor pattern where a Lexicon instance is created with
// all necessary configuration and provides both lexicon lookups and modifier calculations.
package lexicon

import (
	"math"
	"strings"
	"unicode"

	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/config"
)

// Modifier constants define the behavior of sentiment modifiers.

// Error represents a lexicon initialization or configuration error.
type ConfigError struct {
	message string
}

// Error returns the error message.
func (e *ConfigError) Error() string {
	return e.message
}

// NewError creates a new lexicon error with the given message.
func NewError(message string) *ConfigError {
	return &ConfigError{message: message}
}

// Lexicon provides sentiment analysis capabilities including word lookups
// and sentiment modifiers. Instances are created via NewLexicon.
type Lexicon struct {
	positiveWords map[string]float64
	negativeWords map[string]float64
	emoticons     map[string]float64
	negationWords map[string]bool
}

// NewLexicon creates a new Lexicon instance with the provided configuration.
// It validates the configuration and initializes all internal data structures.
//
// Parameters:
//   - config: LexiconConfig containing word maps and negation words
//
// Returns:
//   - *Lexicon: Configured lexicon instance
//   - error: Validation error if configuration is invalid
//
// Returns an error if:
//   - config is nil
//   - Any word map (positive, negative, emoticons) is nil or empty
//   - NegationWords slice is nil or empty
func NewLexicon(positiveWords map[string]float64, negativeWords map[string]float64, emoticons map[string]float64, negationWords []string) (*Lexicon, error) {
	if len(positiveWords) == 0 {
		return nil, NewError("positive words map must not be nil or empty")
	}
	if len(negativeWords) == 0 {
		return nil, NewError("negative words map must not be nil or empty")
	}
	if negationWords == nil {
		return nil, NewError("NegationWords slice cannot be nil")
	}

	if len(negationWords) == 0 {
		return nil, NewError("NegationWords slice cannot be empty")
	}

	// Convert negation words slice to map for O(1) lookups
	negationWordsMap := make(map[string]bool, len(negationWords))
	for _, word := range negationWords {
		negationWordsMap[word] = true
	}

	return &Lexicon{
		positiveWords: positiveWords,
		negativeWords: negativeWords,
		emoticons:     emoticons,
		negationWords: negationWordsMap,
	}, nil
}

// GetScore returns the sentiment score for a word, or 0.0 if not found.
func (l *Lexicon) GetScore(word string) float64 {
	if score, ok := l.positiveWords[word]; ok {
		return score
	}
	if score, ok := l.negativeWords[word]; ok {
		return score
	}
	if score, ok := l.emoticons[word]; ok {
		return score
	}
	return 0.0
}

// IsPositive returns true if the word has a positive sentiment.
func (l *Lexicon) IsPositive(word string) bool {
	_, ok := l.positiveWords[word]
	return ok
}

// IsNegative returns true if the word has a negative sentiment.
func (l *Lexicon) IsNegative(word string) bool {
	_, ok := l.negativeWords[word]
	return ok
}

// DetectNegation checks if a word at the given index is negated by preceding words.
// It looks back up to 3 tokens maximum to find negation words like "not", "no", "never", etc.
// Returns true if the word is negated, false otherwise.
//
// For example, in the phrase "not very good", the word "good" at index 2 would be
// detected as negated because "not" appears 2 positions before it.
func (l *Lexicon) DetectNegation(tokens []string, index int) bool {
	if index <= 0 || len(tokens) == 0 {
		return false
	}

	// Look back up to 3 tokens
	lookbackStart := max(0, index-config.MAX_NEGATION_LOOKBACK)

	// Check preceding tokens for negation words
	for i := lookbackStart; i < index; i++ {
		if l.negationWords[tokens[i]] {
			return true
		}
	}

	return false
}

// ExtractEmoticons finds all emoticons in the given text.
// It searches for both text-based emoticons (e.g., ":)", ":-D") and emoji characters.
func (l *Lexicon) ExtractEmoticons(text string) []string {
	if text == "" {
		return nil
	}

	emoticonsMap := l.emoticons
	found := make([]string, 0)
	seen := make(map[string]bool)

	for emoticon := range emoticonsMap {
		if strings.Contains(text, emoticon) && !seen[emoticon] {
			found = append(found, emoticon)
			seen[emoticon] = true
		}
	}

	return found
}

// CalculatePunctuationBoost calculates a score boost based on repeated punctuation
// in the text. The boost accounts for emphasis conveyed through repeated exclamation
// marks, question marks, or other punctuation patterns.
//
// The boost returns a multiplier between 1.0 (no punctuation emphasis) and 1.5
// (strong punctuation emphasis). For example:
// - "great!" = 1.0 boost (single punctuation, no boost)
// - "great!!" = 1.1 boost (double emphasis)
// - "great!!!" = 1.2 boost (triple emphasis)
// - "great!!!!!!!!" = 1.5 boost (max capped at 1.5)
//
// Returns a multiplier value to apply to sentiment scores.
func (l *Lexicon) GetPunctuationMultiplier(text string) float64 {
	if text == "" {
		return config.NO_PUNCTUATION_BOOST
	}

	// Count repeated punctuation sequences
	runes := []rune(text)
	maxConsecutive := 0
	currentConsecutive := 0
	lastPunct := rune(0)

	for _, r := range runes {
		if (r == '!' || r == '?' || r == '.') && r == lastPunct {
			currentConsecutive++
		} else if r == '!' || r == '?' || r == '.' {
			lastPunct = r
			currentConsecutive = 1
		} else {
			if currentConsecutive > maxConsecutive {
				maxConsecutive = currentConsecutive
			}
			currentConsecutive = 0
			lastPunct = 0
		}
	}

	// Check final sequence
	if currentConsecutive > maxConsecutive {
		maxConsecutive = currentConsecutive
	}

	// Convert consecutive count to boost multiplier
	// No boost for single punctuation or none
	if maxConsecutive <= 1 {
		return config.NO_PUNCTUATION_BOOST
	}

	// Scale from 1.1 (2 consecutive) to 1.5 (6+ consecutive)
	// Formula: 1.0 + (min(maxConsecutive - 1, 5) * 0.1)
	boost := config.NO_PUNCTUATION_BOOST + math.Min(float64(maxConsecutive-1), config.MAX_PUNCTUATION_BOOST_CONSECUTIVE)*config.PUNCTUATION_BOOST_CONSECUTIVE_FACTOR
	return math.Min(boost, config.MAX_PUNCTUATION_BOOST)
}

// CalculateCapsBoost calculates a score boost based on the percentage of ALL CAPS words
// in the text. ALL CAPS words typically indicate emphasis or stronger sentiment.
//
// The boost returns a multiplier between 1.0 (no caps words) and 1.3 (high percentage
// of caps words). For example:
// - "This is great" = 1.0 boost (no all-caps words)
// - "This is GREAT" = 1.1 boost (1/3 words in caps)
// - "THIS IS GREAT" = 1.3 boost (all words in caps)
//
// Returns a multiplier value to apply to sentiment scores.
func (l *Lexicon) GetCapsMultiplier(text string) float64 {
	if text == "" {
		return config.NO_CAPS_BOOST
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return config.NO_CAPS_BOOST
	}

	capsCount := 0
	for _, word := range words {
		if isAllCaps(word) {
			capsCount++
		}
	}

	capsPercentage := float64(capsCount) / float64(len(words))
	boost := config.NO_CAPS_BOOST + (capsPercentage * config.CAPS_BOOST_PERCENTAGE_FACTOR)
	return math.Min(boost, config.MAX_CAPS_BOOST)
}

// GetMultiplier applies sentiment modifiers (punctuation and capitalization) to a base score.
func (l *Lexicon) GetMultiplier(text string) float64 {
	multiplier := 1.0
	if text == "" {
		return multiplier
	}
	punctuationMultiplier := l.GetPunctuationMultiplier(text)
	multiplier *= punctuationMultiplier

	capsMultiplier := l.GetCapsMultiplier(text)
	multiplier *= capsMultiplier

	return multiplier
}

func boundScore(score float64) float64 {
	if score > config.MAX_SCORE {
		return config.MAX_SCORE
	} else if score < config.MIN_SCORE {
		return config.MIN_SCORE
	}
	return score
}

// isAllCaps checks if a word is in ALL CAPS format.
// A word is considered ALL CAPS if it has at least 2 letters and all letters are uppercase.
func isAllCaps(word string) bool {
	letterCount := 0
	upperCount := 0

	for _, r := range word {
		if isLetter(r) {
			letterCount++
			if isUpper(r) {
				upperCount++
			}
		}
	}

	// Must have at least 2 letters and all must be uppercase
	return letterCount >= 2 && letterCount == upperCount
}

// isLetter checks if a rune is a Unicode letter.
func isLetter(r rune) bool {
	return unicode.IsLetter(r)
}

// isUpper checks if a rune is an uppercase letter.
func isUpper(r rune) bool {
	return unicode.IsUpper(r)
}
