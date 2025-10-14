// Package validation provides security-focused validation utilities for Reddit API data.
//
// This package includes:
//   - Format validators for Reddit-specific data types (base36 IDs, fullnames, permalinks, usernames, subreddits)
//   - Struct validators for Reddit API response types (Posts, Comments, Subreddits, etc.)
//   - Security protections against DoS attacks (length limits, ReDoS prevention)
//   - Input validation to prevent injection attacks
//
// All validators are designed with security in mind, performing length checks before
// regex matching to prevent resource exhaustion attacks.
package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Clock provides time operations for testing.
// Use this interface to make time-based validation deterministic in tests.
type Clock interface {
	Now() time.Time
}

// realClock implements Clock using the actual system time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// defaultClock is the default clock used by validators.
// Can be overridden in tests using SetClock.
var defaultClock Clock = realClock{}

// SetClock sets the clock used for time validation.
// This is primarily useful for testing. Production code should not call this.
func SetClock(c Clock) {
	defaultClock = c
}

// ResetClock resets the clock to use system time.
// This is primarily useful for test cleanup.
func ResetClock() {
	defaultClock = realClock{}
}

// Security limits to prevent DoS attacks
const (
	// Maximum length for base36 IDs (Reddit IDs are typically 6-10 chars, but we allow more for safety)
	maxBase36Length = 100

	// Maximum length for fullname IDs (type prefix + underscore + ID)
	maxFullnameLength = 110

	// Maximum length for permalinks to prevent ReDoS
	maxPermalinkLength = 500

	// Maximum length for title slugs in permalinks
	maxTitleSlugLength = 200
)

// Regular expressions for validating Reddit data formats
var (
	// base36Regex matches base36 encoded IDs (0-9, a-z)
	// Note: Length is validated separately before regex matching for security
	base36Regex = regexp.MustCompile(`^[0-9a-z]+$`)

	// subredditRegex matches valid subreddit names (3-21 chars, alphanumeric + underscore)
	subredditRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,21}$`)

	// usernameRegex matches valid Reddit usernames (3-20 chars, alphanumeric + underscore + hyphen)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,20}$`)

	// fullnameRegex matches Reddit fullname IDs (type prefix + base36 ID)
	// Format: t[1-6]_[base36_id]
	// Note: Length is validated separately before regex matching for security
	fullnameRegex = regexp.MustCompile(`^t[1-6]_[0-9a-z]+$`)

	// permalinkRegex matches Reddit permalink format
	// Format: /r/{subreddit}/comments/{post_id}/{title_slug}/ or with /{comment_id}/
	// Note: Length is validated separately before regex matching to prevent ReDoS
	permalinkRegex = regexp.MustCompile(`^/r/[a-zA-Z0-9_]{3,21}/comments/[0-9a-z]+/[^/]+/?([0-9a-z]+/?)?$`)
)

// IsValidBase36 checks if a string is a valid base36 encoded ID.
// It performs length validation before regex matching to prevent DoS attacks.
func IsValidBase36(s string) bool {
	if s == "" || len(s) > maxBase36Length {
		return false
	}
	return base36Regex.MatchString(s)
}

// IsValidSubreddit checks if a string is a valid subreddit name
func IsValidSubreddit(s string) bool {
	return subredditRegex.MatchString(s)
}

// IsValidUsername checks if a string is a valid Reddit username
func IsValidUsername(s string) bool {
	return usernameRegex.MatchString(s)
}

// IsValidFullname checks if a string is a valid Reddit fullname ID.
// Format: t[1-6]_[base36_id]
// It performs length validation before regex matching to prevent DoS attacks.
func IsValidFullname(s string) bool {
	if s == "" || len(s) > maxFullnameLength {
		return false
	}
	// Quick structural check before regex
	if len(s) < 4 || s[0] != 't' || s[2] != '_' {
		return false
	}
	return fullnameRegex.MatchString(s)
}

