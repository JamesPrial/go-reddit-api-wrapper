package analyzer

import (
	"math"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/jamesprial/go-reddit-api-wrapper/sentiment/config"
)

// Sentiment value constants (match sentiment.Sentiment enum values)
const (
	veryNegativeSentiment = -2
	negativeSentiment     = -1
	neutralSentiment      = 0
	positiveSentiment     = 1
	veryPositiveSentiment = 2
)

// ============================================================================
// Mock Implementations
// ============================================================================

// mockLexicon implements LexiconProvider for testing
type mockLexicon struct {
	scores          map[string]float64
	emoticons       []string
	multiplier      float64
	negationWords   map[string]bool
	isPositive      func(string) bool
	isNegative      func(string) bool
	getScore        func(string) float64
	getMultiplier   func(string) float64
	detectNegation  func([]string, int) bool
}

func (m *mockLexicon) GetScore(word string) float64 {
	if m.getScore != nil {
		return m.getScore(word)
	}
	if score, ok := m.scores[word]; ok {
		return score
	}
	return 0.0
}

func (m *mockLexicon) IsPositive(word string) bool {
	if m.isPositive != nil {
		return m.isPositive(word)
	}
	if score, ok := m.scores[word]; ok {
		return score > 0
	}
	return false
}

func (m *mockLexicon) IsNegative(word string) bool {
	if m.isNegative != nil {
		return m.isNegative(word)
	}
	if score, ok := m.scores[word]; ok {
		return score < 0
	}
	return false
}

func (m *mockLexicon) ExtractEmoticons(text string) []string {
	if m.emoticons != nil {
		return m.emoticons
	}
	return []string{}
}

func (m *mockLexicon) GetMultiplier(text string) float64 {
	if m.getMultiplier != nil {
		return m.getMultiplier(text)
	}
	return m.multiplier
}

func (m *mockLexicon) DetectNegation(tokens []string, index int) bool {
	if m.detectNegation != nil {
		return m.detectNegation(tokens, index)
	}
	// Default implementation: check for negation words in previous 3 tokens
	if index <= 0 || len(tokens) == 0 {
		return false
	}
	lookbackStart := max(0, index-3)
	for i := lookbackStart; i < index; i++ {
		if m.negationWords != nil && m.negationWords[tokens[i]] {
			return true
		}
	}
	return false
}

// mockPreprocessor implements PreprocessorProvider for testing
type mockPreprocessor struct {
	isDeleted func(string) bool
	tokenize  func(string) []string
}

func (m *mockPreprocessor) IsDeleted(author string) bool {
	if m.isDeleted != nil {
		return m.isDeleted(author)
	}
	author = strings.TrimSpace(author)
	return author == "[deleted]" || author == "[removed]"
}

