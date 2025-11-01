package preprocessor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Preprocessor handles text preprocessing for sentiment analysis.
// It provides methods for tokenization, normalization, and text feature extraction.
type Preprocessor struct {
}

// NewPreprocessor creates a new Preprocessor instance.
func NewPreprocessor() *Preprocessor {
	return &Preprocessor{}
}

// Tokenize splits text into tokens (words) by converting to lowercase
// and trimming punctuation from the edges of each token while preserving
// internal punctuation (e.g., "don't" remains intact).
func (p *Preprocessor) Tokenize(text string) []string {
	if text == "" {
		return nil
	}

	// Split on whitespace
	words := strings.Fields(text)
	tokens := make([]string, 0, len(words))

	for _, word := range words {
		// Convert to lowercase
		word = strings.ToLower(word)

		// Trim punctuation from edges only
		word = strings.TrimFunc(word, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})

		if word != "" {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

// Normalize converts text to lowercase and normalizes Unicode characters.
// This prepares text for consistent comparison and matching.
func (p *Preprocessor) Normalize(text string) string {
	if text == "" {
		return ""
	}

	// Convert to lowercase
	normalized := strings.ToLower(text)

	// Normalize Unicode (NFD decomposition would go here in production)
	// For now, just ensure it's valid UTF-8
	if !utf8.ValidString(normalized) {
		// Replace invalid UTF-8 sequences
		normalized = strings.ToValidUTF8(normalized, "")
	}

	return normalized
}

// IsDeleted checks if the author string represents a deleted or removed post/comment.
// Returns true if the author is "[deleted]" or "[removed]".
func (p *Preprocessor) IsDeleted(author string) bool {
	author = strings.TrimSpace(author)
	return author == "[deleted]" || author == "[removed]"
}

// CountCaps counts the number of ALL CAPS words in the text.
// A word is considered ALL CAPS if it contains at least 2 uppercase letters
// and all letters are uppercase.
func (p *Preprocessor) CountCaps(text string) int {
	if text == "" {
		return 0
	}

	words := strings.Fields(text)
	count := 0

	for _, word := range words {
		upperCount := 0
		letterCount := 0

		for _, r := range word {
			if unicode.IsLetter(r) {
				letterCount++
				if unicode.IsUpper(r) {
					upperCount++
				}
			}
		}

		// Word is ALL CAPS if it has at least 2 letters and all are uppercase
		if letterCount >= 2 && upperCount == letterCount {
			count++
		}
	}

	return count
}

// CountPunctuation counts repeated punctuation sequences in the text.
// Looks for patterns like "!!!", "???", "...", etc. where the same punctuation
// character appears 2 or more times consecutively.
func (p *Preprocessor) CountPunctuation(text string) int {
	if text == "" {
		return 0
	}

	count := 0
	runes := []rune(text)

	for i := 0; i < len(runes)-1; i++ {
		curr := runes[i]
		next := runes[i+1]

		// Check if both characters are punctuation and the same
		if isPunctuation(curr) && curr == next {
			// Count this as one instance of repeated punctuation
			count++

			// Skip any additional consecutive occurrences of the same punctuation
			for i+1 < len(runes) && runes[i+1] == curr {
				i++
			}
		}
	}

	return count
}

// isPunctuation checks if a rune is a punctuation character.
func isPunctuation(r rune) bool {
	return unicode.IsPunct(r)
}
