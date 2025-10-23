package parse

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

// BenchmarkParser_ParseThing benchmarks parsing different Thing types
func BenchmarkParser_ParseThing(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name  string
		thing *types.Thing
	}{
		{
			name:  "Listing",
			thing: testutil.NewListingBuilder().Build().ToThing(),
		},
		{
			name: "Post",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Post").
				WithAuthor("testuser").
				WithScore(100).
				ToThing(),
		},
		{
			name: "Comment",
			thing: testutil.NewCommentBuilder().
				WithID("comment123").
				WithAuthor("testuser").
				WithBody("Test comment").
				ToThing(),
		},
		{
			name: "Subreddit",
			thing: testutil.NewSubreddit("golang").
				WithID("2qh1i").
				WithTitle("Go Programming Language").
				WithSubscribers(150000).
				ToThing(),
		},
		{
			name: "Account",
			thing: testutil.NewAccount("testuser").
				WithID("user123").
				WithLinkKarma(1000).
				WithCommentKarma(5000).
				ToThing(),
		},
		{
			name: "More",
			thing: testutil.NewMore().
				WithID("more123").
				WithChildren([]string{"id1", "id2", "id3"}).
				ToThing(),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ParseThing(ctx, tt.thing)
			}
		})
	}
}

// BenchmarkParser_ParseListing benchmarks parsing listings of different sizes
func BenchmarkParser_ParseListing(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name string
		size int
	}{
		{"10_children", 10},
		{"100_children", 100},
		{"1000_children", 1000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create listing with N children
			builder := testutil.NewListingBuilder().
				WithAfter("t3_after123").
				WithBefore("t3_before456")

			for i := 0; i < tt.size; i++ {
				builder.AddChild(testutil.NewPostBuilder().
					WithID(generateID(i)).
					ToThing())
			}

			thing := builder.Build().ToThing()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ParseListing(ctx, thing)
			}
		})
	}
}

// BenchmarkParser_ParsePost benchmarks parsing posts with different content types
func BenchmarkParser_ParsePost(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name  string
		thing *types.Thing
	}{
		{
			name: "link_post",
			thing: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Test Link Post").
				WithURL("https://example.com").
				ToThing(),
		},
		{
			name: "text_post_short",
			thing: testutil.NewPostBuilder().
				WithID("def456").
				WithTitle("Test Text Post").
				WithSelfText("Short text content.").
				ToThing(),
		},
		{
			name: "text_post_long",
			thing: testutil.NewPostBuilder().
				WithID("ghi789").
				WithTitle("Test Long Text Post").
				WithSelfText(strings.Repeat("Lorem ipsum dolor sit amet. ", 100)).
				ToThing(),
		},
		{
			name: "image_post",
			thing: testutil.NewPostBuilder().
				WithID("jkl012").
				WithTitle("Test Image Post").
				WithURL("https://i.redd.it/example.jpg").
				ToThing(),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ParsePost(ctx, tt.thing)
			}
		})
	}
}

// BenchmarkParser_ExtractPosts benchmarks extracting posts from listings
func BenchmarkParser_ExtractPosts(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name string
		size int
	}{
		{"10_posts", 10},
		{"25_posts", 25},
		{"100_posts", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create listing with N posts
			builder := testutil.NewListingBuilder()
			for i := 0; i < tt.size; i++ {
				builder.AddChild(testutil.NewPostBuilder().
					WithID(generateID(i)).
					WithTitle("Test Post " + generateID(i)).
					WithAuthor("user" + generateID(i)).
					WithScore(100 + i).
					ToThing())
			}
			thing := builder.Build().ToThing()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ExtractPosts(ctx, thing)
			}
		})
	}
}

// BenchmarkParser_ParseComment_Shallow benchmarks parsing a single comment without replies
func BenchmarkParser_ParseComment_Shallow(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name   string
		length string
	}{
		{"short_body", "Test comment."},
		{"medium_body", strings.Repeat("This is a medium length comment. ", 10)},
		{"long_body", strings.Repeat("This is a very long comment with lots of text. ", 50)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			thing := testutil.NewCommentBuilder().
				WithID("comment123").
				WithAuthor("testuser").
				WithBody(tt.length).
				WithParentID("t3_post123").
				ToThing()

			pc := &parseContext{
				seenIDs: make(map[string]bool),
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset parse context for each iteration
				pc.depth = 0
				clear(pc.seenIDs)

				_, _ = parser.ParseComment(ctx, thing, pc)
			}
		})
	}
}

