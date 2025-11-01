package lexicon

import (
	"math"
	"testing"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestLexicon creates a Lexicon instance for testing with sample data.
func createTestLexicon(t *testing.T) *Lexicon {
	t.Helper()

	positiveWords := map[string]float64{
		"good":      1.0,
		"great":     1.5,
		"excellent": 2.0,
		"love":      1.5,
		"amazing":   2.0,
	}

	negativeWords := map[string]float64{
		"bad":      -1.0,
		"terrible": -1.5,
		"awful":    -2.0,
		"hate":     -1.5,
		"horrible": -2.0,
	}

	emoticons := map[string]float64{
		":)":  0.5,
		":-)": 0.5,
		":D":  1.0,
		":-D": 1.0,
		":(":  -0.5,
		":-(": -0.5,
	}

	negationWords := []string{
		"not", "no", "never", "neither",
		"nobody", "nothing", "nowhere",
		"don't", "doesn't", "didn't",
		"won't", "can't", "shouldn't",
		"couldn't", "wouldn't",
		"isnt", "isn't", "wasnt", "wasn't",
		"havent", "haven't", "hasnt", "hasn't",
		"cant", "wont", "shouldnt", "couldnt", "wouldnt",
	}

	lex, err := NewLexicon(positiveWords, negativeWords, emoticons, negationWords)
	if err != nil {
		t.Fatalf("failed to create test lexicon: %v", err)
	}
	return lex
}

// almostEqual checks if two float64 values are approximately equal.
func almostEqual(a, b float64) bool {
	epsilon := 0.0001
	return math.Abs(a-b) < epsilon
}

// ============================================================================
// Constructor and Configuration Tests
// ============================================================================

// TestNewLexicon tests the NewLexicon constructor with various configurations.
func TestNewLexicon(t *testing.T) {
	tests := []struct {
		name          string
		positiveWords map[string]float64
		negativeWords map[string]float64
		emoticons     map[string]float64
		negationWords []string
		expectErr     bool
		errMsg        string
	}{
		{
			name:          "nil positive words",
			positiveWords: nil,
			negativeWords: map[string]float64{"bad": -1.0},
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: []string{"not"},
			expectErr:     true,
			errMsg:        "positive words map must not be nil or empty",
		},
		{
			name:          "empty positive words",
			positiveWords: map[string]float64{},
			negativeWords: map[string]float64{"bad": -1.0},
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: []string{"not"},
			expectErr:     true,
			errMsg:        "positive words map must not be nil or empty",
		},
		{
			name:          "nil negative words",
			positiveWords: map[string]float64{"good": 1.0},
			negativeWords: nil,
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: []string{"not"},
			expectErr:     true,
			errMsg:        "negative words map must not be nil or empty",
		},
		{
			name:          "empty negative words",
			positiveWords: map[string]float64{"good": 1.0},
			negativeWords: map[string]float64{},
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: []string{"not"},
			expectErr:     true,
			errMsg:        "negative words map must not be nil or empty",
		},
		{
			name:          "nil negation words",
			positiveWords: map[string]float64{"good": 1.0},
			negativeWords: map[string]float64{"bad": -1.0},
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: nil,
			expectErr:     true,
			errMsg:        "NegationWords slice cannot be nil",
		},
		{
			name:          "empty negation words",
			positiveWords: map[string]float64{"good": 1.0},
			negativeWords: map[string]float64{"bad": -1.0},
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: []string{},
			expectErr:     true,
			errMsg:        "NegationWords slice cannot be empty",
		},
		{
			name:          "valid configuration",
			positiveWords: map[string]float64{"good": 1.0},
			negativeWords: map[string]float64{"bad": -1.0},
			emoticons:     map[string]float64{":)": 0.5},
			negationWords: []string{"not"},
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lex, err := NewLexicon(tt.positiveWords, tt.negativeWords, tt.emoticons, tt.negationWords)

			if (err != nil) != tt.expectErr {
				t.Errorf("NewLexicon() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectErr && err.Error() != tt.errMsg {
				t.Errorf("NewLexicon() error message = %v, want %v", err.Error(), tt.errMsg)
			}

			if !tt.expectErr && lex == nil {
				t.Error("NewLexicon() returned nil lexicon without error")
			}
		})
	}
}

// ============================================================================
// Negation Detection Tests
// ============================================================================

// TestDetectNegation tests the DetectNegation method.
func TestDetectNegation(t *testing.T) {
	lex := createTestLexicon(t)

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
			result := lex.DetectNegation(tt.tokens, tt.index)
			if result != tt.expected {
				t.Errorf("DetectNegation(%v, %d) = %v, want %v", tt.tokens, tt.index, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Punctuation Boost Tests
// ============================================================================

// TestGetPunctuationMultiplier tests the GetPunctuationMultiplier method.
func TestGetPunctuationMultiplier(t *testing.T) {
	lex := createTestLexicon(t)

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
			result := lex.GetPunctuationMultiplier(tt.text)

			if tt.expectedEq > 0 {
				if !almostEqual(result, tt.expectedEq) {
					t.Errorf("GetPunctuationMultiplier(%q) = %f, want %f", tt.text, result, tt.expectedEq)
				}
			} else {
				if result < tt.minBoost || result > tt.maxBoost {
					t.Errorf("GetPunctuationMultiplier(%q) = %f, want in range [%f, %f]", tt.text, result, tt.minBoost, tt.maxBoost)
				}
			}

			// Verify boost is always >= 1.0 and <= 1.5
			if result < 1.0 || result > 1.5 {
				t.Errorf("boost %f outside valid range [1.0, 1.5]", result)
			}
		})
	}
}

// TestPunctuationMultiplierRange tests punctuation multiplier stays in range.
func TestPunctuationMultiplierRange(t *testing.T) {
	lex := createTestLexicon(t)

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
		result := lex.GetPunctuationMultiplier(text)
		if result < 1.0 || result > 1.5 {
			t.Errorf("GetPunctuationMultiplier(%q) = %f, expected range [1.0, 1.5]", text, result)
		}
	}
}

