// Command reddit provides a CLI for interacting with the Reddit API.
//
// Usage:
//
//	reddit [flags] <command> [args...]
//
// For complete usage information, run:
//
//	reddit help
//
// Common commands:
//
//	me              Get authenticated user info
//	hot [subreddit] Get hot posts
//	monitor <sub>   Monitor subreddit(s) indefinitely
//
// Environment variables:
//
//	REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET (required)
//	REDDIT_USERNAME, REDDIT_PASSWORD (optional, for user auth)
//	See 'reddit help' for complete list
//
// Exit codes:
//
//	0 - Success
//	1 - General error
//	2 - Configuration/validation error
//	3 - Authentication error
//	4 - Network error
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/commands"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/storage"

	_ "github.com/jamesprial/go-reddit-api-wrapper/storage/sqlite"
)

// Exit codes used by the CLI.
const (
	ExitOK         = 0 // Success
	ExitGeneralErr = 1 // General/unhandled error
	ExitValidation = 2 // Validation error
	ExitAuth       = 3 // Authentication error
	ExitNetwork    = 4 // Network error
)

// Timeout constants
const (
	// DefaultAuthTimeout is the timeout for Reddit OAuth authentication
	DefaultAuthTimeout = 60 * time.Second
)

// Global flags
var (
	flagClientID        = flag.String("client-id", "", "Reddit OAuth2 client ID (env: REDDIT_CLIENT_ID)")
	flagClientSecret    = flag.String("client-secret", "", "Reddit OAuth2 client secret (env: REDDIT_CLIENT_SECRET)")
	flagUsername        = flag.String("username", "", "Reddit username for user auth (env: REDDIT_USERNAME)")
	flagPassword        = flag.String("password", "", "Reddit password for user auth (env: REDDIT_PASSWORD)")
	flagUserAgent       = flag.String("user-agent", "", "Custom user agent string")
	flagOutput          = flag.String("output", "text", "Output format: json, table, or text")
	flagLimit           = flag.Int("limit", 25, "Max items to fetch (1-100)")
	flagAfter           = flag.String("after", "", "Pagination token for next page")
	flagBefore          = flag.String("before", "", "Pagination token for previous page")
	flagVerbose         = flag.Bool("verbose", false, "Enable verbose output")
	flagDebug           = flag.Bool("debug", false, "Enable debug logging")
	flagTimeout         = flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
	flagStore           = flag.Bool("store", false, "Enable storage of posts and comments (env: REDDIT_STORE)")
	flagDBPath          = flag.String("db-path", "", "Path to SQLite database file (env: REDDIT_DB_PATH)")
	flagMonitorDuration = flag.String("monitor-duration", "0", "How long to monitor (e.g., 1h, 30m). 0 means indefinite.")
	flagMonitorInterval = flag.String("monitor-interval", "5m", "Polling interval for monitor command")
	flagMonitorLimit    = flag.Int("monitor-limit", 25, "Posts per fetch for monitor command")
	flagFetchComments   = flag.Bool("fetch-comments", true, "Fetch comments for posts in monitor mode")
)