// BenchmarkParser_ParseComment_Deep benchmarks parsing deeply nested comment trees
func BenchmarkParser_ParseComment_Deep(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name  string
		depth int
	}{
		{"depth_20", 20},
		{"depth_40", 40},
		{"depth_50", 50}, // Max depth
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			thing := createDeepCommentTree(tt.depth)

			pc := &parseContext{
				seenIDs: make(map[string]bool),
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset parse context for each iteration
				pc.depth = 0
				clear(pc.seenIDs)

				_, _ = parser.ParseComment(ctx, thing, pc)
			}
		})
	}
}

// BenchmarkParser_ParseComment_Wide benchmarks parsing comments with many siblings
func BenchmarkParser_ParseComment_Wide(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name     string
		siblings int
	}{
		{"10_siblings", 10},
		{"50_siblings", 50},
		{"100_siblings", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			thing := createWideCommentTree(tt.siblings)

			pc := &parseContext{
				seenIDs: make(map[string]bool),
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Reset parse context for each iteration
				pc.depth = 0
				clear(pc.seenIDs)

				_, _ = parser.ParseComment(ctx, thing, pc)
			}
		})
	}
}

// BenchmarkParser_ExtractComments benchmarks extracting comments from listings
func BenchmarkParser_ExtractComments(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name     string
		comments int
	}{
		{"10_comments", 10},
		{"50_comments", 50},
		{"100_comments", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create listing with N comments
			builder := testutil.NewListingBuilder()
			for i := 0; i < tt.comments; i++ {
				builder.AddChild(testutil.NewCommentBuilder().
					WithID(generateID(i)).
					WithAuthor("user" + generateID(i)).
					WithBody("Test comment " + generateID(i)).
					WithParentID("t3_post123").
					ToThing())
			}
			thing := builder.Build().ToThing()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _, _ = parser.ExtractComments(ctx, thing)
			}
		})
	}
}

// BenchmarkParser_ExtractComments_WithReplies benchmarks extracting comments with nested replies
func BenchmarkParser_ExtractComments_WithReplies(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name        string
		topLevel    int
		repliesEach int
	}{
		{"10_top_5_replies", 10, 5},
		{"25_top_10_replies", 25, 10},
		{"50_top_5_replies", 50, 5},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Create listing with comments that have replies
			builder := testutil.NewListingBuilder()
			for i := 0; i < tt.topLevel; i++ {
				commentBuilder := testutil.NewCommentBuilder().
					WithID(generateID(i)).
					WithAuthor("user" + generateID(i)).
					WithBody("Top level comment " + generateID(i)).
					WithParentID("t3_post123")

				// Add replies to each top-level comment
				for j := 0; j < tt.repliesEach; j++ {
					reply := testutil.NewCommentBuilder().
						WithID(generateID(i*1000 + j)).
						WithAuthor("replier" + generateID(j)).
						WithBody("Reply " + generateID(j)).
						WithParentID("t1_" + generateID(i)).
						Build()
					commentBuilder.WithReply(reply)
				}

				builder.AddChild(commentBuilder.ToThing())
			}
			thing := builder.Build().ToThing()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _, _ = parser.ExtractComments(ctx, thing)
			}
		})
	}
}

// BenchmarkParser_CollectMoreIDs benchmarks recursive collection of "more" IDs
func BenchmarkParser_CollectMoreIDs(b *testing.B) {
	parser := NewParser()

	tests := []struct {
		name      string
		treeShape string
		depth     int
		width     int
	}{
		{"deep_tree", "deep", 30, 1},
		{"wide_tree", "wide", 3, 50},
		{"balanced_tree", "balanced", 4, 4}, // 4^4 = 256 nodes (reasonable)
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			var comment *types.Comment
			if tt.treeShape == "deep" {
				comment = createDeepCommentTreeWithMore(tt.depth)
			} else if tt.treeShape == "wide" {
				comment = createWideCommentTreeWithMore(tt.width)
			} else {
				comment = createBalancedCommentTreeWithMore(tt.depth, tt.width)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = parser.collectMoreIDs(comment)
			}
		})
	}
}

// BenchmarkParser_ExtractPostAndComments benchmarks full comment thread extraction
func BenchmarkParser_ExtractPostAndComments(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name     string
		comments int
		depth    int
	}{
		{"simple_thread", 25, 3},
		{"medium_thread", 100, 5},
		{"large_thread", 250, 8},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			response := createPostAndCommentsResponse(tt.comments, tt.depth)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ExtractPostAndComments(ctx, response)
			}
		})
	}
}

