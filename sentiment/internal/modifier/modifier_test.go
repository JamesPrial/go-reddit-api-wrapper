package modifier

import (
	"math"
	"testing"
)

// init initializes the modifier package with default negation words for testing.
func init() {
	defaultConfig := &ModifierConfig{
		NegationWords: []string{
			"not", "no", "never", "neither",
			"nobody", "nothing", "nowhere",
			"don't", "doesn't", "didn't",
			"won't", "can't", "shouldn't",
			"couldn't", "wouldn't",
			"isnt", "isn't", "wasnt", "wasn't",
			"havent", "haven't", "hasnt", "hasn't",
			"cant", "wont", "shouldnt", "couldnt", "wouldnt",
		},
	}
	if err := Init(defaultConfig); err != nil {
		panic(err)
	}
}

// TestDetectNegation tests the DetectNegation function.
func TestDetectNegation(t *testing.T) {
	tests := []struct {
		name     string
		tokens   []string
		index    int
		expected bool
	}{
		{
			name:     "no negation before word",
			tokens:   []string{"this", "is", "good"},
			index:    2,
			expected: false,
		},
		{
			name:     "negation immediately before word",
			tokens:   []string{"not", "good"},
			index:    1,
			expected: true,
		},
		{
			name:     "negation one token before",
			tokens:   []string{"is", "not", "good"},
			index:    2,
			expected: true,
		},
		{
			name:     "negation two tokens before",
			tokens:   []string{"it", "is", "not", "good"},
			index:    3,
			expected: true,
		},
		{
			name:     "negation three tokens before",
			tokens:   []string{"i", "think", "it", "is", "not", "good"},
			index:    5,
			expected: true,
		},
		{
			name:     "negation too far before (more than 3 tokens)",
			tokens:   []string{"i", "really", "truly", "think", "good"},
			index:    4,
			expected: false,
		},
		{
			name:     "negation after word is not detected",
			tokens:   []string{"good", "not"},
			index:    0,
			expected: false,
		},
		{
			name:     "no negation with no tokens",
			tokens:   []string{},
			index:    0,
			expected: false,
		},
		{
			name:     "no negation at index 0",
			tokens:   []string{"good"},
			index:    0,
			expected: false,
		},
		{
			name:     "different negation words",
			tokens:   []string{"never", "good"},
			index:    1,
			expected: true,
		},
		{
			name:     "multiple negations in lookback range",
			tokens:   []string{"no", "not", "never", "good"},
			index:    3,
			expected: true,
		},
		{
			name:     "contraction negation",
			tokens:   []string{"don't", "like"},
			index:    1,
			expected: true,
		},
		{
			name:     "contraction negation with apostrophe",
			tokens:   []string{"isn't", "great"},
			index:    1,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectNegation(tt.tokens, tt.index)
			if result != tt.expected {
				t.Errorf("DetectNegation(%v, %d) = %v, want %v", tt.tokens, tt.index, result, tt.expected)
			}
		})
	}
}

