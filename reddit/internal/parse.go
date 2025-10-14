package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/validation"
)

// MaxCommentDepth is the maximum depth of nested comments to prevent stack overflow attacks
const MaxCommentDepth = 50

// Parser handles parsing of Reddit API responses with context support and optimized performance
type Parser struct {
	logger *slog.Logger
	pool   sync.Pool // Reuse parsing structures for better performance
}

// NewParser creates a new parser instance with an optional logger.
// If logger is nil, parse errors will not be logged.
func NewParser(logger ...*slog.Logger) *Parser {
	var log *slog.Logger
	if len(logger) > 0 {
		log = logger[0]
	}

	return &Parser{
		logger: log,
		pool: sync.Pool{
			New: func() interface{} {
				return &parseContext{
					seenIDs: make(map[string]bool),
				}
			},
		},
	}
}

// parseContext holds state for parsing operations
type parseContext struct {
	depth   int
	seenIDs map[string]bool // Prevent infinite loops
}

// ParseThing determines the type of a Thing and returns the appropriate typed struct.
func (p *Parser) ParseThing(ctx context.Context, thing *types.Thing) (any, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}

	pc := p.pool.Get().(*parseContext)
	defer p.pool.Put(pc)

	// Reset parse context
	pc.depth = 0
	clear(pc.seenIDs)

	return p.parseThingWithContext(ctx, thing, pc)
}

// parseThingWithContext is the internal implementation with context tracking
func (p *Parser) parseThingWithContext(ctx context.Context, thing *types.Thing, pc *parseContext) (any, error) {
	switch thing.Kind {
	case "Listing":
		return p.ParseListing(ctx, thing)
	case "t1":
		return p.ParseComment(ctx, thing, pc)
	case "t2":
		return p.ParseAccount(ctx, thing)
	case "t3":
		return p.ParsePost(ctx, thing)
	case "t4":
		return p.ParseMessage(ctx, thing)
	case "t5":
		return p.ParseSubreddit(ctx, thing)
	case "more":
		return p.ParseMore(ctx, thing)
	default:
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "unknown thing kind",
				slog.String("kind", thing.Kind))
		}
		return nil, fmt.Errorf("unknown kind: %s", thing.Kind)
	}
}

// ParseListing extracts a ListingData from a Thing of kind "Listing".
func (p *Parser) ParseListing(ctx context.Context, thing *types.Thing) (*types.ListingData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "Listing" {
		return nil, fmt.Errorf("expected Listing, got %s", thing.Kind)
	}

	var result types.ListingData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse listing data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse Listing data: %w", err)
	}

	// Validate pagination tokens
	if result.AfterFullname != "" && !validation.IsValidFullname(result.AfterFullname) {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid AfterFullname from Reddit API",
				slog.String("after", result.AfterFullname))
		}
		return nil, fmt.Errorf("invalid AfterFullname from Reddit API: %s", result.AfterFullname)
	}
	if result.BeforeFullname != "" && !validation.IsValidFullname(result.BeforeFullname) {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid BeforeFullname from Reddit API",
				slog.String("before", result.BeforeFullname))
		}
		return nil, fmt.Errorf("invalid BeforeFullname from Reddit API: %s", result.BeforeFullname)
	}

	return &result, nil
}

// ParsePost extracts a Post from a Thing of kind "t3".
func (p *Parser) ParsePost(ctx context.Context, thing *types.Thing) (*types.Post, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t3" {
		return nil, fmt.Errorf("expected t3 (Post), got %s", thing.Kind)
	}

	var result types.Post
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse post data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse Post data: %w", err)
	}

	// Validate the parsed post
	if err := validation.ValidatePost(&result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid post data from Reddit API",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("invalid post data from Reddit API: %w", err)
	}

	return &result, nil
}

