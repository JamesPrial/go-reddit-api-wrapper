package testutil

import (
	"encoding/json"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// PostBuilder provides a fluent API for creating test Post data with sensible defaults.
// This builder is designed for use in tests to easily construct Post structs without
// needing to specify every field manually.
//
// Default values are set to valid Reddit-like data:
//   - ID: "post123"
//   - Name: "t3_post123" (auto-generated from ID with Reddit's t3 prefix for posts)
//   - Title: "Test Post"
//   - Score: 100
//   - Ups: 100
//   - Downs: 0
//   - Author: "testuser"
//   - Subreddit: "test"
//   - Created/CreatedUTC: current Unix timestamp as float64
//   - Permalink: "/r/test/comments/post123/test_post/"
//   - URL: "https://reddit.com/r/test/comments/post123/test_post/"
//   - NumComments: 0
//   - UpvoteRatio: 0.95
//
// Example usage:
//
//	// Create a basic post with defaults
//	post := NewPostBuilder().Build()
//
//	// Create a post with custom fields
//	post := NewPostBuilder().
//	    WithID("abc123").
//	    WithTitle("Custom Title").
//	    WithScore(500).
//	    WithAuthor("cooluser").
//	    Build()
//
//	// Create a Thing wrapper for the post (for API response mocking)
//	thing := NewPostBuilder().
//	    WithID("xyz789").
//	    WithTitle("Another Post").
//	    ToThing()
//
//	// Get just the JSON for the post data
//	jsonData := NewPostBuilder().
//	    WithSubreddit("golang").
//	    ToJSON()
type PostBuilder struct {
	post *types.Post
}

// NewPostBuilder creates a new PostBuilder with sensible default values.
// All fields are initialized to valid Reddit-like data suitable for testing.
func NewPostBuilder() *PostBuilder {
	now := float64(time.Now().Unix())

	return &PostBuilder{
		post: &types.Post{
			ThingData: types.ThingData{
				ID:   "post123",
				Name: "t3_post123",
			},
			Votable: types.Votable{
				Score: 100,
				Ups:   100,
				Downs: 0,
				Likes: nil,
			},
			Created: types.Created{
				Created:    now,
				CreatedUTC: now,
			},
			Author:      "testuser",
			Title:       "Test Post",
			Subreddit:   "test",
			SubredditID: "t5_test",
			Permalink:   "/r/test/comments/post123/test_post/",
			URL:         "https://reddit.com/r/test/comments/post123/test_post/",
			NumComments: 0,
			UpvoteRatio: 0.95,
			Domain:      "reddit.com",
			IsSelf:      true,
			SelfText:    "",
			Thumbnail:   "self",
			Clicked:     false,
			Hidden:      false,
			Locked:      false,
			Over18:      false,
			Saved:       false,
			Stickied:    false,
			// Edited is left as zero value (IsEdited=false, Timestamp=0) which marshals correctly
		},
	}
}

// WithID sets the post ID and automatically updates the Name field to "t3_" + ID
// to match Reddit's fullname format for posts.
func (pb *PostBuilder) WithID(id string) *PostBuilder {
	pb.post.ID = id
	pb.post.Name = "t3_" + id
	return pb
}

// WithTitle sets the post title.
func (pb *PostBuilder) WithTitle(title string) *PostBuilder {
	pb.post.Title = title
	return pb
}

// WithScore sets the post score (net upvotes) and also sets Ups to match.
// Reddit's API always returns Ups equal to Score since individual vote counts
// are no longer provided.
func (pb *PostBuilder) WithScore(score int) *PostBuilder {
	pb.post.Score = score
	pb.post.Ups = score
	return pb
}

// WithAuthor sets the post author's username.
func (pb *PostBuilder) WithAuthor(author string) *PostBuilder {
	pb.post.Author = author
	return pb
}

// WithSubreddit sets the subreddit name and automatically updates the SubredditID
// to "t5_" + subreddit to match Reddit's fullname format.
func (pb *PostBuilder) WithSubreddit(sub string) *PostBuilder {
	pb.post.Subreddit = sub
	pb.post.SubredditID = "t5_" + sub
	return pb
}

// WithURL sets the post URL.
func (pb *PostBuilder) WithURL(url string) *PostBuilder {
	pb.post.URL = url
	return pb
}

// WithPermalink sets the post permalink (the path to the post on Reddit).
func (pb *PostBuilder) WithPermalink(permalink string) *PostBuilder {
	pb.post.Permalink = permalink
	return pb
}

// WithCreated sets both Created and CreatedUTC fields to the same timestamp value.
// The timestamp should be a Unix timestamp as a float64.
func (pb *PostBuilder) WithCreated(timestamp float64) *PostBuilder {
	pb.post.Created.Created = timestamp
	pb.post.Created.CreatedUTC = timestamp
	return pb
}

// WithNumComments sets the number of comments on the post.
func (pb *PostBuilder) WithNumComments(n int) *PostBuilder {
	pb.post.NumComments = n
	return pb
}

// WithUpvoteRatio sets the upvote ratio (0.0 to 1.0, e.g., 0.95 = 95% upvoted).
func (pb *PostBuilder) WithUpvoteRatio(ratio float64) *PostBuilder {
	pb.post.UpvoteRatio = ratio
	return pb
}

// WithSelfText sets the self-text content for text posts.
func (pb *PostBuilder) WithSelfText(text string) *PostBuilder {
	pb.post.SelfText = text
	pb.post.IsSelf = text != ""
	return pb
}

// WithDomain sets the domain field (e.g., "youtube.com", "self.golang").
func (pb *PostBuilder) WithDomain(domain string) *PostBuilder {
	pb.post.Domain = domain
	return pb
}

// WithOver18 sets the NSFW flag for the post.
func (pb *PostBuilder) WithOver18(over18 bool) *PostBuilder {
	pb.post.Over18 = over18
	return pb
}

// WithStickied sets whether the post is stickied (pinned) by moderators.
func (pb *PostBuilder) WithStickied(stickied bool) *PostBuilder {
	pb.post.Stickied = stickied
	return pb
}

// WithLocked sets whether the post is locked (comments disabled).
func (pb *PostBuilder) WithLocked(locked bool) *PostBuilder {
	pb.post.Locked = locked
	return pb
}

// WithEdited marks the post as edited with the given timestamp.
// Pass 0 to mark as edited without a specific timestamp (old-style edit).
func (pb *PostBuilder) WithEdited(timestamp float64) *PostBuilder {
	pb.post.Edited = types.Edited{
		IsEdited:  true,
		Timestamp: timestamp,
	}
	return pb
}

// WithDistinguished sets the distinguished status ("moderator", "admin", or nil).
func (pb *PostBuilder) WithDistinguished(distinguished string) *PostBuilder {
	pb.post.Distinguished = &distinguished
	return pb
}

// Build returns the constructed Post struct.
func (pb *PostBuilder) Build() *types.Post {
	return pb.post
}

// ToThing wraps the post in a Thing structure with kind "t3" and the post data
// as a JSON RawMessage. This is useful for mocking Reddit API responses which
// return posts wrapped in Thing objects.
//
// Returns nil if JSON marshaling fails (should not happen with valid Post data).
func (pb *PostBuilder) ToThing() *types.Thing {
	data, err := json.Marshal(pb.post)
	if err != nil {
		// This should never happen with a valid Post struct
		return nil
	}

	return &types.Thing{
		ThingData: types.ThingData{
			ID:   pb.post.ID,
			Name: pb.post.Name,
		},
		Kind: "t3",
		Data: data,
	}
}

// ToJSON returns the post marshaled as a JSON RawMessage.
// This is useful when you need just the JSON representation of the post data
// without the Thing wrapper.
//
// Returns nil if JSON marshaling fails (should not happen with valid Post data).
func (pb *PostBuilder) ToJSON() json.RawMessage {
	data, err := json.Marshal(pb.post)
	if err != nil {
		// This should never happen with a valid Post struct
		return nil
	}
	return data
}
