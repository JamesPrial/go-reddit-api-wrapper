package testutil

import (
	"fmt"
	"math/rand"
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
	for range breadth {
		id := fmt.Sprintf("%s_c%d", postID, commentID)
		comment := BuildComment(id, postID, "", 0)
		comments = append(comments, comment)
		topLevelComments = append(topLevelComments, comment)
		commentID++
	}

	// Generate child comments recursively
	if depth > 0 {
		for _, parent := range topLevelComments {
			childComments := buildChildComments(postID, parent.ID, 1, depth-1, breadth, &commentID)
			comments = append(comments, childComments...)
		}
	}

	return comments
}

// buildChildComments is a helper function that recursively builds child comments.
func buildChildComments(postID, parentID string, currentDepth, remainingDepth, breadth int, commentID *int) []*types.Comment {
	var comments []*types.Comment

	// Generate children for this level
	var currentLevelComments []*types.Comment
	for range breadth {
		id := fmt.Sprintf("%s_c%d", postID, *commentID)
		comment := BuildComment(id, postID, parentID, currentDepth)
		comments = append(comments, comment)
		currentLevelComments = append(currentLevelComments, comment)
		*commentID++
	}

	// Recursively generate children for the next level
	if remainingDepth > 0 {
		for _, parent := range currentLevelComments {
			childComments := buildChildComments(postID, parent.ID, currentDepth+1, remainingDepth-1, breadth, commentID)
			comments = append(comments, childComments...)
		}
	}

	return comments
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}

// BuildPostBatch generates N posts with sequential IDs and customizable options.
// All posts are for the same subreddit and have unique IDs (id0, id1, id2, ...).
// Can be customized with functional options that apply to all posts.
func BuildPostBatch(count int, subreddit string, opts ...func(*types.Post)) []*types.Post {
	posts := make([]*types.Post, count)
	for i := range count {
		id := fmt.Sprintf("id%d", i)
		posts[i] = BuildPost(id, subreddit, opts...)
	}
	return posts
}

// BuildCommentWithAllFields creates a comment with all fields populated,
// including nullable fields with realistic data.
// Useful for testing storage of complete comment data.
func BuildCommentWithAllFields(id, postID, parentID string, depth int) *types.Comment {
	now := time.Now().Unix()

	// Format parent_id correctly
	formattedParentID := ""
	if parentID == "" {
		formattedParentID = "t3_" + postID
	} else {
		formattedParentID = "t1_" + parentID
	}

	author := fmt.Sprintf("author_%s", id)
	return &types.Comment{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t1_" + id,
		},
		Votable: types.Votable{
			Score: 42,
			Ups:   42,
			Downs: 0,
			Likes: boolPtr(true),
		},
		Created: types.Created{
			Created:    float64(now),
			CreatedUTC: float64(now),
		},
		ApprovedBy:          stringPtr("moderator"),
		Author:              author,
		AuthorFlairCSSClass: stringPtr("special-flair"),
		AuthorFlairText:     stringPtr("Flair Text"),
		BannedBy:            nil,
		Body:                fmt.Sprintf("This is a comprehensive comment at depth %d with all fields populated. ID: %s", depth, id),
		BodyHTML:            fmt.Sprintf("<p>This is a comprehensive comment at depth %d with all fields populated. ID: %s</p>", depth, id),
		Edited: types.Edited{
			IsEdited:  true,
			Timestamp: float64(now),
		},
		Gilded:          3,
		LinkAuthor:      "testuser",
		LinkID:          "t3_" + postID,
		LinkTitle:       "Test Post with Full Comments",
		LinkURL:         fmt.Sprintf("https://reddit.com/r/test/comments/%s/test_post/", postID),
		NumReports:      intPtr(2),
		ParentID:        formattedParentID,
		Replies:         []*types.Comment{},
		Saved:           true,
		ScoreHidden:     false,
		Subreddit:       "test",
		SubredditID:     "t5_2qh1i",
		Distinguished:   stringPtr("moderator"),
		MoreChildrenIDs: []string{},
	}
}

// WithCreatedAt sets the creation timestamp to the specified time.
func WithCreatedAt(t time.Time) func(*types.Post) {
	return func(p *types.Post) {
		timestamp := float64(t.Unix())
		p.Created.Created = timestamp
		p.Created.CreatedUTC = timestamp
	}
}

// WithRandomData generates random but realistic titles, bodies, and authors.
// Uses a seeded random source for reproducible tests.
func WithRandomData(seed int64) func(*types.Post) {
	return func(p *types.Post) {
		source := rand.New(rand.NewSource(seed))
		titles := []string{
			"Check out this amazing discovery",
			"A question about the best way to do X",
			"TIL something interesting about Y",
			"Help! I need advice on Z",
			"Share: My experience with the new feature",
		}
		bodies := []string{
			"This is a long post body with lots of detail about the topic at hand.",
			"I've been thinking about this for a while and wanted to share my thoughts.",
			"Has anyone else experienced this? Let me know your thoughts!",
			"Here's a comprehensive explanation of what's happening and why it matters.",
		}
		authors := []string{
			"RandomUser42", "CuriousMind", "TechEnthusiast", "DataDrivenDev", "OpenSourceFan",
		}

		p.Title = titles[source.Intn(len(titles))]
		p.SelfText = bodies[source.Intn(len(bodies))]
		p.Author = authors[source.Intn(len(authors))]
	}
}

// WithMediaEmbed sets the media embed field with the provided JSON string.
func WithMediaEmbed(jsonStr string) func(*types.Post) {
	return func(p *types.Post) {
		if jsonStr != "" {
			p.MediaEmbed = []byte(jsonStr)
		}
	}
}

// boolPtr returns a pointer to the given bool.
func boolPtr(b bool) *bool {
	return &b
}

// intPtr returns a pointer to the given int.
func intPtr(i int) *int {
	return &i
}
