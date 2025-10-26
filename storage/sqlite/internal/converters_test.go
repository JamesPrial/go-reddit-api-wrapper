package sqlite

import (
	"database/sql"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/storage/internal"
	"github.com/stretchr/testify/require"
)

// TestConverters_String tests StringToNullString and NullStringToString conversions.
func TestConverters_String(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
		wantVal   string
	}{
		{
			name:      "empty string converts to NULL",
			input:     "",
			wantValid: false,
			wantVal:   "",
		},
		{
			name:      "non-empty string converts to valid NullString",
			input:     "hello",
			wantValid: true,
			wantVal:   "hello",
		},
		{
			name:      "space string converts to valid NullString",
			input:     " ",
			wantValid: true,
			wantVal:   " ",
		},
		{
			name:      "long string converts correctly",
			input:     "this is a very long string with multiple words and special chars !@#$%",
			wantValid: true,
			wantVal:   "this is a very long string with multiple words and special chars !@#$%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test StringToNullString
			nullStr := internal.StringToNullString(tt.input)
			require.Equal(t, tt.wantValid, nullStr.Valid, "Valid field mismatch")
			if tt.wantValid {
				require.Equal(t, tt.wantVal, nullStr.String, "String value mismatch")
			}

			// Test NullStringToString
			recovered := internal.NullStringToString(nullStr)
			require.Equal(t, tt.input, recovered, "recovered string should match original")
		})
	}

	t.Run("round-trip conversion preserves non-empty strings", func(t *testing.T) {
		original := "test data"
		nullStr := internal.StringToNullString(original)
		recovered := internal.NullStringToString(nullStr)
		require.Equal(t, original, recovered)
	})

	t.Run("round-trip conversion of empty string returns empty", func(t *testing.T) {
		original := ""
		nullStr := internal.StringToNullString(original)
		recovered := internal.NullStringToString(nullStr)
		require.Equal(t, "", recovered)
	})
}

// TestConverters_Int64 tests Int64ToNullInt64 and NullInt64ToInt64 conversions.
func TestConverters_Int64(t *testing.T) {
	tests := []struct {
		name      string
		input     int64
		wantValid bool
		wantVal   int64
	}{
		{
			name:      "zero converts to NULL",
			input:     0,
			wantValid: false,
			wantVal:   0,
		},
		{
			name:      "positive value converts to valid NullInt64",
			input:     42,
			wantValid: true,
			wantVal:   42,
		},
		{
			name:      "negative value converts to valid NullInt64",
			input:     -100,
			wantValid: true,
			wantVal:   -100,
		},
		{
			name:      "large positive value",
			input:     9223372036854775807, // max int64
			wantValid: true,
			wantVal:   9223372036854775807,
		},
		{
			name:      "large negative value",
			input:     -9223372036854775808, // min int64
			wantValid: true,
			wantVal:   -9223372036854775808,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Int64ToNullInt64
			nullInt := internal.Int64ToNullInt64(tt.input)
			require.Equal(t, tt.wantValid, nullInt.Valid, "Valid field mismatch")
			if tt.wantValid {
				require.Equal(t, tt.wantVal, nullInt.Int64, "Int64 value mismatch")
			}

			// Test NullInt64ToInt64
			recovered := internal.NullInt64ToInt64(nullInt)
			require.Equal(t, tt.input, recovered, "recovered int64 should match original")
		})
	}

	t.Run("round-trip conversion preserves non-zero values", func(t *testing.T) {
		original := int64(123)
		nullInt := internal.Int64ToNullInt64(original)
		recovered := internal.NullInt64ToInt64(nullInt)
		require.Equal(t, original, recovered)
	})

	t.Run("NULL NullInt64ToInt64 returns zero", func(t *testing.T) {
		nullInt := sql.NullInt64{Valid: false}
		result := internal.NullInt64ToInt64(nullInt)
		require.Equal(t, int64(0), result)
	})
}

