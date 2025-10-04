package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestParseThing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		thing        *types.Thing
		expectError  bool
		expectedType string
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "Listing kind",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{"after":"t3_after123","before":null,"children":[]}`),
			},
			expectError:  false,
			expectedType: "*types.ListingData",
		},
		{
			name: "t1 comment",
			thing: &types.Thing{
				ThingData: types.ThingData{
					ID:   "comment123",
					Name: "t1_comment123",
				},
				Kind: "t1",
				Data: json.RawMessage(`{"id":"comment123","name":"t1_comment123","author":"testuser","body":"test comment","score":10,"ups":10,"downs":0,"created":1234567890,"created_utc":1234567890,"parent_id":"t3_post123","link_id":"t3_post123","subreddit":"test","replies":""}`),
			},
			expectError:  false,
			expectedType: "*types.Comment",
		},
		{
			name: "t2 account",
			thing: &types.Thing{
				Kind: "t2",
				Data: json.RawMessage(`{"id":"user123","name":"t2_user123","link_karma":100,"comment_karma":200,"created":1234567890,"created_utc":1234567890}`),
			},
			expectError:  false,
			expectedType: "*types.AccountData",
		},
		{
			name: "t3 link",
			thing: &types.Thing{
				ThingData: types.ThingData{
					ID:   "post123",
					Name: "t3_post123",
				},
				Kind: "t3",
				Data: json.RawMessage(`{"id":"post123","name":"t3_post123","author":"testuser","title":"Test Post","url":"http://example.com","permalink":"/r/test/comments/post123/test_post/","subreddit":"test","score":100,"ups":100,"downs":0,"created":1234567890,"created_utc":1234567890,"upvote_ratio":0.95,"num_comments":5}`),
			},
			expectError:  false,
			expectedType: "*types.Post",
		},
		{
			name: "t4 message",
			thing: &types.Thing{
				Kind: "t4",
				Data: json.RawMessage(`{"id":"msg123","name":"t4_msg123","author":"testuser","body":"test message","subject":"Test Subject","created":1234567890,"created_utc":1234567890}`),
			},
			expectError:  false,
			expectedType: "*types.MessageData",
		},
		{
			name: "t5 subreddit",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{"id":"2qh1i","name":"t5_2qh1i","display_name":"golang","title":"Go Programming","subscribers":100000,"created":1234567890,"created_utc":1234567890}`),
			},
			expectError:  false,
			expectedType: "*types.SubredditData",
		},
		{
			name: "more kind",
			thing: &types.Thing{
				Kind: "more",
				Data: json.RawMessage(`{"children":["id1","id2","id3"],"id":"more123"}`),
			},
			expectError:  false,
			expectedType: "*types.MoreData",
		},
		{
			name: "unknown kind",
			thing: &types.Thing{
				Kind: "unknown",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseThing(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseListing(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid listing",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{"after":"t3_after123","before":"t3_before456","modhash":"modhash789","children":[]}`),
			},
			expectError: false,
		},
		{
			name: "listing with children",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after":"t3_after123",
					"before":null,
					"children":[
						{"kind":"t3","id":"post1","data":{}},
						{"kind":"t3","id":"post2","data":{}}
					]
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseListing(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParsePost(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid post",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id":"post123",
					"name":"t3_post123",
					"author":"testuser",
					"title":"Test Post",
					"url":"http://example.com",
					"permalink":"/r/golang/comments/post123/test_post/",
					"score":100,
					"ups":100,
					"downs":0,
					"num_comments":50,
					"subreddit":"golang",
					"created":1234567890,
					"created_utc":1234567890,
					"upvote_ratio":0.95,
					"edited":false,
					"is_self":false,
					"over_18":false,
					"saved":false
				}`),
			},
			expectError: false,
		},
		{
			name: "self post",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id":"selfpost456",
					"name":"t3_selfpost456",
					"author":"testuser",
					"title":"Self Post Title",
					"url":"http://example.com",
					"permalink":"/r/AskReddit/comments/selfpost456/self_post_title/",
					"selftext":"This is the self text",
					"is_self":true,
					"score":50,
					"ups":50,
					"downs":0,
					"subreddit":"AskReddit",
					"created":1234567890,
					"created_utc":1234567890,
					"upvote_ratio":0.85,
					"num_comments":10,
					"edited":1234567900
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePost(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseComment(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid comment without replies",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id":"comment123",
					"name":"t1_comment123",
					"author":"testuser",
					"body":"This is a test comment",
					"body_html":"<p>This is a test comment</p>",
					"score":10,
					"ups":10,
					"downs":0,
					"created":1234567890,
					"created_utc":1234567890,
					"edited":false,
					"replies":"",
					"parent_id":"t3_abc123",
					"link_id":"t3_abc123",
					"subreddit":"golang",
					"score_hidden":false,
					"saved":false
				}`),
			},
			expectError: false,
		},
		{
			name: "comment with replies",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id":"parentcomment",
					"name":"t1_parentcomment",
					"author":"testuser",
					"body":"Parent comment",
					"score":20,
					"ups":20,
					"downs":0,
					"created":1234567890,
					"created_utc":1234567890,
					"parent_id":"t3_post123",
					"link_id":"t3_post123",
					"subreddit":"golang",
					"replies":{
						"kind":"Listing",
						"data":{
							"children":[
								{"kind":"t1","id":"reply1","data":{"author":"user2","body":"Reply"}}
							]
						}
					}
				}`),
			},
			expectError: false,
		},
		{
			name: "edited comment",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id":"editedcomment",
					"name":"t1_editedcomment",
					"author":"testuser",
					"body":"Edited comment",
					"score":5,
					"ups":5,
					"downs":0,
					"created":1234567890,
					"created_utc":1234567890,
					"edited":1234567900,
					"replies":"",
					"parent_id":"t1_parent",
					"link_id":"t3_post123",
					"subreddit":"golang"
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseComment(context.Background(), tt.thing, &parseContext{
				seenIDs: make(map[string]bool),
			})

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseSubreddit(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid subreddit",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{
					"id":"2qh1i",
					"name":"t5_2qh1i",
					"display_name":"golang",
					"title":"Go Programming Language",
					"subscribers":150000,
					"description":"A subreddit for Go programmers",
					"public_description":"Public description",
					"url":"/r/golang",
					"over18":false,
					"subreddit_type":"public",
					"created":1234567890,
					"created_utc":1234567890
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSubreddit(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseAccount(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid account",
			thing: &types.Thing{
				Kind: "t2",
				Data: json.RawMessage(`{
					"name":"t2_user123",
					"id":"user123",
					"link_karma":1000,
					"comment_karma":5000,
					"created":1234567890,
					"created_utc":1234567890,
					"is_gold":true,
					"is_mod":false,
					"over_18":false
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "t2",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseAccount(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid message",
			thing: &types.Thing{
				Kind: "t4",
				Data: json.RawMessage(`{
					"id":"msg123",
					"name":"t4_msg123",
					"author":"sender",
					"body":"Message body",
					"body_html":"<p>Message body</p>",
					"subject":"Test Subject",
					"created":1234567890,
					"created_utc":1234567890,
					"new":true,
					"was_comment":false
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "t4",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseMessage(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestParseMore(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
	}{
		{
			name:        "nil thing",
			thing:       nil,
			expectError: true,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
		},
		{
			name: "valid more",
			thing: &types.Thing{
				Kind: "more",
				Data: json.RawMessage(`{
					"children":["id1","id2","id3","id4"],
					"id":"more123",
					"name":"t1_more123"
				}`),
			},
			expectError: false,
		},
		{
			name: "empty children",
			thing: &types.Thing{
				Kind: "more",
				Data: json.RawMessage(`{
					"children":[],
					"id":"more456"
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid JSON",
			thing: &types.Thing{
				Kind: "more",
				Data: json.RawMessage(`{invalid json}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseMore(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestExtractPosts(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		expectCount int
	}{
		{
			name:        "nil listing",
			thing:       nil,
			expectError: true,
			expectCount: 0,
		},
		{
			name: "empty listing",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{"children":[]}`),
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name: "listing with posts",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after":"t3_after123",
					"children":[
						{
							"kind":"t3",
							"id":"post1",
							"name":"t3_post1",
							"data":{
								"id":"post1",
								"name":"t3_post1",
								"author":"user1",
								"title":"First Post",
								"url":"http://example.com/1",
								"permalink":"/r/test/comments/post1/first_post/",
								"subreddit":"test",
								"score":100,
								"ups":100,
								"downs":0,
								"created":1234567890,
								"created_utc":1234567890,
								"upvote_ratio":0.95,
								"num_comments":10
							}
						},
						{
							"kind":"t3",
							"id":"post2",
							"name":"t3_post2",
							"data":{
								"id":"post2",
								"name":"t3_post2",
								"author":"user2",
								"title":"Second Post",
								"url":"http://example.com/2",
								"permalink":"/r/test/comments/post2/second_post/",
								"subreddit":"test",
								"score":200,
								"ups":200,
								"downs":0,
								"created":1234567890,
								"created_utc":1234567890,
								"upvote_ratio":0.90,
								"num_comments":5
							}
						}
					]
				}`),
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name: "listing with mixed content",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"children":[
						{
							"kind":"t3",
							"id":"post1",
							"name":"t3_post1",
							"data":{
								"id":"post1",
								"name":"t3_post1",
								"author":"user1",
								"title":"Post",
								"url":"http://example.com",
								"permalink":"/r/test/comments/post1/post/",
								"subreddit":"test",
								"score":50,
								"ups":50,
								"downs":0,
								"created":1234567890,
								"created_utc":1234567890,
								"upvote_ratio":0.85,
								"num_comments":3
							}
						},
						{
							"kind":"t1",
							"id":"comment1",
							"data":{
								"author":"user2",
								"body":"Comment"
							}
						},
						{
							"kind":"more",
							"id":"more1",
							"data":{
								"children":["id1","id2"]
							}
						}
					]
				}`),
			},
			expectError: false,
			expectCount: 1, // Only the t3 post should be extracted
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			posts, err := parser.ExtractPosts(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(posts) != tt.expectCount {
					t.Errorf("expected %d posts, got %d", tt.expectCount, len(posts))
				}
			}
		})
	}
}