func (m *mockPreprocessor) Tokenize(text string) []string {
	if m.tokenize != nil {
		return m.tokenize(text)
	}
	// Simple tokenization for testing - matches real preprocessor behavior
	text = strings.ToLower(text)
	words := strings.Fields(text)
	var tokens []string
	for _, word := range words {
		// Remove punctuation from edges (not just specific characters)
		word = strings.TrimFunc(word, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		if word != "" {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// ============================================================================
// Test Helpers
// ============================================================================

// almostEqual checks if two float64 values are approximately equal.
func almostEqual(a, b float64) bool {
	epsilon := 0.0001
	return math.Abs(a-b) < epsilon
}

// createTestAnalyzer creates an Analyzer with mock dependencies for testing.
func createTestAnalyzer() *Analyzer {
	lex := &mockLexicon{
		scores: map[string]float64{
			"good":      1.0,
			"great":     1.5,
			"excellent": 2.0,
			"love":      1.5,
			"amazing":   2.0,
			"bad":       -1.0,
			"terrible":  -1.5,
			"awful":     -2.0,
			"hate":      -1.5,
			"horrible":  -2.0,
		},
		multiplier: 1.0,
	}
	
	prep := &mockPreprocessor{}
	
	return &Analyzer{
		lexicon:         lex,
		preprocessor:    prep,
		minWordCount:    0,
		enableEmoticons: false,
	}
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewAnalyzer(t *testing.T) {
	tests := []struct {
		name             string
		minWordCount     int
		enableEmoticons  bool
		expectNonNil     bool
	}{
		{
			name:            "default configuration",
			minWordCount:    0,
			enableEmoticons: false,
			expectNonNil:    true,
		},
		{
			name:            "with min word count",
			minWordCount:    5,
			enableEmoticons: false,
			expectNonNil:    true,
		},
		{
			name:            "with emoticons enabled",
			minWordCount:    0,
			enableEmoticons: true,
			expectNonNil:    true,
		},
		{
			name:            "all options enabled",
			minWordCount:    10,
			enableEmoticons: true,
			expectNonNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lex := &mockLexicon{scores: map[string]float64{"test": 1.0}, multiplier: 1.0}
			analyzer := NewAnalyzer(lex, tt.minWordCount, tt.enableEmoticons)
			
			if (analyzer != nil) != tt.expectNonNil {
				t.Errorf("NewAnalyzer() returned nil=%v, expected non-nil=%v", analyzer == nil, tt.expectNonNil)
			}
			
			if analyzer == nil {
				return
			}
			
			if analyzer.minWordCount != tt.minWordCount {
				t.Errorf("minWordCount = %d, want %d", analyzer.minWordCount, tt.minWordCount)
			}
			
			if analyzer.enableEmoticons != tt.enableEmoticons {
				t.Errorf("enableEmoticons = %v, want %v", analyzer.enableEmoticons, tt.enableEmoticons)
			}
		})
	}
}

// ============================================================================
// AnalyzeText Tests - Basic Cases
// ============================================================================

func TestAnalyzeText_EmptyAndInvalid(t *testing.T) {
	analyzer := createTestAnalyzer()
	
	tests := []struct {
		name              string
		text              string
		expectedSentiment int
		expectedScore     float64
		expectedConfidence float64
	}{
		{
			name:              "empty string",
			text:              "",
			expectedSentiment: neutralSentiment,
			expectedScore:     0.0,
			expectedConfidence: 0.0,
		},
		{
			name:              "deleted content",
			text:              "[deleted]",
			expectedSentiment: neutralSentiment,
			expectedScore:     0.0,
			expectedConfidence: 0.0,
		},
		{
			name:              "removed content",
			text:              "[removed]",
			expectedSentiment: neutralSentiment,
			expectedScore:     0.0,
			expectedConfidence: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentiment, score, confidence := analyzer.AnalyzeText(tt.text)
			
			if sentiment != tt.expectedSentiment {
				t.Errorf("sentiment = %d, want %d", sentiment, tt.expectedSentiment)
			}
			if !almostEqual(score, tt.expectedScore) {
				t.Errorf("score = %f, want %f", score, tt.expectedScore)
			}
			if !almostEqual(confidence, tt.expectedConfidence) {
				t.Errorf("confidence = %f, want %f", confidence, tt.expectedConfidence)
			}
		})
	}
}

func TestAnalyzeText_MinWordCount(t *testing.T) {
	lex := &mockLexicon{
		scores: map[string]float64{
			"good": 1.0,
		},
		multiplier: 1.0,
	}
	prep := &mockPreprocessor{}
	
	analyzer := &Analyzer{
		lexicon:         lex,
		preprocessor:    prep,
		minWordCount:    5,
		enableEmoticons: false,
	}
	
	tests := []struct {
		name              string
		text              string
		expectedSentiment int
		expectedScore     float64
		expectedConfidence float64
	}{
		{
			name:              "below min word count",
			text:              "good",
			expectedSentiment: neutralSentiment,
			expectedScore:     0.0,
			expectedConfidence: 0.0,
		},
		{
			name:              "exactly at min word count",
			text:              "this is very good text",
			expectedSentiment: positiveSentiment,
			expectedScore:     0.2,
			expectedConfidence: 0.24,
		},
		{
			name:              "above min word count",
			text:              "this is a very good long text",
			expectedSentiment: neutralSentiment,
			expectedScore:     0.142857,
			expectedConfidence: 0.171429,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentiment, score, confidence := analyzer.AnalyzeText(tt.text)
			
			if sentiment != tt.expectedSentiment {
				t.Errorf("sentiment = %d, want %d", sentiment, tt.expectedSentiment)
			}
			if !almostEqual(score, tt.expectedScore) {
				t.Errorf("score = %f, want %f", score, tt.expectedScore)
			}
			if !almostEqual(confidence, tt.expectedConfidence) {
				t.Errorf("confidence = %f, want %f", confidence, tt.expectedConfidence)
			}
		})
	}
}

// ============================================================================
// AnalyzeText Tests - Sentiment Classification
// ============================================================================

func TestAnalyzeText_SentimentClassification(t *testing.T) {
	tests := []struct {
		name              string
		setupAnalyzer     func() *Analyzer
		text              string
		expectedSentiment int
		minScore          float64
		maxScore          float64
	}{
		{
			name: "very negative sentiment",
			setupAnalyzer: func() *Analyzer {
				lex := &mockLexicon{
					scores:     map[string]float64{},
					multiplier: 1.0,
					getScore: func(s string) float64 {
						return -0.7 // Very negative score
					},
				}
				prep := &mockPreprocessor{}
				return &Analyzer{
					lexicon:         lex,
					preprocessor:    prep,
					minWordCount:    0,
					enableEmoticons: true,
				}
			},
			text:              "test",
			expectedSentiment: veryNegativeSentiment,
			minScore:          -1.0,
			maxScore:          config.VERY_NEGATIVE_SCORE_THRESHOLD,
		},
		{
			name: "negative sentiment",
			setupAnalyzer: func() *Analyzer {
				lex := &mockLexicon{
					scores:     map[string]float64{},
					multiplier: 1.0,
					getScore: func(s string) float64 {
						return -0.3
					},
				}
				prep := &mockPreprocessor{}
				return &Analyzer{
					lexicon:         lex,
					preprocessor:    prep,
					minWordCount:    0,
					enableEmoticons: true,
				}
			},
			text:              "test",
			expectedSentiment: negativeSentiment,
			minScore:          config.VERY_NEGATIVE_SCORE_THRESHOLD,
			maxScore:          config.NEGATIVE_SCORE_THRESHOLD,
		},
		{
			name: "neutral sentiment",
			setupAnalyzer: func() *Analyzer {
				lex := &mockLexicon{
					scores:     map[string]float64{},
					multiplier: 1.0,
					getScore: func(s string) float64 {
						return 0.0
					},
				}
				prep := &mockPreprocessor{}
				return &Analyzer{
					lexicon:         lex,
					preprocessor:    prep,
					minWordCount:    0,
					enableEmoticons: true,
				}
			},
			text:              "test",
			expectedSentiment: neutralSentiment,
			minScore:          config.NEGATIVE_SCORE_THRESHOLD,
			maxScore:          config.NEUTRAL_SCORE_THRESHOLD,
		},
		{
			name: "positive sentiment",
			setupAnalyzer: func() *Analyzer {
				lex := &mockLexicon{
					scores:     map[string]float64{},
					multiplier: 1.0,
					getScore: func(s string) float64 {
						return 0.4
					},
				}
				prep := &mockPreprocessor{}
				return &Analyzer{
					lexicon:         lex,
					preprocessor:    prep,
					minWordCount:    0,
					enableEmoticons: true,
				}
			},
			text:              "test",
			expectedSentiment: positiveSentiment,
			minScore:          config.NEUTRAL_SCORE_THRESHOLD,
			maxScore:          config.POSITIVE_SCORE_THRESHOLD,
		},
		{
			name: "very positive sentiment",
			setupAnalyzer: func() *Analyzer {
				lex := &mockLexicon{
					scores:     map[string]float64{},
					multiplier: 1.0,
					getScore: func(s string) float64 {
						return 0.8
					},
				}
				prep := &mockPreprocessor{}
				return &Analyzer{
					lexicon:         lex,
					preprocessor:    prep,
					minWordCount:    0,
					enableEmoticons: true,
				}
			},
			text:              "test",
			expectedSentiment: veryPositiveSentiment,
			minScore:          config.POSITIVE_SCORE_THRESHOLD,
			maxScore:          1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := tt.setupAnalyzer()
			sentiment, score, _ := analyzer.AnalyzeText(tt.text)
			
			if sentiment != tt.expectedSentiment {
				t.Errorf("sentiment = %d, want %d", sentiment, tt.expectedSentiment)
			}
			
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("score = %f, expected in range [%f, %f]", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

// ============================================================================
// AnalyzeText Tests - Emoticons
// ============================================================================

func TestAnalyzeText_Emoticons(t *testing.T) {
	tests := []struct {
		name              string
		enableEmoticons   bool
		emoticonScore     float64
		multiplier        float64
		text              string
		expectedEmoticons []string
		expectedScoreMin  float64
		expectedScoreMax  float64
	}{
		{
			name:             "emoticons disabled",
			enableEmoticons:  false,
			emoticonScore:    1.0,
			multiplier:       1.0,
			text:             "test :)",
			expectedEmoticons: []string{":)"},
			expectedScoreMin: 0.0,
			expectedScoreMax: 0.0,
		},
		{
			name:             "emoticons enabled positive",
			enableEmoticons:  true,
			emoticonScore:    0.5,
			multiplier:       1.0,
			text:             "test :)",
			expectedEmoticons: []string{":)"},
			expectedScoreMin: 0.5,
			expectedScoreMax: 0.5,
		},
		{
			name:             "emoticons enabled negative",
			enableEmoticons:  true,
			emoticonScore:    -0.5,
			multiplier:       1.0,
			text:             "test :(",
			expectedEmoticons: []string{":("},
			expectedScoreMin: -0.5,
			expectedScoreMax: -0.5,
		},
		{
			name:             "emoticons with multiplier",
			enableEmoticons:  true,
			emoticonScore:    1.0,
			multiplier:       2.0,
			text:             "test :D",
			expectedEmoticons: []string{":D"},
			expectedScoreMin: 1.0, // Clamped to config.MAX_SCORE
			expectedScoreMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create lexicon where getScore handles both words and emoticons
			lex := &mockLexicon{
				scores:     map[string]float64{},
				emoticons: tt.expectedEmoticons, // Only return emoticons actually in text
				multiplier: tt.multiplier,
				getScore: func(s string) float64 {
					// Only return score if word is in emoticons list
					if slices.Contains(tt.expectedEmoticons, s) {
						return tt.emoticonScore
					}
					return 0.0
				},
			}
			prep := &mockPreprocessor{}
			
			analyzer := &Analyzer{
				lexicon:         lex,
				preprocessor:    prep,
				minWordCount:    0,
				enableEmoticons: tt.enableEmoticons,
			}
			
			_, score, _ := analyzer.AnalyzeText(tt.text)
			
			if score < tt.expectedScoreMin || score > tt.expectedScoreMax {
				t.Errorf("score = %f, expected in range [%f, %f]", 
					score, tt.expectedScoreMin, tt.expectedScoreMax)
			}
		})
	}
}

func TestAnalyzeText_Confidence(t *testing.T) {
	tests := []struct {
		name               string
		text               string
		enableEmoticons    bool
		emoticonScore      float64
		expectedConfMin    float64
		expectedConfMax    float64
	}{
		{
			name:            "no tokens zero confidence",
			text:            "",
			enableEmoticons: false,
			emoticonScore:   0.0,
			expectedConfMin: 0.0,
			expectedConfMax: 0.0,
		},
		{
			name:            "no emoticons no confidence",
			text:            "test word",
			enableEmoticons: false,
			emoticonScore:   0.0,
			expectedConfMin: 0.0,
			expectedConfMax: 0.0,
		},
		{
			name:            "emoticons enabled with match",
			text:            "test",
			enableEmoticons: true,
			emoticonScore:   1.0,
			expectedConfMin: 1.0,
			expectedConfMax: 1.0,
		},
		{
			name:            "confidence capped at max",
			text:            "test",
			enableEmoticons: true,
			emoticonScore:   1.0,
			expectedConfMin: 0.0,
			expectedConfMax: config.MAX_CONFIDENCE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lex := &mockLexicon{
				scores:     map[string]float64{},
				multiplier: 1.0,
				getScore: func(s string) float64 {
					return tt.emoticonScore
				},
			}
			prep := &mockPreprocessor{}
			
			analyzer := &Analyzer{
				lexicon:         lex,
				preprocessor:    prep,
				minWordCount:    0,
				enableEmoticons: tt.enableEmoticons,
			}
			
			_, _, confidence := analyzer.AnalyzeText(tt.text)
			
			if confidence < tt.expectedConfMin || confidence > tt.expectedConfMax {
				t.Errorf("confidence = %f, expected in range [%f, %f]", 
					confidence, tt.expectedConfMin, tt.expectedConfMax)
			}
			
			// Confidence should never exceed config.MAX_CONFIDENCE
			if confidence > config.MAX_CONFIDENCE {
				t.Errorf("confidence = %f exceeds config.MAX_CONFIDENCE %f", confidence, config.MAX_CONFIDENCE)
			}
		})
	}
}

// ============================================================================
// AnalyzeText Tests - Score Clamping
// ============================================================================

func TestAnalyzeText_ScoreClamping(t *testing.T) {
	tests := []struct {
		name          string
		rawScore      float64
		multiplier    float64
		expectedScore float64
	}{
		{
			name:          "score above max clamped",
			rawScore:      2.0,
			multiplier:    1.0,
			expectedScore: config.MAX_SCORE,
		},
		{
			name:          "score below min clamped",
			rawScore:      -2.0,
			multiplier:    1.0,
			expectedScore: config.MIN_SCORE,
		},
		{
			name:          "score within range unchanged",
			rawScore:      0.5,
			multiplier:    1.0,
			expectedScore: 0.5,
		},
		{
			name:          "score with multiplier above max",
			rawScore:      0.8,
			multiplier:    2.0,
			expectedScore: config.MAX_SCORE,
		},
		{
			name:          "score with multiplier below min",
			rawScore:      -0.8,
			multiplier:    2.0,
			expectedScore: config.MIN_SCORE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lex := &mockLexicon{
				scores:     map[string]float64{},
				multiplier: tt.multiplier,
				getScore: func(s string) float64 {
					return tt.rawScore
				},
			}
			prep := &mockPreprocessor{}
			
			analyzer := &Analyzer{
				lexicon:         lex,
				preprocessor:    prep,
				minWordCount:    0,
				enableEmoticons: true,
			}
			
			_, score, _ := analyzer.AnalyzeText("test")
			
			if !almostEqual(score, tt.expectedScore) {
				t.Errorf("score = %f, want %f", score, tt.expectedScore)
			}
			
			// Verify score is always in valid range
			if score < config.MIN_SCORE || score > config.MAX_SCORE {
				t.Errorf("score %f outside valid range [%f, %f]", score, config.MIN_SCORE, config.MAX_SCORE)
			}
		})
	}
}

// ============================================================================
// AnalyzeText Tests - Multiplier Application
// ============================================================================

func TestAnalyzeText_MultiplierApplication(t *testing.T) {
	tests := []struct {
		name           string
		baseScore      float64
		multiplier     float64
		expectedScore  float64
	}{
		{
			name:          "no multiplier",
			baseScore:     0.5,
			multiplier:    1.0,
			expectedScore: 0.5,
		},
		{
			name:          "double multiplier",
			baseScore:     0.3,
			multiplier:    2.0,
			expectedScore: 0.6,
		},
		{
			name:          "punctuation multiplier",
			baseScore:     0.5,
			multiplier:    1.2,
			expectedScore: 0.6,
		},
		{
			name:          "caps multiplier",
			baseScore:     0.4,
			multiplier:    1.3,
			expectedScore: 0.52,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lex := &mockLexicon{
				scores:     map[string]float64{},
				multiplier: tt.multiplier,
				getScore: func(s string) float64 {
					return tt.baseScore
				},
			}
			prep := &mockPreprocessor{}
			
			analyzer := &Analyzer{
				lexicon:         lex,
				preprocessor:    prep,
				minWordCount:    0,
				enableEmoticons: true,
			}
			
			_, score, _ := analyzer.AnalyzeText("test")
			
			if !almostEqual(score, tt.expectedScore) {
				t.Errorf("score = %f, want %f", score, tt.expectedScore)
			}
		})
	}
}

// ============================================================================
// CombineScores Tests
// ============================================================================

func TestCombineScores(t *testing.T) {
	analyzer := createTestAnalyzer()
	
	tests := []struct {
		name          string
		scores        []float64
		expectedScore float64
	}{
		{
			name:          "no scores",
			scores:        []float64{},
			expectedScore: 0.0,
		},
		{
			name:          "single score",
			scores:        []float64{0.5},
			expectedScore: 0.5,
		},
		{
			name:          "two equal scores",
			scores:        []float64{0.5, 0.5},
			expectedScore: 0.5,
		},
		{
			name:          "two different scores",
			scores:        []float64{0.3, 0.7},
			expectedScore: 0.5,
		},
		{
			name:          "multiple scores",
			scores:        []float64{0.2, 0.4, 0.6, 0.8},
			expectedScore: 0.5,
		},
		{
			name:          "negative and positive",
			scores:        []float64{-0.5, 0.5},
			expectedScore: 0.0,
		},
		{
			name:          "all negative",
			scores:        []float64{-0.3, -0.5, -0.7},
			expectedScore: -0.5,
		},
		{
			name:          "all positive",
			scores:        []float64{0.3, 0.5, 0.7},
			expectedScore: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.CombineScores(tt.scores...)
			
			if !almostEqual(result, tt.expectedScore) {
				t.Errorf("CombineScores(%v) = %f, want %f", tt.scores, result, tt.expectedScore)
			}
		})
	}
}

func TestCombineScores_Clamping(t *testing.T) {
	analyzer := createTestAnalyzer()
	
	tests := []struct {
		name          string
		scores        []float64
		expectedScore float64
	}{
		{
			name:          "average above max",
			scores:        []float64{1.0, 1.0, 1.0, 1.0, 1.0},
			expectedScore: config.MAX_SCORE,
		},
		{
			name:          "average below min",
			scores:        []float64{-1.0, -1.0, -1.0, -1.0, -1.0},
			expectedScore: config.MIN_SCORE,
		},
		{
			name:          "extreme positive values",
			scores:        []float64{5.0, 10.0, 15.0},
			expectedScore: config.MAX_SCORE,
		},
		{
			name:          "extreme negative values",
			scores:        []float64{-5.0, -10.0, -15.0},
			expectedScore: config.MIN_SCORE,
		},
		{
			name:          "mixed extreme values averaging to in-range",
			scores:        []float64{5.0, -5.0},
			expectedScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.CombineScores(tt.scores...)
			
			if !almostEqual(result, tt.expectedScore) {
				t.Errorf("CombineScores(%v) = %f, want %f", tt.scores, result, tt.expectedScore)
			}
			
			// Verify result is always in valid range
			if result < config.MIN_SCORE || result > config.MAX_SCORE {
				t.Errorf("combined score %f outside valid range [%f, %f]", result, config.MIN_SCORE, config.MAX_SCORE)
			}
		})
	}
}

