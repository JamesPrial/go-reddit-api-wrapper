package internal

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/validation"
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

	// HTTP timeout constants
	MinimumTimeout                 = 1 * time.Second
	MaximumTimeoutWarningThreshold = 5 * time.Minute
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

	// Use regex validator first
	if !validation.IsValidSubreddit(name) {
		if len(name) < minSubredditLength {
			return &pkgerrs.ConfigError{Field: "subreddit", Message: fmt.Sprintf("subreddit name must be at least %d characters", minSubredditLength)}
		}
		if len(name) > maxSubredditLength {
			return &pkgerrs.ConfigError{Field: "subreddit", Message: fmt.Sprintf("subreddit name cannot exceed %d characters", maxSubredditLength)}
		}
		return &pkgerrs.ConfigError{Field: "subreddit", Message: "subreddit name contains invalid characters (only letters, numbers, and underscores allowed)"}
	}

	// Additional stricter checks beyond regex
	// Check for Reddit naming constraints
	firstChar := rune(name[0])
	if firstChar == '_' || rune(name[len(name)-1]) == '_' {
		return &pkgerrs.ConfigError{Field: "subreddit", Message: "subreddit name cannot start or end with underscore"}
	}

	// Check for consecutive underscores
	prevWasUnderscore := false
	for i, ch := range name {
		if ch == '_' {
			if prevWasUnderscore {
				return &pkgerrs.ConfigError{Field: "subreddit", Message: fmt.Sprintf("subreddit name cannot contain consecutive underscores at position %d", i)}
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
	// Validate After token if present
	if pagination.After != "" {
		if err := v.ValidatePaginationToken(pagination.After); err != nil {
			return &pkgerrs.ConfigError{Field: "pagination.After", Message: fmt.Sprintf("invalid pagination token: %v", err)}
		}
	}
	// Validate Before token if present
	if pagination.Before != "" {
		if err := v.ValidatePaginationToken(pagination.Before); err != nil {
			return &pkgerrs.ConfigError{Field: "pagination.Before", Message: fmt.Sprintf("invalid pagination token: %v", err)}
		}
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

// ValidateLinkID validates and normalizes a Reddit link ID (post ID).
// It checks for proper formatting and adds the "t3_" prefix if not present.
// Returns the normalized link ID with the "t3_" prefix, or an error if invalid.
func (v *Validator) ValidateLinkID(linkID string) (string, error) {
	if linkID == "" {
		return "", &pkgerrs.ConfigError{
			Field:   "LinkID",
			Message: "link ID is required",
		}
	}

	// Add t3_ prefix if not present, but validate if it is
	if strings.HasPrefix(linkID, "t3_") {
		if len(linkID) <= 3 {
			return "", &pkgerrs.ConfigError{
				Field:   "LinkID",
				Message: "link ID has t3_ prefix but no content after",
			}
		}
		// Validate full fullname format
		if !validation.IsValidFullname(linkID) {
			return "", &pkgerrs.ConfigError{
				Field:   "LinkID",
				Message: fmt.Sprintf("link ID has invalid format: %s", linkID),
			}
		}
		return linkID, nil
	}

	// Check for wrong prefix (e.g., t1_, t5_)
	if strings.Contains(linkID, "_") && (strings.HasPrefix(linkID, "t1_") ||
		strings.HasPrefix(linkID, "t2_") || strings.HasPrefix(linkID, "t4_") ||
		strings.HasPrefix(linkID, "t5_") || strings.HasPrefix(linkID, "t6_")) {
		return "", &pkgerrs.ConfigError{
			Field:   "LinkID",
			Message: fmt.Sprintf("link ID has wrong type prefix, expected t3_ for posts but got: %s", linkID[:3]),
		}
	}

	// Validate base36 format before adding prefix
	if !validation.IsValidBase36(linkID) {
		return "", &pkgerrs.ConfigError{
			Field:   "LinkID",
			Message: fmt.Sprintf("link ID has invalid format (must be base36): %s", linkID),
		}
	}

	// Add the t3_ prefix
	return "t3_" + linkID, nil
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

	// Use base36 validator
	if !validation.IsValidBase36(id) {
		return fmt.Errorf("comment ID has invalid format (must be base36: 0-9, a-z)")
	}

	return nil
}

// ValidateConfig validates the configuration fields and returns the validated/defaulted httpClient.
// Returns an error if validation fails.
func (v *Validator) ValidateConfig(clientID, clientSecret, userAgent string, httpClient *http.Client, logger *slog.Logger, defaultTimeout time.Duration) (*http.Client, error) {
	// Validate required fields
	if clientID == "" || clientSecret == "" {
		return nil, &pkgerrs.ConfigError{Message: "ClientID and ClientSecret are required"}
	}

	// Validate user agent (should already be set by caller)
	if err := v.ValidateUserAgent(userAgent); err != nil {
		return nil, &pkgerrs.ConfigError{
			Field:   "UserAgent",
			Message: fmt.Sprintf("invalid user agent: %v", err),
		}
	}

	// Set default HTTP client if not provided
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	} else if httpClient.Timeout == 0 {
		// Create a shallow copy to avoid mutating the user's client
		clientCopy := *httpClient
		clientCopy.Timeout = defaultTimeout
		httpClient = &clientCopy
		if logger != nil {
			logger.Warn("HTTPClient timeout was 0, setting to default",
				slog.Duration("timeout", defaultTimeout))
		}
	} else if httpClient.Timeout < MinimumTimeout {
		// Validate that timeout is not unreasonably short
		return nil, &pkgerrs.ConfigError{
			Field:   "HTTPClient.Timeout",
			Message: fmt.Sprintf("timeout too short: %v (minimum %v)", httpClient.Timeout, MinimumTimeout),
		}
	} else if httpClient.Timeout > MaximumTimeoutWarningThreshold {
		// Warn about very long timeouts
		if logger != nil {
			logger.Warn("HTTPClient timeout may be too long",
				slog.Duration("timeout", httpClient.Timeout))
		}
	}

	return httpClient, nil
}

// ValidatePostID validates a post ID is valid base36 format (without prefix).
// This is stricter than ValidateLinkID - it does not accept or add prefixes.
func (v *Validator) ValidatePostID(postID string) error {
	if postID == "" {
		return &pkgerrs.ConfigError{
			Field:   "PostID",
			Message: "post ID is required",
		}
	}

	// Validate base36 format (lowercase alphanumeric only)
	if !validation.IsValidBase36(postID) {
		return &pkgerrs.ConfigError{
			Field:   "PostID",
			Message: fmt.Sprintf("post ID has invalid format (must be base36: 0-9, a-z): %s", postID),
		}
	}

	// Check for reasonable length
	if len(postID) > maxCommentIDLength {
		return &pkgerrs.ConfigError{
			Field:   "PostID",
			Message: fmt.Sprintf("post ID too long (max %d characters): %s", maxCommentIDLength, postID),
		}
	}

	return nil
}

// ValidatePaginationToken validates that a pagination token (after/before) is a valid Reddit fullname.
func (v *Validator) ValidatePaginationToken(token string) error {
	if token == "" {
		return fmt.Errorf("pagination token cannot be empty")
	}

	// Validate fullname format (e.g., t3_abc123, t1_def456)
	if !validation.IsValidFullname(token) {
		return fmt.Errorf("pagination token has invalid fullname format (expected t[1-6]_[base36]): %s", token)
	}

	return nil
}

// ValidateURL validates that a URL is a valid HTTP/HTTPS URL without protocol injection risks.
func (v *Validator) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Ensure scheme is http or https only (prevent javascript:, file:, etc.)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	// Ensure host is present
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
	}

	// Check for suspicious patterns that could indicate injection
	if strings.ContainsAny(urlStr, "\r\n") {
		return fmt.Errorf("URL cannot contain newline characters")
	}

	return nil
}
