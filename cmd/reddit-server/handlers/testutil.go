package handlers

import (
	"context"
	"fmt"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// MockRedditClient is a mock implementation of the Reddit client for testing.
type MockRedditClient struct {
	MeFn                  func(ctx context.Context) (*types.AccountData, error)
	GetSubredditFn        func(ctx context.Context, name string) (*types.SubredditData, error)
	GetHotFn              func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)
	GetNewFn              func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)
	GetCommentsFn         func(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error)
	GetMoreCommentsFn     func(ctx context.Context, req *types.MoreCommentsRequest) ([]*types.Comment, error)
	GetCommentsMultipleFn func(ctx context.Context, reqs ...*types.CommentsRequest) ([]*types.CommentsResponse, error)
}

// Me calls the mock Me function.
func (m *MockRedditClient) Me(ctx context.Context) (*types.AccountData, error) {
	if m.MeFn != nil {
		return m.MeFn(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

// GetSubreddit calls the mock GetSubreddit function.
func (m *MockRedditClient) GetSubreddit(ctx context.Context, name string) (*types.SubredditData, error) {
	if m.GetSubredditFn != nil {
		return m.GetSubredditFn(ctx, name)
	}
	return nil, fmt.Errorf("not implemented")
}

// GetHot calls the mock GetHot function.
func (m *MockRedditClient) GetHot(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
	if m.GetHotFn != nil {
		return m.GetHotFn(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

// GetNew calls the mock GetNew function.
func (m *MockRedditClient) GetNew(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
	if m.GetNewFn != nil {
		return m.GetNewFn(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

// GetComments calls the mock GetComments function.
func (m *MockRedditClient) GetComments(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error) {
	if m.GetCommentsFn != nil {
		return m.GetCommentsFn(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

// GetMoreComments calls the mock GetMoreComments function.
func (m *MockRedditClient) GetMoreComments(ctx context.Context, req *types.MoreCommentsRequest) ([]*types.Comment, error) {
	if m.GetMoreCommentsFn != nil {
		return m.GetMoreCommentsFn(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

// GetCommentsMultiple calls the mock GetCommentsMultiple function.
func (m *MockRedditClient) GetCommentsMultiple(ctx context.Context, reqs ...*types.CommentsRequest) ([]*types.CommentsResponse, error) {
	if m.GetCommentsMultipleFn != nil {
		return m.GetCommentsMultipleFn(ctx, reqs...)
	}
	return nil, fmt.Errorf("not implemented")
}

// TestDataBuilder provides helper methods for creating test data.
type TestDataBuilder struct {
	t *testing.T
}

// NewTestDataBuilder creates a new test data builder.
func NewTestDataBuilder(t *testing.T) *TestDataBuilder {
	return &TestDataBuilder{t: t}
}

// BuildAccountData creates a test AccountData object.
func (b *TestDataBuilder) BuildAccountData() *types.AccountData {
	return &types.AccountData{
		ThingData: types.ThingData{
			ID:   "test-user-id",
			Name: "t2_testuser",
		},
		Created: types.Created{
			Created:    1234567890,
			CreatedUTC: 1234567890,
		},
		CommentKarma: 5000,
		LinkKarma:    9999,
		IsMod:        false,
		IsGold:       false,
		Over18:       false,
	}
}

// BuildSubreddit creates a test SubredditData object.
func (b *TestDataBuilder) BuildSubreddit(name string) *types.SubredditData {
	return &types.SubredditData{
		ThingData: types.ThingData{
			ID:   "test-sub-id",
			Name: "t5_" + name,
		},
		DisplayName:    name,
		Title:          "Test Subreddit: " + name,
		Description:    "A test subreddit for " + name,
		Subscribers:    10000,
		PublicTraffic:  true,
		Over18:         false,
		AccountsActive: 1000,
		SubredditType:  "public",
	}
}

// BuildPost creates a test Post object.
func (b *TestDataBuilder) BuildPost(id string) *types.Post {
	return &types.Post{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t3_" + id,
		},
		Votable: types.Votable{
			Score: 100,
			Ups:   100,
			Downs: 0,
		},
		Created: types.Created{
			Created:    1234567890,
			CreatedUTC: 1234567890,
		},
		Title:       "Test Post " + id,
		Subreddit:   "golang",
		Author:      "testuser",
		NumComments: 50,
		URL:         "https://example.com/" + id,
		SelfText:    "This is a test post",
		IsSelf:      true,
	}
}

// BuildComment creates a test Comment object.
func (b *TestDataBuilder) BuildComment(id string) *types.Comment {
	return &types.Comment{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t1_" + id,
		},
		Votable: types.Votable{
			Score: 10,
			Ups:   10,
			Downs: 0,
		},
		Created: types.Created{
			Created:    1234567890,
			CreatedUTC: 1234567890,
		},
		Author: "testuser",
		Body:   "This is a test comment",
		Edited: types.Edited{IsEdited: false},
		Gilded: 0,
	}
}

// BuildPostsResponse creates a test PostsResponse.
func (b *TestDataBuilder) BuildPostsResponse(count int) *types.PostsResponse {
	posts := make([]*types.Post, count)
	for i := 0; i < count; i++ {
		posts[i] = b.BuildPost(fmt.Sprintf("post%d", i))
	}
	return &types.PostsResponse{
		Posts:          posts,
		AfterFullname:  "t3_after123",
		BeforeFullname: "t3_before123",
	}
}

// BuildCommentsResponse creates a test CommentsResponse.
func (b *TestDataBuilder) BuildCommentsResponse(commentCount int) *types.CommentsResponse {
	comments := make([]*types.Comment, commentCount)
	for i := 0; i < commentCount; i++ {
		comments[i] = b.BuildComment(fmt.Sprintf("comment%d", i))
	}
	return &types.CommentsResponse{
		Post:           b.BuildPost("test-post"),
		Comments:       comments,
		AfterFullname:  "t1_after123",
		BeforeFullname: "t1_before123",
	}
}
