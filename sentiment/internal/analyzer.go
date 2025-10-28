package internal

import (
	"math"
)

// Analyzer performs sentiment analysis on text using a lexicon-based approach
// with additional modifiers for negation, punctuation emphasis, and capitalization.
type Analyzer struct {
	preprocessor    *Preprocessor
	minWordCount    int
	enableEmoticons bool
}

// NewAnalyzer creates a new Analyzer instance with the specified configuration.
//
// Parameters:
//   - minWordCount: minimum number of tokens required for analysis (0 to disable)
//   - enableEmoticons: whether to include emoticon sentiment in the analysis
//
// Returns a new Analyzer ready for sentiment analysis.
func NewAnalyzer(minWordCount int, enableEmoticons bool) *Analyzer {
	return &Analyzer{
		preprocessor:    NewPreprocessor(),
		minWordCount:    minWordCount,
		enableEmoticons: enableEmoticons,
	}
}

// AnalyzeText performs sentiment analysis on the provided text.
//
// The analysis algorithm:
// 1. Validates input (non-empty, not deleted/removed)
// 2. Tokenizes text into words
// 3. Calculates base sentiment score from lexicon matches
// 4. Applies negation detection to individual words
// 5. Includes emoticon scores if enabled
// 6. Applies punctuation and capitalization modifiers
// 7. Normalizes score by word count
// 8. Calculates confidence based on matched words vs total
// 9. Converts numeric score to sentiment classification
//
// Returns:
//   - sentiment: classification as an integer (-2 to 2)
//     -2: VeryNegative (score < -0.6)
//     -1: Negative (score < -0.2)
//     0: Neutral (score < 0.2)
//     1: Positive (score < 0.6)
//     2: VeryPositive (score >= 0.6)
//   - score: normalized sentiment score (-1.0 to 1.0)
//   - confidence: how confident the analysis is (0.0 to 1.0)
//     based on the ratio of matched words to total words
func (a *Analyzer) AnalyzeText(text string) (sentiment int, score float64, confidence float64) {
	// Validate input
	if text == "" {
		return 0, 0.0, 0.0
	}

	// Check if text represents deleted or removed content
	if a.preprocessor.IsDeleted(text) {
		return 0, 0.0, 0.0
	}

	// Tokenize the text
	tokens := a.preprocessor.Tokenize(text)

	// If we have fewer than minWordCount tokens, return neutral
	if a.minWordCount > 0 && len(tokens) < a.minWordCount {
		return 0, 0.0, 0.0
	}

	// If no tokens, return neutral
	if len(tokens) == 0 {
		return 0, 0.0, 0.0
	}

	// Get lexicons
	positiveWords := GetPositiveWords()
	negativeWords := GetNegativeWords()
	emoticonsMap := GetEmoticons()

	// Calculate base sentiment score
	var totalScore float64
	matchedWords := 0

	for i, token := range tokens {
		tokenScore := 0.0

		// Check for positive and negative word matches
		if posScore, ok := positiveWords[token]; ok {
			tokenScore = posScore
			matchedWords++
		} else if negScore, ok := negativeWords[token]; ok {
			tokenScore = negScore
			matchedWords++
		}

		// Apply negation if this word is negated
		if tokenScore != 0 && DetectNegation(tokens, i) {
			tokenScore = -tokenScore
		}

		totalScore += tokenScore
	}

	// Add emoticon scores if enabled
	if a.enableEmoticons {
		emoticonsFound := a.preprocessor.ExtractEmoticons(text)
		for _, emoticon := range emoticonsFound {
			if emoticonScore, ok := emoticonsMap[emoticon]; ok {
				totalScore += emoticonScore
				matchedWords++
			}
		}
	}

	// Normalize score by word count (geometric mean to reduce word count bias)
	// We use sqrt(word count) as a gentler normalization than simple division
	if len(tokens) > 0 {
		normalizer := math.Sqrt(float64(len(tokens)))
		score = totalScore / normalizer
	} else {
		score = 0.0
	}

	// Apply modifiers (negation, punctuation, caps)
	score = ApplyModifiers(score, text, tokens)

	// Calculate confidence metric
	// Confidence is based on the ratio of matched words to total words
	// Higher ratio = higher confidence
	if len(tokens) > 0 {
		matchRatio := float64(matchedWords) / float64(len(tokens))
		// Apply a gentler scaling: confidence grows with match ratio but never reaches 1.0 if no matches
		confidence = math.Min(matchRatio*1.2, 1.0)
	} else {
		confidence = 0.0
	}

	// Convert score to sentiment classification
	// Thresholds chosen to balance sensitivity with specificity
	switch {
	case score < -0.6:
		sentiment = -2 // VeryNegative
	case score < -0.2:
		sentiment = -1 // Negative
	case score < 0.2:
		sentiment = 0 // Neutral
	case score < 0.6:
		sentiment = 1 // Positive
	default:
		sentiment = 2 // VeryPositive
	}

	// Ensure score is in valid range
	if score > 1.0 {
		score = 1.0
	} else if score < -1.0 {
		score = -1.0
	}

	return sentiment, score, confidence
}

// CombineScores averages multiple sentiment scores with optional weighting.
// This is useful when analyzing multiple related texts (e.g., a post and its comments).
//
// If no scores are provided, returns 0.0. If one score is provided, returns that score.
// For multiple scores, returns the arithmetic mean.
//
// Parameters:
//   - scores: one or more float64 sentiment scores to combine
//
// Returns the averaged score clamped to [-1.0, 1.0].
func (a *Analyzer) CombineScores(scores ...float64) float64 {
	if len(scores) == 0 {
		return 0.0
	}

	if len(scores) == 1 {
		return scores[0]
	}

	var sum float64
	for _, s := range scores {
		sum += s
	}

	combined := sum / float64(len(scores))

	// Clamp to valid range
	if combined > 1.0 {
		return 1.0
	} else if combined < -1.0 {
		return -1.0
	}

	return combined
}
