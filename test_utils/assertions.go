package test_utils

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/validation"
)

func AssertValidId(id string) error {
	if id == "" {
		return fmt.Errorf("id is empty")
	}
	if !validation.IsValidBase36(id) {
		return fmt.Errorf("id has invalid format: %s", id)
	}
	return nil
}

func AssertValidFullname(fullname string) error {
	if fullname == "" {
		return fmt.Errorf("fullname is empty")
	}
	if !validation.IsValidFullname(fullname) {
		return fmt.Errorf("fullname has invalid format: %s", fullname)
	}
	return nil
}

func AssertValidRedditObject(ro types.RedditObject) error {
	id := ro.GetID()
	fullname := ro.GetName()
	if err := AssertValidId(id); err != nil {
		return fmt.Errorf("RedditObject ID is invalid: %v", err)
	}
	if err := AssertValidFullname(fullname); err != nil {
		return fmt.Errorf("RedditObject fullname is invalid: %v", err)
	}
	return nil

}

func AssertRedditObjectType(ro types.RedditObject, expectedType string) error {
	actualType := ro.GetName()[:types.PREFIX_IDX]
	if actualType != expectedType {
		return fmt.Errorf("expected RedditObject type %s, got %s", expectedType, actualType)
	}

	return nil
}
func AssertValidCreated(c types.Created) error {
	return validation.ValidateCreated(&c)
}

func AssertValidVotable(v types.Votable) error {
	return validation.ValidateVotable(&v)
}

// AssertValidPost validates that a post has all required fields and valid data
func AssertValidPost(post types.Post) error {
	return validation.ValidatePost(&post)
}

// AssertValidComment validates that a comment has all required fields and valid data
func AssertValidComment(comment types.Comment) error {
	return validation.ValidateComment(&comment)
}

// AssertValidSubreddit validates that subreddit data is valid
func AssertValidSubreddit(subreddit types.SubredditData) error {
	if subreddit.DisplayName == "" {
		return fmt.Errorf("subreddit display name is empty")
	}

	if subreddit.Name == "" {
		return fmt.Errorf("subreddit name is empty")
	}

	if subreddit.Subscribers < 0 {
		return fmt.Errorf("subreddit subscriber count is negative: %d", subreddit.Subscribers)
	}

	return nil
}

// AssertPostsEqual validates that two posts are equal (ignoring timestamps)
func AssertPostsEqual(expected, actual types.Post) error {
	if expected.ID != actual.ID {
		return fmt.Errorf("post ID mismatch: expected %s, got %s", expected.ID, actual.ID)
	}

	if expected.Title != actual.Title {
		return fmt.Errorf("post title mismatch: expected %s, got %s", expected.Title, actual.Title)
	}

	if expected.Author != actual.Author {
		return fmt.Errorf("post author mismatch: expected %s, got %s", expected.Author, actual.Author)
	}

	if expected.Subreddit != actual.Subreddit {
		return fmt.Errorf("post subreddit mismatch: expected %s, got %s", expected.Subreddit, actual.Subreddit)
	}

	if expected.SelfText != actual.SelfText {
		return fmt.Errorf("post body mismatch: expected %s, got %s", expected.SelfText, actual.SelfText)
	}

	if expected.Score != actual.Score {
		return fmt.Errorf("post score mismatch: expected %d, got %d", expected.Score, actual.Score)
	}

	if expected.NumComments != actual.NumComments {
		return fmt.Errorf("post comment count mismatch: expected %d, got %d", expected.NumComments, actual.NumComments)
	}

	if expected.URL != actual.URL {
		return fmt.Errorf("post URL mismatch: expected %s, got %s", expected.URL, actual.URL)
	}

	if expected.Permalink != actual.Permalink {
		return fmt.Errorf("post permalink mismatch: expected %s, got %s", expected.Permalink, actual.Permalink)
	}

	if expected.Over18 != actual.Over18 {
		return fmt.Errorf("post NSFW mismatch: expected %v, got %v", expected.Over18, actual.Over18)
	}

	return nil
}

