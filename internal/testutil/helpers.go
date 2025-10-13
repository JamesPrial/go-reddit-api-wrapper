// Package testutil provides reusable test helpers and assertions for the Reddit API wrapper.
package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/jamesprial/go-reddit-api-wrapper"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// NewTestClient creates a Reddit client configured to use the provided mock server.
// The client is set up with a mock auth provider that returns "test-token" and
// uses the mock server's URL as the base URL. This is useful for integration-style
// tests where you need a full client connected to a mock HTTP server.
//
// Example:
//
//	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    w.WriteHeader(http.StatusOK)
//	    w.Write([]byte(`{"kind":"Listing","data":{"children":[]}}`))
//	}))
//	defer server.Close()
//
//	client := testutil.NewTestClient(server)
//	posts, err := client.GetHot(ctx, nil)
func NewTestClient(mockServer *httptest.Server) *graw.Reddit {
	config := &graw.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		UserAgent:    "test-agent/1.0",
		BaseURL:      mockServer.URL,
		AuthURL:      mockServer.URL,
		HTTPClient:   mockServer.Client(),
		RateLimitConfig: &graw.RateLimitConfig{
			RequestsPerMinute: 100000, // Effectively disable rate limiting for tests
			Burst:             1000,
		},
	}

	// Create client - note: this will fail auth if mock server doesn't handle it
	// In real tests, you should set up the mock server to handle auth requests
	client, err := graw.NewClient(config)
	if err != nil {
		// Return a partially constructed client for tests that need to inject mocks
		panic("NewTestClient requires mock server to handle auth endpoint: " + err.Error())
	}

	return client
}

// NewTestClientWithMocks creates a Reddit client with custom HTTP client and auth provider.
// This is a lower-level factory that gives you full control over the client's dependencies.
// Use this when you need specific mock behavior or when testing error conditions.
//
// Example:
//
//	mockHTTP := &MockHTTPClient{
//	    DoFunc: func(req *http.Request, v *types.Thing) error {
//	        // Custom mock behavior
//	        return nil
//	    },
//	}
//	mockAuth := &testutil.MockTokenProvider{Token: "custom-token"}
//	client := testutil.NewTestClientWithMocks(mockHTTP, mockAuth)
func NewTestClientWithMocks(httpClient graw.HTTPClient, auth graw.TokenProvider) *graw.Reddit {
	// Note: This function signature expects internal interfaces that may not be exportable.
	// For now, we document it but may need to revisit the implementation based on
	// actual package structure and visibility requirements.
	panic("NewTestClientWithMocks: not yet implemented - requires access to internal Reddit constructor")
}

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

// MockHTTPClient is a mock implementation of the HTTPClient interface for testing.
// It allows tests to control the behavior of HTTP operations without making real network calls.
//
// Example:
//
//	mock := &testutil.MockHTTPClient{
//	    NewRequestFunc: func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
//	        return http.NewRequestWithContext(ctx, method, "http://example.com/"+path, body)
//	    },
//	    DoFunc: func(req *http.Request, v *types.Thing) error {
//	        *v = types.Thing{Kind: "Listing", Data: json.RawMessage(`{"children":[]}`)}
//	        return nil
//	    },
//	}
type MockHTTPClient struct {
	// NewRequestFunc is called by NewRequest
	NewRequestFunc func(ctx context.Context, method, path string, body interface{}, params ...interface{}) (*http.Request, error)
	// DoFunc is called by Do
	DoFunc func(req *http.Request, v *types.Thing) error
	// DoThingArrayFunc is called by DoThingArray
	DoThingArrayFunc func(req *http.Request) ([]*types.Thing, error)
	// DoMoreChildrenFunc is called by DoMoreChildren
	DoMoreChildrenFunc func(req *http.Request) ([]*types.Thing, error)
}

// Note: The actual method signatures for MockHTTPClient would need to match
// the exact interface definition from the main package. The above is a template
// that should be adjusted based on the actual HTTPClient interface.
