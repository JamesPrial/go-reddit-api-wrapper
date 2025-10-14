package graw

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/validator"
)

// createWorkflowClient creates a Reddit client configured for the given mock server
func createWorkflowClient(t *testing.T, server *testutil.MockServer) *Reddit {
	t.Helper()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	internalClient, err := client.NewClient(httpClient, server.URL(), "test/1.0", nil)
	testutil.AssertNoError(t, err)

	return &Reddit{
		httpClient: internalClient,
		parser:     parse.NewParser(nil),
		validator:  validator.NewValidator(),
		auth:       &mockTokenProvider{token: "test_token"},
	}
}

// TestCompletePostBrowsingWorkflow tests the complete flow from subreddit discovery to post browsing
func TestCompletePostBrowsingWorkflow(t *testing.T) {
	// Setup test data using builders
	subreddit := testutil.NewSubreddit("golang").
		WithID("testsub1").
		WithTitle("The Go Programming Language").
		WithDescription("Go discussions").
		WithSubscribers(500000).
		WithActiveUsers(2500).
		Build()

	// First page posts
	firstPagePosts := make([]*types.Post, 5)
	for i := 0; i < 5; i++ {
		firstPagePosts[i] = testutil.NewPostBuilder().
			WithID("post" + string(rune('a'+i))).
			WithTitle("Test Post " + string(rune('A'+i))).
			WithScore(100 + i*10).
			WithAuthor("user" + string(rune('1'+i))).
			WithSubreddit("golang").
			WithNumComments(5 + i).
			WithCreated(1609459200.0 + float64(i*3600)).
			Build()
	}

	// Second page posts
	secondPagePosts := make([]*types.Post, 3)
	for i := 5; i < 8; i++ {
		secondPagePosts[i-5] = testutil.NewPostBuilder().
			WithID("post" + string(rune('a'+i))).
			WithTitle("Test Post " + string(rune('A'+i))).
			WithScore(100 + i*10).
			WithAuthor("user" + string(rune('1'+i))).
			WithSubreddit("golang").
			WithNumComments(5 + i).
			WithCreated(1609459200.0 + float64(i*3600)).
			Build()
	}

	// Configure mock server
	server := testutil.NewMockServer().
		WithSubreddit("golang", subreddit).
		WithPosts("golang", "hot", firstPagePosts...).
		Start()
	defer server.Close()

	client := createWorkflowClient(t, server)
	ctx := context.Background()

	// Step 1: Get subreddit info
	t.Run("GetSubredditInfo", func(t *testing.T) {
		sub, err := client.GetSubreddit(ctx, "golang")
		testutil.AssertNoError(t, err)

		if sub.DisplayName != "golang" {
			t.Errorf("Expected display name 'golang', got '%s'", sub.DisplayName)
		}

		if sub.Subscribers != 500000 {
			t.Errorf("Expected 500000 subscribers, got %d", sub.Subscribers)
		}

		t.Logf("Successfully retrieved subreddit: %s (%d subscribers)", sub.DisplayName, sub.Subscribers)
	})

	// Step 2: Get first page of hot posts
	t.Run("GetFirstPage", func(t *testing.T) {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{
				Limit: 5,
			},
		})

		testutil.AssertNoError(t, err)
		testutil.AssertPostCount(t, resp, 5)

		// Verify post structure
		for i, post := range resp.Posts {
			expectedTitle := "Test Post " + string(rune('A'+i))
			if post.Title != expectedTitle {
				t.Errorf("Post %d: expected title '%s', got '%s'", i, expectedTitle, post.Title)
			}

			if post.Subreddit != "golang" {
				t.Errorf("Post %d: expected subreddit 'golang', got '%s'", i, post.Subreddit)
			}
		}

		t.Logf("Successfully retrieved first page: %d posts", len(resp.Posts))
	})

	// Note: Pagination with "after" parameter requires more complex server setup
	// The MockServer currently doesn't support pagination tokens, so we test
	// the basic workflow without the second page test
}

