// Package testutil provides test utilities and fluent builders for creating Reddit API mock data.
// This file contains builders for metadata types: Subreddit, Account, and More.
package testutil

import (
	"encoding/json"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// SubredditBuilder provides a fluent API for constructing Subreddit test data.
// It uses sensible defaults and allows customization of any field through chained method calls.
//
// Example usage:
//
//	subreddit := NewSubreddit("golang").
//		WithID("sub456").
//		WithSubscribers(50000).
//		WithTitle("The Go Programming Language").
//		Build()
//
//	// Create a Thing wrapper
//	thing := NewSubreddit("askreddit").
//		WithSubscribers(1000000).
//		ToThing()
//
//	// Get JSON for mock HTTP responses
//	jsonData := NewSubreddit("programming").ToJSON()
type SubredditBuilder struct {
	data *types.SubredditData
}

// NewSubreddit creates a new SubredditBuilder with default values and the specified display name.
// Default values:
//   - ID: "sub123"
//   - Name: "t5_sub123"
//   - DisplayName: provided name parameter
//   - Title: "Test Subreddit"
//   - PublicDescription: "A test subreddit"
//   - Subscribers: 10000
//   - AccountsActive: 100
//   - Over18: false
//   - SubredditType: "public"
//   - SubmissionType: "any"
func NewSubreddit(name string) *SubredditBuilder {
	return &SubredditBuilder{
		data: &types.SubredditData{
			ThingData: types.ThingData{
				ID:   "sub123",
				Name: "t5_sub123",
			},
			DisplayName:       name,
			Title:             "Test Subreddit",
			PublicDescription: "A test subreddit",
			Subscribers:       10000,
			AccountsActive:    100,
			Over18:            false,
			SubredditType:     "public",
			SubmissionType:    "any",
		},
	}
}

// WithID sets the subreddit's ID and automatically updates the Name field to "t5_" + id.
func (b *SubredditBuilder) WithID(id string) *SubredditBuilder {
	b.data.ID = id
	b.data.Name = "t5_" + id
	return b
}

// WithSubscribers sets the number of subscribers for the subreddit.
func (b *SubredditBuilder) WithSubscribers(n int) *SubredditBuilder {
	b.data.Subscribers = int64(n)
	return b
}

// WithTitle sets the subreddit's title (the human-readable name shown in the header).
func (b *SubredditBuilder) WithTitle(title string) *SubredditBuilder {
	b.data.Title = title
	return b
}

// WithDescription sets the subreddit's public description (shown in the sidebar).
func (b *SubredditBuilder) WithDescription(desc string) *SubredditBuilder {
	b.data.PublicDescription = desc
	return b
}

// WithActiveUsers sets the number of currently active users in the subreddit.
func (b *SubredditBuilder) WithActiveUsers(n int) *SubredditBuilder {
	b.data.AccountsActive = n
	return b
}

// WithURL sets the subreddit's URL path (e.g., "/r/golang/").
func (b *SubredditBuilder) WithURL(url string) *SubredditBuilder {
	b.data.URL = url
	return b
}

// WithType sets the subreddit type (e.g., "public", "private", "restricted").
func (b *SubredditBuilder) WithType(subredditType string) *SubredditBuilder {
	b.data.SubredditType = subredditType
	return b
}

// WithNSFW sets whether the subreddit is marked as NSFW (over 18).
func (b *SubredditBuilder) WithNSFW(over18 bool) *SubredditBuilder {
	b.data.Over18 = over18
	return b
}

// Build returns the constructed SubredditData.
func (b *SubredditBuilder) Build() *types.SubredditData {
	return b.data
}

// ToThing wraps the SubredditData in a Thing with kind "t5" and properly marshaled data.
// This is useful for constructing Reddit API response structures.
func (b *SubredditBuilder) ToThing() *types.Thing {
	dataJSON, _ := json.Marshal(b.data)
	return &types.Thing{
		ThingData: types.ThingData{
			ID:   b.data.ID,
			Name: b.data.Name,
		},
		Kind: "t5",
		Data: dataJSON,
	}
}

// ToJSON returns the SubredditData as a json.RawMessage.
// This is useful for embedding in mock HTTP responses or test fixtures.
func (b *SubredditBuilder) ToJSON() json.RawMessage {
	data, _ := json.Marshal(b.data)
	return data
}

// AccountBuilder provides a fluent API for constructing Account test data.
// It uses sensible defaults and allows customization of any field through chained method calls.
//
// Example usage:
//
//	account := NewAccount("spez").
//		WithID("user789").
//		WithLinkKarma(5000).
//		WithCommentKarma(10000).
//		Build()
//
//	// Create a Thing wrapper
//	thing := NewAccount("AutoModerator").
//		WithLinkKarma(0).
//		ToThing()
//
//	// Get JSON for mock HTTP responses
//	jsonData := NewAccount("testuser").ToJSON()
type AccountBuilder struct {
	data *types.AccountData
}

// NewAccount creates a new AccountBuilder with default values and the specified username.
// The Name field is automatically set to "t2_" + ID.
// Default values:
//   - ID: "user123"
//   - Name: "t2_user123"
//   - LinkKarma: 1000
//   - CommentKarma: 500
//   - Created/CreatedUTC: current Unix timestamp as float64
//   - Over18: false
//   - IsGold: false
//   - IsMod: false
//   - IsFriend: false
func NewAccount(username string) *AccountBuilder {
	now := float64(time.Now().Unix())
	id := "user123"
	return &AccountBuilder{
		data: &types.AccountData{
			ThingData: types.ThingData{
				ID:   id,
				Name: "t2_" + id,
			},
			Created: types.Created{
				Created:    now,
				CreatedUTC: now,
			},
			LinkKarma:    1000,
			CommentKarma: 500,
			Over18:       false,
			IsGold:       false,
			IsMod:        false,
			IsFriend:     false,
		},
	}
}

// WithID sets the account's ID and automatically updates the Name field to "t2_" + id.
func (b *AccountBuilder) WithID(id string) *AccountBuilder {
	b.data.ID = id
	b.data.Name = "t2_" + id
	return b
}

// WithLinkKarma sets the account's link (post) karma.
func (b *AccountBuilder) WithLinkKarma(karma int) *AccountBuilder {
	b.data.LinkKarma = karma
	return b
}

// WithCommentKarma sets the account's comment karma.
func (b *AccountBuilder) WithCommentKarma(karma int) *AccountBuilder {
	b.data.CommentKarma = karma
	return b
}

// WithCreated sets the account creation timestamp.
func (b *AccountBuilder) WithCreated(timestamp float64) *AccountBuilder {
	b.data.Created = types.Created{
		Created:    timestamp,
		CreatedUTC: timestamp,
	}
	return b
}

// WithGold sets whether the account has Reddit Gold.
func (b *AccountBuilder) WithGold(isGold bool) *AccountBuilder {
	b.data.IsGold = isGold
	return b
}

// WithMod sets whether the account is a moderator.
func (b *AccountBuilder) WithMod(isMod bool) *AccountBuilder {
	b.data.IsMod = isMod
	return b
}

// Build returns the constructed AccountData.
func (b *AccountBuilder) Build() *types.AccountData {
	return b.data
}

// ToThing wraps the AccountData in a Thing with kind "t2" and properly marshaled data.
// This is useful for constructing Reddit API response structures.
func (b *AccountBuilder) ToThing() *types.Thing {
	dataJSON, _ := json.Marshal(b.data)
	return &types.Thing{
		ThingData: types.ThingData{
			ID:   b.data.ID,
			Name: b.data.Name,
		},
		Kind: "t2",
		Data: dataJSON,
	}
}

// ToJSON returns the AccountData as a json.RawMessage.
// This is useful for embedding in mock HTTP responses or test fixtures.
func (b *AccountBuilder) ToJSON() json.RawMessage {
	data, _ := json.Marshal(b.data)
	return data
}

// MoreBuilder provides a fluent API for constructing More (comment continuation) test data.
// More objects represent collapsed comment threads that can be expanded by loading additional comments.
//
// Example usage:
//
//	more := NewMore().
//		WithID("more456").
//		WithCount(15).
//		WithChildren([]string{"abc123", "def456", "ghi789"}).
//		Build()
//
//	// Create a Thing wrapper
//	thing := NewMore().
//		WithChildren([]string{"comment1", "comment2"}).
//		ToThing()
//
//	// Get JSON for mock HTTP responses
//	jsonData := NewMore().WithCount(10).ToJSON()
type MoreBuilder struct {
	data *types.MoreData
}

// NewMore creates a new MoreBuilder with default values.
// Default values:
//   - ID: "more123"
//   - Name: "more_more123"
//   - Children: empty slice
func NewMore() *MoreBuilder {
	id := "more123"
	return &MoreBuilder{
		data: &types.MoreData{
			ThingData: types.ThingData{
				ID:   id,
				Name: "more_" + id,
			},
			Children: []string{},
		},
	}
}

// WithID sets the More object's ID and automatically updates the Name field to "more_" + id.
// Note: Reddit uses "more_" prefix for more continuation objects in API responses.
func (b *MoreBuilder) WithID(id string) *MoreBuilder {
	b.data.ID = id
	b.data.Name = "more_" + id
	return b
}

// WithCount is a no-op method provided for API compatibility.
// Note: The types.MoreData struct does not include a count field, so this method
// has no effect. Reddit's API includes count in "more" responses, but this wrapper's
// internal representation only tracks the children IDs. This method is retained to
// maintain a fluent API and for potential future extension.
func (b *MoreBuilder) WithCount(count int) *MoreBuilder {
	// No-op: MoreData doesn't have a Count field
	return b
}

// WithChildren sets the list of child comment IDs that can be loaded.
// These are typically base36-encoded IDs without the "t1_" prefix.
func (b *MoreBuilder) WithChildren(children []string) *MoreBuilder {
	b.data.Children = children
	return b
}

// Build returns the constructed MoreData.
func (b *MoreBuilder) Build() *types.MoreData {
	return b.data
}

// ToThing wraps the MoreData in a Thing with kind "more" and properly marshaled data.
// This is useful for constructing Reddit API response structures.
func (b *MoreBuilder) ToThing() *types.Thing {
	dataJSON, _ := json.Marshal(b.data)
	return &types.Thing{
		ThingData: types.ThingData{
			ID:   b.data.ID,
			Name: b.data.Name,
		},
		Kind: "more",
		Data: dataJSON,
	}
}

// ToJSON returns the MoreData as a json.RawMessage.
// This is useful for embedding in mock HTTP responses or test fixtures.
func (b *MoreBuilder) ToJSON() json.RawMessage {
	data, _ := json.Marshal(b.data)
	return data
}

// ListingBuilder provides a fluent API for constructing Listing test data.
// Listings are Reddit's standard response format for paginated content.
//
// Example usage:
//
//	listing := NewListingBuilder().
//		WithAfter("t3_abc123").
//		WithBefore("t3_xyz789").
//		AddChild(testutil.NewPostBuilder().WithID("post1").ToThing()).
//		Build()
type ListingBuilder struct {
	data *types.ListingData
}

// NewListingBuilder creates a new ListingBuilder with default empty values.
func NewListingBuilder() *ListingBuilder {
	return &ListingBuilder{
		data: &types.ListingData{
			BeforeFullname: "",
			AfterFullname:  "",
			Modhash:        "",
			Children:       []*types.Thing{},
		},
	}
}

// WithAfter sets the after pagination fullname.
func (b *ListingBuilder) WithAfter(after string) *ListingBuilder {
	b.data.AfterFullname = after
	return b
}

// WithBefore sets the before pagination fullname.
func (b *ListingBuilder) WithBefore(before string) *ListingBuilder {
	b.data.BeforeFullname = before
	return b
}

// WithModhash sets the modhash value.
func (b *ListingBuilder) WithModhash(modhash string) *ListingBuilder {
	b.data.Modhash = modhash
	return b
}

// AddChild adds a Thing child to the listing.
func (b *ListingBuilder) AddChild(child *types.Thing) *ListingBuilder {
	b.data.Children = append(b.data.Children, child)
	return b
}

// WithChildren sets all children at once, replacing any existing children.
func (b *ListingBuilder) WithChildren(children []*types.Thing) *ListingBuilder {
	b.data.Children = children
	return b
}

// Build returns the constructed ListingData.
func (b *ListingBuilder) Build() *ListingBuilder {
	return b
}

// ToThing wraps the ListingData in a Thing with kind "Listing" and properly marshaled data.
func (b *ListingBuilder) ToThing() *types.Thing {
	dataJSON, _ := json.Marshal(b.data)
	return &types.Thing{
		Kind: "Listing",
		Data: dataJSON,
	}
}

// ToJSON returns the ListingData as a json.RawMessage.
func (b *ListingBuilder) ToJSON() json.RawMessage {
	data, _ := json.Marshal(b.data)
	return data
}

// MessageBuilder provides a fluent API for constructing Message test data.
//
// Example usage:
//
//	message := NewMessageBuilder().
//		WithID("msg123").
//		WithAuthor("sender").
//		WithBody("Message body").
//		WithSubject("Test Subject").
//		Build()
type MessageBuilder struct {
	data *types.MessageData
}

// NewMessageBuilder creates a new MessageBuilder with default values.
func NewMessageBuilder() *MessageBuilder {
	now := float64(time.Now().Unix())
	return &MessageBuilder{
		data: &types.MessageData{
			ThingData: types.ThingData{
				ID:   "msg123",
				Name: "t4_msg123",
			},
			Created: types.Created{
				Created:    now,
				CreatedUTC: now,
			},
			Author:   "testuser",
			Body:     "Test message",
			BodyHTML: "&lt;div class=\"md\"&gt;&lt;p&gt;Test message&lt;/p&gt;\n&lt;/div&gt;",
			Subject:  "Test Subject",
		},
	}
}

// WithID sets the message ID and automatically generates the Name field as "t4_" + ID.
func (b *MessageBuilder) WithID(id string) *MessageBuilder {
	b.data.ID = id
	b.data.Name = "t4_" + id
	return b
}

// WithAuthor sets the message author username.
func (b *MessageBuilder) WithAuthor(author string) *MessageBuilder {
	b.data.Author = author
	return b
}

// WithBody sets the message body text.
func (b *MessageBuilder) WithBody(body string) *MessageBuilder {
	b.data.Body = body
	return b
}

// WithSubject sets the message subject.
func (b *MessageBuilder) WithSubject(subject string) *MessageBuilder {
	b.data.Subject = subject
	return b
}

// WithCreated sets both Created and CreatedUTC to the given Unix timestamp.
func (b *MessageBuilder) WithCreated(timestamp float64) *MessageBuilder {
	b.data.Created.Created = timestamp
	b.data.Created.CreatedUTC = timestamp
	return b
}

// Build returns the constructed MessageData.
func (b *MessageBuilder) Build() *types.MessageData {
	return b.data
}

// ToThing wraps the MessageData in a Thing with kind "t4" and properly marshaled data.
func (b *MessageBuilder) ToThing() *types.Thing {
	dataJSON, _ := json.Marshal(b.data)
	return &types.Thing{
		ThingData: types.ThingData{
			ID:   b.data.ID,
			Name: b.data.Name,
		},
		Kind: "t4",
		Data: dataJSON,
	}
}

// ToJSON returns the MessageData as a json.RawMessage.
func (b *MessageBuilder) ToJSON() json.RawMessage {
	data, _ := json.Marshal(b.data)
	return data
}
