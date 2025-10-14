package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// MockServer provides a configurable mock Reddit API server for testing.
// It wraps httptest.Server and provides fluent configuration methods for
// storing mock data that will be returned by the appropriate endpoints.
//
// The MockServer supports all common Reddit API endpoints:
//   - GET /r/{subreddit}/hot - returns posts configured with WithPosts
//   - GET /r/{subreddit}/new - returns posts configured with WithPosts
//   - GET /r/{subreddit}/top - returns posts configured with WithPosts
//   - GET /r/{subreddit}/about - returns subreddit configured with WithSubreddit
//   - GET /r/{subreddit}/comments/{postID} - returns post and comments configured with WithComments
//   - GET /api/v1/me - returns account configured with WithAccount
//
// All responses include realistic rate limit headers (X-Ratelimit-Remaining: 60, X-Ratelimit-Reset: 60).
// Unconfigured endpoints return empty Listings. Error responses can be configured with WithError.
//
// Basic example:
//
//	server := testutil.NewMockServer().
//	    WithSubreddit("golang", testutil.NewSubreddit("golang").Build()).
//	    WithPosts("golang", "hot",
//	        testutil.NewPostBuilder().WithTitle("Post 1").Build(),
//	        testutil.NewPostBuilder().WithTitle("Post 2").Build()).
//	    Start()
//	defer server.Close()
//
//	// Use server.URL() as the base URL for your Reddit client
//	client := reddit.New(server.URL(), "test-agent")
//
// Comprehensive example with all features:
//
//	// Create test data using builders
//	sub := testutil.NewSubreddit("golang").
//	    WithSubscribers(500000).
//	    WithTitle("The Go Programming Language").
//	    Build()
//
//	post1 := testutil.NewPostBuilder().
//	    WithID("post1").
//	    WithTitle("Introduction to Go").
//	    WithAuthor("gopher").
//	    WithScore(1500).
//	    Build()
//
//	post2 := testutil.NewPostBuilder().
//	    WithID("post2").
//	    WithTitle("Go 1.20 Released").
//	    WithAuthor("golang_team").
//	    WithScore(2500).
//	    Build()
//
//	mainPost := testutil.NewPostBuilder().
//	    WithID("abc123").
//	    WithTitle("Ask Anything About Go").
//	    Build()
//
//	comment1 := testutil.NewCommentBuilder().
//	    WithID("c1").
//	    WithBody("Great question!").
//	    WithAuthor("expert").
//	    WithParentPost("abc123").
//	    Build()
//
//	comment2 := testutil.NewCommentBuilder().
//	    WithID("c2").
//	    WithBody("Here's my answer...").
//	    WithAuthor("helper").
//	    WithParentPost("abc123").
//	    Build()
//
//	account := testutil.NewAccount("testuser").
//	    WithLinkKarma(10000).
//	    WithCommentKarma(5000).
//	    Build()
//
//	// Configure mock server
//	server := testutil.NewMockServer().
//	    WithSubreddit("golang", sub).
//	    WithPosts("golang", "hot", post1, post2).
//	    WithComments("golang", "abc123", mainPost, comment1, comment2).
//	    WithAccount(account).
//	    WithError("/r/private", http.StatusForbidden, "Private subreddit").
//	    Start()
//	defer server.Close()
//
//	// Now test your Reddit client against server.URL()
type MockServer struct {
	server     *httptest.Server
	posts      map[string]map[string][]*types.Post // [subreddit][sort]posts
	comments   map[string]map[string]*CommentData  // [subreddit][postID]
	subreddits map[string]*types.SubredditData     // [name]
	errors     map[string]*ErrorConfig             // [pathPattern]statusCode+message
	account    *types.AccountData
}

// CommentData holds a post and its associated comments for the mock server.
type CommentData struct {
	Post     *types.Post
	Comments []*types.Comment
}

// ErrorConfig specifies an error response for a specific path pattern.
type ErrorConfig struct {
	StatusCode int
	Message    string
}

// NewMockServer creates a new MockServer instance.
// The server is not started until Start() is called.
func NewMockServer() *MockServer {
	return &MockServer{
		posts:      make(map[string]map[string][]*types.Post),
		comments:   make(map[string]map[string]*CommentData),
		subreddits: make(map[string]*types.SubredditData),
		errors:     make(map[string]*ErrorConfig),
	}
}

// WithPosts configures posts for a specific subreddit and sort order.
// The sort parameter should be "hot", "new", "top", etc.
// Returns the MockServer for method chaining.
func (m *MockServer) WithPosts(subreddit, sort string, posts ...*types.Post) *MockServer {
	if m.posts[subreddit] == nil {
		m.posts[subreddit] = make(map[string][]*types.Post)
	}
	m.posts[subreddit][sort] = posts
	return m
}

