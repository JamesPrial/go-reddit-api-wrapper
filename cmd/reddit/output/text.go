package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// textFormatter provides human-readable text output for Reddit API responses.
type textFormatter struct {
	w           io.Writer
	colorEnable bool
	compact     bool
}

// newTextFormatter creates a new text formatter.
func newTextFormatter(w io.Writer, colorEnable, compact bool) *textFormatter {
	return &textFormatter{
		w:           w,
		colorEnable: colorEnable,
		compact:     compact,
	}
}

// FormatPosts formats a collection of posts.
func (f *textFormatter) FormatPosts(posts []*types.Post) error {
	if len(posts) == 0 {
		_, err := fmt.Fprintf(f.w, "No posts found.\n")
		return err
	}

	for i, post := range posts {
		if i > 0 {
			_, err := fmt.Fprintf(f.w, "\n")
			if err != nil {
				return err
			}
		}
		if err := f.FormatPost(post); err != nil {
			return err
		}
	}

	return nil
}

// FormatPost formats a single post.
func (f *textFormatter) FormatPost(post *types.Post) error {
	if post == nil {
		_, err := fmt.Fprintf(f.w, "Post is nil.\n")
		return err
	}

	sections := []string{
		f.formatPostTitle(post),
		f.formatPostMetadata(post),
		f.formatPostContent(post),
		f.formatPostStats(post),
	}

	return f.writeSection(sections)
}

// FormatComments formats a post with its comments.
func (f *textFormatter) FormatComments(response *types.CommentsResponse) error {
	if response == nil {
		_, err := fmt.Fprintf(f.w, "Response is nil.\n")
		return err
	}

	if response.Post != nil {
		if err := f.FormatPost(response.Post); err != nil {
			return err
		}
		_, err := fmt.Fprintf(f.w, "\n%s\n\n", strings.Repeat("-", 80))
		if err != nil {
			return err
		}
	}

	if len(response.Comments) == 0 {
		_, err := fmt.Fprintf(f.w, "No comments found.\n")
		return err
	}

	for i, comment := range response.Comments {
		if i > 0 {
			_, err := fmt.Fprintf(f.w, "\n")
			if err != nil {
				return err
			}
		}
		if err := f.formatComment(comment, 0); err != nil {
			return err
		}
	}

	if len(response.MoreIDs) > 0 {
		_, err := fmt.Fprintf(f.w, "\n... and %d more comments (use GetMoreComments to load)\n", len(response.MoreIDs))
		if err != nil {
			return err
		}
	}

	return nil
}

// FormatSubreddit formats subreddit information.
func (f *textFormatter) FormatSubreddit(sub *types.SubredditData) error {
	if sub == nil {
		_, err := fmt.Fprintf(f.w, "Subreddit is nil.\n")
		return err
	}

	lines := []string{
		fmt.Sprintf("Subreddit: %s", sub.DisplayName),
		fmt.Sprintf("URL: %s", sub.URL),
		"",
		f.formatOptionalString("Title", sub.Title),
		f.formatOptionalString("Description", sub.PublicDescription),
		"",
		fmt.Sprintf("Subscribers: %s", f.formatNumber(sub.Subscribers)),
		fmt.Sprintf("Active Users: %d", sub.AccountsActive),
		fmt.Sprintf("Type: %s", sub.SubredditType),
		fmt.Sprintf("Over 18: %v", sub.Over18),
		"",
	}

	if sub.UserIsSubscriber != nil && *sub.UserIsSubscriber {
		lines = append(lines, "Status: You are subscribed")
	}
	if sub.UserIsModerator != nil && *sub.UserIsModerator {
		lines = append(lines, "Status: You are a moderator")
	}
	if sub.UserIsContributor != nil && *sub.UserIsContributor {
		lines = append(lines, "Status: You are a contributor")
	}

	return f.writeLines(lines)
}

// FormatUser formats user account information.
func (f *textFormatter) FormatUser(user *types.AccountData) error {
	if user == nil {
		_, err := fmt.Fprintf(f.w, "User is nil.\n")
		return err
	}

	createdAt := time.Unix(int64(user.CreatedUTC), 0).Format("2006-01-02")

	lines := []string{
		fmt.Sprintf("Username: %s", user.Name),
		fmt.Sprintf("Created: %s", createdAt),
		"",
		fmt.Sprintf("Link Karma: %s", f.formatNumber(int64(user.LinkKarma))),
		fmt.Sprintf("Comment Karma: %s", f.formatNumber(int64(user.CommentKarma))),
		"",
		fmt.Sprintf("Is Gold: %v", user.IsGold),
		fmt.Sprintf("Is Moderator: %v", user.IsMod),
		fmt.Sprintf("Over 18: %v", user.Over18),
		"",
	}

	if user.HasVerifiedEmail != nil {
		lines = append(lines, fmt.Sprintf("Email Verified: %v", *user.HasVerifiedEmail))
	}
	if user.HasMail != nil && *user.HasMail {
		lines = append(lines, fmt.Sprintf("Has Unread Mail: %v", *user.HasMail))
	}
	if user.InboxCount > 0 {
		lines = append(lines, fmt.Sprintf("Inbox Count: %d", user.InboxCount))
	}

	return f.writeLines(lines)
}

// formatPostTitle formats the post title section.
func (f *textFormatter) formatPostTitle(post *types.Post) string {
	return post.Title
}

