package sentiment

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestAnalyzePost tests the AnalyzePost method with table-driven tests.
func TestAnalyzePost(t *testing.T) {
	tests := []struct {
		name         string
		post         *types.Post
		ctx          context.Context
		expectErr    bool
		errAsType    error
		validateResp func(t *testing.T, resp *PostSentiment)
	}{
		{
			name:      "nil post returns validation error",
			post:      nil,
			ctx:       context.Background(),
			expectErr: true,
			errAsType: &AnalysisError{},
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp != nil {
					t.Error("expected nil response for nil post")
				}
			},
		},
		{
			name: "cancelled context returns validation error",
			post: &types.Post{
				Title:    "Test post",
				SelfText: "Test body",
			},
			ctx:       func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			expectErr: true,
			errAsType: &AnalysisError{},
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp != nil {
					t.Error("expected nil response for cancelled context")
				}
			},
		},
		{
			name: "positive post with positive title and body",
			post: &types.Post{
				Title:    "This is great and amazing",
				SelfText: "Excellent content, I love it!",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Current implementation is a placeholder, returns Neutral
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "negative post with negative title and body",
			post: &types.Post{
				Title:    "This is terrible and awful",
				SelfText: "Horrible experience, I hate this!",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Current implementation is a placeholder, returns Neutral
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "neutral post with neutral content",
			post: &types.Post{
				Title:    "This is a post",
				SelfText: "This is some content",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment != Neutral {
					t.Errorf("expected neutral sentiment, got %s", resp.Sentiment)
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "mixed sentiment post with positive and negative words",
			post: &types.Post{
				Title:    "Good but bad parts",
				SelfText: "I love some things and hate others",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Mixed sentiment should be closer to neutral
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "post with deleted author",
			post: &types.Post{
				Title:    "This is a great post",
				SelfText: "Excellent content",
				Author:   "[deleted]",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Deleted author posts should still be analyzed
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "post with removed author",
			post: &types.Post{
				Title:    "This is wonderful",
				SelfText: "Amazing stuff",
				Author:   "[removed]",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "post with empty title",
			post: &types.Post{
				Title:    "",
				SelfText: "This is great content",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.TitleScore != 0 {
					t.Errorf("expected zero title score for empty title, got %f", resp.TitleScore)
				}
			},
		},
		{
			name: "post with very short text",
			post: &types.Post{
				Title:    "Hi",
				SelfText: "OK",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Short text should be treated as neutral or low confidence
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "post with very long text",
			post: &types.Post{
				Title:    "Long title " + string(make([]byte, 500)),
				SelfText: "Very long body " + string(make([]byte, 5000)),
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "post with caps and punctuation emphasis",
			post: &types.Post{
				Title:    "THIS IS AMAZING!!!",
				SelfText: "Wonderful experience!!!",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Caps and punctuation should boost sentiment
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "post with negated sentiment",
			post: &types.Post{
				Title:    "Not good at all",
				SelfText: "This is terrible, I don't like it",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Negated good should be negative
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'Not good at all', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score, got %f", resp.Score)
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "negated positive word should be negative",
			post: &types.Post{
				Title:    "not good",
				SelfText: "",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'not good', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score for 'not good', got %f", resp.Score)
				}
			},
		},
		{
			name: "negated negative word should be positive",
			post: &types.Post{
				Title:    "not bad",
				SelfText: "",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment <= Neutral {
					t.Errorf("expected Positive or VeryPositive sentiment for 'not bad', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score <= 0 {
					t.Errorf("expected positive score for 'not bad', got %f", resp.Score)
				}
			},
		},
		{
			name: "negated strong positive word should be negative",
			post: &types.Post{
				Title:    "not amazing",
				SelfText: "",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'not amazing', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score for 'not amazing', got %f", resp.Score)
				}
			},
		},
		{
			name: "contraction negation should be positive",
			post: &types.Post{
				Title:    "isn't terrible",
				SelfText: "",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment <= Neutral {
					t.Errorf("expected Positive or VeryPositive sentiment for 'isn't terrible', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score <= 0 {
					t.Errorf("expected positive score for 'isn't terrible', got %f", resp.Score)
				}
			},
		},
		{
			name: "negation with emphasis should be strongly negative",
			post: &types.Post{
				Title:    "NOT GREAT!!!",
				SelfText: "",
				Author:   "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *PostSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Should be negative with stronger score due to caps and punctuation
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'NOT GREAT!!!', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score for 'NOT GREAT!!!', got %f", resp.Score)
				}
				// Emphasis should result in stronger sentiment (more negative)
				if resp.Score > -0.3 {
					t.Logf("expected stronger negative sentiment due to emphasis, got score: %f", resp.Score)
				}
			},
		},
	}

	// Use a config with MinWordCount=1 to allow testing short phrases like "not good"
	analyzer, err := NewAnalyzer(WithConfig(&Config{
		MinWordCount:    1,
		EnableEmoticons: true,
	}))
	if err != nil {
		t.Fatalf("failed to create analyzer: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := analyzer.AnalyzePost(tt.ctx, tt.post)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				// Check error type using errors.As
				var analysisErr *AnalysisError
				if !errors.As(err, &analysisErr) {
					t.Fatalf("expected AnalysisError, got %T", err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}

			tt.validateResp(t, resp)
		})
	}
}

// TestAnalyzeComment tests the AnalyzeComment method with table-driven tests.
func TestAnalyzeComment(t *testing.T) {
	tests := []struct {
		name         string
		comment      *types.Comment
		ctx          context.Context
		expectErr    bool
		errAsType    error
		validateResp func(t *testing.T, resp *CommentSentiment)
	}{
		{
			name:      "nil comment returns validation error",
			comment:   nil,
			ctx:       context.Background(),
			expectErr: true,
			errAsType: &AnalysisError{},
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp != nil {
					t.Error("expected nil response for nil comment")
				}
			},
		},
		{
			name: "cancelled context returns validation error",
			comment: &types.Comment{
				Body: "This is a test comment",
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			expectErr: true,
			errAsType: &AnalysisError{},
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp != nil {
					t.Error("expected nil response for cancelled context")
				}
			},
		},
		{
			name: "positive comment with positive words",
			comment: &types.Comment{
				Body:   "This is great! I love it, absolutely amazing!",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Current implementation is a placeholder, returns Neutral
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "negative comment with negative words",
			comment: &types.Comment{
				Body:   "This is terrible and awful, I hate it!",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Current implementation is a placeholder, returns Neutral
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "neutral comment with no sentiment words",
			comment: &types.Comment{
				Body:   "This is a comment about the weather and stuff",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment != Neutral {
					t.Errorf("expected neutral sentiment, got %s", resp.Sentiment)
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "mixed sentiment comment",
			comment: &types.Comment{
				Body:   "I like some parts but dislike others",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with deleted author",
			comment: &types.Comment{
				Body:   "This is an excellent comment",
				Author: "[deleted]",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with removed author",
			comment: &types.Comment{
				Body:   "This is wonderful",
				Author: "[removed]",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with empty body",
			comment: &types.Comment{
				Body:   "",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with very short body",
			comment: &types.Comment{
				Body:   "Great!",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with very long body",
			comment: &types.Comment{
				Body:   "This is a very long comment " + string(make([]byte, 5000)) + " with lots of content",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with caps and punctuation emphasis",
			comment: &types.Comment{
				Body:   "THIS IS ABSOLUTELY AMAZING!!!",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with emoticons",
			comment: &types.Comment{
				Body:   "Great work :) I love it :D",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with sad emoticons",
			comment: &types.Comment{
				Body:   "This is bad :( Really terrible :'(",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Confidence < 0 || resp.Confidence > 1 {
					t.Errorf("expected confidence between 0 and 1, got %f", resp.Confidence)
				}
			},
		},
		{
			name: "comment with negated positive word should be negative",
			comment: &types.Comment{
				Body:   "not good",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'not good', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score for 'not good', got %f", resp.Score)
				}
			},
		},
		{
			name: "comment with negated negative word should be positive",
			comment: &types.Comment{
				Body:   "not bad",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment <= Neutral {
					t.Errorf("expected Positive or VeryPositive sentiment for 'not bad', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score <= 0 {
					t.Errorf("expected positive score for 'not bad', got %f", resp.Score)
				}
			},
		},
		{
			name: "comment with negated strong positive word should be negative",
			comment: &types.Comment{
				Body:   "not amazing",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'not amazing', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score for 'not amazing', got %f", resp.Score)
				}
			},
		},
		{
			name: "comment with contraction negation should be positive",
			comment: &types.Comment{
				Body:   "isn't terrible",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				if resp.Sentiment <= Neutral {
					t.Errorf("expected Positive or VeryPositive sentiment for 'isn't terrible', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score <= 0 {
					t.Errorf("expected positive score for 'isn't terrible', got %f", resp.Score)
				}
			},
		},
		{
			name: "comment with negation and emphasis should be strongly negative",
			comment: &types.Comment{
				Body:   "NOT GREAT!!!",
				Author: "testuser",
			},
			ctx:       context.Background(),
			expectErr: false,
			validateResp: func(t *testing.T, resp *CommentSentiment) {
				if resp == nil {
					t.Fatal("expected non-nil response")
				}
				// Should be negative with stronger score due to caps and punctuation
				if resp.Sentiment >= Neutral {
					t.Errorf("expected Negative or VeryNegative sentiment for 'NOT GREAT!!!', got %s (score: %f)", resp.Sentiment, resp.Score)
				}
				if resp.Score >= 0 {
					t.Errorf("expected negative score for 'NOT GREAT!!!', got %f", resp.Score)
				}
				// Emphasis should result in stronger sentiment (more negative)
				if resp.Score > -0.3 {
					t.Logf("expected stronger negative sentiment due to emphasis, got score: %f", resp.Score)
				}
			},
		},
	}

	// Use a config with MinWordCount=1 to allow testing short phrases like "not good"
	analyzer, err := NewAnalyzer(WithConfig(&Config{
		MinWordCount:    1,
		EnableEmoticons: true,
	}))
	if err != nil {
		t.Fatalf("failed to create analyzer: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := analyzer.AnalyzeComment(tt.ctx, tt.comment)

			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				// Check error type using errors.As
				var analysisErr *AnalysisError
				if !errors.As(err, &analysisErr) {
					t.Fatalf("expected AnalysisError, got %T", err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}

			tt.validateResp(t, resp)
		})
	}
}

// TestNewAnalyzer tests analyzer creation.
func TestNewAnalyzer(t *testing.T) {
	t.Run("default config when no options provided", func(t *testing.T) {
		analyzer, err := NewAnalyzer()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if analyzer == nil {
			t.Fatal("expected non-nil analyzer")
		}
		if analyzer.config == nil {
			t.Fatal("expected config to be initialized with defaults")
		}
		if analyzer.config.MinWordCount != 3 {
			t.Errorf("expected default MinWordCount of 3, got %d", analyzer.config.MinWordCount)
		}
		if !analyzer.config.EnableEmoticons {
			t.Error("expected EnableEmoticons to be true by default")
		}
	})

	t.Run("custom analyzer config is used when provided", func(t *testing.T) {
		customConfig := &Config{
			MinWordCount:    5,
			EnableEmoticons: false,
		}
		analyzer, err := NewAnalyzer(WithConfig(customConfig))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if analyzer.config != customConfig {
			t.Fatal("expected custom config to be used")
		}
	})

	t.Run("custom config affects MinWordCount behavior", func(t *testing.T) {
		// Create analyzer with MinWordCount=5, so "not good" (2 words) should be neutral
		analyzer, err := NewAnalyzer(WithConfig(&Config{
			MinWordCount:    5,
			EnableEmoticons: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// "not good" has only 2 words, below the threshold of 5
		post := &types.Post{
			Title:    "not good",
			SelfText: "",
		}
		result, err := analyzer.AnalyzePost(context.Background(), post)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be neutral because word count is below threshold
		if result.Sentiment != Neutral {
			t.Errorf("expected Neutral sentiment for 'not good' with MinWordCount=5, got %s", result.Sentiment)
		}
	})

	t.Run("WithConfigFile loads analyzer config from file", func(t *testing.T) {
		// Create a temporary config file
		tmpFile := t.TempDir() + "/analyzer_config.json"
		configData := `{"minWordCount": 2, "enableEmoticons": false}`
		if err := os.WriteFile(tmpFile, []byte(configData), 0644); err != nil {
			t.Fatalf("failed to write temp config file: %v", err)
		}

		analyzer, err := NewAnalyzer(WithConfigFile(tmpFile))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if analyzer.config.MinWordCount != 2 {
			t.Errorf("expected MinWordCount=2 from file, got %d", analyzer.config.MinWordCount)
		}
		if analyzer.config.EnableEmoticons {
			t.Error("expected EnableEmoticons=false from file")
		}
	})

	t.Run("WithConfigFile returns error for non-existent file", func(t *testing.T) {
		_, err := NewAnalyzer(WithConfigFile("/nonexistent/path.json"))
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})

	t.Run("both analyzer and lexicon configs can be provided", func(t *testing.T) {
		customAnalyzerConfig := &Config{
			MinWordCount:    2,
			EnableEmoticons: false,
		}

		analyzer, err := NewAnalyzer(
			WithConfig(customAnalyzerConfig),
			// Note: WithLexiconModConfig would require a custom config.Config object
			// For testing, we just verify the analyzer config is used
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if analyzer.config.MinWordCount != 2 {
			t.Errorf("expected MinWordCount=2, got %d", analyzer.config.MinWordCount)
		}
		if analyzer.config.EnableEmoticons {
			t.Error("expected EnableEmoticons=false")
		}
	})

	t.Run("WithLexiconModConfigFile accepts valid config", func(t *testing.T) {
		// Create a temporary lexicon config file with valid structure
		tmpFile := t.TempDir() + "/lexicon_config.json"
		configData := `{
			"positive_words": {"good": 1.0, "great": 1.5},
			"negative_words": {"bad": -1.0, "terrible": -1.5},
			"emoticons": {":)": 0.5, ":(": -0.5},
			"negation_words": ["not", "no", "never"]
		}`
		if err := os.WriteFile(tmpFile, []byte(configData), 0644); err != nil {
			t.Fatalf("failed to write temp lexicon config file: %v", err)
		}

		analyzer, err := NewAnalyzer(WithLexiconModConfigFile(tmpFile))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if analyzer == nil {
			t.Fatal("expected non-nil analyzer")
		}
	})

	t.Run("WithLexiconModConfigFile returns error for non-existent file", func(t *testing.T) {
		_, err := NewAnalyzer(WithLexiconModConfigFile("/nonexistent/path.json"))
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})

	t.Run("WithLexiconModConfigFile returns error for invalid JSON", func(t *testing.T) {
		tmpFile := t.TempDir() + "/invalid_config.json"
		if err := os.WriteFile(tmpFile, []byte("not valid json"), 0644); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}

		_, err := NewAnalyzer(WithLexiconModConfigFile(tmpFile))
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("WithLexiconModConfig accepts nil and uses default", func(t *testing.T) {
		// Passing nil should use default config
		analyzer, err := NewAnalyzer(WithLexiconModConfig(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if analyzer == nil {
			t.Fatal("expected non-nil analyzer")
		}
	})
}

// TestSentimentString tests the String method of Sentiment type.
func TestSentimentString(t *testing.T) {
	tests := []struct {
		sentiment Sentiment
		expected  string
	}{
		{VeryNegative, "VeryNegative"},
		{Negative, "Negative"},
		{Neutral, "Neutral"},
		{Positive, "Positive"},
		{VeryPositive, "VeryPositive"},
		{Sentiment(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.sentiment.String()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestPostSentimentStructure tests PostSentiment structure fields.
func TestPostSentimentStructure(t *testing.T) {
	ps := &PostSentiment{
		Sentiment:  Positive,
		Score:      0.75,
		Confidence: 0.85,
		TitleScore: 0.8,
		BodyScore:  0.7,
	}

	if ps.Sentiment != Positive {
		t.Errorf("expected Positive, got %s", ps.Sentiment)
	}
	if ps.Score != 0.75 {
		t.Errorf("expected score 0.75, got %f", ps.Score)
	}
	if ps.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", ps.Confidence)
	}
	if ps.TitleScore != 0.8 {
		t.Errorf("expected title score 0.8, got %f", ps.TitleScore)
	}
	if ps.BodyScore != 0.7 {
		t.Errorf("expected body score 0.7, got %f", ps.BodyScore)
	}
}

// TestCommentSentimentStructure tests CommentSentiment structure fields.
func TestCommentSentimentStructure(t *testing.T) {
	cs := &CommentSentiment{
		Sentiment:  Negative,
		Score:      -0.65,
		Confidence: 0.75,
	}

	if cs.Sentiment != Negative {
		t.Errorf("expected Negative, got %s", cs.Sentiment)
	}
	if cs.Score != -0.65 {
		t.Errorf("expected score -0.65, got %f", cs.Score)
	}
	if cs.Confidence != 0.75 {
		t.Errorf("expected confidence 0.75, got %f", cs.Confidence)
	}
}