// ============================================================================
// Edge Cases and Integration Tests
// ============================================================================

func TestAnalyzeText_EdgeCases(t *testing.T) {
	t.Run("very long text", func(t *testing.T) {
		analyzer := createTestAnalyzer()
		longText := strings.Repeat("word ", 10000)
		sentiment, score, confidence := analyzer.AnalyzeText(longText)
		
		// Should complete without panic
		if sentiment < veryNegativeSentiment || sentiment > veryPositiveSentiment {
			t.Errorf("unexpected sentiment: %d", sentiment)
		}
		if score < config.MIN_SCORE || score > config.MAX_SCORE {
			t.Errorf("score %f outside valid range", score)
		}
		if confidence < 0.0 || confidence > config.MAX_CONFIDENCE {
			t.Errorf("confidence %f outside valid range", confidence)
		}
	})
	
	t.Run("unicode text", func(t *testing.T) {
		analyzer := createTestAnalyzer()
		text := "café résumé naïve 😀"
		sentiment, score, confidence := analyzer.AnalyzeText(text)
		
		// Should handle unicode gracefully
		if sentiment < veryNegativeSentiment || sentiment > veryPositiveSentiment {
			t.Errorf("unexpected sentiment: %d", sentiment)
		}
		if score < config.MIN_SCORE || score > config.MAX_SCORE {
			t.Errorf("score %f outside valid range", score)
		}
		if confidence < 0.0 || confidence > config.MAX_CONFIDENCE {
			t.Errorf("confidence %f outside valid range", confidence)
		}
	})
	
	t.Run("special characters only", func(t *testing.T) {
		analyzer := createTestAnalyzer()
		text := "!@#$%^&*()"
		sentiment, score, confidence := analyzer.AnalyzeText(text)
		
		// Should return neutral for no recognizable words
		if sentiment != neutralSentiment {
			t.Errorf("expected neutral sentiment, got %d", sentiment)
		}
		if score != 0.0 {
			t.Errorf("expected zero score, got %f", score)
		}
		if confidence != 0.0 {
			t.Errorf("expected zero confidence, got %f", confidence)
		}
	})
	
	t.Run("whitespace variations", func(t *testing.T) {
		analyzer := createTestAnalyzer()
		text := "good\t\n\r\ngreat"
		sentiment, _, _ := analyzer.AnalyzeText(text)
		
		// Should handle different whitespace types
		if sentiment < neutralSentiment {
			t.Errorf("expected positive sentiment, got %d", sentiment)
		}
	})
}

