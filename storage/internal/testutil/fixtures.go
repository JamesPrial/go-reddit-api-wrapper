package testutil

import (
	"fmt"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// BuildPost creates a valid test Post with sensible defaults.
// Uses functional options pattern for customization.
// Returns Post with all required fields populated.
func BuildPost(id, subreddit string, opts ...func(*types.Post)) *types.Post {
	now := time.Now().Unix()

	post := &types.Post{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t3_" + id,
		},
		Votable: types.Votable{
			Score: 42,
			Ups:   42,
			Downs: 0,
			Likes: nil,
		},
		Created: types.Created{
			Created:    float64(now),
			CreatedUTC: float64(now),
		},
		Author:              "testuser",
		AuthorFlairCSSClass: nil,
		AuthorFlairText:     nil,
		Clicked:             false,
		Domain:              "self." + subreddit,
		Hidden:              false,
		IsSelf:              true,
		LinkFlairCSSClass:   nil,
		LinkFlairText:       nil,
		Locked:              false,
		Media:               nil,
		MediaEmbed:          nil,
		NumComments:         0,
		Over18:              false,
		Permalink:           fmt.Sprintf("/r/%s/comments/%s/test_post/", subreddit, id),
		Saved:               false,
		SelfText:            "This is a test post body.",
		SelfTextHTML:        stringPtr("<p>This is a test post body.</p>"),
		Subreddit:           subreddit,
		SubredditID:         "t5_2qh1i",
		Thumbnail:           "self",
		Title:               "Test Post",
		URL:                 fmt.Sprintf("https://reddit.com/r/%s/comments/%s/test_post/", subreddit, id),
		Edited: types.Edited{
			IsEdited:  false,
			Timestamp: 0,
		},
		Distinguished: nil,
		Stickied:      false,
		UpvoteRatio:   0.95,
	}

	// Apply options
	for _, opt := range opts {
		opt(post)
	}

	return post
}

// WithScore sets the post score.
func WithScore(score int) func(*types.Post) {
	return func(p *types.Post) {
		p.Score = score
		p.Ups = score
	}
}

// WithAuthor sets the post author.
func WithAuthor(author string) func(*types.Post) {
	return func(p *types.Post) {
		p.Author = author
	}
}

// WithTitle sets the post title.
func WithTitle(title string) func(*types.Post) {
	return func(p *types.Post) {
		p.Title = title
	}
}

// WithNumComments sets the number of comments.
func WithNumComments(count int) func(*types.Post) {
	return func(p *types.Post) {
		p.NumComments = count
	}
}

// WithCreatedUTC sets the creation timestamp.
func WithCreatedUTC(timestamp float64) func(*types.Post) {
	return func(p *types.Post) {
		p.Created.CreatedUTC = timestamp
		p.Created.Created = timestamp
	}
}

// WithEdited sets the post as edited with an optional timestamp.
func WithEdited(timestamp float64) func(*types.Post) {
	return func(p *types.Post) {
		p.Edited.IsEdited = true
		p.Edited.Timestamp = timestamp
	}
}

// BuildComment creates a valid test Comment.
// Handles parent_id formatting (empty for top-level, or "t1_" prefix for child).
// Handles link_id formatting ("t3_" + postID).
// Returns Comment with all required fields populated.
func BuildComment(id, postID, parentID string, depth int) *types.Comment {
	now := time.Now().Unix()

	// Format parent_id correctly
	formattedParentID := ""
	if parentID == "" {
		// Top-level comment - parent is the post
		formattedParentID = "t3_" + postID
	} else {
		// Child comment - parent is another comment
		formattedParentID = "t1_" + parentID
	}

	comment := &types.Comment{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t1_" + id,
		},
		Votable: types.Votable{
			Score: 10,
			Ups:   10,
			Downs: 0,
			Likes: nil,
		},
		Created: types.Created{
			Created:    float64(now),
			CreatedUTC: float64(now),
		},
		ApprovedBy:          nil,
		Author:              "commenter",
		AuthorFlairCSSClass: nil,
		AuthorFlairText:     nil,
		BannedBy:            nil,
		Body:                fmt.Sprintf("This is comment %s at depth %d", id, depth),
		BodyHTML:            fmt.Sprintf("<p>This is comment %s at depth %d</p>", id, depth),
		Edited: types.Edited{
			IsEdited:  false,
			Timestamp: 0,
		},
		Gilded:          0,
		LinkAuthor:      "testuser",
		LinkID:          "t3_" + postID,
		LinkTitle:       "Test Post",
		LinkURL:         fmt.Sprintf("https://reddit.com/r/test/comments/%s/test_post/", postID),
		NumReports:      nil,
		ParentID:        formattedParentID,
		Replies:         []*types.Comment{},
		Saved:           false,
		ScoreHidden:     false,
		Subreddit:       "test",
		SubredditID:     "t5_2qh1i",
		Distinguished:   nil,
		MoreChildrenIDs: []string{},
	}

	return comment
}

// BuildCommentTree generates a tree of comments for testing.
// depth: how many levels deep (0 = just top-level)
// breadth: how many children per level
// Returns flat slice of comments in insertion order.
// Note: Comment IDs are generated as "{postID}_c{N}" to ensure uniqueness across different posts.
func BuildCommentTree(postID string, depth, breadth int) []*types.Comment {
	var comments []*types.Comment
	commentID := 1

	// Generate top-level comments
	var topLevelComments []*types.Comment
	for i := 0; i < breadth; i++ {
		id := fmt.Sprintf("%s_c%d", postID, commentID)
		comment := BuildComment(id, postID, "", 0)
		comments = append(comments, comment)
		topLevelComments = append(topLevelComments, comment)
		commentID++
	}

	// Generate child comments recursively
	if depth > 0 {
		for _, parent := range topLevelComments {
			childComments := buildChildComments(postID, parent.ID, depth-1, breadth, &commentID)
			comments = append(comments, childComments...)
		}
	}

	return comments
}

// buildChildComments is a helper function that recursively builds child comments.
func buildChildComments(postID, parentID string, remainingDepth, breadth int, commentID *int) []*types.Comment {
	var comments []*types.Comment

	// Calculate the current depth based on parent
	currentDepth := 0
	// For simplicity, we'll let the storage layer calculate depth
	// We just need to track parent relationships

	// Generate children for this level
	var currentLevelComments []*types.Comment
	for i := 0; i < breadth; i++ {
		id := fmt.Sprintf("%s_c%d", postID, *commentID)
		comment := BuildComment(id, postID, parentID, currentDepth+1)
		comments = append(comments, comment)
		currentLevelComments = append(currentLevelComments, comment)
		*commentID++
	}

	// Recursively generate children for the next level
	if remainingDepth > 0 {
		for _, parent := range currentLevelComments {
			childComments := buildChildComments(postID, parent.ID, remainingDepth-1, breadth, commentID)
			comments = append(comments, childComments...)
		}
	}

	return comments
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}
