# End-to-End Benchmark Suite

This package provides comprehensive end-to-end benchmarks for the Reddit API client that test against Reddit's real API infrastructure. These benchmarks measure real-world performance characteristics including network latency, API response times, authentication overhead, and rate limiting behavior.

## Overview

The E2E benchmark suite validates the Reddit API client's performance in production-like conditions by making actual API calls to Reddit's servers. Unlike unit benchmarks that test isolated components with mocks, these benchmarks measure:

- Complete request/response cycles including network overhead
- OAuth2 authentication flows (cold start, cached tokens, concurrent access)
- Rate limiting behavior and throttling mechanisms
- Pagination performance across different dataset sizes
- Real API response parsing and data extraction

## Prerequisites

These benchmarks require valid Reddit API credentials to run. You must set the following environment variables:

- `REDDIT_CLIENT_ID`: Your Reddit application's client ID
- `REDDIT_CLIENT_SECRET`: Your Reddit application's client secret

If credentials are not provided, benchmarks will be automatically skipped with an appropriate message.

## Setup Instructions

### 1. Obtain Reddit API Credentials

1. Visit [https://www.reddit.com/prefs/apps](https://www.reddit.com/prefs/apps)
2. Click "Create App" or "Create Another App" at the bottom
3. Fill in the form:
   - **name**: Choose any name (e.g., "go-reddit-benchmark")
   - **app type**: Select "script" for testing/development
   - **description**: Optional
   - **about url**: Optional
   - **redirect uri**: Use `http://localhost:8080` (required but unused for script apps)
4. Click "Create app"
5. Note your credentials:
   - **Client ID**: The string displayed under the app name (looks like `abc123xyz789`)
   - **Client secret**: The string labeled "secret"

### 2. Set Environment Variables

Export your credentials in your shell:

```bash
export REDDIT_CLIENT_ID="your-client-id-here"
export REDDIT_CLIENT_SECRET="your-client-secret-here"
```

To make these permanent, add them to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.).

### 3. Verify Setup

Run a simple benchmark to verify your credentials work:

```bash
go test -bench=BenchmarkE2E_GetHot/golang_10_posts -benchtime=1x ./benchmarks/e2e
```

If successful, you should see benchmark output. If credentials are invalid, you'll see an error message.

## Running Benchmarks

### Run All E2E Benchmarks

```bash
# Basic run with default settings
go test -bench=. ./benchmarks/e2e

# With memory allocation statistics
go test -bench=. -benchmem ./benchmarks/e2e

# With verbose output
go test -bench=. -benchmem -v ./benchmarks/e2e
```

### Run Specific Categories

```bash
# API endpoints only
go test -bench=BenchmarkE2E_Get ./benchmarks/e2e

# Authentication benchmarks only
go test -bench=BenchmarkE2E_Auth ./benchmarks/e2e
go test -bench=BenchmarkE2E_Token ./benchmarks/e2e

# Rate limiting benchmarks only
go test -bench=BenchmarkE2E_RateLimit ./benchmarks/e2e

# Pagination benchmarks only
go test -bench=BenchmarkE2E_Pagination ./benchmarks/e2e
```

### Run Specific Benchmarks

```bash
# Specific API endpoint benchmark
go test -bench=BenchmarkE2E_GetHot ./benchmarks/e2e

# Specific subreddit and limit combination
go test -bench=BenchmarkE2E_GetHot/golang_10_posts ./benchmarks/e2e

# Cold start authentication
go test -bench=BenchmarkE2E_TokenFetch_ColdStart ./benchmarks/e2e
```

### Customize Benchmark Execution

```bash
# Run for a specific duration (useful for rate limit tests)
go test -bench=. -benchtime=30s ./benchmarks/e2e

# Run a specific number of iterations
go test -bench=. -benchtime=10x ./benchmarks/e2e

# Limit CPU usage for benchmarks
go test -bench=. -cpu=1 ./benchmarks/e2e

# Generate CPU profile
go test -bench=. -cpuprofile=cpu.prof ./benchmarks/e2e

# Generate memory profile
go test -bench=. -memprofile=mem.prof ./benchmarks/e2e
```

### Profile Analysis

After generating profiles, analyze them with:

```bash
# CPU profile
go tool pprof cpu.prof

# Memory profile
go tool pprof mem.prof

# Web-based visualization
go tool pprof -http=:8080 cpu.prof
```

## Benchmark Categories

### 1. API Endpoints (`api_endpoints_test.go`)

Tests core Reddit API endpoints with real data:

- **`BenchmarkE2E_GetHot`**: Fetches hot/trending posts from subreddits
  - Tests small (r/golang), medium (r/programming), and large (r/AskReddit) subreddits
  - Varies post limits (10, 25, 50, 100) to measure pagination impact
  - Measures network latency, response parsing, and data extraction

