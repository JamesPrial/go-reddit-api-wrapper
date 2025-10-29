// Package lexicon provides internal implementation details for sentiment analysis.
package lexicon

// positiveWords maps positive sentiment words to their scores.
// Scores range from 1.0 to 2.0, with higher values indicating stronger positive sentiment.
var positiveWords = map[string]float64{
	"good":        1.0,
	"great":       1.5,
	"excellent":   2.0,
	"love":        1.5,
	"happy":       1.5,
	"amazing":     2.0,
	"wonderful":   1.8,
	"fantastic":   1.8,
	"awesome":     1.8,
	"brilliant":   2.0,
	"perfect":     2.0,
	"beautiful":   1.8,
	"superb":      1.8,
	"marvelous":   1.8,
	"outstanding": 2.0,
	"incredible":  1.8,
	"fabulous":    1.8,
	"delightful":  1.5,
	"lovely":      1.5,
	"nice":        1.0,
	"cool":        1.0,
	"impressed":   1.3,
	"proud":       1.5,
	"joyful":      1.5,
	"blessed":     1.3,
}

// negativeWords maps negative sentiment words to their scores.
// Scores range from -1.0 to -2.0, with lower values indicating stronger negative sentiment.
var negativeWords = map[string]float64{
	"bad":           -1.0,
	"terrible":      -1.5,
	"awful":         -2.0,
	"hate":          -1.5,
	"horrible":      -2.0,
	"worst":         -2.0,
	"disappointing": -1.3,
	"disgusting":    -2.0,
	"pathetic":      -1.8,
	"atrocious":     -2.0,
	"miserable":     -1.5,
	"dreadful":      -1.8,
	"tedious":       -1.0,
	"boring":        -1.0,
	"ugly":          -1.5,
	"stupid":        -1.5,
	"ridiculous":    -1.3,
	"shameful":      -1.5,
	"useless":       -1.5,
	"abysmal":       -2.0,
	"offensive":     -1.3,
	"mediocre":      -0.8,
	"poor":          -1.0,
	"waste":         -1.3,
	"sad":           -1.2,
}

// emoticons maps common emoticons to their sentiment scores.
// Scores range from -1.2 to 1.5, indicating negative or positive sentiment.
var emoticons = map[string]float64{
	":)":  0.5,
	":-)": 0.5,
	":-D": 1.0,
	":D":  1.0,
	":P":  0.5,
	";)":  0.5,
	"😊":   0.8,
	"😄":   0.8,
	"😍":   1.5,
	"❤️":  1.5,
	"🥰":   1.3,
	"😂":   1.0,
	":(":  -0.5,
	":-(": -0.5,
	":'(": -0.8,
	"😞":   -0.8,
	"😡":   -1.2,
	"😤":   -1.0,
	"😔":   -0.8,
	"💔":   -1.5,
	"😠":   -1.2,
}

// GetPositiveWords returns a copy of the positive words lexicon.
func GetPositiveWords() map[string]float64 {
	result := make(map[string]float64, len(positiveWords))
	for k, v := range positiveWords {
		result[k] = v
	}
	return result
}

// GetNegativeWords returns a copy of the negative words lexicon.
func GetNegativeWords() map[string]float64 {
	result := make(map[string]float64, len(negativeWords))
	for k, v := range negativeWords {
		result[k] = v
	}
	return result
}

// GetEmoticons returns a copy of the emoticons lexicon.
func GetEmoticons() map[string]float64 {
	result := make(map[string]float64, len(emoticons))
	for k, v := range emoticons {
		result[k] = v
	}
	return result
}