func main() {
	flag.Parse()

	// Get command and arguments first (before loading config)
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(ExitValidation)
	}

	command := args[0]
	commandArgs := args[1:]

	// Handle help command early without requiring authentication
	if command == "help" || command == "-h" || command == "--help" || command == "-help" {
		printUsage()
		os.Exit(ExitOK)
	}

	// Load configuration from environment and apply flag overrides
	cfg, err := loadConfig()
	if err != nil {
		printError("Configuration error", err)
		os.Exit(ExitValidation)
	}

	// Setup signal handling for graceful shutdown
	cmdCtx, cmdCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cmdCancel()

	// Apply timeout if configured
	if *flagTimeout > 0 {
		var timeoutCancel context.CancelFunc
		cmdCtx, timeoutCancel = context.WithTimeout(cmdCtx, *flagTimeout)
		defer timeoutCancel()
	}

	// Initialize storage if enabled
	var store storage.Store
	if cfg.Store {
		// DBPath is already expanded by config.FromEnv()
		dbPath := cfg.DBPath

		// Validate path to prevent directory traversal attacks
		if strings.Contains(dbPath, "..") {
			printError("storage", fmt.Errorf("invalid database path: contains '..'"))
			os.Exit(ExitValidation)
		}

		// Create directory if needed (but not for in-memory database)
		if dbPath != ":memory:" {
			dbDir := filepath.Dir(dbPath)
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				printError("storage", fmt.Errorf("failed to create database directory: %w", err))
				os.Exit(ExitGeneralErr)
			}
		}

		logger := slog.Default()
		if cfg.Debug {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
		}

		// Use context.Background() for storage initialization (one-time setup, not command timeout)
		storeCtx := context.Background()
		var err error
		store, err = storage.New(storeCtx, storage.Config{
			DSN:    dbPath,
			Driver: "sqlite",
			Logger: logger,
		})
		if err != nil {
			printError("storage", fmt.Errorf("failed to initialize storage: %w", err))
			os.Exit(ExitGeneralErr)
		}
		defer store.Close()
	}

	// Execute command (handles auth and client creation internally)
	if err := executeCommand(cmdCtx, cfg, command, commandArgs, store); err != nil {
		exitCode := classifyError(err)
		printError(command, err)
		os.Exit(exitCode)
	}
}

// loadConfig loads configuration from environment variables and CLI flags.
func loadConfig() (*config.Config, error) {
	cfg, err := config.FromEnv()
	if err != nil {
		// It's okay if env vars are not set, we might have flags
		cfg = &config.Config{
			Output: "text",
			Limit:  25,
			DBPath: "~/.reddit/data.db",
		}
	}

	// Apply CLI flag overrides (non-empty values)
	if *flagClientID != "" {
		cfg.ClientID = *flagClientID
	}
	if *flagClientSecret != "" {
		cfg.ClientSecret = *flagClientSecret
	}
	if *flagUsername != "" {
		cfg.Username = *flagUsername
	}
	if *flagPassword != "" {
		cfg.Password = *flagPassword
	}
	if *flagUserAgent != "" {
		cfg.UserAgent = *flagUserAgent
	}
	if *flagOutput != "text" { // Respect default
		cfg.Output = *flagOutput
	}
	if *flagLimit != 25 { // Respect default
		cfg.Limit = *flagLimit
	}
	if *flagAfter != "" {
		cfg.After = *flagAfter
	}
	if *flagBefore != "" {
		cfg.Before = *flagBefore
	}
	if *flagVerbose {
		cfg.Verbose = *flagVerbose
	}
	if *flagDebug {
		cfg.Debug = *flagDebug
	}
	if *flagStore {
		cfg.Store = *flagStore
	}
	if *flagDBPath != "" {
		cfg.DBPath = *flagDBPath
	}

	// Monitor flag overrides
	if *flagMonitorDuration != "0" {
		duration, err := time.ParseDuration(*flagMonitorDuration)
		if err != nil {
			return nil, fmt.Errorf("invalid --monitor-duration: %w", err)
		}
		cfg.MonitorDuration = duration
	}
	if *flagMonitorInterval != "" {
		duration, err := time.ParseDuration(*flagMonitorInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid --monitor-interval: %w", err)
		}
		cfg.MonitorInterval = duration
	}
	if *flagMonitorLimit != 25 { // Respect default
		cfg.MonitorLimit = *flagMonitorLimit
	}
	// fetchComments flag handling
	// Note: flag default (true) matches config default, so we can safely use flag value
	cfg.FetchComments = *flagFetchComments

	// Validate configuration (but not credentials yet)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// executeCommand dispatches to the appropriate command handler.
func executeCommand(ctx context.Context, cfg *config.Config, command string, args []string, store storage.Store) error {
	switch command {
	case "me":
		// Create client for authentication-required command
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}
		return commandMe(ctx, cfg, client)

	case "subreddit":
		if len(args) != 1 {
			return fmt.Errorf("subreddit command requires exactly 1 argument: subreddit name")
		}
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}
		return commandSubreddit(ctx, cfg, client, args[0])

	case "hot":
		subreddit := ""
		if len(args) > 0 {
			subreddit = args[0]
		}
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}
		return commandHot(ctx, cfg, client, subreddit, store)

	case "new":
		subreddit := ""
		if len(args) > 0 {
			subreddit = args[0]
		}
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}
		return commandNew(ctx, cfg, client, subreddit, store)

	case "comments":
		if len(args) < 2 {
			return fmt.Errorf("comments command requires 2 arguments: subreddit post-id")
		}
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}
		return commandComments(ctx, cfg, client, args[0], args[1], store)

	case "more-comments":
		if len(args) < 2 {
			return fmt.Errorf("more-comments command requires at least 2 arguments: link-id comment-id [comment-id...]")
		}
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}
		linkID := args[0]
		commentIDs := args[1:]
		formatter, err := createFormatter(cfg.Output)
		if err != nil {
			return err
		}
		return commands.GetMoreComments(ctx, client, linkID, commentIDs, formatter, store)

	case "list-posts":
		if store == nil {
			return fmt.Errorf("storage not enabled (use --store flag)")
		}
		return commands.ListStoredPosts(ctx, store, cfg)

	case "stats":
		if store == nil {
			return fmt.Errorf("storage not enabled (use --store flag)")
		}
		return commands.ShowStats(ctx, store, cfg)

	case "monitor":
		if len(args) < 1 {
			return fmt.Errorf("monitor requires at least one subreddit (comma-separated for multiple)")
		}

		// Parse comma-separated subreddit list
		subreddits := strings.Split(args[0], ",")
		for i, sub := range subreddits {
			subreddits[i] = strings.TrimSpace(sub)
		}

		// Require storage for monitor command
		if store == nil {
			return fmt.Errorf("monitor command requires --store flag (e.g., --store --db-path ~/.reddit/data.db)")
		}

		// Create client
		if err := cfg.ValidateCredentials(); err != nil {
			return err
		}
		client, err := createRedditClient(ctx, cfg)
		if err != nil {
			return err
		}

		return commands.MonitorSubreddits(ctx, client, subreddits, cfg.MonitorInterval, cfg.MonitorDuration, cfg.MonitorLimit, cfg.FetchComments, store)

	default:
		return fmt.Errorf("unknown command: %q", command)
	}
}

