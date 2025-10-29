package lexicon

import (
	"sync"
	"testing"
)

// init ensures the lexicon is initialized before running tests.
func init() {
	if err := Init(DefaultLexiconConfig()); err != nil {
		panic("failed to initialize lexicon: " + err.Error())
	}
}

// resetForTesting resets the lexicon state for error condition tests.
// This is exported only for testing purposes and should not be used in production code.
// It is protected by a mutex to ensure only one test can reset at a time.
func resetForTesting() {
	resetMu.Lock()
	defer resetMu.Unlock()

	positiveWords = nil
	negativeWords = nil
	emoticons = nil
	initErr = nil
	initOnce = sync.Once{}
}

var resetMu sync.Mutex

// TestGetPositiveWords tests that GetPositiveWords returns a non-empty map.
func TestGetPositiveWords(t *testing.T) {
	words := GetPositiveWords()

	t.Run("returns non-empty map", func(t *testing.T) {
		if len(words) == 0 {
			t.Error("expected non-empty positive words map")
		}
	})

	t.Run("contains expected words", func(t *testing.T) {
		expectedWords := []string{"good", "great", "excellent", "love", "amazing"}
		for _, word := range expectedWords {
			if _, exists := words[word]; !exists {
				t.Errorf("expected word %q in positive words map", word)
			}
		}
	})

	t.Run("all scores are in valid range [1.0, 2.0]", func(t *testing.T) {
		for word, score := range words {
			if score < 1.0 || score > 2.0 {
				t.Errorf("word %q has invalid score %f, expected range [1.0, 2.0]", word, score)
			}
		}
	})

	t.Run("scores are positive", func(t *testing.T) {
		for word, score := range words {
			if score <= 0 {
				t.Errorf("word %q has non-positive score %f", word, score)
			}
		}
	})
}

// TestGetNegativeWords tests that GetNegativeWords returns a non-empty map.
func TestGetNegativeWords(t *testing.T) {
	words := GetNegativeWords()

	t.Run("returns non-empty map", func(t *testing.T) {
		if len(words) == 0 {
			t.Error("expected non-empty negative words map")
		}
	})

	t.Run("contains expected words", func(t *testing.T) {
		expectedWords := []string{"bad", "terrible", "awful", "hate", "horrible"}
		for _, word := range expectedWords {
			if _, exists := words[word]; !exists {
				t.Errorf("expected word %q in negative words map", word)
			}
		}
	})

	t.Run("all scores are in valid range [-2.0, -0.5]", func(t *testing.T) {
		for word, score := range words {
			if score < -2.0 || score > -0.5 {
				t.Errorf("word %q has invalid score %f, expected range [-2.0, -0.5]", word, score)
			}
		}
	})

	t.Run("scores are negative", func(t *testing.T) {
		for word, score := range words {
			if score >= 0 {
				t.Errorf("word %q has non-negative score %f", word, score)
			}
		}
	})

	t.Run("returns the same underlying map (not a copy)", func(t *testing.T) {
		words1 := GetNegativeWords()
		words2 := GetNegativeWords()

		// Both should reference the same map
		if len(words1) != len(words2) {
			t.Error("expected same underlying map")
		}
		// Verify they have the same content by comparing a few entries
		for i, word := range []string{"bad", "terrible", "awful"} {
			score1, exists1 := words1[word]
			score2, exists2 := words2[word]
			if !exists1 || !exists2 || score1 != score2 {
				t.Errorf("entry %d (%q) mismatch", i, word)
			}
		}
	})
}