// TestCalculatePunctuationBoost tests the CalculatePunctuationBoost function.
func TestCalculatePunctuationBoost(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		minBoost   float64
		maxBoost   float64
		expectedEq float64 // exact match if set to non-zero
	}{
		{
			name:       "empty string",
			text:       "",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "no punctuation",
			text:       "hello world",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "single punctuation",
			text:       "hello!",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "double exclamation",
			text:       "hello!!",
			minBoost:   1.1,
			maxBoost:   1.1,
			expectedEq: 1.1,
		},
		{
			name:       "triple exclamation",
			text:       "hello!!!",
			minBoost:   1.2,
			maxBoost:   1.2,
			expectedEq: 1.2,
		},
		{
			name:       "five exclamations",
			text:       "hello!!!!!",
			minBoost:   1.4,
			maxBoost:   1.4,
			expectedEq: 1.4,
		},
		{
			name:       "many exclamations capped at 1.5",
			text:       "hello!!!!!!!!!",
			minBoost:   1.5,
			maxBoost:   1.5,
			expectedEq: 1.5,
		},
		{
			name:       "double question marks",
			text:       "what??",
			minBoost:   1.1,
			maxBoost:   1.1,
			expectedEq: 1.1,
		},
		{
			name:       "multiple punctuation sequences",
			text:       "amazing!!! and great??",
			minBoost:   1.2, // max consecutive is 3 (!!!)
			maxBoost:   1.2,
			expectedEq: 1.2,
		},
		{
			name:       "mixed punctuation not repeated",
			text:       "hello!? world?!",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "dots repeated",
			text:       "wait...",
			minBoost:   1.2,
			maxBoost:   1.2,
			expectedEq: 1.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculatePunctuationBoost(tt.text)

			if tt.expectedEq > 0 {
				if !almostEqual(result, tt.expectedEq) {
					t.Errorf("CalculatePunctuationBoost(%q) = %f, want %f", tt.text, result, tt.expectedEq)
				}
			} else {
				if result < tt.minBoost || result > tt.maxBoost {
					t.Errorf("CalculatePunctuationBoost(%q) = %f, want in range [%f, %f]", tt.text, result, tt.minBoost, tt.maxBoost)
				}
			}

			// Verify boost is always >= 1.0 and <= 1.5
			if result < 1.0 || result > 1.5 {
				t.Errorf("boost %f outside valid range [1.0, 1.5]", result)
			}
		})
	}
}

// TestCalculateCapsBoost tests the CalculateCapsBoost function.
func TestCalculateCapsBoost(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		minBoost   float64
		maxBoost   float64
		expectedEq float64 // exact match if set to non-zero
	}{
		{
			name:       "empty string",
			text:       "",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "no caps words",
			text:       "this is a test",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "single word all caps",
			text:       "This is GREAT",
			minBoost:   1.0,
			maxBoost:   1.1,
			expectedEq: 0, // approximately 1.05 (1/3 words in caps)
		},
		{
			name:       "two words all caps",
			text:       "THIS IS great",
			minBoost:   1.15,
			maxBoost:   1.25,
			expectedEq: 0, // approximately 1.2 (2/3 words in caps)
		},
		{
			name:       "all words all caps",
			text:       "THIS IS GREAT",
			minBoost:   1.25,
			maxBoost:   1.3,
			expectedEq: 1.3,
		},
		{
			name:       "single letter words ignored",
			text:       "I A TEST",
			minBoost:   1.0,
			maxBoost:   1.1,
			expectedEq: 0, // TEST is caps, but only 1/3 words
		},
		{
			name:       "mixed case not counted as caps",
			text:       "TeSt test",
			minBoost:   1.0,
			maxBoost:   1.0,
			expectedEq: 1.0,
		},
		{
			name:       "caps with punctuation",
			text:       "HELLO! WORLD?",
			minBoost:   1.25,
			maxBoost:   1.3,
			expectedEq: 1.3,
		},
		{
			name:       "fifty percent caps",
			text:       "HELLO world GOOD bad",
			minBoost:   1.1,
			maxBoost:   1.2,
			expectedEq: 0, // 2/4 words = 50% = 1.15
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCapsBoost(tt.text)

			if tt.expectedEq > 0 {
				if !almostEqual(result, tt.expectedEq) {
					t.Errorf("CalculateCapsBoost(%q) = %f, want %f", tt.text, result, tt.expectedEq)
				}
			} else {
				if result < tt.minBoost || result > tt.maxBoost {
					t.Errorf("CalculateCapsBoost(%q) = %f, want in range [%f, %f]", tt.text, result, tt.minBoost, tt.maxBoost)
				}
			}

			// Verify boost is always >= 1.0 and <= 1.3
			if result < 1.0 || result > 1.3 {
				t.Errorf("boost %f outside valid range [1.0, 1.3]", result)
			}
		})
	}
}