// IsValidPermalink checks if a string is a valid Reddit permalink.
// Format: /r/{subreddit}/comments/{post_id}/{title_slug}/ or with /{comment_id}/
// It performs length validation before regex matching to prevent ReDoS attacks.
func IsValidPermalink(s string) bool {
	if s == "" || len(s) > maxPermalinkLength {
		return false
	}

	// SECURITY: Check for control characters (null bytes, newlines, etc.) to prevent injection attacks
	for _, ch := range s {
		// Reject all control characters (ASCII 0-31) except space (32)
		if ch < 32 || ch == 127 { // 127 is DEL character
			return false
		}
	}

	// Quick structural validation before regex to avoid ReDoS
	if !strings.HasPrefix(s, "/r/") || !strings.Contains(s, "/comments/") {
		return false
	}

	// Validate title slug length separately to prevent DoS
	parts := strings.Split(s, "/")
	// Format: ["", "r", "subreddit", "comments", "postid", "slug", "commentid?", ""]
	// Minimum: /r/sub/comments/id/slug/ = 7 parts (with trailing slash) or 6 without
	// Maximum: /r/sub/comments/id/slug/commentid/ = 8 parts or 7 without trailing slash
	if len(parts) < 6 || len(parts) > 8 {
		return false
	}

	// Check title slug and segment lengths
	for i, part := range parts {
		if i > 0 && len(part) > maxTitleSlugLength {
			return false
		}
	}

	return permalinkRegex.MatchString(s)
}