// ParseComment extracts a Comment from a Thing of kind "t1" and builds a proper tree structure.
// The Replies field will contain only direct children, and each child will have its own Replies.
func (p *Parser) ParseComment(ctx context.Context, thing *types.Thing, pc *parseContext) (*types.Comment, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t1" {
		return nil, fmt.Errorf("expected t1 (Comment), got %s", thing.Kind)
	}

	// Prevent stack overflow from deeply nested comments
	if pc.depth > MaxCommentDepth {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "comment tree depth exceeds maximum",
				slog.Int("depth", pc.depth),
				slog.Int("max_depth", MaxCommentDepth))
		}
		return nil, fmt.Errorf("comment tree depth exceeds maximum of %d", MaxCommentDepth)
	}

	// Optimized single unmarshal with unified structure
	var data struct {
		types.Comment
		Replies json.RawMessage `json:"replies"`
	}

	if err := json.Unmarshal(thing.Data, &data); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse comment data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse Comment data: %w", err)
	}

	// Validate the parsed comment
	if err := validation.ValidateComment(&data.Comment); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid comment data from Reddit API",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("invalid comment data from Reddit API: %w", err)
	}

	// Check for infinite loops
	if pc.seenIDs[data.ID] {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "detected comment loop",
				slog.String("id", data.ID))
		}
		return &data.Comment, nil // Return what we have, skip the loop
	}
	pc.seenIDs[data.ID] = true

	// Parse replies if present
	if len(data.Replies) > 0 && !bytes.Equal(data.Replies, []byte(`""`)) {
		if err := p.parseReplies(ctx, &data.Comment, data.Replies, pc); err != nil {
			if p.logger != nil {
				p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse replies",
					slog.String("error", err.Error()),
					slog.String("comment_id", data.ID))
			}
			// Don't fail the entire comment for reply parsing errors
		}
	}

	return &data.Comment, nil
}

// parseReplies handles the replies field parsing with error recovery
func (p *Parser) parseReplies(ctx context.Context, comment *types.Comment, repliesData json.RawMessage, pc *parseContext) error {
	var repliesThing types.Thing
	if err := json.Unmarshal(repliesData, &repliesThing); err != nil {
		return fmt.Errorf("failed to unmarshal replies: %w", err)
	}

	if repliesThing.Kind != "Listing" {
		return fmt.Errorf("expected Listing for replies, got %s", repliesThing.Kind)
	}

	listingData, err := p.ParseListing(ctx, &repliesThing)
	if err != nil {
		return fmt.Errorf("failed to parse replies listing: %w", err)
	}

	// Process children with error recovery
	for _, child := range listingData.Children {
		switch child.Kind {
		case "t1":
			pc.depth++
			childComment, err := p.ParseComment(ctx, child, pc)
			pc.depth--
			if err != nil {
				continue // Skip unparseable replies
			}
			comment.Replies = append(comment.Replies, childComment)

		case "more":
			more, err := p.ParseMore(ctx, child)
			if err != nil {
				continue
			}
			comment.MoreChildrenIDs = append(comment.MoreChildrenIDs, more.Children...)
		}
	}

	return nil
}

// ParseSubreddit extracts a SubredditData from a Thing of kind "t5".
func (p *Parser) ParseSubreddit(ctx context.Context, thing *types.Thing) (*types.SubredditData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t5" {
		return nil, fmt.Errorf("expected t5 (Subreddit), got %s", thing.Kind)
	}

	var result types.SubredditData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse subreddit data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse Subreddit data: %w", err)
	}

	// Validate the parsed subreddit
	if err := validation.ValidateSubredditData(&result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid subreddit data from Reddit API",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("invalid subreddit data from Reddit API: %w", err)
	}

	return &result, nil
}

// ParseAccount extracts an AccountData from a Thing of kind "t2".
func (p *Parser) ParseAccount(ctx context.Context, thing *types.Thing) (*types.AccountData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t2" {
		return nil, fmt.Errorf("expected t2 (Account), got %s", thing.Kind)
	}

	var result types.AccountData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse account data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse Account data: %w", err)
	}

	// Validate the parsed account
	if err := validation.ValidateAccountData(&result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid account data from Reddit API",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("invalid account data from Reddit API: %w", err)
	}

	return &result, nil
}

// ParseMessage extracts a MessageData from a Thing of kind "t4".
func (p *Parser) ParseMessage(ctx context.Context, thing *types.Thing) (*types.MessageData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t4" {
		return nil, fmt.Errorf("expected t4 (Message), got %s", thing.Kind)
	}

	var result types.MessageData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse message data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse Message data: %w", err)
	}

	// Validate the parsed message
	if err := validation.ValidateMessageData(&result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid message data from Reddit API",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("invalid message data from Reddit API: %w", err)
	}

	return &result, nil
}

