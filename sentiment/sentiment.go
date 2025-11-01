package sentiment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/config"
	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/internal/analyzer"
	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/internal/lexicon"
	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/internal/modifier"
)

// Sentiment score thresholds used to categorize text sentiment, imported from analyzer package.
const (
	VERY_NEGATIVE_SENTIMENT_THRESHOLD = analyzer.VERY_NEGATIVE_SENTIMENT_THRESHOLD // Scores below this are VeryNegative
	NEGATIVE_SENTIMENT_THRESHOLD      = analyzer.NEGATIVE_SENTIMENT_THRESHOLD      // Scores below this are Negative
	NEUTRAL_SENTIMENT_THRESHOLD       = analyzer.NEUTRAL_SENTIMENT_THRESHOLD       // Scores below this are Neutral
	POSITIVE_SENTIMENT_THRESHOLD      = analyzer.POSITIVE_SENTIMENT_THRESHOLD      // Scores below this are Positive
	// Scores >= 0.6 are VeryPositive
)

func init() {
	// Initialize lexicon and modifier packages with default configuration on package load.
	// This ensures the packages are ready even if NewAnalyzer is never called.
	// If NewAnalyzer is called, it may reinitialize with custom config, but sync.Once
	// in both packages prevents actual re-initialization - the first call wins.
	// See initializePackages() for details.
	initializePackages(nil)
}

// initializePackages initializes the lexicon and modifier internal packages.
// If cfg is nil, loads the default embedded configuration.
// Otherwise, uses the provided configuration.
//
// IMPORTANT: Due to sync.Once protection in lexicon and modifier packages,
// the first call to Init() in each package "locks in" the configuration.
// If this function is called multiple times, only the first call takes effect.
// This is why package initialization happens in init(), ensuring defaults are set.
func initializePackages(cfg *config.Config) error {
	// Load configuration if not provided
	if cfg == nil {
		var err error
		cfg, err = config.LoadDefaultConfig()
		if err != nil {
			return fmt.Errorf("failed to load default lexicon config: %w", err)
		}
	}

	// Initialize lexicon package with the provided config
	lexiconCfg := &lexicon.LexiconConfig{
		PositiveWords: cfg.Lexicon.PositiveWords,
		NegativeWords: cfg.Lexicon.NegativeWords,
		Emoticons:     cfg.Lexicon.Emoticons,
	}
	if err := lexicon.Init(lexiconCfg); err != nil {
		return fmt.Errorf("failed to initialize lexicon: %w", err)
	}

	// Initialize modifier package with the provided config
	modifierCfg := &modifier.ModifierConfig{
		NegationWords: cfg.Modifier.NegationWords,
	}
	if err := modifier.Init(modifierCfg); err != nil {
		return fmt.Errorf("failed to initialize modifier: %w", err)
	}

	return nil
}

// Analyzer performs sentiment analysis on Reddit posts and comments.
// It uses a keyword-based approach to classify content sentiment.
// The zero value of Analyzer is not usable; use NewAnalyzer to create an instance.
//
// Analyzer is safe for concurrent use from multiple goroutines.
type Analyzer struct {
	config *Config
}

// AnalyzerOption is a functional option for configuring an Analyzer.
type AnalyzerOption func(*analyzerOptions) error

// analyzerOptions holds configuration options for analyzer initialization.
type analyzerOptions struct {
	config           *Config
	lexiconModConfig *config.Config
}

// WithConfig returns an AnalyzerOption that uses the provided config.
// If config is nil, this option is ignored and defaults will be used.
// Note: This only configures analyzer-specific settings (MinWordCount, EnableEmoticons).
// To customize lexicon and modifier configuration, use WithConfigFile or WithLexiconModConfig.
func WithConfig(cfg *Config) AnalyzerOption {
	return func(opts *analyzerOptions) error {
		if cfg != nil {
			opts.config = cfg
		}
		return nil
	}
}

// WithConfigFile returns an AnalyzerOption that loads analyzer config from a JSON file.
// The file should contain analyzer-specific fields: minWordCount, enableEmoticons.
// This does NOT load lexicon/modifier configuration.
// Returns an error if the file cannot be read or parsed.
func WithConfigFile(path string) AnalyzerOption {
	return func(opts *analyzerOptions) error {
		if path == "" {
			return nil
		}
		// Load config from file
		cfg, err := LoadAnalyzerConfigFromFile(path)
		if err != nil {
			return fmt.Errorf("failed to load analyzer config from file %q: %w", path, err)
		}
		opts.config = cfg
		return nil
	}
}

// WithLexiconModConfig returns an AnalyzerOption that uses the provided lexicon/modifier config.
// This config contains the sentiment lexicons (positive/negative words, emoticons) and modifier settings.
// If config is nil, the default embedded configuration will be used.
func WithLexiconModConfig(cfg *config.Config) AnalyzerOption {
	return func(opts *analyzerOptions) error {
		if cfg != nil {
			opts.lexiconModConfig = cfg
		}
		return nil
	}
}

// WithLexiconModConfigFile returns an AnalyzerOption that loads lexicon/modifier config from a file.
// The file should be in the format supported by sentiment/config package, containing
// positive_words, negative_words, emoticons, and negation_words fields.
// Returns an error if the file cannot be read or parsed.
func WithLexiconModConfigFile(path string) AnalyzerOption {
	return func(opts *analyzerOptions) error {
		if path == "" {
			return nil
		}
		cfg, err := config.LoadLexiconConfigFromFile(path)
		if err != nil {
			return fmt.Errorf("failed to load lexicon/modifier config from file %q: %w", path, err)
		}
		opts.lexiconModConfig = cfg
		return nil
	}
}