func TestExtractComments(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name           string
		thing          *types.Thing
		expectError    bool
		expectComments int
		expectMore     int
	}{
		{
			name: "single comment without replies",
			thing: &types.Thing{
				ThingData: types.ThingData{
					ID:   "comment1",
					Name: "t1_comment1",
				},
				Kind: "t1",
				Data: json.RawMessage(`{
					"id":"comment1",
					"name":"t1_comment1",
					"author":"user1",
					"body":"Test comment",
					"score":10,
					"ups":10,
					"downs":0,
					"created":1234567890,
					"created_utc":1234567890,
					"parent_id":"t3_post1",
					"link_id":"t3_post1",
					"subreddit":"test",
					"replies":""
				}`),
			},
			expectError:    false,
			expectComments: 1,
			expectMore:     0,
		},
		{
			name: "single comment with replies",
			thing: &types.Thing{
				ThingData: types.ThingData{
					ID:   "comment1",
					Name: "t1_comment1",
				},
				Kind: "t1",
				Data: json.RawMessage(`{
					"id":"comment1",
					"name":"t1_comment1",
					"author":"user1",
					"body":"Parent comment",
					"score":10,
					"ups":10,
					"downs":0,
					"created":1234567890,
					"created_utc":1234567890,
					"parent_id":"t3_post1",
					"link_id":"t3_post1",
					"subreddit":"test",
					"replies":{
						"kind":"Listing",
						"data":{
							"children":[
								{
									"kind":"t1",
									"id":"reply1",
									"name":"t1_reply1",
									"data":{
										"id":"reply1",
										"name":"t1_reply1",
										"author":"user2",
										"body":"Reply",
										"score":5,
										"ups":5,
										"downs":0,
										"created":1234567895,
										"created_utc":1234567895,
										"parent_id":"t1_comment1",
										"link_id":"t3_post1",
										"subreddit":"test",
										"replies":""
									}
								}
							]
						}
					}
				}`),
			},
			expectError:    false,
			expectComments: 1, // Parent only (reply is in Replies field)
			expectMore:     0,
		},
		{
			name: "listing with comments and more",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"children":[
						{
							"kind":"t1",
							"id":"comment1",
							"name":"t1_comment1",
							"data":{
								"id":"comment1",
								"name":"t1_comment1",
								"author":"user1",
								"body":"First comment",
								"score":10,
								"ups":10,
								"downs":0,
								"created":1234567890,
								"created_utc":1234567890,
								"parent_id":"t3_post1",
								"link_id":"t3_post1",
								"subreddit":"test",
								"replies":""
							}
						},
						{
							"kind":"t1",
							"id":"comment2",
							"name":"t1_comment2",
							"data":{
								"id":"comment2",
								"name":"t1_comment2",
								"author":"user2",
								"body":"Second comment",
								"score":5,
								"ups":5,
								"downs":0,
								"created":1234567895,
								"created_utc":1234567895,
								"parent_id":"t3_post1",
								"link_id":"t3_post1",
								"subreddit":"test",
								"replies":""
							}
						},
						{
							"kind":"more",
							"id":"more1",
							"data":{
								"id":"more1",
								"name":"t1_more1",
								"children":["id1","id2","id3"]
							}
						}
					]
				}`),
			},
			expectError:    false,
			expectComments: 2,
			expectMore:     3,
		},
		{
			name: "nested comments",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"children":[
						{
							"kind":"t1",
							"id":"comment1",
							"name":"t1_comment1",
							"data":{
								"id":"comment1",
								"name":"t1_comment1",
								"author":"user1",
								"body":"Parent",
								"score":10,
								"ups":10,
								"downs":0,
								"created":1234567890,
								"created_utc":1234567890,
								"parent_id":"t3_post1",
								"link_id":"t3_post1",
								"subreddit":"test",
								"replies":{
									"kind":"Listing",
									"data":{
										"children":[
											{
												"kind":"t1",
												"id":"reply1",
												"name":"t1_reply1",
												"data":{
													"id":"reply1",
													"name":"t1_reply1",
													"author":"user2",
													"body":"Child",
													"score":5,
													"ups":5,
													"downs":0,
													"created":1234567895,
													"created_utc":1234567895,
													"parent_id":"t1_comment1",
													"link_id":"t3_post1",
													"subreddit":"test",
													"replies":{
														"kind":"Listing",
														"data":{
															"children":[
																{
																	"kind":"t1",
																	"id":"reply2",
																	"name":"t1_reply2",
																	"data":{
																		"id":"reply2",
																		"name":"t1_reply2",
																		"author":"user3",
																		"body":"Grandchild",
																		"score":3,
																		"ups":3,
																		"downs":0,
																		"created":1234567900,
																		"created_utc":1234567900,
																		"parent_id":"t1_reply1",
																		"link_id":"t3_post1",
																		"subreddit":"test",
																		"replies":""
																	}
																}
															]
														}
													}
												}
											}
										]
									}
								}
							}
						}
					]
				}`),
			},
			expectError:    false,
			expectComments: 1, // Parent only (tree structure maintained)
			expectMore:     0,
		},
		{
			name: "wrong kind",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{}`),
			},
			expectError:    true,
			expectComments: 0,
			expectMore:     0,
		},
		{
			name: "empty listing",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{"children":[]}`),
			},
			expectError:    false,
			expectComments: 0,
			expectMore:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comments, moreIDs, err := parser.ExtractComments(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(comments) != tt.expectComments {
					t.Errorf("expected %d comments, got %d", tt.expectComments, len(comments))
				}
				if len(moreIDs) != tt.expectMore {
					t.Errorf("expected %d more IDs, got %d", tt.expectMore, len(moreIDs))
				}
			}
		})
	}
}

