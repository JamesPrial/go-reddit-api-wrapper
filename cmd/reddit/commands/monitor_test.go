package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// Testing Note:
//
// The MonitorSubreddits function currently accepts a concrete *graw.Reddit type
// instead of an interface, which makes it difficult to test with mocks without
// making actual Reddit API calls.
//
// To make this function fully testable, we recommend refactoring it to accept
// an interface that defines the methods it needs:
//
//   type RedditMonitorClient interface {
//       GetNew(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)
//       GetComments(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error)
//   }
//
// This follows the dependency injection pattern used elsewhere in the codebase
// (see reddit/reddit.go for examples of HTTPClient, TokenProvider, etc.).
//
// Until that refactoring is complete, these tests focus on validation and
// error handling that can be tested without a mock client.

// TestMonitorSubreddits_ValidationErrors tests input validation.
func TestMonitorSubreddits_ValidationErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create a minimal mock storage that satisfies the interface
	mockStorage := &minimalMockStore{}

	tests := []struct {
		name        string
		client      *graw.Reddit
		subreddits  []string
		interval    time.Duration
		limit       int
		store       storage.Store
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil client",
			client:      nil,
			subreddits:  []string{"golang"},
			interval:    time.Second,
			limit:       10,
			store:       mockStorage,
			wantErr:     true,
			errContains: "client cannot be nil",
		},
		{
			name:        "empty subreddits list",
			client:      &graw.Reddit{}, // Use non-nil client to test subreddit validation
			subreddits:  []string{},
			interval:    time.Second,
			limit:       10,
			store:       mockStorage,
			wantErr:     true,
			errContains: "subreddits list cannot be empty",
		},
		{
			name:        "nil subreddits list",
			client:      &graw.Reddit{}, // Use non-nil client to test subreddit validation
			subreddits:  nil,
			interval:    time.Second,
			limit:       10,
			store:       mockStorage,
			wantErr:     true,
			errContains: "subreddits list cannot be empty",
		},
		{
			name:        "zero interval",
			client:      &graw.Reddit{}, // Use non-nil client to test interval validation
			subreddits:  []string{"golang"},
			interval:    0,
			limit:       10,
			store:       mockStorage,
			wantErr:     true,
			errContains: "interval must be greater than 0",
		},
		{
			name:        "negative interval",
			client:      &graw.Reddit{}, // Use non-nil client to test interval validation
			subreddits:  []string{"golang"},
			interval:    -1 * time.Second,
			limit:       10,
			store:       mockStorage,
			wantErr:     true,
			errContains: "interval must be greater than 0",
		},
		{
			name:        "nil store",
			client:      &graw.Reddit{}, // Use non-nil client to test store validation
			subreddits:  []string{"golang"},
			interval:    time.Second,
			limit:       10,
			store:       nil,
			wantErr:     true,
			errContains: "store cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All validation tests should fail immediately without making API calls
			err := MonitorSubreddits(ctx, tt.client, tt.subreddits, tt.interval, tt.limit, true, tt.store)

			if !tt.wantErr {
				t.Fatalf("expected no error, got: %v", err)
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

// TestIsFatalError tests the fatal error classification logic.
func TestIsFatalError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantVal bool
	}{
		{
			name:    "nil error",
			err:     nil,
			wantVal: false,
		},
		{
			name:    "auth error is fatal",
			err:     &graw.AuthError{Message: "unauthorized"},
			wantVal: true,
		},
		{
			name:    "validation error is fatal",
			err:     &graw.ValidationError{Field: "subreddit", Reason: "invalid"},
			wantVal: true,
		},
		{
			name:    "config error is fatal",
			err:     &graw.ConfigError{Field: "client_id", Message: "missing"},
			wantVal: true,
		},
		{
			name:    "rate limit error is not fatal",
			err:     &graw.RateLimitError{Reason: "too_many_requests", WaitDuration: time.Second},
			wantVal: false,
		},
		{
			name:    "network error is not fatal",
			err:     &graw.NetworkError{Method: "GET", URL: "https://reddit.com", Err: errors.New("timeout")},
			wantVal: false,
		},
		{
			name:    "API error is not fatal",
			err:     &graw.APIError{StatusCode: 500, Message: "server error"},
			wantVal: false,
		},
		{
			name:    "parse error is not fatal",
			err:     &graw.ParseError{Operation: "unmarshal", Err: errors.New("invalid json")},
			wantVal: false,
		},
		{
			name:    "generic error is not fatal",
			err:     errors.New("some other error"),
			wantVal: false,
		},
		{
			name:    "context canceled is not fatal",
			err:     context.Canceled,
			wantVal: false,
		},
		{
			name:    "context deadline exceeded is not fatal",
			err:     context.DeadlineExceeded,
			wantVal: false,
		},
		{
			name:    "wrapped auth error is fatal",
			err:     fmt.Errorf("failed to fetch: %w", &graw.AuthError{Message: "unauthorized"}),
			wantVal: true,
		},
		{
			name:    "wrapped validation error is fatal",
			err:     fmt.Errorf("request failed: %w", &graw.ValidationError{Field: "limit", Reason: "too large"}),
			wantVal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFatalError(tt.err)
			if got != tt.wantVal {
				t.Errorf("isFatalError(%v) = %v, want %v", tt.err, got, tt.wantVal)
			}
		})
	}
}

// minimalMockStore is a minimal implementation of storage.Store for validation testing.
// It implements all required methods but does nothing.
type minimalMockStore struct{}

func (m *minimalMockStore) UpsertPost(ctx context.Context, post *types.Post) error {
	return nil
}

func (m *minimalMockStore) UpsertPosts(ctx context.Context, posts []*types.Post) error {
	return nil
}

func (m *minimalMockStore) GetPost(ctx context.Context, id string) (*types.Post, error) {
	return nil, nil
}

func (m *minimalMockStore) ListPosts(ctx context.Context, opts *storage.ListPostsOptions) ([]*types.Post, error) {
	return nil, nil
}

func (m *minimalMockStore) CountPosts(ctx context.Context, opts *storage.ListPostsOptions) (int64, error) {
	return 0, nil
}

func (m *minimalMockStore) DeletePost(ctx context.Context, id string) error {
	return nil
}

func (m *minimalMockStore) UpsertComment(ctx context.Context, comment *types.Comment) error {
	return nil
}

func (m *minimalMockStore) UpsertComments(ctx context.Context, comments []*types.Comment) error {
	return nil
}

func (m *minimalMockStore) GetComment(ctx context.Context, id string) (*types.Comment, error) {
	return nil, nil
}

func (m *minimalMockStore) GetCommentTree(ctx context.Context, postID string, opts *storage.CommentTreeOptions) ([]*types.Comment, error) {
	return nil, nil
}

func (m *minimalMockStore) DeleteComment(ctx context.Context, id string) error {
	return nil
}

func (m *minimalMockStore) Close() error {
	return nil
}

func (m *minimalMockStore) Ping(ctx context.Context) error {
	return nil
}

func (m *minimalMockStore) GetStats(ctx context.Context) (*storage.CacheStats, error) {
	return nil, nil
}

func (m *minimalMockStore) EvictStale(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}
