package types

import (
	"encoding/json"
	"testing"
)

func TestEdited_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantEdit  bool
		wantTime  float64
		wantError bool
	}{
		{
			name:      "false boolean",
			input:     `false`,
			wantEdit:  false,
			wantTime:  0,
			wantError: false,
		},
		{
			name:      "true boolean",
			input:     `true`,
			wantEdit:  true,
			wantTime:  0,
			wantError: false,
		},
		{
			name:      "null value",
			input:     `null`,
			wantEdit:  false,
			wantTime:  0,
			wantError: false,
		},
		{
			name:      "timestamp",
			input:     `1234567890.5`,
			wantEdit:  true,
			wantTime:  1234567890.5,
			wantError: false,
		},
		{
			name:      "invalid value",
			input:     `"invalid"`,
			wantEdit:  false,
			wantTime:  0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e Edited
			err := json.Unmarshal([]byte(tt.input), &e)

			if (err != nil) != tt.wantError {
				t.Errorf("Edited.UnmarshalJSON() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if err != nil {
				return
			}

			if e.IsEdited != tt.wantEdit {
				t.Errorf("Edited.IsEdited = %v, want %v", e.IsEdited, tt.wantEdit)
			}
			if e.Timestamp != tt.wantTime {
				t.Errorf("Edited.Timestamp = %v, want %v", e.Timestamp, tt.wantTime)
			}
		})
	}
}

func TestThingData(t *testing.T) {
	td := ThingData{
		ID:   "abc123",
		Name: "t3_abc123",
	}

	if got := td.GetID(); got != "abc123" {
		t.Errorf("ThingData.GetID() = %v, want %v", got, "abc123")
	}

	if got := td.GetName(); got != "t3_abc123" {
		t.Errorf("ThingData.GetName() = %v, want %v", got, "t3_abc123")
	}
}

func TestPagination(t *testing.T) {
	// Test that Pagination fields work correctly
	p := Pagination{
		Limit:  100,
		After:  "t3_abc123",
		Before: "",
	}

	if p.Limit != 100 {
		t.Errorf("Pagination.Limit = %v, want %v", p.Limit, 100)
	}
	if p.After != "t3_abc123" {
		t.Errorf("Pagination.After = %v, want %v", p.After, "t3_abc123")
	}
	if p.Before != "" {
		t.Errorf("Pagination.Before = %v, want %v", p.Before, "")
	}
}

func TestPostsResponse(t *testing.T) {
	// Test PostsResponse with sample data
	pr := &PostsResponse{
		Posts: []*Post{
			{
				ThingData: ThingData{ID: "post1", Name: "t3_post1"},
				Title:     "Test Post 1",
			},
			{
				ThingData: ThingData{ID: "post2", Name: "t3_post2"},
				Title:     "Test Post 2",
			},
		},
		AfterFullname:  "t3_post2",
		BeforeFullname: "t3_post0",
	}

	if len(pr.Posts) != 2 {
		t.Errorf("PostsResponse.Posts length = %v, want %v", len(pr.Posts), 2)
	}
	if pr.AfterFullname != "t3_post2" {
		t.Errorf("PostsResponse.AfterFullname = %v, want %v", pr.AfterFullname, "t3_post2")
	}
}

func TestCommentsResponse(t *testing.T) {
	// Test CommentsResponse structure
	cr := &CommentsResponse{
		Post: &Post{
			ThingData: ThingData{ID: "post1", Name: "t3_post1"},
			Title:     "Test Post",
		},
		Comments: []*Comment{
			{
				ThingData: ThingData{ID: "comment1", Name: "t1_comment1"},
				Body:      "Test comment",
			},
		},
		MoreIDs: []string{"comment2", "comment3"},
	}

	if cr.Post.Title != "Test Post" {
		t.Errorf("CommentsResponse.Post.Title = %v, want %v", cr.Post.Title, "Test Post")
	}
	if len(cr.Comments) != 1 {
		t.Errorf("CommentsResponse.Comments length = %v, want %v", len(cr.Comments), 1)
	}
	if len(cr.MoreIDs) != 2 {
		t.Errorf("CommentsResponse.MoreIDs length = %v, want %v", len(cr.MoreIDs), 2)
	}
}

func TestSubredditData(t *testing.T) {
	// Test SubredditData fields
	sub := &SubredditData{
		ThingData:         ThingData{ID: "sub1", Name: "t5_sub1"},
		DisplayName:       "golang",
		Title:             "The Go Programming Language",
		PublicDescription: "A place to discuss Go",
		Subscribers:       100000,
		URL:               "/r/golang",
	}

	if sub.DisplayName != "golang" {
		t.Errorf("SubredditData.DisplayName = %v, want %v", sub.DisplayName, "golang")
	}
	if sub.Subscribers != 100000 {
		t.Errorf("SubredditData.Subscribers = %v, want %v", sub.Subscribers, 100000)
	}
}

func TestMoreCommentsRequest(t *testing.T) {
	// Test MoreCommentsRequest structure
	mcr := &MoreCommentsRequest{
		LinkID:     "t3_abc123",
		CommentIDs: []string{"comment1", "comment2"},
		Sort:       "confidence",
	}

	if mcr.LinkID != "t3_abc123" {
		t.Errorf("MoreCommentsRequest.LinkID = %v, want %v", mcr.LinkID, "t3_abc123")
	}
	if len(mcr.CommentIDs) != 2 {
		t.Errorf("MoreCommentsRequest.CommentIDs length = %v, want %v", len(mcr.CommentIDs), 2)
	}
	if mcr.Sort != "confidence" {
		t.Errorf("MoreCommentsRequest.Sort = %v, want %v", mcr.Sort, "confidence")
	}
}