func TestExtractPostAndComments(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name           string
		response       []*types.Thing
		expectError    bool
		expectPost     bool
		expectComments int
		expectMore     int
	}{
		{
			name:        "nil response",
			response:    nil,
			expectError: true,
		},
		{
			name:        "empty response",
			response:    []*types.Thing{},
			expectError: true,
		},
		{
			name: "single element response with comments",
			response: []*types.Thing{
				{
					Kind: "Listing",
					Data: json.RawMessage(`{"children":[]}`),
				},
			},
			expectError: false,
			// Should return nil post with empty comments
		},
		{
			name: "valid post and comments",
			response: []*types.Thing{
				{
					Kind: "Listing",
					Data: json.RawMessage(`{
						"children":[
							{
								"kind":"t3",
								"id":"post1",
								"name":"t3_post1",
								"data":{
									"id":"post1",
									"name":"t3_post1",
									"author":"postauthor",
									"title":"Test Post",
									"url":"http://example.com",
									"permalink":"/r/test/comments/post1/test_post/",
									"subreddit":"test",
									"score":100,
									"ups":100,
									"downs":0,
									"created":1234567890,
									"created_utc":1234567890,
									"upvote_ratio":0.95,
									"num_comments":2
								}
							}
						]
					}`),
				},
				{
					Kind: "Listing",
					Data: json.RawMessage(`{
						"children":[
							{
								"kind":"t1",
								"id":"comment1",
								"name":"t1_comment1",
								"data":{
									"id":"comment1",
									"name":"t1_comment1",
									"author":"commenter1",
									"body":"First comment",
									"score":10,
									"ups":10,
									"downs":0,
									"created":1234567890,
									"created_utc":1234567890,
									"parent_id":"t3_post1",
									"link_id":"t3_post1",
									"subreddit":"test",
									"replies":""
								}
							},
							{
								"kind":"t1",
								"id":"comment2",
								"name":"t1_comment2",
								"data":{
									"id":"comment2",
									"name":"t1_comment2",
									"author":"commenter2",
									"body":"Second comment",
									"score":5,
									"ups":5,
									"downs":0,
									"created":1234567890,
									"created_utc":1234567890,
									"parent_id":"t3_post1",
									"link_id":"t3_post1",
									"subreddit":"test",
									"replies":{
										"kind":"Listing",
										"data":{
											"children":[
												{
													"kind":"t1",
													"id":"reply1",
													"name":"t1_reply1",
													"data":{
														"id":"reply1",
														"name":"t1_reply1",
														"author":"replier",
														"body":"Reply",
														"score":1,
														"ups":1,
														"downs":0,
														"created":1234567890,
														"created_utc":1234567890,
														"parent_id":"t1_comment2",
														"link_id":"t3_post1",
														"subreddit":"test",
														"replies":""
													}
												}
											]
										}
									}
								}
							},
							{
								"kind":"more",
								"id":"more1",
								"name":"t2_more1",
								"data":{
									"id":"more1",
									"name":"t2_more1",
									"children":["id1","id2"]
								}
							}
						]
					}`),
				},
			},
			expectError:    false,
			expectPost:     true,
			expectComments: 2, // 2 top-level comments (reply is in Replies field)
			expectMore:     2,
		},
		{
			name: "no post in first listing",
			response: []*types.Thing{
				{
					Kind: "Listing",
					Data: json.RawMessage(`{"children":[]}`),
				},
				{
					Kind: "Listing",
					Data: json.RawMessage(`{"children":[]}`),
				},
			},
			expectError:    false, // Changed: We now handle missing posts gracefully
			expectPost:     false,
			expectComments: 0,
			expectMore:     0,
		},
		{
			name: "invalid second listing",
			response: []*types.Thing{
				{
					Kind: "Listing",
					Data: json.RawMessage(`{
						"children":[
							{
								"kind":"t3",
								"id":"post1",
								"name":"t3_post1",
								"data":{
									"id":"post1",
									"name":"t3_post1",
									"author":"postauthor",
									"title":"Test Post",
									"url":"http://example.com",
									"permalink":"/r/test/comments/post1/test_post/",
									"subreddit":"test",
									"score":100,
									"ups":100,
									"downs":0,
									"created":1234567890,
									"created_utc":1234567890,
									"upvote_ratio":0.95,
									"num_comments":0
								}
							}
						]
					}`),
				},
				{
					Kind: "t3", // Wrong kind, should be Listing
					Data: json.RawMessage(`{}`),
				},
			},
			expectError:    false, // Post extraction succeeds, comment extraction fails but error contains post
			expectPost:     true,
			expectComments: 0,
			expectMore:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ExtractPostAndComments(context.Background(), tt.response)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				// For the "invalid second listing" case, we expect an error about comment extraction
				// but the post should still be returned
				if tt.name == "invalid second listing" {
					if err == nil {
						t.Errorf("expected comment extraction error but got none")
					}
				} else if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if result == nil {
					t.Fatal("expected result but got nil")
				}

				if tt.expectPost {
					if result.Post == nil {
						t.Errorf("expected post but got nil")
					}
				} else {
					if result.Post != nil {
						t.Errorf("expected no post but got one")
					}
				}

				if len(result.Comments) != tt.expectComments {
					t.Errorf("expected %d comments, got %d", tt.expectComments, len(result.Comments))
				}
				if len(result.MoreIDs) != tt.expectMore {
					t.Errorf("expected %d more IDs, got %d", tt.expectMore, len(result.MoreIDs))
				}
			}
		})
	}
}