// BenchmarkParseContext_Pool benchmarks the sync.Pool operations
func BenchmarkParseContext_Pool(b *testing.B) {
	parser := NewParser()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pc := parser.pool.Get().(*parseContext)
		parser.pool.Put(pc)
	}
}

// Helper functions for creating test data

// generateID creates a simple base36-like ID from an integer
func generateID(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	if n < 36 {
		return string(chars[n])
	}
	var result []byte
	for n > 0 {
		result = append([]byte{chars[n%36]}, result...)
		n /= 36
	}
	return string(result)
}

// createDeepCommentTree creates a deeply nested comment tree using raw JSON
func createDeepCommentTree(depth int) *types.Thing {
	// Build from the bottom up
	var repliesJSON string
	for i := depth - 1; i >= 0; i-- {
		commentID := generateID(i)
		parentID := "t3_post123"
		if i > 0 {
			parentID = "t1_" + generateID(i-1)
		}

		// Determine replies value based on position in tree
		var commentReplies string
		if i == depth-1 {
			// Leaf node with no replies
			commentReplies = `""`
		} else {
			// Node with one child - use the previously built repliesJSON
			commentReplies = repliesJSON
		}

		commentJSON := `{
			"id": "` + commentID + `",
			"name": "t1_` + commentID + `",
			"author": "user` + commentID + `",
			"body": "Comment at depth ` + generateID(i) + `",
			"score": 10,
			"ups": 10,
			"downs": 0,
			"created": 1234567890,
			"created_utc": 1234567890,
			"parent_id": "` + parentID + `",
			"link_id": "t3_post123",
			"subreddit": "test",
			"replies": ` + commentReplies + `
		}`

		if i < depth-1 {
			repliesJSON = `{
				"kind": "Listing",
				"data": {
					"children": [
						{
							"kind": "t1",
							"data": ` + commentJSON + `
						}
					]
				}
			}`
		} else {
			repliesJSON = commentJSON
		}
	}

	data := []byte(repliesJSON)
	return &types.Thing{
		Kind: "t1",
		Data: data,
	}
}

// createWideCommentTree creates a comment with many direct replies using raw JSON
func createWideCommentTree(siblings int) *types.Thing {
	var children []string
	for i := 0; i < siblings; i++ {
		commentID := generateID(i + 1)
		child := `{
			"kind": "t1",
			"data": {
				"id": "` + commentID + `",
				"name": "t1_` + commentID + `",
				"author": "user` + commentID + `",
				"body": "Reply ` + generateID(i) + `",
				"score": 5,
				"ups": 5,
				"downs": 0,
				"created": 1234567890,
				"created_utc": 1234567890,
				"parent_id": "t1_parent",
				"link_id": "t3_post123",
				"subreddit": "test",
				"replies": ""
			}
		}`
		children = append(children, child)
	}

	data := []byte(`{
		"id": "parent",
		"name": "t1_parent",
		"author": "user0",
		"body": "Parent comment",
		"score": 100,
		"ups": 100,
		"downs": 0,
		"created": 1234567890,
		"created_utc": 1234567890,
		"parent_id": "t3_post123",
		"link_id": "t3_post123",
		"subreddit": "test",
		"replies": {
			"kind": "Listing",
			"data": {
				"children": [` + strings.Join(children, ",") + `]
			}
		}
	}`)

	return &types.Thing{
		Kind: "t1",
		Data: data,
	}
}

// createDeepCommentTreeWithMore creates a deep tree with MoreChildrenIDs at each level
func createDeepCommentTreeWithMore(depth int) *types.Comment {
	comment := &types.Comment{
		ThingData: types.ThingData{
			ID:   "root",
			Name: "t1_root",
		},
		Votable: types.Votable{
			Score: 10,
			Ups:   10,
			Downs: 0,
		},
		Created: types.Created{
			Created:    1234567890,
			CreatedUTC: 1234567890,
		},
		Body:            "Root comment",
		MoreChildrenIDs: []string{"more_root_1", "more_root_2", "more_root_3"},
		Replies:         []*types.Comment{},
		Author:          "user0",
		LinkID:          "t3_post123",
		ParentID:        "t3_post123",
		Subreddit:       "test",
		SubredditID:     "t5_test",
	}

	current := comment
	for i := 1; i < depth; i++ {
		child := &types.Comment{
			ThingData: types.ThingData{
				ID:   generateID(i),
				Name: "t1_" + generateID(i),
			},
			Votable: types.Votable{
				Score: 10,
				Ups:   10,
				Downs: 0,
			},
			Created: types.Created{
				Created:    1234567890,
				CreatedUTC: 1234567890,
			},
			Body:            "Comment at depth " + generateID(i),
			MoreChildrenIDs: []string{"more_" + generateID(i) + "_1", "more_" + generateID(i) + "_2"},
			Replies:         []*types.Comment{},
			Author:          "user" + generateID(i),
			LinkID:          "t3_post123",
			ParentID:        "t1_" + current.ID,
			Subreddit:       "test",
			SubredditID:     "t5_test",
		}
		current.Replies = append(current.Replies, child)
		current = child
	}

	return comment
}