// createRedditClient creates an authenticated Reddit API client.
func createRedditClient(ctx context.Context, cfg *config.Config) (*graw.Reddit, error) {
	// Create client with a longer timeout for authentication.
	authCtx, authCancel := context.WithTimeout(ctx, DefaultAuthTimeout)
	defer authCancel()

	client, err := graw.NewClientWithContext(authCtx, cfg.ToRedditConfig())
	if err != nil {
		return nil, err
	}
	return client, nil
}

// createFormatter creates a new formatter with the specified format.
func createFormatter(format string) (output.Formatter, error) {
	return output.New(output.Config{
		Writer:       os.Stdout,
		Format:       format,
		ColorEnabled: true,
		Compact:      false,
	})
}

// commandMe displays information about the authenticated user.
func commandMe(ctx context.Context, cfg *config.Config, client *graw.Reddit) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}
	return commands.GetMe(ctx, client, formatter)
}

// commandSubreddit displays information about a subreddit.
func commandSubreddit(ctx context.Context, cfg *config.Config, client *graw.Reddit, name string) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}
	return commands.GetSubreddit(ctx, client, name, formatter)
}

// commandHot fetches hot posts from a subreddit or front page.
func commandHot(ctx context.Context, cfg *config.Config, client *graw.Reddit, subreddit string, store storage.Store) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}

	pagination := types.Pagination{
		Limit:  cfg.Limit,
		After:  cfg.After,
		Before: cfg.Before,
	}

	return commands.GetHotPosts(ctx, client, subreddit, pagination, formatter, store)
}

// commandNew fetches new posts from a subreddit or front page.
func commandNew(ctx context.Context, cfg *config.Config, client *graw.Reddit, subreddit string, store storage.Store) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}

	pagination := types.Pagination{
		Limit:  cfg.Limit,
		After:  cfg.After,
		Before: cfg.Before,
	}

	return commands.GetNewPosts(ctx, client, subreddit, pagination, formatter, store)
}

// commandComments fetches comments for a specific post.
func commandComments(ctx context.Context, cfg *config.Config, client *graw.Reddit, subreddit, postID string, store storage.Store) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}

	pagination := types.Pagination{
		Limit:  cfg.Limit,
		After:  cfg.After,
		Before: cfg.Before,
	}

	return commands.GetComments(ctx, client, subreddit, postID, pagination, formatter, store)
}