func TestExtractPostAndComments_EdgeCases(t *testing.T) {
	parser := NewParser()

	t.Run("single listing with post only", func(t *testing.T) {
		// Single listing format tries ExtractComments first, which succeeds with 0 comments
		// when children are posts (t3). This is expected behavior - single listing is assumed
		// to be comments, not posts.
		response := []*types.Thing{
			{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"children":[
						{
							"kind":"t3",
							"id":"post1",
							"name":"t3_post1",
							"data":{
								"author":"author",
								"title":"Post",
								"url":"http://example.com"
							}
						}
					]
				}`),
			},
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// ExtractComments succeeds but finds no t1 children, returns nil post
		if result.Post != nil {
			t.Errorf("expected no post for single listing, got %v", result.Post)
		}
		if len(result.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(result.Comments))
		}
		if len(result.MoreIDs) != 0 {
			t.Errorf("expected 0 more IDs, got %d", len(result.MoreIDs))
		}
	})

	t.Run("first listing fails to parse, second has comments", func(t *testing.T) {
		response := []*types.Thing{
			{
				Kind: "t3", // Wrong kind, should be Listing
				Data: json.RawMessage(`{}`),
			},
			{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"children":[
						{
							"kind":"t1",
							"id":"comment1",
							"name":"t1_comment1",
							"data":{
								"id":"comment1",
								"name":"t1_comment1",
								"author":"commenter",
								"body":"Comment",
								"score":10,
								"ups":10,
								"downs":0,
								"created":1234567890,
								"created_utc":1234567890,
								"parent_id":"t3_post1",
								"link_id":"t3_post1",
								"subreddit":"test",
								"replies":""
							}
						}
					]
				}`),
			},
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Post != nil {
			t.Errorf("expected no post but got one")
		}
		if len(result.Comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(result.Comments))
		}
		if len(result.MoreIDs) != 0 {
			t.Errorf("expected 0 more IDs, got %d", len(result.MoreIDs))
		}
	})

	t.Run("both post and comment extraction fail", func(t *testing.T) {
		response := []*types.Thing{
			{
				Kind: "t1", // Wrong kind for posts
				Data: json.RawMessage(`{}`),
			},
			{
				Kind: "t3", // Wrong kind for comments
				Data: json.RawMessage(`{}`),
			},
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		if err == nil {
			t.Fatal("expected error but got none")
		}
		if result != nil {
			t.Errorf("expected nil result but got %v", result)
		}
	})

	t.Run("single listing with invalid data", func(t *testing.T) {
		response := []*types.Thing{
			{
				Kind: "t3", // Wrong kind, not Listing or t1
				Data: json.RawMessage(`{}`),
			},
		}

		result, err := parser.ExtractPostAndComments(context.Background(), response)
		if err == nil {
			t.Fatal("expected error but got none")
		}
		if result != nil {
			t.Error("expected nil result on error")
		}
	})
}