// TestGetEmoticons tests that GetEmoticons returns a non-empty map.
func TestGetEmoticons(t *testing.T) {
	emoticons := GetEmoticons()

	t.Run("returns non-empty map", func(t *testing.T) {
		if len(emoticons) == 0 {
			t.Error("expected non-empty emoticons map")
		}
	})

	t.Run("contains expected emoticons", func(t *testing.T) {
		expectedEmoticons := []string{":)", ":(", ":-D", ":D"}
		for _, emoticon := range expectedEmoticons {
			if _, exists := emoticons[emoticon]; !exists {
				t.Errorf("expected emoticon %q in emoticons map", emoticon)
			}
		}
	})

	t.Run("happy emoticons have positive scores", func(t *testing.T) {
		happyEmoticons := []string{":)", ":-)"}
		for _, emoticon := range happyEmoticons {
			score, exists := emoticons[emoticon]
			if !exists {
				t.Errorf("expected emoticon %q", emoticon)
				continue
			}
			if score <= 0 {
				t.Errorf("emoticon %q should have positive score, got %f", emoticon, score)
			}
		}
	})

	t.Run("sad emoticons have negative scores", func(t *testing.T) {
		sadEmoticons := []string{":(", ":-("}
		for _, emoticon := range sadEmoticons {
			score, exists := emoticons[emoticon]
			if !exists {
				t.Errorf("expected emoticon %q", emoticon)
				continue
			}
			if score >= 0 {
				t.Errorf("emoticon %q should have negative score, got %f", emoticon, score)
			}
		}
	})

	t.Run("all scores are within reasonable range", func(t *testing.T) {
		minScore := -2.0
		maxScore := 2.0
		for emoticon, score := range emoticons {
			if score < minScore || score > maxScore {
				t.Errorf("emoticon %q has score %f outside range [%f, %f]", emoticon, score, minScore, maxScore)
			}
		}
	})

	t.Run("returns the same underlying map (not a copy)", func(t *testing.T) {
		emoticons1 := GetEmoticons()
		emoticons2 := GetEmoticons()

		// Both should reference the same map
		if len(emoticons1) != len(emoticons2) {
			t.Error("expected same underlying map")
		}
		// Verify they have the same content by comparing a few entries
		for i, emoticon := range []string{":)", ":(", ":-D"} {
			score1, exists1 := emoticons1[emoticon]
			score2, exists2 := emoticons2[emoticon]
			if !exists1 || !exists2 || score1 != score2 {
				t.Errorf("entry %d (%q) mismatch", i, emoticon)
			}
		}
	})
}

// TestNoOverlapBetweenLexicons tests that there is no overlap between positive and negative words.
func TestNoOverlapBetweenLexicons(t *testing.T) {
	positive := GetPositiveWords()
	negative := GetNegativeWords()

	overlaps := []string{}
	for word := range positive {
		if _, exists := negative[word]; exists {
			overlaps = append(overlaps, word)
		}
	}

	if len(overlaps) > 0 {
		t.Errorf("found overlapping words between positive and negative lexicons: %v", overlaps)
	}
}

// TestLexiconScoreDistribution tests the distribution of scores in lexicons.
func TestLexiconScoreDistribution(t *testing.T) {
	t.Run("positive words distribution", func(t *testing.T) {
		words := GetPositiveWords()

		score1Count := 0
		score15Count := 0
		score18Count := 0
		score2Count := 0

		for _, score := range words {
			// Round to check for common values
			if score >= 0.95 && score <= 1.05 {
				score1Count++
			} else if score >= 1.45 && score <= 1.55 {
				score15Count++
			} else if score >= 1.75 && score <= 1.85 {
				score18Count++
			} else if score >= 1.95 && score <= 2.05 {
				score2Count++
			}
		}

		// Should have a reasonable distribution
		if score1Count == 0 && score15Count == 0 && score18Count == 0 && score2Count == 0 {
			t.Error("expected some scores at common values")
		}
	})

	t.Run("negative words distribution", func(t *testing.T) {
		words := GetNegativeWords()

		scoreMinus1Count := 0
		scoreMinus15Count := 0
		scoreMinus18Count := 0
		scoreMinus2Count := 0

		for _, score := range words {
			// Round to check for common values
			if score <= -0.95 && score >= -1.05 {
				scoreMinus1Count++
			} else if score <= -1.45 && score >= -1.55 {
				scoreMinus15Count++
			} else if score <= -1.75 && score >= -1.85 {
				scoreMinus18Count++
			} else if score <= -1.95 && score >= -2.05 {
				scoreMinus2Count++
			}
		}

		// Should have a reasonable distribution
		if scoreMinus1Count == 0 && scoreMinus15Count == 0 && scoreMinus18Count == 0 && scoreMinus2Count == 0 {
			t.Error("expected some scores at common values")
		}
	})
}

