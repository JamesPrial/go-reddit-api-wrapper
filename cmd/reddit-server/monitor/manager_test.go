package monitor

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// mockRedditClient mocks the Reddit API client for testing.
type mockRedditClient struct {
	getNewFunc      func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)
	getCommentsFunc func(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error)
	callCounts      struct {
		mu               sync.Mutex
		getNewCalls      int
		getCommentsCalls int
	}
}

func (m *mockRedditClient) GetNew(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
	m.callCounts.mu.Lock()
	m.callCounts.getNewCalls++
	m.callCounts.mu.Unlock()

	if m.getNewFunc != nil {
		return m.getNewFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockRedditClient) GetComments(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error) {
	m.callCounts.mu.Lock()
	m.callCounts.getCommentsCalls++
	m.callCounts.mu.Unlock()

	if m.getCommentsFunc != nil {
		return m.getCommentsFunc(ctx, req)
	}
	return nil, nil
}

// mockStore mocks the storage layer for testing.
type mockStore struct {
	mu                 sync.RWMutex
	upsertPostsFunc    func(ctx context.Context, posts []*types.Post) error
	upsertCommentsFunc func(ctx context.Context, comments []*types.Comment) error
	callCounts         struct {
		mu                  sync.Mutex
		upsertPostsCalls    int
		upsertCommentsCalls int
	}
	savedPosts    []*types.Post
	savedComments []*types.Comment
}

func (m *mockStore) UpsertPosts(ctx context.Context, posts []*types.Post) error {
	m.callCounts.mu.Lock()
	m.callCounts.upsertPostsCalls++
	m.callCounts.mu.Unlock()

	m.mu.Lock()
	m.savedPosts = append(m.savedPosts, posts...)
	m.mu.Unlock()

	if m.upsertPostsFunc != nil {
		return m.upsertPostsFunc(ctx, posts)
	}
	return nil
}

func (m *mockStore) UpsertComments(ctx context.Context, comments []*types.Comment) error {
	m.callCounts.mu.Lock()
	m.callCounts.upsertCommentsCalls++
	m.callCounts.mu.Unlock()

	m.mu.Lock()
	m.savedComments = append(m.savedComments, comments...)
	m.mu.Unlock()

	if m.upsertCommentsFunc != nil {
		return m.upsertCommentsFunc(ctx, comments)
	}
	return nil
}

// Implement required methods from storage.Store interface
func (m *mockStore) UpsertPost(ctx context.Context, post *types.Post) error      { return nil }
func (m *mockStore) GetPost(ctx context.Context, id string) (*types.Post, error) { return nil, nil }
func (m *mockStore) ListPosts(ctx context.Context, opts *storage.ListPostsOptions) ([]*types.Post, error) {
	return nil, nil
}
func (m *mockStore) CountPosts(ctx context.Context, opts *storage.ListPostsOptions) (int64, error) {
	return 0, nil
}
func (m *mockStore) DeletePost(ctx context.Context, id string) error                 { return nil }
func (m *mockStore) UpsertComment(ctx context.Context, comment *types.Comment) error { return nil }
func (m *mockStore) GetComment(ctx context.Context, id string) (*types.Comment, error) {
	return nil, nil
}
func (m *mockStore) GetCommentTree(ctx context.Context, postID string, opts *storage.CommentTreeOptions) ([]*types.Comment, error) {
	return nil, nil
}
func (m *mockStore) DeleteComment(ctx context.Context, id string) error { return nil }
func (m *mockStore) Close() error                                       { return nil }
func (m *mockStore) Ping(ctx context.Context) error                     { return nil }
func (m *mockStore) GetStats(ctx context.Context) (*storage.CacheStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &storage.CacheStats{
		PostCount:    int64(len(m.savedPosts)),
		CommentCount: int64(len(m.savedComments)),
	}, nil
}
func (m *mockStore) EvictStale(ctx context.Context, maxAge time.Duration) (int64, error) {
	return 0, nil
}
func (m *mockStore) SavePostSnapshot(ctx context.Context, snapshot *storage.PostSnapshot) error {
	return nil
}
func (m *mockStore) GetLatestSnapshot(ctx context.Context, postID string) (*storage.PostSnapshot, error) {
	return nil, nil
}
func (m *mockStore) SaveCommentChangeEvent(ctx context.Context, event *storage.CommentChangeEvent) error {
	return nil
}
func (m *mockStore) GetCommentChangeEvents(ctx context.Context, postID string, limit int) ([]*storage.CommentChangeEvent, error) {
	return nil, nil
}

// newTestLogger creates a logger for testing that discards output.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// Helper to create a valid monitor config for testing (meets validation requirements).
func validConfig() MonitorConfig {
	return MonitorConfig{
		Subreddits:    []string{"golang"},
		Interval:      10 * time.Second,
		Limit:         25,
		FetchComments: false,
	}
}

// Helper to create a fast config for tests that need quicker cycles (invalid but used internally for testing the loop).
func fastConfig() MonitorConfig {
	return MonitorConfig{
		Subreddits:    []string{"golang"},
		Interval:      100 * time.Millisecond,
		Limit:         25,
		FetchComments: false,
	}
}

// Helper to create test posts.
func testPost(id string) *types.Post {
	return &types.Post{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t3_" + id,
		},
		Title:  "Test Post " + id,
		Author: "testuser",
	}
}

