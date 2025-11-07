package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

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
// Unconfigured endpoints return empty Listings. Error responses can be configured with WithError,
// WithStatusCode, WithMalformedJSON, WithEmptyResponse, or WithTimeout.
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
// Error scenario example:
//
//	server := testutil.NewMockServer().
//	    WithStatusCode(http.StatusInternalServerError).
//	    Start()
//	defer server.Close()
//	// All requests will return 500 Internal Server Error
//
// Pagination example:
//
//	pages := map[string][]*types.Post{
//	    "": {post1, post2, post3},           // First page (no after param)
//	    "t3_post3": {post4, post5, post6},  // Second page (after=t3_post3)
//	}
//	server := testutil.NewMockServer().
//	    WithPaginatedPosts("golang", "hot", pages).
//	    Start()
//	defer server.Close()
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

	// Error scenario configuration
	statusCode     int                                            // Global status code override (0 means no override)
	timeout        time.Duration                                  // Delay before responding (0 means no delay)
	malformedJSON  bool                                           // Return malformed JSON
	emptyResponse  bool                                           // Return empty response body
	paginatedPosts map[string]map[string]map[string][]*types.Post // [subreddit][sort][after]posts
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
		posts:          make(map[string]map[string][]*types.Post),
		comments:       make(map[string]map[string]*CommentData),
		subreddits:     make(map[string]*types.SubredditData),
		errors:         make(map[string]*ErrorConfig),
		paginatedPosts: make(map[string]map[string]map[string][]*types.Post),
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

// WithStatusCode configures the mock server to return a specific HTTP status code for all requests.
// This is useful for testing error handling and edge cases.
// Pass 0 to disable the status code override.
// Returns the MockServer for method chaining.
//
// Example:
//
//	server := testutil.NewMockServer().
//	    WithStatusCode(http.StatusServiceUnavailable).
//	    Start()
//	defer server.Close()
//	// All requests will return 503 Service Unavailable
func (m *MockServer) WithStatusCode(code int) *MockServer {
	m.statusCode = code
	return m
}

// WithTimeout configures the mock server to delay responses by the specified duration.
// This is useful for testing timeout handling and network latency scenarios.
// Pass 0 to disable the timeout.
// Returns the MockServer for method chaining.
//
// Example:
//
//	server := testutil.NewMockServer().
//	    WithTimeout(2 * time.Second).
//	    Start()
//	defer server.Close()
//	// All requests will be delayed by 2 seconds before responding
func (m *MockServer) WithTimeout(duration time.Duration) *MockServer {
	m.timeout = duration
	return m
}

// WithMalformedJSON configures the mock server to return malformed JSON in responses.
// This is useful for testing JSON parsing error handling.
// Returns the MockServer for method chaining.
//
// Example:
//
//	server := testutil.NewMockServer().
//	    WithMalformedJSON().
//	    Start()
//	defer server.Close()
//	// All requests will return invalid JSON
func (m *MockServer) WithMalformedJSON() *MockServer {
	m.malformedJSON = true
	return m
}

// WithEmptyResponse configures the mock server to return empty response bodies.
// This is useful for testing handling of unexpected empty responses.
// The server will still return a 200 OK status code with standard headers.
// Returns the MockServer for method chaining.
//
// Example:
//
//	server := testutil.NewMockServer().
//	    WithEmptyResponse().
//	    Start()
//	defer server.Close()
//	// All requests will return 200 OK with an empty body
func (m *MockServer) WithEmptyResponse() *MockServer {
	m.emptyResponse = true
	return m
}

