package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Parser handles parsing of Reddit API responses
type Parser struct {
	logger *slog.Logger
}

// NewParser creates a new parser instance with an optional logger.
// If logger is nil, parse errors will not be logged.
func NewParser(logger ...*slog.Logger) *Parser {
	var log *slog.Logger
	if len(logger) > 0 {
		log = logger[0]
	}
	return &Parser{logger: log}
}

// ParseThing determines the type of a Thing and returns the appropriate typed struct.
func (p *Parser) ParseThing(thing *types.Thing) (interface{}, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}

	switch thing.Kind {
	case "Listing":
		return p.ParseListing(thing)
	case "t1":
		return p.ParseComment(thing)
	case "t2":
		return p.ParseAccount(thing)
	case "t3":
		return p.ParsePost(thing)
	case "t4":
		return p.ParseMessage(thing)
	case "t5":
		return p.ParseSubreddit(thing)
	case "more":
		return p.ParseMore(thing)
	default:
		return nil, fmt.Errorf("unknown kind: %s", thing.Kind)
	}
}

// ParseListing extracts a ListingData from a Thing of kind "Listing".
func (p *Parser) ParseListing(thing *types.Thing) (*types.ListingData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "Listing" {
		return nil, fmt.Errorf("expected Listing, got %s", thing.Kind)
	}

	var result types.ListingData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Listing data: %w", err)
	}

	return &result, nil
}

// ParsePost extracts a Post from a Thing of kind "t3".
func (p *Parser) ParsePost(thing *types.Thing) (*types.Post, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t3" {
		return nil, fmt.Errorf("expected t3 (Post), got %s", thing.Kind)
	}

	var result types.Post
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Post data: %w", err)
	}

	return &result, nil
}

// ParseComment extracts a Comment from a Thing of kind "t1".
func (p *Parser) ParseComment(thing *types.Thing) (*types.Comment, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t1" {
		return nil, fmt.Errorf("expected t1 (Comment), got %s", thing.Kind)
	}

	var result types.Comment
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Comment data: %w", err)
	}

	// Handle the replies field which can be a Listing object or an empty string
	var rawData struct {
		Replies json.RawMessage `json:"replies"`
	}
	if err := json.Unmarshal(thing.Data, &rawData); err == nil && len(rawData.Replies) > 0 {
		// Check if it's an empty string (Reddit sends "" when there are no replies)
		if string(rawData.Replies) != `""` {
			// Parse the replies Thing - already have the raw JSON, no new buffer needed
			var repliesThing types.Thing
			if err := json.Unmarshal(rawData.Replies, &repliesThing); err == nil {
				replies, moreIDs, err := p.ExtractComments(&repliesThing)
				if err == nil {
					result.Replies = replies
					if len(moreIDs) > 0 {
						result.MoreChildrenIDs = append(result.MoreChildrenIDs, moreIDs...)
					}
				}
			}
		}
	}

	return &result, nil
}

// ParseSubreddit extracts a SubredditData from a Thing of kind "t5".
func (p *Parser) ParseSubreddit(thing *types.Thing) (*types.SubredditData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t5" {
		return nil, fmt.Errorf("expected t5 (Subreddit), got %s", thing.Kind)
	}

	var result types.SubredditData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Subreddit data: %w", err)
	}

	return &result, nil
}

// ParseAccount extracts an AccountData from a Thing of kind "t2".
func (p *Parser) ParseAccount(thing *types.Thing) (*types.AccountData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t2" {
		return nil, fmt.Errorf("expected t2 (Account), got %s", thing.Kind)
	}

	var result types.AccountData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Account data: %w", err)
	}

	return &result, nil
}

// ParseMessage extracts a MessageData from a Thing of kind "t4".
func (p *Parser) ParseMessage(thing *types.Thing) (*types.MessageData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t4" {
		return nil, fmt.Errorf("expected t4 (Message), got %s", thing.Kind)
	}

	var result types.MessageData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Message data: %w", err)
	}

	return &result, nil
}

// ParseMore extracts a MoreData from a Thing of kind "more".
func (p *Parser) ParseMore(thing *types.Thing) (*types.MoreData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "more" {
		return nil, fmt.Errorf("expected more, got %s", thing.Kind)
	}

	var result types.MoreData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse More data: %w", err)
	}

	return &result, nil
}

// ExtractPosts extracts all Post objects from a listing Thing.
func (p *Parser) ExtractPosts(thing *types.Thing) ([]*types.Post, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "Listing" {
		return nil, fmt.Errorf("expected Listing, got %s", thing.Kind)
	}

	listingData, err := p.ParseListing(thing)
	if err != nil {
		return nil, err
	}

	posts := make([]*types.Post, 0, len(listingData.Children))
	for _, child := range listingData.Children {
		if child.Kind == "t3" {
			post, err := p.ParsePost(child)
			if err != nil {
				// Log parse error if logger is available
				if p.logger != nil {
					p.logger.LogAttrs(context.Background(), slog.LevelWarn, "failed to parse post",
						slog.String("error", err.Error()),
						slog.String("kind", child.Kind))
				}
				continue // Skip unparseable posts
			}
			posts = append(posts, post)
		}
	}

	return posts, nil
}

