package sentiment

import (
	"context"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/internal/analyzer"
)

// Analyzer performs sentiment analysis on Reddit posts and comments.
// It uses a keyword-based approach to classify content sentiment.
// The zero value of Analyzer is not usable; use NewAnalyzer to create an instance.
type Analyzer struct {
	config *Config
}

// NewAnalyzer creates a new Analyzer with the provided configuration.
// If config is nil, DefaultConfig() will be used.
// The returned Analyzer is ready for use immediately.
func NewAnalyzer(config *Config) *Analyzer {
	if config == nil {
		config = DefaultConfig()
	}
	return &Analyzer{
		config: config,
	}
}

// AnalyzePost analyzes the sentiment of a Reddit post.
// It examines both the post title and body (self text) and returns
// detailed sentiment analysis including overall classification and scoring.
//
// The function validates that the post is not nil and that the context
// has not been cancelled. It returns a NewValidationError if validation fails.
//
// The returned PostSentiment includes:
//   - Sentiment: Overall sentiment classification (VeryNegative to VeryPositive)
//   - Score: Numeric score typically ranging from -1.0 to 1.0
//   - Confidence: Confidence metric from 0.0 to 1.0
//   - TitleScore: Sentiment score of the title alone
//   - BodyScore: Sentiment score of the body alone
func (a *Analyzer) AnalyzePost(ctx context.Context, post *types.Post) (*PostSentiment, error) {
	// Validate context is not cancelled
	if err := ctx.Err(); err != nil {
		return nil, NewValidationErrorWithErr("context already cancelled", err)
	}

	// Validate post is not nil
	if post == nil {
		return nil, NewValidationError("post cannot be nil")
	}

	// Log analysis start if logger is configured
	if a.config.Logger != nil {
		a.config.Logger.DebugContext(ctx, "analyzing post sentiment",
			slog.String("post_id", post.GetID()),
			slog.String("subreddit", post.Subreddit),
			slog.Int("title_length", len(post.Title)),
			slog.Int("body_length", len(post.SelfText)),
		)
	}

	// Delegate to internal analyzer
	result := a.analyzePostInternal(ctx, post)

	// Log analysis completion if logger is configured
	if a.config.Logger != nil {
		a.config.Logger.DebugContext(ctx, "post sentiment analysis complete",
			slog.String("post_id", post.GetID()),
			slog.String("sentiment", result.Sentiment.String()),
			slog.Float64("score", result.Score),
		)
	}

	return result, nil
}

// AnalyzeComment analyzes the sentiment of a Reddit comment.
// It examines the comment body and returns sentiment analysis results.
//
// The function validates that the comment is not nil and that the context
// has not been cancelled. It returns a NewValidationError if validation fails.
//
// The returned CommentSentiment includes:
//   - Sentiment: Overall sentiment classification (VeryNegative to VeryPositive)
//   - Score: Numeric score typically ranging from -1.0 to 1.0
//   - Confidence: Confidence metric from 0.0 to 1.0
func (a *Analyzer) AnalyzeComment(ctx context.Context, comment *types.Comment) (*CommentSentiment, error) {
	// Validate context is not cancelled
	if err := ctx.Err(); err != nil {
		return nil, NewValidationErrorWithErr("context already cancelled", err)
	}

	// Validate comment is not nil
	if comment == nil {
		return nil, NewValidationError("comment cannot be nil")
	}

	// Log analysis start if logger is configured
	if a.config.Logger != nil {
		a.config.Logger.DebugContext(ctx, "analyzing comment sentiment",
			slog.String("comment_id", comment.GetID()),
			slog.String("subreddit", comment.Subreddit),
			slog.Int("body_length", len(comment.Body)),
		)
	}

	// Delegate to internal analyzer
	result := a.analyzeCommentInternal(ctx, comment)

	// Log analysis completion if logger is configured
	if a.config.Logger != nil {
		a.config.Logger.DebugContext(ctx, "comment sentiment analysis complete",
			slog.String("comment_id", comment.GetID()),
			slog.String("sentiment", result.Sentiment.String()),
			slog.Float64("score", result.Score),
		)
	}

	return result, nil
}

// analyzePostInternal performs the actual sentiment analysis on a post.
// It analyzes the title and body separately, then combines the scores.
func (a *Analyzer) analyzePostInternal(ctx context.Context, post *types.Post) *PostSentiment {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &PostSentiment{
			Sentiment:  Neutral,
			Score:      0.0,
			Confidence: 0.0,
			TitleScore: 0.0,
			BodyScore:  0.0,
		}
	}

	// Create internal analyzer with config settings
	internalAnalyzer := analyzer.NewAnalyzer(a.config.MinWordCount, a.config.EnableEmoticons)

	// Analyze title and capture score and confidence
	_, titleScore, titleConf := internalAnalyzer.AnalyzeText(post.Title)

	// Analyze body and capture score and confidence
	_, bodyScore, bodyConf := internalAnalyzer.AnalyzeText(post.SelfText)

	// Combine scores using internal analyzer
	combinedScore := internalAnalyzer.CombineScores(titleScore, bodyScore)

	// Convert combined score to sentiment classification
	var sentiment Sentiment
	switch {
	case combinedScore < -0.6:
		sentiment = VeryNegative
	case combinedScore < -0.2:
		sentiment = Negative
	case combinedScore < 0.2:
		sentiment = Neutral
	case combinedScore < 0.6:
		sentiment = Positive
	default:
		sentiment = VeryPositive
	}

	// Calculate average confidence from title and body analyses
	// If either title or body is empty, we rely on the other
	var confidence float64
	if post.Title != "" && post.SelfText != "" {
		// Both title and body have content - average their confidence scores
		confidence = (titleConf + bodyConf) / 2.0
	} else if post.Title != "" {
		// Only title has content
		confidence = titleConf
	} else if post.SelfText != "" {
		// Only body has content
		confidence = bodyConf
	} else {
		// No content to analyze
		confidence = 0.0
	}

	return &PostSentiment{
		Sentiment:  sentiment,
		Score:      combinedScore,
		Confidence: confidence,
		TitleScore: titleScore,
		BodyScore:  bodyScore,
	}
}

// analyzeCommentInternal performs the actual sentiment analysis on a comment.
// It analyzes the comment body and returns sentiment results.
func (a *Analyzer) analyzeCommentInternal(ctx context.Context, comment *types.Comment) *CommentSentiment {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &CommentSentiment{
			Sentiment:  Neutral,
			Score:      0.0,
			Confidence: 0.0,
		}
	}

	// Create internal analyzer with config settings
	internalAnalyzer := analyzer.NewAnalyzer(a.config.MinWordCount, a.config.EnableEmoticons)

	// Analyze comment body
	_, score, confidence := internalAnalyzer.AnalyzeText(comment.Body)

	// Convert score to sentiment classification
	var sentiment Sentiment
	switch {
	case score < -0.6:
		sentiment = VeryNegative
	case score < -0.2:
		sentiment = Negative
	case score < 0.2:
		sentiment = Neutral
	case score < 0.6:
		sentiment = Positive
	default:
		sentiment = VeryPositive
	}

	return &CommentSentiment{
		Sentiment:  sentiment,
		Score:      score,
		Confidence: confidence,
	}
}