// ============================================================================
// Capitalization Boost Tests
// ============================================================================

// TestGetCapsMultiplier tests the GetCapsMultiplier method.
func TestGetCapsMultiplier(t *testing.T) {
	lex := createTestLexicon(t)

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
			expectedEq: 0, // approximately 1.1 (1/3 words in caps)
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
			result := lex.GetCapsMultiplier(tt.text)

			if tt.expectedEq > 0 {
				if !almostEqual(result, tt.expectedEq) {
					t.Errorf("GetCapsMultiplier(%q) = %f, want %f", tt.text, result, tt.expectedEq)
				}
			} else {
				if result < tt.minBoost || result > tt.maxBoost {
					t.Errorf("GetCapsMultiplier(%q) = %f, want in range [%f, %f]", tt.text, result, tt.minBoost, tt.maxBoost)
				}
			}

			// Verify boost is always >= 1.0 and <= 1.3
			if result < 1.0 || result > 1.3 {
				t.Errorf("boost %f outside valid range [1.0, 1.3]", result)
			}
		})
	}
}

// TestCapsMultiplierRange tests caps multiplier stays in range.
func TestCapsMultiplierRange(t *testing.T) {
	lex := createTestLexicon(t)

	tests := []string{
		"",
		"test",
		"TEST",
		"THIS IS TEST",
		"THIS IS A TEST",
		"THIS IS A TEST CASE",
	}

	for _, text := range tests {
		result := lex.GetCapsMultiplier(text)
		if result < 1.0 || result > 1.3 {
			t.Errorf("GetCapsMultiplier(%q) = %f, expected range [1.0, 1.3]", text, result)
		}
	}
}

// ============================================================================
// Multiplier Tests
// ============================================================================

// TestGetMultiplier tests the GetMultiplier method which combines punctuation and caps boosts.
func TestGetMultiplier(t *testing.T) {
	lex := createTestLexicon(t)

	tests := []struct {
		name    string
		text    string
		minMult float64
		maxMult float64
	}{
		{
			name:    "no modifiers",
			text:    "test",
			minMult: 1.0,
			maxMult: 1.0,
		},
		{
			name:    "punctuation only",
			text:    "test!!!",
			minMult: 1.2,
			maxMult: 1.2,
		},
		{
			name:    "caps only",
			text:    "THIS IS TEST",
			minMult: 1.25,
			maxMult: 1.3,
		},
		{
			name:    "both punctuation and caps",
			text:    "THIS IS TEST!!!",
			minMult: 1.4,
			maxMult: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lex.GetMultiplier(tt.text)

			if result < tt.minMult || result > tt.maxMult {
				t.Errorf("GetMultiplier(%q) = %f, want in range [%f, %f]", tt.text, result, tt.minMult, tt.maxMult)
			}

			// Verify multiplier is always >= 1.0
			if result < 1.0 {
				t.Errorf("multiplier %f below valid minimum 1.0", result)
			}
		})
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

// TestBoostEdgeCases tests edge cases in boost calculations.
func TestBoostEdgeCases(t *testing.T) {
	lex := createTestLexicon(t)

	t.Run("punctuation multiplier with only special characters", func(t *testing.T) {
		result := lex.GetPunctuationMultiplier("!!!...???")
		if result < 1.0 || result > 1.5 {
			t.Errorf("unexpected multiplier for special characters: %f", result)
		}
	})

	t.Run("caps multiplier with only single letters", func(t *testing.T) {
		result := lex.GetCapsMultiplier("I A B C")
		if result != 1.0 {
			t.Errorf("expected no multiplier for single letters, got %f", result)
		}
	})

	t.Run("caps multiplier with numbers only", func(t *testing.T) {
		result := lex.GetCapsMultiplier("123 456 789")
		if result != 1.0 {
			t.Errorf("expected no multiplier for numbers only, got %f", result)
		}
	})
}
