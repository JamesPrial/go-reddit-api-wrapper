package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
)

func main() {
	// Get credentials from environment variables
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	username := os.Getenv("REDDIT_USERNAME")
	password := os.Getenv("REDDIT_PASSWORD")

	if clientID == "" || clientSecret == "" {
		log.Fatal("REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET environment variables are required")
	}

	// Route structured logs to stdout; adjust the level as needed.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create client configuration
	config := &graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Username:     username, // Optional: for user-authenticated requests
		Password:     password, // Optional: for user-authenticated requests
		UserAgent:    "example-bot/1.0 by YourUsername",
		Logger:       logger,
	}

	// Create the client
	client, err := graw.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to Reddit (authenticate)
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to Reddit: %v", err)
	}

	fmt.Println("Successfully connected to Reddit!")

	// If we have user credentials, get user info
	if username != "" && password != "" {
		userInfo, err := client.Me(ctx)
		if err != nil {
			log.Printf("Failed to get user info: %v", err)
		} else {
			fmt.Printf("Authenticated as user: %s\n", userInfo.User.Name)
		}
	}

	// Get hot posts from r/golang
	hotPosts, err := client.GetHot(ctx, "golang", &graw.ListingOptions{Limit: 5})
	if err != nil {
		log.Printf("Failed to get hot posts: %v", err)
	} else {
		fmt.Println("\nHot posts from r/golang:")
		for i, post := range hotPosts.Posts {
			fmt.Printf("%d. %s (score: %d, comments: %d)\n",
				i+1, post.Data.Title, post.Data.Score, post.Data.NumComments)
		}
		if hotPosts.After != "" {
			fmt.Printf("Next page: %s\n", hotPosts.After)
		}
	}

	// Get subreddit info
	subredditInfo, err := client.GetSubreddit(ctx, "golang")
	if err != nil {
		log.Printf("Failed to get subreddit info: %v", err)
	} else {
		fmt.Printf("\nSubreddit: r/%s\n", subredditInfo.Subreddit.DisplayName)
		fmt.Printf("Subscribers: %d\n", subredditInfo.Subreddit.Subscribers)
		fmt.Printf("Description: %.100s...\n", subredditInfo.Subreddit.PublicDescription)
	}

	// Get comments for a post (if we have posts)
	if len(hotPosts.Posts) > 0 {
		firstPost := hotPosts.Posts[0]
		// Use post ID directly
		postID := firstPost.ID
		comments, err := client.GetComments(ctx, "golang", postID, &graw.ListingOptions{Limit: 5})
		if err != nil {
			log.Printf("Failed to get comments: %v", err)
		} else {
			fmt.Printf("\nComments for post: %s\n", comments.Post.Data.Title)
			for i, comment := range comments.Comments {
				if i >= 3 { // Show only first 3 comments
					break
				}
				fmt.Printf("  - %s: %.100s...\n", comment.Data.Author, comment.Data.Body)
			}
			if len(comments.MoreIDs) > 0 {
				fmt.Printf("  (%d more comments available)\n", len(comments.MoreIDs))
			}
		}
	}
}