// TestConverters_IntToInt64 tests IntToNullInt64 and NullInt64ToInt conversions.
func TestConverters_IntToInt64(t *testing.T) {
	tests := []struct {
		name      string
		input     int
		wantValid bool
		wantVal   int
	}{
		{
			name:      "zero converts to NULL",
			input:     0,
			wantValid: false,
			wantVal:   0,
		},
		{
			name:      "positive value converts to valid NullInt64",
			input:     99,
			wantValid: true,
			wantVal:   99,
		},
		{
			name:      "negative value converts to valid NullInt64",
			input:     -50,
			wantValid: true,
			wantVal:   -50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IntToNullInt64
			nullInt := internal.IntToNullInt64(tt.input)
			require.Equal(t, tt.wantValid, nullInt.Valid, "Valid field mismatch")
			if tt.wantValid {
				require.Equal(t, int64(tt.wantVal), nullInt.Int64, "Int64 value mismatch")
			}

			// Test NullInt64ToInt
			recovered := internal.NullInt64ToInt(nullInt)
			require.Equal(t, tt.input, recovered, "recovered int should match original")
		})
	}

	t.Run("round-trip preserves int values", func(t *testing.T) {
		original := 456
		nullInt := internal.IntToNullInt64(original)
		recovered := internal.NullInt64ToInt(nullInt)
		require.Equal(t, original, recovered)
	})
}

// TestConverters_Bool tests BoolToNullBool and NullBoolToBool conversions.
func TestConverters_Bool(t *testing.T) {
	tests := []struct {
		name      string
		input     bool
		wantValid bool
		wantVal   bool
	}{
		{
			name:      "false converts to valid NullBool",
			input:     false,
			wantValid: true,
			wantVal:   false,
		},
		{
			name:      "true converts to valid NullBool",
			input:     true,
			wantValid: true,
			wantVal:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test BoolToNullBool
			nullBool := internal.BoolToNullBool(tt.input)
			require.Equal(t, tt.wantValid, nullBool.Valid, "BoolToNullBool should always be Valid")
			require.Equal(t, tt.wantVal, nullBool.Bool, "Bool value mismatch")

			// Test NullBoolToBool
			recovered := internal.NullBoolToBool(nullBool)
			require.Equal(t, tt.input, recovered, "recovered bool should match original")
		})
	}

	t.Run("round-trip preserves true value", func(t *testing.T) {
		original := true
		nullBool := internal.BoolToNullBool(original)
		recovered := internal.NullBoolToBool(nullBool)
		require.Equal(t, original, recovered)
	})

	t.Run("round-trip preserves false value", func(t *testing.T) {
		original := false
		nullBool := internal.BoolToNullBool(original)
		recovered := internal.NullBoolToBool(nullBool)
		require.Equal(t, original, recovered)
	})

	t.Run("NULL NullBoolToBool returns false", func(t *testing.T) {
		nullBool := sql.NullBool{Valid: false}
		result := internal.NullBoolToBool(nullBool)
		require.Equal(t, false, result)
	})
}

// TestConverters_Float64 tests Float64ToNullFloat64 and NullFloat64ToFloat64 conversions.
func TestConverters_Float64(t *testing.T) {
	tests := []struct {
		name      string
		input     float64
		wantValid bool
		wantVal   float64
	}{
		{
			name:      "zero converts to NULL",
			input:     0.0,
			wantValid: false,
			wantVal:   0.0,
		},
		{
			name:      "positive value converts to valid NullFloat64",
			input:     3.14159,
			wantValid: true,
			wantVal:   3.14159,
		},
		{
			name:      "negative value converts to valid NullFloat64",
			input:     -2.71828,
			wantValid: true,
			wantVal:   -2.71828,
		},
		{
			name:      "very small positive value",
			input:     0.0001,
			wantValid: true,
			wantVal:   0.0001,
		},
		{
			name:      "very large value",
			input:     1.7976931348623157e+308, // near max float64
			wantValid: true,
			wantVal:   1.7976931348623157e+308,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Float64ToNullFloat64
			nullFloat := internal.Float64ToNullFloat64(tt.input)
			require.Equal(t, tt.wantValid, nullFloat.Valid, "Valid field mismatch")
			if tt.wantValid {
				require.Equal(t, tt.wantVal, nullFloat.Float64, "Float64 value mismatch")
			}

			// Test NullFloat64ToFloat64
			recovered := internal.NullFloat64ToFloat64(nullFloat)
			require.Equal(t, tt.input, recovered, "recovered float64 should match original")
		})
	}

	t.Run("round-trip conversion preserves non-zero values", func(t *testing.T) {
		original := 99.99
		nullFloat := internal.Float64ToNullFloat64(original)
		recovered := internal.NullFloat64ToFloat64(nullFloat)
		require.Equal(t, original, recovered)
	})

	t.Run("NULL NullFloat64ToFloat64 returns zero", func(t *testing.T) {
		nullFloat := sql.NullFloat64{Valid: false}
		result := internal.NullFloat64ToFloat64(nullFloat)
		require.Equal(t, 0.0, result)
	})
}

