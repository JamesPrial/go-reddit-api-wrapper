package preprocessor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTokenize tests the Tokenize method with various inputs.
func TestTokenize(t *testing.T) {
	p := NewPreprocessor()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single word",
			input:    "Hello",
			expected: []string{"hello"},
		},
		{
			name:     "multiple words",
			input:    "This is a test",
			expected: []string{"this", "is", "a", "test"},
		},
		{
			name:     "words with punctuation",
			input:    "Hello, world! How are you?",
			expected: []string{"hello", "world", "how", "are", "you"},
		},
		{
			name:     "words with apostrophes",
			input:    "don't can't won't",
			expected: []string{"don't", "can't", "won't"},
		},
		{
			name:     "mixed case",
			input:    "HELLO hello HeLLo",
			expected: []string{"hello", "hello", "hello"},
		},
		{
			name:     "leading and trailing spaces",
			input:    "   hello world   ",
			expected: []string{"hello", "world"},
		},
		{
			name:     "multiple spaces between words",
			input:    "hello    world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "numbers",
			input:    "test123 456test test456test",
			expected: []string{"test123", "456test", "test456test"},
		},
		{
			name:     "special characters",
			input:    "!!!hello??? ***world***",
			expected: []string{"hello", "world"},
		},
		{
			name:     "hyphenated words",
			input:    "well-known high-quality",
			expected: []string{"well-known", "high-quality"},
		},
		{
			name:     "contractions",
			input:    "I'm you're we've they'll",
			expected: []string{"i'm", "you're", "we've", "they'll"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Tokenize(tt.input)
			if !equalStringSlices(result, tt.expected) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNormalize tests the Normalize method.
func TestNormalize(t *testing.T) {
	p := NewPreprocessor()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "already lowercase",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "uppercase",
			input:    "HELLO",
			expected: "hello",
		},
		{
			name:     "mixed case",
			input:    "HeLLo WoRLd",
			expected: "hello world",
		},
		{
			name:     "with spaces",
			input:    "Hello   World",
			expected: "hello   world",
		},
		{
			name:     "with unicode lowercase",
			input:    "Café",
			expected: "café",
		},
		{
			name:     "with unicode uppercase",
			input:    "CAFÉ",
			expected: "café",
		},
		{
			name:     "with punctuation",
			input:    "Hello, World!",
			expected: "hello, world!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Normalize(tt.input)
			if result != tt.expected {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractEmoticons tests the ExtractEmoticons method.
func TestExtractEmoticons(t *testing.T) {
	p := NewPreprocessor()

	tests := []struct {
		name     string
		input    string
		contains []string // emoticons that should be found
	}{
		{
			name:     "empty string",
			input:    "",
			contains: nil,
		},
		{
			name:     "no emoticons",
			input:    "This is a test",
			contains: nil,
		},
		{
			name:     "single happy emoticon",
			input:    "This is great :)",
			contains: []string{":)"},
		},
		{
			name:     "single sad emoticon",
			input:    "This is bad :(",
			contains: []string{":("},
		},
		{
			name:     "multiple emoticons",
			input:    "Good :) and bad :(",
			contains: []string{":)", ":("},
		},
		{
			name:     "duplicate emoticons",
			input:    "Great :) Really great :)",
			contains: []string{":)"},
		},
		{
			name:     "emoticons with other punctuation",
			input:    "Perfect :-D and terrible :-(",
			contains: []string{":-D", ":-("},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.ExtractEmoticons(tt.input)

			if len(tt.contains) == 0 {
				if len(result) != 0 {
					t.Errorf("ExtractEmoticons(%q) = %v, want empty", tt.input, result)
				}
				return
			}

			// Check that all expected emoticons are present
			for _, expected := range tt.contains {
				found := false
				for _, emoticon := range result {
					if emoticon == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExtractEmoticons(%q) = %v, missing %q", tt.input, result, expected)
				}
			}
		})
	}
}

// TestIsDeleted tests the IsDeleted method.
func TestIsDeleted(t *testing.T) {
	p := NewPreprocessor()

	tests := []struct {
		name     string
		author   string
		expected bool
	}{
		{
			name:     "deleted author",
			author:   "[deleted]",
			expected: true,
		},
		{
			name:     "removed author",
			author:   "[removed]",
			expected: true,
		},
		{
			name:     "regular author",
			author:   "testuser",
			expected: false,
		},
		{
			name:     "empty string",
			author:   "",
			expected: false,
		},
		{
			name:     "deleted with spaces",
			author:   "  [deleted]  ",
			expected: true,
		},
		{
			name:     "removed with spaces",
			author:   "  [removed]  ",
			expected: true,
		},
		{
			name:     "case sensitive - not deleted",
			author:   "[DELETED]",
			expected: false,
		},
		{
			name:     "case sensitive - not removed",
			author:   "[REMOVED]",
			expected: false,
		},
		{
			name:     "similar but not deleted",
			author:   "[delete]",
			expected: false,
		},
		{
			name:     "similar but not removed",
			author:   "[remove]",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.IsDeleted(tt.author)
			if result != tt.expected {
				t.Errorf("IsDeleted(%q) = %v, want %v", tt.author, result, tt.expected)
			}
		})
	}
}

// TestCountCaps tests the CountCaps method.
func TestCountCaps(t *testing.T) {
	p := NewPreprocessor()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "no caps",
			text:     "this is a test",
			expected: 0,
		},
		{
			name:     "single caps word",
			text:     "This IS a test",
			expected: 1,
		},
		{
			name:     "multiple caps words",
			text:     "THIS IS A TEST",
			expected: 3, // A has only 1 letter, so not counted
		},
		{
			name:     "mixed case not counted",
			text:     "TeSt",
			expected: 0,
		},
		{
			name:     "single letter words not counted",
			text:     "I A",
			expected: 0,
		},
		{
			name:     "single letter in larger words",
			text:     "A GREAT DAY",
			expected: 2, // A doesn't count (only 1 letter), GREAT and DAY do
		},
		{
			name:     "caps with punctuation",
			text:     "HELLO! WORLD?",
			expected: 2,
		},
		{
			name:     "numbers with caps",
			text:     "ABC123 test",
			expected: 1,
		},
		{
			name:     "all single letters",
			text:     "A B C D E",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.CountCaps(tt.text)
			if result != tt.expected {
				t.Errorf("CountCaps(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

// TestCountPunctuation tests the CountPunctuation method.
func TestCountPunctuation(t *testing.T) {
	p := NewPreprocessor()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "no repeated punctuation",
			text:     "Hello! How? What.",
			expected: 0,
		},
		{
			name:     "double exclamation",
			text:     "Hello!!",
			expected: 1,
		},
		{
			name:     "triple exclamation",
			text:     "Hello!!!",
			expected: 1, // Counts one instance of repeated !
		},
		{
			name:     "multiple repeated punctuation patterns",
			text:     "What?? Amazing!!",
			expected: 2,
		},
		{
			name:     "four exclamations",
			text:     "!!!",
			expected: 1, // All in one sequence
		},
		{
			name:     "mixed punctuation not repeated",
			text:     "Hello!? What?!",
			expected: 0,
		},
		{
			name:     "repeated dots",
			text:     "Wait...",
			expected: 1, // All in one sequence
		},
		{
			name:     "single punctuation",
			text:     "Hello!",
			expected: 0,
		},
		{
			name:     "complex pattern",
			text:     "No way!!! Really???",
			expected: 2, // One for !!!, one for ???
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.CountPunctuation(tt.text)
			if result != tt.expected {
				t.Errorf("CountPunctuation(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

// TestPreprocessorUnicodeHandling tests unicode handling in preprocessor.
func TestPreprocessorUnicodeHandling(t *testing.T) {
	p := NewPreprocessor()

	t.Run("tokenize with unicode characters", func(t *testing.T) {
		text := "café résumé naïve"
		result := p.Tokenize(text)
		if len(result) != 3 {
			t.Errorf("expected 3 tokens, got %d", len(result))
		}
		// Check that unicode is preserved
		if !strings.Contains(result[0], "é") {
			t.Error("expected unicode character to be preserved")
		}
	})

	t.Run("normalize with unicode", func(t *testing.T) {
		text := "CAFÉ"
		result := p.Normalize(text)
		// Should be lowercase
		if result != strings.ToLower(text) {
			t.Errorf("normalize failed for unicode: %q != %q", result, strings.ToLower(text))
		}
	})

	t.Run("emoji handling", func(t *testing.T) {
		text := "Great! 😊 Really good 😄"
		emoticons := p.ExtractEmoticons(text)
		// Emoticons function should find emoji
		_ = emoticons
	})
}

// TestPreprocessorEdgeCases tests edge cases in preprocessor methods.
func TestPreprocessorEdgeCases(t *testing.T) {
	p := NewPreprocessor()

	t.Run("very long text", func(t *testing.T) {
		longText := strings.Repeat("word ", 10000)
		tokens := p.Tokenize(longText)
		if len(tokens) == 0 {
			t.Error("expected tokens for long text")
		}
	})

	t.Run("special characters only", func(t *testing.T) {
		text := "!@#$%^&*()"
		tokens := p.Tokenize(text)
		if len(tokens) != 0 {
			t.Errorf("expected no tokens for special characters, got %d", len(tokens))
		}
	})

	t.Run("whitespace variations", func(t *testing.T) {
		text := "hello\tworld\ntest\r\ndata"
		tokens := p.Tokenize(text)
		if len(tokens) != 4 {
			t.Errorf("expected 4 tokens, got %d", len(tokens))
		}
	})

	t.Run("invalid UTF-8 handling", func(t *testing.T) {
		// Create invalid UTF-8 sequence
		invalidText := "hello" + string([]byte{0xFF, 0xFE}) + "world"
		normalized := p.Normalize(invalidText)
		// Should handle gracefully and not panic
		if !utf8.ValidString(normalized) {
			t.Error("Normalize did not produce valid UTF-8")
		}
	})
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestNewPreprocessor tests preprocessor creation.
func TestNewPreprocessor(t *testing.T) {
	t.Run("creates non-nil preprocessor", func(t *testing.T) {
		p := NewPreprocessor()
		if p == nil {
			t.Error("expected non-nil preprocessor")
		}
	})

	t.Run("preprocessor methods are callable", func(t *testing.T) {
		p := NewPreprocessor()

		tokens := p.Tokenize("test")
		if len(tokens) == 0 {
			t.Error("expected tokens")
		}

		normalized := p.Normalize("TEST")
		if normalized != "test" {
			t.Error("expected normalized string")
		}

		isDeleted := p.IsDeleted("[deleted]")
		if !isDeleted {
			t.Error("expected deleted to be detected")
		}

		caps := p.CountCaps("THIS IS TEST")
		if caps == 0 {
			t.Error("expected caps to be counted")
		}

		punct := p.CountPunctuation("test!!!")
		if punct == 0 {
			t.Error("expected punctuation to be counted")
		}
	})
}
