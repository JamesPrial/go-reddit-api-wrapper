package output

import (
	"bytes"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestNewFormatter_Text(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if f == nil {
		t.Fatal("New() returned nil formatter")
	}
}

func TestNewFormatter_JSON(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "json"}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if f == nil {
		t.Fatal("New() returned nil formatter")
	}
}

func TestNewFormatter_Table(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "table"}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if f == nil {
		t.Fatal("New() returned nil formatter")
	}
}

func TestNewFormatter_DefaultText(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: ""}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if f == nil {
		t.Fatal("New() returned nil formatter")
	}
}

func TestNewFormatter_InvalidFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "invalid"}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() should have failed with invalid format")
	}
}

func TestNewFormatter_NilWriter(t *testing.T) {
	cfg := Config{Writer: nil, Format: "text"}
	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() should have failed with nil writer")
	}
}

func TestFormatPostsEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, _ := New(cfg)

	err := f.FormatPosts([]*types.Post{})
	if err != nil {
		t.Fatalf("FormatPosts() failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("FormatPosts() should have written output for empty posts")
	}
}

func TestFormatPostNil(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, _ := New(cfg)

	err := f.FormatPost(nil)
	if err != nil {
		t.Fatalf("FormatPost() failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("FormatPost() should have written output for nil post")
	}
}

func TestFormatPost(t *testing.T) {
	post := &types.Post{
		ThingData: types.ThingData{
			ID:   "abc123",
			Name: "t3_abc123",
		},
		Votable: types.Votable{
			Score: 42,
		},
		Created: types.Created{
			CreatedUTC: 1000000000,
		},
		Title:       "Test Post",
		Author:      "testuser",
		Subreddit:   "golang",
		NumComments: 5,
		URL:         "https://example.com",
		Domain:      "example.com",
		IsSelf:      false,
		UpvoteRatio: 0.95,
	}

	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, _ := New(cfg)

	err := f.FormatPost(post)
	if err != nil {
		t.Fatalf("FormatPost() failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("FormatPost() should have written output")
	}

	output := buf.String()
	if !contains(output, "Test Post") {
		t.Fatal("Output should contain post title")
	}
}

func TestFormatCommentsEmpty(t *testing.T) {
	response := &types.CommentsResponse{
		Comments: []*types.Comment{},
	}

	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, _ := New(cfg)

	err := f.FormatComments(response)
	if err != nil {
		t.Fatalf("FormatComments() failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("FormatComments() should have written output")
	}
}

func TestFormatSubreddit(t *testing.T) {
	sub := &types.SubredditData{
		ThingData: types.ThingData{
			ID:   "sub123",
			Name: "t5_sub123",
		},
		DisplayName: "golang",
		Title:       "The Go Programming Language",
		Subscribers: 100000,
		Over18:      false,
	}

	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, _ := New(cfg)

	err := f.FormatSubreddit(sub)
	if err != nil {
		t.Fatalf("FormatSubreddit() failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("FormatSubreddit() should have written output")
	}

	output := buf.String()
	if !contains(output, "golang") {
		t.Fatal("Output should contain subreddit name")
	}
}

func TestFormatUser(t *testing.T) {
	user := &types.AccountData{
		ThingData: types.ThingData{
			ID:   "user123",
			Name: "testuser",
		},
		Created: types.Created{
			CreatedUTC: 1000000000,
		},
		LinkKarma:    1000,
		CommentKarma: 5000,
		IsGold:       true,
		IsMod:        false,
	}

	buf := &bytes.Buffer{}
	cfg := Config{Writer: buf, Format: "text"}
	f, _ := New(cfg)

	err := f.FormatUser(user)
	if err != nil {
		t.Fatalf("FormatUser() failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("FormatUser() should have written output")
	}

	output := buf.String()
	if !contains(output, "testuser") {
		t.Fatal("Output should contain user name")
	}
}

// Helper function to check if string contains substring.
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