// createWideCommentTreeWithMore creates a wide tree with MoreChildrenIDs
func createWideCommentTreeWithMore(width int) *types.Comment {
	comment := &types.Comment{
		ThingData: types.ThingData{
			ID:   "root",
			Name: "t1_root",
		},
		Votable: types.Votable{
			Score: 100,
			Ups:   100,
			Downs: 0,
		},
		Created: types.Created{
			Created:    1234567890,
			CreatedUTC: 1234567890,
		},
		Body:            "Root comment",
		MoreChildrenIDs: []string{"more_root_1", "more_root_2"},
		Replies:         []*types.Comment{},
		Author:          "user0",
		LinkID:          "t3_post123",
		ParentID:        "t3_post123",
		Subreddit:       "test",
		SubredditID:     "t5_test",
	}

	for i := 0; i < width; i++ {
		child := &types.Comment{
			ThingData: types.ThingData{
				ID:   generateID(i + 1),
				Name: "t1_" + generateID(i+1),
			},
			Votable: types.Votable{
				Score: 5,
				Ups:   5,
				Downs: 0,
			},
			Created: types.Created{
				Created:    1234567890,
				CreatedUTC: 1234567890,
			},
			Body:            "Reply " + generateID(i),
			MoreChildrenIDs: []string{"more_" + generateID(i) + "_1"},
			Replies:         []*types.Comment{},
			Author:          "user" + generateID(i+1),
			LinkID:          "t3_post123",
			ParentID:        "t1_root",
			Subreddit:       "test",
			SubredditID:     "t5_test",
		}
		comment.Replies = append(comment.Replies, child)
	}

	return comment
}

// createBalancedCommentTreeWithMore creates a balanced tree with MoreChildrenIDs
func createBalancedCommentTreeWithMore(depth int, width int) *types.Comment {
	var buildTree func(currentDepth int, id string, parentID string) *types.Comment
	buildTree = func(currentDepth int, id string, parentID string) *types.Comment {
		comment := &types.Comment{
			ThingData: types.ThingData{
				ID:   id,
				Name: "t1_" + id,
			},
			Votable: types.Votable{
				Score: 10,
				Ups:   10,
				Downs: 0,
			},
			Created: types.Created{
				Created:    1234567890,
				CreatedUTC: 1234567890,
			},
			Body:            "Comment " + id,
			MoreChildrenIDs: []string{"more_" + id + "_1", "more_" + id + "_2"},
			Replies:         []*types.Comment{},
			Author:          "user_" + id,
			LinkID:          "t3_post123",
			ParentID:        parentID,
			Subreddit:       "test",
			SubredditID:     "t5_test",
		}

		if currentDepth < depth {
			for i := 0; i < width; i++ {
				childID := id + "_" + generateID(i)
				child := buildTree(currentDepth+1, childID, "t1_"+id)
				comment.Replies = append(comment.Replies, child)
			}
		}

		return comment
	}

	return buildTree(1, "root", "t3_post123")
}

// createPostAndCommentsResponse creates a realistic post+comments response
func createPostAndCommentsResponse(numComments int, maxDepth int) []*types.Thing {
	// First listing: post
	postListing := testutil.NewListingBuilder().
		AddChild(testutil.NewPostBuilder().
			WithID("post123").
			WithTitle("Test Post").
			WithAuthor("postauthor").
			WithScore(500).
			WithNumComments(numComments).
			ToThing()).
		Build().ToThing()

	// Second listing: comments
	commentBuilder := testutil.NewListingBuilder()

	// Create top-level comments
	topLevelCount := numComments / (maxDepth + 1)
	if topLevelCount < 1 {
		topLevelCount = numComments
	}

	for i := 0; i < topLevelCount; i++ {
		comment := createCommentWithDepth(i, 0, maxDepth, "t3_post123")
		commentBuilder.AddChild(comment)
	}

	commentListing := commentBuilder.Build().ToThing()

	return []*types.Thing{postListing, commentListing}
}

