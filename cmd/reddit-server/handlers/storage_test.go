package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// setupStorageTest creates a fresh in-memory SQLite store and Handlers for testing.
// Returns the Handlers, Store, and a cleanup function to close the store.
func setupStorageTest(t *testing.T) (*Handlers, storage.Store, func()) {
	// Create in-memory SQLite store
	st, err := storage.New(context.Background(), storage.Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	if err != nil {
		t.Fatalf("failed to create in-memory storage: %v", err)
	}

	// Create handlers with nil Reddit client and the test store
	h := NewHandlers(nil, st, nil)

	// Return handlers, store, and cleanup function
	cleanup := func() {
		if err := st.Close(); err != nil {
			t.Errorf("failed to close store: %v", err)
		}
	}

	return h, st, cleanup
}

// TestSavePost_Success tests successful save of a valid post
func TestSavePost_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	post := &types.Post{
		ThingData: types.ThingData{
			ID:   "abc123",
			Name: "t3_abc123",
		},
		Votable: types.Votable{
			Score: 100,
		},
		Title:       "Test Post",
		Author:      "testuser",
		Subreddit:   "golang",
		NumComments: 42,
	}

	// Make request to save post (wrapped in savePostRequest)
	reqBody := map[string]interface{}{"post": post}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SavePost(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Errorf("SavePost() status = %d, want 200 or 201, body: %s", w.Code, w.Body.String())
	}

	// Verify the post was actually saved
	savedPost, err := st.GetPost(context.Background(), "abc123")
	if err != nil {
		t.Errorf("GetPost() failed: %v", err)
	}
	if savedPost == nil {
		t.Error("SavePost() did not actually save the post")
	}
	if savedPost.ID != "abc123" {
		t.Errorf("SavePost() saved post with wrong ID: %s", savedPost.ID)
	}
}

// TestSavePost_InvalidJSON tests rejection of malformed JSON
func TestSavePost_InvalidJSON(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SavePost(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("SavePost() with invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	if !strings.Contains(w.Body.String(), "error") {
		t.Errorf("SavePost() response should contain error message, got: %s", w.Body.String())
	}
}

// TestSavePost_UpsertBehavior tests that saving the same post twice updates it
func TestSavePost_UpsertBehavior(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	post := &types.Post{
		ThingData: types.ThingData{
			ID:   "def456",
			Name: "t3_def456",
		},
		Votable: types.Votable{
			Score: 50,
		},
		Title:     "Original Post",
		Author:    "user1",
		Subreddit: "golang",
	}

	// Save the post first time
	reqBody := map[string]interface{}{"post": post}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SavePost(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("first SavePost() failed with status %d", w.Code)
	}

	// Update the post with new score
	post.Score = 150
	post.Title = "Updated Post"
	reqBody = map[string]interface{}{"post": post}
	body, err = json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.SavePost(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("second SavePost() failed with status %d", w.Code)
	}

	// Verify the post was updated
	savedPost, err := st.GetPost(context.Background(), "def456")
	if err != nil {
		t.Fatalf("GetPost() failed: %v", err)
	}
	if savedPost.Score != 150 {
		t.Errorf("SavePost() upsert didn't update score: got %d, want 150", savedPost.Score)
	}
	if savedPost.Title != "Updated Post" {
		t.Errorf("SavePost() upsert didn't update title: got %s, want 'Updated Post'", savedPost.Title)
	}
}

// TestListSavedPosts_Empty tests listing posts when none exist
func TestListSavedPosts_Empty(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts", nil)
	w := httptest.NewRecorder()

	h.ListSavedPosts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListSavedPosts() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("ListSavedPosts() response is not valid JSON: %v", err)
	}

	// Check that posts array exists and is empty
	posts, ok := response["posts"].([]interface{})
	if !ok {
		t.Errorf("ListSavedPosts() response missing 'posts' array")
	}
	if len(posts) != 0 {
		t.Errorf("ListSavedPosts() empty store returned %d posts, want 0", len(posts))
	}
}