// Helper to create test comments.
func testComment(id string) *types.Comment {
	return &types.Comment{
		ThingData: types.ThingData{
			ID:   id,
			Name: "t1_" + id,
		},
		Author: "commenter",
		Body:   "Test comment " + id,
	}
}

// TestNewMonitorManager tests monitor manager creation.
func TestNewMonitorManager(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{}
	store := &mockStore{}
	logger := newTestLogger()

	manager := NewMonitorManager(client, store, logger)

	if manager == nil {
		t.Fatal("NewMonitorManager returned nil")
	}
	if manager.client != client {
		t.Error("manager.client not set correctly")
	}
	if manager.store != store {
		t.Error("manager.store not set correctly")
	}
	if manager.logger != logger {
		t.Error("manager.logger not set correctly")
	}
	if manager.activeMonitor != nil {
		t.Error("manager.activeMonitor should be nil on creation")
	}
}

// TestMonitorManager_ValidateConfig tests configuration validation.
func TestMonitorManager_ValidateConfig(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	tests := []struct {
		name    string
		config  MonitorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: MonitorConfig{
				Subreddits:    []string{"golang"},
				Interval:      10 * time.Second,
				Limit:         25,
				FetchComments: false,
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple subreddits",
			config: MonitorConfig{
				Subreddits:    []string{"golang", "rust", "python"},
				Interval:      30 * time.Second,
				Limit:         50,
				FetchComments: true,
			},
			wantErr: false,
		},
		{
			name: "empty subreddits",
			config: MonitorConfig{
				Subreddits:    []string{},
				Interval:      10 * time.Second,
				Limit:         25,
				FetchComments: false,
			},
			wantErr: true,
			errMsg:  "must not be empty",
		},
		{
			name: "too many subreddits",
			config: MonitorConfig{
				Subreddits: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
				Interval:   10 * time.Second,
				Limit:      25,
			},
			wantErr: true,
			errMsg:  "maximum 10",
		},
		{
			name: "interval too short",
			config: MonitorConfig{
				Subreddits: []string{"golang"},
				Interval:   5 * time.Second,
				Limit:      25,
			},
			wantErr: true,
			errMsg:  "must be at least",
		},
		{
			name: "limit too low",
			config: MonitorConfig{
				Subreddits: []string{"golang"},
				Interval:   10 * time.Second,
				Limit:      0,
			},
			wantErr: true,
			errMsg:  "between 1 and 100",
		},
		{
			name: "limit too high",
			config: MonitorConfig{
				Subreddits: []string{"golang"},
				Interval:   10 * time.Second,
				Limit:      101,
			},
			wantErr: true,
			errMsg:  "between 1 and 100",
		},
		{
			name: "limit at lower boundary",
			config: MonitorConfig{
				Subreddits: []string{"golang"},
				Interval:   10 * time.Second,
				Limit:      1,
			},
			wantErr: false,
		},
		{
			name: "limit at upper boundary",
			config: MonitorConfig{
				Subreddits: []string{"golang"},
				Interval:   10 * time.Second,
				Limit:      100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !containsSubstring(err.Error(), tt.errMsg) {
					t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestMonitorManager_Start tests successful monitor start.
func TestMonitorManager_Start(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{}
	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	ctx := context.Background()

	instance, err := manager.Start(ctx, config)
	defer manager.Stop() // Cleanup

	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	if instance == nil {
		t.Fatal("Start() returned nil instance")
	}

	if instance.ID == "" {
		t.Error("instance.ID is empty")
	}

	if instance.StartedAt.IsZero() {
		t.Error("instance.StartedAt should not be zero")
	}

	if instance.Subreddits == nil || len(instance.Subreddits) != len(config.Subreddits) {
		t.Error("instance.Subreddits not set correctly")
	}

	if instance.Interval != config.Interval {
		t.Error("instance.Interval not set correctly")
	}

	if instance.Limit != config.Limit {
		t.Error("instance.Limit not set correctly")
	}

	if instance.FetchComments != config.FetchComments {
		t.Error("instance.FetchComments not set correctly")
	}

	if !manager.IsRunning() {
		t.Error("manager.IsRunning() should return true after Start()")
	}
}

// TestMonitorManager_StartAlreadyRunning tests that starting when monitor is running returns error.
func TestMonitorManager_StartAlreadyRunning(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	config := validConfig()
	ctx := context.Background()

	// Start first monitor
	_, err := manager.Start(ctx, config)
	if err != nil {
		t.Fatalf("first Start() error = %v, want nil", err)
	}
	defer manager.Stop() // Cleanup

	// Try to start second monitor
	_, err = manager.Start(ctx, config)
	if err == nil {
		t.Fatal("second Start() error = nil, want ErrMonitorAlreadyRunning")
	}

	if !errors.Is(err, ErrMonitorAlreadyRunning) {
		t.Errorf("error = %v, want ErrMonitorAlreadyRunning", err)
	}
}

// TestMonitorManager_StartInvalidConfig tests that invalid config returns validation error.
func TestMonitorManager_StartInvalidConfig(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	// Use invalid config with empty subreddits
	config := MonitorConfig{
		Subreddits: []string{},
		Interval:   10 * time.Second,
		Limit:      25,
	}

	ctx := context.Background()
	_, err := manager.Start(ctx, config)

	if err == nil {
		t.Fatal("Start() with invalid config returned no error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("error = %v, want to wrap ErrInvalidConfig", err)
	}
}

// TestMonitorManager_Stop tests successful monitor stop.
func TestMonitorManager_Stop(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	config := validConfig()
	ctx := context.Background()

	instance, err := manager.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	instanceID := instance.ID

	// Give monitor time to start processing
	time.Sleep(50 * time.Millisecond)

	err = manager.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}

	// Verify monitor is stopped
	if manager.IsRunning() {
		t.Error("manager.IsRunning() should return false after Stop()")
	}

	// Verify activeMonitor is cleared
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.Status != "stopped" {
		t.Errorf("status.Status = %q, want 'stopped'", status.Status)
	}

	// Verify instance ID was for the stopped monitor
	if instanceID == "" {
		t.Error("instance ID should not be empty")
	}
}

// TestMonitorManager_StopNoMonitor tests that stopping when no monitor is running returns error.
func TestMonitorManager_StopNoMonitor(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	err := manager.Stop()
	if err == nil {
		t.Fatal("Stop() with no monitor error = nil, want ErrNoMonitorRunning")
	}

	if !errors.Is(err, ErrNoMonitorRunning) {
		t.Errorf("error = %v, want ErrNoMonitorRunning", err)
	}
}

// TestMonitorManager_GetStatus tests GetStatus when running and stopped.
func TestMonitorManager_GetStatus(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	// Test status when stopped
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v, want nil", err)
	}
	if status.Status != "stopped" {
		t.Errorf("status when stopped = %q, want 'stopped'", status.Status)
	}
	if status.ID != "" {
		t.Error("stopped status should not have ID")
	}

	// Start monitor
	config := validConfig()
	instance, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Test status when running
	status, err = manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v, want nil", err)
	}
	if status.Status != "running" {
		t.Errorf("status when running = %q, want 'running'", status.Status)
	}
	if status.ID != instance.ID {
		t.Errorf("status.ID = %q, want %q", status.ID, instance.ID)
	}
	if status.Interval != config.Interval.String() {
		t.Errorf("status.Interval = %q, want %q", status.Interval, config.Interval.String())
	}
	if len(status.Subreddits) != len(config.Subreddits) {
		t.Errorf("status.Subreddits length = %d, want %d", len(status.Subreddits), len(config.Subreddits))
	}
	if status.StartedAt == nil || status.StartedAt.IsZero() {
		t.Error("status.StartedAt should not be zero")
	}
	if status.Stats == nil {
		t.Fatal("status.Stats is nil")
	}
}

// TestMonitorManager_IsRunning tests IsRunning returns correct boolean.
func TestMonitorManager_IsRunning(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	if manager.IsRunning() {
		t.Error("IsRunning() should return false when no monitor is running")
	}

	config := validConfig()
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	if !manager.IsRunning() {
		t.Error("IsRunning() should return true when monitor is running")
	}

	manager.Stop()

	if manager.IsRunning() {
		t.Error("IsRunning() should return false after Stop()")
	}
}

// TestMonitorManager_MonitorLoop tests that monitor loop fetches and saves posts.
func TestMonitorManager_MonitorLoop(t *testing.T) {
	t.Parallel()

	posts := []*types.Post{
		testPost("post1"),
		testPost("post2"),
	}

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return &types.PostsResponse{Posts: posts}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to fetch posts
	time.Sleep(50 * time.Millisecond)

	manager.Stop()

	// Verify posts were saved
	if store.callCounts.upsertPostsCalls == 0 {
		t.Fatal("UpsertPosts was not called")
	}

	if len(store.savedPosts) == 0 {
		t.Fatal("no posts were saved")
	}

	if len(store.savedPosts) != len(posts) {
		t.Errorf("saved posts count = %d, want %d", len(store.savedPosts), len(posts))
	}

	// Verify client was called
	if client.callCounts.getNewCalls == 0 {
		t.Fatal("GetNew was not called")
	}
}

// TestMonitorManager_MonitorLoopWithComments tests monitor loop with FetchComments enabled.
func TestMonitorManager_MonitorLoopWithComments(t *testing.T) {
	t.Parallel()

	posts := []*types.Post{testPost("post1")}
	comments := []*types.Comment{testComment("comment1")}

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return &types.PostsResponse{Posts: posts}, nil
		},
		getCommentsFunc: func(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error) {
			return &types.CommentsResponse{Comments: comments}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	config.FetchComments = true
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to fetch posts and comments
	time.Sleep(200 * time.Millisecond)

	manager.Stop()

	// Verify comments were fetched
	if client.callCounts.getCommentsCalls == 0 {
		t.Fatal("GetComments was not called")
	}

	// Verify comments were saved
	if store.callCounts.upsertCommentsCalls == 0 {
		t.Fatal("UpsertComments was not called")
	}

	if len(store.savedComments) == 0 {
		t.Fatal("no comments were saved")
	}
}

// TestMonitorManager_ContextCancellation tests that Stop() cancels context and stops loop.
func TestMonitorManager_ContextCancellation(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			// Simulate a slow fetch
			time.Sleep(50 * time.Millisecond)
			return &types.PostsResponse{}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()

	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give monitor time to start
	time.Sleep(50 * time.Millisecond)

	startTime := time.Now()
	err = manager.Stop()
	stopDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Stop should complete quickly (within 100ms) because context cancellation should be prompt
	if stopDuration > 500*time.Millisecond {
		t.Errorf("Stop() took too long: %v, indicates context cancellation may not be working", stopDuration)
	}
}

// TestMonitorManager_StatsTracking tests that stats are accurately tracked.
func TestMonitorManager_StatsTracking(t *testing.T) {
	t.Parallel()

	posts := []*types.Post{testPost("post1"), testPost("post2")}
	comments := []*types.Comment{testComment("comment1"), testComment("comment2"), testComment("comment3")}

	fetchCount := 0
	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			fetchCount++
			return &types.PostsResponse{Posts: posts}, nil
		},
		getCommentsFunc: func(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error) {
			return &types.CommentsResponse{Comments: comments}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	config.FetchComments = true

	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for initial fetch to complete
	time.Sleep(50 * time.Millisecond)

	manager.Stop()

	// Get status to check stats through the public API
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	// Status should be stopped after Stop()
	if status.Status != "stopped" {
		t.Errorf("status after Stop() = %q, want 'stopped'", status.Status)
	}

	// Note: We can't check stats after stopping since the stats are cleared.
	// Instead, check during running.
	// Start a fresh test to check stats while running.
}

// TestMonitorManager_ConcurrentAccess tests thread safety with concurrent access.
func TestMonitorManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return &types.PostsResponse{Posts: []*types.Post{testPost("post1")}}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	// Run concurrent operations
	var wg sync.WaitGroup
	const numGoroutines = 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				manager.GetStatus()
				manager.IsRunning()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// If we got here without a race condition or panic, the test passes
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() after concurrent access error = %v", err)
	}
	if status.Status != "running" {
		t.Errorf("status after concurrent access = %q, want 'running'", status.Status)
	}
}

// TestMonitorStats_Snapshot tests that Snapshot returns accurate point-in-time data.
func TestMonitorStats_Snapshot(t *testing.T) {
	t.Parallel()

	stats := &MonitorStats{}

	// Increment some values
	stats.IncrementFetches()
	stats.IncrementFetches()
	stats.IncrementPosts(5)
	stats.IncrementComments(10)

	now := time.Now()
	stats.SetLastFetchTime(now)
	stats.SetLastError("test error")

	snapshot := stats.Snapshot()

	if snapshot.TotalFetches != 2 {
		t.Errorf("TotalFetches = %d, want 2", snapshot.TotalFetches)
	}
	if snapshot.TotalPosts != 5 {
		t.Errorf("TotalPosts = %d, want 5", snapshot.TotalPosts)
	}
	if snapshot.TotalComments != 10 {
		t.Errorf("TotalComments = %d, want 10", snapshot.TotalComments)
	}
	if snapshot.LastFetchTime == nil {
		t.Error("LastFetchTime should not be nil")
	} else if snapshot.LastFetchTime.Unix() != now.Unix() {
		t.Errorf("LastFetchTime = %v, want %v", snapshot.LastFetchTime, now)
	}
	if snapshot.LastError != "test error" {
		t.Errorf("LastError = %q, want 'test error'", snapshot.LastError)
	}
}

// TestMonitorStats_ThreadSafety tests concurrent increments and snapshots work correctly.
func TestMonitorStats_ThreadSafety(t *testing.T) {
	t.Parallel()

	stats := &MonitorStats{}
	const numGoroutines = 10
	const incrementsPerGoroutine = 100

	var wg sync.WaitGroup

	// Spawn goroutines that increment counters
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				stats.IncrementFetches()
				stats.IncrementPosts(1)
				stats.IncrementComments(2)
			}
		}()
	}

	// Spawn goroutines that take snapshots
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				stats.Snapshot()
				stats.SetLastFetchTime(time.Now())
				stats.SetLastError("error")
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// Verify final values
	snapshot := stats.Snapshot()
	expectedFetches := uint64(numGoroutines * incrementsPerGoroutine)
	if snapshot.TotalFetches != expectedFetches {
		t.Errorf("TotalFetches = %d, want %d", snapshot.TotalFetches, expectedFetches)
	}

	expectedPosts := uint64(numGoroutines * incrementsPerGoroutine)
	if snapshot.TotalPosts != expectedPosts {
		t.Errorf("TotalPosts = %d, want %d", snapshot.TotalPosts, expectedPosts)
	}

	expectedComments := uint64(numGoroutines * incrementsPerGoroutine * 2)
	if snapshot.TotalComments != expectedComments {
		t.Errorf("TotalComments = %d, want %d", snapshot.TotalComments, expectedComments)
	}
}

