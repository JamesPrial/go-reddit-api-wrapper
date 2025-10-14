package internal

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestParseThing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		thing        *types.Thing
		expectError  bool
		expectedType string
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:         "Listing kind",
			thing:        testutil.NewListingBuilder().Build().ToThing(),
			expectError:  false,
			expectedType: "*types.ListingData",
		},
		{
			name:         "t1 comment",
			thing:        testutil.NewCommentBuilder().WithID("comment123").ToThing(),
			expectError:  false,
			expectedType: "*types.Comment",
		},
		{
			name:         "t2 account",
			thing:        testutil.NewAccount("testuser").WithID("user123").ToThing(),
			expectError:  false,
			expectedType: "*types.AccountData",
		},
		{
			name:         "t3 link",
			thing:        testutil.NewPostBuilder().WithID("post123").ToThing(),
			expectError:  false,
			expectedType: "*types.Post",
		},
		{
			name:         "t4 message",
			thing:        testutil.NewMessageBuilder().WithID("msg123").ToThing(),
			expectError:  false,
			expectedType: "*types.MessageData",
		},
		{
			name:         "t5 subreddit",
			thing:        testutil.NewSubreddit("golang").WithID("2qh1i").ToThing(),
			expectError:  false,
			expectedType: "*types.SubredditData",
		},
		{
			name:         "more kind",
			thing:        testutil.NewMore().WithID("more123").WithChildren([]string{"id1", "id2", "id3"}).ToThing(),
			expectError:  false,
			expectedType: "*types.MoreData",
		},
		{
			name:        "unknown kind",
			thing:       &types.Thing{Kind: "unknown", Data: []byte(`{}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseThing(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseListing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
		},
		{
			name:        "valid listing",
			thing:       testutil.NewListingBuilder().WithAfter("t3_after123").WithBefore("t3_before456").Build().ToThing(),
			expectError: false,
		},
		{
			name: "listing with children",
			thing: testutil.NewListingBuilder().
				WithAfter("t3_after123").
				AddChild(testutil.NewPostBuilder().WithID("post1").ToThing()).
				AddChild(testutil.NewPostBuilder().WithID("post2").ToThing()).
				Build().
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "Listing", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseListing(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParsePost(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewCommentBuilder().ToThing(),
			expectError: true,
		},
		{
			name: "valid post",
			thing: testutil.NewPostBuilder().
				WithID("post123").
				WithTitle("Test Post").
				WithAuthor("testuser").
				WithScore(100).
				WithSubreddit("golang").
				WithNumComments(50).
				ToThing(),
			expectError: false,
		},
		{
			name: "self post",
			thing: testutil.NewPostBuilder().
				WithID("selfpost456").
				WithTitle("Self Post Title").
				WithAuthor("testuser").
				WithSubreddit("AskReddit").
				WithScore(50).
				WithNumComments(10).
				WithSelfText("This is the self text").
				WithEdited(1234567900).
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "t3", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePost(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseComment(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
		},
		{
			name: "valid comment without replies",
			thing: testutil.NewCommentBuilder().
				WithID("comment123").
				WithAuthor("testuser").
				WithBody("This is a test comment").
				WithScore(10).
				WithParentID("t3_abc123").
				WithLinkID("t3_abc123").
				WithSubreddit("golang").
				ToThing(),
			expectError: false,
		},
		{
			name: "comment with replies",
			thing: testutil.NewCommentBuilder().
				WithID("parentcomment").
				WithAuthor("testuser").
				WithBody("Parent comment").
				WithScore(20).
				WithParentID("t3_post123").
				WithLinkID("t3_post123").
				WithSubreddit("golang").
				WithReply(testutil.NewCommentBuilder().
					WithID("reply1").
					WithAuthor("user2").
					WithBody("Reply").
					Build()).
				ToThing(),
			expectError: false,
		},
		{
			name: "edited comment",
			thing: testutil.NewCommentBuilder().
				WithID("editedcomment").
				WithAuthor("testuser").
				WithBody("Edited comment").
				WithScore(5).
				WithParentID("t1_parent").
				WithLinkID("t3_post123").
				WithSubreddit("golang").
				WithEdited(true, 1234567900).
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "t1", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseComment(context.Background(), tt.thing, &parseContext{
				seenIDs: make(map[string]bool),
			})

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseSubreddit(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
		},
		{
			name: "valid subreddit",
			thing: testutil.NewSubreddit("golang").
				WithID("2qh1i").
				WithTitle("Go Programming Language").
				WithSubscribers(150000).
				WithDescription("A subreddit for Go programmers").
				WithURL("/r/golang").
				WithType("public").
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "t5", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSubreddit(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseAccount(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
		},
		{
			name: "valid account",
			thing: testutil.NewAccount("testuser").
				WithID("user123").
				WithLinkKarma(1000).
				WithCommentKarma(5000).
				WithGold(true).
				WithMod(false).
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "t2", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseAccount(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
		},
		{
			name: "valid message",
			thing: testutil.NewMessageBuilder().
				WithID("msg123").
				WithAuthor("sender").
				WithBody("Message body").
				WithSubject("Test Subject").
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "t4", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseMessage(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseMore(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
		},
		{
			name: "valid more",
			thing: testutil.NewMore().
				WithID("more123").
				WithChildren([]string{"id1", "id2", "id3", "id4"}).
				ToThing(),
			expectError: false,
		},
		{
			name: "empty children",
			thing: testutil.NewMore().
				WithID("more456").
				WithChildren([]string{}).
				ToThing(),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			thing:       &types.Thing{Kind: "more", Data: []byte(`{invalid json}`)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseMore(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestExtractPosts(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		expectCount int
	}{
		{
			name:        "nil listing",
			thing:       nil,
			expectError: true,
			expectCount: 0,
		},
		{
			name:        "empty listing",
			thing:       testutil.NewListingBuilder().Build().ToThing(),
			expectError: false,
			expectCount: 0,
		},
		{
			name: "listing with posts",
			thing: testutil.NewListingBuilder().
				WithAfter("t3_after123").
				AddChild(testutil.NewPostBuilder().
					WithID("post1").
					WithTitle("First Post").
					WithAuthor("user1").
					WithScore(100).
					ToThing()).
				AddChild(testutil.NewPostBuilder().
					WithID("post2").
					WithTitle("Second Post").
					WithAuthor("user2").
					WithScore(200).
					ToThing()).
				Build().
				ToThing(),
			expectError: false,
			expectCount: 2,
		},
		{
			name: "listing with mixed content",
			thing: testutil.NewListingBuilder().
				AddChild(testutil.NewPostBuilder().WithID("post1").ToThing()).
				AddChild(testutil.NewCommentBuilder().WithID("comment1").ToThing()).
				AddChild(testutil.NewMore().WithID("more1").ToThing()).
				Build().
				ToThing(),
			expectError: false,
			expectCount: 1, // Only the t3 post should be extracted
		},
		{
			name:        "wrong kind",
			thing:       testutil.NewPostBuilder().ToThing(),
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			posts, err := parser.ExtractPosts(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if len(posts) != tt.expectCount {
					t.Errorf("expected %d posts, got %d", tt.expectCount, len(posts))
				}
			}
		})
	}
}

func TestExtractComments(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name           string
		thing          *types.Thing
		expectError    bool
		expectComments int
		expectMore     int
	}{
		{
			name: "single comment without replies",
			thing: testutil.NewCommentBuilder().
				WithID("comment1").
				WithAuthor("user1").
				WithBody("Test comment").
				WithParentID("t3_post1").
				ToThing(),
			expectError:    false,
			expectComments: 1,
			expectMore:     0,
		},
		{
			name: "single comment with replies",
			thing: testutil.NewCommentBuilder().
				WithID("comment1").
				WithAuthor("user1").
				WithBody("Parent comment").
				WithParentID("t3_post1").
				WithReply(testutil.NewCommentBuilder().
					WithID("reply1").
					WithAuthor("user2").
					WithBody("Reply").
					WithParentID("t1_comment1").
					Build()).
				ToThing(),
			expectError:    false,
			expectComments: 1, // Parent only (reply is in Replies field)
			expectMore:     0,
		},
		{
			name: "listing with comments and more",
			thing: testutil.NewListingBuilder().
				AddChild(testutil.NewCommentBuilder().
					WithID("comment1").
					WithAuthor("user1").
					WithBody("First comment").
					WithParentID("t3_post1").
					ToThing()).
				AddChild(testutil.NewCommentBuilder().
					WithID("comment2").
					WithAuthor("user2").
					WithBody("Second comment").
					WithParentID("t3_post1").
					ToThing()).
				AddChild(testutil.NewMore().
					WithID("more1").
					WithChildren([]string{"id1", "id2", "id3"}).
					ToThing()).
				Build().
				ToThing(),
			expectError:    false,
			expectComments: 2,
			expectMore:     3,
		},
		{
			name: "nested comments",
			thing: testutil.NewListingBuilder().
				AddChild(testutil.NewCommentBuilder().
					WithID("comment1").
					WithAuthor("user1").
					WithBody("Parent").
					WithParentID("t3_post1").
					WithReply(testutil.NewCommentBuilder().
						WithID("reply1").
						WithAuthor("user2").
						WithBody("Child").
						WithParentID("t1_comment1").
						WithReply(testutil.NewCommentBuilder().
							WithID("reply2").
							WithAuthor("user3").
							WithBody("Grandchild").
							WithParentID("t1_reply1").
							Build()).
						Build()).
					ToThing()).
				Build().
				ToThing(),
			expectError:    false,
			expectComments: 1, // Parent only (tree structure maintained)
			expectMore:     0,
		},
		{
			name:           "wrong kind",
			thing:          testutil.NewPostBuilder().ToThing(),
			expectError:    true,
			expectComments: 0,
			expectMore:     0,
		},
		{
			name:           "empty listing",
			thing:          testutil.NewListingBuilder().Build().ToThing(),
			expectError:    false,
			expectComments: 0,
			expectMore:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comments, moreIDs, err := parser.ExtractComments(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if len(comments) != tt.expectComments {
					t.Errorf("expected %d comments, got %d", tt.expectComments, len(comments))
				}
				if len(moreIDs) != tt.expectMore {
					t.Errorf("expected %d more IDs, got %d", tt.expectMore, len(moreIDs))
				}
			}
		})
	}
}

func TestExtractPostAndComments(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name           string
		response       []*types.Thing
		expectError    bool
		expectPost     bool
		expectComments int
		expectMore     int
	}{
		{
			name:        "nil response",
			response:    nil,
			expectError: true,
		},
		{
			name:        "empty response",
			response:    []*types.Thing{},
			expectError: true,
		},
		{
			name: "single element response with comments",
			response: []*types.Thing{
				testutil.NewListingBuilder().Build().ToThing(),
			},
			expectError: false,
			// Should return nil post with empty comments
		},
		{
			name: "valid post and comments",
			response: []*types.Thing{
				testutil.NewListingBuilder().
					AddChild(testutil.NewPostBuilder().
						WithID("post1").
						WithTitle("Test Post").
						WithAuthor("postauthor").
						WithScore(100).
						WithNumComments(2).
						ToThing()).
					Build().
					ToThing(),
				testutil.NewListingBuilder().
					AddChild(testutil.NewCommentBuilder().
						WithID("comment1").
						WithAuthor("commenter1").
						WithBody("First comment").
						WithParentID("t3_post1").
						ToThing()).
					AddChild(testutil.NewCommentBuilder().
						WithID("comment2").
						WithAuthor("commenter2").
						WithBody("Second comment").
						WithParentID("t3_post1").
						WithReply(testutil.NewCommentBuilder().
							WithID("reply1").
							WithAuthor("replier").
							WithBody("Reply").
							WithParentID("t1_comment2").
							Build()).
						ToThing()).
					AddChild(testutil.NewMore().
						WithID("more1").
						WithChildren([]string{"id1", "id2"}).
						ToThing()).
					Build().
					ToThing(),
			},
			expectError:    false,
			expectPost:     true,
			expectComments: 2, // 2 top-level comments (reply is in Replies field)
			expectMore:     2,
		},
		{
			name: "no post in first listing",
			response: []*types.Thing{
				testutil.NewListingBuilder().Build().ToThing(),
				testutil.NewListingBuilder().Build().ToThing(),
			},
			expectError:    false, // Changed: We now handle missing posts gracefully
			expectPost:     false,
			expectComments: 0,
			expectMore:     0,
		},
		{
			name: "invalid second listing",
			response: []*types.Thing{
				testutil.NewListingBuilder().
					AddChild(testutil.NewPostBuilder().
						WithID("post1").
						WithTitle("Test Post").
						WithAuthor("postauthor").
						ToThing()).
					Build().
					ToThing(),
				testutil.NewPostBuilder().ToThing(), // Wrong kind, should be Listing
			},
			expectError:    false, // Post extraction succeeds, comment extraction fails but error contains post
			expectPost:     true,
			expectComments: 0,
			expectMore:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ExtractPostAndComments(context.Background(), tt.response)

			if tt.expectError {
				testutil.AssertError(t, err)
			} else {
				// For the "invalid second listing" case, we expect an error about comment extraction
				// but the post should still be returned
				if tt.name == "invalid second listing" {
					if err == nil {
						t.Errorf("expected comment extraction error but got none")
					}
				} else if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if result == nil {
					t.Fatal("expected result but got nil")
				}

				if tt.expectPost {
					if result.Post == nil {
						t.Errorf("expected post but got nil")
					}
				} else {
					if result.Post != nil {
						t.Errorf("expected no post but got one")
					}
				}

				if len(result.Comments) != tt.expectComments {
					t.Errorf("expected %d comments, got %d", tt.expectComments, len(result.Comments))
				}
				if len(result.MoreIDs) != tt.expectMore {
					t.Errorf("expected %d more IDs, got %d", tt.expectMore, len(result.MoreIDs))
				}
			}
		})
	}
}

func TestExtractPostAndComments_EdgeCases(t *testing.T) {
	parser := NewParser()

	t.Run("single listing with post only", func(t *testing.T) {
		// Single listing format tries ExtractComments first, which succeeds with 0 comments
		// when children are posts (t3). This is expected behavior - single listing is assumed
		// to be comments, not posts.
		response := []*types.Thing{
			testutil.NewListingBuilder().
				AddChild(testutil.NewPostBuilder().
					WithID("post1").
					WithTitle("Post").
					WithAuthor("author").
					ToThing()).
				Build().
				ToThing(),
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		testutil.AssertNoError(t, err)
		// ExtractComments succeeds but finds no t1 children, returns nil post
		if result.Post != nil {
			t.Errorf("expected no post for single listing, got %v", result.Post)
		}
		if len(result.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(result.Comments))
		}
		if len(result.MoreIDs) != 0 {
			t.Errorf("expected 0 more IDs, got %d", len(result.MoreIDs))
		}
	})

	t.Run("first listing fails to parse, second has comments", func(t *testing.T) {
		response := []*types.Thing{
			testutil.NewPostBuilder().ToThing(), // Wrong kind, should be Listing
			testutil.NewListingBuilder().
				AddChild(testutil.NewCommentBuilder().
					WithID("comment1").
					WithAuthor("commenter").
					WithBody("Comment").
					WithParentID("t3_post1").
					ToThing()).
				Build().
				ToThing(),
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		testutil.AssertNoError(t, err)
		if result.Post != nil {
			t.Errorf("expected no post but got one")
		}
		if len(result.Comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(result.Comments))
		}
		if len(result.MoreIDs) != 0 {
			t.Errorf("expected 0 more IDs, got %d", len(result.MoreIDs))
		}
	})

	t.Run("both post and comment extraction fail", func(t *testing.T) {
		response := []*types.Thing{
			testutil.NewCommentBuilder().ToThing(), // Wrong kind for posts
			testutil.NewPostBuilder().ToThing(),    // Wrong kind for comments
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		testutil.AssertError(t, err)
		if result != nil {
			t.Errorf("expected nil result but got %v", result)
		}
	})

	t.Run("single listing with invalid data", func(t *testing.T) {
		response := []*types.Thing{
			testutil.NewPostBuilder().ToThing(), // Wrong kind, not Listing or t1
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		testutil.AssertError(t, err)
		if result != nil {
			t.Error("expected nil result on error")
		}
	})
}

// Test edge cases for Edited type unmarshaling
func TestEditedUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		isEdited  bool
		timestamp float64
	}{
		{
			name:      "false boolean",
			input:     `false`,
			expectErr: false,
			isEdited:  false,
			timestamp: 0,
		},
		{
			name:      "true boolean",
			input:     `true`,
			expectErr: false,
			isEdited:  true,
			timestamp: 0,
		},
		{
			name:      "timestamp",
			input:     `1234567890.5`,
			expectErr: false,
			isEdited:  true,
			timestamp: 1234567890.5,
		},
		{
			name:      "null",
			input:     `null`,
			expectErr: false,
			isEdited:  false, // null means not edited
			timestamp: 0,
		},
		{
			name:      "invalid",
			input:     `"string"`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e types.Edited
			err := e.UnmarshalJSON([]byte(tt.input))

			if tt.expectErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				if e.IsEdited != tt.isEdited {
					t.Errorf("expected IsEdited=%v, got %v", tt.isEdited, e.IsEdited)
				}
				if e.Timestamp != tt.timestamp {
					t.Errorf("expected Timestamp=%v, got %v", tt.timestamp, e.Timestamp)
				}
			}
		})
	}
}

// TestCommentTreeStructure verifies that comments maintain a proper tree structure
// where each comment's Replies field contains only direct children, not all descendants.
// Note: This test uses raw JSON because it tests the parser's ability to handle
// Reddit's nested Listing format for replies, which is complex to recreate with builders.
func TestCommentTreeStructure(t *testing.T) {
	parser := NewParser()

	// Create a complex tree: parent -> child -> grandchild
	thing := &types.Thing{
		Kind: "t1",
		Data: []byte(`{
			"id": "parent",
			"name": "t1_parent",
			"author": "user1",
			"body": "Parent comment",
			"score": 100,
			"ups": 100,
			"downs": 0,
			"created": 1234567890,
			"created_utc": 1234567890,
			"parent_id": "t3_post1",
			"link_id": "t3_post1",
			"subreddit": "test",
			"replies": {
				"kind": "Listing",
				"data": {
					"children": [
						{
							"kind": "t1",
							"id": "child",
							"name": "t1_child",
							"data": {
								"id": "child",
								"name": "t1_child",
								"author": "user2",
								"body": "Child comment",
								"score": 50,
								"ups": 50,
								"downs": 0,
								"created": 1234567890,
								"created_utc": 1234567890,
								"parent_id": "t1_parent",
								"link_id": "t3_post1",
								"subreddit": "test",
								"replies": {
									"kind": "Listing",
									"data": {
										"children": [
											{
												"kind": "t1",
												"id": "grandchild",
												"name": "t1_grandchild",
												"data": {
													"id": "grandchild",
													"name": "t1_grandchild",
													"author": "user3",
													"body": "Grandchild comment",
													"score": 10,
													"ups": 10,
													"downs": 0,
													"created": 1234567890,
													"created_utc": 1234567890,
													"parent_id": "t1_child",
													"link_id": "t3_post1",
													"subreddit": "test",
													"replies": ""
												}
											}
										]
									}
								}
							}
						},
						{
							"kind": "t1",
							"id": "child2",
							"name": "t1_child2",
							"data": {
								"id": "child2",
								"name": "t1_child2",
								"author": "user4",
								"body": "Second child",
								"score": 25,
								"ups": 25,
								"downs": 0,
								"created": 1234567890,
								"created_utc": 1234567890,
								"parent_id": "t1_parent",
								"link_id": "t3_post1",
								"subreddit": "test",
								"replies": ""
							}
						}
					]
				}
			}
		}`),
	}

	parent, err := parser.ParseComment(context.Background(), thing, &parseContext{
		seenIDs: make(map[string]bool),
	})
	testutil.AssertNoError(t, err)

	// Verify parent has exactly 2 direct children (not 3 with grandchild)
	if len(parent.Replies) != 2 {
		t.Errorf("Parent should have 2 direct children, got %d", len(parent.Replies))
	}

	// Verify first child exists
	if parent.Replies[0].Author != "user2" {
		t.Errorf("First child author = %q, want %q", parent.Replies[0].Author, "user2")
	}

	// Verify first child has exactly 1 child (grandchild)
	if len(parent.Replies[0].Replies) != 1 {
		t.Errorf("First child should have 1 reply, got %d", len(parent.Replies[0].Replies))
	}

	// Verify grandchild exists at correct level
	if parent.Replies[0].Replies[0].Author != "user3" {
		t.Errorf("Grandchild author = %q, want %q", parent.Replies[0].Replies[0].Author, "user3")
	}

	// Verify grandchild has no replies
	if len(parent.Replies[0].Replies[0].Replies) != 0 {
		t.Errorf("Grandchild should have 0 replies, got %d", len(parent.Replies[0].Replies[0].Replies))
	}

	// Verify second child exists and has no replies
	if parent.Replies[1].Author != "user4" {
		t.Errorf("Second child author = %q, want %q", parent.Replies[1].Author, "user4")
	}
	if len(parent.Replies[1].Replies) != 0 {
		t.Errorf("Second child should have 0 replies, got %d", len(parent.Replies[1].Replies))
	}
}

// TestParsePost_MaliciousData tests that malicious or malformed post data is rejected
func TestParsePost_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "uppercase post ID",
			thing: testutil.NewPostBuilder().
				WithID("ABC123").
				WithTitle("Test Post").
				ToThing(),
			expectError: true,
			errorText:   "ID has invalid format",
		},
		{
			name: "SQL injection in ID",
			thing: testutil.NewPostBuilder().
				WithID("abc'; DROP TABLE posts--").
				WithTitle("Test Post").
				ToThing(),
			expectError: true,
			errorText:   "ID has invalid format",
		},
		{
			name: "invalid subreddit name - too short",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithSubreddit("ab").
				ToThing(),
			expectError: true,
			errorText:   "Subreddit has invalid format",
		},
		{
			name: "invalid subreddit name - special chars",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithSubreddit("test$").
				ToThing(),
			expectError: true,
			errorText:   "Subreddit has invalid format",
		},
		{
			name: "invalid permalink format",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithPermalink("/invalid/permalink/format").
				ToThing(),
			expectError: true,
			errorText:   "Permalink has invalid format",
		},
		{
			name: "negative NumComments",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithNumComments(-5).
				ToThing(),
			expectError: true,
			errorText:   "NumComments cannot be negative",
		},
		{
			name: "UpvoteRatio out of range - too high",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithUpvoteRatio(1.5).
				ToThing(),
			expectError: true,
			errorText:   "UpvoteRatio must be between 0 and 1",
		},
		{
			name: "UpvoteRatio out of range - negative",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithUpvoteRatio(-0.5).
				ToThing(),
			expectError: true,
			errorText:   "UpvoteRatio must be between 0 and 1",
		},
		{
			name: "future timestamp",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithCreated(float64(time.Now().Add(48 * time.Hour).Unix())).
				ToThing(),
			expectError: true,
			errorText:   "CreatedUTC is in the future",
		},
		{
			name: "timestamp before Reddit existed",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithCreated(946684800).
				ToThing(),
			expectError: true,
			errorText:   "CreatedUTC is before Reddit existed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePost(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
				if tt.errorText != "" {
					testutil.AssertStringContains(t, err.Error(), tt.errorText)
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestParseComment_MaliciousData tests that malicious or malformed comment data is rejected
func TestParseComment_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "uppercase comment ID",
			thing: testutil.NewCommentBuilder().
				WithID("DEF456").
				WithBody("Test comment").
				ToThing(),
			expectError: true,
			errorText:   "ID has invalid format",
		},
		{
			name: "invalid ParentID format",
			thing: testutil.NewCommentBuilder().
				WithID("def456").
				WithBody("Test comment").
				WithParentID("INVALID_PARENT").
				ToThing(),
			expectError: true,
			errorText:   "ParentID has invalid fullname format",
		},
		{
			name: "invalid LinkID format",
			thing: testutil.NewCommentBuilder().
				WithID("def456").
				WithBody("Test comment").
				WithLinkID("invalid_link").
				ToThing(),
			expectError: true,
			errorText:   "LinkID has invalid fullname format",
		},
		{
			name: "future timestamp",
			thing: testutil.NewCommentBuilder().
				WithID("def456").
				WithBody("Test comment").
				WithCreated(float64(time.Now().Add(48 * time.Hour).Unix())).
				ToThing(),
			expectError: true,
			errorText:   "CreatedUTC is in the future",
		},
		{
			name: "negative score - should pass (downvoted comments are valid)",
			thing: testutil.NewCommentBuilder().
				WithID("def456").
				WithBody("Test comment").
				WithScore(-50).
				ToThing(),
			expectError: false,
		},
		{
			name: "invalid subreddit name",
			thing: testutil.NewCommentBuilder().
				WithID("def456").
				WithBody("Test comment").
				WithSubreddit("x").
				ToThing(),
			expectError: true,
			errorText:   "Subreddit has invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseComment(context.Background(), tt.thing, &parseContext{
				seenIDs: make(map[string]bool),
			})

			if tt.expectError {
				testutil.AssertError(t, err)
				if tt.errorText != "" {
					testutil.AssertStringContains(t, err.Error(), tt.errorText)
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestParseListing_MaliciousData tests that malicious pagination tokens are rejected
func TestParseListing_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "invalid AfterFullname - uppercase",
			thing: testutil.NewListingBuilder().
				WithAfter("T3_ABC123").
				Build().
				ToThing(),
			expectError: true,
			errorText:   "invalid AfterFullname from Reddit API",
		},
		{
			name: "invalid AfterFullname - SQL injection",
			thing: testutil.NewListingBuilder().
				WithAfter("t3_abc'; DROP TABLE--").
				Build().
				ToThing(),
			expectError: true,
			errorText:   "invalid AfterFullname from Reddit API",
		},
		{
			name: "invalid BeforeFullname - wrong format",
			thing: testutil.NewListingBuilder().
				WithBefore("invalid_format").
				Build().
				ToThing(),
			expectError: true,
			errorText:   "invalid BeforeFullname from Reddit API",
		},
		{
			name: "valid pagination tokens",
			thing: testutil.NewListingBuilder().
				WithAfter("t3_abc123").
				WithBefore("t3_xyz789").
				Build().
				ToThing(),
			expectError: false,
		},
		{
			name: "empty pagination tokens - should pass",
			thing: testutil.NewListingBuilder().
				WithAfter("").
				WithBefore("").
				Build().
				ToThing(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseListing(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
				if tt.errorText != "" {
					testutil.AssertStringContains(t, err.Error(), tt.errorText)
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestParseSubreddit_MaliciousData tests that malicious subreddit data is rejected
func TestParseSubreddit_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "invalid subreddit name - special chars",
			thing: testutil.NewSubreddit("test$subreddit").
				WithID("2qh1i").
				WithTitle("Test Subreddit").
				WithSubscribers(1000).
				ToThing(),
			expectError: true,
			errorText:   "DisplayName has invalid format",
		},
		{
			name: "negative subscriber count",
			thing: testutil.NewSubreddit("testsubreddit").
				WithID("2qh1i").
				WithTitle("Test Subreddit").
				WithSubscribers(-100).
				ToThing(),
			expectError: true,
			errorText:   "Subscribers cannot be negative",
		},
		{
			name: "invalid subreddit name - too short",
			thing: testutil.NewSubreddit("ab").
				WithID("2qh1i").
				WithTitle("Test Subreddit").
				WithSubscribers(1000).
				ToThing(),
			expectError: true,
			errorText:   "DisplayName has invalid format",
		},
		{
			name: "valid subreddit",
			thing: testutil.NewSubreddit("golang").
				WithID("2qh1i").
				WithTitle("Go Programming").
				WithSubscribers(150000).
				ToThing(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSubreddit(context.Background(), tt.thing)

			if tt.expectError {
				testutil.AssertError(t, err)
				if tt.errorText != "" {
					testutil.AssertStringContains(t, err.Error(), tt.errorText)
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				testutil.AssertNoError(t, err)
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestCommentTreeWithMoreIDs verifies that MoreChildrenIDs are properly collected
// at each level of the tree.
// Note: This test uses raw JSON because it tests the parser's ability to handle
// Reddit's nested Listing format for replies with "more" continuations.
func TestCommentTreeWithMoreIDs(t *testing.T) {
	parser := NewParser()

	thing := &types.Thing{
		Kind: "t1",
		Data: []byte(`{
			"id": "parent",
			"name": "t1_parent",
			"author": "user1",
			"body": "Parent comment",
			"score": 100,
			"ups": 100,
			"downs": 0,
			"created": 1234567890,
			"created_utc": 1234567890,
			"parent_id": "t3_post1",
			"link_id": "t3_post1",
			"subreddit": "test",
			"replies": {
				"kind": "Listing",
				"data": {
					"children": [
						{
							"kind": "t1",
							"id": "child",
							"name": "t1_child",
							"data": {
								"id": "child",
								"name": "t1_child",
								"author": "user2",
								"body": "Child comment",
								"score": 50,
								"ups": 50,
								"downs": 0,
								"created": 1234567890,
								"created_utc": 1234567890,
								"parent_id": "t1_parent",
								"link_id": "t3_post1",
								"subreddit": "test",
								"replies": ""
							}
						},
						{
							"kind": "more",
							"id": "more1",
							"name": "t2_more1",
							"data": {
								"id": "more1",
								"name": "t2_more1",
								"children": ["id1", "id2", "id3"]
							}
						}
					]
				}
			}
		}`),
	}

	parent, err := parser.ParseComment(context.Background(), thing, &parseContext{
		seenIDs: make(map[string]bool),
	})
	testutil.AssertNoError(t, err)

	// Verify parent has 1 child and 3 more IDs
	if len(parent.Replies) != 1 {
		t.Errorf("Parent should have 1 child, got %d", len(parent.Replies))
	}
	if len(parent.MoreChildrenIDs) != 3 {
		t.Errorf("Parent should have 3 more IDs, got %d", len(parent.MoreChildrenIDs))
	}

	// Verify more IDs are correct
	expectedIDs := []string{"id1", "id2", "id3"}
	for i, id := range expectedIDs {
		if i >= len(parent.MoreChildrenIDs) || parent.MoreChildrenIDs[i] != id {
			t.Errorf("MoreChildrenIDs[%d] = %q, want %q", i, parent.MoreChildrenIDs[i], id)
		}
	}
}
