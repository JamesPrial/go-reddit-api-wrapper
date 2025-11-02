// Command reddit provides a CLI interface to the Reddit API.
// It supports authentication, fetching posts, comments, and subreddit information.
//
// Usage:
//
//	reddit [flags] <command> [args]
//
// Global Flags:
//
//	-client-id string
//		Reddit OAuth2 client ID (default: REDDIT_CLIENT_ID env var)
//	-client-secret string
//		Reddit OAuth2 client secret (default: REDDIT_CLIENT_SECRET env var)
//	-username string
//		Reddit username for user authentication (default: REDDIT_USERNAME env var)
//	-password string
//		Reddit password for user authentication (default: REDDIT_PASSWORD env var)
//	-user-agent string
//		Custom user agent string (default: auto-generated)
//	-output format
//		Output format: json, table, or text (default: text)
//	-limit int
//		Maximum number of items to fetch (default: 25, max: 100)
//	-after string
//		Pagination token to fetch next page
//	-before string
//		Pagination token to fetch previous page
//	-verbose
//		Enable verbose output
//	-debug
//		Enable debug logging
//
// Commands:
//
//	me                       Show authenticated user information
//	subreddit <name>         Get information about a subreddit
//	hot <subreddit>          Fetch hot posts from a subreddit (or front page if omitted)
//	new <subreddit>          Fetch new posts from a subreddit (or front page if omitted)
//	comments <sub> <post-id> Get comments for a specific post
//
// Examples:
//
//	# Set up credentials
//	export REDDIT_CLIENT_ID="your-client-id"
//	export REDDIT_CLIENT_SECRET="your-client-secret"
//
//	# Show authenticated user info
//	reddit me
//
//	# Fetch hot posts from golang subreddit
//	reddit hot golang
//
//	# Get new posts with custom limit and JSON output
//	reddit -limit 50 -output json new programming
//
//	# Get subreddit info
//	reddit subreddit golang
//
//	# Fetch comments for a post
//	reddit comments golang abc123def
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/commands"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/config"
	"github.com/jamesprial/go-reddit-api-wrapper/cmd/reddit/output"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// Exit codes used by the CLI.
const (
	ExitOK         = 0 // Success
	ExitGeneralErr = 1 // General/unhandled error
	ExitValidation = 2 // Validation error
	ExitAuth       = 3 // Authentication error
	ExitNetwork    = 4 // Network error
)

// Global flags
var (
	flagClientID     = flag.String("client-id", "", "Reddit OAuth2 client ID (env: REDDIT_CLIENT_ID)")
	flagClientSecret = flag.String("client-secret", "", "Reddit OAuth2 client secret (env: REDDIT_CLIENT_SECRET)")
	flagUsername     = flag.String("username", "", "Reddit username for user auth (env: REDDIT_USERNAME)")
	flagPassword     = flag.String("password", "", "Reddit password for user auth (env: REDDIT_PASSWORD)")
	flagUserAgent    = flag.String("user-agent", "", "Custom user agent string")
	flagOutput       = flag.String("output", "text", "Output format: json, table, or text")
	flagLimit        = flag.Int("limit", 25, "Max items to fetch (1-100)")
	flagAfter        = flag.String("after", "", "Pagination token for next page")
	flagBefore       = flag.String("before", "", "Pagination token for previous page")
	flagVerbose      = flag.Bool("verbose", false, "Enable verbose output")
	flagDebug        = flag.Bool("debug", false, "Enable debug logging")
	flagTimeout      = flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
)

func main() {
	flag.Parse()

	// Load configuration from environment and apply flag overrides
	cfg, err := loadConfig()
	if err != nil {
		printError("Configuration error", err)
		os.Exit(ExitValidation)
	}

	// Get command and arguments
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(ExitValidation)
	}

	command := args[0]
	commandArgs := args[1:]

	// Create client with a longer timeout for authentication
	// Use 60s for client creation to allow slow auth, separate from command timeout
	authCtx, authCancel := context.WithTimeout(context.Background(), 60*time.Second)
	client, err := graw.NewClientWithContext(authCtx, cfg.ToRedditConfig())
	authCancel()

	if err != nil {
		printError("authentication", err)
		os.Exit(classifyError(err))
	}

	// Create context with timeout for actual command execution
	cmdCtx, cmdCancel := context.WithTimeout(context.Background(), *flagTimeout)
	defer cmdCancel()

	// Execute command with shared client
	if err := executeCommand(cmdCtx, cfg, command, commandArgs, client); err != nil {
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

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// executeCommand dispatches to the appropriate command handler.
func executeCommand(ctx context.Context, cfg *config.Config, command string, args []string, client *graw.Reddit) error {
	switch command {
	case "me":
		return commandMe(ctx, cfg, client)
	case "subreddit":
		if len(args) != 1 {
			return fmt.Errorf("subreddit command requires exactly 1 argument: subreddit name")
		}
		return commandSubreddit(ctx, cfg, client, args[0])
	case "hot":
		subreddit := ""
		if len(args) > 0 {
			subreddit = args[0]
		}
		return commandHot(ctx, cfg, client, subreddit)
	case "new":
		subreddit := ""
		if len(args) > 0 {
			subreddit = args[0]
		}
		return commandNew(ctx, cfg, client, subreddit)
	case "comments":
		if len(args) < 2 {
			return fmt.Errorf("comments command requires 2 arguments: subreddit post-id")
		}
		return commandComments(ctx, cfg, client, args[0], args[1])
	case "help", "-h", "--help", "-help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %q", command)
	}
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
func commandHot(ctx context.Context, cfg *config.Config, client *graw.Reddit, subreddit string) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}

	pagination := types.Pagination{
		Limit:  cfg.Limit,
		After:  cfg.After,
		Before: cfg.Before,
	}

	return commands.GetHotPosts(ctx, client, subreddit, pagination, formatter)
}

// commandNew fetches new posts from a subreddit or front page.
func commandNew(ctx context.Context, cfg *config.Config, client *graw.Reddit, subreddit string) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}

	pagination := types.Pagination{
		Limit:  cfg.Limit,
		After:  cfg.After,
		Before: cfg.Before,
	}

	return commands.GetNewPosts(ctx, client, subreddit, pagination, formatter)
}

// commandComments fetches comments for a specific post.
func commandComments(ctx context.Context, cfg *config.Config, client *graw.Reddit, subreddit, postID string) error {
	formatter, err := createFormatter(cfg.Output)
	if err != nil {
		return err
	}

	pagination := types.Pagination{
		Limit:  cfg.Limit,
		After:  cfg.After,
		Before: cfg.Before,
	}

	return commands.GetComments(ctx, client, subreddit, postID, pagination, formatter)
}

// classifyError determines the appropriate exit code based on error type.
func classifyError(err error) int {
	if err == nil {
		return ExitOK
	}

	// Check for specific error types
	var validErr *graw.ValidationError
	var authErr *graw.AuthError
	var netErr *graw.NetworkError

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

Commands:
  me                       Show authenticated user information
  subreddit <name>         Get information about a subreddit
  hot [subreddit]          Fetch hot posts (omit subreddit for front page)
  new [subreddit]          Fetch new posts (omit subreddit for front page)
  comments <sub> <post-id> Get comments for a specific post
  help                     Show this help message

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

Set environment variables for credentials:
  export REDDIT_CLIENT_ID="your-client-id"
  export REDDIT_CLIENT_SECRET="your-client-secret"
`)
}