// ExtractComments recursively extracts all comments from a comment tree.
func (p *Parser) ExtractComments(thing *types.Thing) ([]*types.Comment, []string, error) {
	comments := make([]*types.Comment, 0)
	moreIDs := make([]string, 0)

	// Handle both single comments and listings
	if thing.Kind == "t1" {
		comment, err := p.ParseComment(thing)
		if err != nil {
			return nil, nil, err
		}
		comments = append(comments, comment)

		// Replies are already parsed in ParseComment, just add them
		if comment.Replies != nil {
			comments = append(comments, comment.Replies...)
		}
		if len(comment.MoreChildrenIDs) > 0 {
			moreIDs = append(moreIDs, comment.MoreChildrenIDs...)
		}
		return comments, moreIDs, nil
	}

	// Handle listing of comments
	if thing.Kind != "Listing" {
		return nil, nil, fmt.Errorf("expected Listing or t1, got %s", thing.Kind)
	}

	listingData, err := p.ParseListing(thing)
	if err != nil {
		return nil, nil, err
	}

	for _, child := range listingData.Children {
		switch child.Kind {
		case "t1":
			comment, err := p.ParseComment(child)
			if err != nil {
				// Log parse error if logger is available
				if p.logger != nil {
					p.logger.LogAttrs(context.Background(), slog.LevelWarn, "failed to parse comment",
						slog.String("error", err.Error()),
						slog.String("kind", child.Kind))
				}
				continue // Skip unparseable comments
			}

			comments = append(comments, comment)

			// Replies are already parsed in ParseComment, just add them
			if comment.Replies != nil {
				comments = append(comments, comment.Replies...)
			}
			if len(comment.MoreChildrenIDs) > 0 {
				moreIDs = append(moreIDs, comment.MoreChildrenIDs...)
			}
		case "more":
			more, err := p.ParseMore(child)
			if err != nil {
				continue
			}
			moreIDs = append(moreIDs, more.Children...)
		}
	}

	return comments, moreIDs, nil
}

// ExtractPostAndComments parses the typical response from GetComments which contains
// [post_listing, comments_listing]. Returns the post, comments, more IDs, pagination info, and error.
func (p *Parser) ExtractPostAndComments(response []*types.Thing) (*types.Post, []*types.Comment, []string, string, string, error) {
	if len(response) == 0 {
		return nil, nil, nil, "", "", fmt.Errorf("empty response")
	}

	// Reddit can return either:
	// 1. Two listings: [post_listing, comments_listing]
	// 2. One listing with just comments (when fetching comments for a specific post)

	if len(response) >= 2 {
		// Standard format: first is post, second is comments
		var post *types.Post
		posts, err := p.ExtractPosts(response[0])
		if err == nil && len(posts) > 0 {
			post = posts[0]
		}
		// Even if post extraction fails, try to extract comments

		// Second element should be the comments listing - extract pagination info
		var after, before string
		if response[1] != nil && response[1].Kind == "Listing" {
			listingData, err := p.ParseListing(response[1])
			if err == nil {
				after = listingData.AfterFullname
				before = listingData.BeforeFullname
			}
		}

		// Extract comments from the listing
		comments, moreIDs, err := p.ExtractComments(response[1])
		if err != nil {
			// If we have a post but no comments, return the post
			if post != nil {
				return post, nil, nil, after, before, fmt.Errorf("failed to extract comments: %w", err)
			}
			// If we have neither post nor comments, return error
			return nil, nil, nil, "", "", fmt.Errorf("failed to extract both post and comments")
		}

		// Return whatever we successfully extracted (post might be nil)
		return post, comments, moreIDs, after, before, nil
	}

	// Single listing format: just comments, no post
	// This happens when fetching additional comments or in certain API responses
	var after, before string
	if response[0] != nil && response[0].Kind == "Listing" {
		listingData, err := p.ParseListing(response[0])
		if err == nil {
			after = listingData.AfterFullname
			before = listingData.BeforeFullname
		}
	}

	comments, moreIDs, err := p.ExtractComments(response[0])
	if err != nil {
		// Try to extract as posts instead (might be a post-only response)
		posts, err := p.ExtractPosts(response[0])
		if err != nil || len(posts) == 0 {
			return nil, nil, nil, "", "", fmt.Errorf("failed to extract data from single listing: %w", err)
		}
		return posts[0], nil, nil, "", "", nil
	}

	// Return nil post with the comments
	return nil, comments, moreIDs, after, before, nil
}