// ValidateRedditObject validates any type that implements RedditObject interface
func ValidateRedditObject(obj types.RedditObject) error {
	if obj == nil {
		return fmt.Errorf("reddit object is nil")
	}

	var errs []error

	// Validate ID - required and must be valid base36
	id := obj.GetID()
	if id == "" {
		errs = append(errs, fmt.Errorf("ID is required"))
	} else if !IsValidBase36(id) {
		errs = append(errs, fmt.Errorf("ID has invalid format: %s", id))
	}

	// Validate Name (fullname) - if present, must be a valid fullname
	name := obj.GetName()
	if name != "" && !IsValidFullname(name) {
		errs = append(errs, fmt.Errorf("Name has invalid fullname format: %s", name))
	}

	if len(errs) > 0 {
		return fmt.Errorf("reddit object validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateThingData validates the base ThingData fields
func ValidateThingData(td *types.ThingData) error {
	if td == nil {
		return fmt.Errorf("thing data is nil")
	}
	return ValidateRedditObject(td)
}

// ValidateVotable validates the Votable embedded struct
func ValidateVotable(v *types.Votable) error {
	if v == nil {
		return fmt.Errorf("votable is nil")
	}

	var errs []error

	// Score can be negative (downvoted posts/comments)
	// But Ups should match Score (Reddit legacy field)
	if v.Ups != v.Score {
		errs = append(errs, fmt.Errorf("Ups (%d) does not match Score (%d)", v.Ups, v.Score))
	}

	// Downs is always 0 (deprecated by Reddit)
	if v.Downs != 0 {
		errs = append(errs, fmt.Errorf("Downs should be 0, got %d", v.Downs))
	}

	if len(errs) > 0 {
		return fmt.Errorf("votable validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateCreated validates the Created embedded struct.
// It uses the configured Clock for time-based validation, making tests deterministic.
func ValidateCreated(c *types.Created) error {
	if c == nil {
		return fmt.Errorf("created is nil")
	}

	var errs []error

	// Created and CreatedUTC should be the same (Reddit uses UTC)
	if c.Created != c.CreatedUTC {
		errs = append(errs, fmt.Errorf("Created (%f) does not match CreatedUTC (%f)", c.Created, c.CreatedUTC))
	}

	// Validate timestamp is valid (must be positive, non-zero)
	if c.CreatedUTC <= 0 {
		errs = append(errs, fmt.Errorf("CreatedUTC must be positive, got %f", c.CreatedUTC))
	}

	// Check timestamp is not in the future (with 1 hour grace period for clock skew)
	maxTime := float64(defaultClock.Now().Add(time.Hour).Unix())
	if c.CreatedUTC > maxTime {
		errs = append(errs, fmt.Errorf("CreatedUTC is in the future: %f", c.CreatedUTC))
	}

	// Check timestamp is after Reddit's founding (June 2005)
	minTime := float64(time.Date(2005, 6, 1, 0, 0, 0, 0, time.UTC).Unix())
	if c.CreatedUTC < minTime {
		errs = append(errs, fmt.Errorf("CreatedUTC is before Reddit existed: %f", c.CreatedUTC))
	}

	if len(errs) > 0 {
		return fmt.Errorf("created validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidatePost validates a Post struct's fields
func ValidatePost(p *types.Post) error {
	if p == nil {
		return fmt.Errorf("post is nil")
	}

	var errs []error

	// Validate embedded structs
	if err := ValidateThingData(&p.ThingData); err != nil {
		errs = append(errs, err)
	}

	if err := ValidateVotable(&p.Votable); err != nil {
		errs = append(errs, err)
	}

	if err := ValidateCreated(&p.Created); err != nil {
		errs = append(errs, err)
	}

	// Validate title
	if p.Title == "" {
		errs = append(errs, fmt.Errorf("Title is required"))
	} else if len(p.Title) > types.MAX_POST_TITLE_LENGTH {
		errs = append(errs, fmt.Errorf("Title exceeds %d character limit (%d chars)", types.MAX_POST_TITLE_LENGTH, len(p.Title)))
	}

	// Validate subreddit
	if p.Subreddit == "" {
		errs = append(errs, fmt.Errorf("Subreddit is required"))
	} else if !IsValidSubreddit(p.Subreddit) {
		errs = append(errs, fmt.Errorf("Subreddit has invalid format: %s", p.Subreddit))
	}

	// Validate SubredditID
	if p.SubredditID != "" && !IsValidFullname(p.SubredditID) {
		errs = append(errs, fmt.Errorf("SubredditID has invalid fullname format: %s", p.SubredditID))
	}

	// Validate author
	if p.Author == "" {
		errs = append(errs, fmt.Errorf("Author is required"))
	} else if p.Author != "[deleted]" && !IsValidUsername(p.Author) {
		errs = append(errs, fmt.Errorf("Author has invalid username format: %s", p.Author))
	}

	// Validate permalink
	if p.Permalink == "" {
		errs = append(errs, fmt.Errorf("Permalink is required"))
	} else if !IsValidPermalink(p.Permalink) {
		errs = append(errs, fmt.Errorf("Permalink has invalid format: %s", p.Permalink))
	}

	// Validate URL
	if p.URL == "" {
		errs = append(errs, fmt.Errorf("URL is required"))
	}

	// Validate upvote ratio
	if p.UpvoteRatio < 0 || p.UpvoteRatio > 1 {
		errs = append(errs, fmt.Errorf("UpvoteRatio must be between 0 and 1, got %f", p.UpvoteRatio))
	}

	// Validate NumComments
	if p.NumComments < 0 {
		errs = append(errs, fmt.Errorf("NumComments cannot be negative, got %d", p.NumComments))
	}

	if len(errs) > 0 {
		return fmt.Errorf("post validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateComment validates a Comment struct's fields
func ValidateComment(c *types.Comment) error {
	if c == nil {
		return fmt.Errorf("comment is nil")
	}

	var errs []error

	// Validate embedded structs
	if err := ValidateThingData(&c.ThingData); err != nil {
		errs = append(errs, err)
	}

	if err := ValidateVotable(&c.Votable); err != nil {
		errs = append(errs, err)
	}

	if err := ValidateCreated(&c.Created); err != nil {
		errs = append(errs, err)
	}

	// Validate body
	if c.Body == "" {
		errs = append(errs, fmt.Errorf("Body is required"))
	} else if len(c.Body) > types.MAX_COMMENT_BODY_LENGTH {
		errs = append(errs, fmt.Errorf("Body exceeds %d character limit (%d chars)", types.MAX_COMMENT_BODY_LENGTH, len(c.Body)))
	}

	// Validate subreddit
	if c.Subreddit == "" {
		errs = append(errs, fmt.Errorf("Subreddit is required"))
	} else if !IsValidSubreddit(c.Subreddit) {
		errs = append(errs, fmt.Errorf("Subreddit has invalid format: %s", c.Subreddit))
	}

	// Validate SubredditID
	if c.SubredditID != "" && !IsValidFullname(c.SubredditID) {
		errs = append(errs, fmt.Errorf("SubredditID has invalid fullname format: %s", c.SubredditID))
	}

	// Validate author
	if c.Author == "" {
		errs = append(errs, fmt.Errorf("Author is required"))
	} else if c.Author != "[deleted]" && !IsValidUsername(c.Author) {
		errs = append(errs, fmt.Errorf("Author has invalid username format: %s", c.Author))
	}

	// Validate ParentID
	if c.ParentID == "" {
		errs = append(errs, fmt.Errorf("ParentID is required"))
	} else if !IsValidFullname(c.ParentID) {
		errs = append(errs, fmt.Errorf("ParentID has invalid fullname format: %s", c.ParentID))
	}

	// Validate LinkID
	if c.LinkID == "" {
		errs = append(errs, fmt.Errorf("LinkID is required"))
	} else if !IsValidFullname(c.LinkID) {
		errs = append(errs, fmt.Errorf("LinkID has invalid fullname format: %s", c.LinkID))
	}

	if len(errs) > 0 {
		return fmt.Errorf("comment validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateSubredditData validates a SubredditData struct's fields
func ValidateSubredditData(s *types.SubredditData) error {
	if s == nil {
		return fmt.Errorf("subreddit is nil")
	}

	var errs []error

	// Validate embedded ThingData
	if err := ValidateThingData(&s.ThingData); err != nil {
		errs = append(errs, err)
	}

	// Validate DisplayName
	if s.DisplayName == "" {
		errs = append(errs, fmt.Errorf("DisplayName is required"))
	} else if !IsValidSubreddit(s.DisplayName) {
		errs = append(errs, fmt.Errorf("DisplayName has invalid format: %s", s.DisplayName))
	}

	// Validate subscriber count
	if s.Subscribers < 0 {
		errs = append(errs, fmt.Errorf("Subscribers cannot be negative, got %d", s.Subscribers))
	}

	if len(errs) > 0 {
		return fmt.Errorf("subreddit validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateMessageData validates a MessageData struct's fields
func ValidateMessageData(m *types.MessageData) error {
	if m == nil {
		return fmt.Errorf("message is nil")
	}

	var errs []error

	// Validate embedded structs
	if err := ValidateThingData(&m.ThingData); err != nil {
		errs = append(errs, err)
	}

	if err := ValidateCreated(&m.Created); err != nil {
		errs = append(errs, err)
	}

	// Validate body
	if m.Body == "" {
		errs = append(errs, fmt.Errorf("Body is required"))
	}

	// Validate author
	if m.Author == "" {
		errs = append(errs, fmt.Errorf("Author is required"))
	} else if m.Author != "[deleted]" && !IsValidUsername(m.Author) {
		errs = append(errs, fmt.Errorf("Author has invalid username format: %s", m.Author))
	}

	// Validate subject
	if m.Subject == "" {
		errs = append(errs, fmt.Errorf("Subject is required"))
	}

	// Validate ParentID if present
	if m.ParentID != nil && *m.ParentID != "" && !IsValidFullname(*m.ParentID) {
		errs = append(errs, fmt.Errorf("ParentID has invalid fullname format: %s", *m.ParentID))
	}

	if len(errs) > 0 {
		return fmt.Errorf("message validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateAccountData validates an AccountData struct's fields
func ValidateAccountData(a *types.AccountData) error {
	if a == nil {
		return fmt.Errorf("account is nil")
	}

	var errs []error

	// Validate embedded structs
	if err := ValidateThingData(&a.ThingData); err != nil {
		errs = append(errs, err)
	}

	if err := ValidateCreated(&a.Created); err != nil {
		errs = append(errs, err)
	}

	// Validate karma counts
	if a.CommentKarma < 0 {
		errs = append(errs, fmt.Errorf("CommentKarma cannot be negative, got %d", a.CommentKarma))
	}

	if a.LinkKarma < 0 {
		errs = append(errs, fmt.Errorf("LinkKarma cannot be negative, got %d", a.LinkKarma))
	}

	if len(errs) > 0 {
		return fmt.Errorf("account validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// ValidateMoreData validates a MoreData struct's fields
func ValidateMoreData(m *types.MoreData) error {
	if m == nil {
		return fmt.Errorf("more data is nil")
	}

	var errs []error

	// Validate embedded ThingData
	if err := ValidateThingData(&m.ThingData); err != nil {
		errs = append(errs, err)
	}

	// Validate children IDs
	for i, childID := range m.Children {
		if !IsValidBase36(childID) {
			errs = append(errs, fmt.Errorf("Child ID at index %d has invalid format: %s", i, childID))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("more data validation failed: %w", joinValidationErrors(errs))
	}

	return nil
}

// joinValidationErrors combines multiple errors into a single error message
func joinValidationErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}

	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}