- **`BenchmarkE2E_GetNew`**: Fetches newest posts from subreddits
  - Similar subreddit and limit variations as GetHot
  - Tests different sorting endpoint performance

- **`BenchmarkE2E_GetComments`**: Retrieves comments for specific posts
  - Tests single post comment fetching
  - Measures deeply nested comment tree parsing
  - Includes "more comments" continuation handling

- **`BenchmarkE2E_GetCommentsMultiple`**: Concurrent comment fetching
  - Tests worker pool pattern (max 10 concurrent requests)
  - Measures concurrency overhead and coordination
  - Validates semaphore-based throttling

### 2. Authentication (`auth_test.go`)

Tests OAuth2 authentication flows and token management:

- **`BenchmarkE2E_TokenFetch_ColdStart`**: Initial authentication from scratch
  - Simulates application startup scenario
  - Measures full OAuth2 round-trip time
  - No cached token state

- **`BenchmarkE2E_TokenFetch_Cached`**: Uses cached valid tokens
  - Best-case scenario with warm cache
  - Measures token retrieval overhead
  - Validates atomic pointer performance

- **`BenchmarkE2E_ConcurrentAuth`**: Concurrent authentication requests
  - Tests thread-safety of token cache
  - Validates mutex coordination during refresh
  - Measures contention under concurrent load

### 3. Rate Limiting (`ratelimit_test.go`)

Tests client behavior under Reddit's rate limits:

- **`BenchmarkE2E_RateLimitHeaders`**: Header processing overhead
  - Measures extraction of `X-Ratelimit-Remaining`, `X-Ratelimit-Reset`, `Retry-After`
  - Validates no significant latency from header parsing
  - Tests rate limit tracking accuracy

- **`BenchmarkE2E_BurstRequests`**: Rapid successive requests
  - Sends multiple requests in quick succession
  - Measures client throttling behavior
  - Validates rate limiter effectiveness

- **`BenchmarkE2E_SustainedLoad`**: Long-running sustained request pattern
  - Tests behavior over extended periods
  - Measures recovery after approaching limits
  - Validates sliding window implementation

**Warning**: Rate limit benchmarks consume significant API quota and may take several minutes to complete.

### 4. Pagination (`pagination_test.go`)

Tests Reddit's cursor-based pagination:

- **`BenchmarkE2E_PaginationCursors`**: Cursor handling mechanics
  - Validates `AfterFullname` extraction and usage
  - Tests forward and backward pagination
  - Ensures no duplicate posts between pages
  - Verifies cursor format (e.g., "t3_xxxxx")

- **`BenchmarkE2E_LargeDatasetPagination`**: Multi-page traversal
  - Fetches multiple pages sequentially
  - Measures performance degradation over pages
  - Tests cursor state maintenance

- **`BenchmarkE2E_PageSizeVariations`**: Different page size efficiency
  - Compares small (10), medium (25), large (50), and maximum (100) page sizes
  - Measures total time vs throughput trade-offs
  - Identifies optimal page size for different use cases

## Important Notes

### Real API Calls

These benchmarks make actual HTTP requests to Reddit's production API servers. This means:

- Benchmarks require internet connectivity
- Results depend on network conditions and Reddit's server load
- Benchmarks consume your API rate limit quota
- Geographic distance from Reddit's servers affects latency

### Rate Limits

Reddit enforces rate limits on OAuth2 applications:

- **Limit**: 600 requests per 10 minutes (60 requests/minute average)
- **Window**: Sliding window, not fixed intervals
- **Headers**: Reddit returns `X-Ratelimit-Remaining` and `X-Ratelimit-Reset` in responses
- **Enforcement**: The client automatically throttles to respect these limits

Running multiple benchmarks in succession may trigger rate limiting. If you encounter delays, this is expected behavior as the client waits for quota to replenish.

### Execution Time

Some benchmarks may take considerable time:

- **Quick** (seconds): API endpoint benchmarks with few iterations
- **Moderate** (1-2 minutes): Authentication and pagination benchmarks
- **Slow** (5+ minutes): Rate limit benchmarks that test throttling and recovery

Use `-benchtime=1x` to run each benchmark only once for faster validation.

### Fixtures and Test Data

Real API responses are captured and saved to the `testdata/` directory:

- Fixtures provide offline examples of actual API response structures
- Filenames indicate the operation and timestamp (e.g., `hot_golang_20250123.json`)
- Useful for comparing response formats across API versions
- Support development and debugging without live API access
- Document real-world data schemas and edge cases

Fixtures are automatically generated during benchmark runs and are not committed to version control.

## Interpreting Results

