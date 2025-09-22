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

		// Demonstrate new pagination and tree traversal features
		fmt.Println("\n=== PAGINATION & TREE TRAVERSAL DEMOS ===")

		// 1. Demonstrate PostIterator for pagination
		fmt.Println("\n1. Using PostIterator to paginate through posts:")
		iterator := client.NewHotIterator(ctx, "golang").WithLimit(10)
		postCount := 0
		for iterator.HasNext() && postCount < 15 {
			post, err := iterator.Next()
			if err != nil {
				log.Printf("Iterator error: %v", err)
				break
			}
			postCount++
			fmt.Printf("   %d. %s (score: %d)\n", postCount, post.Data.Title, post.Data.Score)
		}

		// 2. Demonstrate GetMoreComments for loading truncated comments
		if len(comments.MoreIDs) > 0 {
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
					if comment.Data != nil {
						fmt.Printf("   - %s: %.80s...\n", comment.Data.Author, comment.Data.Body)
					}
				}
			}
		}

		// 3. Demonstrate CommentTree utilities
		fmt.Println("\n3. Using CommentTree utilities:")
		tree := graw.NewCommentTree(comments.Comments)

		// Count total comments including nested replies
		totalComments := tree.Count()
		fmt.Printf("   Total comments in tree: %d\n", totalComments)

		// Find highly scored comments
		highScored := tree.GetScoreRange(10, 99999)
		fmt.Printf("   Comments with score >= 10: %d\n", len(highScored))

		// Get all comments by a specific author (if any)
		if len(comments.Comments) > 0 && comments.Comments[0].Data != nil {
			authorName := comments.Comments[0].Data.Author
			byAuthor := tree.GetByAuthor(authorName)
			fmt.Printf("   Comments by %s: %d\n", authorName, len(byAuthor))
		}

		// Find gilded comments
		gilded := tree.GetGilded()
		fmt.Printf("   Gilded comments: %d\n", len(gilded))

		// Get tree depth
		depth := tree.GetDepth()
		fmt.Printf("   Max comment tree depth: %d\n", depth)

		// 4. Demonstrate CommentIterator for traversal
		fmt.Println("\n4. Using CommentIterator for traversal:")
		commentIter := graw.NewCommentIterator(comments.Comments, &graw.TraversalOptions{
			MaxDepth:      3,
			MinScore:      0,
			IterativeMode: true,
			Order:         graw.DepthFirst,
		})

		traversedCount := 0
		for commentIter.HasNext() && traversedCount < 10 {
			comment, err := commentIter.Next()
			if err != nil {
				break
			}
			traversedCount++
			if comment.Data != nil {
				fmt.Printf("   [Depth-first #%d] %s (score: %d)\n",
					traversedCount, comment.Data.Author, comment.Data.Score)
			}
		}

		// 5. Demonstrate batch comment loading for multiple posts
		if len(hotPosts.Posts) >= 3 {
			fmt.Println("\n5. Batch loading comments for multiple posts:")

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
					if result != nil && result.Post != nil {
						fmt.Printf("   Post %d: %s - %d comments loaded\n",
							i+1, result.Post.Data.Title, len(result.Comments))
					}
				}
			}
		}

		// 6. Demonstrate collecting multiple pages of posts
		fmt.Println("\n6. Collecting multiple pages of posts:")
		collector := client.NewNewIterator(ctx, "golang").WithLimit(25)
		allNewPosts, err := collector.Collect(50) // Collect up to 50 posts
		if err != nil {
			log.Printf("Failed to collect posts: %v", err)
		} else {
			fmt.Printf("   Collected %d new posts from r/golang\n", len(allNewPosts))

			// Show score distribution
			var totalScore int
			for _, post := range allNewPosts {
				if post.Data != nil {
					totalScore += post.Data.Score
				}
			}
			if len(allNewPosts) > 0 {
				avgScore := totalScore / len(allNewPosts)
				fmt.Printf("   Average score: %d\n", avgScore)
			}
		}
	}
}
