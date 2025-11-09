// Package monitor provides lifecycle management for background Reddit monitoring operations.
package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"
)

// Package-level constants for monitor configuration validation and behavior.
const (
	MinMonitorInterval      = 10 * time.Second
	MaxSubredditsPerMonitor = 10
	MinPostLimit            = 1
	MaxPostLimit            = 100
	CommentsPerPost         = 100
	StopTimeout             = 10 * time.Second
	MaxSubredditNameLength  = 21
)

// MonitorConfig contains configuration for starting a monitor.
type MonitorConfig struct {
	Subreddits    []string
	Interval      time.Duration
	Limit         int
	FetchComments bool
}

// MonitorInstance represents a running monitor.
type MonitorInstance struct {
	ID            string
	Subreddits    []string
	Interval      time.Duration
	Limit         int
	FetchComments bool
	StartedAt     time.Time
}

// MonitorStatus represents the current status of monitoring.
type MonitorStatus struct {
	Status     string
	ID         string
	Subreddits []string
	Interval   string
	StartedAt  *time.Time
	Stats      *StatsSnapshot
}

// StatsSnapshot is a point-in-time snapshot of monitoring statistics.
type StatsSnapshot struct {
	TotalFetches      uint64
	TotalPosts        uint64
	TotalComments     uint64
	FailedFetches     uint64
	ConsecutiveErrors uint64
	LastFetchTime     *time.Time
	LastError         string
}

// RedditClient defines the Reddit API operations needed by the monitor.
type RedditClient interface {
	GetNew(ctx context.Context, req *types.PostsRequest) (*types.PostsResponse, error)
	GetComments(ctx context.Context, req *types.CommentsRequest) (*types.CommentsResponse, error)
}

// MonitorManager manages the lifecycle of background monitoring operations.
type MonitorManager struct {
	mu               sync.RWMutex
	activeMonitor    *monitorRuntimeInstance
	lastMonitorStats *StatsSnapshot
	client           RedditClient
	store            storage.Store
	logger           *slog.Logger
}

// monitorRuntimeInstance represents the internal runtime state of a running monitor.
// This is different from handlers.MonitorInstance which is the public API type.
type monitorRuntimeInstance struct {
	ID            string
	Subreddits    []string
	Interval      time.Duration
	Limit         int
	FetchComments bool
	StartedAt     time.Time
	cancel        context.CancelFunc
	stats         *MonitorStats
	done          chan error
}

// MonitorStats tracks monitoring statistics in a thread-safe manner.
type MonitorStats struct {
	mu                sync.RWMutex
	totalFetches      uint64 // use atomic operations
	totalPosts        uint64 // use atomic operations
	totalComments     uint64 // use atomic operations
	failedFetches     uint64 // use atomic operations
	consecutiveErrors uint64 // use atomic operations
	LastFetchTime     time.Time
	LastError         string
}

// Error types for monitor operations.
var (
	// ErrNoMonitorRunning is returned when attempting to stop a monitor that is not running.
	ErrNoMonitorRunning = errors.New("no monitor currently running")
	// ErrMonitorAlreadyRunning is returned when attempting to start a monitor while one is already running.
	ErrMonitorAlreadyRunning = errors.New("monitor is already running")
	// ErrInvalidConfig is returned when the monitor configuration is invalid.
	ErrInvalidConfig = errors.New("invalid monitor configuration")
)

// NewMonitorManager creates a new monitor manager.
func NewMonitorManager(client RedditClient, store storage.Store, logger *slog.Logger) *MonitorManager {
	return &MonitorManager{
		client: client,
		store:  store,
		logger: logger,
	}
}