// AssertCommentsEqual validates that two comments are equal (ignoring timestamps)
func AssertCommentsEqual(expected, actual types.Comment) error {
	if expected.ID != actual.ID {
		return fmt.Errorf("comment ID mismatch: expected %s, got %s", expected.ID, actual.ID)
	}

	if expected.Body != actual.Body {
		return fmt.Errorf("comment body mismatch: expected %s, got %s", expected.Body, actual.Body)
	}

	if expected.Author != actual.Author {
		return fmt.Errorf("comment author mismatch: expected %s, got %s", expected.Author, actual.Author)
	}

	if expected.Subreddit != actual.Subreddit {
		return fmt.Errorf("comment subreddit mismatch: expected %s, got %s", expected.Subreddit, actual.Subreddit)
	}

	if expected.Score != actual.Score {
		return fmt.Errorf("comment score mismatch: expected %d, got %d", expected.Score, actual.Score)
	}

	if expected.ParentID != actual.ParentID {
		return fmt.Errorf("comment parent ID mismatch: expected %s, got %s", expected.ParentID, actual.ParentID)
	}

	if expected.LinkID != actual.LinkID {
		return fmt.Errorf("comment link ID mismatch: expected %s, got %s", expected.LinkID, actual.LinkID)
	}

	return nil
}

// AssertPostListValid validates that a list of posts contains valid posts
func AssertPostListValid(posts []types.Post) error {
	for i, post := range posts {
		if err := AssertValidPost(post); err != nil {
			return fmt.Errorf("post at index %d is invalid: %v", i, err)
		}
	}

	// Check for duplicate IDs
	seenIDs := make(map[string]bool)
	for i, post := range posts {
		if seenIDs[post.ID] {
			return fmt.Errorf("duplicate post ID found at index %d: %s", i, post.ID)
		}
		seenIDs[post.ID] = true
	}

	return nil
}

// AssertCommentListValid validates that a list of comments contains valid comments
func AssertCommentListValid(comments []types.Comment) error {
	for i, comment := range comments {
		if err := AssertValidComment(comment); err != nil {
			return fmt.Errorf("comment at index %d is invalid: %v", i, err)
		}
	}

	// Check for duplicate IDs
	seenIDs := make(map[string]bool)
	for i, comment := range comments {
		if seenIDs[comment.ID] {
			return fmt.Errorf("duplicate comment ID found at index %d: %s", i, comment.ID)
		}
		seenIDs[comment.ID] = true
	}

	return nil
}

// AssertCommentThreadValid validates that a comment thread has valid structure
func AssertCommentThreadValid(comments []types.Comment) error {
	if err := AssertCommentListValid(comments); err != nil {
		return err
	}

	// Validate thread structure
	for i, comment := range comments {
		// Validate parent references
		if comment.ParentID != "" && !strings.HasPrefix(comment.ParentID, "t3_") {
			// Check parent exists if it's a comment (t1_)
			parentFound := false
			for _, potentialParent := range comments {
				if potentialParent.ID == comment.ParentID {
					parentFound = true
					break
				}
			}

			if !parentFound && strings.HasPrefix(comment.ParentID, "t1_") {
				return fmt.Errorf("comment at index %d references non-existent parent: %s", i, comment.ParentID)
			}
		}

		// Validate nested replies
		if len(comment.Replies) > 0 {
			// Convert []*Comment to []Comment for recursion
			var replyComments []types.Comment
			for _, reply := range comment.Replies {
				if reply != nil {
					replyComments = append(replyComments, *reply)
				}
			}
			if err := AssertCommentThreadValid(replyComments); err != nil {
				return fmt.Errorf("invalid replies for comment at index %d: %v", i, err)
			}
		}
	}

	return nil
}

// AssertScoreRange validates that a score is within expected range
func AssertScoreRange(score, min, max int) error {
	if score < min {
		return fmt.Errorf("score %d is below minimum %d", score, min)
	}
	if score > max {
		return fmt.Errorf("score %d is above maximum %d", score, max)
	}
	return nil
}

// AssertTimeRange validates that a time is within expected range
func AssertTimeRange(t, min, max time.Time) error {
	if t.Before(min) {
		return fmt.Errorf("time %v is before minimum %v", t, min)
	}
	if t.After(max) {
		return fmt.Errorf("time %v is after maximum %v", t, max)
	}
	return nil
}

// AssertStringNotEmpty validates that a string is not empty
func AssertStringNotEmpty(s, fieldName string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s is empty or whitespace", fieldName)
	}
	return nil
}

// AssertStringLength validates that a string length is within expected range
func AssertStringLength(s, fieldName string, min, max int) error {
	length := len(s)
	if length < min {
		return fmt.Errorf("%s length %d is below minimum %d", fieldName, length, min)
	}
	if length > max {
		return fmt.Errorf("%s length %d is above maximum %d", fieldName, length, max)
	}
	return nil
}