// TestConverters_Time tests TimeToNullTime and NullTimeToTime conversions.
func TestConverters_Time(t *testing.T) {
	now := time.Now()
	epoch := time.Unix(0, 0)

	tests := []struct {
		name      string
		input     time.Time
		wantValid bool
		wantTime  time.Time
	}{
		{
			name:      "zero time converts to NULL",
			input:     time.Time{},
			wantValid: false,
			wantTime:  time.Time{},
		},
		{
			name:      "non-zero time converts to valid NullTime",
			input:     now,
			wantValid: true,
			wantTime:  now,
		},
		{
			name:      "unix epoch (non-zero) converts to valid NullTime",
			input:     epoch,
			wantValid: true,
			wantTime:  epoch,
		},
		{
			name:      "past time converts to valid NullTime",
			input:     time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			wantValid: true,
			wantTime:  time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test TimeToNullTime
			nullTime := internal.TimeToNullTime(tt.input)
			require.Equal(t, tt.wantValid, nullTime.Valid, "Valid field mismatch")
			if tt.wantValid {
				require.Equal(t, tt.wantTime, nullTime.Time, "Time value mismatch")
			}

			// Test NullTimeToTime
			recovered := internal.NullTimeToTime(nullTime)
			require.Equal(t, tt.input, recovered, "recovered time should match original")
		})
	}

	t.Run("round-trip preserves non-zero time", func(t *testing.T) {
		original := time.Date(2023, 6, 15, 10, 30, 45, 123456789, time.UTC)
		nullTime := internal.TimeToNullTime(original)
		recovered := internal.NullTimeToTime(nullTime)
		require.Equal(t, original, recovered)
	})

	t.Run("round-trip preserves zero time", func(t *testing.T) {
		original := time.Time{}
		nullTime := internal.TimeToNullTime(original)
		recovered := internal.NullTimeToTime(nullTime)
		require.True(t, recovered.IsZero())
	})

	t.Run("NULL NullTimeToTime returns zero time", func(t *testing.T) {
		nullTime := sql.NullTime{Valid: false}
		result := internal.NullTimeToTime(nullTime)
		require.True(t, result.IsZero())
	})
}