func TestAnalyzeText_ConfidenceScaling(t *testing.T) {
	t.Run("confidence scaling factor applied", func(t *testing.T) {
		lex := &mockLexicon{
			scores:     map[string]float64{},
			multiplier: 1.0,
			getScore: func(s string) float64 {
				return 1.0
			},
		}
		prep := &mockPreprocessor{
			tokenize: func(s string) []string {
				return []string{"word"}
			},
		}
		
		analyzer := &Analyzer{
			lexicon:         lex,
			preprocessor:    prep,
			minWordCount:    0,
			enableEmoticons: true,
		}
		
		_, _, confidence := analyzer.AnalyzeText("word")
		
		// With 1 match out of 1 word, confidence should be 1.0 * 1.2 = 1.2, capped at 1.0
		if !almostEqual(confidence, config.MAX_CONFIDENCE) {
			t.Errorf("confidence = %f, expected %f (with scaling)", confidence, config.MAX_CONFIDENCE)
		}
	})
	
	t.Run("confidence never exceeds max", func(t *testing.T) {
		lex := &mockLexicon{
			scores:     map[string]float64{},
			multiplier: 1.0,
			getScore: func(s string) float64 {
				return 1.0
			},
		}
		prep := &mockPreprocessor{
			tokenize: func(s string) []string {
				return []string{"word", "word", "word"}
			},
		}
		
		analyzer := &Analyzer{
			lexicon:         lex,
			preprocessor:    prep,
			minWordCount:    0,
			enableEmoticons: true,
		}
		
		_, _, confidence := analyzer.AnalyzeText("word word word")
		
		if confidence > config.MAX_CONFIDENCE {
			t.Errorf("confidence = %f exceeds config.MAX_CONFIDENCE %f", confidence, config.MAX_CONFIDENCE)
		}
	})
}

