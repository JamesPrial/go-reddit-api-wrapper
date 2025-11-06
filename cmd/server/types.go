package main

import "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"

// ErrorResponse represents a standard API error response.
type ErrorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
	Details   string `json:"details,omitempty"`
}

// HealthResponse represents the response for the health check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// PostData represents a post in API responses.
type PostData struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Subreddit   string  `json:"subreddit"`
	SelfText    string  `json:"selftext"`
	Created     float64 `json:"created_utc"`
	UpvoteRatio float64 `json:"upvote_ratio"`
	IsSelf      bool    `json:"is_self"`
	Over18      bool    `json:"over_18"`
	Stickied    bool    `json:"stickied"`
	Locked      bool    `json:"locked"`
}

// CommentData represents a comment in API responses.
type CommentData struct {
	ID        string  `json:"id"`
	Author    string  `json:"author"`
	Body      string  `json:"body"`
	Score     int     `json:"score"`
	Created   float64 `json:"created_utc"`
	Subreddit string  `json:"subreddit"`
	ParentID  string  `json:"parent_id"`
	Depth     int     `json:"depth"`
	Edited    bool    `json:"edited"`
}

// PostsResponse represents a collection of posts with pagination.
type PostsResponse struct {
	Posts  []*PostData `json:"posts"`
	After  string      `json:"after,omitempty"`
	Before string      `json:"before,omitempty"`
	Count  int         `json:"count"`
	Total  int         `json:"total,omitempty"`
}

// CommentsResponse represents a collection of comments with pagination.
type CommentsResponse struct {
	Post     *PostData      `json:"post"`
	Comments []*CommentData `json:"comments"`
	After    string         `json:"after,omitempty"`
	Before   string         `json:"before,omitempty"`
	Count    int            `json:"count"`
	Total    int            `json:"total,omitempty"`
}

// UserResponse represents authenticated user information.
type UserResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	LinkKarma    int     `json:"link_karma"`
	CommentKarma int     `json:"comment_karma"`
	Created      float64 `json:"created_utc"`
	IsModerator  bool    `json:"is_moderator"`
	IsGold       bool    `json:"is_gold"`
}

// SubredditResponse represents subreddit information.
type SubredditResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Subscribers int64  `json:"subscribers"`
	Over18      bool   `json:"over_18"`
}

// convertPost converts a types.Post to a PostData.
func convertPost(post *types.Post) *PostData {
	if post == nil {
		return nil
	}
	return &PostData{
		ID:          post.ID,
		Title:       post.Title,
		Author:      post.Author,
		Score:       post.Score,
		NumComments: post.NumComments,
		URL:         post.URL,
		Permalink:   post.Permalink,
		Subreddit:   post.Subreddit,
		SelfText:    post.SelfText,
		Created:     post.CreatedUTC,
		UpvoteRatio: post.UpvoteRatio,
		IsSelf:      post.IsSelf,
		Over18:      post.Over18,
		Stickied:    post.Stickied,
		Locked:      post.Locked,
	}
}

// convertComment converts a types.Comment to a CommentData.
func convertComment(comment *types.Comment) *CommentData {
	if comment == nil {
		return nil
	}
	return &CommentData{
		ID:        comment.ID,
		Author:    comment.Author,
		Body:      comment.Body,
		Score:     comment.Score,
		Created:   comment.CreatedUTC,
		Subreddit: comment.Subreddit,
		ParentID:  comment.ParentID,
		Edited:    comment.Edited.IsEdited,
	}
}

// convertAccountData converts types.AccountData to UserResponse.
func convertAccountData(account *types.AccountData) *UserResponse {
	if account == nil {
		return nil
	}
	return &UserResponse{
		ID:           account.ID,
		Name:         account.Name,
		LinkKarma:    account.LinkKarma,
		CommentKarma: account.CommentKarma,
		Created:      account.CreatedUTC,
		IsModerator:  account.IsMod,
		IsGold:       account.IsGold,
	}
}

// convertSubredditData converts types.SubredditData to SubredditResponse.
func convertSubredditData(subreddit *types.SubredditData) *SubredditResponse {
	if subreddit == nil {
		return nil
	}
	return &SubredditResponse{
		Name:        subreddit.Name,
		DisplayName: subreddit.DisplayName,
		Title:       subreddit.Title,
		Description: subreddit.Description,
		Subscribers: subreddit.Subscribers,
		Over18:      subreddit.Over18,
	}
}
