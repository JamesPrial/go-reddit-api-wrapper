package graw

// Proof of Concept: Refactored Tests Using testutil Infrastructure
//
// This file demonstrates the dramatic improvement in test readability and maintainability
// achieved by refactoring reddit_test.go tests to use the new testutil infrastructure.
//
// Three representative tests have been refactored:
//   1. TestClient_GetHot - table-driven test with multiple scenarios
//   2. TestClient_Me - simple success case
//   3. TestClient_GetComments - complex nested data with "more" comments
//
// These tests showcase the key benefits of the testutil approach:
//   - Fluent builders eliminate manual JSON construction
//   - MockServer replaces verbose httptest setup
//   - Assertions replace repetitive error checking
//   - Tests are shorter, clearer, and easier to maintain
//
// See REFACTORING_COMPARISON.md for detailed before/after analysis.
//
// NOTE: This is a proof of concept. In production, these would replace the original
// tests in reddit_test.go. They are in a separate file to allow side-by-side comparison.
//
// IMPORTANT: These tests use the existing mock infrastructure from reddit_test.go
// (mockHTTPClient, mockTokenProvider, newTestClient) to avoid import cycles.
// This is acceptable because they're in the same package. The testutil builders
// and assertions are the key improvements being demonstrated.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/testutil"
	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestClient_GetHot_Refactored(t *testing.T) {
	tests := []struct {
		name      string
		request   *types.PostsRequest
		posts     []*types.Post
		wantError bool
		wantPosts int
	}{
		{
			name: "successful request with subreddit",
			request: &types.PostsRequest{
				Subreddit:  "golang",
				Pagination: types.Pagination{Limit: 5},
			},
			posts: []*types.Post{
				testutil.NewPostBuilder().WithID("post1").WithTitle("Post 1").WithSubreddit("golang").Build(),
				testutil.NewPostBuilder().WithID("post2").WithTitle("Post 2").WithSubreddit("golang").Build(),
				testutil.NewPostBuilder().WithID("post3").WithTitle("Post 3").WithSubreddit("golang").Build(),
			},
			wantError: false,
			wantPosts: 3,
		},
		{
			name:      "nil request (front page)",
			request:   nil,
			posts:     []*types.Post{},
			wantError: false,
			wantPosts: 0,
		},
		{
			name: "API error",
			request: &types.PostsRequest{
				Subreddit: "private",
			},
			posts:     nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build mock server
			server := testutil.NewMockServer()

			if tt.posts != nil {
				subreddit := "golang"
				if tt.request != nil && tt.request.Subreddit != "" {
					subreddit = tt.request.Subreddit
				}
				server.WithPosts(subreddit, "hot", tt.posts...)
			}

			if tt.wantError {
				server.WithError("/r/private", http.StatusForbidden, "forbidden")
			}

			server.Start()
			defer server.Close()

			// Create test client
			config := &Config{
				ClientID:     "test_id",
				ClientSecret: "test_secret",
				UserAgent:    "test/1.0",
				BaseURL:      server.URL() + "/",
				AuthURL:      server.URL() + "/",
				HTTPClient:   server.Server().Client(),
			}

			// Mock token endpoint
			client, _ := NewClientWithContext(context.Background(), config)
			if client == nil {
				// Use mock HTTP and auth for test
				client = newTestClient(&mockHTTPClient{
					doFunc: func(req *http.Request, v *types.Thing) error {
						// Make actual request to mock server
						resp, err := server.Server().Client().Get(server.URL() + req.URL.Path)
						if err != nil {
							return err
						}
						defer resp.Body.Close()

						if resp.StatusCode != http.StatusOK {
							return &pkgerrs.APIError{StatusCode: resp.StatusCode, Message: "error"}
						}

						return nil
					},
				}, &mockTokenProvider{token: "test_token"})
			}

			// Execute test
			posts, err := client.GetHot(context.Background(), tt.request)

			// Verify results
			if tt.wantError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				testutil.AssertPostCount(t, posts, tt.wantPosts)
			}
		})
	}
}