// Test edge cases for Edited type unmarshaling
func TestEditedUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		isEdited  bool
		timestamp float64
	}{
		{
			name:      "false boolean",
			input:     `false`,
			expectErr: false,
			isEdited:  false,
			timestamp: 0,
		},
		{
			name:      "true boolean",
			input:     `true`,
			expectErr: false,
			isEdited:  true,
			timestamp: 0,
		},
		{
			name:      "timestamp",
			input:     `1234567890.5`,
			expectErr: false,
			isEdited:  true,
			timestamp: 1234567890.5,
		},
		{
			name:      "null",
			input:     `null`,
			expectErr: false,
			isEdited:  false, // null means not edited
			timestamp: 0,
		},
		{
			name:      "invalid",
			input:     `"string"`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e types.Edited
			err := e.UnmarshalJSON([]byte(tt.input))

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if e.IsEdited != tt.isEdited {
					t.Errorf("expected IsEdited=%v, got %v", tt.isEdited, e.IsEdited)
				}
				if e.Timestamp != tt.timestamp {
					t.Errorf("expected Timestamp=%v, got %v", tt.timestamp, e.Timestamp)
				}
			}
		})
	}
}

// TestCommentTreeStructure verifies that comments maintain a proper tree structure
// where each comment's Replies field contains only direct children, not all descendants.
func TestCommentTreeStructure(t *testing.T) {
	parser := NewParser()

	// Create a complex tree: parent -> child -> grandchild
	thing := &types.Thing{
		Kind: "t1",
		Data: json.RawMessage(`{
			"id": "parent",
			"name": "t1_parent",
			"author": "user1",
			"body": "Parent comment",
			"score": 100,
			"ups": 100,
			"downs": 0,
			"created": 1234567890,
			"created_utc": 1234567890,
			"parent_id": "t3_post1",
			"link_id": "t3_post1",
			"subreddit": "test",
			"replies": {
				"kind": "Listing",
				"data": {
					"children": [
						{
							"kind": "t1",
							"id": "child",
							"name": "t1_child",
							"data": {
								"id": "child",
								"name": "t1_child",
								"author": "user2",
								"body": "Child comment",
								"score": 50,
								"ups": 50,
								"downs": 0,
								"created": 1234567890,
								"created_utc": 1234567890,
								"parent_id": "t1_parent",
								"link_id": "t3_post1",
								"subreddit": "test",
								"replies": {
									"kind": "Listing",
									"data": {
										"children": [
											{
												"kind": "t1",
												"id": "grandchild",
												"name": "t1_grandchild",
												"data": {
													"id": "grandchild",
													"name": "t1_grandchild",
													"author": "user3",
													"body": "Grandchild comment",
													"score": 10,
													"ups": 10,
													"downs": 0,
													"created": 1234567890,
													"created_utc": 1234567890,
													"parent_id": "t1_child",
													"link_id": "t3_post1",
													"subreddit": "test",
													"replies": ""
												}
											}
										]
									}
								}
							}
						},
						{
							"kind": "t1",
							"id": "child2",
							"name": "t1_child2",
							"data": {
								"id": "child2",
								"name": "t1_child2",
								"author": "user4",
								"body": "Second child",
								"score": 25,
								"ups": 25,
								"downs": 0,
								"created": 1234567890,
								"created_utc": 1234567890,
								"parent_id": "t1_parent",
								"link_id": "t3_post1",
								"subreddit": "test",
								"replies": ""
							}
						}
					]
				}
			}
		}`),
	}

	parent, err := parser.ParseComment(context.Background(), thing, &parseContext{
		seenIDs: make(map[string]bool),
	})
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}

	// Verify parent has exactly 2 direct children (not 3 with grandchild)
	if len(parent.Replies) != 2 {
		t.Errorf("Parent should have 2 direct children, got %d", len(parent.Replies))
	}

	// Verify first child exists
	if parent.Replies[0].Author != "user2" {
		t.Errorf("First child author = %q, want %q", parent.Replies[0].Author, "user2")
	}

	// Verify first child has exactly 1 child (grandchild)
	if len(parent.Replies[0].Replies) != 1 {
		t.Errorf("First child should have 1 reply, got %d", len(parent.Replies[0].Replies))
	}

	// Verify grandchild exists at correct level
	if parent.Replies[0].Replies[0].Author != "user3" {
		t.Errorf("Grandchild author = %q, want %q", parent.Replies[0].Replies[0].Author, "user3")
	}

	// Verify grandchild has no replies
	if len(parent.Replies[0].Replies[0].Replies) != 0 {
		t.Errorf("Grandchild should have 0 replies, got %d", len(parent.Replies[0].Replies[0].Replies))
	}

	// Verify second child exists and has no replies
	if parent.Replies[1].Author != "user4" {
		t.Errorf("Second child author = %q, want %q", parent.Replies[1].Author, "user4")
	}
	if len(parent.Replies[1].Replies) != 0 {
		t.Errorf("Second child should have 0 replies, got %d", len(parent.Replies[1].Replies))
	}
}