// TestApplyModifiers tests the ApplyModifiers function.
// NOTE: These tests verify that ApplyModifiers applies punctuation and
// capitalization boosts to a base score. Negation handling is NOT tested
// here because ApplyModifiers does not flip scores based on negation.
// Negation is handled earlier in the sentiment pipeline at the per-token
// level (in AnalyzeText), and the base scores passed to ApplyModifiers
// already have negation applied.
//
// HISTORICAL CONTEXT: In a previous buggy implementation, ApplyModifiers
// would flip the entire score if ANY negation word existed in the tokens.
// This caused "not good" to be classified as Positive because:
//  1. Per-token: "good" (+0.7) was correctly negated to -0.7
//  2. ApplyModifiers: -0.7 was incorrectly flipped AGAIN to +0.7 (BUG!)
//
// The tests "positive/negative score with negation - score NOT flipped"
// prevent regression of this bug.
func TestApplyModifiers(t *testing.T) {
	tests := []struct {
		name       string
		baseScore  float64
		text       string
		tokens     []string
		checkRange bool // if true, check min/max range instead of exact value
		minScore   float64
		maxScore   float64
		expectedEq float64 // exact match if set to non-zero
	}{
		{
			name:       "zero base score unchanged",
			baseScore:  0.0,
			text:       "test text",
			tokens:     []string{"test", "text"},
			expectedEq: 0.0,
		},
		{
			name:       "positive score with no modifiers",
			baseScore:  0.5,
			text:       "good",
			tokens:     []string{"good"},
			expectedEq: 0.5,
		},
		{
			name:       "negative score with no modifiers",
			baseScore:  -0.5,
			text:       "bad",
			tokens:     []string{"bad"},
			expectedEq: -0.5,
		},
		{
			name:       "positive score with negation - score NOT flipped",
			baseScore:  0.5,
			text:       "not good",
			tokens:     []string{"not", "good"},
			expectedEq: 0.5,
			// NOTE: ApplyModifiers does NOT handle negation flipping. Negation is
			// detected and applied earlier in the sentiment pipeline (in AnalyzeText)
			// on a per-token basis. This test verifies that ApplyModifiers works with
			// base scores that have already had negation applied.
		},
		{
			name:       "negative score with negation - score NOT flipped",
			baseScore:  -0.5,
			text:       "not bad",
			tokens:     []string{"not", "bad"},
			expectedEq: -0.5,
			// NOTE: ApplyModifiers does NOT handle negation flipping. The base score
			// passed here is already negation-adjusted from per-token analysis.
			// This test verifies punctuation/caps modifiers work independently.
		},
		{
			name:       "score boosted by punctuation",
			baseScore:  0.5,
			text:       "good!!!",
			tokens:     []string{"good"},
			checkRange: true,
			minScore:   0.5,
			maxScore:   0.8, // 0.5 * 1.5 = 0.75
		},
		{
			name:       "score boosted by caps",
			baseScore:  0.5,
			text:       "THIS IS GOOD",
			tokens:     []string{"this", "is", "good"},
			checkRange: true,
			minScore:   0.5,
			maxScore:   0.8, // 0.5 * 1.3 = 0.65
		},
		{
			name:       "score boosted by both caps and punctuation",
			baseScore:  0.5,
			text:       "THIS IS GOOD!!!",
			tokens:     []string{"this", "is", "good"},
			checkRange: true,
			minScore:   0.5,
			maxScore:   1.0, // 0.5 * 1.3 * 1.5 = 0.975, clamped to 1.0
		},
		{
			name:       "very positive score clamped to 1.0",
			baseScore:  2.0,
			text:       "GREAT!!!",
			tokens:     []string{"great"},
			expectedEq: 1.0,
		},
		{
			name:       "very negative score clamped to -1.0",
			baseScore:  -2.0,
			text:       "TERRIBLE!!!",
			tokens:     []string{"terrible"},
			expectedEq: -1.0,
		},
		{
			name:       "empty tokens no modifiers",
			baseScore:  0.5,
			text:       "test",
			tokens:     []string{},
			expectedEq: 0.5,
		},
		{
			name:       "empty text no modifiers",
			baseScore:  0.5,
			text:       "",
			tokens:     []string{"test"},
			expectedEq: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyModifiers(tt.baseScore, tt.text, tt.tokens)

			if tt.checkRange {
				if result < tt.minScore || result > tt.maxScore {
					t.Errorf("ApplyModifiers = %f, want in range [%f, %f]", result, tt.minScore, tt.maxScore)
				}
			} else if tt.expectedEq != 0 || (tt.expectedEq == 0 && tt.baseScore == 0) {
				if !almostEqual(result, tt.expectedEq) {
					t.Errorf("ApplyModifiers = %f, want %f", result, tt.expectedEq)
				}
			}

			// Verify score is always in valid range
			if result < -1.0 || result > 1.0 {
				t.Errorf("score %f outside valid range [-1.0, 1.0]", result)
			}
		})
	}
}