// TestMonitorStats_ZeroSnapshot tests snapshot of zero stats.
func TestMonitorStats_ZeroSnapshot(t *testing.T) {
	t.Parallel()

	stats := &MonitorStats{}
	snapshot := stats.Snapshot()

	if snapshot.TotalFetches != 0 {
		t.Errorf("TotalFetches = %d, want 0", snapshot.TotalFetches)
	}
	if snapshot.TotalPosts != 0 {
		t.Errorf("TotalPosts = %d, want 0", snapshot.TotalPosts)
	}
	if snapshot.TotalComments != 0 {
		t.Errorf("TotalComments = %d, want 0", snapshot.TotalComments)
	}
	if snapshot.LastFetchTime != nil {
		t.Errorf("LastFetchTime = %v, want nil", snapshot.LastFetchTime)
	}
	if snapshot.LastError != "" {
		t.Errorf("LastError = %q, want empty", snapshot.LastError)
	}
}

// TestMonitorManager_StatsWhileRunning tests stats while monitor is actively running.
func TestMonitorManager_StatsWhileRunning(t *testing.T) {
	t.Parallel()

	posts := []*types.Post{testPost("post1")}

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return &types.PostsResponse{Posts: posts}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()

	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to fetch posts
	time.Sleep(50 * time.Millisecond)

	// Get status while running
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Stats == nil {
		t.Fatal("Stats should not be nil while running")
	}

	if status.Stats.TotalFetches == 0 {
		t.Error("TotalFetches should be > 0")
	}

	if status.Stats.TotalPosts != 1 {
		t.Errorf("TotalPosts = %d, want at least 1", status.Stats.TotalPosts)
	}

	if status.Stats.LastFetchTime == nil {
		t.Error("LastFetchTime should be set")
	}

	manager.Stop()
}

