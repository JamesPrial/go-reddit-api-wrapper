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
			fmt.Printf("Authenticated as user: %s\n", userInfo.Name)
		}
	}

	// Get hot posts from r/golang
	hotPosts, err := client.GetHot(ctx, "golang", &graw.ListingOptions{Limit: 5})
	if err != nil {
		log.Printf("Failed to get hot posts: %v", err)
	} else {
		fmt.Println("Hot posts from r/golang:")
		fmt.Printf("Got listing with kind: %s\n", hotPosts.Kind)
		// Note: You'd need to unmarshal the Data field to get the actual posts
		// This is just a basic example showing the API structure
	}

	// Get subreddit info
	subredditInfo, err := client.GetSubreddit(ctx, "golang")
	if err != nil {
		log.Printf("Failed to get subreddit info: %v", err)
	} else {
		fmt.Printf("Subreddit info: %s (kind: %s)\n", subredditInfo.Name, subredditInfo.Kind)
	}
}
