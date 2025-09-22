package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Thing is the base class for all Reddit API objects. It provides a common
// structure for different types of content like comments, links, and subreddits.
type Thing struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name,omitempty"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// Votable is an embeddable struct for things that can be voted on.
type Votable struct {
	Ups   int `json:"ups"`
	Downs int `json:"downs"`
	// Likes indicates the user's vote: true for upvote, false for downvote, null for no vote.
	Likes *bool `json:"likes"`
}

// Created is an embeddable struct for things that have a creation time.
type Created struct {
	Created    float64 `json:"created"`
	CreatedUTC float64 `json:"created_utc"`
}

// Edited represents a field that can be a boolean or a timestamp.
// If IsEdited is true and Timestamp is 0, it was an old edit marked as `true`.
// If IsEdited is true and Timestamp is non-zero, it's a modern edit with a timestamp.
// If IsEdited is false, the item was not edited.
type Edited struct {
	IsEdited  bool
	Timestamp float64
}

// UnmarshalJSON implements json.Unmarshaler to handle mixed types for the "edited" field.
func (e *Edited) UnmarshalJSON(data []byte) error {
	s := string(data)
	// It can be a boolean `false`.
	if strings.ToLower(s) == "false" {
		e.IsEdited = false
		e.Timestamp = 0
		return nil
	}

	// It can be a boolean `true` for old edits.
	if strings.ToLower(s) == "true" {
		e.IsEdited = true
		e.Timestamp = 0
		return nil
	}

	// It could be null, which we treat as not edited.
	if strings.ToLower(s) == "null" {
		e.IsEdited = false
		e.Timestamp = 0
		return nil
	}

	// It can be a float timestamp.
	var timestamp float64
	if err := json.Unmarshal(data, &timestamp); err == nil {
		e.IsEdited = true
		e.Timestamp = timestamp
		return nil
	}

	return fmt.Errorf("unrecognized type for 'edited' field: %s", s)
}

// CommentReplies handles a "replies" field which can be a Listing object or an empty string.
type CommentReplies struct {
	*Thing
}

// UnmarshalJSON implements json.Unmarshaler to handle the mixed types of the "replies" field.
func (cr *CommentReplies) UnmarshalJSON(data []byte) error {
	if string(data) == `""` {
		cr.Thing = nil
		return nil
	}

	return json.Unmarshal(data, &cr.Thing)
}

// ListingData contains the data for a Listing, which is used for pagination.
type ListingData struct {
	Before   string   `json:"before"`
	After    string   `json:"after"`
	Modhash  string   `json:"modhash"`
	Children []*Thing `json:"children"`
}

// CommentData contains the data for a Comment.
type CommentData struct {
	Votable
	Created
	ApprovedBy          *string `json:"approved_by"`
	Author              string  `json:"author"`
	AuthorFlairCSSClass *string `json:"author_flair_css_class"`
	AuthorFlairText     *string `json:"author_flair_text"`
	BannedBy            *string `json:"banned_by"`
	Body                string  `json:"body"`
	BodyHTML            string  `json:"body_html"`
	// Edited can be a boolean (for old comments) or a float64 timestamp.
	Edited        Edited         `json:"edited"`
	Gilded        int            `json:"gilded"`
	LinkAuthor    string         `json:"link_author,omitempty"`
	LinkID        string         `json:"link_id"`
	LinkTitle     string         `json:"link_title,omitempty"`
	LinkURL       string         `json:"link_url,omitempty"`
	NumReports    *int           `json:"num_reports"`
	ParentID      string         `json:"parent_id"`
	Replies       CommentReplies `json:"replies"`
	Saved         bool           `json:"saved"`
	Score         int            `json:"score"`
	ScoreHidden   bool           `json:"score_hidden"`
	Subreddit     string         `json:"subreddit"`
	SubredditID   string         `json:"subreddit_id"`
	Distinguished *string        `json:"distinguished"`
}

