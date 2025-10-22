// Package testutil provides reusable test helpers and assertions for the Reddit API wrapper.
package testutil

import (
	"context"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// MockParser is a mock implementation of the Parser interface for testing.
// It provides configurable behavior for all parsing operations with sensible defaults.
//
// By default, all methods return empty/nil values representing successful parsing
// with no data. Tests can override specific methods by setting the corresponding
// function fields.
//
// Example:
//
//	// Use default behavior (returns nil/empty)
//	mock := &testutil.MockParser{}
//	result, err := mock.ParseThing(ctx, thing) // returns nil, nil
//
//	// Customize specific methods
//	mock := &testutil.MockParser{
//		ExtractPostsFunc: func(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
//			return []*types.Post{testutil.DefaultPost()}, nil
//		},
//	}
//
//	// Use helper methods for common scenarios
//	mock := testutil.NewMockParser().WithPosts([]*types.Post{testutil.DefaultPost()})
type MockParser struct {
	// ParseThingFunc is called by ParseThing if set. If nil, returns nil, nil.
	ParseThingFunc func(ctx context.Context, thing *types.Thing) (any, error)

	// ExtractPostsFunc is called by ExtractPosts if set. If nil, returns nil, nil.
	ExtractPostsFunc func(ctx context.Context, thing *types.Thing) ([]*types.Post, error)

	// ExtractPostAndCommentsFunc is called by ExtractPostAndComments if set.
	// If nil, returns an empty CommentsResponse with no error.
	ExtractPostAndCommentsFunc func(ctx context.Context, things []*types.Thing) (*types.CommentsResponse, error)
}

// NewMockParser creates a new MockParser with default behavior.
// All methods return empty/nil values unless customized.
func NewMockParser() *MockParser {
	return &MockParser{}
}

// ParseThing parses a Reddit Thing. If ParseThingFunc is set, it delegates to that function.
// Otherwise, returns nil, nil.
func (m *MockParser) ParseThing(ctx context.Context, thing *types.Thing) (any, error) {
	if m.ParseThingFunc != nil {
		return m.ParseThingFunc(ctx, thing)
	}
	return nil, nil
}

// ExtractPosts extracts posts from a Thing. If ExtractPostsFunc is set, it delegates to that function.
// Otherwise, returns nil, nil.
func (m *MockParser) ExtractPosts(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
	if m.ExtractPostsFunc != nil {
		return m.ExtractPostsFunc(ctx, thing)
	}
	return nil, nil
}

// ExtractPostAndComments extracts a post and comments. If ExtractPostAndCommentsFunc is set,
// it delegates to that function. Otherwise, returns an empty CommentsResponse with no error.
func (m *MockParser) ExtractPostAndComments(ctx context.Context, things []*types.Thing) (*types.CommentsResponse, error) {
	if m.ExtractPostAndCommentsFunc != nil {
		return m.ExtractPostAndCommentsFunc(ctx, things)
	}
	return &types.CommentsResponse{}, nil
}

// Helper methods for common test scenarios

// WithParseThingFunc sets the ParseThingFunc and returns the mock for method chaining.
//
// Example:
//
//	mock := NewMockParser().WithParseThingFunc(func(ctx context.Context, thing *types.Thing) (any, error) {
//		return &types.Post{Title: "Test"}, nil
//	})
func (m *MockParser) WithParseThingFunc(fn func(ctx context.Context, thing *types.Thing) (any, error)) *MockParser {
	m.ParseThingFunc = fn
	return m
}

// WithExtractPostsFunc sets the ExtractPostsFunc and returns the mock for method chaining.
//
// Example:
//
//	mock := NewMockParser().WithExtractPostsFunc(func(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
//		return []*types.Post{testutil.DefaultPost()}, nil
//	})
func (m *MockParser) WithExtractPostsFunc(fn func(ctx context.Context, thing *types.Thing) ([]*types.Post, error)) *MockParser {
	m.ExtractPostsFunc = fn
	return m
}

// WithExtractPostAndCommentsFunc sets the ExtractPostAndCommentsFunc and returns the mock for method chaining.
//
// Example:
//
//	mock := NewMockParser().WithExtractPostAndCommentsFunc(func(ctx context.Context, things []*types.Thing) (*types.CommentsResponse, error) {
//		return &types.CommentsResponse{Post: testutil.DefaultPost()}, nil
//	})
func (m *MockParser) WithExtractPostAndCommentsFunc(fn func(ctx context.Context, things []*types.Thing) (*types.CommentsResponse, error)) *MockParser {
	m.ExtractPostAndCommentsFunc = fn
	return m
}

// WithPosts configures ExtractPosts to return the given posts.
// This is useful for testing successful post listing responses.
//
// Example:
//
//	mock := NewMockParser().WithPosts([]*types.Post{
//		testutil.DefaultPost(),
//	})
func (m *MockParser) WithPosts(posts []*types.Post) *MockParser {
	m.ExtractPostsFunc = func(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
		return posts, nil
	}
	return m
}

// WithCommentsResponse configures ExtractPostAndComments to return the given response.
// This is useful for testing successful comments endpoint responses.
//
// Example:
//
//	mock := NewMockParser().WithCommentsResponse(&types.CommentsResponse{
//		Post: testutil.DefaultPost(),
//		Comments: []*types.Comment{testutil.DefaultComment()},
//	})
func (m *MockParser) WithCommentsResponse(response *types.CommentsResponse) *MockParser {
	m.ExtractPostAndCommentsFunc = func(ctx context.Context, things []*types.Thing) (*types.CommentsResponse, error) {
		return response, nil
	}
	return m
}

// WithError configures all methods to return the given error.
// This is useful for testing error handling paths.
//
// Example:
//
//	mock := NewMockParser().WithError(errors.New("parse error"))
func (m *MockParser) WithError(err error) *MockParser {
	m.ParseThingFunc = func(ctx context.Context, thing *types.Thing) (any, error) {
		return nil, err
	}
	m.ExtractPostsFunc = func(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
		return nil, err
	}
	m.ExtractPostAndCommentsFunc = func(ctx context.Context, things []*types.Thing) (*types.CommentsResponse, error) {
		return nil, err
	}
	return m
}
