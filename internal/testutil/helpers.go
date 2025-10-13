// Package testutil provides reusable test helpers and assertions for the Reddit API wrapper.
package testutil

import (
	"context"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// NOTE: NewTestClient and NewTestClientWithMocks have been removed to avoid import cycles.
// The main package (graw) cannot be imported from internal/testutil because it would create
// a circular dependency. Instead, test files should create their own client factory functions
// using the builders and MockServer from this package.
//
// See reddit_test.go for an example of how to create test clients without import cycles.

// DefaultPost returns a fully populated Post with realistic default values.
// This is useful for tests that need a complete post object without caring
// about specific field values.
//
// Example:
//
//	post := testutil.DefaultPost()
//	post.Title = "Custom title for this test"
//	// Use post in your test
func DefaultPost() *types.Post {
	return &types.Post{
		ThingData: types.ThingData{
			ID:   "abc123",
			Name: "t3_abc123",
		},
		Votable: types.Votable{
			Score: 100,
			Ups:   100,
			Downs: 0,
			Likes: nil,
		},
		Created: types.Created{
			Created:    1609459200.0,
			CreatedUTC: 1609459200.0,
		},
		Author:              "test_user",
		AuthorFlairCSSClass: nil,
		AuthorFlairText:     nil,
		Clicked:             false,
		Domain:              "self.golang",
		Hidden:              false,
		IsSelf:              true,
		LinkFlairCSSClass:   nil,
		LinkFlairText:       nil,
		Locked:              false,
		Media:               nil,
		MediaEmbed:          nil,
		NumComments:         42,
		Over18:              false,
		Permalink:           "/r/golang/comments/abc123/test_post/",
		Saved:               false,
		SelfText:            "This is a test post body",
		SelfTextHTML:        stringPtr("<p>This is a test post body</p>"),
		Subreddit:           "golang",
		SubredditID:         "t5_2rc7j",
		Thumbnail:           "self",
		Title:               "Test Post Title",
		URL:                 "https://reddit.com/r/golang/comments/abc123/test_post/",
		Edited: types.Edited{
			IsEdited:  false,
			Timestamp: 0,
		},
		Distinguished: nil,
		Stickied:      false,
		UpvoteRatio:   0.95,
	}
}

// DefaultComment returns a fully populated Comment with realistic default values.
// This is useful for tests that need a complete comment object without caring
// about specific field values.
//
// Example:
//
//	comment := testutil.DefaultComment()
//	comment.Body = "Custom comment body for this test"
//	// Use comment in your test
func DefaultComment() *types.Comment {
	return &types.Comment{
		ThingData: types.ThingData{
			ID:   "def456",
			Name: "t1_def456",
		},
		Votable: types.Votable{
			Score: 50,
			Ups:   50,
			Downs: 0,
			Likes: nil,
		},
		Created: types.Created{
			Created:    1609459200.0,
			CreatedUTC: 1609459200.0,
		},
		ApprovedBy:          nil,
		Author:              "test_commenter",
		AuthorFlairCSSClass: nil,
		AuthorFlairText:     nil,
		BannedBy:            nil,
		Body:                "This is a test comment",
		BodyHTML:            "<div>This is a test comment</div>",
		Edited: types.Edited{
			IsEdited:  false,
			Timestamp: 0,
		},
		Gilded:          0,
		LinkAuthor:      "test_user",
		LinkID:          "t3_abc123",
		LinkTitle:       "Test Post Title",
		LinkURL:         "https://reddit.com/r/golang/comments/abc123/test_post/",
		NumReports:      nil,
		ParentID:        "t3_abc123",
		Replies:         nil,
		Saved:           false,
		ScoreHidden:     false,
		Subreddit:       "golang",
		SubredditID:     "t5_2rc7j",
		Distinguished:   nil,
		MoreChildrenIDs: nil,
	}
}

// DefaultSubreddit returns a fully populated SubredditData with realistic default values.
// This is useful for tests that need a complete subreddit object.
//
// Example:
//
//	subreddit := testutil.DefaultSubreddit()
//	subreddit.DisplayName = "testsubreddit"
//	// Use subreddit in your test
func DefaultSubreddit() *types.SubredditData {
	return &types.SubredditData{
		ThingData: types.ThingData{
			ID:   "2rc7j",
			Name: "t5_2rc7j",
		},
		AccountsActive:       1000,
		CommentScoreHideMins: 0,
		Description:          "A subreddit for testing",
		DescriptionHTML:      "<p>A subreddit for testing</p>",
		DisplayName:          "golang",
		HeaderImg:            stringPtr("https://example.com/header.png"),
		HeaderSize:           []int{120, 40},
		HeaderTitle:          stringPtr("Go Programming"),
		Over18:               false,
		PublicDescription:    "Public description of the test subreddit",
		PublicTraffic:        true,
		Subscribers:          100000,
		SubmissionType:       "any",
		SubmitLinkLabel:      stringPtr("Submit a link"),
		SubmitTextLabel:      stringPtr("Submit a text post"),
		SubredditType:        "public",
		Title:                "The Go Programming Language",
		URL:                  "/r/golang/",
		UserIsBanned:         boolPtr(false),
		UserIsContributor:    boolPtr(false),
		UserIsModerator:      boolPtr(false),
		UserIsSubscriber:     boolPtr(true),
	}
}

// DefaultAccount returns a fully populated AccountData with realistic default values.
// This is useful for tests that need a complete account object.
//
// Example:
//
//	account := testutil.DefaultAccount()
//	account.LinkKarma = 5000
//	// Use account in your test
func DefaultAccount() *types.AccountData {
	return &types.AccountData{
		ThingData: types.ThingData{
			ID:   "test123",
			Name: "t2_test123",
		},
		Created: types.Created{
			Created:    1609459200.0,
			CreatedUTC: 1609459200.0,
		},
		CommentKarma:     1000,
		HasMail:          boolPtr(false),
		HasModMail:       boolPtr(false),
		HasVerifiedEmail: boolPtr(true),
		InboxCount:       0,
		IsFriend:         false,
		IsGold:           false,
		IsMod:            false,
		LinkKarma:        2000,
		Modhash:          "testmodhash",
		Over18:           false,
	}
}

// MockTokenProvider is a simple implementation of the TokenProvider interface for testing.
// It returns a configurable token and error, allowing tests to simulate both successful
// and failed authentication scenarios.
//
// Example:
//
//	// Successful auth
//	mockAuth := &testutil.MockTokenProvider{Token: "valid-token"}
//	token, err := mockAuth.GetToken(ctx)
//
//	// Failed auth
//	mockAuth := &testutil.MockTokenProvider{Err: errors.New("auth failed")}
//	token, err := mockAuth.GetToken(ctx)
type MockTokenProvider struct {
	// Token is the token to return from GetToken
	Token string
	// Err is the error to return from GetToken (if set, Token is ignored)
	Err error
	// InvalidateCount tracks how many times InvalidateToken was called
	InvalidateCount int
}

// GetToken returns the configured token or error.
// This implements the TokenProvider interface.
func (m *MockTokenProvider) GetToken(ctx context.Context) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Token, nil
}

// InvalidateToken increments the InvalidateCount.
// This implements the TokenProvider interface.
func (m *MockTokenProvider) InvalidateToken() {
	m.InvalidateCount++
}

// stringPtr returns a pointer to the given string.
// This is a helper for initializing pointer fields in test data.
func stringPtr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the given bool.
// This is a helper for initializing pointer fields in test data.
func boolPtr(b bool) *bool {
	return &b
}

// intPtr returns a pointer to the given int.
// This is a helper for initializing pointer fields in test data.
func intPtr(i int) *int {
	return &i
}

// NOTE: MockHTTPClient has been removed to avoid import cycles and type mismatches.
// Test files should define their own mock HTTP client implementations that match
// the exact interface required by the Reddit client.
//
// See reddit_test.go for an example of how to define a mockHTTPClient that works
// with the actual client interfaces without causing import cycles.