// NewAnalyzer creates a new Analyzer with optional configuration.
// It initializes two separate configuration domains:
//
//  1. Analyzer Configuration (MinWordCount, EnableEmoticons):
//     Set via WithConfig() or WithConfigFile(). Defaults are used if not provided.
//
//  2. Lexicon/Modifier Configuration (sentiment words, emoticons, negation words):
//     Set via WithLexiconModConfig() or WithLexiconModConfigFile().
//     Defaults from embedded files are used if not provided.
//
// IMPORTANT: Due to sync.Once protection in the lexicon and modifier packages,
// custom lexicon/modifier configuration can only take effect if provided BEFORE
// NewAnalyzer is called for the first time. If the packages are already initialized
// with defaults (from the init() function), subsequent calls to NewAnalyzer with
// different lexicon/modifier configs will not change the already-initialized packages.
//
// The returned Analyzer is ready for use immediately and is safe for concurrent use
// from multiple goroutines.
//
// Example usage:
//
//	// Using default configuration
//	analyzer, err := NewAnalyzer()
//	if err != nil {
//		// handle error
//	}
//
//	// Using custom analyzer settings
//	analyzer, err := NewAnalyzer(WithConfig(&Config{
//		MinWordCount:    1,
//		EnableEmoticons: true,
//	}))
//	if err != nil {
//		// handle error
//	}
//
//	// Using custom lexicon/modifier configuration from file
//	analyzer, err := NewAnalyzer(WithLexiconModConfigFile("/path/to/lexicon_config.json"))
//	if err != nil {
//		// handle error
//	}
//
//	// Using both custom analyzer and lexicon/modifier configs
//	analyzer, err := NewAnalyzer(
//		WithConfig(&Config{MinWordCount: 2, EnableEmoticons: false}),
//		WithLexiconModConfigFile("/path/to/lexicon_config.json"),
//	)
//	if err != nil {
//		// handle error
//	}
func NewAnalyzer(opts ...AnalyzerOption) (*Analyzer, error) {
	options := &analyzerOptions{
		config:           nil,
		lexiconModConfig: nil,
	}

	// Apply all options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	// Use default analyzer config if none was provided
	if options.config == nil {
		options.config = DefaultConfig()
	}

	// Initialize lexicon and modifier packages with the provided or default config
	// Note: Due to sync.Once in these packages, only the first call takes effect.
	// The init() function already initializes with defaults, so this call will not
	// change the packages if they're already initialized.
	if err := initializePackages(options.lexiconModConfig); err != nil {
		return nil, err
	}

	return &Analyzer{
		config: options.config,
	}, nil
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
	case combinedScore < NEGATIVE_SENTIMENT_THRESHOLD:
		sentiment = VeryNegative
	case combinedScore < NEGATIVE_SENTIMENT_THRESHOLD:
		sentiment = Negative
	case combinedScore < NEUTRAL_SENTIMENT_THRESHOLD:
		sentiment = Neutral
	case combinedScore < POSITIVE_SENTIMENT_THRESHOLD:
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
	case score < NEGATIVE_SENTIMENT_THRESHOLD:
		sentiment = VeryNegative
	case score < NEGATIVE_SENTIMENT_THRESHOLD:
		sentiment = Negative
	case score < NEUTRAL_SENTIMENT_THRESHOLD:
		sentiment = Neutral
	case score < POSITIVE_SENTIMENT_THRESHOLD:
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

// AnalyzePostText analyzes the sentiment of text content using a default analyzer.
// This is a convenience function for simple use cases that don't require custom configuration.
//
// For more control over configuration, use NewAnalyzer with WithConfig or WithConfigFile,
// then call AnalyzePost or AnalyzeComment on the returned analyzer.
//
// This function always creates a new analyzer internally, so it's less efficient
// if you need to analyze multiple posts or comments. In that case, create a single
// analyzer instance and reuse it.
//
// The returned PostSentiment includes overall sentiment classification and detailed
// scoring information. If analysis fails, an error is returned.
func AnalyzePostText(ctx context.Context, post *types.Post) (*PostSentiment, error) {
	analyzer, err := NewAnalyzer()
	if err != nil {
		return nil, err
	}
	return analyzer.AnalyzePost(ctx, post)
}

// AnalyzeCommentText analyzes the sentiment of a comment using a default analyzer.
// This is a convenience function for simple use cases that don't require custom configuration.
//
// For more control over configuration, use NewAnalyzer with WithConfig or WithConfigFile,
// then call AnalyzeComment on the returned analyzer.
//
// This function always creates a new analyzer internally, so it's less efficient
// if you need to analyze multiple comments. In that case, create a single
// analyzer instance and reuse it.
//
// The returned CommentSentiment includes sentiment classification and scoring information.
// If analysis fails, an error is returned.
func AnalyzeCommentText(ctx context.Context, comment *types.Comment) (*CommentSentiment, error) {
	analyzer, err := NewAnalyzer()
	if err != nil {
		return nil, err
	}
	return analyzer.AnalyzeComment(ctx, comment)
}
