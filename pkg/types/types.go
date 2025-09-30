package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RedditObject defines the common behavior for all Reddit API objects like
// Posts, Comments, and Subreddits.
type RedditObject interface {
	GetID() string
	GetName() string
}

// ThingData holds the common fields for Reddit objects.
// It can be embedded into specific types like Post and Comment.
type ThingData struct {
	ID   string `json:"id"`   // ID (without prefix)
	Name string `json:"name"` // Full name (e.g., "t3_abc123")
}

// GetID returns the object's ID.
func (td ThingData) GetID() string {
	return td.ID
}

// GetName returns the object's full name.
func (td ThingData) GetName() string {
	return td.Name
}

// Thing is the base class for all Reddit API objects. It provides a common
// structure for different types of content like comments, links, and subreddits.
type Thing struct {
	ThingData
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

// ListingData contains the data for a Listing, which is used for pagination.
type ListingData struct {
	BeforeFullname string   `json:"before"` // Reddit fullname for pagination (previous page)
	AfterFullname  string   `json:"after"`  // Reddit fullname for pagination (next page)
	Modhash        string   `json:"modhash"`
	Children       []*Thing `json:"children"` // Raw Things with kind+data, parsed by caller
}

// Pagination captures the shared pagination behaviour for Reddit listing endpoints.
// Reddit uses "fullnames" for pagination, which are strings like "t3_abc123" where
// "t3" indicates the type (link/post) and "abc123" is the item ID.
type Pagination struct {
	// Limit specifies the number of items to retrieve.
	// Reddit enforces a maximum of 100 items per request.
	// If 0 or not specified, Reddit's default limit (usually 25) is used.
	Limit int

	// After specifies the Reddit fullname after which to get items.
	// Used for forward pagination. Format: "t3_abc123" for posts, "t1_def456" for comments.
	// Cannot be used together with Before.
	After string

	// Before specifies the Reddit fullname before which to get items.
	// Used for backward pagination. Format: "t3_abc123" for posts, "t1_def456" for comments.
	// Cannot be used together with After.
	Before string
}

// PostsRequest describes a request to retrieve posts from a subreddit (or the front page).
// The Subreddit field can be left blank to target the front page.
type PostsRequest struct {
	Subreddit string
	Pagination
}

// CommentsRequest describes a request to retrieve comments for a specific post.
type CommentsRequest struct {
	Subreddit string
	PostID    string
	Pagination
}

// MoreCommentsRequest describes a request to expand previously truncated comment trees.
// Pass the post identifier (link) together with the comment identifiers you want to load.
type MoreCommentsRequest struct {
	LinkID     string
	CommentIDs []string

	// Sort specifies the comment sort order.
	// Valid values: "confidence" (default), "new", "top", "controversial", "old", "qa".
	Sort string

	// Depth specifies the maximum depth of comment replies to retrieve.
	// 0 means no limit, 1 means only top-level comments, 2 means one level of replies, etc.
	Depth int

	// Limit specifies the maximum number of comments to retrieve.
	// Reddit's default is 100. Setting this too high may cause timeouts.
	Limit int
}

// SubredditData contains the data for a Subreddit.
type SubredditData struct {
	ThingData
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
	ThingData
	Created
	Author           string          `json:"author"`
	Body             string          `json:"body"`
	BodyHTML         string          `json:"body_html"`
	Context          string          `json:"context"`
	FirstMessage     *int64          `json:"first_message"`
	FirstMessageName *string         `json:"first_message_name"`
	Likes            *bool           `json:"likes"`
	LinkTitle        string          `json:"link_title"`
	New              bool            `json:"new"`
	ParentID         *string         `json:"parent_id"`
	RepliesData      json.RawMessage `json:"replies"` // Raw replies data, handled separately
	Subject          string          `json:"subject"`
	Subreddit        *string         `json:"subreddit"`
	WasComment       bool            `json:"was_comment"`
}

// AccountData contains the data for a user Account.
type AccountData struct {
	ThingData
	Created
	CommentKarma     int    `json:"comment_karma"`
	HasMail          *bool  `json:"has_mail"`
	HasModMail       *bool  `json:"has_mod_mail"`
	HasVerifiedEmail *bool  `json:"has_verified_email"`
	InboxCount       int    `json:"inbox_count,omitempty"`
	IsFriend         bool   `json:"is_friend"`
	IsGold           bool   `json:"is_gold"`
	IsMod            bool   `json:"is_mod"`
	LinkKarma        int    `json:"link_karma"`
	Modhash          string `json:"modhash,omitempty"`
	Over18           bool   `json:"over_18"`
}

// MoreData represents a "more" object, used for comment pagination.
type MoreData struct {
	ThingData
	Children []string `json:"children"`
}

// Post represents a Reddit post with all its fields
type Post struct {
	ThingData
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
	Edited              Edited          `json:"edited"` // Can be a boolean or a float64 timestamp
	Distinguished       *string         `json:"distinguished"`
	Stickied            bool            `json:"stickied"`
}

// Comment represents a Reddit comment with all its fields
type Comment struct {
	ThingData
	Votable
	Created
	ApprovedBy          *string    `json:"approved_by"`
	Author              string     `json:"author"`
	AuthorFlairCSSClass *string    `json:"author_flair_css_class"`
	AuthorFlairText     *string    `json:"author_flair_text"`
	BannedBy            *string    `json:"banned_by"`
	Body                string     `json:"body"`
	BodyHTML            string     `json:"body_html"`
	Edited              Edited     `json:"edited"` // Can be a boolean (for old comments) or a float64 timestamp
	Gilded              int        `json:"gilded"`
	LinkAuthor          string     `json:"link_author,omitempty"`
	LinkID              string     `json:"link_id"`
	LinkTitle           string     `json:"link_title,omitempty"`
	LinkURL             string     `json:"link_url,omitempty"`
	NumReports          *int       `json:"num_reports"`
	ParentID            string     `json:"parent_id"`
	Replies             []*Comment `json:"-"` // Parsed by Parser from the raw replies field
	Saved               bool       `json:"saved"`
	Score               int        `json:"score"`
	ScoreHidden         bool       `json:"score_hidden"`
	Subreddit           string     `json:"subreddit"`
	SubredditID         string     `json:"subreddit_id"`
	Distinguished       *string    `json:"distinguished"`
	MoreChildrenIDs     []string   `json:"-"` // Aggregated IDs for deferred comment loading
}

// PostsResponse represents a collection of posts from a subreddit with pagination info.
type PostsResponse struct {
	Posts          []*Post
	AfterFullname  string // Reddit fullname (e.g. "t3_abc123") of last item for next page
	BeforeFullname string // Reddit fullname (e.g. "t3_abc123") of first item for prev page
}

// CommentsResponse represents a post with its comments and more IDs for loading truncated comments.
type CommentsResponse struct {
	Post           *Post
	Comments       []*Comment
	MoreIDs        []string // IDs of additional comments that can be loaded
	AfterFullname  string   // Reddit fullname (e.g. "t1_abc123") of last comment for next page
	BeforeFullname string   // Reddit fullname (e.g. "t1_abc123") of first comment for prev page
}