// TestConverters_PostScanDestToPost tests postScanDest.toPost() conversion.
func TestConverters_PostScanDestToPost(t *testing.T) {
	t.Run("minimal post with required fields", func(t *testing.T) {
		dest := &postScanDest{
			id:          "abc123",
			name:        "t3_abc123",
			score:       42,
			ups:         50,
			downs:       8,
			created:     1609459200.0,
			createdUTC:  1609459200.0,
			author:      "testuser",
			domain:      "self.golang",
			title:       "Test Post",
			url:         "https://reddit.com/r/golang/comments/abc123/test_post/",
			permalink:   "/r/golang/comments/abc123/test_post/",
			subreddit:   "golang",
			subredditID: "t5_xyz123",
		}

		post := dest.toPost()

		require.NotNil(t, post)
		require.Equal(t, "abc123", post.ID)
		require.Equal(t, "t3_abc123", post.Name)
		require.Equal(t, 42, post.Score)
		require.Equal(t, 50, post.Ups)
		require.Equal(t, 8, post.Downs)
		require.Equal(t, 1609459200.0, post.Created.Created)
		require.Equal(t, 1609459200.0, post.Created.CreatedUTC)
		require.Equal(t, "testuser", post.Author)
		require.Equal(t, "golang", post.Subreddit)
	})

	t.Run("post with all nullable fields populated", func(t *testing.T) {
		dest := &postScanDest{
			id:              "post1",
			name:            "t3_post1",
			score:           100,
			ups:             120,
			downs:           20,
			created:         1000.0,
			createdUTC:      1000.0,
			author:          "author1",
			domain:          "reddit.com",
			title:           "Title",
			url:             "https://example.com",
			permalink:       "/r/test/comments/post1/",
			subreddit:       "test",
			subredditID:     "t5_test",
			likes:           sql.NullInt64{Int64: 1, Valid: true},
			authorFlairCSS:  sql.NullString{String: "flair-css", Valid: true},
			authorFlairText: sql.NullString{String: "Flair Text", Valid: true},
			linkFlairCSS:    sql.NullString{String: "link-flair-css", Valid: true},
			linkFlairText:   sql.NullString{String: "Link Flair", Valid: true},
			media:           sql.NullString{String: `{"type":"image"}`, Valid: true},
			mediaEmbed:      sql.NullString{String: `{"embed":"html"}`, Valid: true},
			selftextHTML:    sql.NullString{String: "<p>HTML content</p>", Valid: true},
			distinguished:   sql.NullString{String: "moderator", Valid: true},
			editedTs:        sql.NullFloat64{Float64: 2000.0, Valid: true},
			editedIsEdited:  true,
		}

		post := dest.toPost()

		require.NotNil(t, post)
		require.NotNil(t, post.Likes)
		require.True(t, *post.Likes)
		require.NotNil(t, post.AuthorFlairCSSClass)
		require.Equal(t, "flair-css", *post.AuthorFlairCSSClass)
		require.NotNil(t, post.AuthorFlairText)
		require.Equal(t, "Flair Text", *post.AuthorFlairText)
		require.NotNil(t, post.LinkFlairCSSClass)
		require.Equal(t, "link-flair-css", *post.LinkFlairCSSClass)
		require.NotNil(t, post.LinkFlairText)
		require.Equal(t, "Link Flair", *post.LinkFlairText)
		require.NotNil(t, post.SelfTextHTML)
		require.Equal(t, "<p>HTML content</p>", *post.SelfTextHTML)
		require.NotNil(t, post.Distinguished)
		require.Equal(t, "moderator", *post.Distinguished)
		require.True(t, post.Edited.IsEdited)
		require.Equal(t, 2000.0, post.Edited.Timestamp)
	})

	t.Run("post with NULL nullable fields", func(t *testing.T) {
		dest := &postScanDest{
			id:          "post2",
			name:        "t3_post2",
			score:       0,
			ups:         0,
			downs:       0,
			created:     500.0,
			createdUTC:  500.0,
			author:      "anon",
			domain:      "self.test",
			title:       "No Flairs",
			url:         "https://reddit.com",
			permalink:   "/r/test/comments/post2/",
			subreddit:   "test",
			subredditID: "t5_test",
			// All nullable fields left as zero values (NULL)
			likes:           sql.NullInt64{},
			authorFlairCSS:  sql.NullString{},
			authorFlairText: sql.NullString{},
			linkFlairCSS:    sql.NullString{},
			linkFlairText:   sql.NullString{},
			media:           sql.NullString{},
			mediaEmbed:      sql.NullString{},
			selftextHTML:    sql.NullString{},
			distinguished:   sql.NullString{},
			editedTs:        sql.NullFloat64{},
		}

		post := dest.toPost()

		require.NotNil(t, post)
		require.Nil(t, post.Likes)
		require.Nil(t, post.AuthorFlairCSSClass)
		require.Nil(t, post.AuthorFlairText)
		require.Nil(t, post.LinkFlairCSSClass)
		require.Nil(t, post.LinkFlairText)
		require.Nil(t, post.SelfTextHTML)
		require.Nil(t, post.Distinguished)
		require.False(t, post.Edited.IsEdited)
		require.Equal(t, 0.0, post.Edited.Timestamp)
	})

	t.Run("post with likes as false (0)", func(t *testing.T) {
		dest := &postScanDest{
			id:          "post3",
			name:        "t3_post3",
			score:       0,
			ups:         0,
			downs:       0,
			created:     0.0,
			createdUTC:  0.0,
			author:      "user",
			domain:      "test",
			title:       "Test",
			url:         "test",
			permalink:   "test",
			subreddit:   "test",
			subredditID: "test",
			likes:       sql.NullInt64{Int64: 0, Valid: true}, // 0 should mean false
		}

		post := dest.toPost()

		require.NotNil(t, post.Likes)
		require.False(t, *post.Likes)
	})

	t.Run("post with empty media strings treated as NULL", func(t *testing.T) {
		dest := &postScanDest{
			id:          "post4",
			name:        "t3_post4",
			score:       0,
			ups:         0,
			downs:       0,
			created:     0.0,
			createdUTC:  0.0,
			author:      "user",
			domain:      "test",
			title:       "Test",
			url:         "test",
			permalink:   "test",
			subreddit:   "test",
			subredditID: "test",
			media:       sql.NullString{String: "", Valid: true}, // empty string
			mediaEmbed:  sql.NullString{String: "", Valid: true},
		}

		post := dest.toPost()

		require.Nil(t, post.Media)
		require.Nil(t, post.MediaEmbed)
	})
}