// Start begins monitoring with the given configuration.
// Returns error if validation fails or a monitor is already running.
func (m *MonitorManager) Start(ctx context.Context, config MonitorConfig) (*MonitorInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeMonitor != nil {
		return nil, ErrMonitorAlreadyRunning
	}

	if err := m.validateConfig(config); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	id := uuid.New().String()
	startedAt := time.Now()

	instance := &monitorRuntimeInstance{
		ID:            id,
		Subreddits:    config.Subreddits,
		Interval:      config.Interval,
		Limit:         config.Limit,
		FetchComments: config.FetchComments,
		StartedAt:     startedAt,
		cancel:        cancel,
		stats:         &MonitorStats{},
		done:          make(chan error, 1),
	}

	m.activeMonitor = instance

	m.logger.Info("starting monitor",
		slog.String("monitor_id", id),
		slog.Any("subreddits", config.Subreddits),
		slog.Duration("interval", config.Interval),
		slog.Int("limit", config.Limit),
		slog.Bool("fetch_comments", config.FetchComments),
	)

	go m.monitorLoop(monitorCtx, instance)

	// Return the MonitorInstance type
	return &MonitorInstance{
		ID:            id,
		Subreddits:    config.Subreddits,
		Interval:      config.Interval,
		Limit:         config.Limit,
		FetchComments: config.FetchComments,
		StartedAt:     startedAt,
	}, nil
}

// Stop gracefully stops the currently running monitor.
// Returns error if no monitor is running.
func (m *MonitorManager) Stop() error {
	m.mu.Lock()
	instance := m.activeMonitor
	if instance == nil {
		m.mu.Unlock()
		return ErrNoMonitorRunning
	}
	// Clear activeMonitor while holding the lock to prevent race condition
	m.activeMonitor = nil
	m.mu.Unlock()

	m.logger.Info("stopping monitor", slog.String("monitor_id", instance.ID))

	instance.cancel()

	// Wait for monitor loop to finish with timeout
	ctx, cancel := context.WithTimeout(context.Background(), StopTimeout)
	defer cancel()

	var timeoutErr error
	select {
	case err := <-instance.done:
		if err != nil {
			m.logger.Warn("monitor stopped with error",
				slog.String("monitor_id", instance.ID),
				slog.Any("error", err),
			)
		}
	case <-ctx.Done():
		m.logger.Warn("monitor stop timeout",
			slog.String("monitor_id", instance.ID),
		)
		timeoutErr = fmt.Errorf("monitor stop timeout after %v", StopTimeout)
	}

	snapshot := instance.stats.Snapshot()

	// Query actual database stats to get real unique post/comment counts
	// This prevents inflated counts from duplicate fetches
	statsCtx, statsCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer statsCancel()

	if dbStats, err := m.store.GetStats(statsCtx); err == nil && dbStats != nil {
		snapshot.TotalPosts = uint64(dbStats.PostCount)
		snapshot.TotalComments = uint64(dbStats.CommentCount)
	} else if err != nil {
		m.logger.Warn("failed to get database stats after monitor stop",
			slog.String("monitor_id", instance.ID),
			slog.Any("error", err),
		)
	}

	m.logger.Info("monitor stopped",
		slog.String("monitor_id", instance.ID),
		slog.Uint64("total_fetches", snapshot.TotalFetches),
		slog.Uint64("total_posts", snapshot.TotalPosts),
		slog.Uint64("total_comments", snapshot.TotalComments),
	)

	// Preserve stats after stop
	m.mu.Lock()
	m.lastMonitorStats = snapshot
	m.mu.Unlock()

	return timeoutErr
}

// GetStatus returns the current monitoring status.
func (m *MonitorManager) GetStatus() (*MonitorStatus, error) {
	m.mu.RLock()
	instance := m.activeMonitor
	var id string
	var subreddits []string
	var interval time.Duration
	var started time.Time
	if instance != nil {
		id = instance.ID
		subreddits = append([]string{}, instance.Subreddits...)
		interval = instance.Interval
		started = instance.StartedAt
	}
	m.mu.RUnlock()

	if instance == nil {
		return &MonitorStatus{
			Status: "stopped",
		}, nil
	}

	snapshot := instance.stats.Snapshot()

	// Query actual database stats to get real unique post/comment counts
	// This prevents inflated counts from duplicate fetches
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if dbStats, err := m.store.GetStats(ctx); err == nil && dbStats != nil {
		snapshot.TotalPosts = uint64(dbStats.PostCount)
		snapshot.TotalComments = uint64(dbStats.CommentCount)
	} else if err != nil {
		m.logger.Warn("failed to get database stats for monitor status",
			slog.String("monitor_id", id),
			slog.Any("error", err),
		)
	}

	return &MonitorStatus{
		Status:     "running",
		ID:         id,
		Subreddits: subreddits,
		Interval:   interval.String(),
		StartedAt:  &started,
		Stats:      snapshot,
	}, nil
}

