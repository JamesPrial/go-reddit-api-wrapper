package constants

// Sentiment score thresholds and bounds used to categorize text sentiment.
// These define the ranges that map numeric sentiment scores to Sentiment classifications.
const (
	// MinScore is the minimum valid sentiment score (inclusive).
	MinScore = -1.0
	// MaxScore is the maximum valid sentiment score (inclusive).
	MaxScore = 1.0

	// VeryNegativeThreshold defines the upper bound for VeryNegative sentiment.
	// Scores below this threshold are classified as VeryNegative.
	VeryNegativeThreshold = -0.6

	// NegativeThreshold defines the upper bound for Negative sentiment.
	// Scores >= VeryNegativeThreshold and < NegativeThreshold are classified as Negative.
	NegativeThreshold = -0.2

	// NeutralThreshold defines the upper bound for Neutral sentiment.
	// Scores >= NegativeThreshold and < NeutralThreshold are classified as Neutral.
	NeutralThreshold = 0.2

	// PositiveThreshold defines the upper bound for Positive sentiment.
	// Scores >= NeutralThreshold and < PositiveThreshold are classified as Positive.
	// Scores >= PositiveThreshold are classified as VeryPositive.
	PositiveThreshold = 0.6

	// ConfidenceScalingFactor is used to scale confidence calculations.
	// Applied to the match ratio to produce final confidence scores.
	ConfidenceScalingFactor = 1.2

	// MaxConfidence is the maximum valid confidence value (inclusive).
	MaxConfidence = 1.0
)