// TestModifiersIntegration tests modifiers working together.
// NOTE: These tests use pre-adjusted base scores where negation has already been
// applied (from per-token negation detection in AnalyzeText). ApplyModifiers only
// applies punctuation and capitalization boosts, not negation flipping.
func TestModifiersIntegration(t *testing.T) {
	t.Run("positive score boosted by punctuation", func(t *testing.T) {
		baseScore := 0.8
		text := "good!!!"
		tokens := []string{"good"}

		result := ApplyModifiers(baseScore, text, tokens)

		// Should boost the positive score with punctuation modifier
		if result <= baseScore {
			t.Errorf("expected score boosted from %f, got %f", baseScore, result)
		}
		// Should not exceed bounds
		if result > 1.0 {
			t.Errorf("expected score clamped to 1.0, got %f", result)
		}
	})

	t.Run("negative score with caps and punctuation boosts", func(t *testing.T) {
		baseScore := -0.5
		text := "TERRIBLE!!!"
		tokens := []string{"terrible"}

		result := ApplyModifiers(baseScore, text, tokens)

		// Should boost the magnitude (more negative)
		if result >= baseScore {
			t.Errorf("expected magnitude increased from %f, got %f", baseScore, result)
		}

		// Should apply boosts (more negative than -0.5)
		if result > -0.5 {
			t.Errorf("expected more negative score with boosts, got %f", result)
		}
	})

	t.Run("boosts don't exceed bounds", func(t *testing.T) {
		baseScore := 1.0
		text := "AMAZING!!!"
		tokens := []string{"amazing"}

		result := ApplyModifiers(baseScore, text, tokens)

		if result > 1.0 {
			t.Errorf("expected clamped score, got %f", result)
		}
	})
}

// TestPunctuationBoostRange tests punctuation boost stays in range.
func TestPunctuationBoostRange(t *testing.T) {
	tests := []string{
		"",
		"test",
		"test!",
		"test!!",
		"test!!!",
		"test!!!!",
		"test!!!!!",
		"test!!!!!!",
	}

	for _, text := range tests {
		result := CalculatePunctuationBoost(text)
		if result < 1.0 || result > 1.5 {
			t.Errorf("CalculatePunctuationBoost(%q) = %f, expected range [1.0, 1.5]", text, result)
		}
	}
}

// TestCapsBoostRange tests caps boost stays in range.
func TestCapsBoostRange(t *testing.T) {
	tests := []string{
		"",
		"test",
		"TEST",
		"THIS IS TEST",
		"THIS IS A TEST",
		"THIS IS A TEST CASE",
	}

	for _, text := range tests {
		result := CalculateCapsBoost(text)
		if result < 1.0 || result > 1.3 {
			t.Errorf("CalculateCapsBoost(%q) = %f, expected range [1.0, 1.3]", text, result)
		}
	}
}

