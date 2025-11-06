// Package output provides output formatting for the Reddit CLI.
package output

import (
	"fmt"
	"io"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// Formatter defines the interface for formatting Reddit API responses for output.
type Formatter interface {
	// FormatPosts formats a collection of posts.
	FormatPosts(posts []*types.Post) error

	// FormatPost formats a single post.
	FormatPost(post *types.Post) error

	// FormatComments formats a post with its comments.
	FormatComments(response *types.CommentsResponse) error

	// FormatSubreddit formats subreddit information.
	FormatSubreddit(sub *types.SubredditData) error

	// FormatUser formats user account information.
	FormatUser(user *types.AccountData) error
}

// Config holds configuration for the formatter.
type Config struct {
	// Writer is where formatted output is written.
	Writer io.Writer

	// Format specifies the output format ("text", "json", "table").
	Format string

	// ColorEnabled controls whether to use colored output (for text format).
	ColorEnabled bool

	// Compact controls whether to use compact formatting (for text format).
	Compact bool
}

// New returns a new formatter based on the specified format.
// Supported formats: "text" (default), "json", "table".
// Returns an error if the format is not supported.
func New(cfg Config) (Formatter, error) {
	if cfg.Writer == nil {
		return nil, fmt.Errorf("writer cannot be nil")
	}

	switch cfg.Format {
	case "text", "":
		return newTextFormatter(cfg.Writer, cfg.ColorEnabled, cfg.Compact), nil
	case "json":
		return newJSONFormatter(cfg.Writer), nil
	case "table":
		return newTableFormatter(cfg.Writer), nil
	default:
		return nil, fmt.Errorf("unsupported format: %q (supported: text, json, table)", cfg.Format)
	}
}
