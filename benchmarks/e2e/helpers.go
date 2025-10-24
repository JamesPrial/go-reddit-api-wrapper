package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
)

// newE2EClient creates a Reddit client configured for E2E benchmarks against the real Reddit API.
// This function reads credentials from environment variables and returns a fully authenticated client
// ready to make real API calls.
//
// Required environment variables:
//   - REDDIT_CLIENT_ID: OAuth2 client ID from Reddit app preferences
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret from Reddit app preferences
//
// Optional environment variables:
//   - REDDIT_USERNAME: Reddit username for user authentication (if omitted, uses app-only auth)
//   - REDDIT_PASSWORD: Reddit password for user authentication (if omitted, uses app-only auth)
//
// The client is configured with:
//   - Default rate limiting (100 requests/min with burst of 10)
//   - Standard timeout (30 seconds)
//   - No debug logging to minimize benchmark overhead
//
// If client creation fails (missing credentials, authentication failure, etc.), the benchmark
// is marked as failed using b.Fatal().
func newE2EClient(b *testing.B) *graw.Reddit {
	b.Helper()

	// Read credentials from environment
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	username := os.Getenv("REDDIT_USERNAME")
	password := os.Getenv("REDDIT_PASSWORD")

	// Validate required credentials
	if clientID == "" || clientSecret == "" {
		b.Fatal("REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET environment variables are required for E2E benchmarks")
	}

	// Create client configuration
	config := &graw.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "go-reddit-api-wrapper:e2e-benchmarks:v1.0.0 (by /u/e2e-test-suite)",
	}

	// Add user credentials if provided (for user auth flow)
	if username != "" && password != "" {
		config.Username = username
		config.Password = password
	} else if username != "" || password != "" {
		b.Logf("Warning: Only one of REDDIT_USERNAME/REDDIT_PASSWORD provided, using app-only auth")
	}

	// Create authenticated client with 30-second timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := graw.NewClientWithContext(ctx, config)
	if err != nil {
		b.Fatalf("Failed to create Reddit client: %v", err)
	}

	return client
}
