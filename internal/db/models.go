package db

import (
	"time"

	"gorm.io/gorm"
)

// Subreddit represents a Reddit subreddit in the database.
// It has a one-to-many relationship with Posts.
type Subreddit struct {
	gorm.Model
	// Fullname is the Reddit fullname (unique identifier, e.g., "t5_2qh33").
	Fullname string `gorm:"uniqueIndex;not null;size:20"`
	// Name is the subreddit name without the /r/ prefix (e.g., "golang").
	// Indexed for fast lookups by name.
	Name string `gorm:"index;not null;size:21"`
	// Description is the subreddit description (public_description field from Reddit API).
	Description string `gorm:"type:text"`
	// Subscribers is the number of subscribers to the subreddit.
	Subscribers int64 `gorm:"default:0"`
	// Posts contains all posts from this subreddit (one-to-many relationship).
	Posts []Post `gorm:"foreignKey:SubredditID;constraint:OnDelete:CASCADE"`
}

// Post represents a Reddit post (submission/link) in the database.
// It belongs to a Subreddit and has a one-to-many relationship with Comments.
type Post struct {
	gorm.Model
	// Fullname is the Reddit fullname (unique identifier, e.g., "t3_abc123").
	Fullname string `gorm:"uniqueIndex;not null;size:20"`
	// SubredditID is the foreign key to the Subreddit table.
	// Indexed as part of composite index for efficient subreddit+time queries.
	SubredditID uint `gorm:"not null;index:idx_subreddit_time"`
	// Title is the post title (required, max 300 characters per Reddit's limit).
	Title string `gorm:"not null;size:300"`
	// Author is the Reddit username who created the post.
	// Indexed for efficient author queries.
	Author string `gorm:"index;not null;size:20"`
	// Score is the post score (upvotes minus downvotes).
	// Indexed for sorting/filtering by popularity.
	Score int `gorm:"index;default:0"`
	// NumComments is the total number of comments on the post.
	NumComments int `gorm:"default:0"`
	// URL is the URL of the linked content (for link posts) or the permalink (for self posts).
	URL string `gorm:"type:text"`
	// Selftext is the post body text (for self/text posts).
	Selftext string `gorm:"type:text"`
	// CreatedUTC is the timestamp when the post was created on Reddit (Unix timestamp).
	// Indexed as part of composite index for efficient time-based queries.
	// Also has a separate index for time-based sorting.
	CreatedUTC time.Time `gorm:"index:idx_subreddit_time;index;not null"`
	// Comments contains all comments on this post (one-to-many relationship).
	Comments []Comment `gorm:"foreignKey:PostID;constraint:OnDelete:CASCADE"`
	// Subreddit is the belongs-to relationship with Subreddit table.
	Subreddit Subreddit `gorm:"foreignKey:SubredditID"`
}

// Comment represents a Reddit comment in the database.
// It belongs to a Post and can have a self-referential relationship for nested replies.
type Comment struct {
	gorm.Model
	// Fullname is the Reddit fullname (unique identifier, e.g., "t1_xyz789").
	Fullname string `gorm:"uniqueIndex;not null;size:20"`
	// PostID is the foreign key to the Post table.
	// Indexed as part of composite index for efficient post+time queries.
	PostID uint `gorm:"not null;index:idx_post_time"`
	// ParentID is the foreign key to the parent Comment (for nested replies).
	// Null for top-level comments. Indexed for efficient parent-child queries.
	ParentID *uint `gorm:"index"`
	// Author is the Reddit username who created the comment.
	// Indexed for efficient author queries.
	Author string `gorm:"index;not null;size:20"`
	// Body is the comment text content.
	Body string `gorm:"type:text;not null"`
	// Score is the comment score (upvotes minus downvotes).
	// Indexed for sorting/filtering by popularity.
	Score int `gorm:"index;default:0"`
	// CreatedUTC is the timestamp when the comment was created on Reddit (Unix timestamp).
	// Indexed as part of composite index for efficient time-based queries.
	// Also has a separate index for time-based sorting.
	CreatedUTC time.Time `gorm:"index:idx_post_time;index;not null"`
	// Post is the belongs-to relationship with Post table.
	Post Post `gorm:"foreignKey:PostID"`
	// Parent is the belongs-to relationship for nested comments (self-referential).
	Parent *Comment `gorm:"foreignKey:ParentID"`
	// Replies contains child comments (one-to-many self-referential relationship).
	Replies []Comment `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
}

// TrackingConfig stores configuration for subreddit tracking/polling.
// Optional table for tracking which subreddits to monitor and how often.
type TrackingConfig struct {
	gorm.Model
	// SubredditName is the name of the subreddit to track (e.g., "golang").
	// Indexed for fast lookups.
	SubredditName string `gorm:"index;not null;size:21"`
	// PollingInterval is the number of seconds between polling this subreddit.
	PollingInterval int `gorm:"not null;default:300"` // Default to 5 minutes
	// Enabled controls whether this subreddit is currently being tracked.
	Enabled bool `gorm:"not null;default:true"`
	// LastFetchedAt is the timestamp of the last successful fetch for this subreddit.
	LastFetchedAt *time.Time
}
