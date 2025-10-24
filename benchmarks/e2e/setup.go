package e2e

import (
	"os"
	"testing"
)

// skipIfNoCredentials checks for required Reddit API credentials in environment
// variables and skips the benchmark if they are not present.
//
// This function is designed for E2E (end-to-end) benchmarks that test against
// Reddit's real API. Since these benchmarks require valid OAuth2 credentials,
// this helper ensures they are skipped gracefully when credentials are not
// available (e.g., in CI environments or during local development without setup).
//
// Required environment variables:
//   - REDDIT_CLIENT_ID: OAuth2 client ID from Reddit app registration
//   - REDDIT_CLIENT_SECRET: OAuth2 client secret from Reddit app registration
//
// Usage:
//
//	func BenchmarkE2E_GetHot(b *testing.B) {
//	    skipIfNoCredentials(b)
//	    // ... benchmark code that uses real Reddit API ...
//	}
func skipIfNoCredentials(b *testing.B) {
	b.Helper()

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		b.Skip("Skipping E2E benchmark: REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET environment variables must be set")
	}
}