// ============================================================================
// Boundary Value Tests
// ============================================================================

func TestAnalyzer_BoundaryValues(t *testing.T) {
	t.Run("sentiment threshold boundaries", func(t *testing.T) {
		thresholds := []struct {
			score             float64
			expectedSentiment int
		}{
			{-0.61, veryNegativeSentiment},
			{-0.60, negativeSentiment}, // boundary: >= -0.6 is NEGATIVE
			{-0.59, negativeSentiment},
			{-0.21, negativeSentiment},
			{-0.20, neutralSentiment}, // boundary: >= -0.2 is NEUTRAL
			{-0.19, neutralSentiment},
			{0.19, neutralSentiment},
			{0.20, positiveSentiment}, // boundary: >= 0.2 is POSITIVE
			{0.21, positiveSentiment},
			{0.59, positiveSentiment},
			{0.60, veryPositiveSentiment}, // boundary: >= 0.6 is VERY_POSITIVE
			{0.61, veryPositiveSentiment},
		}
		
		for _, tt := range thresholds {
			t.Run(string(rune(tt.score)), func(t *testing.T) {
				lex := &mockLexicon{
					scores:     map[string]float64{},
					multiplier: 1.0,
					getScore: func(s string) float64 {
						return tt.score
					},
				}
				prep := &mockPreprocessor{}
				
				analyzer := &Analyzer{
					lexicon:         lex,
					preprocessor:    prep,
					minWordCount:    0,
					enableEmoticons: true,
				}
				
				sentiment, _, _ := analyzer.AnalyzeText("test")
				
				if sentiment != tt.expectedSentiment {
					t.Errorf("score %f: got sentiment %d, want %d", tt.score, sentiment, tt.expectedSentiment)
				}
			})
		}
	})
}

