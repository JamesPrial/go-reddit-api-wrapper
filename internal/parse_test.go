package internal

import (
	"encoding/json"
	"testing"

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
				Data: json.RawMessage(`{"after":"after123","before":null,"children":[]}`),
			},
			expectError:  false,
			expectedType: "*types.ListingData",
		},
		{
			name: "t1 comment",
			thing: &types.Thing{
				Kind: "t1",
				ID:   "comment123",
				Name: "t1_comment123",
				Data: json.RawMessage(`{"author":"testuser","body":"test comment","score":10,"replies":""}`),
			},
			expectError:  false,
			expectedType: "*types.Comment",
		},
		{
			name: "t2 account",
			thing: &types.Thing{
				Kind: "t2",
				Data: json.RawMessage(`{"name":"testuser","link_karma":100,"comment_karma":200}`),
			},
			expectError:  false,
			expectedType: "*types.AccountData",
		},
		{
			name: "t3 link",
			thing: &types.Thing{
				Kind: "t3",
				ID:   "post123",
				Name: "t3_post123",
				Data: json.RawMessage(`{"author":"testuser","title":"Test Post","url":"http://example.com","score":100}`),
			},
			expectError:  false,
			expectedType: "*types.Post",
		},
		{
			name: "t4 message",
			thing: &types.Thing{
				Kind: "t4",
				Data: json.RawMessage(`{"author":"testuser","body":"test message","subject":"Test Subject"}`),
			},
			expectError:  false,
			expectedType: "*types.MessageData",
		},
		{
			name: "t5 subreddit",
			thing: &types.Thing{
				Kind: "t5",
				Data: json.RawMessage(`{"display_name":"golang","title":"Go Programming","subscribers":100000}`),
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
			result, err := parser.ParseThing(tt.thing)

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
				Data: json.RawMessage(`{"after":"after123","before":"before456","modhash":"modhash789","children":[]}`),
			},
			expectError: false,
		},
		{
			name: "listing with children",
			thing: &types.Thing{
				Kind: "Listing",
				Data: json.RawMessage(`{
					"after":"after123",
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
			result, err := parser.ParseListing(tt.thing)

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

func TestParseLink(t *testing.T) {
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
			name: "valid link",
			thing: &types.Thing{
				Kind: "t3",
				Data: json.RawMessage(`{
					"author":"testuser",
					"title":"Test Post",
					"url":"http://example.com",
					"score":100,
					"num_comments":50,
					"subreddit":"golang",
					"created_utc":1234567890,
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
					"author":"testuser",
					"title":"Self Post Title",
					"selftext":"This is the self text",
					"is_self":true,
					"score":50,
					"subreddit":"AskReddit",
					"created_utc":1234567890,
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
			result, err := parser.ParseLink(tt.thing)

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
					"author":"testuser",
					"body":"This is a test comment",
					"body_html":"<p>This is a test comment</p>",
					"score":10,
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
					"author":"testuser",
					"body":"Parent comment",
					"score":20,
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
					"author":"testuser",
					"body":"Edited comment",
					"score":5,
					"edited":1234567900,
					"replies":"",
					"parent_id":"t1_parent"
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
			result, err := parser.ParseComment(tt.thing)

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
					"display_name":"golang",
					"title":"Go Programming Language",
					"subscribers":150000,
					"description":"A subreddit for Go programmers",
					"public_description":"Public description",
					"url":"/r/golang",
					"over18":false,
					"subreddit_type":"public"
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
			result, err := parser.ParseSubreddit(tt.thing)

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
					"name":"testuser",
					"id":"user123",
					"link_karma":1000,
					"comment_karma":5000,
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
			result, err := parser.ParseAccount(tt.thing)

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
					"author":"sender",
					"body":"Message body",
					"body_html":"<p>Message body</p>",
					"subject":"Test Subject",
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
			result, err := parser.ParseMessage(tt.thing)

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
					"name":"more_more123"
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
			result, err := parser.ParseMore(tt.thing)

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
					"after":"after123",
					"children":[
						{
							"kind":"t3",
							"id":"post1",
							"name":"t3_post1",
							"data":{
								"author":"user1",
								"title":"First Post",
								"url":"http://example.com/1",
								"score":100
							}
						},
						{
							"kind":"t3",
							"id":"post2",
							"name":"t3_post2",
							"data":{
								"author":"user2",
								"title":"Second Post",
								"url":"http://example.com/2",
								"score":200
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
								"author":"user1",
								"title":"Post",
								"url":"http://example.com"
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
			posts, err := parser.ExtractPosts(tt.thing)

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
				Kind: "t1",
				ID:   "comment1",
				Name: "t1_comment1",
				Data: json.RawMessage(`{
					"author":"user1",
					"body":"Test comment",
					"score":10,
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
				Kind: "t1",
				ID:   "comment1",
				Name: "t1_comment1",
				Data: json.RawMessage(`{
					"author":"user1",
					"body":"Parent comment",
					"replies":{
						"kind":"Listing",
						"data":{
							"children":[
								{
									"kind":"t1",
									"id":"reply1",
									"name":"t1_reply1",
									"data":{
										"author":"user2",
										"body":"Reply",
										"replies":""
									}
								}
							]
						}
					}
				}`),
			},
			expectError:    false,
			expectComments: 2, // Parent + reply
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
								"author":"user1",
								"body":"First comment",
								"replies":""
							}
						},
						{
							"kind":"t1",
							"id":"comment2",
							"name":"t1_comment2",
							"data":{
								"author":"user2",
								"body":"Second comment",
								"replies":""
							}
						},
						{
							"kind":"more",
							"id":"more1",
							"data":{
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
								"author":"user1",
								"body":"Parent",
								"replies":{
									"kind":"Listing",
									"data":{
										"children":[
											{
												"kind":"t1",
												"id":"reply1",
												"name":"t1_reply1",
												"data":{
													"author":"user2",
													"body":"Child",
													"replies":{
														"kind":"Listing",
														"data":{
															"children":[
																{
																	"kind":"t1",
																	"id":"reply2",
																	"name":"t1_reply2",
																	"data":{
																		"author":"user3",
																		"body":"Grandchild",
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
			expectComments: 3, // Parent + child + grandchild
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
			comments, moreIDs, err := parser.ExtractComments(tt.thing)

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
									"author":"postauthor",
									"title":"Test Post",
									"url":"http://example.com",
									"score":100
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
									"author":"commenter1",
									"body":"First comment",
									"replies":""
								}
							},
							{
								"kind":"t1",
								"id":"comment2",
								"name":"t1_comment2",
								"data":{
									"author":"commenter2",
									"body":"Second comment",
									"replies":{
										"kind":"Listing",
										"data":{
											"children":[
												{
													"kind":"t1",
													"id":"reply1",
													"name":"t1_reply1",
													"data":{
														"author":"replier",
														"body":"Reply",
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
								"data":{
									"children":["id1","id2"]
								}
							}
						]
					}`),
				},
			},
			expectError:    false,
			expectPost:     true,
			expectComments: 3, // 2 top-level comments + 1 reply
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
									"author":"postauthor",
									"title":"Test Post",
									"url":"http://example.com"
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
			post, comments, moreIDs, err := parser.ExtractPostAndComments(tt.response)

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

				if tt.expectPost {
					if post == nil {
						t.Errorf("expected post but got nil")
					}
				} else {
					if post != nil {
						t.Errorf("expected no post but got one")
					}
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