// ParseMore extracts a MoreData from a Thing of kind "more".
func (p *Parser) ParseMore(ctx context.Context, thing *types.Thing) (*types.MoreData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "more" {
		return nil, fmt.Errorf("expected more, got %s", thing.Kind)
	}

	var result types.MoreData
	if err := json.Unmarshal(thing.Data, &result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse more data",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("failed to parse More data: %w", err)
	}

	// Validate the parsed more data
	if err := validation.ValidateMoreData(&result); err != nil {
		if p.logger != nil {
			p.logger.LogAttrs(ctx, slog.LevelWarn, "invalid more data from Reddit API",
				slog.String("error", err.Error()))
		}
		return nil, fmt.Errorf("invalid more data from Reddit API: %w", err)
	}

	return &result, nil
}

// ExtractPosts extracts all Post objects from a listing Thing.
func (p *Parser) ExtractPosts(ctx context.Context, thing *types.Thing) ([]*types.Post, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "Listing" {
		return nil, fmt.Errorf("expected Listing, got %s", thing.Kind)
	}

	listingData, err := p.ParseListing(ctx, thing)
	if err != nil {
		return nil, err
	}

	posts := make([]*types.Post, 0, len(listingData.Children))
	for _, child := range listingData.Children {
		if child.Kind == "t3" {
			post, err := p.ParsePost(ctx, child)
			if err != nil {
				// Log parse error if logger is available
				if p.logger != nil {
					p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse post",
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
func (p *Parser) ExtractComments(ctx context.Context, thing *types.Thing) ([]*types.Comment, []string, error) {
	comments := make([]*types.Comment, 0)
	moreIDs := make([]string, 0)

	// Handle both single comments and listings
	if thing.Kind == "t1" {
		pc := p.pool.Get().(*parseContext)
		defer p.pool.Put(pc)
		pc.depth = 0
		clear(pc.seenIDs)

		comment, err := p.ParseComment(ctx, thing, pc)
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

	listingData, err := p.ParseListing(ctx, thing)
	if err != nil {
		return nil, nil, err
	}

	pc := p.pool.Get().(*parseContext)
	defer p.pool.Put(pc)
	pc.depth = 0
	clear(pc.seenIDs)

	for _, child := range listingData.Children {
		switch child.Kind {
		case "t1":
			comment, err := p.ParseComment(ctx, child, pc)
			if err != nil {
				// Log parse error if logger is available
				if p.logger != nil {
					p.logger.LogAttrs(ctx, slog.LevelWarn, "failed to parse comment",
						slog.String("error", err.Error()),
						slog.String("kind", child.Kind))
				}
				continue // Skip unparseable comments
			}

			comments = append(comments, comment)
			// Collect more IDs from the entire tree
			moreIDs = append(moreIDs, p.collectMoreIDs(comment)...)
		case "more":
			more, err := p.ParseMore(ctx, child)
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
func (p *Parser) ExtractPostAndComments(ctx context.Context, response []*types.Thing) (*types.CommentsResponse, error) {
	if len(response) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// Reddit can return either:
	// 1. Two listings: [post_listing, comments_listing]
	// 2. One listing with just comments (when fetching comments for a specific post)

	result := &types.CommentsResponse{}

	if len(response) >= 2 {
		// Standard format: first is post, second is comments
		posts, err := p.ExtractPosts(ctx, response[0])
		if err == nil && len(posts) > 0 {
			result.Post = posts[0]
		}
		// Even if post extraction fails, try to extract comments

		// Second element should be the comments listing - extract pagination info
		if response[1] != nil && response[1].Kind == "Listing" {
			listingData, err := p.ParseListing(ctx, response[1])
			if err == nil {
				result.AfterFullname = listingData.AfterFullname
				result.BeforeFullname = listingData.BeforeFullname
			}
		}

		// Extract comments from the listing
		comments, moreIDs, err := p.ExtractComments(ctx, response[1])
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
		listingData, err := p.ParseListing(ctx, response[0])
		if err == nil {
			result.AfterFullname = listingData.AfterFullname
			result.BeforeFullname = listingData.BeforeFullname
		}
	}

	comments, moreIDs, err := p.ExtractComments(ctx, response[0])
	if err != nil {
		// Try to extract as posts instead (might be a post-only response)
		posts, err := p.ExtractPosts(ctx, response[0])
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