// formatPostMetadata formats the post metadata section.
func (f *textFormatter) formatPostMetadata(post *types.Post) string {
	createdAt := time.Unix(int64(post.CreatedUTC), 0).Format("2006-01-02 15:04:05")
	edited := ""

	if post.Edited.IsEdited {
		if post.Edited.Timestamp > 0 {
			edited = fmt.Sprintf(" (edited %s)", time.Unix(int64(post.Edited.Timestamp), 0).Format("2006-01-02 15:04:05"))
		} else {
			edited = " (edited)"
		}
	}

	var authFlairText string
	if post.AuthorFlairText != nil && *post.AuthorFlairText != "" {
		authFlairText = fmt.Sprintf(" [%s]", *post.AuthorFlairText)
	}

	subredditPrefix := "r/" + post.Subreddit
	if post.Distinguished != nil {
		subredditPrefix += fmt.Sprintf(" (distinguished: %s)", *post.Distinguished)
	}

	return fmt.Sprintf("by %s%s in %s on %s%s", post.Author, authFlairText, subredditPrefix, createdAt, edited)
}

// formatPostContent formats the post content section.
func (f *textFormatter) formatPostContent(post *types.Post) string {
	var content strings.Builder

	if post.IsSelf && post.SelfText != "" {
		content.WriteString(post.SelfText)
	} else if !post.IsSelf && post.URL != "" {
		content.WriteString(fmt.Sprintf("Link: %s (%s)", post.URL, post.Domain))
	}

	if content.Len() > 0 && !f.compact {
		// Truncate long content in compact mode
		text := content.String()
		if len(text) > 500 {
			text = text[:497] + "..."
		}
		return text
	}

	return ""
}

// formatPostStats formats the post statistics section.
func (f *textFormatter) formatPostStats(post *types.Post) string {
	upvotePercent := int(post.UpvoteRatio * 100)
	return fmt.Sprintf("Score: %s | %d%% upvoted | Comments: %d | %s",
		f.formatNumber(int64(post.Score)),
		upvotePercent,
		post.NumComments,
		f.formatLocked(post.Locked, post.Stickied),
	)
}

// formatComment formats a single comment with optional indentation for replies.
func (f *textFormatter) formatComment(comment *types.Comment, depth int) error {
	if comment == nil {
		return nil
	}

	indent := strings.Repeat("  ", depth)
	createdAt := time.Unix(int64(comment.CreatedUTC), 0).Format("2006-01-02 15:04:05")
	edited := ""

	if comment.Edited.IsEdited {
		if comment.Edited.Timestamp > 0 {
			edited = fmt.Sprintf(" (edited %s)", time.Unix(int64(comment.Edited.Timestamp), 0).Format("2006-01-02 15:04:05"))
		} else {
			edited = " (edited)"
		}
	}

	var flairText string
	if comment.AuthorFlairText != nil && *comment.AuthorFlairText != "" {
		flairText = fmt.Sprintf(" [%s]", *comment.AuthorFlairText)
	}

	// Format header
	header := fmt.Sprintf("%s%s%s • %s • %s", comment.Author, flairText, edited, f.formatNumber(int64(comment.Score)), createdAt)
	_, err := fmt.Fprintf(f.w, "%s%s\n", indent, header)
	if err != nil {
		return err
	}

	// Format body - handle potential score hidden
	body := comment.Body
	if comment.ScoreHidden {
		body = fmt.Sprintf("[score hidden]\n%s", body)
	}

	// Write body with indentation
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if line != "" {
			_, err := fmt.Fprintf(f.w, "%s%s\n", indent, line)
			if err != nil {
				return err
			}
		}
	}

	// Format replies if present
	for _, reply := range comment.Replies {
		_, err := fmt.Fprintf(f.w, "\n")
		if err != nil {
			return err
		}
		if err := f.formatComment(reply, depth+1); err != nil {
			return err
		}
	}

	return nil
}

// writeSection writes multiple sections separated by blank lines.
func (f *textFormatter) writeSection(sections []string) error {
	for i, section := range sections {
		if section != "" {
			_, err := fmt.Fprintf(f.w, "%s\n", section)
			if err != nil {
				return err
			}
			if i < len(sections)-1 {
				_, err := fmt.Fprintf(f.w, "\n")
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// writeLines writes multiple lines to the output.
func (f *textFormatter) writeLines(lines []string) error {
	for _, line := range lines {
		_, err := fmt.Fprintf(f.w, "%s\n", line)
		if err != nil {
			return err
		}
	}
	return nil
}

// formatNumber formats large numbers with human-readable suffixes (K, M, B, etc.).
func (f *textFormatter) formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	suffixes := []struct {
		threshold int64
		suffix    string
		divisor   float64
	}{
		{1_000_000_000, "B", 1_000_000_000},
		{1_000_000, "M", 1_000_000},
		{1_000, "K", 1_000},
	}

	for _, s := range suffixes {
		if n >= s.threshold {
			value := float64(n) / s.divisor
			// Use 1 decimal place if it's less than 10
			if value < 10 {
				return fmt.Sprintf("%.1f%s", value, s.suffix)
			}
			return fmt.Sprintf("%.0f%s", value, s.suffix)
		}
	}

	return fmt.Sprintf("%d", n)
}

// formatOptionalString formats an optional string field with a label.
func (f *textFormatter) formatOptionalString(label, value string) string {
	if value == "" {
		return ""
	}
	// Truncate very long descriptions
	if len(value) > 200 {
		value = value[:197] + "..."
	}
	return fmt.Sprintf("%s: %s", label, value)
}

// formatLocked formats the locked and stickied status indicators.
func (f *textFormatter) formatLocked(locked, stickied bool) string {
	var flags []string
	if locked {
		flags = append(flags, "[LOCKED]")
	}
	if stickied {
		flags = append(flags, "[STICKIED]")
	}
	if len(flags) == 0 {
		return ""
	}
	return strings.Join(flags, " | ")
}