// almostEqual checks if two float64 values are approximately equal.
func almostEqual(a, b float64) bool {
	epsilon := 0.0001
	return math.Abs(a-b) < epsilon
}

// TestInit tests the Init function with various configuration scenarios.
func TestInit(t *testing.T) {
	tests := []struct {
		name      string
		config    *ModifierConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "nil config",
			config:    nil,
			expectErr: true,
			errMsg:    "config cannot be nil",
		},
		{
			name:      "nil negation words slice",
			config:    &ModifierConfig{NegationWords: nil},
			expectErr: true,
			errMsg:    "NegationWords slice cannot be nil",
		},
		{
			name:      "empty negation words slice",
			config:    &ModifierConfig{NegationWords: []string{}},
			expectErr: true,
			errMsg:    "NegationWords slice cannot be empty",
		},
		{
			name: "valid single negation word",
			config: &ModifierConfig{
				NegationWords: []string{"not"},
			},
			expectErr: false,
		},
		{
			name: "valid multiple negation words",
			config: &ModifierConfig{
				NegationWords: []string{"not", "no", "never", "neither"},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("Init() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if tt.expectErr && err.Error() != tt.errMsg {
				t.Errorf("Init() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestInitUpdatesNegationWords verifies that Init properly initializes the negation words.
func TestInitUpdatesNegationWords(t *testing.T) {
	// Initialize with a simple set of negation words
	config := &ModifierConfig{
		NegationWords: []string{"not", "no"},
	}
	err := Init(config)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify detection works with initialized words
	if !DetectNegation([]string{"not", "good"}, 1) {
		t.Errorf("expected 'not' to be detected as negation word")
	}

	if !DetectNegation([]string{"no", "good"}, 1) {
		t.Errorf("expected 'no' to be detected as negation word")
	}

	// Reinitialize with different words
	config2 := &ModifierConfig{
		NegationWords: []string{"never"},
	}
	err = Init(config2)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify the new word is detected
	if !DetectNegation([]string{"never", "good"}, 1) {
		t.Errorf("expected 'never' to be detected as negation word after reinitialization")
	}

	// Verify old words are no longer in the map
	if DetectNegation([]string{"not", "good"}, 1) {
		t.Errorf("expected 'not' to NOT be detected after reinitialization with different words")
	}
}

// TestNegationWordsList tests that all expected negation words are recognized.
func TestNegationWordsList(t *testing.T) {
	negationWordsList := []string{
		"not", "no", "never", "neither",
		"don't", "doesn't", "didn't",
		"won't", "can't", "shouldn't",
		"couldn't", "wouldn't",
		"isn't", "wasn't", "haven't", "hasn't",
	}

	// Reinitialize with the expected words to ensure consistent state
	config := &ModifierConfig{
		NegationWords: negationWordsList,
	}
	if err := Init(config); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	for _, word := range negationWordsList {
		tokens := []string{word, "good"}
		if !DetectNegation(tokens, 1) {
			t.Errorf("expected negation word %q to be detected", word)
		}
	}
}

// TestBoostEdgeCases tests edge cases in boost calculations.
func TestBoostEdgeCases(t *testing.T) {
	t.Run("punctuation boost with only special characters", func(t *testing.T) {
		result := CalculatePunctuationBoost("!!!...???")
		if result < 1.0 || result > 1.5 {
			t.Errorf("unexpected boost for special characters: %f", result)
		}
	})

	t.Run("caps boost with only single letters", func(t *testing.T) {
		result := CalculateCapsBoost("I A B C")
		if result != 1.0 {
			t.Errorf("expected no boost for single letters, got %f", result)
		}
	})

	t.Run("caps boost with numbers only", func(t *testing.T) {
		result := CalculateCapsBoost("123 456 789")
		if result != 1.0 {
			t.Errorf("expected no boost for numbers only, got %f", result)
		}
	})
}
