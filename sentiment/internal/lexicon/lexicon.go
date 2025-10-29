// Package lexicon provides internal implementation details for sentiment analysis.
// The lexicon must be initialized by calling Init() with a LexiconConfig
// before any of the getter functions can be used.
package lexicon

import (
	"maps"
	"sync"
)

// Error represents a lexicon initialization or configuration error.
type Error struct {
	message string
}

// Error returns the error message.
func (e *Error) Error() string {
	return e.message
}

// NewError creates a new lexicon error with the given message.
func NewError(message string) *Error {
	return &Error{message: message}
}

// Package-level variables initialized from configuration.
// These are protected by a sync.Once to ensure thread-safe initialization.
// initMu protects initErr access to ensure safe concurrent reads.
var (
	positiveWords map[string]float64
	negativeWords map[string]float64
	emoticons     map[string]float64
	initOnce      sync.Once
	initErr       error
	initMu        sync.RWMutex
)

// LexiconConfig contains the configuration for sentiment lexicons.
// All fields must be non-nil and non-empty for valid configuration.
type LexiconConfig struct {
	// PositiveWords maps positive sentiment words to their scores.
	// Scores range from 1.0 to 2.0, with higher values indicating stronger positive sentiment.
	PositiveWords map[string]float64
	// NegativeWords maps negative sentiment words to their scores.
	// Scores range from -1.0 to -2.0, with lower values indicating stronger negative sentiment.
	NegativeWords map[string]float64
	// Emoticons maps common emoticons to their sentiment scores.
	// Scores range from -1.2 to 1.5, indicating negative or positive sentiment.
	Emoticons map[string]float64
}

// DefaultLexiconConfig returns a LexiconConfig with default lexicon values.
// The returned configuration contains the default positive words, negative words,
// and emoticons suitable for general sentiment analysis.
func DefaultLexiconConfig() *LexiconConfig {
	return &LexiconConfig{
		PositiveWords: map[string]float64{
			"good":        1.0,
			"great":       1.5,
			"excellent":   2.0,
			"love":        1.5,
			"happy":       1.5,
			"amazing":     2.0,
			"wonderful":   1.8,
			"fantastic":   1.8,
			"awesome":     1.8,
			"brilliant":   2.0,
			"perfect":     2.0,
			"beautiful":   1.8,
			"superb":      1.8,
			"marvelous":   1.8,
			"outstanding": 2.0,
			"incredible":  1.8,
			"fabulous":    1.8,
			"delightful":  1.5,
			"lovely":      1.5,
			"nice":        1.0,
			"cool":        1.0,
			"impressed":   1.3,
			"proud":       1.5,
			"joyful":      1.5,
			"blessed":     1.3,
		},
		NegativeWords: map[string]float64{
			"bad":           -1.0,
			"terrible":      -1.5,
			"awful":         -2.0,
			"hate":          -1.5,
			"horrible":      -2.0,
			"worst":         -2.0,
			"disappointing": -1.3,
			"disgusting":    -2.0,
			"pathetic":      -1.8,
			"atrocious":     -2.0,
			"miserable":     -1.5,
			"dreadful":      -1.8,
			"tedious":       -1.0,
			"boring":        -1.0,
			"ugly":          -1.5,
			"stupid":        -1.5,
			"ridiculous":    -1.3,
			"shameful":      -1.5,
			"useless":       -1.5,
			"abysmal":       -2.0,
			"offensive":     -1.3,
			"mediocre":      -0.8,
			"poor":          -1.0,
			"waste":         -1.3,
			"sad":           -1.2,
		},
		Emoticons: map[string]float64{
			":)":  0.5,
			":-)": 0.5,
			":-D": 1.0,
			":D":  1.0,
			":P":  0.5,
			";)":  0.5,
			"😊":   0.8,
			"😄":   0.8,
			"😍":   1.5,
			"❤️":  1.5,
			"🥰":   1.3,
			"😂":   1.0,
			":(":  -0.5,
			":-(": -0.5,
			":'(": -0.8,
			"😞":   -0.8,
			"😡":   -1.2,
			"😤":   -1.0,
			"😔":   -0.8,
			"💔":   -1.5,
			"😠":   -1.2,
		},
	}
}

// Init initializes the lexicon package with the provided configuration.
// This function must be called once before using GetPositiveWords, GetNegativeWords,
// or GetEmoticons. Subsequent calls have no effect due to sync.Once protection.
//
// The function validates that:
//   - config is not nil
//   - all maps in the config are not nil and not empty
//
// If validation fails, an error is returned and the package remains uninitialized.
// The error can be retrieved on subsequent calls to understand initialization state.
func Init(config *LexiconConfig) error {
	initOnce.Do(func() {
		if config == nil {
			initMu.Lock()
			initErr = NewError("lexicon config cannot be nil")
			initMu.Unlock()
			return
		}

		if config.PositiveWords == nil || len(config.PositiveWords) == 0 {
			initMu.Lock()
			initErr = NewError("positive words map must not be nil or empty")
			initMu.Unlock()
			return
		}

		if config.NegativeWords == nil || len(config.NegativeWords) == 0 {
			initMu.Lock()
			initErr = NewError("negative words map must not be nil or empty")
			initMu.Unlock()
			return
		}

		if config.Emoticons == nil || len(config.Emoticons) == 0 {
			initMu.Lock()
			initErr = NewError("emoticons map must not be nil or empty")
			initMu.Unlock()
			return
		}

		// Copy maps to package-level variables
		positiveWords = make(map[string]float64, len(config.PositiveWords))
		maps.Copy(positiveWords, config.PositiveWords)

		negativeWords = make(map[string]float64, len(config.NegativeWords))
		maps.Copy(negativeWords, config.NegativeWords)

		emoticons = make(map[string]float64, len(config.Emoticons))
		maps.Copy(emoticons, config.Emoticons)
	})

	initMu.RLock()
	defer initMu.RUnlock()
	return initErr
}

// GetPositiveWords returns the positive words lexicon.
// The returned map must not be modified by callers.
// Init must be called before using this function.
// Panics if called before Init() completes successfully.
func GetPositiveWords() map[string]float64 {
	if positiveWords == nil {
		panic("lexicon: GetPositiveWords called before Init()")
	}
	return positiveWords
}

// GetNegativeWords returns the negative words lexicon.
// The returned map must not be modified by callers.
// Init must be called before using this function.
// Panics if called before Init() completes successfully.
func GetNegativeWords() map[string]float64 {
	if negativeWords == nil {
		panic("lexicon: GetNegativeWords called before Init()")
	}
	return negativeWords
}

// GetEmoticons returns the emoticons lexicon.
// The returned map must not be modified by callers.
// Init must be called before using this function.
// Panics if called before Init() completes successfully.
func GetEmoticons() map[string]float64 {
	if emoticons == nil {
		panic("lexicon: GetEmoticons called before Init()")
	}
	return emoticons
}