// TestCommentTreeNavigationWorkflow tests the complete flow from post to comments to more comments
func TestCommentTreeNavigationWorkflow(t *testing.T) {
	// Setup test data
	post := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Test Post for Comments").
		WithAuthor("testuser").
		WithSubreddit("golang").
		WithScore(100).
		WithNumComments(10).
		Build()

	// Create nested reply
	reply := testutil.NewCommentBuilder().
		WithID("comment2").
		WithBody("This is a reply").
		WithAuthor("user2").
		WithScore(5).
		WithLinkID("t3_post1").
		WithParentID("t1_comment1").
		WithSubreddit("golang").
		Build()

	// Top-level comment with reply
	comment1 := testutil.NewCommentBuilder().
		WithID("comment1").
		WithBody("This is a top-level comment").
		WithAuthor("user1").
		WithScore(10).
		WithParentPost("post1").
		WithSubreddit("golang").
		WithReplies(reply).
		Build()

	// Second top-level comment
	comment3 := testutil.NewCommentBuilder().
		WithID("comment3").
		WithBody("Another top-level comment").
		WithAuthor("user3").
		WithScore(8).
		WithParentPost("post1").
		WithSubreddit("golang").
		Build()

	// Note: MockServer doesn't currently support "more" comments in replies
	// For this test, we'll verify the basic comment tree structure

	server := testutil.NewMockServer().
		WithComments("golang", "post1", post, comment1, comment3).
		Start()
	defer server.Close()

	client := createWorkflowClient(t, server)
	ctx := context.Background()

	// Step 1: Get initial comments
	t.Run("GetInitialComments", func(t *testing.T) {
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    "post1",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		testutil.AssertNoError(t, err)

		if commentsResp.Post == nil {
			t.Fatal("Expected post in response, got nil")
		}

		if commentsResp.Post.Title != "Test Post for Comments" {
			t.Errorf("Expected post title 'Test Post for Comments', got '%s'", commentsResp.Post.Title)
		}

		testutil.AssertCommentCount(t, commentsResp, 2)

		// Verify comment tree structure
		if len(commentsResp.Comments[0].Replies) != 1 {
			t.Errorf("Expected 1 reply to first comment, got %d", len(commentsResp.Comments[0].Replies))
		}

		t.Logf("Successfully retrieved %d comments", len(commentsResp.Comments))
	})

	// Note: MockServer doesn't have a /api/morechildren endpoint yet,
	// so we skip the GetMoreComments test for now
}

// TestSubredditDiscoveryWorkflow tests discovering and exploring subreddits
func TestSubredditDiscoveryWorkflow(t *testing.T) {
	// Setup test data for multiple subreddits
	golangSub := testutil.NewSubreddit("golang").
		WithID("golang123").
		WithTitle("The Go Programming Language").
		WithDescription("Go discussions and news").
		WithSubscribers(500000).
		WithActiveUsers(2500).
		Build()

	rustSub := testutil.NewSubreddit("rust").
		WithID("rust123").
		WithTitle("Rust Programming Language").
		WithDescription("Rust discussions and questions").
		WithSubscribers(300000).
		WithActiveUsers(1800).
		Build()

	golangPost := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Go 1.20 Released").
		WithAuthor("testuser").
		WithSubreddit("golang").
		WithScore(1500).
		Build()

	rustPost := testutil.NewPostBuilder().
		WithID("post2").
		WithTitle("Rust 2023 Roadmap").
		WithAuthor("rustuser").
		WithSubreddit("rust").
		WithScore(800).
		Build()

	server := testutil.NewMockServer().
		WithSubreddit("golang", golangSub).
		WithSubreddit("rust", rustSub).
		WithPosts("golang", "hot", golangPost).
		WithPosts("rust", "hot", rustPost).
		Start()
	defer server.Close()

	client := createWorkflowClient(t, server)
	ctx := context.Background()
	subreddits := []string{"golang", "rust"}

	// Discover each subreddit
	for _, subredditName := range subreddits {
		t.Run("Discover_"+subredditName, func(t *testing.T) {
			// Step 1: Get subreddit info
			subreddit, err := client.GetSubreddit(ctx, subredditName)
			testutil.AssertNoError(t, err)

			if subreddit.DisplayName != subredditName {
				t.Errorf("Expected display name '%s', got '%s'", subredditName, subreddit.DisplayName)
			}

			t.Logf("Discovered subreddit: %s (%d subscribers)", subreddit.DisplayName, subreddit.Subscribers)

			// Step 2: Get hot posts to verify it's active
			resp, err := client.GetHot(ctx, &types.PostsRequest{
				Subreddit: subredditName,
				Pagination: types.Pagination{
					Limit: 5,
				},
			})

			testutil.AssertNoError(t, err)

			if len(resp.Posts) == 0 {
				t.Errorf("Expected at least 1 post in %s, got 0", subredditName)
			}

			// Verify posts belong to the correct subreddit
			for _, post := range resp.Posts {
				if post.Subreddit != subredditName {
					t.Errorf("Expected post from %s, got post from %s", subredditName, post.Subreddit)
				}
			}

			t.Logf("Verified %s is active with %d hot posts", subredditName, len(resp.Posts))
		})
	}
}

