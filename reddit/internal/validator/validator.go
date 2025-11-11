package validator

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/validation"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/client"
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
		return &ValidationError{Field: "subreddit", Value: "", Reason: "subreddit name cannot be empty"}
	}

	// Use regex validator first
	if !validation.IsValidSubreddit(name) {
		if len(name) < minSubredditLength {
			return &ValidationError{Field: "subreddit", Value: name, Reason: fmt.Sprintf("subreddit name must be at least %d characters", minSubredditLength)}
		}
		if len(name) > maxSubredditLength {
			return &ValidationError{Field: "subreddit", Value: name, Reason: fmt.Sprintf("subreddit name cannot exceed %d characters", maxSubredditLength)}
		}
		return &ValidationError{Field: "subreddit", Value: name, Reason: "subreddit name contains invalid characters (only letters, numbers, and underscores allowed)"}
	}

	// Additional stricter checks beyond regex
	// Check for Reddit naming constraints
	firstChar := rune(name[0])
	if firstChar == '_' || rune(name[len(name)-1]) == '_' {
		return &ValidationError{Field: "subreddit", Value: name, Reason: "subreddit name cannot start or end with underscore"}
	}

	// Check for consecutive underscores
	prevWasUnderscore := false
	for i, ch := range name {
		if ch == '_' {
			if prevWasUnderscore {
				return &ValidationError{Field: "subreddit", Value: name, Reason: fmt.Sprintf("subreddit name cannot contain consecutive underscores at position %d", i)}
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
		return &ValidationError{Field: "pagination", Value: "", Reason: "cannot set both After and Before pagination parameters"}
	}
	// Validate After token if present
	if pagination.After != "" {
		if err := v.ValidatePaginationToken(pagination.After); err != nil {
			return &ValidationError{Field: "pagination.After", Value: pagination.After, Reason: "invalid pagination token", Err: err}
		}
	}
	// Validate Before token if present
	if pagination.Before != "" {
		if err := v.ValidatePaginationToken(pagination.Before); err != nil {
			return &ValidationError{Field: "pagination.Before", Value: pagination.Before, Reason: "invalid pagination token", Err: err}
		}
	}
	// Validate limit range
	if pagination.Limit < 0 {
		return &ValidationError{Field: "pagination.Limit", Value: fmt.Sprintf("%d", pagination.Limit), Reason: "limit cannot be negative"}
	}
	if pagination.Limit > maxPaginationLimit {
		return &ValidationError{Field: "pagination.Limit", Value: fmt.Sprintf("%d", pagination.Limit), Reason: fmt.Sprintf("limit cannot exceed %d", maxPaginationLimit)}
	}
	return nil
}

// ValidateCommentIDs checks if the comment IDs slice is within Reddit's API limits.
// Returns an error if there are too many IDs or if any ID is invalid.
func (v *Validator) ValidateCommentIDs(ids []string) error {
	if len(ids) > maxCommentIDs {
		return &ValidationError{Field: "CommentIDs", Value: fmt.Sprintf("%d", len(ids)), Reason: fmt.Sprintf("cannot request more than %d comment IDs at once (got %d)", maxCommentIDs, len(ids))}
	}

	// Validate each comment ID content
	for i, id := range ids {
		if err := validateCommentID(id); err != nil {
			return &ValidationError{
				Field:  fmt.Sprintf("CommentIDs[%d]", i),
				Value:  id,
				Reason: fmt.Sprintf("invalid comment ID at index %d", i),
				Err:    err,
			}
		}
	}

	return nil
}

// ValidateUserAgent validates the User-Agent string to prevent header injection attacks.
func (v *Validator) ValidateUserAgent(ua string) error {
	// User-Agent cannot be empty (should have been set to default before this check)
	if len(ua) == 0 {
		return &ValidationError{Field: "UserAgent", Value: "", Reason: "user agent cannot be empty"}
	}

	// Check for newline characters that could be used for header injection
	if strings.ContainsAny(ua, "\r\n") {
		return &ValidationError{Field: "UserAgent", Value: ua, Reason: "user agent cannot contain newline characters"}
	}

	// User-Agent should have a reasonable maximum length
	if len(ua) > maxUserAgentLength {
		return &ValidationError{Field: "UserAgent", Value: ua, Reason: fmt.Sprintf("user agent too long (max %d characters)", maxUserAgentLength)}
	}

	return nil
}

// ValidateLinkID validates and normalizes a Reddit link ID (post ID).
// It checks for proper formatting and adds the "t3_" prefix if not present.
// Returns the normalized link ID with the "t3_" prefix, or an error if invalid.
func (v *Validator) ValidateLinkID(linkID string) (string, error) {
	if linkID == "" {
		return "", &ValidationError{
			Field:  "LinkID",
			Value:  "",
			Reason: "link ID is required",
		}
	}

	// Add t3_ prefix if not present, but validate if it is
	if strings.HasPrefix(linkID, "t3_") {
		if len(linkID) <= 3 {
			return "", &ValidationError{
				Field:  "LinkID",
				Value:  linkID,
				Reason: "link ID has t3_ prefix but no content after",
			}
		}
		// Validate full fullname format
		if !validation.IsValidFullname(linkID) {
			return "", &ValidationError{
				Field:  "LinkID",
				Value:  linkID,
				Reason: "link ID has invalid format",
			}
		}
		return linkID, nil
	}

	// Check for wrong prefix (e.g., t1_, t5_)
	if strings.HasPrefix(linkID, "t1_") || strings.HasPrefix(linkID, "t2_") ||
		strings.HasPrefix(linkID, "t4_") || strings.HasPrefix(linkID, "t5_") ||
		strings.HasPrefix(linkID, "t6_") {
		return "", &ValidationError{
			Field:  "LinkID",
			Value:  linkID,
			Reason: fmt.Sprintf("link ID has wrong type prefix, expected t3_ for posts but got: %s", linkID[:3]),
		}
	}

	// Validate base36 format before adding prefix
	if !validation.IsValidBase36(linkID) {
		return "", &ValidationError{
			Field:  "LinkID",
			Value:  linkID,
			Reason: "link ID has invalid format (must be base36)",
		}
	}

	// Add the t3_ prefix
	return "t3_" + linkID, nil
}

// validateCommentID validates the format and content of a single comment ID.
// This is an internal helper function used by ValidateCommentIDs.
func validateCommentID(id string) error {
	if len(id) == 0 {
		return &ValidationError{Field: "CommentID", Value: "", Reason: "comment ID cannot be empty"}
	}

	// Reddit comment IDs have a reasonable maximum length (typically 6-10 characters)
	if len(id) > maxCommentIDLength {
		return &ValidationError{Field: "CommentID", Value: id, Reason: fmt.Sprintf("comment ID too long (max %d characters)", maxCommentIDLength)}
	}

	// Use base36 validator
	if !validation.IsValidBase36(id) {
		return &ValidationError{Field: "CommentID", Value: id, Reason: "comment ID has invalid format (must be base36: 0-9, a-z)"}
	}

	return nil
}

// ValidateConfig validates the configuration fields and returns the validated/defaulted httpClient.
// Returns an error if validation fails.
func (v *Validator) ValidateConfig(clientID, clientSecret, userAgent string, httpClient *http.Client, logger *slog.Logger, defaultTimeout time.Duration) (*http.Client, error) {
	// Validate required fields
	if clientID == "" || clientSecret == "" {
		return nil, &ValidationError{Field: "Credentials", Value: "", Reason: "ClientID and ClientSecret are required"}
	}

	// Validate user agent (should already be set by caller)
	if err := v.ValidateUserAgent(userAgent); err != nil {
		return nil, &ValidationError{
			Field:  "UserAgent",
			Reason: "invalid user agent",
			Err:    err,
		}
	}

	// Set default HTTP client if not provided
	if httpClient == nil {
		// Create HTTP client with optimized transport for connection pooling and HTTP/2.
		// Metrics are not captured here since this is the default client creation path.
		// Users who need metrics should create their own transport and call SetTransportMetrics.
		transport, _ := client.NewOptimizedTransport(nil)
		httpClient = &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		}
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
		return nil, &ValidationError{
			Field:  "HTTPClient.Timeout",
			Value:  httpClient.Timeout.String(),
			Reason: fmt.Sprintf("timeout too short: %v (minimum %v)", httpClient.Timeout, MinimumTimeout),
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
		return &ValidationError{
			Field:  "PostID",
			Value:  "",
			Reason: "post ID is required",
		}
	}

	// Check for reasonable length BEFORE format check for better error messages
	if len(postID) > maxCommentIDLength {
		return &ValidationError{
			Field:  "PostID",
			Value:  postID,
			Reason: fmt.Sprintf("post ID too long (max %d characters)", maxCommentIDLength),
		}
	}

	// Validate base36 format (lowercase alphanumeric only)
	if !validation.IsValidBase36(postID) {
		return &ValidationError{
			Field:  "PostID",
			Value:  postID,
			Reason: "post ID has invalid format (must be base36: 0-9, a-z)",
		}
	}

	return nil
}

// ValidatePaginationToken validates that a pagination token (after/before) is a valid Reddit fullname.
func (v *Validator) ValidatePaginationToken(token string) error {
	if token == "" {
		return &ValidationError{Field: "PaginationToken", Value: "", Reason: "pagination token cannot be empty"}
	}

	// Validate fullname format (e.g., t3_abc123, t1_def456)
	if !validation.IsValidFullname(token) {
		return &ValidationError{Field: "PaginationToken", Value: token, Reason: "pagination token has invalid fullname format (expected t[1-6]_[base36])"}
	}

	return nil
}

// ValidateURL validates that a URL is a valid HTTP/HTTPS URL without protocol injection risks.
func (v *Validator) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return &ValidationError{Field: "URL", Value: "", Reason: "URL cannot be empty"}
	}

	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &ValidationError{Field: "URL", Value: urlStr, Reason: "invalid URL format", Err: err}
	}

	// Ensure scheme is http or https only (prevent javascript:, file:, etc.)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ValidationError{Field: "URL", Value: urlStr, Reason: fmt.Sprintf("URL must use http or https scheme, got: %s", parsedURL.Scheme)}
	}

	// Ensure host is present
	if parsedURL.Host == "" {
		return &ValidationError{Field: "URL", Value: urlStr, Reason: "URL must have a valid host"}
	}

	// Check for suspicious patterns that could indicate injection
	if strings.ContainsAny(urlStr, "\r\n") {
		return &ValidationError{Field: "URL", Value: urlStr, Reason: "URL cannot contain newline characters"}
	}

	return nil
}