// WithComments configures comments for a specific subreddit and post ID.
// The post parameter is the post that the comments belong to.
// Returns the MockServer for method chaining.
func (m *MockServer) WithComments(subreddit, postID string, post *types.Post, comments ...*types.Comment) *MockServer {
	if m.comments[subreddit] == nil {
		m.comments[subreddit] = make(map[string]*CommentData)
	}
	m.comments[subreddit][postID] = &CommentData{
		Post:     post,
		Comments: comments,
	}
	return m
}

// WithSubreddit configures subreddit information for a specific subreddit.
// Returns the MockServer for method chaining.
func (m *MockServer) WithSubreddit(name string, sub *types.SubredditData) *MockServer {
	m.subreddits[name] = sub
	return m
}

// WithAccount configures account information for the /api/v1/me endpoint.
// Returns the MockServer for method chaining.
func (m *MockServer) WithAccount(account *types.AccountData) *MockServer {
	m.account = account
	return m
}

// WithError configures an error response for a specific path pattern.
// The pathPattern is a substring that will be matched against the request path.
// Returns the MockServer for method chaining.
func (m *MockServer) WithError(pathPattern string, statusCode int, message string) *MockServer {
	m.errors[pathPattern] = &ErrorConfig{
		StatusCode: statusCode,
		Message:    message,
	}
	return m
}

// Start creates and starts the mock HTTP server.
// Returns the MockServer itself for convenience in chaining and accessing the URL.
func (m *MockServer) Start() *MockServer {
	m.server = httptest.NewServer(http.HandlerFunc(m.handler))
	return m
}