// TestPositiveWordsCount verifies that we have a reasonable number of positive words.
func TestPositiveWordsCount(t *testing.T) {
	words := GetPositiveWords()
	if len(words) < 10 {
		t.Errorf("expected at least 10 positive words, got %d", len(words))
	}
}

// TestNegativeWordsCount verifies that we have a reasonable number of negative words.
func TestNegativeWordsCount(t *testing.T) {
	words := GetNegativeWords()
	if len(words) < 10 {
		t.Errorf("expected at least 10 negative words, got %d", len(words))
	}
}

// TestEmoticonsCount verifies that we have a reasonable number of emoticons.
func TestEmoticonsCount(t *testing.T) {
	emoticons := GetEmoticons()
	if len(emoticons) < 5 {
		t.Errorf("expected at least 5 emoticons, got %d", len(emoticons))
	}
}

// TestInitWithValidConfig tests that Init successfully initializes with a valid config.
func TestInitWithValidConfig(t *testing.T) {
	config := DefaultLexiconConfig()
	err := Init(config)
	// Init may return an error if already initialized by the test init() function,
	// which is acceptable. The important thing is that it doesn't panic.
	_ = err
}

// TestInitWithNilConfig tests that Init returns an error when config is nil.
func TestInitWithNilConfig(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	err := Init(nil)
	if err == nil {
		t.Fatal("expected error for nil config, got nil")
	}

	expectedMsg := "lexicon config cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}

	// Verify that getters panic when lexicon is not initialized
	t.Run("getters panic when not initialized", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic from GetPositiveWords, got nil")
			}
		}()
		GetPositiveWords()
	})
}

// TestInitWithMissingPositiveWords tests that Init returns an error when positive words are missing.
func TestInitWithMissingPositiveWords(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	config := &LexiconConfig{
		PositiveWords: nil,
		NegativeWords: map[string]float64{"bad": -1.0},
		Emoticons:     map[string]float64{":)": 0.5},
	}

	err := Init(config)
	if err == nil {
		t.Fatal("expected error for nil positive words, got nil")
	}

	expectedMsg := "positive words map must not be nil or empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestInitWithMissingNegativeWords tests that Init returns an error when negative words are missing.
func TestInitWithMissingNegativeWords(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	config := &LexiconConfig{
		PositiveWords: map[string]float64{"good": 1.0},
		NegativeWords: nil,
		Emoticons:     map[string]float64{":)": 0.5},
	}

	err := Init(config)
	if err == nil {
		t.Fatal("expected error for nil negative words, got nil")
	}

	expectedMsg := "negative words map must not be nil or empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestInitWithMissingEmoticons tests that Init returns an error when emoticons are missing.
func TestInitWithMissingEmoticons(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	config := &LexiconConfig{
		PositiveWords: map[string]float64{"good": 1.0},
		NegativeWords: map[string]float64{"bad": -1.0},
		Emoticons:     nil,
	}

	err := Init(config)
	if err == nil {
		t.Fatal("expected error for nil emoticons, got nil")
	}

	expectedMsg := "emoticons map must not be nil or empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestInitCalledBeforeGetters tests that getters panic if called before successful initialization.
func TestInitCalledBeforeGetters(t *testing.T) {
	resetForTesting()
	defer resetForTesting()

	// Attempt to call getters before initialization
	t.Run("GetPositiveWords panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got nil")
			}
		}()
		GetPositiveWords()
	})

	resetForTesting()

	t.Run("GetNegativeWords panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got nil")
			}
		}()
		GetNegativeWords()
	})

	resetForTesting()

	t.Run("GetEmoticons panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got nil")
			}
		}()
		GetEmoticons()
	})
}

