package internal

import (
	"fmt"
	"strings"

	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

const (
	// Subreddit name constraints
	minSubredditLength = 3
	maxSubredditLength = 21

	// Pagination constraints
	maxPaginationLimit = 100

	// Comment ID constraints
	maxCommentIDs      = 100
	maxCommentIDLength = 100

	// User agent constraints
	maxUserAgentLength = 256
)

// Validator provides validation operations for Reddit API parameters.
type Validator struct{}

// NewValidator creates a new Validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateSubredditName checks if a subreddit name is valid according to Reddit's naming rules.
// Returns an error if the name is invalid.
func (v *Validator) ValidateSubredditName(name string) error {
	if name == "" {
		return &pkgerrs.ConfigError{Field: "subreddit", Message: "subreddit name cannot be empty"}
	}
	if len(name) < minSubredditLength {
		return &pkgerrs.ConfigError{Field: "subreddit", Message: fmt.Sprintf("subreddit name must be at least %d characters", minSubredditLength)}
	}
	if len(name) > maxSubredditLength {
		return &pkgerrs.ConfigError{Field: "subreddit", Message: fmt.Sprintf("subreddit name cannot exceed %d characters", maxSubredditLength)}
	}
	// Check for Reddit naming constraints
	firstChar := rune(name[0])
	if firstChar == '_' || rune(name[len(name)-1]) == '_' {
		return &pkgerrs.ConfigError{Field: "subreddit", Message: "subreddit name cannot start or end with underscore"}
	}
	// Check for valid characters: letters, numbers, underscores only
	prevWasUnderscore := false
	for i, ch := range name {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '_' {
			return &pkgerrs.ConfigError{Field: "subreddit", Message: fmt.Sprintf("subreddit name contains invalid character '%c' at position %d", ch, i)}
		}
		if ch == '_' {
			if prevWasUnderscore {
				return &pkgerrs.ConfigError{Field: "subreddit", Message: "subreddit name cannot contain consecutive underscores"}
			}
			prevWasUnderscore = true
		} else {
			prevWasUnderscore = false
		}
	}
	return nil
}

// ValidatePagination checks if pagination parameters are valid.
// Returns an error if the parameters are invalid.
func (v *Validator) ValidatePagination(pagination *types.Pagination) error {
	if pagination == nil {
		return nil
	}
	// Reddit API doesn't allow both After and Before to be set
	if pagination.After != "" && pagination.Before != "" {
		return &pkgerrs.ConfigError{Field: "pagination", Message: "cannot set both After and Before pagination parameters"}
	}
	// Validate limit range
	if pagination.Limit < 0 {
		return &pkgerrs.ConfigError{Field: "pagination.Limit", Message: "limit cannot be negative"}
	}
	if pagination.Limit > maxPaginationLimit {
		return &pkgerrs.ConfigError{Field: "pagination.Limit", Message: fmt.Sprintf("limit cannot exceed %d", maxPaginationLimit)}
	}
	return nil
}

// ValidateCommentIDs checks if the comment IDs slice is within Reddit's API limits.
// Returns an error if there are too many IDs or if any ID is invalid.
func (v *Validator) ValidateCommentIDs(ids []string) error {
	if len(ids) > maxCommentIDs {
		return &pkgerrs.ConfigError{Field: "CommentIDs", Message: fmt.Sprintf("cannot request more than %d comment IDs at once (got %d)", maxCommentIDs, len(ids))}
	}

	// Validate each comment ID content
	for i, id := range ids {
		if err := validateCommentID(id); err != nil {
			return &pkgerrs.ConfigError{
				Field:   fmt.Sprintf("CommentIDs[%d]", i),
				Message: fmt.Sprintf("invalid comment ID at index %d: %v", i, err),
			}
		}
	}

	return nil
}

// ValidateUserAgent validates the User-Agent string to prevent header injection attacks.
func (v *Validator) ValidateUserAgent(ua string) error {
	// User-Agent cannot be empty (should have been set to default before this check)
	if len(ua) == 0 {
		return fmt.Errorf("user agent cannot be empty")
	}

	// Check for newline characters that could be used for header injection
	if strings.ContainsAny(ua, "\r\n") {
		return fmt.Errorf("user agent cannot contain newline characters")
	}

	// User-Agent should have a reasonable maximum length
	if len(ua) > maxUserAgentLength {
		return fmt.Errorf("user agent too long (max %d characters)", maxUserAgentLength)
	}

	return nil
}

// validateCommentID validates the format and content of a single comment ID.
// This is an internal helper function used by ValidateCommentIDs.
func validateCommentID(id string) error {
	if len(id) == 0 {
		return fmt.Errorf("comment ID cannot be empty")
	}

	// Reddit comment IDs have a reasonable maximum length (typically 6-10 characters)
	if len(id) > maxCommentIDLength {
		return fmt.Errorf("comment ID too long (max %d characters)", maxCommentIDLength)
	}

	// Reddit comment IDs are alphanumeric base36 strings
	// They should only contain: 0-9, a-z, A-Z
	for _, char := range id {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z')) {
			return fmt.Errorf("comment ID contains invalid character: %c (only alphanumeric allowed)", char)
		}
	}

	return nil
}