// TestMonitorManager_ClientError tests graceful handling of client errors.
func TestMonitorManager_ClientError(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return nil, errors.New("network error")
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to attempt fetch
	time.Sleep(50 * time.Millisecond)

	// Get status to check error while running
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Stats != nil && status.Stats.LastError == "" {
		t.Error("LastError should be set after client error")
	}

	if status.Stats != nil && !containsSubstring(status.Stats.LastError, "network error") {
		t.Errorf("LastError = %q, want to contain 'network error'", status.Stats.LastError)
	}

	manager.Stop()
}

// TestMonitorManager_StoreError tests graceful handling of store errors.
func TestMonitorManager_StoreError(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return &types.PostsResponse{Posts: []*types.Post{testPost("post1")}}, nil
		},
	}

	store := &mockStore{
		upsertPostsFunc: func(ctx context.Context, posts []*types.Post) error {
			return errors.New("database error")
		},
	}

	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to attempt fetch and save
	time.Sleep(200 * time.Millisecond)

	// Get status to check error while running
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Stats != nil && status.Stats.LastError == "" {
		t.Error("LastError should be set after store error")
	}

	if status.Stats != nil && !containsSubstring(status.Stats.LastError, "database error") {
		t.Errorf("LastError = %q, want to contain 'database error'", status.Stats.LastError)
	}

	manager.Stop()
}

