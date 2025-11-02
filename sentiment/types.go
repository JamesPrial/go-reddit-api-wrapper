package sentiment

import "github.com/jamesprial/go-reddit-api-wrapper/sentiment/config"

// Sentiment represents the sentiment classification of content.
// It is expressed as an integer ranging from VeryNegative to VeryPositive.
type Sentiment int

const (
	// VeryNegative represents highly negative sentiment.
	VeryNegative Sentiment = -2
	// Negative represents negative sentiment.
	Negative Sentiment = -1
	// Neutral represents neutral sentiment with no clear positive or negative bias.
	Neutral Sentiment = 0
	// Positive represents positive sentiment.
	Positive Sentiment = 1
	// VeryPositive represents highly positive sentiment.
	VeryPositive Sentiment = 2
)

// Sentiment score thresholds and bounds used to categorize text sentiment.
// These define the ranges that map numeric sentiment scores to Sentiment classifications.
// Re-exported from config for public API access.
const (
	// MIN_SCORE is the minimum valid sentiment score (inclusive).
	MIN_SCORE = config.MIN_SCORE
	// MAX_SCORE is the maximum valid sentiment score (inclusive).
	MAX_SCORE = config.MAX_SCORE

	// VERY_NEGATIVE_SCORE_THRESHOLD defines the upper bound for VeryNegative sentiment.
	// Scores below this threshold are classified as VeryNegative.
	VERY_NEGATIVE_SCORE_THRESHOLD = config.VERY_NEGATIVE_SCORE_THRESHOLD

	// NEGATIVE_SCORE_THRESHOLD defines the upper bound for Negative sentiment.
	// Scores >= VERY_NEGATIVE_SCORE_THRESHOLD and < NEGATIVE_SCORE_THRESHOLD are classified as Negative.
	NEGATIVE_SCORE_THRESHOLD = config.NEGATIVE_SCORE_THRESHOLD

	// NEUTRAL_SCORE_THRESHOLD defines the upper bound for Neutral sentiment.
	// Scores >= NEGATIVE_SCORE_THRESHOLD and < NEUTRAL_SCORE_THRESHOLD are classified as Neutral.
	NEUTRAL_SCORE_THRESHOLD = config.NEUTRAL_SCORE_THRESHOLD

	// POSITIVE_SCORE_THRESHOLD defines the upper bound for Positive sentiment.
	// Scores >= NEUTRAL_SCORE_THRESHOLD and < POSITIVE_SCORE_THRESHOLD are classified as Positive.
	// Scores >= POSITIVE_SCORE_THRESHOLD are classified as VeryPositive.
	POSITIVE_SCORE_THRESHOLD = config.POSITIVE_SCORE_THRESHOLD

	// CONFIDENCE_SCALING_FACTOR is used to scale confidence calculations.
	// Applied to the match ratio to produce final confidence scores.
	CONFIDENCE_SCALING_FACTOR = config.CONFIDENCE_SCALING_FACTOR

	// MAX_CONFIDENCE is the maximum valid confidence value (inclusive).
	MAX_CONFIDENCE = config.MAX_CONFIDENCE
)

// String returns a human-readable string representation of the sentiment.
func (s Sentiment) String() string {
	switch s {
	case VeryNegative:
		return "VeryNegative"
	case Negative:
		return "Negative"
	case Neutral:
		return "Neutral"
	case Positive:
		return "Positive"
	case VeryPositive:
		return "VeryPositive"
	default:
		return "Unknown"
	}
}

// PostSentiment represents the sentiment analysis results for a Reddit post.
// It includes overall sentiment classification and detailed scoring information.
type PostSentiment struct {
	// Sentiment is the overall sentiment classification of the post.
	Sentiment Sentiment
	// Score is the numeric sentiment score, typically ranging from -1.0 to 1.0.
	Score float64
	// Confidence is a value between 0.0 and 1.0 indicating how confident
	// the sentiment analysis is in the result. Higher values indicate greater confidence.
	Confidence float64
	// TitleScore is the sentiment score calculated from the post title alone.
	TitleScore float64
	// BodyScore is the sentiment score calculated from the post body (self text) alone.
	BodyScore float64
}

// CommentSentiment represents the sentiment analysis results for a Reddit comment.
// It includes sentiment classification and scoring information.
type CommentSentiment struct {
	// Sentiment is the overall sentiment classification of the comment.
	Sentiment Sentiment
	// Score is the numeric sentiment score, typically ranging from -1.0 to 1.0.
	Score float64
	// Confidence is a value between 0.0 and 1.0 indicating how confident
	// the sentiment analysis is in the result. Higher values indicate greater confidence.
	Confidence float64
}
