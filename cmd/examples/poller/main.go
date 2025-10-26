package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/internal/db"
	"github.com/jamesprial/go-reddit-api-wrapper/internal/services"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// This example demonstrates how to use the polling service to continuously
// fetch Reddit data and store it in the database.
//
// Required environment variables:
//   - REDDIT_CLIENT_ID: Your Reddit application client ID
//   - REDDIT_CLIENT_SECRET: Your Reddit application client secret
//
// Optional environment variables:
//   - REDDIT_USER_AGENT: Custom user agent (defaults to go-reddit-api-wrapper)
//   - POLL_INTERVAL: Polling interval in seconds (defaults to 60)
//   - WORKER_COUNT: Number of concurrent workers (defaults to 10)
//   - DB_PATH: Path to SQLite database file (defaults to reddit_tracker.db)
//
// Usage:
//
//	export REDDIT_CLIENT_ID="your-client-id"
//	export REDDIT_CLIENT_SECRET="your-client-secret"
//	go run ./cmd/examples/poller
func main() {
	// Get configuration from environment
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	userAgent := os.Getenv("REDDIT_USER_AGENT")
	if userAgent == "" {
		userAgent = "go-reddit-api-wrapper-poller/1.0"
	}

	if clientID == "" || clientSecret == "" {
		log.Fatal("REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET must be set")
	}

	// Parse optional configuration
	pollInterval := 60 * time.Second
	if intervalStr := os.Getenv("POLL_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr + "s"); err == nil {
			pollInterval = interval
		}
	}

	workerCount := 10
	if countStr := os.Getenv("WORKER_COUNT"); countStr != "" {
		if _, err := fmt.Sscanf(countStr, "%d", &workerCount); err != nil {
			log.Printf("Invalid WORKER_COUNT, using default: %v", err)
			workerCount = 10
		}
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "reddit_tracker.db"
	}

	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting poller example",
		slog.String("db_path", dbPath),
		slog.Duration("poll_interval", pollInterval),
		slog.Int("worker_count", workerCount))

	// Initialize database
	database, err := db.InitDB(db.Config{
		Path:        dbPath,
		EnableDebug: false,
	})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("Failed to get SQL database: %v", err)
	}
	defer sqlDB.Close()

	logger.Info("database initialized")

	// Create repository
	repo := db.NewRepository(database)

	// Create Reddit client with custom logger
	redditClient, err := graw.NewClient(&graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    userAgent,
		Logger:       logger,
	})
	if err != nil {
		log.Fatalf("Failed to create Reddit client: %v", err)
	}

	logger.Info("reddit client created")

	// Ensure we have some subreddits to track
	// Check if database is empty and add sample subreddits
	ctx := context.Background()
	subreddits, err := repo.ListSubreddits(ctx)
	if err != nil {
		log.Fatalf("Failed to list subreddits: %v", err)
	}

	if len(subreddits) == 0 {
		logger.Info("no subreddits found, adding sample subreddits")

		// Fetch subreddit info from Reddit and store it
		sampleSubreddits := []string{"golang", "programming", "technology"}
		for _, name := range sampleSubreddits {
			subInfo, err := redditClient.GetSubreddit(ctx, name)
			if err != nil {
				logger.Warn("failed to fetch subreddit info",
					slog.String("subreddit", name),
					slog.String("error", err.Error()))
				continue
			}

			sub := services.SubredditToModel(subInfo)
			if err := repo.CreateSubreddit(ctx, sub); err != nil {
				logger.Warn("failed to store subreddit",
					slog.String("subreddit", name),
					slog.String("error", err.Error()))
				continue
			}

			logger.Info("added subreddit to tracking",
				slog.String("name", sub.Name),
				slog.Int64("subscribers", sub.Subscribers))
		}
	} else {
		logger.Info("tracking subreddits",
			slog.Int("count", len(subreddits)))
		for _, sub := range subreddits {
			logger.Info("  tracking",
				slog.String("subreddit", sub.Name),
				slog.Int64("subscribers", sub.Subscribers))
		}
	}

	// Create poller
	poller, err := services.NewPoller(services.Config{
		RedditClient: redditClient,
		Repository:   repo,
		PollInterval: pollInterval,
		WorkerCount:  workerCount,
		Logger:       logger,
	})
	if err != nil {
		log.Fatalf("Failed to create poller: %v", err)
	}

	logger.Info("poller created")

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))
		cancel()
	}()

	// Start the poller (blocks until context cancelled)
	logger.Info("starting poller - press Ctrl+C to stop")
	if err := poller.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Poller error: %v", err)
	}

	logger.Info("poller stopped gracefully")
}
