// Package services provides background polling services for fetching Reddit data.
//
// The services package implements a worker pool pattern for efficiently polling
// multiple subreddits concurrently, fetching posts and comments, and storing them
// in the database.
//
// # Overview
//
// The main components are:
//
//   - Poller: Orchestrates the polling process, manages worker pool, and handles lifecycle
//   - Worker: Processes individual jobs (subreddit polling) and stores data
//   - Converter functions: Transform Reddit API types to database models
//
// # Architecture
//
// The polling service uses a producer-consumer pattern:
//
//	┌─────────┐
//	│ Poller  │  (Producer)
//	└────┬────┘
//	     │ Creates jobs every interval
//	     │
//	     ▼
//	┌─────────────┐
//	│ Job Channel │  (Buffer size: 100)
//	└──────┬──────┘
//	       │
//	       │ Distributed to workers
//	       │
//	  ┌────┴────┬────────┬────────┐
//	  ▼         ▼        ▼        ▼
//	┌────┐   ┌────┐   ┌────┐   ┌────┐
//	│ W1 │   │ W2 │   │ W3 │...│ Wn │  (Consumers, max 10-20)
//	└─┬──┘   └─┬──┘   └─┬──┘   └─┬──┘
//	  │        │        │        │
//	  │ Fetch  │ Fetch  │ Fetch  │ Fetch
//	  ▼        ▼        ▼        ▼
//	┌─────────────────────────────────┐
//	│      Reddit API Client          │
//	└────────────┬────────────────────┘
//	             │
//	             │ Store data
//	             ▼
//	     ┌───────────────┐
//	     │   Database    │
//	     └───────────────┘
//
// # Usage
//
// Basic usage:
//
//	// Create Reddit client
//	redditClient, err := graw.NewClient(&graw.Config{
//	    ClientID:     "your-client-id",
//	    ClientSecret: "your-client-secret",
//	    UserAgent:    "poller/1.0",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Initialize database
//	database, err := db.InitDB(db.Config{Path: "reddit.db"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	repo := db.NewRepository(database)
//
//	// Create and configure poller
//	poller, err := services.NewPoller(services.Config{
//	    RedditClient: redditClient,
//	    Repository:   repo,
//	    PollInterval: 60 * time.Second,
//	    WorkerCount:  10,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start polling (blocks until context cancelled)
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	go func() {
//	    if err := poller.Start(ctx); err != nil {
//	        log.Printf("Poller error: %v", err)
//	    }
//	}()
//
//	// ... application runs ...
//
//	// Graceful shutdown
//	cancel()
//	poller.Stop()
//
// # Concurrency
//
// The service is designed for concurrent operation:
//
//   - Worker pool processes multiple subreddits in parallel (default: 10 workers)
//   - Each worker can handle one subreddit at a time
//   - Semaphore-based concurrency control prevents overload
//   - Context cancellation propagates to all workers
//   - Panic recovery in workers prevents deadlocks
//
// # Error Handling
//
// The service implements resilient error handling:
//
//   - Network/API errors are logged but don't stop the poller
//   - Database duplicate errors are ignored (idempotent operations)
//   - Individual post/comment failures don't prevent processing others
//   - Worker panics are recovered and logged
//   - Graceful shutdown waits for in-flight jobs to complete
//
// # Data Flow
//
// For each polling cycle:
//
//  1. Poller fetches list of tracked subreddits from database
//  2. Creates a job for each subreddit
//  3. Enqueues jobs to worker channel
//  4. Workers pick up jobs and for each subreddit:
//     a. Fetch hot posts (limit 25) via Reddit API
//     b. Convert posts to database models
//     c. Check if post exists (by fullname), update if exists, create if new
//     d. For each new post, fetch comments (limit 100)
//     e. Convert comments to database models
//     f. Batch insert comments into database
//  5. Wait for configured interval, then repeat
//
// # Performance Considerations
//
//   - Worker pool size is configurable (default 10, max 20)
//   - Job channel is buffered (size 100) to prevent blocking
//   - Comments are batch-inserted (100 per batch) for efficiency
//   - Reddit API rate limits are handled by the underlying client
//   - Database queries use indexes for optimal performance
//
// # Deduplication
//
// Posts and comments are deduplicated using Reddit fullnames:
//
//   - Fullname is a unique constraint in the database (e.g., "t3_abc123")
//   - Existing posts are updated (score, comment count)
//   - Duplicate comment inserts are handled gracefully
//   - No polling state is maintained - idempotent operations
//
// # Limitations
//
//   - Comments are stored flat (nested structure not preserved initially)
//   - Maximum 25 posts fetched per subreddit per poll
//   - Maximum 100 comments fetched per post
//   - No support for fetching "more" comments (truncated threads)
//   - Polling interval is global (not per-subreddit)
package services