// TestConverters_CommentScanDestToComment tests commentScanDest.toComment() conversion.
func TestConverters_CommentScanDestToComment(t *testing.T) {
	t.Run("minimal comment with required fields", func(t *testing.T) {
		dest := &commentScanDest{
			id:          "abc123",
			name:        "t1_abc123",
			score:       10,
			ups:         12,
			downs:       2,
			created:     1609459200.0,
			createdUTC:  1609459200.0,
			author:      "testuser",
			body:        "This is a test comment",
			bodyHTML:    "&lt;div&gt;This is a test comment&lt;/div&gt;",
			linkID:      "t3_post123",
			parentID:    "t3_post123",
			subreddit:   "golang",
			subredditID: "t5_xyz123",
		}

		comment := dest.toComment()

		require.NotNil(t, comment)
		require.Equal(t, "abc123", comment.ID)
		require.Equal(t, "t1_abc123", comment.Name)
		require.Equal(t, 10, comment.Score)
		require.Equal(t, 12, comment.Ups)
		require.Equal(t, 2, comment.Downs)
		require.Equal(t, 1609459200.0, comment.Created.Created)
		require.Equal(t, 1609459200.0, comment.Created.CreatedUTC)
		require.Equal(t, "testuser", comment.Author)
		require.Equal(t, "This is a test comment", comment.Body)
		require.Equal(t, "golang", comment.Subreddit)
	})

	t.Run("comment with all nullable fields populated", func(t *testing.T) {
		dest := &commentScanDest{
			id:              "cmt1",
			name:            "t1_cmt1",
			score:           50,
			ups:             55,
			downs:           5,
			created:         1000.0,
			createdUTC:      1000.0,
			author:          "user1",
			body:            "Comment body",
			bodyHTML:        "<p>Comment body</p>",
			linkID:          "t3_link1",
			parentID:        "t1_parent1",
			subreddit:       "test",
			subredditID:     "t5_test",
			gilded:          2,
			saved:           true,
			scoreHidden:     false,
			editedIsEdited:  true,
			editedTs:        2000.0,
			likes:           sql.NullInt64{Int64: 1, Valid: true},
			approvedBy:      sql.NullString{String: "mod1", Valid: true},
			authorFlairCSS:  sql.NullString{String: "user-flair", Valid: true},
			authorFlairText: sql.NullString{String: "User Flair", Valid: true},
			bannedBy:        sql.NullString{String: "", Valid: false}, // explicitly nil
			linkAuthor:      sql.NullString{String: "post_author", Valid: true},
			linkTitle:       sql.NullString{String: "Post Title", Valid: true},
			linkURL:         sql.NullString{String: "https://reddit.com/r/test", Valid: true},
			numReports:      sql.NullInt64{Int64: 1, Valid: true},
			distinguished:   sql.NullString{String: "moderator", Valid: true},
		}

		comment := dest.toComment()

		require.NotNil(t, comment)
		require.NotNil(t, comment.Likes)
		require.True(t, *comment.Likes)
		require.NotNil(t, comment.ApprovedBy)
		require.Equal(t, "mod1", *comment.ApprovedBy)
		require.NotNil(t, comment.AuthorFlairCSSClass)
		require.Equal(t, "user-flair", *comment.AuthorFlairCSSClass)
		require.NotNil(t, comment.AuthorFlairText)
		require.Equal(t, "User Flair", *comment.AuthorFlairText)
		require.Nil(t, comment.BannedBy)
		require.Equal(t, "post_author", comment.LinkAuthor)
		require.Equal(t, "Post Title", comment.LinkTitle)
		require.Equal(t, "https://reddit.com/r/test", comment.LinkURL)
		require.NotNil(t, comment.NumReports)
		require.Equal(t, 1, *comment.NumReports)
		require.NotNil(t, comment.Distinguished)
		require.Equal(t, "moderator", *comment.Distinguished)
		require.True(t, comment.Edited.IsEdited)
		require.Equal(t, 2000.0, comment.Edited.Timestamp)
	})

	t.Run("comment with NULL nullable fields", func(t *testing.T) {
		dest := &commentScanDest{
			id:          "cmt2",
			name:        "t1_cmt2",
			score:       0,
			ups:         0,
			downs:       0,
			created:     500.0,
			createdUTC:  500.0,
			author:      "anon",
			body:        "Simple comment",
			bodyHTML:    "Simple comment",
			linkID:      "t3_link2",
			parentID:    "t3_link2",
			subreddit:   "test",
			subredditID: "t5_test",
			// All nullable fields left as zero values (NULL)
			likes:           sql.NullInt64{},
			approvedBy:      sql.NullString{},
			authorFlairCSS:  sql.NullString{},
			authorFlairText: sql.NullString{},
			bannedBy:        sql.NullString{},
			linkAuthor:      sql.NullString{},
			linkTitle:       sql.NullString{},
			linkURL:         sql.NullString{},
			numReports:      sql.NullInt64{},
			distinguished:   sql.NullString{},
		}

		comment := dest.toComment()

		require.NotNil(t, comment)
		require.Nil(t, comment.Likes)
		require.Nil(t, comment.ApprovedBy)
		require.Nil(t, comment.AuthorFlairCSSClass)
		require.Nil(t, comment.AuthorFlairText)
		require.Nil(t, comment.BannedBy)
		require.Equal(t, "", comment.LinkAuthor)
		require.Equal(t, "", comment.LinkTitle)
		require.Equal(t, "", comment.LinkURL)
		require.Nil(t, comment.NumReports)
		require.Nil(t, comment.Distinguished)
	})

	t.Run("comment with likes as false (0)", func(t *testing.T) {
		dest := &commentScanDest{
			id:          "cmt3",
			name:        "t1_cmt3",
			score:       0,
			ups:         0,
			downs:       0,
			created:     0.0,
			createdUTC:  0.0,
			author:      "user",
			body:        "test",
			bodyHTML:    "test",
			linkID:      "t3_link",
			parentID:    "t1_parent",
			subreddit:   "test",
			subredditID: "test",
			likes:       sql.NullInt64{Int64: 0, Valid: true}, // 0 should mean false
		}

		comment := dest.toComment()

		require.NotNil(t, comment.Likes)
		require.False(t, *comment.Likes)
	})

	t.Run("comment initializes replies and more children as empty slices", func(t *testing.T) {
		dest := &commentScanDest{
			id:          "cmt4",
			name:        "t1_cmt4",
			score:       0,
			ups:         0,
			downs:       0,
			created:     0.0,
			createdUTC:  0.0,
			author:      "user",
			body:        "test",
			bodyHTML:    "test",
			linkID:      "t3_link",
			parentID:    "t1_parent",
			subreddit:   "test",
			subredditID: "test",
		}

		comment := dest.toComment()

		require.NotNil(t, comment.Replies)
		require.Equal(t, 0, len(comment.Replies))
		require.NotNil(t, comment.MoreChildrenIDs)
		require.Equal(t, 0, len(comment.MoreChildrenIDs))
	})

	t.Run("comment with numreports as specific value", func(t *testing.T) {
		dest := &commentScanDest{
			id:          "cmt5",
			name:        "t1_cmt5",
			score:       0,
			ups:         0,
			downs:       0,
			created:     0.0,
			createdUTC:  0.0,
			author:      "user",
			body:        "test",
			bodyHTML:    "test",
			linkID:      "t3_link",
			parentID:    "t1_parent",
			subreddit:   "test",
			subredditID: "test",
			numReports:  sql.NullInt64{Int64: 5, Valid: true},
		}

		comment := dest.toComment()

		require.NotNil(t, comment.NumReports)
		require.Equal(t, 5, *comment.NumReports)
	})
}