// TestParsePost_MaliciousData tests that malicious or malformed post data is rejected
func TestParsePost_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "uppercase post ID",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "ABC123",
					"name": "t3_ABC123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "ID has invalid format",
		},
		{
			name: "SQL injection in ID",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc'; DROP TABLE posts--",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "ID has invalid format",
		},
		{
			name: "invalid subreddit name - too short",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/ab/comments/abc123/test_post/",
					"subreddit": "ab",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "Subreddit has invalid format",
		},
		{
			name: "invalid subreddit name - special chars",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test$/comments/abc123/test_post/",
					"subreddit": "test$",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "Subreddit has invalid format",
		},
		{
			name: "invalid permalink format",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/invalid/permalink/format",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "Permalink has invalid format",
		},
		{
			name: "negative NumComments",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": -5
				}`),
			},
			expectError: true,
			errorText:   "NumComments cannot be negative",
		},
		{
			name: "UpvoteRatio out of range - too high",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 1.5,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "UpvoteRatio must be between 0 and 1",
		},
		{
			name: "UpvoteRatio out of range - negative",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": -0.5,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "UpvoteRatio must be between 0 and 1",
		},
		{
			name: "future timestamp",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(fmt.Sprintf(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": %d,
					"created_utc": %d,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`, time.Now().Add(48*time.Hour).Unix(), time.Now().Add(48*time.Hour).Unix())),
			},
			expectError: true,
			errorText:   "CreatedUTC is in the future",
		},
		{
			name: "timestamp before Reddit existed",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "t3_abc123",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 946684800,
					"created_utc": 946684800,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "CreatedUTC is before Reddit existed",
		},
		{
			name: "invalid fullname format",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"id": "abc123",
					"name": "INVALID_FULLNAME",
					"author": "testuser",
					"title": "Test Post",
					"url": "http://example.com",
					"permalink": "/r/test/comments/abc123/test_post/",
					"subreddit": "test",
					"score": 100,
					"ups": 100,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"upvote_ratio": 0.95,
					"num_comments": 10
				}`),
			},
			expectError: true,
			errorText:   "Name has invalid fullname format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePost(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorText != "" && !containsText(err.Error(), tt.errorText) {
					t.Errorf("expected error containing %q, got %q", tt.errorText, err.Error())
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestParseComment_MaliciousData tests that malicious or malformed comment data is rejected
func TestParseComment_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "uppercase comment ID",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id": "DEF456",
					"name": "t1_DEF456",
					"author": "testuser",
					"body": "Test comment",
					"parent_id": "t3_abc123",
					"link_id": "t3_abc123",
					"subreddit": "test",
					"score": 10,
					"ups": 10,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"replies": ""
				}`),
			},
			expectError: true,
			errorText:   "ID has invalid format",
		},
		{
			name: "invalid ParentID format",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id": "def456",
					"name": "t1_def456",
					"author": "testuser",
					"body": "Test comment",
					"parent_id": "INVALID_PARENT",
					"link_id": "t3_abc123",
					"subreddit": "test",
					"score": 10,
					"ups": 10,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"replies": ""
				}`),
			},
			expectError: true,
			errorText:   "ParentID has invalid fullname format",
		},
		{
			name: "invalid LinkID format",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id": "def456",
					"name": "t1_def456",
					"author": "testuser",
					"body": "Test comment",
					"parent_id": "t3_abc123",
					"link_id": "invalid_link",
					"subreddit": "test",
					"score": 10,
					"ups": 10,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"replies": ""
				}`),
			},
			expectError: true,
			errorText:   "LinkID has invalid fullname format",
		},
		{
			name: "future timestamp",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(fmt.Sprintf(`{
					"id": "def456",
					"name": "t1_def456",
					"author": "testuser",
					"body": "Test comment",
					"parent_id": "t3_abc123",
					"link_id": "t3_abc123",
					"subreddit": "test",
					"score": 10,
					"ups": 10,
					"downs": 0,
					"created": %d,
					"created_utc": %d,
					"replies": ""
				}`, time.Now().Add(48*time.Hour).Unix(), time.Now().Add(48*time.Hour).Unix())),
			},
			expectError: true,
			errorText:   "CreatedUTC is in the future",
		},
		{
			name: "negative score - should pass (downvoted comments are valid)",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id": "def456",
					"name": "t1_def456",
					"author": "testuser",
					"body": "Test comment",
					"parent_id": "t3_abc123",
					"link_id": "t3_abc123",
					"subreddit": "test",
					"score": -50,
					"ups": -50,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"replies": ""
				}`),
			},
			expectError: false,
		},
		{
			name: "invalid subreddit name",
			thing: &types.Thing{
				Kind: "t1",
				Data: json.RawMessage(`{
					"id": "def456",
					"name": "t1_def456",
					"author": "testuser",
					"body": "Test comment",
					"parent_id": "t3_abc123",
					"link_id": "t3_abc123",
					"subreddit": "x",
					"score": 10,
					"ups": 10,
					"downs": 0,
					"created": 1234567890,
					"created_utc": 1234567890,
					"replies": ""
				}`),
			},
			expectError: true,
			errorText:   "Subreddit has invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseComment(context.Background(), tt.thing, &parseContext{
				seenIDs: make(map[string]bool),
			})

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorText != "" && !containsText(err.Error(), tt.errorText) {
					t.Errorf("expected error containing %q, got %q", tt.errorText, err.Error())
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestParseListing_MaliciousData tests that malicious pagination tokens are rejected
func TestParseListing_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "invalid AfterFullname - uppercase",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after": "T3_ABC123",
					"before": null,
					"children": []
				}`),
			},
			expectError: true,
			errorText:   "invalid AfterFullname from Reddit API",
		},
		{
			name: "invalid AfterFullname - SQL injection",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after": "t3_abc'; DROP TABLE--",
					"before": null,
					"children": []
				}`),
			},
			expectError: true,
			errorText:   "invalid AfterFullname from Reddit API",
		},
		{
			name: "invalid BeforeFullname - wrong format",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after": null,
					"before": "invalid_format",
					"children": []
				}`),
			},
			expectError: true,
			errorText:   "invalid BeforeFullname from Reddit API",
		},
		{
			name: "valid pagination tokens",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after": "t3_abc123",
					"before": "t3_xyz789",
					"children": []
				}`),
			},
			expectError: false,
		},
		{
			name: "empty pagination tokens - should pass",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after": "",
					"before": "",
					"children": []
				}`),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseListing(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorText != "" && !containsText(err.Error(), tt.errorText) {
					t.Errorf("expected error containing %q, got %q", tt.errorText, err.Error())
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// TestParseSubreddit_MaliciousData tests that malicious subreddit data is rejected
func TestParseSubreddit_MaliciousData(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		thing       *types.Thing
		expectError bool
		errorText   string
	}{
		{
			name: "invalid subreddit name - special chars",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{
					"id": "2qh1i",
					"name": "t5_2qh1i",
					"display_name": "test$subreddit",
					"title": "Test Subreddit",
					"subscribers": 1000,
					"created": 1234567890,
					"created_utc": 1234567890
				}`),
			},
			expectError: true,
			errorText:   "DisplayName has invalid format",
		},
		{
			name: "negative subscriber count",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{
					"id": "2qh1i",
					"name": "t5_2qh1i",
					"display_name": "testsubreddit",
					"title": "Test Subreddit",
					"subscribers": -100,
					"created": 1234567890,
					"created_utc": 1234567890
				}`),
			},
			expectError: true,
			errorText:   "Subscribers cannot be negative",
		},
		{
			name: "invalid subreddit name - too short",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{
					"id": "2qh1i",
					"name": "t5_2qh1i",
					"display_name": "ab",
					"title": "Test Subreddit",
					"subscribers": 1000,
					"created": 1234567890,
					"created_utc": 1234567890
				}`),
			},
			expectError: true,
			errorText:   "DisplayName has invalid format",
		},
		{
			name: "valid subreddit",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{
					"id": "2qh1i",
					"name": "t5_2qh1i",
					"display_name": "golang",
					"title": "Go Programming",
					"subscribers": 150000,
					"created": 1234567890,
					"created_utc": 1234567890
				}`),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSubreddit(context.Background(), tt.thing)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorText != "" && !containsText(err.Error(), tt.errorText) {
					t.Errorf("expected error containing %q, got %q", tt.errorText, err.Error())
				}
				if result != nil {
					t.Errorf("expected nil result on error, got %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

// containsText is a helper function to check if a string contains a substring
func containsText(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && contains(s, substr)))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCommentTreeWithMoreIDs verifies that MoreChildrenIDs are properly collected
// at each level of the tree.
func TestCommentTreeWithMoreIDs(t *testing.T) {
	parser := NewParser()

	thing := &types.Thing{
		Kind: "t1",
		Data: json.RawMessage(`{
			"id": "parent",
			"name": "t1_parent",
			"author": "user1",
			"body": "Parent comment",
			"score": 100,
			"ups": 100,
			"downs": 0,
			"created": 1234567890,
			"created_utc": 1234567890,
			"parent_id": "t3_post1",
			"link_id": "t3_post1",
			"subreddit": "test",
			"replies": {
				"kind": "Listing",
				"data": {
					"children": [
						{
							"kind": "t1",
							"id": "child",
							"name": "t1_child",
							"data": {
								"id": "child",
								"name": "t1_child",
								"author": "user2",
								"body": "Child comment",
								"score": 50,
								"ups": 50,
								"downs": 0,
								"created": 1234567890,
								"created_utc": 1234567890,
								"parent_id": "t1_parent",
								"link_id": "t3_post1",
								"subreddit": "test",
								"replies": ""
							}
						},
						{
							"kind": "more",
							"id": "more1",
							"name": "t2_more1",
							"data": {
								"id": "more1",
								"name": "t2_more1",
								"children": ["id1", "id2", "id3"]
							}
						}
					]
				}
			}
		}`),
	}

	parent, err := parser.ParseComment(context.Background(), thing, &parseContext{
		seenIDs: make(map[string]bool),
	})
	if err != nil {
		t.Fatalf("ParseComment failed: %v", err)
	}

	// Verify parent has 1 child and 3 more IDs
	if len(parent.Replies) != 1 {
		t.Errorf("Parent should have 1 child, got %d", len(parent.Replies))
	}
	if len(parent.MoreChildrenIDs) != 3 {
		t.Errorf("Parent should have 3 more IDs, got %d", len(parent.MoreChildrenIDs))
	}

	// Verify more IDs are correct
	expectedIDs := []string{"id1", "id2", "id3"}
	for i, id := range expectedIDs {
		if i >= len(parent.MoreChildrenIDs) || parent.MoreChildrenIDs[i] != id {
			t.Errorf("MoreChildrenIDs[%d] = %q, want %q", i, parent.MoreChildrenIDs[i], id)
		}
	}
}