// TestListSavedPosts_WithFilters tests filtering posts by subreddit and author
func TestListSavedPosts_WithFilters(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Save posts in different subreddits
	posts := []*types.Post{
		{
			ThingData: types.ThingData{ID: "post1", Name: "t3_post1"},
			Votable:   types.Votable{Score: 10},
			Title:     "Go Article", Author: "alice", Subreddit: "golang",
		},
		{
			ThingData: types.ThingData{ID: "post2", Name: "t3_post2"},
			Votable:   types.Votable{Score: 20},
			Title:     "Go Blog", Author: "bob", Subreddit: "golang",
		},
		{
			ThingData: types.ThingData{ID: "post3", Name: "t3_post3"},
			Votable:   types.Votable{Score: 30},
			Title:     "Python News", Author: "alice", Subreddit: "python",
		},
	}

	for _, post := range posts {
		if err := st.UpsertPost(context.Background(), post); err != nil {
			t.Fatalf("UpsertPost() failed: %v", err)
		}
	}

	// Test filtering by subreddit
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts?subreddit=golang", nil)
	w := httptest.NewRecorder()
	h.ListSavedPosts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListSavedPosts() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v, body: %s", err, w.Body.String())
	}
	posts_array := response["posts"].([]interface{})
	if len(posts_array) != 2 {
		t.Errorf("ListSavedPosts() with subreddit filter returned %d posts, want 2", len(posts_array))
	}

	// Test filtering by author
	req = httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts?author=alice", nil)
	w = httptest.NewRecorder()
	h.ListSavedPosts(w, req)

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v, body: %s", err, w.Body.String())
	}
	posts_array = response["posts"].([]interface{})
	if len(posts_array) != 2 {
		t.Errorf("ListSavedPosts() with author filter returned %d posts, want 2", len(posts_array))
	}
}

// TestListSavedPosts_Pagination tests pagination with limit and offset
func TestListSavedPosts_Pagination(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Save 10 posts
	for i := 1; i <= 10; i++ {
		post := &types.Post{
			ThingData: types.ThingData{ID: formatID(i), Name: "t3_" + formatID(i)},
			Votable:   types.Votable{Score: i * 10},
			Title:     "Post " + formatID(i),
			Author:    "user1",
			Subreddit: "test",
		}
		if err := st.UpsertPost(context.Background(), post); err != nil {
			t.Fatalf("UpsertPost() failed: %v", err)
		}
	}

	// Test with limit
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts?limit=5", nil)
	w := httptest.NewRecorder()
	h.ListSavedPosts(w, req)

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v, body: %s", err, w.Body.String())
	}
	posts_array := response["posts"].([]interface{})
	if len(posts_array) != 5 {
		t.Errorf("ListSavedPosts() with limit=5 returned %d posts, want 5", len(posts_array))
	}

	// Test with offset
	req = httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts?limit=5&offset=5", nil)
	w = httptest.NewRecorder()
	h.ListSavedPosts(w, req)

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v, body: %s", err, w.Body.String())
	}
	posts_array = response["posts"].([]interface{})
	if len(posts_array) != 5 {
		t.Errorf("ListSavedPosts() with offset=5 returned %d posts, want 5", len(posts_array))
	}
}

// TestListSavedPosts_Sorting tests sorting by score and date
func TestListSavedPosts_Sorting(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Save posts with different scores
	posts := []*types.Post{
		{
			ThingData: types.ThingData{ID: "s1", Name: "t3_s1"},
			Votable:   types.Votable{Score: 50},
			Title:     "Mid Score", Author: "user", Subreddit: "test",
		},
		{
			ThingData: types.ThingData{ID: "s2", Name: "t3_s2"},
			Votable:   types.Votable{Score: 100},
			Title:     "High Score", Author: "user", Subreddit: "test",
		},
		{
			ThingData: types.ThingData{ID: "s3", Name: "t3_s3"},
			Votable:   types.Votable{Score: 10},
			Title:     "Low Score", Author: "user", Subreddit: "test",
		},
	}

	for _, post := range posts {
		if err := st.UpsertPost(context.Background(), post); err != nil {
			t.Fatalf("UpsertPost() failed: %v", err)
		}
	}

	// Test sorting by score descending
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts?sort_by=score&sort_dir=desc", nil)
	w := httptest.NewRecorder()
	h.ListSavedPosts(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListSavedPosts() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v, body: %s", err, w.Body.String())
	}
	posts_array := response["posts"].([]interface{})
	if len(posts_array) != 3 {
		t.Errorf("ListSavedPosts() returned %d posts, want 3", len(posts_array))
	}
}