// TestHelpers_BoolPtrToNullInt64 tests the boolPtrToNullInt64 helper function.
func TestHelpers_BoolPtrToNullInt64(t *testing.T) {
	tests := []struct {
		name      string
		input     *bool
		wantValid bool
		wantVal   int64
	}{
		{
			name:      "nil pointer converts to NULL",
			input:     nil,
			wantValid: false,
			wantVal:   0,
		},
		{
			name:      "true pointer converts to 1",
			input:     ptrBool(true),
			wantValid: true,
			wantVal:   1,
		},
		{
			name:      "false pointer converts to 0",
			input:     ptrBool(false),
			wantValid: true,
			wantVal:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolPtrToNullInt64(tt.input)
			require.Equal(t, tt.wantValid, result.Valid)
			if tt.wantValid {
				require.Equal(t, tt.wantVal, result.Int64)
			}
		})
	}
}

// TestHelpers_StringPtrToNullString tests the stringPtrToNullString helper function.
func TestHelpers_StringPtrToNullString(t *testing.T) {
	tests := []struct {
		name      string
		input     *string
		wantValid bool
		wantVal   string
	}{
		{
			name:      "nil pointer converts to NULL",
			input:     nil,
			wantValid: false,
			wantVal:   "",
		},
		{
			name:      "empty string pointer converts to valid NullString",
			input:     ptrString(""),
			wantValid: true,
			wantVal:   "",
		},
		{
			name:      "non-empty string pointer converts to valid NullString",
			input:     ptrString("hello"),
			wantValid: true,
			wantVal:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringPtrToNullString(tt.input)
			require.Equal(t, tt.wantValid, result.Valid)
			if tt.wantValid {
				require.Equal(t, tt.wantVal, result.String)
			}
		})
	}
}

// TestHelpers_IntPtrToNullInt64 tests the intPtrToNullInt64 helper function.
func TestHelpers_IntPtrToNullInt64(t *testing.T) {
	tests := []struct {
		name      string
		input     *int
		wantValid bool
		wantVal   int64
	}{
		{
			name:      "nil pointer converts to NULL",
			input:     nil,
			wantValid: false,
			wantVal:   0,
		},
		{
			name:      "zero pointer converts to valid 0",
			input:     ptrInt(0),
			wantValid: true,
			wantVal:   0,
		},
		{
			name:      "positive pointer converts to valid NullInt64",
			input:     ptrInt(42),
			wantValid: true,
			wantVal:   42,
		},
		{
			name:      "negative pointer converts to valid NullInt64",
			input:     ptrInt(-99),
			wantValid: true,
			wantVal:   -99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intPtrToNullInt64(tt.input)
			require.Equal(t, tt.wantValid, result.Valid)
			if tt.wantValid {
				require.Equal(t, tt.wantVal, result.Int64)
			}
		})
	}
}

// Helper functions for tests
func ptrBool(b bool) *bool       { return &b }
func ptrString(s string) *string { return &s }
func ptrInt(i int) *int          { return &i }