// TestUserActivityWorkflow tests user-related workflows
func TestUserActivityWorkflow(t *testing.T) {
	// Setup test data
	account := testutil.NewAccount("testuser").
		WithID("user123").
		WithLinkKarma(5000).
		WithCommentKarma(3000).
		Build()

	userPost1 := testutil.NewPostBuilder().
		WithID("userpost1").
		WithTitle("My Go Project").
		WithAuthor("testuser").
		WithSubreddit("golang").
		WithScore(50).
		Build()

	userPost2 := testutil.NewPostBuilder().
		WithID("userpost2").
		WithTitle("Rust vs Go").
		WithAuthor("testuser").
		WithSubreddit("rust").
		WithScore(25).
		Build()

	// Comments on user's post
	comment := testutil.NewCommentBuilder().
		WithID("c1").
		WithBody("Great project!").
		WithAuthor("commenter1").
		WithParentPost("userpost1").
		WithSubreddit("golang").
		WithScore(5).
		Build()

	server := testutil.NewMockServer().
		WithAccount(account).
		WithPosts("testuser", "hot", userPost1, userPost2).
		WithComments("testuser", "userpost1", userPost1, comment).
		Start()
	defer server.Close()

	client := createWorkflowClient(t, server)
	ctx := context.Background()

	// Step 1: Get current user info
	t.Run("GetUserInfo", func(t *testing.T) {
		acc, err := client.Me(ctx)
		testutil.AssertNoError(t, err)

		if acc.Name != "t2_user123" {
			t.Errorf("Expected username 't2_user123', got '%s'", acc.Name)
		}

		if acc.LinkKarma != 5000 {
			t.Errorf("Expected 5000 link karma, got %d", acc.LinkKarma)
		}

		t.Logf("Retrieved user info: %s (%d link karma, %d comment karma)",
			acc.Name, acc.LinkKarma, acc.CommentKarma)
	})

	// Step 2: Get user's posts
	t.Run("GetUserPosts", func(t *testing.T) {
		resp, err := client.GetHot(ctx, &types.PostsRequest{
			Subreddit: "testuser",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		testutil.AssertNoError(t, err)
		testutil.AssertPostCount(t, resp, 2)

		// Verify all posts belong to the user
		for _, post := range resp.Posts {
			if post.Author != "testuser" {
				t.Errorf("Expected post author 'testuser', got '%s'", post.Author)
			}
		}

		t.Logf("Retrieved %d user posts", len(resp.Posts))
	})

	// Step 3: Get user's comments
	t.Run("GetUserComments", func(t *testing.T) {
		_, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "testuser",
			PostID:    "userpost1",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		testutil.AssertNoError(t, err)
		t.Logf("Retrieved user comments (simulated)")
	})
}

// TestMoreCommentsIntegrationWorkflow tests the complete more comments flow
func TestMoreCommentsIntegrationWorkflow(t *testing.T) {
	// Setup test data
	post := testutil.NewPostBuilder().
		WithID("post1").
		WithTitle("Post with Many Comments").
		WithAuthor("testuser").
		WithSubreddit("golang").
		WithScore(100).
		WithNumComments(100).
		Build()

	// Create comment with many children IDs for "more" functionality
	moreIDs := make([]string, 20)
	for i := 0; i < 20; i++ {
		moreIDs[i] = fmt.Sprintf("comment%c", rune('a'+i+2))
	}

	comment1 := testutil.NewCommentBuilder().
		WithID("comment1").
		WithBody("First comment").
		WithAuthor("user1").
		WithScore(10).
		WithParentPost("post1").
		WithSubreddit("golang").
		Build()

	// Store more IDs in comment (this would normally be in a More object in replies)
	comment1.MoreChildrenIDs = moreIDs[:10]

	moreIDs2 := make([]string, 30)
	for i := 0; i < 30; i++ {
		moreIDs2[i] = fmt.Sprintf("c%d", i+100)
	}

	comment2 := testutil.NewCommentBuilder().
		WithID("comment2").
		WithBody("Second comment").
		WithAuthor("user2").
		WithScore(8).
		WithParentPost("post1").
		WithSubreddit("golang").
		Build()

	comment2.MoreChildrenIDs = moreIDs2[:10]

	server := testutil.NewMockServer().
		WithComments("golang", "post1", post, comment1, comment2).
		Start()
	defer server.Close()

	client := createWorkflowClient(t, server)
	ctx := context.Background()

	// Step 1: Get initial comments with many "more" placeholders
	t.Run("GetInitialCommentsWithManyMore", func(t *testing.T) {
		commentsResp, err := client.GetComments(ctx, &types.CommentsRequest{
			Subreddit: "golang",
			PostID:    "post1",
			Pagination: types.Pagination{
				Limit: 10,
			},
		})

		testutil.AssertNoError(t, err)
		testutil.AssertCommentCount(t, commentsResp, 2)

		// Note: The mock server doesn't populate MoreIDs from MoreChildrenIDs
		// In a real scenario, the parser would extract these from nested "more" objects
		t.Logf("Retrieved %d comments", len(commentsResp.Comments))
	})

	// Note: MockServer doesn't have /api/morechildren endpoint yet,
	// so we skip the batch loading tests for now
}
