package sentiment

import (
	"context"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// BenchmarkAnalyzePost benchmarks the AnalyzePost method.
func BenchmarkAnalyzePost(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	// Realistic Reddit post with varied content
	post := &types.Post{
		Title:    "This is an amazing post that I absolutely love! It's fantastic and wonderful.",
		SelfText: "I spent weeks working on this project and I'm really proud of the results. The quality is excellent and the performance is outstanding. Everyone who has seen it thinks it's incredible. I would highly recommend checking it out. This is truly one of my best works ever!",
		Author:   "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzePost(ctx, post)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzePostNegative benchmarks AnalyzePost with negative content.
func BenchmarkAnalyzePostNegative(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	post := &types.Post{
		Title:    "This is a terrible and awful post that I absolutely hate!",
		SelfText: "I can't believe how horrible this is. The quality is horrible and the experience was miserable. This is the worst thing I've ever seen. I would never recommend this to anyone. This is absolutely pathetic and disgusting.",
		Author:   "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzePost(ctx, post)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzePostNeutral benchmarks AnalyzePost with neutral content.
func BenchmarkAnalyzePostNeutral(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	post := &types.Post{
		Title:    "A post about the weather and other things",
		SelfText: "This is a post about general information. The weather today was clear. I went to the store and bought some items. Nothing particularly good or bad happened. Just a regular day.",
		Author:   "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzePost(ctx, post)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzePostLong benchmarks AnalyzePost with very long content.
func BenchmarkAnalyzePostLong(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	// Generate a long post with realistic content
	longBody := `
	This is an extremely long post that contains detailed information about various topics.
	The quality of the content is excellent and comprehensive. I've spent considerable time
	researching and writing this post. The information presented is accurate and well-sourced.

	In the first section, I discuss the historical context and background information.
	This section provides important context for understanding the topic. The research involved
	reading numerous sources and synthesizing the information into a coherent narrative.

	The second section covers the current state of affairs. This is a critical analysis that
	examines the existing conditions and trends. The analysis is thorough and considers multiple
	perspectives. The conclusions drawn are supported by evidence.

	The third section discusses future implications and possibilities. This forward-looking
	analysis considers potential outcomes and their likelihood. The reasoning is sound and based
	on logical extrapolation from current trends.

	Finally, I provide recommendations and suggestions for further reading. These recommendations
	are based on my research and professional experience. I believe they will be valuable to anyone
	interested in this topic.

	This has been an amazing journey of discovery and learning. I'm pleased with the results and
	hope you find this post informative and helpful. The insights provided should enhance your
	understanding of the subject matter. Thank you for taking the time to read this detailed analysis.
	`

	post := &types.Post{
		Title:    "A Comprehensive Guide to Understanding Complex Topics",
		SelfText: longBody,
		Author:   "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzePost(ctx, post)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeComment benchmarks the AnalyzeComment method.
func BenchmarkAnalyzeComment(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body:   "This is a great comment! I really love this perspective. It's amazing and wonderful!",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeCommentNegative benchmarks AnalyzeComment with negative content.
func BenchmarkAnalyzeCommentNegative(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body:   "This is a terrible comment. I hate this perspective. It's horrible and awful!",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeCommentWithEmojis benchmarks AnalyzeComment with emoji emoticons.
func BenchmarkAnalyzeCommentWithEmojis(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body:   "This is absolutely amazing! 😊 I love it! 😍 This is the best! ❤️",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeCommentWithCapsAndPunctuation benchmarks with emphasis markers.
func BenchmarkAnalyzeCommentWithCapsAndPunctuation(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body:   "THIS IS ABSOLUTELY AMAZING!!! I LOVE IT!!! THIS IS FANTASTIC!!!",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeCommentLong benchmarks AnalyzeComment with long content.
func BenchmarkAnalyzeCommentLong(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body: `This is a detailed and thoughtful comment that provides valuable insights into the discussion.
		I've given this considerable thought and I believe the following points are important:

		First, the original post makes some excellent points that deserve recognition. The analysis is thorough
		and well-reasoned. I agree with the main conclusions and appreciate the careful research.

		Second, I think it's worth noting that there are some nuances that could be explored further. While the
		basic argument is sound, there are some edge cases that might complicate the picture.

		Finally, I want to emphasize that this is a complex topic and there's no single right answer. However,
		the approach taken in the original post is certainly reasonable and well-founded.

		Overall, I found this comment thread to be enlightening and I appreciate the diverse perspectives shared.
		The quality of discussion here is outstanding and I'm impressed by the thoughtfulness of the community.`,
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeCommentNeutral benchmarks AnalyzeComment with neutral content.
func BenchmarkAnalyzeCommentNeutral(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body:   "I agree with your point. I have seen similar situations before. The data supports this conclusion.",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeCommentShort benchmarks AnalyzeComment with short content.
func BenchmarkAnalyzeCommentShort(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body:   "Great!",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkNewAnalyzer benchmarks Analyzer creation.
func BenchmarkNewAnalyzer(b *testing.B) {
	b.Run("with nil config", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewAnalyzer(nil)
		}
	})

	b.Run("with custom config", func(b *testing.B) {
		config := &Config{
			MinWordCount:    5,
			EnableEmoticons: true,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewAnalyzer(config)
		}
	})
}

// BenchmarkAnalyzeMixedSentiment benchmarks posts/comments with mixed sentiment.
func BenchmarkAnalyzeMixedSentiment(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body: `I really like some aspects of your argument, but I find other parts problematic.
		The good points you make are valid and well-supported. However, I think you're missing
		some important context. Overall, it's a decent effort but needs refinement.`,
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeNegatedSentiment benchmarks content with negation.
func BenchmarkAnalyzeNegatedSentiment(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	comment := &types.Comment{
		Body: `I don't think this is good. It's not amazing or wonderful. I can't say I'm impressed.
		This isn't what I was hoping for. The quality isn't excellent. I wouldn't recommend it.`,
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzeDeletedAuthor benchmarks posts with deleted authors.
func BenchmarkAnalyzeDeletedAuthor(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	post := &types.Post{
		Title:    "This is amazing content",
		SelfText: "I love this! It's excellent!",
		Author:   "[deleted]",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzePost(ctx, post)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkMultipleAnalyzers benchmarks creating and using multiple analyzers.
func BenchmarkMultipleAnalyzers(b *testing.B) {
	ctx := context.Background()
	comment := &types.Comment{
		Body:   "This is a great comment! Really amazing!",
		Author: "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer := NewAnalyzer(nil)
		_, err := analyzer.AnalyzeComment(ctx, comment)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkAnalyzePostTitleAndBody benchmarks the separate title and body analysis.
func BenchmarkAnalyzePostTitleAndBody(b *testing.B) {
	analyzer := NewAnalyzer(nil)
	ctx := context.Background()

	post := &types.Post{
		Title:    "This title is great and amazing!",
		SelfText: "The body discusses wonderful and excellent topics in detail. The content is really fantastic!",
		Author:   "testuser",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := analyzer.AnalyzePost(ctx, post)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
		// Verify that both title and body scores are calculated
		_ = result.TitleScore
		_ = result.BodyScore
	}
}