// IsRunning returns true if a monitor is currently active.
func (m *MonitorManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeMonitor != nil
}

// validateConfig validates the monitor configuration.
func (m *MonitorManager) validateConfig(config MonitorConfig) error {
	if len(config.Subreddits) == 0 {
		return errors.New("subreddits: must not be empty")
	}

	if len(config.Subreddits) > MaxSubredditsPerMonitor {
		return fmt.Errorf("subreddits: maximum %d subreddits allowed", MaxSubredditsPerMonitor)
	}

	// Validate individual subreddit names
	for _, subreddit := range config.Subreddits {
		if err := m.validateSubredditName(subreddit); err != nil {
			return err
		}
	}

	if config.Interval < MinMonitorInterval {
		return fmt.Errorf("interval: must be at least %v", MinMonitorInterval)
	}

	if config.Limit < MinPostLimit || config.Limit > MaxPostLimit {
		return fmt.Errorf("limit: must be between %d and %d", MinPostLimit, MaxPostLimit)
	}

	return nil
}

// validateSubredditName validates an individual subreddit name.
// It checks for empty strings, length constraints, and format validity.
func (m *MonitorManager) validateSubredditName(name string) error {
	if name == "" {
		return errors.New("subreddit name: must not be empty")
	}

	if len(name) > MaxSubredditNameLength {
		return fmt.Errorf("subreddit name %q: exceeds maximum length of %d characters", name, MaxSubredditNameLength)
	}

	return nil
}

// monitorLoop is the main monitoring loop (runs in goroutine).
func (m *MonitorManager) monitorLoop(ctx context.Context, instance *monitorRuntimeInstance) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("monitor loop panic",
				slog.String("monitor_id", instance.ID),
				slog.Any("panic", r),
			)
			instance.done <- fmt.Errorf("monitor panic: %v", r)
		} else {
			instance.done <- nil
		}
		close(instance.done)
	}()

	ticker := time.NewTicker(instance.Interval)
	defer ticker.Stop()

	// Fetch once immediately
	for _, subreddit := range instance.Subreddits {
		// Check context before each fetch
		if ctx.Err() != nil {
			return
		}

		if err := m.fetchAndSave(ctx, subreddit, instance); err != nil {
			m.logger.Warn("initial fetch failed",
				slog.String("monitor_id", instance.ID),
				slog.String("subreddit", subreddit),
				slog.Any("error", err),
			)
			instance.stats.SetLastError(err.Error())
			instance.stats.IncrementFailedFetches()
		}
	}

	// Then loop on ticker
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, subreddit := range instance.Subreddits {
				// Check context before each fetch
				if ctx.Err() != nil {
					return
				}

				if err := m.fetchAndSave(ctx, subreddit, instance); err != nil {
					m.logger.Warn("fetch failed",
						slog.String("monitor_id", instance.ID),
						slog.String("subreddit", subreddit),
						slog.Any("error", err),
					)
					instance.stats.SetLastError(err.Error())
					instance.stats.IncrementFailedFetches()
				}
			}
		}
	}
}

