// Package sentiment provides sentiment analysis for Reddit content.
//
// This package analyzes the sentiment of Reddit posts and comments using a
// keyword-based approach. It scores content on a scale from VeryNegative (-2)
// to VeryPositive (+2), providing detailed sentiment information including
// overall sentiment classification, numeric score, and confidence metrics.
//
// The Analyzer type is the primary entry point. Create an instance with
// NewAnalyzer and provide optional configuration. If no config is provided,
// sensible defaults will be used.
//
// Example usage:
//
//	// Create analyzer with default configuration
//	analyzer := sentiment.NewAnalyzer(nil)
//
//	// Analyze a post
//	postSentiment, err := analyzer.AnalyzePost(ctx, post)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Post sentiment: %s (score: %.2f)\n",
//	    postSentiment.Sentiment, postSentiment.Score)
//
//	// Analyze a comment
//	commentSentiment, err := analyzer.AnalyzeComment(ctx, comment)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Comment sentiment: %s\n", commentSentiment.Sentiment)
//
// The sentiment analysis works with types from pkg/types, making it easy
// to integrate into workflows that already use the main Reddit API client.
package sentiment