// classifyError determines the appropriate exit code based on error type.
func classifyError(err error) int {
	if err == nil {
		return ExitOK
	}

	// Check for specific error types
	var configErr *graw.ConfigError
	var validErr *graw.ValidationError
	var authErr *graw.AuthError
	var netErr *graw.NetworkError

	if errors.As(err, &configErr) {
		return ExitValidation
	}
	if errors.As(err, &validErr) {
		return ExitValidation
	}
	if errors.As(err, &authErr) {
		return ExitAuth
	}
	if errors.As(err, &netErr) {
		return ExitNetwork
	}

	// Check for context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ExitNetwork
	}

	return ExitGeneralErr
}

// printError prints an error message to stderr.
func printError(command string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Error (%s): %v\n", command, err)
}

// printUsage prints the CLI usage information.
func printUsage() {
	fmt.Fprintf(os.Stderr, `Reddit CLI - Command-line interface to the Reddit API

Usage: reddit [flags] <command> [args]

Global Flags:
  -client-id string
        Reddit OAuth2 client ID (default: REDDIT_CLIENT_ID env var)
  -client-secret string
        Reddit OAuth2 client secret (default: REDDIT_CLIENT_SECRET env var)
  -username string
        Reddit username for user authentication (default: REDDIT_USERNAME env var)
  -password string
        Reddit password for user authentication (default: REDDIT_PASSWORD env var)
  -user-agent string
        Custom user agent string
  -output format
        Output format: json, table, or text (default: text)
  -limit int
        Maximum number of items to fetch (default: 25, max: 100)
  -after string
        Pagination token to fetch next page
  -before string
        Pagination token to fetch previous page
  -timeout duration
        HTTP request timeout (default: 30s)
  -verbose
        Enable verbose output
  -debug
        Enable debug logging
  -store
        Enable storage of posts and comments (env: REDDIT_STORE)
  -db-path string
        Path to SQLite database file (env: REDDIT_DB_PATH, default: ~/.reddit/data.db)
  -monitor-duration string
        How long to monitor (e.g., 1h, 30m). 0 means indefinite. (default: 0)
  -monitor-interval string
        Polling interval for monitor command (default: 5m)
  -monitor-limit int
        Posts per fetch for monitor command (default: 25)
  -fetch-comments
        Fetch comments for posts in monitor mode (default: true)

Commands:
  me                           Show authenticated user information
  subreddit <name>             Get information about a subreddit
  hot [subreddit]              Fetch hot posts (omit subreddit for front page)
  new [subreddit]              Fetch new posts (omit subreddit for front page)
  comments <sub> <post-id>     Get comments for a specific post
  more-comments <link-id> <id> Load additional comments by ID [id...]
  list-posts                   List all stored posts (requires --store)
  stats                        Show storage statistics (requires --store)
  monitor <subreddit[,subreddit2,...]>  Monitor subreddit(s) indefinitely (requires --store)
  help                         Show this help message

Examples:
  # Show authenticated user info
  reddit me

  # Fetch hot posts from golang subreddit
  reddit hot golang

  # Get new posts with custom limit and JSON output
  reddit -limit 50 -output json new programming

  # Get subreddit info
  reddit subreddit golang

  # Fetch comments for a post
  reddit comments golang abc123def

  # Load additional comments by ID
  reddit more-comments abc123def comment1 comment2

  # Fetch hot posts and store them
  reddit -store hot golang

  # List all stored posts
  reddit -store list-posts

  # Show storage statistics
  reddit -store stats

  # Monitor multiple subreddits with custom interval
  reddit -store -monitor-interval 10m monitor golang,programming,rust

  # Monitor for 1 hour with 5-minute intervals
  reddit -store -monitor-duration 1h -monitor-interval 5m monitor golang

  # Monitor indefinitely with custom interval and fetch comments
  reddit -store -monitor-interval 10m -fetch-comments monitor golang,programming

Set environment variables for credentials:
  export REDDIT_CLIENT_ID="your-client-id"
  export REDDIT_CLIENT_SECRET="your-client-secret"
`)
}
