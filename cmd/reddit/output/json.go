package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// jsonFormatter provides JSON output for Reddit API responses.
type jsonFormatter struct {
	w io.Writer
}

// newJSONFormatter creates a new JSON formatter.
func newJSONFormatter(w io.Writer) *jsonFormatter {
	return &jsonFormatter{w: w}
}

// FormatPosts formats a collection of posts as JSON.
func (f *jsonFormatter) FormatPosts(posts []*types.Post) error {
	return f.encodeJSON(posts)
}

// FormatPost formats a single post as JSON.
func (f *jsonFormatter) FormatPost(post *types.Post) error {
	return f.encodeJSON(post)
}

// FormatComments formats a post with its comments as JSON.
func (f *jsonFormatter) FormatComments(response *types.CommentsResponse) error {
	return f.encodeJSON(response)
}

// FormatSubreddit formats subreddit information as JSON.
func (f *jsonFormatter) FormatSubreddit(sub *types.SubredditData) error {
	return f.encodeJSON(sub)
}

// FormatUser formats user account information as JSON.
func (f *jsonFormatter) FormatUser(user *types.AccountData) error {
	return f.encodeJSON(user)
}

// encodeJSON encodes the given value as pretty-printed JSON.
func (f *jsonFormatter) encodeJSON(v interface{}) error {
	encoder := json.NewEncoder(f.w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}
