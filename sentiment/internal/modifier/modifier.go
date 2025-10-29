package modifier

import (
	"math"
	"strings"
	"unicode"
)

// negationWords contains words that typically negate or invert the sentiment
// of the word that follows them.
var negationWords = map[string]bool{
	"not":       true,
	"no":        true,
	"never":     true,
	"neither":   true,
	"nobody":    true,
	"nothing":   true,
	"nowhere":   true,
	"don't":     true,
	"doesn't":   true,
	"didn't":    true,
	"won't":     true,
	"can't":     true,
	"shouldn't": true,
	"couldn't":  true,
	"wouldn't":  true,
	"isnt":      true,
	"isn't":     true,
	"wasnt":     true,
	"wasn't":    true,
	"havent":    true,
	"haven't":   true,
	"hasnt":     true,
	"hasn't":    true,
	"cant":      true,
	"wont":      true,
	"shouldnt":  true,
	"couldnt":   true,
	"wouldnt":   true,
}

// DetectNegation checks if a word at the given index is negated by preceding words.
// It looks back up to 3 tokens maximum to find negation words like "not", "no", "never", etc.
// Returns true if the word is negated, false otherwise.
//
// For example, in the phrase "not very good", the word "good" at index 2 would be
// detected as negated because "not" appears 2 positions before it.
func DetectNegation(tokens []string, index int) bool {
	if index <= 0 || len(tokens) == 0 {
		return false
	}

	// Look back up to 3 tokens
	lookbackStart := index - 3
	if lookbackStart < 0 {
		lookbackStart = 0
	}

	// Check preceding tokens for negation words
	for i := lookbackStart; i < index; i++ {
		if negationWords[tokens[i]] {
			return true
		}
	}

	return false
}

// CalculatePunctuationBoost calculates a score boost based on repeated punctuation
// in the text. The boost accounts for emphasis conveyed through repeated exclamation
// marks, question marks, or other punctuation patterns.
//
// The boost returns a multiplier between 1.0 (no punctuation emphasis) and 1.5
// (strong punctuation emphasis). For example:
// - "great!" = 1.1 boost (single emphasis)
// - "great!!" = 1.2 boost (double emphasis)
// - "great!!!" = 1.3 boost (triple emphasis)
// - "great!!!!!!!" = 1.5 boost (max capped at 1.5)
//
// Returns a multiplier value to apply to sentiment scores.
func CalculatePunctuationBoost(text string) float64 {
	if text == "" {
		return 1.0
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
		return 1.0
	}

	// Scale from 1.1 (2 consecutive) to 1.5 (6+ consecutive)
	// Formula: 1.0 + (min(maxConsecutive - 1, 5) * 0.1)
	boost := 1.0 + math.Min(float64(maxConsecutive-1), 5.0)*0.1
	return math.Min(boost, 1.5)
}

// CalculateCapsBoost calculates a score boost based on the percentage of ALL CAPS words
// in the text. ALL CAPS words typically indicate emphasis or stronger sentiment.
//
// The boost returns a multiplier between 1.0 (no caps words) and 1.3 (high percentage
// of caps words). For example:
// - "This is great" = 1.0 boost (no all-caps words)
// - "This is GREAT" = 1.05 boost (1/3 words in caps)
// - "THIS IS GREAT" = 1.3 boost (all words in caps)
//
// Returns a multiplier value to apply to sentiment scores.
func CalculateCapsBoost(text string) float64 {
	if text == "" {
		return 1.0
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return 1.0
	}

	capsCount := 0
	for _, word := range words {
		if isAllCaps(word) {
			capsCount++
		}
	}

	// Calculate percentage of caps words
	capsPercentage := float64(capsCount) / float64(len(words))

	// Scale from 1.0 (0%) to 1.3 (100%)
	// Formula: 1.0 + (capsPercentage * 0.3)
	boost := 1.0 + (capsPercentage * 0.3)
	return math.Min(boost, 1.3)
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

// ApplyModifiers applies sentiment modifiers (punctuation and capitalization) to a base score.
//
// The algorithm:
// 1. Applies punctuation boost as a multiplier (e.g., "!!!" increases emphasis)
// 2. Applies capitalization boost as a multiplier (e.g., "GREAT" indicates stronger sentiment)
// 3. Clamps the final score to the valid [-1.0, 1.0] range
//
// Note: Negation is handled per-token in AnalyzeText (analyzer.go), not here.
// This avoids double negation which would incorrectly invert the sentiment.
//
// Example of the bug this design prevents:
//   Text: "not good"
//   - Per-token analysis: "good" (+0.7) → negated to -0.7 ✓
//   - If we flipped here again: -0.7 → +0.7 ✗ (WRONG - classified as Positive!)
//
// By not flipping here, we preserve the correct per-token negation result.
//
// Returns the modified score value in the range [-1.0, 1.0].
func ApplyModifiers(baseScore float64, text string, tokens []string) float64 {
	modifiedScore := baseScore

	// Note: Negation is handled per-token in analyzer.go (AnalyzeText function).
	// We do NOT flip the score here to avoid double negation.

	// Skip boost calculations if text is empty
	if text == "" {
		// Clamp the final score to [-1.0, 1.0] range
		if modifiedScore > 1.0 {
			modifiedScore = 1.0
		} else if modifiedScore < -1.0 {
			modifiedScore = -1.0
		}
		return modifiedScore
	}

	// Apply punctuation boost as a multiplier
	punctuationBoost := CalculatePunctuationBoost(text)
	modifiedScore *= punctuationBoost

	// Apply caps boost as a multiplier
	capsBoost := CalculateCapsBoost(text)
	modifiedScore *= capsBoost

	// Clamp the final score to [-1.0, 1.0] range
	if modifiedScore > 1.0 {
		modifiedScore = 1.0
	} else if modifiedScore < -1.0 {
		modifiedScore = -1.0
	}

	return modifiedScore
}