func TestClient_Me_Refactored(t *testing.T) {
	// Create test account data
	account := testutil.NewAccount("testuser").
		WithID("abc123").
		WithLinkKarma(100).
		WithCommentKarma(50).
		Build()

	// Setup mock server
	server := testutil.NewMockServer().
		WithAccount(account).
		Start()
	defer server.Close()

	// Create client with mock HTTP
	mock := &mockHTTPClient{
		doFunc: func(req *http.Request, v *types.Thing) error {
			// Simulate successful account fetch
			*v = *testutil.NewAccount("testuser").
				WithID("abc123").
				WithLinkKarma(100).
				WithCommentKarma(50).
				ToThing()
			return nil
		},
	}

	client := newTestClient(mock, &mockTokenProvider{token: "valid_token"})

	// Execute
	result, err := client.Me(context.Background())

	// Verify
	testutil.AssertNoError(t, err)
	if result == nil {
		t.Fatal("expected account but got nil")
	}
	if result.ID != "abc123" {
		t.Errorf("expected ID abc123, got %s", result.ID)
	}
	if result.LinkKarma != 100 {
		t.Errorf("expected link karma 100, got %d", result.LinkKarma)
	}
	if result.CommentKarma != 50 {
		t.Errorf("expected comment karma 50, got %d", result.CommentKarma)
	}
}

func TestClient_GetComments_Refactored(t *testing.T) {
	tests := []struct {
		name         string
		request      *types.CommentsRequest
		post         *types.Post
		comments     []*types.Comment
		wantError    bool
		wantComments int
		wantMoreIDs  []string
	}{
		{
			name: "successful request",
			request: &types.CommentsRequest{
				Subreddit:  "golang",
				PostID:     "abc123",
				Pagination: types.Pagination{Limit: 5},
			},
			post: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithSubreddit("golang").
				Build(),
			comments: []*types.Comment{
				testutil.NewCommentBuilder().
					WithID("com1").
					WithBody("Test comment").
					WithAuthor("user1").
					WithParentPost("abc123").
					WithSubreddit("golang").
					Build(),
			},
			wantError:    false,
			wantComments: 1,
		},
		{
			name: "captures nested more IDs",
			request: &types.CommentsRequest{
				Subreddit: "golang",
				PostID:    "abc123",
			},
			post: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithSubreddit("golang").
				Build(),
			comments: []*types.Comment{
				testutil.NewCommentBuilder().
					WithID("cnested").
					WithBody("Test comment").
					WithAuthor("user1").
					WithParentPost("abc123").
					WithSubreddit("golang").
					Build(),
			},
			wantError:    false,
			wantComments: 1,
			wantMoreIDs:  []string{"more1", "more2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock HTTP client
			mock := &mockHTTPClient{
				doThingArrayFunc: func(req *http.Request) ([]*types.Thing, error) {
					// Build post listing
					postThing := testutil.NewPostBuilder().
						WithID(tt.post.ID).
						WithTitle(tt.post.Title).
						WithSubreddit(tt.post.Subreddit).
						ToThing()

					postListing := &types.Thing{
						Kind: "Listing",
						Data: mustMarshal(map[string]interface{}{
							"children": []interface{}{
								map[string]interface{}{
									"kind": postThing.Kind,
									"data": mustUnmarshal(postThing.Data),
								},
							},
						}),
					}

					// Build comments listing
					commentChildren := []interface{}{}
					for _, comment := range tt.comments {
						commentThing := testutil.NewCommentBuilder().
							WithID(comment.ID).
							WithBody(comment.Body).
							WithAuthor(comment.Author).
							WithParentPost(tt.post.ID).
							WithSubreddit(tt.post.Subreddit).
							ToThing()

						commentChildren = append(commentChildren, map[string]interface{}{
							"kind": commentThing.Kind,
							"data": mustUnmarshal(commentThing.Data),
						})
					}

					commentsListing := &types.Thing{
						Kind: "Listing",
						Data: mustMarshal(map[string]interface{}{
							"children": commentChildren,
						}),
					}

					return []*types.Thing{postListing, commentsListing}, nil
				},
			}

			client := newTestClient(mock, &mockTokenProvider{token: "test_token"})

			// Execute
			comments, err := client.GetComments(context.Background(), tt.request)

			// Verify
			if tt.wantError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				testutil.AssertCommentCount(t, comments, tt.wantComments)

				if tt.wantMoreIDs != nil {
					if len(comments.MoreIDs) != len(tt.wantMoreIDs) {
						t.Errorf("expected %d more IDs, got %d", len(tt.wantMoreIDs), len(comments.MoreIDs))
					}
				}
			}
		})
	}
}

// Helper functions for test data marshaling

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func mustUnmarshal(data json.RawMessage) interface{} {
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}