// ============================================================================
// Constants Tests
// ============================================================================

func TestAnalyzer_Constants(t *testing.T) {
	t.Run("score thresholds are ordered", func(t *testing.T) {
		if !(config.MIN_SCORE < config.VERY_NEGATIVE_SCORE_THRESHOLD &&
			config.VERY_NEGATIVE_SCORE_THRESHOLD < config.NEGATIVE_SCORE_THRESHOLD &&
			config.NEGATIVE_SCORE_THRESHOLD < config.NEUTRAL_SCORE_THRESHOLD &&
			config.NEUTRAL_SCORE_THRESHOLD < config.POSITIVE_SCORE_THRESHOLD &&
			config.POSITIVE_SCORE_THRESHOLD < config.MAX_SCORE) {
			t.Error("score thresholds are not properly ordered")
		}
	})
	
	t.Run("sentiment values are ordered", func(t *testing.T) {
		if !(veryNegativeSentiment < negativeSentiment &&
			negativeSentiment < neutralSentiment &&
			neutralSentiment < positiveSentiment &&
			positiveSentiment < veryPositiveSentiment) {
			t.Error("sentiment values are not properly ordered")
		}
	})
	
	t.Run("confidence values are valid", func(t *testing.T) {
		if config.MAX_CONFIDENCE != 1.0 {
			t.Errorf("config.MAX_CONFIDENCE = %f, expected 1.0", config.MAX_CONFIDENCE)
		}
		if config.CONFIDENCE_SCALING_FACTOR <= 1.0 {
			t.Errorf("config.CONFIDENCE_SCALING_FACTOR = %f, expected > 1.0", config.CONFIDENCE_SCALING_FACTOR)
		}
	})
}