Benchmark output follows Go's standard format:

```
BenchmarkE2E_GetHot/golang_10_posts-8    50    234567890 ns/op    12345 B/op    123 allocs/op
```

Breaking down each field:

- **`BenchmarkE2E_GetHot/golang_10_posts-8`**: Benchmark name and CPU count
- **`50`**: Number of iterations run
- **`234567890 ns/op`**: Average time per operation (nanoseconds)
- **`12345 B/op`**: Average bytes allocated per operation (with `-benchmem`)
- **`123 allocs/op`**: Average number of allocations per operation (with `-benchmem`)

### Understanding Times

Convert nanoseconds to human-readable units:

- 1,000,000 ns = 1 ms (millisecond)
- 1,000,000,000 ns = 1 s (second)

Example: `234567890 ns/op` = ~235 ms per request

### What's Normal?

Typical performance ranges (highly dependent on network):

- **API calls**: 100-500 ms (network latency dominates)
- **Cold auth**: 200-800 ms (includes OAuth2 round-trip)
- **Cached auth**: <1 ms (atomic pointer read)
- **Concurrent auth**: 10-50 ms (mutex coordination)

Significant deviations suggest network issues or Reddit API performance degradation.

### Comparing Results

Run benchmarks multiple times and compare:

```bash
# First run
go test -bench=BenchmarkE2E_GetHot -benchmem ./benchmarks/e2e > old.txt

# After code changes
go test -bench=BenchmarkE2E_GetHot -benchmem ./benchmarks/e2e > new.txt

# Compare with benchstat
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

## API Quota Considerations

### Quota Consumption

Each benchmark iteration consumes API quota:

- **GetHot/GetNew**: 1 request per iteration
- **GetComments**: 1-2 requests per iteration (may include "more comments")
- **GetCommentsMultiple**: N requests per iteration (where N = number of posts)
- **Auth benchmarks**: 1 token request per unique client
- **Rate limit benchmarks**: Many requests (50-100+)

### Best Practices

To avoid exhausting your quota:

1. **Limit iterations**: Use `-benchtime=10x` instead of default duration
2. **Run selectively**: Focus on specific benchmarks instead of entire suite
3. **Monitor quota**: Check `X-Ratelimit-Remaining` in verbose output
4. **Off-peak hours**: Run during low-traffic times for better consistency
5. **Staging credentials**: Use separate test credentials if available

### Recovery from Rate Limiting

If you exhaust your quota:

- The client will automatically throttle and wait for quota replenishment
- Rate limits reset on a sliding window (quota returns gradually)
- Wait 10-15 minutes for full quota recovery
- Monitor `X-Ratelimit-Reset` header for exact reset time

## Troubleshooting

### Benchmarks Skipped

If you see "Skipping benchmark: credentials not configured":

1. Verify environment variables are set: `echo $REDDIT_CLIENT_ID`
2. Check credentials are valid (no typos, correct app type)
3. Ensure variables are exported in the current shell session

### Authentication Failures

If benchmarks fail with "401 Unauthorized":

1. Verify your client ID and secret are correct
2. Ensure your Reddit app type is "script" (not "web app" or "installed app")
3. Check your Reddit account is in good standing (not suspended/banned)

### Network Errors

If you see "connection refused" or "timeout" errors:

1. Verify internet connectivity
2. Check if Reddit is accessible: `curl -I https://www.reddit.com`
3. Verify no firewall/proxy is blocking outbound HTTPS
4. Consider network latency if on slow/unreliable connection

### Slow Performance

If benchmarks are much slower than expected:

1. Check network latency: `ping reddit.com`
2. Verify no rate limiting is occurring (check verbose output)
3. Consider geographic distance from Reddit's servers
4. Run during off-peak hours
5. Check system resources (CPU, memory)

## Contributing

When adding new benchmarks:

1. Follow existing naming conventions: `BenchmarkE2E_<Operation>_<Variant>`
2. Include comprehensive comments explaining what's being measured
3. Use `skipIfNoCredentials(b)` to handle missing credentials gracefully
4. Add memory statistics with `-benchmem` flag
5. Consider quota consumption (avoid excessive iterations)
6. Save relevant fixtures to testdata/ for documentation

## Related Documentation

- **Unit Benchmarks**: See `reddit/internal/` for component-level benchmarks
- **Package Documentation**: Run `go doc -all github.com/jamesprial/go-reddit-api-wrapper/benchmarks/e2e`
- **Reddit API Docs**: [https://www.reddit.com/dev/api](https://www.reddit.com/dev/api)
- **OAuth2 Setup**: [https://github.com/reddit-archive/reddit/wiki/OAuth2](https://github.com/reddit-archive/reddit/wiki/OAuth2)