// TestGetSavedPost_Success tests retrieving an existing post
func TestGetSavedPost_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	post := &types.Post{
		ThingData: types.ThingData{ID: "retrieve1", Name: "t3_retrieve1"},
		Votable:   types.Votable{Score: 123},
		Title:     "Retrieve Test", Author: "author1", Subreddit: "test",
	}

	if err := st.UpsertPost(context.Background(), post); err != nil {
		t.Fatalf("UpsertPost() failed: %v", err)
	}

	// Request the saved post
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts/retrieve1", nil)
	w := httptest.NewRecorder()

	h.GetSavedPost(w, req, "retrieve1")

	if w.Code != http.StatusOK {
		t.Errorf("GetSavedPost() status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response types.Post
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("GetSavedPost() response is not valid JSON: %v", err)
	}

	if response.ID != "retrieve1" {
		t.Errorf("GetSavedPost() returned post with wrong ID: %s", response.ID)
	}
	if response.Score != 123 {
		t.Errorf("GetSavedPost() returned post with wrong score: %d", response.Score)
	}
}

// TestGetSavedPost_NotFound tests 404 for missing post
func TestGetSavedPost_NotFound(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts/nonexistent", nil)
	w := httptest.NewRecorder()

	h.GetSavedPost(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("GetSavedPost() status = %d, want %d", w.Code, http.StatusNotFound)
	}

	if !strings.Contains(w.Body.String(), "error") || !strings.Contains(strings.ToLower(w.Body.String()), "not found") {
		t.Errorf("GetSavedPost() response should contain not found error, got: %s", w.Body.String())
	}
}

// TestDeleteSavedPost_Success tests deletion of an existing post
func TestDeleteSavedPost_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	post := &types.Post{
		ThingData: types.ThingData{ID: "delete1", Name: "t3_delete1"},
		Votable:   types.Votable{Score: 50},
		Title:     "To Delete", Author: "user", Subreddit: "test",
	}

	if err := st.UpsertPost(context.Background(), post); err != nil {
		t.Fatalf("UpsertPost() failed: %v", err)
	}

	// Delete the post
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/storage/posts/delete1", nil)
	w := httptest.NewRecorder()

	h.DeleteSavedPost(w, req, "delete1")

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DeleteSavedPost() status = %d, want 200 or 204", w.Code)
	}

	// Verify the post is gone
	_, err := st.GetPost(context.Background(), "delete1")
	if err == nil {
		t.Error("DeleteSavedPost() did not actually delete the post")
	}
}

// TestDeleteSavedPost_NotFound tests successful deletion with idempotent behavior
// DELETE is idempotent, so deleting a non-existent post returns 200 success
func TestDeleteSavedPost_NotFound(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/storage/posts/nonexistent", nil)
	w := httptest.NewRecorder()

	h.DeleteSavedPost(w, req, "nonexistent")

	// DELETE is idempotent - deleting a non-existent post succeeds (200)
	if w.Code != http.StatusOK {
		t.Errorf("DeleteSavedPost() status = %d, want %d (idempotent DELETE)", w.Code, http.StatusOK)
	}
}

