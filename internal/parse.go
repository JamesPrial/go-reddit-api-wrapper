package internal

import (
	"encoding/json"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Parser handles parsing of Reddit API responses
type Parser struct{}

// NewParser creates a new parser instance
func NewParser() *Parser {
	return &Parser{}
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
		return p.ParseLink(thing)
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

	var listing types.ListingData
	if err := json.Unmarshal(thing.Data, &listing); err != nil {
		return nil, fmt.Errorf("failed to parse Listing data: %w", err)
	}
	return &listing, nil
}

// ParseLink extracts a LinkData from a Thing of kind "t3".
func (p *Parser) ParseLink(thing *types.Thing) (*types.LinkData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t3" {
		return nil, fmt.Errorf("expected t3 (Link), got %s", thing.Kind)
	}

	var link types.LinkData
	if err := json.Unmarshal(thing.Data, &link); err != nil {
		return nil, fmt.Errorf("failed to parse Link data: %w", err)
	}
	return &link, nil
}

// ParseComment extracts a CommentData from a Thing of kind "t1".
func (p *Parser) ParseComment(thing *types.Thing) (*types.CommentData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t1" {
		return nil, fmt.Errorf("expected t1 (Comment), got %s", thing.Kind)
	}

	var comment types.CommentData
	if err := json.Unmarshal(thing.Data, &comment); err != nil {
		return nil, fmt.Errorf("failed to parse Comment data: %w", err)
	}
	return &comment, nil
}

// ParseSubreddit extracts a SubredditData from a Thing of kind "t5".
func (p *Parser) ParseSubreddit(thing *types.Thing) (*types.SubredditData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t5" {
		return nil, fmt.Errorf("expected t5 (Subreddit), got %s", thing.Kind)
	}

	var subreddit types.SubredditData
	if err := json.Unmarshal(thing.Data, &subreddit); err != nil {
		return nil, fmt.Errorf("failed to parse Subreddit data: %w", err)
	}
	return &subreddit, nil
}

// ParseAccount extracts an AccountData from a Thing of kind "t2".
func (p *Parser) ParseAccount(thing *types.Thing) (*types.AccountData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t2" {
		return nil, fmt.Errorf("expected t2 (Account), got %s", thing.Kind)
	}

	var account types.AccountData
	if err := json.Unmarshal(thing.Data, &account); err != nil {
		return nil, fmt.Errorf("failed to parse Account data: %w", err)
	}
	return &account, nil
}

// ParseMessage extracts a MessageData from a Thing of kind "t4".
func (p *Parser) ParseMessage(thing *types.Thing) (*types.MessageData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "t4" {
		return nil, fmt.Errorf("expected t4 (Message), got %s", thing.Kind)
	}

	var message types.MessageData
	if err := json.Unmarshal(thing.Data, &message); err != nil {
		return nil, fmt.Errorf("failed to parse Message data: %w", err)
	}
	return &message, nil
}

// ParseMore extracts a MoreData from a Thing of kind "more".
func (p *Parser) ParseMore(thing *types.Thing) (*types.MoreData, error) {
	if thing == nil {
		return nil, fmt.Errorf("thing is nil")
	}
	if thing.Kind != "more" {
		return nil, fmt.Errorf("expected more, got %s", thing.Kind)
	}

	var more types.MoreData
	if err := json.Unmarshal(thing.Data, &more); err != nil {
		return nil, fmt.Errorf("failed to parse More data: %w", err)
	}
	return &more, nil
}

// ExtractPosts extracts all Post objects from a listing Thing.
func (p *Parser) ExtractPosts(listing *types.Thing) ([]*types.Post, error) {
	listingData, err := p.ParseListing(listing)
	if err != nil {
		return nil, err
	}

	posts := make([]*types.Post, 0, len(listingData.Children))
	for _, child := range listingData.Children {
		if child.Kind == "t3" {
			link, err := p.ParseLink(child)
			if err != nil {
				continue
			}
			posts = append(posts, &types.Post{
				ID:   child.ID,
				Name: child.Name,
				Data: link,
			})
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
		commentData, err := p.ParseComment(thing)
		if err != nil {
			return nil, nil, err
		}
		comments = append(comments, &types.Comment{
			ID:   thing.ID,
			Name: thing.Name,
			Data: commentData,
		})

		// Process replies recursively
		if commentData.Replies.Thing != nil {
			replies, moreReplies, _ := p.ExtractComments(commentData.Replies.Thing)
			comments = append(comments, replies...)
			moreIDs = append(moreIDs, moreReplies...)
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
			commentData, err := p.ParseComment(child)
			if err != nil {
				continue
			}
			comments = append(comments, &types.Comment{
				ID:   child.ID,
				Name: child.Name,
				Data: commentData,
			})

			// Process nested replies
			if commentData.Replies.Thing != nil {
				replies, moreReplies, _ := p.ExtractComments(commentData.Replies.Thing)
				comments = append(comments, replies...)
				moreIDs = append(moreIDs, moreReplies...)
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
// [post_listing, comments_listing]
func (p *Parser) ExtractPostAndComments(response []*types.Thing) (*types.Post, []*types.Comment, []string, error) {
	if len(response) < 2 {
		return nil, nil, nil, fmt.Errorf("expected at least 2 elements in response")
	}

	// First element should be a listing with the post
	posts, err := p.ExtractPosts(response[0])
	if err != nil || len(posts) == 0 {
		return nil, nil, nil, fmt.Errorf("failed to extract post: %w", err)
	}

	// Second element should be the comments listing
	comments, moreIDs, err := p.ExtractComments(response[1])
	if err != nil {
		return posts[0], nil, nil, fmt.Errorf("failed to extract comments: %w", err)
	}

	return posts[0], comments, moreIDs, nil
}