package lexicon

import (
	"testing"
)

// TestGetPositiveWords tests that GetPositiveWords returns a non-empty map.
func TestGetPositiveWords(t *testing.T) {
	words := GetPositiveWords()

	t.Run("returns non-empty map", func(t *testing.T) {
		if len(words) == 0 {
			t.Error("expected non-empty positive words map")
		}
	})

	t.Run("contains expected words", func(t *testing.T) {
		expectedWords := []string{"good", "great", "excellent", "love", "amazing"}
		for _, word := range expectedWords {
			if _, exists := words[word]; !exists {
				t.Errorf("expected word %q in positive words map", word)
			}
		}
	})

	t.Run("all scores are in valid range [1.0, 2.0]", func(t *testing.T) {
		for word, score := range words {
			if score < 1.0 || score > 2.0 {
				t.Errorf("word %q has invalid score %f, expected range [1.0, 2.0]", word, score)
			}
		}
	})

	t.Run("scores are positive", func(t *testing.T) {
		for word, score := range words {
			if score <= 0 {
				t.Errorf("word %q has non-positive score %f", word, score)
			}
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		words1 := GetPositiveWords()
		words2 := GetPositiveWords()

		// Modify the first map
		words1["testword"] = 1.5

		// Check that the second map is not affected
		if _, exists := words2["testword"]; exists {
			t.Error("modifying returned map affected the original")
		}
	})
}

// TestGetNegativeWords tests that GetNegativeWords returns a non-empty map.
func TestGetNegativeWords(t *testing.T) {
	words := GetNegativeWords()

	t.Run("returns non-empty map", func(t *testing.T) {
		if len(words) == 0 {
			t.Error("expected non-empty negative words map")
		}
	})

	t.Run("contains expected words", func(t *testing.T) {
		expectedWords := []string{"bad", "terrible", "awful", "hate", "horrible"}
		for _, word := range expectedWords {
			if _, exists := words[word]; !exists {
				t.Errorf("expected word %q in negative words map", word)
			}
		}
	})

	t.Run("all scores are in valid range [-2.0, -0.5]", func(t *testing.T) {
		for word, score := range words {
			if score < -2.0 || score > -0.5 {
				t.Errorf("word %q has invalid score %f, expected range [-2.0, -0.5]", word, score)
			}
		}
	})

	t.Run("scores are negative", func(t *testing.T) {
		for word, score := range words {
			if score >= 0 {
				t.Errorf("word %q has non-negative score %f", word, score)
			}
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		words1 := GetNegativeWords()
		words2 := GetNegativeWords()

		// Modify the first map
		words1["testword"] = -1.5

		// Check that the second map is not affected
		if _, exists := words2["testword"]; exists {
			t.Error("modifying returned map affected the original")
		}
	})
}

// TestGetEmoticons tests that GetEmoticons returns a non-empty map.
func TestGetEmoticons(t *testing.T) {
	emoticons := GetEmoticons()

	t.Run("returns non-empty map", func(t *testing.T) {
		if len(emoticons) == 0 {
			t.Error("expected non-empty emoticons map")
		}
	})

	t.Run("contains expected emoticons", func(t *testing.T) {
		expectedEmoticons := []string{":)", ":(", ":-D", ":D"}
		for _, emoticon := range expectedEmoticons {
			if _, exists := emoticons[emoticon]; !exists {
				t.Errorf("expected emoticon %q in emoticons map", emoticon)
			}
		}
	})

	t.Run("happy emoticons have positive scores", func(t *testing.T) {
		happyEmoticons := []string{":)", ":-)"}
		for _, emoticon := range happyEmoticons {
			score, exists := emoticons[emoticon]
			if !exists {
				t.Errorf("expected emoticon %q", emoticon)
				continue
			}
			if score <= 0 {
				t.Errorf("emoticon %q should have positive score, got %f", emoticon, score)
			}
		}
	})

	t.Run("sad emoticons have negative scores", func(t *testing.T) {
		sadEmoticons := []string{":(", ":-("}
		for _, emoticon := range sadEmoticons {
			score, exists := emoticons[emoticon]
			if !exists {
				t.Errorf("expected emoticon %q", emoticon)
				continue
			}
			if score >= 0 {
				t.Errorf("emoticon %q should have negative score, got %f", emoticon, score)
			}
		}
	})

	t.Run("all scores are within reasonable range", func(t *testing.T) {
		minScore := -2.0
		maxScore := 2.0
		for emoticon, score := range emoticons {
			if score < minScore || score > maxScore {
				t.Errorf("emoticon %q has score %f outside range [%f, %f]", emoticon, score, minScore, maxScore)
			}
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		emoticons1 := GetEmoticons()
		emoticons2 := GetEmoticons()

		// Modify the first map
		emoticons1[":test:"] = 0.5

		// Check that the second map is not affected
		if _, exists := emoticons2[":test:"]; exists {
			t.Error("modifying returned map affected the original")
		}
	})
}

// TestNoOverlapBetweenLexicons tests that there is no overlap between positive and negative words.
func TestNoOverlapBetweenLexicons(t *testing.T) {
	positive := GetPositiveWords()
	negative := GetNegativeWords()

	overlaps := []string{}
	for word := range positive {
		if _, exists := negative[word]; exists {
			overlaps = append(overlaps, word)
		}
	}

	if len(overlaps) > 0 {
		t.Errorf("found overlapping words between positive and negative lexicons: %v", overlaps)
	}
}

// TestLexiconScoreDistribution tests the distribution of scores in lexicons.
func TestLexiconScoreDistribution(t *testing.T) {
	t.Run("positive words distribution", func(t *testing.T) {
		words := GetPositiveWords()

		score1Count := 0
		score15Count := 0
		score18Count := 0
		score2Count := 0

		for _, score := range words {
			// Round to check for common values
			if score >= 0.95 && score <= 1.05 {
				score1Count++
			} else if score >= 1.45 && score <= 1.55 {
				score15Count++
			} else if score >= 1.75 && score <= 1.85 {
				score18Count++
			} else if score >= 1.95 && score <= 2.05 {
				score2Count++
			}
		}

		// Should have a reasonable distribution
		if score1Count == 0 && score15Count == 0 && score18Count == 0 && score2Count == 0 {
			t.Error("expected some scores at common values")
		}
	})

	t.Run("negative words distribution", func(t *testing.T) {
		words := GetNegativeWords()

		scoreMinus1Count := 0
		scoreMinus15Count := 0
		scoreMinus18Count := 0
		scoreMinus2Count := 0

		for _, score := range words {
			// Round to check for common values
			if score <= -0.95 && score >= -1.05 {
				scoreMinus1Count++
			} else if score <= -1.45 && score >= -1.55 {
				scoreMinus15Count++
			} else if score <= -1.75 && score >= -1.85 {
				scoreMinus18Count++
			} else if score <= -1.95 && score >= -2.05 {
				scoreMinus2Count++
			}
		}

		// Should have a reasonable distribution
		if scoreMinus1Count == 0 && scoreMinus15Count == 0 && scoreMinus18Count == 0 && scoreMinus2Count == 0 {
			t.Error("expected some scores at common values")
		}
	})
}

// TestPositiveWordsCount verifies that we have a reasonable number of positive words.
func TestPositiveWordsCount(t *testing.T) {
	words := GetPositiveWords()
	if len(words) < 10 {
		t.Errorf("expected at least 10 positive words, got %d", len(words))
	}
}

// TestNegativeWordsCount verifies that we have a reasonable number of negative words.
func TestNegativeWordsCount(t *testing.T) {
	words := GetNegativeWords()
	if len(words) < 10 {
		t.Errorf("expected at least 10 negative words, got %d", len(words))
	}
}

// TestEmoticonsCount verifies that we have a reasonable number of emoticons.
func TestEmoticonsCount(t *testing.T) {
	emoticons := GetEmoticons()
	if len(emoticons) < 5 {
		t.Errorf("expected at least 5 emoticons, got %d", len(emoticons))
	}
}