// fetchAndSave fetches posts from a subreddit and saves them.
// Comment fetch errors are non-fatal and will not cause the function to fail,
// allowing the monitor to continue operating even if comments cannot be retrieved.
func (m *MonitorManager) fetchAndSave(ctx context.Context, subreddit string, instance *monitorRuntimeInstance) error {
	req := &types.PostsRequest{
		Subreddit: subreddit,
		Pagination: types.Pagination{
			Limit: instance.Limit,
		},
	}

	resp, err := m.client.GetNew(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch new posts from %s: %w", subreddit, err)
	}

	instance.stats.IncrementFetches()

	if resp == nil || len(resp.Posts) == 0 {
		instance.stats.SetLastFetchTime(time.Now())
		return nil
	}

	if err := m.store.UpsertPosts(ctx, resp.Posts); err != nil {
		return fmt.Errorf("failed to save posts from %s: %w", subreddit, err)
	}

	// Note: Post count is tracked via database stats, not incremented here
	// This prevents counting duplicate posts on subsequent fetches

	// Optionally fetch comments for each post
	if instance.FetchComments {
		for _, post := range resp.Posts {
			// Check context before fetching each comment
			if ctx.Err() != nil {
				return ctx.Err()
			}

			commentsReq := &types.CommentsRequest{
				Subreddit: subreddit,
				PostID:    post.ID,
				Pagination: types.Pagination{
					Limit: CommentsPerPost,
				},
			}

			commentsResp, err := m.client.GetComments(ctx, commentsReq)
			if err != nil {
				// Non-fatal error: log and continue with next post
				m.logger.Warn("failed to fetch comments",
					slog.String("monitor_id", instance.ID),
					slog.String("post_id", post.ID),
					slog.Any("error", err),
				)
				instance.stats.IncrementConsecutiveErrors()
				continue
			}

			// Reset consecutive error count on successful fetch
			instance.stats.ResetConsecutiveErrors()

			if commentsResp != nil && len(commentsResp.Comments) > 0 {
				if err := m.store.UpsertComments(ctx, commentsResp.Comments); err != nil {
					// Non-fatal error: log and continue with next post
					m.logger.Warn("failed to save comments",
						slog.String("monitor_id", instance.ID),
						slog.String("post_id", post.ID),
						slog.Any("error", err),
					)
					instance.stats.IncrementConsecutiveErrors()
					continue
				}
				// Note: Comment count is tracked via database stats, not incremented here
				// This prevents counting duplicate comments on subsequent fetches
			}
		}
	}

	instance.stats.SetLastFetchTime(time.Now())
	instance.stats.SetLastError("")

	return nil
}

// IncrementFetches atomically increments the fetch counter.
func (s *MonitorStats) IncrementFetches() {
	atomic.AddUint64(&s.totalFetches, 1)
}

// IncrementPosts atomically increments the post counter by n.
func (s *MonitorStats) IncrementPosts(n uint64) {
	atomic.AddUint64(&s.totalPosts, n)
}

// IncrementComments atomically increments the comment counter by n.
func (s *MonitorStats) IncrementComments(n uint64) {
	atomic.AddUint64(&s.totalComments, n)
}

// IncrementFailedFetches atomically increments the failed fetch counter.
func (s *MonitorStats) IncrementFailedFetches() {
	atomic.AddUint64(&s.failedFetches, 1)
}

// IncrementConsecutiveErrors atomically increments the consecutive error counter.
func (s *MonitorStats) IncrementConsecutiveErrors() {
	atomic.AddUint64(&s.consecutiveErrors, 1)
}

// ResetConsecutiveErrors resets the consecutive error counter to zero.
func (s *MonitorStats) ResetConsecutiveErrors() {
	atomic.StoreUint64(&s.consecutiveErrors, 0)
}

// SetLastFetchTime sets the last fetch time (thread-safe).
func (s *MonitorStats) SetLastFetchTime(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastFetchTime = t
}

// SetLastError sets the last error message (thread-safe).
func (s *MonitorStats) SetLastError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
}

// Snapshot returns a point-in-time copy of the stats.
func (s *MonitorStats) Snapshot() *StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := &StatsSnapshot{
		TotalFetches:      atomic.LoadUint64(&s.totalFetches),
		TotalPosts:        atomic.LoadUint64(&s.totalPosts),
		TotalComments:     atomic.LoadUint64(&s.totalComments),
		FailedFetches:     atomic.LoadUint64(&s.failedFetches),
		ConsecutiveErrors: atomic.LoadUint64(&s.consecutiveErrors),
		LastError:         s.LastError,
	}

	if !s.LastFetchTime.IsZero() {
		snapshot.LastFetchTime = &s.LastFetchTime
	}

	return snapshot
}