// createCommentWithDepth creates a comment with nested replies up to maxDepth
func createCommentWithDepth(id int, currentDepth int, maxDepth int, parentID string) *types.Thing {
	commentID := generateID(id*1000 + currentDepth)
	builder := testutil.NewCommentBuilder().
		WithID(commentID).
		WithAuthor("user_" + commentID).
		WithBody("Comment at depth " + generateID(currentDepth)).
		WithParentID(parentID)

	// Add a couple of replies if we haven't reached max depth
	if currentDepth < maxDepth {
		for j := 0; j < 2; j++ {
			replyID := id*1000 + currentDepth*10 + j + 1
			reply := createCommentData(replyID, currentDepth+1, maxDepth, "t1_"+commentID)
			builder.WithReply(reply)
		}
	}

	return builder.ToThing()
}

// createCommentData creates comment data (not Thing) with nested replies
func createCommentData(id int, currentDepth int, maxDepth int, parentID string) *types.Comment {
	commentID := generateID(id)
	builder := testutil.NewCommentBuilder().
		WithID(commentID).
		WithAuthor("user_" + commentID).
		WithBody("Comment at depth " + generateID(currentDepth)).
		WithParentID(parentID)

	// Add replies if we haven't reached max depth
	if currentDepth < maxDepth {
		for j := 0; j < 2; j++ {
			replyID := id*10 + j + 1
			reply := createCommentData(replyID, currentDepth+1, maxDepth, "t1_"+commentID)
			builder.WithReply(reply)
		}
	}

	return builder.Build()
}

// BenchmarkParser_ParseSubreddit benchmarks parsing subreddit data
func BenchmarkParser_ParseSubreddit(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name  string
		thing *types.Thing
	}{
		{
			name: "minimal",
			thing: testutil.NewSubreddit("golang").
				WithID("2qh1i").
				ToThing(),
		},
		{
			name: "full_data",
			thing: testutil.NewSubreddit("programming").
				WithID("2qh1i").
				WithTitle("Programming").
				WithDescription(strings.Repeat("A subreddit for programming discussion. ", 20)).
				WithSubscribers(5000000).
				WithActiveUsers(5000).
				ToThing(),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ParseSubreddit(ctx, tt.thing)
			}
		})
	}
}

// BenchmarkParser_ParseAccount benchmarks parsing account data
func BenchmarkParser_ParseAccount(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	thing := testutil.NewAccount("testuser").
		WithID("user123").
		WithLinkKarma(50000).
		WithCommentKarma(100000).
		WithGold(true).
		ToThing()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseAccount(ctx, thing)
	}
}

// BenchmarkParser_ParseMore benchmarks parsing "more" data
func BenchmarkParser_ParseMore(b *testing.B) {
	parser := NewParser()
	ctx := context.Background()

	tests := []struct {
		name     string
		children int
	}{
		{"10_children", 10},
		{"100_children", 100},
		{"500_children", 500},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			children := make([]string, tt.children)
			for i := 0; i < tt.children; i++ {
				children[i] = generateID(i)
			}

			thing := testutil.NewMore().
				WithID("more123").
				WithChildren(children).
				ToThing()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = parser.ParseMore(ctx, thing)
			}
		})
	}
}

// BenchmarkParser_UnmarshalJSON benchmarks the raw JSON unmarshaling overhead
func BenchmarkParser_UnmarshalJSON(b *testing.B) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "post",
			data: testutil.NewPostBuilder().
				WithID("abc123").
				WithTitle("Benchmark Post").
				ToJSON(),
		},
		{
			name: "comment",
			data: testutil.NewCommentBuilder().
				WithID("comment123").
				WithBody("Benchmark comment").
				ToJSON(),
		},
		{
			name: "listing_10",
			data: func() json.RawMessage {
				builder := testutil.NewListingBuilder()
				for i := 0; i < 10; i++ {
					builder.AddChild(testutil.NewPostBuilder().WithID(generateID(i)).ToThing())
				}
				return builder.ToJSON()
			}(),
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				var thing types.Thing
				_ = json.Unmarshal(tt.data, &thing)
			}
		})
	}
}