// WithPaginatedPosts configures paginated posts for a specific subreddit and sort order.
// The pages map uses the "after" parameter as the key, with an empty string for the first page.
// Each page should contain the posts to return for that pagination state.
// The server will automatically set the "after" field in the response to the fullname of the last post.
// Returns the MockServer for method chaining.
//
// Example:
//
//	post1 := testutil.NewPostBuilder().WithID("post1").WithTitle("First").Build()
//	post2 := testutil.NewPostBuilder().WithID("post2").WithTitle("Second").Build()
//	post3 := testutil.NewPostBuilder().WithID("post3").WithTitle("Third").Build()
//	post4 := testutil.NewPostBuilder().WithID("post4").WithTitle("Fourth").Build()
//
//	pages := map[string][]*types.Post{
//	    "":          {post1, post2},      // First page (no after param)
//	    "t3_post2": {post3, post4},       // Second page (after=t3_post2)
//	}
//
//	server := testutil.NewMockServer().
//	    WithPaginatedPosts("golang", "hot", pages).
//	    Start()
//	defer server.Close()
func (m *MockServer) WithPaginatedPosts(subreddit, sort string, pages map[string][]*types.Post) *MockServer {
	if m.paginatedPosts[subreddit] == nil {
		m.paginatedPosts[subreddit] = make(map[string]map[string][]*types.Post)
	}
	if m.paginatedPosts[subreddit][sort] == nil {
		m.paginatedPosts[subreddit][sort] = make(map[string][]*types.Post)
	}
	m.paginatedPosts[subreddit][sort] = pages
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
	// Apply timeout if configured
	if m.timeout > 0 {
		time.Sleep(m.timeout)
	}

	// Set standard Reddit API headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Ratelimit-Remaining", "60")
	w.Header().Set("X-Ratelimit-Reset", "60")

	// Handle global status code override
	if m.statusCode != 0 {
		w.WriteHeader(m.statusCode)
		errorData := map[string]interface{}{
			"error":   http.StatusText(m.statusCode),
			"message": "Configured error response",
		}
		json.NewEncoder(w).Encode(errorData)
		return
	}

	// Handle malformed JSON
	if m.malformedJSON {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))
		return
	}

	// Handle empty response
	if m.emptyResponse {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := r.URL.Path

	// Check for configured errors for specific paths
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
// Note: The Reddit API returns account data directly, not wrapped in a Thing structure.
func (m *MockServer) handleAccount(w http.ResponseWriter, r *http.Request) {
	if m.account == nil {
		http.Error(w, "Account not configured", http.StatusNotFound)
		return
	}

	// Return account data directly (not wrapped in a Thing)
	json.NewEncoder(w).Encode(m.account)
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

	// Check if pagination is configured for this subreddit/sort
	after := r.URL.Query().Get("after")
	if paginatedPages, hasPagination := m.paginatedPosts[subreddit][sort]; hasPagination {
		posts, pageExists := paginatedPages[after]
		if !pageExists {
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

		// Determine the "after" value for the next page
		var nextAfter string
		if len(posts) > 0 {
			lastPost := posts[len(posts)-1]
			nextAfter = "t3_" + lastPost.ID
			// Check if there's actually a next page configured
			if _, hasNext := paginatedPages[nextAfter]; !hasNext {
				nextAfter = "" // No next page
			}
		}

		listing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": children,
				"after":    nextAfter,
				"before":   after,
			},
		}
		json.NewEncoder(w).Encode(listing)
		return
	}

	// Fall back to non-paginated posts
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

// NewCustomResponseServer creates a test server with a custom HTTP handler.
// This is useful for testing edge cases that require complete control over the HTTP response.
// The handler receives the standard http.ResponseWriter and http.Request parameters and can
// write any response. The returned server should be closed with defer server.Close().
//
// This helper wraps httptest.NewServer but adds standard Reddit API headers to the handler
// for consistency with other test servers.
//
// Example:
//
//	server := testutil.NewCustomResponseServer(func(w http.ResponseWriter, r *http.Request) {
//	    w.Header().Set("Content-Type", "application/json")
//	    w.WriteHeader(http.StatusOK)
//	    // Write partial JSON to test stream errors
//	    w.Write([]byte(`{"kind": "Listing", "data": {"children": [`))
//	    // Optionally hijack connection to simulate network issues
//	    if hj, ok := w.(http.Hijacker); ok {
//	        conn, _, _ := hj.Hijack()
//	        conn.Close()
//	    }
//	})
//	defer server.Close()
//
//	// Use server.URL in your client
//	client := reddit.New(server.URL, "test-agent")
func NewCustomResponseServer(handler http.HandlerFunc) *httptest.Server {
	// Wrap the handler to add standard headers
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set standard Reddit API headers before calling custom handler
		// Custom handler can override these if needed
		w.Header().Set("X-Ratelimit-Remaining", "60")
		w.Header().Set("X-Ratelimit-Reset", "60")

		// Call the custom handler
		handler(w, r)
	})

	return httptest.NewServer(wrappedHandler)
}
