//go:build integration
// +build integration

package graw

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Integration tests require real Reddit API credentials.
// Set these environment variables:
//   - REDDIT_CLIENT_ID: Your Reddit application client ID
//   - REDDIT_CLIENT_SECRET: Your Reddit application client secret
//   - REDDIT_USERNAME: (optional) Your Reddit username for user auth
//   - REDDIT_PASSWORD: (optional) Your Reddit password for user auth
//
// Run with: go test -tags=integration -v

func getTestClient(t *testing.T) *Client {
	t.Helper()

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	username := os.Getenv("REDDIT_USERNAME")
	password := os.Getenv("REDDIT_PASSWORD")

	if clientID == "" || clientSecret == ""  {
		t.Skip("Skipping integration test: REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET must be set")
	}

	config := &Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Username:     username,
		Password:     password,
		UserAgent:    "go-reddit-api-wrapper:integration-tests:v1.0.0",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	return client
}

func TestIntegration_GetHot(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Test getting hot posts from r/golang
	resp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	if err != nil {
		t.Fatalf("GetHot failed: %v", err)
	}

	if len(resp.Posts) == 0 {
		t.Error("Expected at least 1 post, got 0")
	}

	// Verify post structure
	for i, post := range resp.Posts {
		if post.ID == "" {
			t.Errorf("Post %d has empty ID", i)
		}
		if post.Title == "" {
			t.Errorf("Post %d has empty title", i)
		}
		if post.Subreddit == "" {
			t.Errorf("Post %d has empty subreddit", i)
		}
	}
}

func TestIntegration_GetComments(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// First, get a post to fetch comments for
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 1,
		},
	})

	if err != nil {
		t.Fatalf("GetHot failed: %v", err)
	}

	if len(postsResp.Posts) == 0 {
		t.Skip("No posts available to test comments")
	}

	post := postsResp.Posts[0]

	// Get comments for the post
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "golang",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 10,
		},
	})

	if err != nil {
		t.Fatalf("GetComments failed: %v", err)
	}

	if commentsResp.Post == nil {
		t.Error("Expected post in response, got nil")
	}

	// Verify comment tree structure
	for i, comment := range commentsResp.Comments {
		if comment.ID == "" {
			t.Errorf("Comment %d has empty ID", i)
		}
		if comment.Author == "" {
			t.Errorf("Comment %d has empty author", i)
		}

		// Verify proper tree structure - replies should only contain direct children
		verifyCommentTreeStructure(t, comment, 0)
	}
}

// verifyCommentTreeStructure recursively verifies that the comment tree is properly structured
func verifyCommentTreeStructure(t *testing.T, comment *types.Comment, depth int) {
	t.Helper()

	if depth > 10 {
		// Sanity check to prevent infinite recursion
		t.Error("Comment tree depth exceeds 10 levels")
		return
	}

	// Each reply should be a direct child, not a grandchild or deeper
	for i, reply := range comment.Replies {
		if reply.ID == "" {
			t.Errorf("Reply %d at depth %d has empty ID", i, depth)
		}
		if reply.ParentID == "" {
			t.Errorf("Reply %d at depth %d has empty ParentID", i, depth)
		}

		// Recursively verify children
		verifyCommentTreeStructure(t, reply, depth+1)
	}
}

func TestIntegration_GetMoreComments(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Get a post with comments
	postsResp, err := client.GetHot(ctx, &types.PostsRequest{
		Subreddit: "golang",
		Pagination: types.Pagination{
			Limit: 1,
		},
	})

	if err != nil {
		t.Fatalf("GetHot failed: %v", err)
	}

	if len(postsResp.Posts) == 0 {
		t.Skip("No posts available to test more comments")
	}

	post := postsResp.Posts[0]

	// Get initial comments
	commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
		Subreddit: "golang",
		PostID:    post.ID,
		Pagination: types.Pagination{
			Limit: 5,
		},
	})

	if err != nil {
		t.Fatalf("GetComments failed: %v", err)
	}

	// If there are more comment IDs, try to fetch them
	if len(commentsResp.MoreIDs) == 0 {
		t.Skip("No more comment IDs available to test")
	}

	// Test with LimitChildren parameter variations
	testCases := []struct {
		name          string
		limitChildren bool
		maxIDs        int
	}{
		{"without limit", false, 10},
		{"with limit", true, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Take a subset of more IDs
			moreIDs := commentsResp.MoreIDs
			if len(moreIDs) > tc.maxIDs {
				moreIDs = moreIDs[:tc.maxIDs]
			}

			moreComments, err := client.GetMoreComments(ctx, &types.MoreCommentsRequest{
				LinkID:        post.ID,
				CommentIDs:    moreIDs,
				Sort:          "confidence",
				LimitChildren: tc.limitChildren,
			})

			if err != nil {
				t.Fatalf("GetMoreComments failed: %v", err)
			}

			t.Logf("Fetched %d more comments with LimitChildren=%v", len(moreComments), tc.limitChildren)

			// Verify returned comments
			for i, comment := range moreComments {
				if comment.ID == "" {
					t.Errorf("More comment %d has empty ID", i)
				}
			}
		})
	}
}

func TestIntegration_GetSubreddit(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	subreddit, err := client.GetSubreddit(ctx, "golang")
	if err != nil {
		t.Fatalf("GetSubreddit failed: %v", err)
	}

	if subreddit.DisplayName != "golang" {
		t.Errorf("Expected DisplayName 'golang', got '%s'", subreddit.DisplayName)
	}

	if subreddit.Subscribers == 0 {
		t.Error("Expected non-zero subscribers")
	}
}

func TestIntegration_RateLimiting(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Make multiple rapid requests to test rate limiting
	for i := 0; i < 3; i++ {
		start := time.Now()

		_, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 1,
			},
		})

		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}

		elapsed := time.Since(start)
		t.Logf("Request %d completed in %v", i, elapsed)
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	t.Run("invalid subreddit", func(t *testing.T) {
		_, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "thisisnotarealsubredditname123456789",
			Pagination: types.Pagination{
				Limit: 1,
			},
		})

		// This may or may not error depending on Reddit's behavior
		// Some invalid subreddits return empty results, others error
		t.Logf("Result for invalid subreddit: %v", err)
	})

	t.Run("invalid post ID", func(t *testing.T) {
		_, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    "invalidpostid123",
			Pagination: types.Pagination{
				Limit: 1,
			},
		})

		if err == nil {
			t.Error("Expected error for invalid post ID, got nil")
		}
	})
}
