package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
)

// This is a simple example showing how to use the database layer.
// To run: go run ./cmd/api/example.go
func main() {
	// Initialize database
	database, err := db.InitDB(db.Config{
		Path:        "reddit_tracker.db",
		EnableDebug: true,
	})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Get underlying SQL database for connection management
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("Failed to get SQL database: %v", err)
	}
	defer sqlDB.Close()

	// Create repository
	repo := db.NewRepository(database)

	// Create a subreddit
	ctx := context.Background()
	subreddit := &db.Subreddit{
		Fullname:    "t5_2qh33",
		Name:        "golang",
		Description: "The Go programming language",
		Subscribers: 250000,
	}

	if err := repo.CreateSubreddit(ctx, subreddit); err != nil {
		log.Fatalf("Failed to create subreddit: %v", err)
	}
	fmt.Printf("Created subreddit: %s (ID: %d)\n", subreddit.Name, subreddit.ID)

	// Create a post
	post := &db.Post{
		Fullname:    "t3_abc123",
		SubredditID: subreddit.ID,
		Title:       "Why Go is awesome",
		Author:      "gopher",
		Score:       42,
		NumComments: 10,
		URL:         "https://reddit.com/r/golang/comments/abc123",
		Selftext:    "Go is great because of its simplicity and concurrency model.",
		CreatedUTC:  time.Now().UTC(),
	}

	if err := repo.CreatePost(ctx, post); err != nil {
		log.Fatalf("Failed to create post: %v", err)
	}
	fmt.Printf("Created post: %s (ID: %d)\n", post.Title, post.ID)

	// Create comments
	comments := []db.Comment{
		{
			Fullname:   "t1_xyz001",
			PostID:     post.ID,
			Author:     "user1",
			Body:       "I totally agree!",
			Score:      5,
			CreatedUTC: time.Now().UTC(),
		},
		{
			Fullname:   "t1_xyz002",
			PostID:     post.ID,
			Author:     "user2",
			Body:       "Great point about concurrency.",
			Score:      3,
			CreatedUTC: time.Now().UTC().Add(1 * time.Minute),
		},
	}

	if err := repo.CreateComments(ctx, comments); err != nil {
		log.Fatalf("Failed to create comments: %v", err)
	}
	fmt.Printf("Created %d comments\n", len(comments))

	// List all subreddits
	subs, err := repo.ListSubreddits(ctx)
	if err != nil {
		log.Fatalf("Failed to list subreddits: %v", err)
	}
	fmt.Printf("\nSubreddits in database:\n")
	for _, s := range subs {
		fmt.Printf("  - %s (%s): %d subscribers\n", s.Name, s.Fullname, s.Subscribers)
	}

	// List posts for a subreddit
	posts, err := repo.ListPosts(ctx, "golang", 10, 0)
	if err != nil {
		log.Fatalf("Failed to list posts: %v", err)
	}
	fmt.Printf("\nPosts in r/golang:\n")
	for _, p := range posts {
		fmt.Printf("  - %s (score: %d, comments: %d)\n", p.Title, p.Score, p.NumComments)
	}

	// Get comments for a post
	retrievedComments, err := repo.GetCommentsByPost(ctx, post.Fullname)
	if err != nil {
		log.Fatalf("Failed to get comments: %v", err)
	}
	fmt.Printf("\nComments on post '%s':\n", post.Title)
	for _, c := range retrievedComments {
		fmt.Printf("  - %s: %s (score: %d)\n", c.Author, c.Body, c.Score)
	}

	fmt.Println("\nDatabase operations completed successfully!")
}