// Close stops the mock server.
func (m *MockServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// URL returns the base URL of the mock server as a string.
// Returns empty string if the server hasn't been started.
// This is a convenience method that returns m.server.URL directly.
func (m *MockServer) URL() string {
	if m.server != nil {
		return m.server.URL
	}
	return ""
}

// Server returns the underlying httptest.Server.
// Returns nil if the server hasn't been started.
// Use this if you need direct access to the httptest.Server for advanced configuration.
func (m *MockServer) Server() *httptest.Server {
	return m.server
}

// handler routes incoming requests to the appropriate mock endpoint.
func (m *MockServer) handler(w http.ResponseWriter, r *http.Request) {
	// Set standard Reddit API headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Ratelimit-Remaining", "60")
	w.Header().Set("X-Ratelimit-Reset", "60")

	path := r.URL.Path

	// Check for configured errors first
	for pattern, errCfg := range m.errors {
		if strings.Contains(path, pattern) {
			w.WriteHeader(errCfg.StatusCode)
			errorData := map[string]interface{}{
				"error":   http.StatusText(errCfg.StatusCode),
				"message": errCfg.Message,
			}
			json.NewEncoder(w).Encode(errorData)
			return
		}
	}

	// Route to appropriate handler
	switch {
	case path == "/api/v1/me":
		m.handleAccount(w, r)
	case strings.HasSuffix(path, "/about"):
		m.handleSubreddit(w, r)
	case strings.Contains(path, "/comments/"):
		m.handleComments(w, r)
	case strings.HasSuffix(path, "/hot"):
		m.handlePosts(w, r, "hot")
	case strings.HasSuffix(path, "/new"):
		m.handlePosts(w, r, "new")
	case strings.HasSuffix(path, "/top"):
		m.handlePosts(w, r, "top")
	default:
		// Return empty listing for unconfigured endpoints
		m.writeEmptyListing(w)
	}
}

// handleAccount handles GET /api/v1/me
func (m *MockServer) handleAccount(w http.ResponseWriter, r *http.Request) {
	if m.account == nil {
		http.Error(w, "Account not configured", http.StatusNotFound)
		return
	}

	thing := map[string]interface{}{
		"kind": "t2",
		"data": m.account,
	}
	json.NewEncoder(w).Encode(thing)
}

// handleSubreddit handles GET /r/{subreddit}/about
func (m *MockServer) handleSubreddit(w http.ResponseWriter, r *http.Request) {
	subreddit := extractSubreddit(r.URL.Path)
	if subreddit == "" {
		http.Error(w, "Invalid subreddit path", http.StatusBadRequest)
		return
	}

	sub, ok := m.subreddits[subreddit]
	if !ok {
		http.Error(w, "Subreddit not found", http.StatusNotFound)
		return
	}

	thing := map[string]interface{}{
		"kind": "t5",
		"data": sub,
	}
	json.NewEncoder(w).Encode(thing)
}

// handlePosts handles GET /r/{subreddit}/{sort}
func (m *MockServer) handlePosts(w http.ResponseWriter, r *http.Request, sort string) {
	subreddit := extractSubreddit(r.URL.Path)
	if subreddit == "" {
		m.writeEmptyListing(w)
		return
	}

	posts, ok := m.posts[subreddit][sort]
	if !ok {
		m.writeEmptyListing(w)
		return
	}

	// Convert posts to Things
	children := make([]interface{}, len(posts))
	for i, post := range posts {
		children[i] = map[string]interface{}{
			"kind": "t3",
			"data": post,
		}
	}

	listing := map[string]interface{}{
		"kind": "Listing",
		"data": map[string]interface{}{
			"children": children,
			"after":    "",
			"before":   "",
		},
	}
	json.NewEncoder(w).Encode(listing)
}

// handleComments handles GET /r/{subreddit}/comments/{postID}
func (m *MockServer) handleComments(w http.ResponseWriter, r *http.Request) {
	subreddit := extractSubreddit(r.URL.Path)
	postID := extractPostIDFromPath(r.URL.Path)

	if subreddit == "" || postID == "" {
		http.Error(w, "Invalid comments path", http.StatusBadRequest)
		return
	}

	commentData, ok := m.comments[subreddit][postID]
	if !ok {
		// Return empty listings
		response := []interface{}{
			map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": []interface{}{},
				},
			},
			map[string]interface{}{
				"kind": "Listing",
				"data": map[string]interface{}{
					"children": []interface{}{},
					"after":    "",
					"before":   "",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Build post listing
	postListing := map[string]interface{}{
		"kind": "Listing",
		"data": map[string]interface{}{
			"children": []interface{}{
				map[string]interface{}{
					"kind": "t3",
					"data": commentData.Post,
				},
			},
		},
	}

	// Build comments listing
	commentChildren := make([]interface{}, len(commentData.Comments))
	for i, comment := range commentData.Comments {
		commentChildren[i] = buildCommentThing(comment)
	}

	commentsListing := map[string]interface{}{
		"kind": "Listing",
		"data": map[string]interface{}{
			"children": commentChildren,
			"after":    "",
			"before":   "",
		},
	}

	// Reddit returns [postListing, commentsListing]
	response := []interface{}{postListing, commentsListing}
	json.NewEncoder(w).Encode(response)
}

// buildCommentThing recursively builds a comment Thing with nested replies.
func buildCommentThing(comment *types.Comment) map[string]interface{} {
	// Build replies listing if there are any
	var replies interface{}
	if len(comment.Replies) > 0 {
		replyChildren := make([]interface{}, len(comment.Replies))
		for i, reply := range comment.Replies {
			replyChildren[i] = buildCommentThing(reply)
		}
		replies = map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": replyChildren,
			},
		}
	} else {
		// Empty string for no replies
		replies = ""
	}

	// Create a copy of the comment data to include replies
	commentData := map[string]interface{}{
		"id":           comment.ID,
		"name":         comment.Name,
		"author":       comment.Author,
		"body":         comment.Body,
		"body_html":    comment.BodyHTML,
		"score":        comment.Score,
		"ups":          comment.Ups,
		"downs":        comment.Downs,
		"created":      comment.Created.Created,
		"created_utc":  comment.Created.CreatedUTC,
		"link_id":      comment.LinkID,
		"parent_id":    comment.ParentID,
		"subreddit":    comment.Subreddit,
		"subreddit_id": comment.SubredditID,
		"replies":      replies,
	}

	// Add optional fields if present
	if comment.Likes != nil {
		commentData["likes"] = *comment.Likes
	}
	if comment.Distinguished != nil {
		commentData["distinguished"] = *comment.Distinguished
	}

	return map[string]interface{}{
		"kind": "t1",
		"data": commentData,
	}
}

// writeEmptyListing writes an empty Listing response.
func (m *MockServer) writeEmptyListing(w http.ResponseWriter) {
	listing := map[string]interface{}{
		"kind": "Listing",
		"data": map[string]interface{}{
			"children": []interface{}{},
			"after":    "",
			"before":   "",
		},
	}
	json.NewEncoder(w).Encode(listing)
}

// extractSubreddit extracts the subreddit name from a path like /r/golang/hot
func extractSubreddit(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "r" {
		return parts[1]
	}
	return ""
}

// extractPostIDFromPath extracts the post ID from a comments path like /r/golang/comments/abc123/...
func extractPostIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Format: /r/{subreddit}/comments/{postID}/...
	if len(parts) >= 4 && parts[0] == "r" && parts[2] == "comments" {
		return parts[3]
	}
	return ""
}