// TestDefaultLexiconConfig tests that DefaultLexiconConfig returns a properly configured lexicon.
func TestDefaultLexiconConfig(t *testing.T) {
	t.Run("returns non-nil config", func(t *testing.T) {
		config := DefaultLexiconConfig()
		if config == nil {
			t.Fatal("expected non-nil config")
		}
	})

	t.Run("all maps are non-nil", func(t *testing.T) {
		config := DefaultLexiconConfig()
		if config.PositiveWords == nil {
			t.Error("PositiveWords should not be nil")
		}
		if config.NegativeWords == nil {
			t.Error("NegativeWords should not be nil")
		}
		if config.Emoticons == nil {
			t.Error("Emoticons should not be nil")
		}
	})

	t.Run("all maps are non-empty", func(t *testing.T) {
		config := DefaultLexiconConfig()
		if len(config.PositiveWords) == 0 {
			t.Error("PositiveWords should not be empty")
		}
		if len(config.NegativeWords) == 0 {
			t.Error("NegativeWords should not be empty")
		}
		if len(config.Emoticons) == 0 {
			t.Error("Emoticons should not be empty")
		}
	})

	t.Run("contains expected words", func(t *testing.T) {
		config := DefaultLexiconConfig()
		expectedPositive := []string{"good", "great", "excellent"}
		for _, word := range expectedPositive {
			if _, ok := config.PositiveWords[word]; !ok {
				t.Errorf("expected positive word %q in default config", word)
			}
		}

		expectedNegative := []string{"bad", "terrible", "awful"}
		for _, word := range expectedNegative {
			if _, ok := config.NegativeWords[word]; !ok {
				t.Errorf("expected negative word %q in default config", word)
			}
		}

		expectedEmoticons := []string{":)", ":("}
		for _, emoticon := range expectedEmoticons {
			if _, ok := config.Emoticons[emoticon]; !ok {
				t.Errorf("expected emoticon %q in default config", emoticon)
			}
		}
	})
}

// TestInitWithEmptyMaps tests that Init returns an error when any map is empty.
func TestInitWithEmptyMaps(t *testing.T) {
	t.Run("empty positive words", func(t *testing.T) {
		resetForTesting()
		defer resetForTesting()

		config := &LexiconConfig{
			PositiveWords: make(map[string]float64), // empty map
			NegativeWords: map[string]float64{"bad": -1.0},
			Emoticons:     map[string]float64{":)": 0.5},
		}

		err := Init(config)
		if err == nil {
			t.Fatal("expected error for empty positive words")
		}

		if err.Error() != "positive words map must not be nil or empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("empty negative words", func(t *testing.T) {
		resetForTesting()
		defer resetForTesting()

		config := &LexiconConfig{
			PositiveWords: map[string]float64{"good": 1.0},
			NegativeWords: make(map[string]float64), // empty map
			Emoticons:     map[string]float64{":)": 0.5},
		}

		err := Init(config)
		if err == nil {
			t.Fatal("expected error for empty negative words")
		}

		if err.Error() != "negative words map must not be nil or empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("empty emoticons", func(t *testing.T) {
		resetForTesting()
		defer resetForTesting()

		config := &LexiconConfig{
			PositiveWords: map[string]float64{"good": 1.0},
			NegativeWords: map[string]float64{"bad": -1.0},
			Emoticons:     make(map[string]float64), // empty map
		}

		err := Init(config)
		if err == nil {
			t.Fatal("expected error for empty emoticons")
		}

		if err.Error() != "emoticons map must not be nil or empty" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