// LinkData contains the data for a Link (submission).
type LinkData struct {
	Votable
	Created
	Author              string          `json:"author"`
	AuthorFlairCSSClass *string         `json:"author_flair_css_class"`
	AuthorFlairText     *string         `json:"author_flair_text"`
	Clicked             bool            `json:"clicked"`
	Domain              string          `json:"domain"`
	Hidden              bool            `json:"hidden"`
	IsSelf              bool            `json:"is_self"`
	LinkFlairCSSClass   *string         `json:"link_flair_css_class"`
	LinkFlairText       *string         `json:"link_flair_text"`
	Locked              bool            `json:"locked"`
	Media               json.RawMessage `json:"media"`
	MediaEmbed          json.RawMessage `json:"media_embed"`
	NumComments         int             `json:"num_comments"`
	Over18              bool            `json:"over_18"`
	Permalink           string          `json:"permalink"`
	Saved               bool            `json:"saved"`
	Score               int             `json:"score"`
	SelfText            string          `json:"selftext"`
	SelfTextHTML        *string         `json:"selftext_html"`
	Subreddit           string          `json:"subreddit"`
	SubredditID         string          `json:"subreddit_id"`
	Thumbnail           string          `json:"thumbnail"`
	Title               string          `json:"title"`
	URL                 string          `json:"url"`
	// Edited can be a boolean or a float64 timestamp.
	Edited        Edited  `json:"edited"`
	Distinguished *string `json:"distinguished"`
	Stickied      bool    `json:"stickied"`
}

// SubredditData contains the data for a Subreddit.
type SubredditData struct {
	AccountsActive       int     `json:"accounts_active"`
	CommentScoreHideMins int     `json:"comment_score_hide_mins"`
	Description          string  `json:"description"`
	DescriptionHTML      string  `json:"description_html"`
	DisplayName          string  `json:"display_name"`
	HeaderImg            *string `json:"header_img"`
	HeaderSize           []int   `json:"header_size"`
	HeaderTitle          *string `json:"header_title"`
	Over18               bool    `json:"over18"`
	PublicDescription    string  `json:"public_description"`
	PublicTraffic        bool    `json:"public_traffic"`
	Subscribers          int64   `json:"subscribers"`
	SubmissionType       string  `json:"submission_type"`
	SubmitLinkLabel      *string `json:"submit_link_label"`
	SubmitTextLabel      *string `json:"submit_text_label"`
	SubredditType        string  `json:"subreddit_type"`
	Title                string  `json:"title"`
	URL                  string  `json:"url"`
	UserIsBanned         *bool   `json:"user_is_banned"`
	UserIsContributor    *bool   `json:"user_is_contributor"`
	UserIsModerator      *bool   `json:"user_is_moderator"`
	UserIsSubscriber     *bool   `json:"user_is_subscriber"`
}

// MessageData contains the data for a private Message.
type MessageData struct {
	Created
	Author           string         `json:"author"`
	Body             string         `json:"body"`
	BodyHTML         string         `json:"body_html"`
	Context          string         `json:"context"`
	FirstMessage     *int64         `json:"first_message"`
	FirstMessageName *string        `json:"first_message_name"`
	Likes            *bool          `json:"likes"`
	LinkTitle        string         `json:"link_title"`
	Name             string         `json:"name"`
	New              bool           `json:"new"`
	ParentID         *string        `json:"parent_id"`
	Replies          CommentReplies `json:"replies"`
	Subject          string         `json:"subject"`
	Subreddit        *string        `json:"subreddit"`
	WasComment       bool           `json:"was_comment"`
}

// AccountData contains the data for a user Account.
type AccountData struct {
	Created
	CommentKarma     int    `json:"comment_karma"`
	HasMail          *bool  `json:"has_mail"`
	HasModMail       *bool  `json:"has_mod_mail"`
	HasVerifiedEmail *bool  `json:"has_verified_email"`
	ID               string `json:"id"`
	InboxCount       int    `json:"inbox_count,omitempty"`
	IsFriend         bool   `json:"is_friend"`
	IsGold           bool   `json:"is_gold"`
	IsMod            bool   `json:"is_mod"`
	LinkKarma        int    `json:"link_karma"`
	Modhash          string `json:"modhash,omitempty"`
	Name             string `json:"name"`
	Over18           bool   `json:"over_18"`
}

// MoreData represents a "more" object, used for comment pagination.
type MoreData struct {
	Children []string `json:"children"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
}

// Post represents a Reddit post with its metadata
type Post struct {
	ID   string    `json:"id"`   // Post ID (without prefix)
	Name string    `json:"name"` // Full name (e.g., "t3_abc123")
	Data *LinkData `json:"data"`
}

// Comment represents a Reddit comment with its metadata
type Comment struct {
	ID   string       `json:"id"`   // Comment ID (without prefix)
	Name string       `json:"name"` // Full name (e.g., "t1_abc123")
	Data *CommentData `json:"data"`
}
