package validation

import (
	"strings"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestIsValidBase36(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "abc123", true},
		{"valid numbers", "123456", true},
		{"valid mixed", "1a2b3c", true},
		{"invalid uppercase", "ABC123", false},
		{"invalid special chars", "abc_123", false},
		{"empty string", "", false},
		{"invalid hyphen", "abc-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidBase36(tt.input); got != tt.want {
				t.Errorf("IsValidBase36(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidSubreddit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "golang", true},
		{"valid with underscore", "ask_reddit", true},
		{"valid mixed case", "AskReddit", true},
		{"valid at min length", "abc", true},
		{"valid at max length", "a123456789012345678_x", true},
		{"invalid too short", "ab", false},
		{"invalid too long", "a1234567890123456789xy", false},
		{"invalid hyphen", "ask-reddit", false},
		{"invalid space", "ask reddit", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidSubreddit(tt.input); got != tt.want {
				t.Errorf("IsValidSubreddit(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "johndoe", true},
		{"valid with underscore", "john_doe", true},
		{"valid with hyphen", "john-doe", true},
		{"valid mixed", "John_Doe-123", true},
		{"valid at min length", "abc", true},
		{"valid at max length", "a1234567890123456789", true},
		{"invalid too short", "ab", false},
		{"invalid too long", "a12345678901234567890", false},
		{"invalid space", "john doe", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidUsername(tt.input); got != tt.want {
				t.Errorf("IsValidUsername(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidFullname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid comment", "t1_abc123", true},
		{"valid account", "t2_xyz789", true},
		{"valid post", "t3_def456", true},
		{"valid message", "t4_ghi789", true},
		{"valid subreddit", "t5_jkl012", true},
		{"valid award", "t6_mno345", true},
		{"invalid prefix t0", "t0_abc123", false},
		{"invalid prefix t7", "t7_abc123", false},
		{"invalid no underscore", "t1abc123", false},
		{"invalid uppercase ID", "t1_ABC123", false},
		{"invalid missing ID", "t1_", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidFullname(tt.input); got != tt.want {
				t.Errorf("IsValidFullname(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidPermalink(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid post permalink", "/r/golang/comments/abc123/test_post/", true},
		{"valid post without trailing slash", "/r/golang/comments/abc123/test_post", true},
		{"valid comment permalink", "/r/golang/comments/abc123/test_post/def456/", true},
		{"valid comment without trailing slash", "/r/golang/comments/abc123/test_post/def456", true},
		{"invalid no /r/ prefix", "/golang/comments/abc123/test/", false},
		{"invalid missing comments", "/r/golang/abc123/test/", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidPermalink(tt.input); got != tt.want {
				t.Errorf("IsValidPermalink(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateThingData(t *testing.T) {
	tests := []struct {
		name    string
		data    *types.ThingData
		wantErr bool
	}{
		{
			name:    "valid thing data",
			data:    &types.ThingData{ID: "abc123", Name: "t3_abc123"},
			wantErr: false,
		},
		{
			name:    "nil thing data",
			data:    nil,
			wantErr: true,
		},
		{
			name:    "empty ID",
			data:    &types.ThingData{ID: "", Name: "t3_abc123"},
			wantErr: true,
		},
		{
			name:    "invalid ID format",
			data:    &types.ThingData{ID: "ABC123", Name: "t3_ABC123"},
			wantErr: true,
		},
		{
			name:    "invalid fullname",
			data:    &types.ThingData{ID: "abc123", Name: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateThingData(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateThingData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVotable(t *testing.T) {
	tests := []struct {
		name    string
		data    *types.Votable
		wantErr bool
	}{
		{
			name:    "valid votable",
			data:    &types.Votable{Score: 100, Ups: 100, Downs: 0},
			wantErr: false,
		},
		{
			name:    "valid negative score",
			data:    &types.Votable{Score: -50, Ups: -50, Downs: 0},
			wantErr: false,
		},
		{
			name:    "nil votable",
			data:    nil,
			wantErr: true,
		},
		{
			name:    "ups != score",
			data:    &types.Votable{Score: 100, Ups: 90, Downs: 0},
			wantErr: true,
		},
		{
			name:    "downs != 0",
			data:    &types.Votable{Score: 100, Ups: 100, Downs: 10},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVotable(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVotable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCreated(t *testing.T) {
	now := float64(time.Now().Unix())
	redditFounding := float64(time.Date(2005, 6, 1, 0, 0, 0, 0, time.UTC).Unix())

	tests := []struct {
		name    string
		data    *types.Created
		wantErr bool
	}{
		{
			name:    "valid created",
			data:    &types.Created{Created: now, CreatedUTC: now},
			wantErr: false,
		},
		{
			name:    "nil created",
			data:    nil,
			wantErr: true,
		},
		{
			name:    "created != createdUTC",
			data:    &types.Created{Created: now, CreatedUTC: now + 100},
			wantErr: true,
		},
		{
			name:    "zero timestamp",
			data:    &types.Created{Created: 0, CreatedUTC: 0},
			wantErr: true,
		},
		{
			name:    "negative timestamp",
			data:    &types.Created{Created: -100, CreatedUTC: -100},
			wantErr: true,
		},
		{
			name:    "future timestamp",
			data:    &types.Created{Created: now + 7200, CreatedUTC: now + 7200},
			wantErr: true,
		},
		{
			name:    "before reddit existed",
			data:    &types.Created{Created: redditFounding - 100, CreatedUTC: redditFounding - 100},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCreated(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreated() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePost(t *testing.T) {
	now := float64(time.Now().Unix())
	validPost := &types.Post{
		ThingData: types.ThingData{ID: "abc123", Name: "t3_abc123"},
		Votable:   types.Votable{Score: 100, Ups: 100, Downs: 0},
		Created:   types.Created{Created: now, CreatedUTC: now},
		Title:     "Test Post",
		Author:    "testuser",
		Subreddit: "golang",
		SubredditID: "t5_2rcjn",
		Permalink: "/r/golang/comments/abc123/test_post/",
		URL:       "https://reddit.com/r/golang/comments/abc123/test_post/",
		UpvoteRatio: 0.95,
		NumComments: 10,
	}

	tests := []struct {
		name    string
		post    *types.Post
		wantErr bool
	}{
		{
			name:    "valid post",
			post:    validPost,
			wantErr: false,
		},
		{
			name:    "nil post",
			post:    nil,
			wantErr: true,
		},
		{
			name: "title too long",
			post: func() *types.Post {
				p := *validPost
				p.Title = strings.Repeat("a", 301)
				return &p
			}(),
			wantErr: true,
		},
		{
			name: "empty title",
			post: func() *types.Post {
				p := *validPost
				p.Title = ""
				return &p
			}(),
			wantErr: true,
		},
		{
			name: "invalid upvote ratio",
			post: func() *types.Post {
				p := *validPost
				p.UpvoteRatio = 1.5
				return &p
			}(),
			wantErr: true,
		},
		{
			name: "negative comment count",
			post: func() *types.Post {
				p := *validPost
				p.NumComments = -1
				return &p
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePost(tt.post)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComment(t *testing.T) {
	now := float64(time.Now().Unix())
	validComment := &types.Comment{
		ThingData: types.ThingData{ID: "def456", Name: "t1_def456"},
		Votable:   types.Votable{Score: 50, Ups: 50, Downs: 0},
		Created:   types.Created{Created: now, CreatedUTC: now},
		Body:      "Test comment",
		Author:    "testuser",
		Subreddit: "golang",
		SubredditID: "t5_2rcjn",
		ParentID:  "t3_abc123",
		LinkID:    "t3_abc123",
	}

	tests := []struct {
		name    string
		comment *types.Comment
		wantErr bool
	}{
		{
			name:    "valid comment",
			comment: validComment,
			wantErr: false,
		},
		{
			name:    "nil comment",
			comment: nil,
			wantErr: true,
		},
		{
			name: "body too long",
			comment: func() *types.Comment {
				c := *validComment
				c.Body = strings.Repeat("a", 10001)
				return &c
			}(),
			wantErr: true,
		},
		{
			name: "empty body",
			comment: func() *types.Comment {
				c := *validComment
				c.Body = ""
				return &c
			}(),
			wantErr: true,
		},
		{
			name: "invalid parent ID",
			comment: func() *types.Comment {
				c := *validComment
				c.ParentID = "invalid"
				return &c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComment(tt.comment)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateComment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubredditData(t *testing.T) {
	validSubreddit := &types.SubredditData{
		ThingData:   types.ThingData{ID: "2rcjn", Name: "t5_2rcjn"},
		DisplayName: "golang",
		Subscribers: 1000000,
	}

	tests := []struct {
		name      string
		subreddit *types.SubredditData
		wantErr   bool
	}{
		{
			name:      "valid subreddit",
			subreddit: validSubreddit,
			wantErr:   false,
		},
		{
			name:      "nil subreddit",
			subreddit: nil,
			wantErr:   true,
		},
		{
			name: "empty display name",
			subreddit: func() *types.SubredditData {
				s := *validSubreddit
				s.DisplayName = ""
				return &s
			}(),
			wantErr: true,
		},
		{
			name: "negative subscribers",
			subreddit: func() *types.SubredditData {
				s := *validSubreddit
				s.Subscribers = -1
				return &s
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubredditData(tt.subreddit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSubredditData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