// AssertContains validates that a slice contains an element
func AssertContains(slice interface{}, element interface{}) error {
	sliceValue := reflect.ValueOf(slice)
	elementValue := reflect.ValueOf(element)

	if sliceValue.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %T", slice)
	}

	for i := 0; i < sliceValue.Len(); i++ {
		if reflect.DeepEqual(sliceValue.Index(i).Interface(), elementValue.Interface()) {
			return nil
		}
	}

	return fmt.Errorf("slice %v does not contain element %v", slice, element)
}

// AssertSliceLength validates that a slice has expected length
func AssertSliceLength(slice interface{}, expectedLength int) error {
	sliceValue := reflect.ValueOf(slice)
	if sliceValue.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %T", slice)
	}

	actualLength := sliceValue.Len()
	if actualLength != expectedLength {
		return fmt.Errorf("expected slice length %d, got %d", expectedLength, actualLength)
	}

	return nil
}

// AssertNotNil validates that a value is not nil
func AssertNotNil(value interface{}, fieldName string) error {
	if value == nil {
		return fmt.Errorf("%s is nil", fieldName)
	}

	// Check for nil pointers
	if reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil() {
		return fmt.Errorf("%s is nil pointer", fieldName)
	}

	return nil
}

func AssertValidUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username is empty")
	}

	if !validation.IsValidUsername(username) {
		return fmt.Errorf("username has invalid format: %s", username)
	}

	return nil
}

func AssertValidSubredditName(name string) error {
	if name == "" {
		return fmt.Errorf("subreddit name is empty")
	}

	if !validation.IsValidSubreddit(name) {
		return fmt.Errorf("subreddit name has invalid format: %s", name)
	}

	return nil
}

// AssertURL validates that a string is a valid URL
func AssertURL(url, fieldName string) error {
	if url == "" {
		return fmt.Errorf("%s is empty", fieldName)
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("%s does not start with http:// or https://: %s", fieldName, url)
	}

	return nil
}

// AssertPermalink validates that a string is a valid Reddit permalink
func AssertPermalink(permalink string) error {
	if permalink == "" {
		return fmt.Errorf("permalink is empty")
	}

	if !strings.HasPrefix(permalink, "/r/") {
		return fmt.Errorf("permalink does not start with /r/: %s", permalink)
	}

	return nil
}

// AssertPagination validates pagination data
func AssertPagination(before, after string, hasBefore, hasAfter bool) error {
	if hasBefore && before == "" {
		return fmt.Errorf("hasBefore is true but before is empty")
	}

	if !hasBefore && before != "" {
		return fmt.Errorf("hasBefore is false but before is not empty: %s", before)
	}

	if hasAfter && after == "" {
		return fmt.Errorf("hasAfter is true but after is empty")
	}

	if !hasAfter && after != "" {
		return fmt.Errorf("hasAfter is false but after is not empty: %s", after)
	}

	return nil
}

// AssertRateLimit validates rate limit data
func AssertRateLimit(remaining, used, reset int64) error {
	if remaining < 0 {
		return fmt.Errorf("rate limit remaining is negative: %d", remaining)
	}

	if used < 0 {
		return fmt.Errorf("rate limit used is negative: %d", used)
	}

	if reset <= 0 {
		return fmt.Errorf("rate limit reset time is not positive: %d", reset)
	}

	// Reset time should be in the future (Unix timestamp)
	resetTime := time.Unix(reset, 0)
	if resetTime.Before(time.Now().Add(-time.Minute)) {
		return fmt.Errorf("rate limit reset time is too far in the past: %v", resetTime)
	}

	return nil
}

// AssertErrorType validates that an error is of expected type
func AssertErrorType(err error, expectedType string) error {
	if err == nil {
		return fmt.Errorf("expected error of type %s, got nil", expectedType)
	}

	actualType := reflect.TypeOf(err).String()
	if !strings.Contains(actualType, expectedType) {
		return fmt.Errorf("expected error type containing %s, got %s", expectedType, actualType)
	}

	return nil
}

// AssertErrorMessage validates that an error message contains expected text
func AssertErrorMessage(err error, expectedMessage string) error {
	if err == nil {
		return fmt.Errorf("expected error containing message '%s', got nil", expectedMessage)
	}

	if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(expectedMessage)) {
		return fmt.Errorf("expected error message containing '%s', got '%s'", expectedMessage, err.Error())
	}

	return nil
}