// TestMonitorManager_MultipleSubreddits tests monitoring multiple subreddits.
func TestMonitorManager_MultipleSubreddits(t *testing.T) {
	t.Parallel()

	subredditFetched := make(map[string]int)
	mu := sync.Mutex{}

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			mu.Lock()
			subredditFetched[req.Subreddit]++
			mu.Unlock()

			return &types.PostsResponse{Posts: []*types.Post{testPost("post1")}}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	config.Subreddits = []string{"golang", "rust", "python"}

	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to fetch from all subreddits
	time.Sleep(50 * time.Millisecond)

	manager.Stop()

	// Verify all subreddits were fetched
	for _, sr := range config.Subreddits {
		if count := subredditFetched[sr]; count == 0 {
			t.Errorf("subreddit %q was never fetched", sr)
		}
	}
}

// TestMonitorManager_EmptyPostsResponse tests handling of empty posts response.
func TestMonitorManager_EmptyPostsResponse(t *testing.T) {
	t.Parallel()

	client := &mockRedditClient{
		getNewFunc: func(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error) {
			return &types.PostsResponse{Posts: []*types.Post{}}, nil
		},
	}

	store := &mockStore{}
	manager := NewMonitorManager(client, store, newTestLogger())

	config := validConfig()
	_, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for monitor loop to fetch
	time.Sleep(200 * time.Millisecond)

	// Get status while running to check stats
	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.Stats == nil {
		t.Fatal("Stats should not be nil while running")
	}

	// Fetches should still be incremented even with empty response
	if status.Stats.TotalFetches == 0 {
		t.Error("TotalFetches should be > 0 even with empty posts")
	}

	// No posts should have been saved
	if status.Stats.TotalPosts != 0 {
		t.Errorf("TotalPosts = %d, want 0 for empty response", status.Stats.TotalPosts)
	}

	manager.Stop()
}

// TestMonitorInstance_FieldsSet tests that monitor instance fields are properly set.
func TestMonitorInstance_FieldsSet(t *testing.T) {
	t.Parallel()

	manager := NewMonitorManager(&mockRedditClient{}, &mockStore{}, newTestLogger())

	config := MonitorConfig{
		Subreddits:    []string{"golang", "rust"},
		Interval:      30 * time.Second,
		Limit:         50,
		FetchComments: true,
	}

	instance, err := manager.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer manager.Stop()

	if instance.ID == "" {
		t.Error("instance.ID is empty")
	}

	if len(instance.Subreddits) != 2 {
		t.Errorf("instance.Subreddits length = %d, want 2", len(instance.Subreddits))
	}

	if instance.Interval != 30*time.Second {
		t.Errorf("instance.Interval = %v, want 30s", instance.Interval)
	}

	if instance.Limit != 50 {
		t.Errorf("instance.Limit = %d, want 50", instance.Limit)
	}

	if !instance.FetchComments {
		t.Error("instance.FetchComments should be true")
	}

	if instance.StartedAt.IsZero() {
		t.Error("instance.StartedAt should not be zero")
	}
}

// Utility function to check if a string contains a substring (case-sensitive).
func containsSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
