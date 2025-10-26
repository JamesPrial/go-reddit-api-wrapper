package services

import (
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// PostToModel converts a Reddit API Post to a database Post model.
// The subredditID must be the database ID of the associated Subreddit record.
//
// This function:
//   - Extracts all relevant fields from the Reddit Post
//   - Converts Unix timestamp (float64) to time.Time
//   - Maps the Reddit fullname (e.g., "t3_abc123") to the database Fullname field
//   - Handles optional fields (self-text, URLs) appropriately
//
// Returns a database Post ready for insertion or update.
func PostToModel(redditPost *types.Post, subredditID uint) *db.Post {
	if redditPost == nil {
		return nil
	}

	// Convert Unix timestamp to time.Time
	createdTime := time.Unix(int64(redditPost.CreatedUTC), 0).UTC()

	return &db.Post{
		Fullname:    redditPost.Name,        // Reddit fullname (e.g., "t3_abc123")
		SubredditID: subredditID,            // Foreign key to subreddit
		Title:       redditPost.Title,       // Post title
		Author:      redditPost.Author,      // Reddit username
		Score:       redditPost.Score,       // Net score (upvotes - downvotes)
		NumComments: redditPost.NumComments, // Comment count
		URL:         redditPost.URL,         // Link URL or permalink
		Selftext:    redditPost.SelfText,    // Self-post body text
		CreatedUTC:  createdTime,            // Creation timestamp
	}
}

// CommentToModel converts a Reddit API Comment to a database Comment model.
// The postID must be the database ID of the associated Post record.
//
// This function:
//   - Extracts all relevant fields from the Reddit Comment
//   - Converts Unix timestamp (float64) to time.Time
//   - Maps the Reddit fullname (e.g., "t1_xyz789") to the database Fullname field
//   - Does NOT set ParentID - caller must handle parent-child relationships
//
// Note: This converter does not handle nested comment structures or parent relationships.
// For initial implementation, comments are stored flat. The caller is responsible for
// building parent-child relationships if needed based on the ParentID field in the
// Reddit Comment (redditComment.ParentID contains the fullname of the parent).
//
// Returns a database Comment ready for insertion or update.
func CommentToModel(redditComment *types.Comment, postID uint) *db.Comment {
	if redditComment == nil {
		return nil
	}

	// Convert Unix timestamp to time.Time
	createdTime := time.Unix(int64(redditComment.CreatedUTC), 0).UTC()

	return &db.Comment{
		Fullname:   redditComment.Name,   // Reddit fullname (e.g., "t1_xyz789")
		PostID:     postID,               // Foreign key to post
		ParentID:   nil,                  // Not set - caller handles relationships
		Author:     redditComment.Author, // Reddit username
		Body:       redditComment.Body,   // Comment text
		Score:      redditComment.Score,  // Net score (upvotes - downvotes)
		CreatedUTC: createdTime,          // Creation timestamp
	}
}

// CommentsToModels converts a slice of Reddit API Comments to database Comment models.
// This is a convenience function that calls CommentToModel for each comment.
//
// Parameters:
//   - redditComments: Slice of Reddit API comments to convert
//   - postID: Database ID of the associated Post record
//
// Returns a slice of database Comments ready for batch insertion.
func CommentsToModels(redditComments []*types.Comment, postID uint) []db.Comment {
	if len(redditComments) == 0 {
		return nil
	}

	models := make([]db.Comment, 0, len(redditComments))
	for _, rc := range redditComments {
		if rc == nil {
			continue
		}
		model := CommentToModel(rc, postID)
		if model != nil {
			models = append(models, *model)
		}
	}

	return models
}

// SubredditToModel converts Reddit API SubredditData to a database Subreddit model.
// This is useful when fetching subreddit information via GetSubreddit API call.
//
// This function:
//   - Extracts the subreddit name and fullname
//   - Maps the description (public_description from Reddit API)
//   - Stores subscriber count
//
// Returns a database Subreddit ready for insertion or update.
func SubredditToModel(redditSubreddit *types.SubredditData) *db.Subreddit {
	if redditSubreddit == nil {
		return nil
	}

	return &db.Subreddit{
		Fullname:    redditSubreddit.Name,              // Reddit fullname (e.g., "t5_2qh33")
		Name:        redditSubreddit.DisplayName,       // Subreddit name (e.g., "golang")
		Description: redditSubreddit.PublicDescription, // Public description
		Subscribers: redditSubreddit.Subscribers,       // Subscriber count
	}
}