// TestSaveComments_Success tests saving comments for a post
func TestSaveComments_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// First save a post
	post := &types.Post{
		ThingData: types.ThingData{ID: "post_for_comments", Name: "t3_post_for_comments"},
		Votable:   types.Votable{Score: 100},
		Title:     "Post with Comments", Author: "user", Subreddit: "test",
	}
	if err := st.UpsertPost(context.Background(), post); err != nil {
		t.Fatalf("UpsertPost() failed: %v", err)
	}

	// Save comments
	comments := []*types.Comment{
		{
			ThingData: types.ThingData{ID: "c1", Name: "t1_c1"},
			Votable:   types.Votable{Score: 10},
			Author:    "commenter1",
			Body:      "Great post!",
			LinkID:    "t3_post_for_comments",
			ParentID:  "t3_post_for_comments",
		},
		{
			ThingData: types.ThingData{ID: "c2", Name: "t1_c2"},
			Votable:   types.Votable{Score: 5},
			Author:    "commenter2",
			Body:      "I agree!",
			LinkID:    "t3_post_for_comments",
			ParentID:  "t1_c1",
		},
	}

	body, err := json.Marshal(map[string]interface{}{
		"post_id":  "post_for_comments",
		"comments": comments,
	})
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts/post_for_comments/comments", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SaveComments(w, req, "post_for_comments")

	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Errorf("SaveComments() status = %d, want 200 or 201, body: %s", w.Code, w.Body.String())
	}

	// Verify comments were saved
	for _, comment := range comments {
		saved, err := st.GetComment(context.Background(), comment.ID)
		if err != nil {
			t.Errorf("GetComment() failed: %v", err)
		}
		if saved == nil {
			t.Errorf("SaveComments() did not save comment %s", comment.ID)
		}
	}
}

// TestGetCommentTree_Success tests retrieving comment tree for a post
func TestGetCommentTree_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Save a post
	post := &types.Post{
		ThingData: types.ThingData{ID: "tree_post", Name: "t3_tree_post"},
		Votable:   types.Votable{Score: 100},
		Title:     "Tree Test", Author: "user", Subreddit: "test",
	}
	if err := st.UpsertPost(context.Background(), post); err != nil {
		t.Fatalf("UpsertPost() failed: %v", err)
	}

	// Save comments
	comments := []*types.Comment{
		{
			ThingData: types.ThingData{ID: "tc1", Name: "t1_tc1"},
			Votable:   types.Votable{Score: 20},
			Author:    "user1",
			Body:      "Top comment",
			LinkID:    "t3_tree_post",
			ParentID:  "t3_tree_post",
		},
		{
			ThingData: types.ThingData{ID: "tc2", Name: "t1_tc2"},
			Votable:   types.Votable{Score: 15},
			Author:    "user2",
			Body:      "Another comment",
			LinkID:    "t3_tree_post",
			ParentID:  "t3_tree_post",
		},
	}
	for _, c := range comments {
		if err := st.UpsertComment(context.Background(), c); err != nil {
			t.Fatalf("UpsertComment() failed: %v", err)
		}
	}

	// Get the comment tree
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts/tree_post/comments", nil)
	w := httptest.NewRecorder()

	h.GetCommentTree(w, req, "tree_post")

	if w.Code != http.StatusOK {
		t.Errorf("GetCommentTree() status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("GetCommentTree() response is not valid JSON: %v", err)
	}

	commentsArray := response["comments"].([]interface{})
	if len(commentsArray) != 2 {
		t.Errorf("GetCommentTree() returned %d comments, want 2", len(commentsArray))
	}
}

// TestGetCommentTree_WithDepth tests depth limiting in comment tree retrieval
func TestGetCommentTree_WithDepth(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Save a post
	post := &types.Post{
		ThingData: types.ThingData{ID: "depth_post", Name: "t3_depth_post"},
		Votable:   types.Votable{Score: 100},
		Title:     "Depth Test", Author: "user", Subreddit: "test",
	}
	if err := st.UpsertPost(context.Background(), post); err != nil {
		t.Fatalf("UpsertPost() failed: %v", err)
	}

	// Save a comment
	comment := &types.Comment{
		ThingData: types.ThingData{ID: "depth_c1", Name: "t1_depth_c1"},
		Votable:   types.Votable{Score: 10},
		Author:    "user1",
		Body:      "Comment",
		LinkID:    "t3_depth_post",
		ParentID:  "t3_depth_post",
	}
	if err := st.UpsertComment(context.Background(), comment); err != nil {
		t.Fatalf("UpsertComment() failed: %v", err)
	}

	// Request with depth limit
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts/depth_post/comments?max_depth=1", nil)
	w := httptest.NewRecorder()

	h.GetCommentTree(w, req, "depth_post")

	if w.Code != http.StatusOK {
		t.Errorf("GetCommentTree() status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v, body: %s", err, w.Body.String())
	}
	commentsArray := response["comments"].([]interface{})
	if len(commentsArray) != 1 {
		t.Errorf("GetCommentTree() with depth filter returned %d comments, want 1", len(commentsArray))
	}
}

