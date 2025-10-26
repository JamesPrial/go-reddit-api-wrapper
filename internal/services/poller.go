package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

const (
	// DefaultPollInterval is the default time between polling cycles (60 seconds).
	DefaultPollInterval = 60 * time.Second

	// DefaultWorkerCount is the default number of concurrent workers.
	// Matches MaxConcurrentCommentRequests from the Reddit client to prevent overload.
	DefaultWorkerCount = 10

	// MinPollInterval is the minimum allowed polling interval to prevent API abuse.
	MinPollInterval = 10 * time.Second

	// MaxWorkerCount is the maximum number of concurrent workers.
	MaxWorkerCount = 20
)

// Poller is the main polling service that coordinates fetching Reddit data.
// It manages a worker pool that concurrently polls multiple subreddits,
// stores the data in the database, and handles graceful shutdown.
//
// The poller:
//   - Loads tracked subreddits from the database on startup
//   - Creates a worker pool for concurrent processing
//   - Polls each subreddit at the configured interval
//   - Distributes work across workers via a job channel
//   - Supports context-based cancellation and graceful shutdown
type Poller struct {
	redditClient *graw.Reddit
	repository   db.Repository
	interval     time.Duration
	workerCount  int
	logger       *slog.Logger

	// Internal state
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	mu         sync.RWMutex
}

// Config holds the configuration for creating a Poller.
// All fields are required except PollInterval and WorkerCount which have defaults.
type Config struct {
	// RedditClient is the Reddit API client for fetching data.
	// Required - must not be nil.
	RedditClient *graw.Reddit

	// Repository is the database repository for storing/retrieving data.
	// Required - must not be nil.
	Repository db.Repository

	// PollInterval is the time to wait between polling cycles.
	// Optional - defaults to DefaultPollInterval (60 seconds).
	// Must be at least MinPollInterval (10 seconds).
	PollInterval time.Duration

	// WorkerCount is the number of concurrent workers in the pool.
	// Optional - defaults to DefaultWorkerCount (10 workers).
	// Must be between 1 and MaxWorkerCount (20).
	WorkerCount int

	// Logger for structured logging.
	// Optional - if nil, a default logger will be created.
	Logger *slog.Logger
}

// NewPoller creates a new Poller instance with the given configuration.
// It validates the configuration and applies defaults for optional fields.
//
// Returns an error if:
//   - RedditClient is nil
//   - Repository is nil
//   - PollInterval is less than MinPollInterval
//   - WorkerCount is invalid (less than 1 or greater than MaxWorkerCount)
func NewPoller(cfg Config) (*Poller, error) {
	// Validate required fields
	if cfg.RedditClient == nil {
		return nil, fmt.Errorf("reddit client is required")
	}
	if cfg.Repository == nil {
		return nil, fmt.Errorf("repository is required")
	}

	// Apply defaults and validate optional fields
	interval := cfg.PollInterval
	if interval == 0 {
		interval = DefaultPollInterval
	}
	if interval < MinPollInterval {
		return nil, fmt.Errorf("poll interval must be at least %v, got %v", MinPollInterval, interval)
	}

	workerCount := cfg.WorkerCount
	if workerCount == 0 {
		workerCount = DefaultWorkerCount
	}
	if workerCount < 1 || workerCount > MaxWorkerCount {
		return nil, fmt.Errorf("worker count must be between 1 and %d, got %d", MaxWorkerCount, workerCount)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Poller{
		redditClient: cfg.RedditClient,
		repository:   cfg.Repository,
		interval:     interval,
		workerCount:  workerCount,
		logger:       logger,
		running:      false,
	}, nil
}

// Start begins the polling service.
// It:
//   - Loads tracked subreddits from the database
//   - Creates a worker pool
//   - Starts a polling loop that runs at the configured interval
//   - Blocks until the context is cancelled or Stop is called
//
// This method should be called in a goroutine if you need it to run in the background.
// Use the provided context to control cancellation.
//
// Returns an error if:
//   - The poller is already running
//   - Failed to load subreddits from the database
//
// Example usage:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	go func() {
//	    if err := poller.Start(ctx); err != nil {
//	        log.Printf("Poller error: %v", err)
//	    }
//	}()
//
//	// ... do other work ...
//	cancel() // Stop the poller
func (p *Poller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("poller is already running")
	}
	p.running = true
	p.mu.Unlock()

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	p.cancelFunc = cancel

	// Ensure cleanup on exit
	defer func() {
		p.mu.Lock()
		p.running = false
		p.cancelFunc = nil
		p.mu.Unlock()
	}()

	p.logger.Info("starting poller",
		slog.Duration("interval", p.interval),
		slog.Int("worker_count", p.workerCount))

	// Create job channel
	jobs := make(chan job, 100) // Buffer to prevent blocking

	// Start worker pool
	for i := 0; i < p.workerCount; i++ {
		w := newWorker(i, jobs, p.redditClient, p.repository, p.logger)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			w.start(ctx)
		}()
	}

	// Start polling loop
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run initial poll immediately
	if err := p.pollOnce(ctx, jobs); err != nil {
		p.logger.Error("initial poll failed", slog.String("error", err.Error()))
	}

	// Poll at regular intervals
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("poller stopping - context cancelled")
			close(jobs) // Signal workers to stop
			p.wg.Wait() // Wait for all workers to finish
			p.logger.Info("poller stopped - all workers finished")
			return ctx.Err()

		case <-ticker.C:
			if err := p.pollOnce(ctx, jobs); err != nil {
				p.logger.Error("poll cycle failed", slog.String("error", err.Error()))
			}
		}
	}
}

// Stop gracefully stops the polling service.
// It cancels the context, waits for all workers to finish, and returns.
// This method blocks until all workers have completed their current jobs.
//
// It is safe to call Stop multiple times or when the poller is not running.
func (p *Poller) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	cancelFunc := p.cancelFunc
	p.mu.Unlock()

	if cancelFunc != nil {
		p.logger.Info("stopping poller")
		cancelFunc()
		// Wait is handled in the Start method's defer
	}

	return nil
}

// IsRunning returns true if the poller is currently running.
func (p *Poller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// pollOnce performs a single polling cycle:
//   - Fetches all tracked subreddits from the database
//   - Creates a job for each subreddit
//   - Sends jobs to the worker pool via the jobs channel
//
// This method does not block waiting for jobs to complete - it just enqueues them.
func (p *Poller) pollOnce(ctx context.Context, jobs chan<- job) error {
	// Fetch all subreddits from the database
	subreddits, err := p.repository.ListSubreddits(ctx)
	if err != nil {
		return fmt.Errorf("failed to list subreddits: %w", err)
	}

	if len(subreddits) == 0 {
		p.logger.Debug("no subreddits to poll")
		return nil
	}

	p.logger.Info("starting poll cycle", slog.Int("subreddit_count", len(subreddits)))

	// Create a job for each subreddit
	enqueuedCount := 0
	for _, sub := range subreddits {
		select {
		case <-ctx.Done():
			p.logger.Info("poll cycle cancelled", slog.Int("enqueued", enqueuedCount))
			return ctx.Err()
		case jobs <- job{subreddit: sub}:
			enqueuedCount++
		}
	}

	p.logger.Debug("poll cycle enqueued jobs", slog.Int("job_count", enqueuedCount))
	return nil
}
