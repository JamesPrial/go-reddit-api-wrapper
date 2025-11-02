// Package commands provides command handlers for the Reddit CLI.
package commands

import (
	"context"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// GetSubreddit fetches subreddit information and formats it for display.
// It retrieves basic information about a subreddit including subscriber count,
// description, and other metadata.
func GetSubreddit(ctx context.Context, client *graw.Reddit, name string, formatter output.Formatter) error {
	if name == "" {
		return fmt.Errorf("subreddit name cannot be empty")
	}

	subreddit, err := client.GetSubreddit(ctx, name)
	if err != nil {
		return err
	}

	if subreddit == nil {
		return fmt.Errorf("subreddit not found or returned nil")
	}

	return formatter.FormatSubreddit(subreddit)
}

// GetHotPosts fetches hot posts from a subreddit and formats them for display.
// Hot posts are the currently trending posts on the subreddit.
// If subreddit is empty, fetches hot posts from the front page.
// Supports pagination via the Pagination parameter.
func GetHotPosts(ctx context.Context, client *graw.Reddit, subreddit string, pagination types.Pagination, formatter output.Formatter) error {
	request := &types.PostsRequest{
		Subreddit:  subreddit,
		Pagination: pagination,
	}

	response, err := client.GetHot(ctx, request)
	if err != nil {
		return err
	}

	if response == nil || len(response.Posts) == 0 {
		fmt.Println("No posts found")
		return nil
	}

	return formatter.FormatPosts(response.Posts)
}

// GetNewPosts fetches new posts from a subreddit and formats them for display.
// New posts are the most recently submitted posts on the subreddit.
// If subreddit is empty, fetches new posts from the front page.
// Supports pagination via the Pagination parameter.
func GetNewPosts(ctx context.Context, client *graw.Reddit, subreddit string, pagination types.Pagination, formatter output.Formatter) error {
	request := &types.PostsRequest{
		Subreddit:  subreddit,
		Pagination: pagination,
	}

	response, err := client.GetNew(ctx, request)
	if err != nil {
		return err
	}

	if response == nil || len(response.Posts) == 0 {
		fmt.Println("No posts found")
		return nil
	}

	return formatter.FormatPosts(response.Posts)
}
