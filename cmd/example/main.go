package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
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
		fmt.Println("\nHot posts from r/golang:")
		for i, post := range hotPosts.Posts {
			fmt.Printf("%d. %s (score: %d, comments: %d)\n",
				i+1, post.Title, post.Score, post.NumComments)
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
		fmt.Printf("\nSubreddit: r/%s\n", subredditInfo.DisplayName)
		fmt.Printf("Subscribers: %d\n", subredditInfo.Subscribers)
		fmt.Printf("Description: %.100s...\n", subredditInfo.PublicDescription)
	}

	// Get comments for a post (if we have posts)
	var comments *types.CommentsResponse
	if len(hotPosts.Posts) > 0 {
		firstPost := hotPosts.Posts[0]
		// Use post ID directly
		postID := firstPost.ID
		comments, err = client.GetComments(ctx, "golang", postID, &graw.ListingOptions{Limit: 5})
		if err != nil {
			log.Printf("Failed to get comments: %v", err)
		} else if comments == nil || comments.Post == nil {
			log.Printf("No post data returned with comments")
		} else {
			fmt.Printf("\nComments for post: %s\n", comments.Post.Title)
			for i, comment := range comments.Comments {
				if i >= 3 { // Show only first 3 comments
					break
				}
				fmt.Printf("  - %s: %.100s...\n", comment.Author, comment.Body)
			}
			if len(comments.MoreIDs) > 0 {
				fmt.Printf("  (%d more comments available)\n", len(comments.MoreIDs))
			}
		}

		// Demonstrate pagination and tree traversal features
		fmt.Println("\n=== PAGINATION & TREE TRAVERSAL DEMOS ===")

		// 1. Demonstrate simple pagination using After token
		fmt.Println("\n1. Simple pagination through posts:")
		after := ""
		totalPosts := 0
		for page := 1; page <= 3; page++ { // Get 3 pages
			resp, err := client.GetHot(ctx, "golang", &graw.ListingOptions{
				Limit: 5,
				After: after,
			})
			if err != nil {
				log.Printf("Failed to get page %d: %v", page, err)
				break
			}
			fmt.Printf("   Page %d: %d posts\n", page, len(resp.Posts))
			totalPosts += len(resp.Posts)

			// Show first 2 titles from each page
			for i, post := range resp.Posts {
				if i < 2 && post != nil {
					fmt.Printf("     - %.60s... (score: %d)\n", post.Title, post.Score)
				}
			}

			after = resp.After
			if after == "" {
				fmt.Println("   No more pages available")
				break
			}
		}
		fmt.Printf("   Total posts fetched: %d\n", totalPosts)

		// 2. Demonstrate GetMoreComments for loading truncated comments
		if comments != nil && len(comments.MoreIDs) > 0 {
			fmt.Printf("\n2. Loading more comments (found %d truncated):\n", len(comments.MoreIDs))

			// Load up to 10 more comments
			moreToLoad := comments.MoreIDs
			if len(moreToLoad) > 10 {
				moreToLoad = moreToLoad[:10]
			}

			moreComments, err := client.GetMoreComments(ctx, firstPost.ID, moreToLoad, &graw.MoreCommentsOptions{
				Sort:  "best",
				Limit: 10,
			})
			if err != nil {
				log.Printf("Failed to load more comments: %v", err)
			} else {
				fmt.Printf("   Loaded %d additional comments:\n", len(moreComments))
				for i, comment := range moreComments {
					if i >= 3 {
						break
					}
					if comment != nil {
						fmt.Printf("   - %s: %.80s...\n", comment.Author, comment.Body)
					}
				}
			}
		}

		// 3. Demonstrate batch comment loading for multiple posts
		if len(hotPosts.Posts) >= 3 {
			fmt.Println("\n3. Batch loading comments for multiple posts:")

			requests := []graw.CommentRequest{
				{Subreddit: "golang", PostID: hotPosts.Posts[0].ID, Options: &graw.ListingOptions{Limit: 5}},
				{Subreddit: "golang", PostID: hotPosts.Posts[1].ID, Options: &graw.ListingOptions{Limit: 5}},
				{Subreddit: "golang", PostID: hotPosts.Posts[2].ID, Options: &graw.ListingOptions{Limit: 5}},
			}

			batchResults, err := client.GetCommentsMultiple(ctx, requests)
			if err != nil {
				log.Printf("Batch loading error: %v", err)
			} else {
				for i, result := range batchResults {
					if result != nil {
						if result.Post != nil {
							fmt.Printf("   Post %d: %.50s... - %d comments loaded\n",
								i+1, result.Post.Title, len(result.Comments))
						} else {
							fmt.Printf("   Post %d: (post data not included) - %d comments loaded\n",
								i+1, len(result.Comments))
						}
					}
				}
			}
		}
	}
}
