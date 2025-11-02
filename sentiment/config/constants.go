package config

// Sentiment score thresholds and bounds used to categorize text sentiment.
// These define the ranges that map numeric sentiment scores to Sentiment classifications.
const (
	// MIN_SCORE is the minimum valid sentiment score (inclusive).
	MIN_SCORE = -1.0
	// MAX_SCORE is the maximum valid sentiment score (inclusive).
	MAX_SCORE = 1.0

	// VERY_NEGATIVE_SCORE_THRESHOLD defines the upper bound for VeryNegative sentiment.
	// Scores below this threshold are classified as VeryNegative.
	VERY_NEGATIVE_SCORE_THRESHOLD = -0.6

	// NEGATIVE_SCORE_THRESHOLD defines the upper bound for Negative sentiment.
	// Scores >= VERY_NEGATIVE_SCORE_THRESHOLD and < NEGATIVE_SCORE_THRESHOLD are classified as Negative.
	NEGATIVE_SCORE_THRESHOLD = -0.2

	// NEUTRAL_SCORE_THRESHOLD defines the upper bound for Neutral sentiment.
	// Scores >= NEGATIVE_SCORE_THRESHOLD and < NEUTRAL_SCORE_THRESHOLD are classified as Neutral.
	NEUTRAL_SCORE_THRESHOLD = 0.2

	// POSITIVE_SCORE_THRESHOLD defines the upper bound for Positive sentiment.
	// Scores >= NEUTRAL_SCORE_THRESHOLD and < POSITIVE_SCORE_THRESHOLD are classified as Positive.
	// Scores >= POSITIVE_SCORE_THRESHOLD are classified as VeryPositive.
	POSITIVE_SCORE_THRESHOLD = 0.6

	// CONFIDENCE_SCALING_FACTOR is used to scale confidence calculations.
	// Applied to the match ratio to produce final confidence scores.
	CONFIDENCE_SCALING_FACTOR = 1.2

	// MAX_CONFIDENCE is the maximum valid confidence value (inclusive).
	MAX_CONFIDENCE = 1.0

	// Sentiment constants
	VERY_NEGATIVE_SENTIMENT = -2.0
	NEGATIVE_SENTIMENT      = -1.0
	NEUTRAL_SENTIMENT       = 0.0
	POSITIVE_SENTIMENT      = 1.0
	VERY_POSITIVE_SENTIMENT = 2.0

	// Lexicon constants
	MAX_NEGATION_LOOKBACK                = 3
	PUNCTUATION_BOOST_CONSECUTIVE_FACTOR = 0.1
	NO_PUNCTUATION_BOOST                 = 1.0
	MAX_PUNCTUATION_BOOST                = 1.5
	MAX_PUNCTUATION_BOOST_CONSECUTIVE    = 5
	NO_CAPS_BOOST                        = 1.0
	MAX_CAPS_BOOST                       = 1.3
	CAPS_BOOST_PERCENTAGE_FACTOR         = 0.3
	NO_NEGATION_BOOST                    = 1.0
	MAX_NEGATION_BOOST                   = 1.5
)
