package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// MaxCommentDepth is the maximum depth of nested comments to prevent stack overflow attacks
const MaxCommentDepth = 50

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

// ParseComment extracts a Comment from a Thing of kind "t1" and builds a proper tree structure.
// The Replies field will contain only direct children, and each child will have its own Replies.
func (p *Parser) ParseComment(thing *types.Thing) (*types.Comment, error) {
	return p.parseCommentWithDepth(thing, 0)
}

// parseCommentWithDepth is the internal implementation that tracks recursion depth
func (p *Parser) parseCommentWithDepth(thing *types.Thing, depth int) (*types.Comment, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t1" {
		return nil, fmt.Errorf("expected t1 (Comment), got %s", thing.Kind)
	}

	// Prevent stack overflow from deeply nested comments
	if depth > MaxCommentDepth {
		return nil, fmt.Errorf("comment tree depth exceeds maximum of %d", MaxCommentDepth)
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
			// Parse the replies Thing
			var repliesThing types.Thing
			if err := json.Unmarshal(rawData.Replies, &repliesThing); err == nil {
				// Parse only direct children to maintain tree structure
				if repliesThing.Kind == "Listing" {
					listingData, err := p.ParseListing(&repliesThing)
					if err == nil {
						// Process each direct child
						for _, child := range listingData.Children {
							switch child.Kind {
							case "t1":
								// Recursively parse child comment with incremented depth
								childComment, err := p.parseCommentWithDepth(child, depth+1)
								if err == nil {
									result.Replies = append(result.Replies, childComment)
								}
							case "more":
								// Collect "more" IDs for deferred loading
								more, err := p.ParseMore(child)
								if err == nil {
									result.MoreChildrenIDs = append(result.MoreChildrenIDs, more.Children...)
								}
							}
						}
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

// ExtractComments extracts top-level comments from a Listing or single comment Thing.
// Returns comments with proper tree structure (each comment has its Replies populated).
// Also returns all "more" IDs found at any level in the tree for deferred loading.
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
		// Collect more IDs from the entire tree
		moreIDs = append(moreIDs, p.collectMoreIDs(comment)...)
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
			// Collect more IDs from the entire tree
			moreIDs = append(moreIDs, p.collectMoreIDs(comment)...)
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

// collectMoreIDs recursively collects all MoreChildrenIDs from a comment tree.
func (p *Parser) collectMoreIDs(comment *types.Comment) []string {
	return p.collectMoreIDsWithDepth(comment, 0)
}

// collectMoreIDsWithDepth is the internal implementation that tracks recursion depth
func (p *Parser) collectMoreIDsWithDepth(comment *types.Comment, depth int) []string {
	moreIDs := make([]string, 0)

	// Prevent stack overflow from deeply nested comments
	if depth > MaxCommentDepth {
		return moreIDs
	}

	if len(comment.MoreChildrenIDs) > 0 {
		moreIDs = append(moreIDs, comment.MoreChildrenIDs...)
	}
	for _, reply := range comment.Replies {
		moreIDs = append(moreIDs, p.collectMoreIDsWithDepth(reply, depth+1)...)
	}
	return moreIDs
}

// ExtractPostAndComments parses the typical response from GetComments which contains
// [post_listing, comments_listing]. Returns the extracted post and comments data.
func (p *Parser) ExtractPostAndComments(response []*types.Thing) (*types.CommentsResponse, error) {
	if len(response) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// Reddit can return either:
	// 1. Two listings: [post_listing, comments_listing]
	// 2. One listing with just comments (when fetching comments for a specific post)

	result := &types.CommentsResponse{}

	if len(response) >= 2 {
		// Standard format: first is post, second is comments
		posts, err := p.ExtractPosts(response[0])
		if err == nil && len(posts) > 0 {
			result.Post = posts[0]
		}
		// Even if post extraction fails, try to extract comments

		// Second element should be the comments listing - extract pagination info
		if response[1] != nil && response[1].Kind == "Listing" {
			listingData, err := p.ParseListing(response[1])
			if err == nil {
				result.AfterFullname = listingData.AfterFullname
				result.BeforeFullname = listingData.BeforeFullname
			}
		}

		// Extract comments from the listing
		comments, moreIDs, err := p.ExtractComments(response[1])
		if err != nil {
			// If we have a post but no comments, return the post
			if result.Post != nil {
				return result, fmt.Errorf("failed to extract comments: %w", err)
			}
			// If we have neither post nor comments, return error
			return nil, fmt.Errorf("failed to extract both post and comments")
		}

		result.Comments = comments
		result.MoreIDs = moreIDs
		return result, nil
	}

	// Single listing format: just comments, no post
	// This happens when fetching additional comments or in certain API responses
	if response[0] != nil && response[0].Kind == "Listing" {
		listingData, err := p.ParseListing(response[0])
		if err == nil {
			result.AfterFullname = listingData.AfterFullname
			result.BeforeFullname = listingData.BeforeFullname
		}
	}

	comments, moreIDs, err := p.ExtractComments(response[0])
	if err != nil {
		// Try to extract as posts instead (might be a post-only response)
		posts, err := p.ExtractPosts(response[0])
		if err != nil || len(posts) == 0 {
			return nil, fmt.Errorf("failed to extract data from single listing: %w", err)
		}
		result.Post = posts[0]
		return result, nil
	}

	result.Comments = comments
	result.MoreIDs = moreIDs
	return result, nil
}