// TestBulkSaveFromSubreddit_Success tests bulk saving posts from a subreddit
func TestBulkSaveFromSubreddit_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Create mock Reddit client that returns posts
	postsToReturn := &types.PostsResponse{
		Posts: []*types.Post{
			{
				ThingData: types.ThingData{ID: "bulk1", Name: "t3_bulk1"},
				Votable:   types.Votable{Score: 100},
				Title:     "Bulk Post 1", Author: "user1", Subreddit: "golang",
			},
			{
				ThingData: types.ThingData{ID: "bulk2", Name: "t3_bulk2"},
				Votable:   types.Votable{Score: 50},
				Title:     "Bulk Post 2", Author: "user2", Subreddit: "golang",
			},
		},
	}

	mock := &mockRedditClient{
		hotResponse: postsToReturn,
		hotError:    nil,
	}

	h.client = mock

	// Make request to bulk save
	body, err := json.Marshal(map[string]interface{}{
		"subreddit": "golang",
		"limit":     10,
	})
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/bulk/subreddit", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.BulkSaveFromSubreddit(w, req)

	// Should return success (200 or 201)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("BulkSaveFromSubreddit() status = %d, want 200 or 201, body: %s", w.Code, w.Body.String())
	}

	// Verify posts were saved
	count := 0
	for _, p := range postsToReturn.Posts {
		saved, err := st.GetPost(context.Background(), p.ID)
		if err == nil && saved != nil {
			count++
		}
	}
	if count != len(postsToReturn.Posts) {
		t.Errorf("BulkSaveFromSubreddit() saved %d posts, want %d", count, len(postsToReturn.Posts))
	}
}

// TestBulkSaveFromSubreddit_InvalidRequest tests rejection of bad input
func TestBulkSaveFromSubreddit_InvalidRequest(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/bulk/subreddit", strings.NewReader("{bad json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.BulkSaveFromSubreddit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("BulkSaveFromSubreddit() with invalid JSON status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	if !strings.Contains(w.Body.String(), "error") {
		t.Errorf("BulkSaveFromSubreddit() response should contain error, got: %s", w.Body.String())
	}
}

// TestGetStorageStats_Success tests retrieval of storage statistics
func TestGetStorageStats_Success(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	// Save some posts and comments
	for i := 1; i <= 3; i++ {
		post := &types.Post{
			ThingData: types.ThingData{ID: formatID(i), Name: "t3_" + formatID(i)},
			Votable:   types.Votable{Score: i * 10},
			Title:     "Post " + formatID(i),
			Author:    "user", Subreddit: "test",
		}
		if err := st.UpsertPost(context.Background(), post); err != nil {
			t.Fatalf("UpsertPost() failed: %v", err)
		}
	}

	// Verify posts were saved before checking stats
	posts, err := st.ListPosts(context.Background(), &storage.ListPostsOptions{Limit: 100})
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 3 {
		t.Fatalf("Expected 3 posts after save, got %d", len(posts))
	}

	// Get stats
	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/stats", nil)
	w := httptest.NewRecorder()

	h.GetStorageStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetStorageStats() status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("GetStorageStats() response is not valid JSON: %v", err)
	}

	postCount, ok := response["post_count"]
	if !ok {
		t.Errorf("GetStorageStats() response missing 'post_count' field")
	}
	if postCountFloat, ok := postCount.(float64); ok {
		if int64(postCountFloat) != 3 {
			t.Errorf("GetStorageStats() returned post_count = %d, want 3", int64(postCountFloat))
		}
	}
}

