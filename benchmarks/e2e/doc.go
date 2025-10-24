// Package e2e provides end-to-end benchmarks for the Reddit API client that test
// against Reddit's real API infrastructure. These benchmarks measure real-world
// performance characteristics including network latency, API response times,
// authentication overhead, and rate limiting behavior.
//
// # Requirements
//
// These benchmarks require valid Reddit API credentials to run. The following
// environment variables must be set:
//
//   - REDDIT_CLIENT_ID: Your Reddit application's client ID
//   - REDDIT_CLIENT_SECRET: Your Reddit application's client secret
//
// If these credentials are not provided, the benchmarks will be automatically
// skipped with an appropriate message.
//
// # Setup
//
// To obtain Reddit API credentials:
//
//  1. Visit https://www.reddit.com/prefs/apps
//  2. Create a new application (select "script" type for testing)
//  3. Note the client ID (under the app name) and client secret
//
// Set the credentials in your environment:
//
//	export REDDIT_CLIENT_ID="your-client-id-here"
//	export REDDIT_CLIENT_SECRET="your-client-secret-here"
//
// # Running Benchmarks
//
// Run all E2E benchmarks:
//
//	go test -bench=. ./benchmarks/e2e
//
// Run with verbose output:
//
//	go test -bench=. -benchmem -v ./benchmarks/e2e
//
// Run a specific benchmark:
//
//	go test -bench=BenchmarkE2E_GetHot ./benchmarks/e2e
//
// Run with custom duration:
//
//	go test -bench=. -benchtime=30s ./benchmarks/e2e
//
// # Fixtures and Caching
//
// Real API responses are captured and stored as fixtures in the testdata/
// directory. These fixtures serve multiple purposes:
//
//   - Provide offline examples of actual API response structures
//   - Enable comparison of response formats across API versions
//   - Support development and testing without live API access
//   - Document real-world data schemas and edge cases
//
// Fixtures are automatically generated during benchmark runs and are stored
// with descriptive filenames indicating the operation and timestamp.
//
// # Performance Considerations
//
// These benchmarks measure real-world performance and are subject to:
//
//   - Network latency and bandwidth variations
//   - Reddit API server response times and load
//   - Rate limiting imposed by Reddit (600 requests per 10 minutes)
//   - Geographic distance from Reddit's servers
//
// For consistent results, run benchmarks multiple times and consider the median
// values. Use -benchtime to increase the number of iterations for more stable
// measurements.
//
// # Rate Limiting
//
// The client automatically respects Reddit's rate limits. During benchmark runs,
// you may observe throttling behavior as the client approaches rate limit
// thresholds. This is expected and demonstrates the client's real-world behavior.
//
// # Best Practices
//
//   - Run benchmarks during off-peak hours for more consistent results
//   - Use -benchmem to track memory allocations
//   - Compare results across different network conditions
//   - Review captured fixtures to understand API response patterns
//   - Avoid running excessive benchmarks to respect Reddit's API limits
package e2e
