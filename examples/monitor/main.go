// Package main demonstrates how to monitor a subreddit for new posts in real-time.
// This example polls Reddit periodically and reports new posts as they appear.
//
// Environment Variables Required:
//   - REDDIT_CLIENT_ID: Your Reddit app's client ID
//   - REDDIT_CLIENT_SECRET: Your Reddit app's client secret
//
// Usage:
//
//	export REDDIT_CLIENT_ID="your_client_id"
//	export REDDIT_CLIENT_SECRET="your_client_secret"
//	go run ./examples/monitor/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

const (
	// Subreddit to monitor
	targetSubreddit = "golang"
	// Poll interval in seconds
	pollInterval = 30
	// Number of posts to fetch per request
	fetchLimit = 10
)

func main() {
	// Get credentials from environment
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET environment variables are required")
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Create client configuration
	config := &graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "subreddit-monitor/1.0 (monitoring example)",
		Logger:       logger,
	}

	// Create the client
	ctx := context.Background()
	client, err := graw.NewClientWithContext(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Printf("Starting monitor for r/%s (checking every %d seconds)\n", targetSubreddit, pollInterval)
	fmt.Println("Press Ctrl+C to stop")

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down gracefully...")
		cancel()
	}()

	// Start monitoring
	if err := monitorSubreddit(ctx, client, targetSubreddit); err != nil {
		log.Printf("Monitor stopped: %v", err)
	}
}

// monitorSubreddit polls a subreddit for new posts at regular intervals
func monitorSubreddit(ctx context.Context, client *graw.Client, subreddit string) error {
	// Track the last seen post to avoid duplicates
	seenPosts := make(map[string]bool)
	ticker := time.NewTicker(pollInterval * time.Second)
	defer ticker.Stop()

	// Do initial fetch
	if err := fetchAndProcessNewPosts(ctx, client, subreddit, seenPosts); err != nil {
		return fmt.Errorf("initial fetch failed: %w", err)
	}

	// Continue polling
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := fetchAndProcessNewPosts(ctx, client, subreddit, seenPosts); err != nil {
				log.Printf("Error fetching posts: %v", err)
				// Continue monitoring despite errors
			}
		}
	}
}

// fetchAndProcessNewPosts retrieves new posts and processes them
func fetchAndProcessNewPosts(ctx context.Context, client *graw.Client, subreddit string, seenPosts map[string]bool) error {
	resp, err := client.GetNew(ctx, &types.PostsRequest{
		Subreddit:  subreddit,
		Pagination: types.Pagination{Limit: fetchLimit},
	})
	if err != nil {
		return err
	}

	// Process posts in reverse order (oldest first)
	newCount := 0
	for i := len(resp.Posts) - 1; i >= 0; i-- {
		post := resp.Posts[i]
		if post == nil {
			continue
		}

		// Skip if we've already seen this post
		if seenPosts[post.ID] {
			continue
		}

		// Mark as seen
		seenPosts[post.ID] = true
		newCount++

		// Process the new post
		processNewPost(post)
	}

	if newCount > 0 {
		fmt.Printf("[%s] Found %d new post(s)\n", time.Now().Format("15:04:05"), newCount)
	}

	// Clean up old entries to prevent memory growth
	// Keep only the most recent posts
	if len(seenPosts) > 1000 {
		// Reset with current posts
		seenPosts = make(map[string]bool)
		for _, post := range resp.Posts {
			if post != nil {
				seenPosts[post.ID] = true
			}
		}
	}

	return nil
}

// processNewPost handles a newly discovered post
func processNewPost(post *types.Post) {
	timestamp := time.Unix(int64(post.CreatedUTC), 0).Format("15:04:05")

	fmt.Printf("\n[NEW POST] %s\n", timestamp)
	fmt.Printf("  Title: %s\n", post.Title)
	fmt.Printf("  Author: u/%s\n", post.Author)
	fmt.Printf("  URL: https://reddit.com%s\n", post.Permalink)

	if post.IsSelf {
		fmt.Printf("  Type: Self post\n")
		if len(post.SelfText) > 100 {
			fmt.Printf("  Text: %s...\n", post.SelfText[:100])
		} else if post.SelfText != "" {
			fmt.Printf("  Text: %s\n", post.SelfText)
		}
	} else {
		fmt.Printf("  Type: Link post\n")
		fmt.Printf("  Link: %s\n", post.URL)
	}

	fmt.Printf("  Score: %d | Comments: %d\n", post.Score, post.NumComments)

	// Here you could:
	// - Send a notification (email, Slack, Discord, etc.)
	// - Store in a database
	// - Trigger automated analysis
	// - Filter based on keywords or patterns
}