// TestSavePost_MethodNotAllowed tests that non-POST methods are rejected
func TestSavePost_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts", nil)
	w := httptest.NewRecorder()

	h.SavePost(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("SavePost() with GET status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestListSavedPosts_MethodNotAllowed tests that non-GET methods are rejected
func TestListSavedPosts_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts", nil)
	w := httptest.NewRecorder()

	h.ListSavedPosts(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("ListSavedPosts() with POST status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestGetSavedPost_MethodNotAllowed tests that non-GET methods are rejected
func TestGetSavedPost_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts/abc123", nil)
	w := httptest.NewRecorder()

	h.GetSavedPost(w, req, "abc123")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GetSavedPost() with POST status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestDeleteSavedPost_MethodNotAllowed tests that non-DELETE methods are rejected
func TestDeleteSavedPost_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts/abc123", nil)
	w := httptest.NewRecorder()

	h.DeleteSavedPost(w, req, "abc123")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("DeleteSavedPost() with GET status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestSaveComments_MethodNotAllowed tests that non-POST methods are rejected
func TestSaveComments_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/posts/abc123/comments", nil)
	w := httptest.NewRecorder()

	h.SaveComments(w, req, "abc123")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("SaveComments() with GET status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestGetCommentTree_MethodNotAllowed tests that non-GET methods are rejected
func TestGetCommentTree_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts/abc123/comments", nil)
	w := httptest.NewRecorder()

	h.GetCommentTree(w, req, "abc123")

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GetCommentTree() with POST status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestBulkSaveFromSubreddit_MethodNotAllowed tests that non-POST methods are rejected
func TestBulkSaveFromSubreddit_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/bulk/subreddit", nil)
	w := httptest.NewRecorder()

	h.BulkSaveFromSubreddit(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("BulkSaveFromSubreddit() with GET status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestGetStorageStats_MethodNotAllowed tests that non-GET methods are rejected
func TestGetStorageStats_MethodNotAllowed(t *testing.T) {
	h, _, cleanup := setupStorageTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/stats", nil)
	w := httptest.NewRecorder()

	h.GetStorageStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GetStorageStats() with POST status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// TestConcurrentPostSaves tests concurrent saves to ensure thread-safety
func TestConcurrentPostSaves(t *testing.T) {
	h, st, cleanup := setupStorageTest(t)
	defer cleanup()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string

	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			post := &types.Post{
				ThingData: types.ThingData{
					ID:   formatID(id),
					Name: "t3_" + formatID(id),
				},
				Title:     fmt.Sprintf("Concurrent Post %d", id),
				Subreddit: "test",
				Author:    "testuser",
				Votable:   types.Votable{Score: id * 10},
			}

			reqBody := map[string]interface{}{"post": post}
			body, err := json.Marshal(reqBody)
			if err != nil {
				mu.Lock()
				failures = append(failures, fmt.Sprintf("post %d: marshal error: %v", id, err))
				mu.Unlock()
				return
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/posts", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.SavePost(w, req)

			if w.Code != http.StatusCreated && w.Code != http.StatusOK {
				mu.Lock()
				failures = append(failures, fmt.Sprintf("post %d: status %d, body: %s", id, w.Code, w.Body.String()))
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if len(failures) > 0 {
		t.Errorf("Concurrent saves had %d failures:\n%s", len(failures), strings.Join(failures, "\n"))
	}

	// Verify all posts were saved
	ctx := context.Background()
	opts := &storage.ListPostsOptions{Limit: 100}
	posts, err := st.ListPosts(ctx, opts)
	if err != nil {
		t.Fatalf("ListPosts() failed: %v", err)
	}
	if len(posts) != 10 {
		t.Errorf("Expected 10 posts after concurrent saves, got %d", len(posts))
	}
}

// formatID returns a zero-padded ID string for testing.
func formatID(i int) string {
	return fmt.Sprintf("%02d", i)
}
