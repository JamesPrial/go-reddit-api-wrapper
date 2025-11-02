package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// tableFormatter provides table output for Reddit API responses.
type tableFormatter struct {
	w io.Writer
}

// newTableFormatter creates a new table formatter.
func newTableFormatter(w io.Writer) *tableFormatter {
	return &tableFormatter{w: w}
}

// FormatPosts formats a collection of posts as a table.
func (f *tableFormatter) FormatPosts(posts []*types.Post) error {
	if len(posts) == 0 {
		_, err := fmt.Fprintf(f.w, "No posts found.\n")
		return err
	}

	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Write header
	fmt.Fprintf(tw, "Author\tSubreddit\tTitle\tScore\tComments\tCreated\n")
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
		strings.Repeat("-", 15),
		strings.Repeat("-", 12),
		strings.Repeat("-", 30),
		strings.Repeat("-", 8),
		strings.Repeat("-", 10),
		strings.Repeat("-", 15),
	)

	// Write rows
	for _, post := range posts {
		createdAt := time.Unix(int64(post.CreatedUTC), 0).Format("2006-01-02")
		title := post.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%s\n",
			post.Author,
			post.Subreddit,
			title,
			post.Score,
			post.NumComments,
			createdAt,
		)
	}

	return nil
}

// FormatPost formats a single post as a table (single row).
func (f *tableFormatter) FormatPost(post *types.Post) error {
	if post == nil {
		_, err := fmt.Fprintf(f.w, "Post is nil.\n")
		return err
	}

	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Write header
	fmt.Fprintf(tw, "Author\tSubreddit\tTitle\tScore\tComments\tCreated\n")
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
		strings.Repeat("-", 15),
		strings.Repeat("-", 12),
		strings.Repeat("-", 30),
		strings.Repeat("-", 8),
		strings.Repeat("-", 10),
		strings.Repeat("-", 15),
	)

	// Write single row
	createdAt := time.Unix(int64(post.CreatedUTC), 0).Format("2006-01-02")
	title := post.Title
	if len(title) > 30 {
		title = title[:27] + "..."
	}
	fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%s\n",
		post.Author,
		post.Subreddit,
		title,
		post.Score,
		post.NumComments,
		createdAt,
	)

	return nil
}

// FormatComments formats a post with its comments as a table.
func (f *tableFormatter) FormatComments(response *types.CommentsResponse) error {
	if response == nil {
		_, err := fmt.Fprintf(f.w, "Response is nil.\n")
		return err
	}

	if response.Post != nil {
		if err := f.FormatPost(response.Post); err != nil {
			return err
		}
		_, err := fmt.Fprintf(f.w, "\n")
		if err != nil {
			return err
		}
	}

	if len(response.Comments) == 0 {
		_, err := fmt.Fprintf(f.w, "No comments found.\n")
		return err
	}

	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Write header
	fmt.Fprintf(tw, "Author\tScore\tCreated\tBody Preview\n")
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
		strings.Repeat("-", 15),
		strings.Repeat("-", 8),
		strings.Repeat("-", 15),
		strings.Repeat("-", 40),
	)

	// Write rows for top-level comments
	for _, comment := range response.Comments {
		createdAt := time.Unix(int64(comment.CreatedUTC), 0).Format("2006-01-02")
		bodyPreview := comment.Body
		if len(bodyPreview) > 40 {
			bodyPreview = bodyPreview[:37] + "..."
		}
		// Remove newlines from preview
		bodyPreview = strings.ReplaceAll(bodyPreview, "\n", " ")

		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			comment.Author,
			comment.Score,
			createdAt,
			bodyPreview,
		)
	}

	if len(response.MoreIDs) > 0 {
		fmt.Fprintf(tw, "\n")
	}

	return tw.Flush()
}

// FormatSubreddit formats subreddit information as a table.
func (f *tableFormatter) FormatSubreddit(sub *types.SubredditData) error {
	if sub == nil {
		_, err := fmt.Fprintf(f.w, "Subreddit is nil.\n")
		return err
	}

	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	fmt.Fprintf(tw, "Field\tValue\n")
	fmt.Fprintf(tw, "%s\t%s\n", strings.Repeat("-", 20), strings.Repeat("-", 40))

	fmt.Fprintf(tw, "Display Name\t%s\n", sub.DisplayName)
	fmt.Fprintf(tw, "URL\t%s\n", sub.URL)
	fmt.Fprintf(tw, "Title\t%s\n", sub.Title)
	fmt.Fprintf(tw, "Subscribers\t%d\n", sub.Subscribers)
	fmt.Fprintf(tw, "Active Users\t%d\n", sub.AccountsActive)
	fmt.Fprintf(tw, "Type\t%s\n", sub.SubredditType)
	fmt.Fprintf(tw, "Over 18\t%v\n", sub.Over18)

	if sub.UserIsSubscriber != nil && *sub.UserIsSubscriber {
		fmt.Fprintf(tw, "Subscribed\t%v\n", true)
	}
	if sub.UserIsModerator != nil && *sub.UserIsModerator {
		fmt.Fprintf(tw, "Moderator\t%v\n", true)
	}

	return tw.Flush()
}

// FormatUser formats user account information as a table.
func (f *tableFormatter) FormatUser(user *types.AccountData) error {
	if user == nil {
		_, err := fmt.Fprintf(f.w, "User is nil.\n")
		return err
	}

	tw := tabwriter.NewWriter(f.w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	fmt.Fprintf(tw, "Field\tValue\n")
	fmt.Fprintf(tw, "%s\t%s\n", strings.Repeat("-", 20), strings.Repeat("-", 40))

	createdAt := time.Unix(int64(user.CreatedUTC), 0).Format("2006-01-02")

	fmt.Fprintf(tw, "Username\t%s\n", user.Name)
	fmt.Fprintf(tw, "Created\t%s\n", createdAt)
	fmt.Fprintf(tw, "Link Karma\t%d\n", user.LinkKarma)
	fmt.Fprintf(tw, "Comment Karma\t%d\n", user.CommentKarma)
	fmt.Fprintf(tw, "Is Gold\t%v\n", user.IsGold)
	fmt.Fprintf(tw, "Is Moderator\t%v\n", user.IsMod)
	fmt.Fprintf(tw, "Over 18\t%v\n", user.Over18)

	if user.HasVerifiedEmail != nil {
		fmt.Fprintf(tw, "Email Verified\t%v\n", *user.HasVerifiedEmail)
	}
	if user.InboxCount > 0 {
		fmt.Fprintf(tw, "Inbox Count\t%d\n", user.InboxCount)
	}

	return tw.Flush()
}
