package validation_test

import (
	"fmt"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/validation"
)

// Example of validating a Reddit post ID
func ExampleIsValidBase36() {
	valid := validation.IsValidBase36("abc123")
	invalid := validation.IsValidBase36("ABC123") // uppercase not allowed

	fmt.Printf("Valid: %v\n", valid)
	fmt.Printf("Invalid: %v\n", invalid)
	// Output:
	// Valid: true
	// Invalid: false
}

// Example of validating a subreddit name
func ExampleIsValidSubreddit() {
	valid := validation.IsValidSubreddit("golang")
	tooShort := validation.IsValidSubreddit("go")                    // must be 3+ chars
	tooLong := validation.IsValidSubreddit("a1234567890123456789xy") // max 21 chars

	fmt.Printf("Valid: %v\n", valid)
	fmt.Printf("Too short: %v\n", tooShort)
	fmt.Printf("Too long: %v\n", tooLong)
	// Output:
	// Valid: true
	// Too short: false
	// Too long: false
}

// Example of validating a Reddit fullname (type prefix + ID)
func ExampleIsValidFullname() {
	post := validation.IsValidFullname("t3_abc123")
	comment := validation.IsValidFullname("t1_def456")
	invalid := validation.IsValidFullname("invalid")

	fmt.Printf("Post fullname: %v\n", post)
	fmt.Printf("Comment fullname: %v\n", comment)
	fmt.Printf("Invalid: %v\n", invalid)
	// Output:
	// Post fullname: true
	// Comment fullname: true
	// Invalid: false
}

// Example of validating a Reddit username
func ExampleIsValidUsername() {
	valid := validation.IsValidUsername("john_doe")
	withHyphen := validation.IsValidUsername("john-doe")
	tooShort := validation.IsValidUsername("ab")

	fmt.Printf("Valid: %v\n", valid)
	fmt.Printf("With hyphen: %v\n", withHyphen)
	fmt.Printf("Too short: %v\n", tooShort)
	// Output:
	// Valid: true
	// With hyphen: true
	// Too short: false
}

// Example of validating a Reddit permalink
func ExampleIsValidPermalink() {
	postLink := validation.IsValidPermalink("/r/golang/comments/abc123/test_post/")
	commentLink := validation.IsValidPermalink("/r/golang/comments/abc123/test_post/def456/")
	invalid := validation.IsValidPermalink("/invalid/link")

	fmt.Printf("Post permalink: %v\n", postLink)
	fmt.Printf("Comment permalink: %v\n", commentLink)
	fmt.Printf("Invalid: %v\n", invalid)
	// Output:
	// Post permalink: true
	// Comment permalink: true
	// Invalid: false
}

// Example of validating a complete Post struct
func ExampleValidatePost() {
	now := float64(time.Now().Unix())

	// Valid post
	validPost := &types.Post{
		ThingData:   types.ThingData{ID: "abc123", Name: "t3_abc123"},
		Votable:     types.Votable{Score: 100, Ups: 100, Downs: 0},
		Created:     types.Created{Created: now, CreatedUTC: now},
		Title:       "Test Post",
		Author:      "testuser",
		Subreddit:   "golang",
		SubredditID: "t5_2rcjn",
		Permalink:   "/r/golang/comments/abc123/test_post/",
		URL:         "https://reddit.com/r/golang/comments/abc123/test_post/",
		UpvoteRatio: 0.95,
		NumComments: 10,
	}

	err := validation.ValidatePost(validPost)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("Post is valid")
	}
	// Output: Post is valid
}

// Example of validating a Comment struct
func ExampleValidateComment() {
	now := float64(time.Now().Unix())

	// Valid comment
	validComment := &types.Comment{
		ThingData:   types.ThingData{ID: "def456", Name: "t1_def456"},
		Votable:     types.Votable{Score: 50, Ups: 50, Downs: 0},
		Created:     types.Created{Created: now, CreatedUTC: now},
		Body:        "This is a test comment",
		Author:      "testuser",
		Subreddit:   "golang",
		SubredditID: "t5_2rcjn",
		ParentID:    "t3_abc123",
		LinkID:      "t3_abc123",
	}

	err := validation.ValidateComment(validComment)
	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("Comment is valid")
	}
	// Output: Comment is valid
}

// mockClock implements the Clock interface for testing
type mockClock struct {
	now time.Time
}

func (m mockClock) Now() time.Time {
	return m.now
}

// Example of using the Clock interface for deterministic testing
func ExampleSetClock() {
	// Create a mock clock for testing
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	mock := mockClock{now: fixedTime}
	validation.SetClock(mock)

	// Now ValidateCreated will use the fixed time
	pastTime := float64(fixedTime.Add(-1 * time.Hour).Unix())
	created := &types.Created{Created: pastTime, CreatedUTC: pastTime}

	err := validation.ValidateCreated(created)

	// Reset to real time after testing
	validation.ResetClock()

	if err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("Created timestamp is valid")
	}
	// Output: Created timestamp is valid
}
