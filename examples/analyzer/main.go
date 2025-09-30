// Package main demonstrates analyzing Reddit comments for insights.
// This example fetches comments from popular posts and provides statistics
// about discussion activity, author participation, and comment structure.
//
// Environment Variables Required:
//   - REDDIT_CLIENT_ID: Your Reddit app's client ID
//   - REDDIT_CLIENT_SECRET: Your Reddit app's client secret
//
// Usage:
//
//	export REDDIT_CLIENT_ID="your_client_id"
//	export REDDIT_CLIENT_SECRET="your_client_secret"
//	go run ./examples/analyzer/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

const (
	targetSubreddit = "golang"
	postsToAnalyze  = 5
	commentsPerPost = 100
)

// CommentStats holds statistical information about comments
type CommentStats struct {
	TotalComments   int
	TotalScore      int
	AverageScore    float64
	MaxScore        int
	MinScore        int
	UniqueAuthors   int
	AuthorActivity  map[string]int
	TopAuthors      []AuthorStat
	DeletedComments int
}

// AuthorStat represents an author's activity statistics
type AuthorStat struct {
	Author       string
	CommentCount int
	TotalScore   int
}

func main() {
	// Get credentials from environment
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET environment variables are required")
	}

	// Create logger (suppress debug logs for cleaner output)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Create client configuration
	config := &graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "comment-analyzer/1.0 (analysis example)",
		Logger:       logger,
	}

	// Create the client
	ctx := context.Background()
	client, err := graw.NewClientWithContext(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Printf("Reddit Comment Analyzer\n")
	fmt.Printf("========================\n\n")

	// Get hot posts
	fmt.Printf("Fetching hot posts from r/%s...\n", targetSubreddit)
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit:  targetSubreddit,
		Pagination: types.Pagination{Limit: postsToAnalyze},
	})
	if err != nil {
		log.Fatalf("Failed to fetch posts: %v", err)
	}

	if len(postsResp.Posts) == 0 {
		log.Fatal("No posts found")
	}

	fmt.Printf("Analyzing %d posts...\n\n", len(postsResp.Posts))

	// Analyze each post
	for i, post := range postsResp.Posts {
		if post == nil {
			continue
		}

		fmt.Printf("=== POST %d/%d ===\n", i+1, len(postsResp.Posts))
		analyzePost(ctx, client, post)
		fmt.Println()
	}
}

// analyzePost fetches and analyzes comments for a single post
func analyzePost(ctx context.Context, client *graw.Client, post *types.Post) {
	fmt.Printf("Title: %s\n", post.Title)
	fmt.Printf("Author: u/%s\n", post.Author)
	fmt.Printf("Score: %d | Comments: %d\n", post.Score, post.NumComments)
	fmt.Printf("URL: https://reddit.com%s\n\n", post.Permalink)

	// Fetch comments
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit:  strings.Split(post.Subreddit, "/")[0],
		PostID:     post.ID,
		Pagination: types.Pagination{Limit: commentsPerPost},
	})
	if err != nil {
		log.Printf("Failed to fetch comments: %v", err)
		return
	}

	if len(commentsResp.Comments) == 0 {
		fmt.Println("No comments found.")
		return
	}

	// Calculate statistics
	stats := calculateStats(commentsResp.Comments)

	// Display statistics
	displayStats(stats)

	// Load more comments if truncated
	if len(commentsResp.MoreIDs) > 0 {
		fmt.Printf("\nNote: %d additional comments available (truncated)\n", len(commentsResp.MoreIDs))

		// Optionally load a sample of more comments
		if len(commentsResp.MoreIDs) > 0 {
			moreToLoad := commentsResp.MoreIDs
			if len(moreToLoad) > 20 {
				moreToLoad = moreToLoad[:20]
			}

			moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
				LinkID:     post.ID,
				CommentIDs: moreToLoad,
				Sort:       "confidence",
			})
			if err == nil && len(moreComments) > 0 {
				fmt.Printf("Loaded %d additional comments for deeper analysis\n", len(moreComments))
			}
		}
	}
}

// calculateStats computes statistics from a slice of comments
func calculateStats(comments []*types.Comment) CommentStats {
	stats := CommentStats{
		TotalComments:  len(comments),
		AuthorActivity: make(map[string]int),
		MinScore:       int(^uint(0) >> 1), // Max int
	}

	authorScores := make(map[string]int)

	for _, comment := range comments {
		if comment == nil {
			continue
		}

		// Score statistics
		stats.TotalScore += comment.Score
		if comment.Score > stats.MaxScore {
			stats.MaxScore = comment.Score
		}
		if comment.Score < stats.MinScore {
			stats.MinScore = comment.Score
		}

		// Author statistics
		author := comment.Author
		if author == "[deleted]" {
			stats.DeletedComments++
		} else {
			stats.AuthorActivity[author]++
			authorScores[author] += comment.Score
		}
	}

	// Calculate average
	if stats.TotalComments > 0 {
		stats.AverageScore = float64(stats.TotalScore) / float64(stats.TotalComments)
	}

	// Count unique authors
	stats.UniqueAuthors = len(stats.AuthorActivity)

	// Find top authors
	for author, count := range stats.AuthorActivity {
		stats.TopAuthors = append(stats.TopAuthors, AuthorStat{
			Author:       author,
			CommentCount: count,
			TotalScore:   authorScores[author],
		})
	}

	// Sort by comment count
	sort.Slice(stats.TopAuthors, func(i, j int) bool {
		return stats.TopAuthors[i].CommentCount > stats.TopAuthors[j].CommentCount
	})

	// Keep top 5
	if len(stats.TopAuthors) > 5 {
		stats.TopAuthors = stats.TopAuthors[:5]
	}

	return stats
}

// displayStats prints formatted statistics
func displayStats(stats CommentStats) {
	fmt.Println("Comment Analysis:")
	fmt.Println("-----------------")
	fmt.Printf("Total Comments: %d\n", stats.TotalComments)
	fmt.Printf("Unique Authors: %d\n", stats.UniqueAuthors)
	fmt.Printf("Deleted Comments: %d (%.1f%%)\n",
		stats.DeletedComments,
		float64(stats.DeletedComments)/float64(stats.TotalComments)*100)

	fmt.Println("\nScore Statistics:")
	fmt.Printf("  Total Score: %d\n", stats.TotalScore)
	fmt.Printf("  Average Score: %.2f\n", stats.AverageScore)
	fmt.Printf("  Highest Score: %d\n", stats.MaxScore)
	fmt.Printf("  Lowest Score: %d\n", stats.MinScore)

	fmt.Println("\nMost Active Commenters:")
	for i, author := range stats.TopAuthors {
		fmt.Printf("  %d. u/%s - %d comments (total score: %d)\n",
			i+1, author.Author, author.CommentCount, author.TotalScore)
	}

	// Thread engagement metrics
	avgCommentsPerAuthor := float64(stats.TotalComments) / float64(stats.UniqueAuthors)
	fmt.Printf("\nEngagement Metrics:\n")
	fmt.Printf("  Avg comments per author: %.2f\n", avgCommentsPerAuthor)

	if avgCommentsPerAuthor > 3.0 {
		fmt.Printf("  Assessment: Highly engaged discussion\n")
	} else if avgCommentsPerAuthor > 1.5 {
		fmt.Printf("  Assessment: Moderate engagement\n")
	} else {
		fmt.Printf("  Assessment: Many one-time commenters\n")
	}
}
