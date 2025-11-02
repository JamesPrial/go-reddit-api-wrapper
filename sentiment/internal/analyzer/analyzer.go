package analyzer

import (
	"math"

	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/internal/constants"
	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/internal/preprocessor"
)

// LexiconProvider defines the interface for lexicon operations used by the analyzer.
// This interface enables dependency injection and testing with mock implementations.
type LexiconProvider interface {
	GetScore(word string) float64              // Returns the sentiment score for a word, or 0.0 if not found.
	IsPositive(word string) bool               // Returns true if the word has a positive sentiment.
	IsNegative(word string) bool               // Returns true if the word has a negative sentiment.
	DetectNegation(tokens []string, index int) bool // Checks if a word at the given index is negated by preceding words.
	ExtractEmoticons(text string) []string     // Finds all emoticons in the given text.
	GetMultiplier(text string) float64         // Applies sentiment modifiers (punctuation and capitalization) to a base score.
}

// PreprocessorProvider defines the interface for text preprocessing operations used by the analyzer.
// This interface enables dependency injection and testing with mock implementations.
type PreprocessorProvider interface {
	// IsDeleted checks if the author string represents a deleted or removed post/comment.
	IsDeleted(author string) bool
	// Tokenize splits text into tokens (words) by converting to lowercase
	// and trimming punctuation from the edges of each token.
	Tokenize(text string) []string
}

// Analyzer performs sentiment analysis on text using a lexicon-based approach
// with additional modifiers for negation, punctuation emphasis, and capitalization.
type Analyzer struct {
	lexicon         LexiconProvider
	preprocessor    PreprocessorProvider
	minWordCount    int
	enableEmoticons bool
}

// NewAnalyzer creates a new Analyzer instance with the specified configuration.
//
// Parameters:
//   - lex: Lexicon instance containing sentiment words and modifier configuration
//   - minWordCount: minimum number of tokens required for analysis (0 to disable)
//   - enableEmoticons: whether to include emoticon sentiment in the analysis
//
// Returns a new Analyzer ready for sentiment analysis.
func NewAnalyzer(lex LexiconProvider, minWordCount int, enableEmoticons bool) *Analyzer {
	return &Analyzer{
		lexicon:         lex,
		preprocessor:    preprocessor.NewPreprocessor(),
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
//   - sentiment: classification as an integer (VERY_NEGATIVE_SENTIMENT to VERY_POSITIVE_SENTIMENT)
//     VERY_NEGATIVE_SENTIMENT: VeryNegative (score < VERY_NEGATIVE_SCORE_THRESHOLD)
//     NEGATIVE_SENTIMENT: Negative (score >= VERY_NEGATIVE_SCORE_THRESHOLD && score < NEGATIVE_SCORE_THRESHOLD)
//     NEUTRAL_SENTIMENT: Neutral (score >= NEGATIVE_SCORE_THRESHOLD && score < NEUTRAL_SCORE_THRESHOLD)
//     POSITIVE_SENTIMENT: Positive (score >= NEUTRAL_SCORE_THRESHOLD && score < POSITIVE_SCORE_THRESHOLD)
//     VERY_POSITIVE_SENTIMENT: VeryPositive (score >= POSITIVE_SCORE_THRESHOLD)
//   - score: normalized sentiment score (VERY_NEGATIVE_SCORE_THRESHOLD to VERY_POSITIVE_SCORE_THRESHOLD)
//   - confidence: how confident the analysis is (0.0 to 1.0)
//     based on the ratio of matched words to total words
func (a *Analyzer) AnalyzeText(text string) (sentimentValue int, score float64, confidence float64) {
	// Sentiment value constants (match sentiment.Sentiment enum values)
	const (
		veryNegativeSentiment = -2
		negativeSentiment     = -1
		neutralSentiment      = 0
		positiveSentiment     = 1
		veryPositiveSentiment = 2
	)

	if text == "" {
		return neutralSentiment, 0.0, 0.0
	}
	if a.preprocessor.IsDeleted(text) {
		return neutralSentiment, 0.0, 0.0
	}
	tokens := a.preprocessor.Tokenize(text)
	if a.minWordCount > 0 && len(tokens) < a.minWordCount {
		return neutralSentiment, 0.0, 0.0
	}

	totalScore := 0.0
	matchedWords := 0

	// Score each token
	for i, token := range tokens {
		wordScore := a.lexicon.GetScore(token)
		if wordScore != 0.0 {
			// Check if this word is negated
			if a.lexicon.DetectNegation(tokens, i) {
				wordScore = -wordScore
			}
			totalScore += wordScore
			matchedWords++
		}
	}

	// Add emoticon scores if enabled
	if a.enableEmoticons {
		emoticons := a.lexicon.ExtractEmoticons(text)
		for _, emoticon := range emoticons {
			totalScore += a.lexicon.GetScore(emoticon)
			matchedWords++
		}
	}

	// Normalize score by token count to get average score per word
	if len(tokens) > 0 {
		score = totalScore / float64(len(tokens))
	} else {
		score = 0.0
	}

	// Apply multipliers (punctuation and capitalization emphasis)
	multiplier := a.lexicon.GetMultiplier(text)
	score = score * multiplier

	if len(tokens) > 0 {
		matchRatio := float64(matchedWords) / float64(len(tokens))
		confidence = math.Min(matchRatio*constants.ConfidenceScalingFactor, constants.MaxConfidence)
	} else {
		confidence = 0.0
	}

	// Convert score to sentiment classification
	// Thresholds chosen to balance sensitivity with specificity
	switch {
	case score < constants.VeryNegativeThreshold:
		sentimentValue = veryNegativeSentiment // VeryNegative
	case score < constants.NegativeThreshold:
		sentimentValue = negativeSentiment // Negative
	case score < constants.NeutralThreshold:
		sentimentValue = neutralSentiment // Neutral
	case score < constants.PositiveThreshold:
		sentimentValue = positiveSentiment // Positive
	default:
		sentimentValue = veryPositiveSentiment // VeryPositive
	}

	// Ensure score is in valid range
	if score > constants.MaxScore {
		score = constants.MaxScore
	} else if score < constants.MinScore {
		score = constants.MinScore
	}

	return sentimentValue, score, confidence
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
// Returns the averaged score clamped to [MIN_SCORE, MAX_SCORE].
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
	if combined > constants.MaxScore {
		return constants.MaxScore
	} else if combined < constants.MinScore {
		return constants.MinScore
	}

	return combined
}
