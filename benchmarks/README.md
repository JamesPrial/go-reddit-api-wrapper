# Go Reddit API Wrapper - Benchmark Suite

This directory contains comprehensive benchmarks for the Go Reddit API Wrapper, measuring real-world performance characteristics and comparing design tradeoffs.

## Overview

The benchmark suite consists of two complementary types of benchmarks:

1. **Scenario Benchmarks** (`reddit/benchmark_test.go`) - Simulate realistic user workflows like monitoring subreddits, analyzing threads, bulk fetching, and concurrent operations
2. **Comparative Benchmarks** (`benchmarks/comparative/reddit_comparison_test.go`) - Compare performance tradeoffs between different implementation approaches (buffer pooling, rate limiting, typed errors, etc.)

All benchmarks use mock HTTP servers and validated JSON fixtures to ensure consistent, reproducible measurements without making real API calls.

## Benchmark Structure

### Scenario Benchmarks (`reddit/benchmark_test.go`)

These benchmarks measure complete workflows that real applications would perform:

- **MonitorSubreddit** - Continuous polling for new posts (simulates bot monitoring)
- **AnalyzeThread** - Deep comment tree traversal and analysis (simulates research tools)
- **BulkFetch** - Fetching comments from multiple posts concurrently (simulates data collection)
- **UserActivityTracking** - Tracking user activity across subreddits (simulates profile analysis)
- **TrendingTopics** - Identifying trending topics across multiple subreddits (simulates trend analysis)
- **ConcurrentFetch** - Concurrent vs sequential subreddit fetching (measures parallelism benefits)
- **ContextCancellation** - Cancellation response time and cleanup (measures graceful shutdown)

**Key Characteristics:**
- Use `MockClock` for deterministic timing (no real delays)
- Report allocations for memory profiling
- Exclude setup/fixture loading from timing measurements
- Test realistic parameter combinations (poll intervals, post counts, concurrency levels)

### Comparative Benchmarks (`benchmarks/comparative/reddit_comparison_test.go`)

These benchmarks compare implementation approaches to quantify tradeoffs:

**Comparison 1: Our Client vs Raw HTTP**
- Measures overhead of abstractions (auth caching, rate limiting, type-safe parsing)
- Baseline: stdlib `http.Client` + `json.Unmarshal` into `map[string]interface{}`

**Comparison 2: Buffer Pooling (With vs Without)**
- Measures memory efficiency gains from `sync.Pool`
- Tests allocation reduction across different response sizes

**Comparison 3: Rate Limiting (With vs Without)**
- Measures throughput protection vs performance cost
- Tests burst request patterns with/without rate limiting

**Comparison 4: Typed Errors vs Generic Errors**
- Measures structured error handling overhead
- Tests error creation, type checking (`errors.As`), and chain traversal

**Comparison 5: Concurrent Request Patterns**
- Measures buffer pool effectiveness under concurrent load
- Tests rate limiter fairness across goroutines
- Measures worker pool pattern efficiency (`GetCommentsMultiple`)

## Running Benchmarks

### Run All Benchmarks

```bash
# Run all benchmarks with default settings
go test -bench=. ./reddit ./benchmarks/comparative

# Run with verbose output and memory allocations
go test -bench=. -benchmem -v ./reddit ./benchmarks/comparative
```

### Run Specific Benchmark Categories

```bash
# Run only scenario benchmarks
go test -bench=. -benchmem ./reddit

# Run only comparative benchmarks
go test -bench=. -benchmem ./benchmarks/comparative

# Run specific scenario
go test -bench=BenchmarkScenario_MonitorSubreddit -benchmem ./reddit

# Run specific comparison
go test -bench=BenchmarkComparison_GetPosts -benchmem ./benchmarks/comparative
```

### Filter by Name Pattern

```bash
# Run all MonitorSubreddit variants
go test -bench=MonitorSubreddit -benchmem ./reddit

# Run all buffer pooling comparisons
go test -bench=Pool -benchmem ./benchmarks/comparative

# Run all rate limiting tests
go test -bench=RateLimit -benchmem ./benchmarks/comparative
```

### Performance Testing Options

```bash
# Run with race detector (slower but catches race conditions)
go test -bench=. -race -benchmem ./reddit

# Run longer for more accurate results
go test -bench=. -benchtime=10s -benchmem ./reddit

# Run with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./reddit

# Run with memory profiling
go test -bench=. -memprofile=mem.prof -benchmem ./reddit

# Analyze profiles
go tool pprof cpu.prof
go tool pprof mem.prof
```

### CI/CD Integration

The benchmark suite runs in GitHub Actions to detect performance regressions:

```bash
# Run benchmarks exactly as CI does
go test -bench=. -benchmem -benchtime=1s ./reddit ./benchmarks/comparative
```

## Interpreting Results

Benchmark output shows several key metrics:

```
BenchmarkScenario_MonitorSubreddit/fast_poll_10posts_5iterations-8    1000    1234567 ns/op    12345 B/op    123 allocs/op
```

### Metrics Explained

- **`-8`** - Number of CPUs (GOMAXPROCS)
- **`1000`** - Number of iterations run (higher is better for accuracy)
- **`ns/op`** - Nanoseconds per operation (lower is better)
  - 1,000 ns = 1 microsecond (µs)
  - 1,000,000 ns = 1 millisecond (ms)
  - 1,000,000,000 ns = 1 second
- **`B/op`** - Bytes allocated per operation (lower is better)
- **`allocs/op`** - Number of allocations per operation (lower is better)
- **`MB/s`** - Throughput in megabytes per second (higher is better, shown when using `b.SetBytes()`)

### Performance Characteristics

Based on benchmark measurements, here are the documented tradeoffs:

#### Buffer Pooling Benefits
- **Allocation Reduction**: 20-40% fewer allocations for medium-large responses
- **GC Pressure**: Reduced garbage collection frequency under sustained load
- **Tradeoff**: Small overhead for pool management (~2-5% CPU time)
- **Best For**: High-throughput applications making many requests

#### Rate Limiting Cost
- **Throughput Impact**: Proportional to configured rate (30 req/s = 33ms average wait)
- **Overhead**: Minimal when under limit (~100ns per request for token check)
- **Protection**: Prevents 429 errors that would require exponential backoff
- **Best For**: Production applications that must respect API limits

#### Typed Errors vs Generic
- **Creation**: Typed errors can be faster (struct init vs fmt.Sprintf formatting)
- **Type Checking**: `errors.As()` adds ~50-100ns per check
- **Benefit**: Type-safe field access without string parsing
- **Best For**: Applications that need structured error handling and retry logic

#### Concurrent vs Sequential
- **Speedup**: Near-linear up to ~10 concurrent requests, then diminishing returns
- **Overhead**: Goroutine management adds ~1-2µs per request
- **Worker Pool**: Limits concurrency to prevent resource exhaustion
- **Best For**: Fetching multiple independent resources (posts, comments from different threads)

#### Auth Token Caching
- **Cache Hit**: ~100ns (atomic pointer read)
- **Cache Miss**: 50-100ms (OAuth2 token acquisition)
- **Amortization**: Token valid for 3600s, so cost is negligible over time
- **Best For**: All applications (always enabled, minimal overhead)

## Fixture Management

### Location and Organization

Fixtures are stored in `benchmarks/testdata/`:

```
benchmarks/testdata/
├── README.md                  # Fixture documentation
├── small_posts.json          # 10 posts, 17 KB
├── medium_posts.json         # 100 posts, 138 KB
├── large_posts.json          # 1000 posts, 1.4 MB
├── deep_comments.json        # 50 levels deep, 506 KB
└── wide_comments.json        # 100 top-level + 5 replies each, 716 KB
```

### Fixture Formats

All fixtures conform to Reddit's API response format:

**Posts** (`kind: "Listing"`):
```json
{
  "kind": "Listing",
  "data": {
    "children": [
      {
        "kind": "t3",
        "data": { /* post fields */ }
      }
    ],
    "after": "t3_...",
    "before": null
  }
}
```

**Comments** (array with `[post, comments]`):
```json
[
  {
    "kind": "Listing",
    "data": {
      "children": [{ "kind": "t3", "data": { /* post */ } }]
    }
  },
  {
    "kind": "Listing",
    "data": {
      "children": [
        {
          "kind": "t1",
          "data": {
            /* comment fields */
            "replies": { /* nested Listing */ }
          }
        }
      ]
    }
  }
]
```

### Fixture Validation

All fixtures are validated during benchmark setup:

1. **JSON Validity**: Must parse with `json.Unmarshal`
2. **Structure**: Must have `kind` and `data` fields
3. **Content**: Must contain at least one child element
4. **Type**: Must match expected format (posts vs comments)

Validation happens **before** benchmark timing starts to exclude I/O overhead.

### Adding New Fixtures

To add a new fixture:

1. **Create JSON file** in `benchmarks/testdata/`:
   ```bash
   # Example: Create a new fixture
   cat > benchmarks/testdata/my_fixture.json <<EOF
   {
     "kind": "Listing",
     "data": {
       "children": [ /* your data */ ]
     }
   }
   EOF
   ```

2. **Validate structure** with Go:
   ```go
   data, _ := os.ReadFile("benchmarks/testdata/my_fixture.json")
   var thing types.Thing
   if err := json.Unmarshal(data, &thing); err != nil {
       log.Fatal(err)
   }
   ```

3. **Use in benchmarks**:
   ```go
   fixture := loadScenarioFixture(b, "my_fixture.json")
   server := setupMockServer(fixture)
   ```

4. **Document in testdata/README.md** with:
   - File size
   - Content description
   - Use case
   - Key features

### Fixture Size Guidelines

- **Small** (10-50 items): Quick tests, basic parsing validation
- **Medium** (100-500 items): Realistic workloads, memory profiling
- **Large** (1000+ items): Stress testing, allocation efficiency

## Troubleshooting Guide

### "Fixture Not Found" Errors

**Error**: `failed to load fixture X: no such file or directory`

**Cause**: Benchmark running from unexpected working directory

**Solution**:
```bash
# Check current directory when running benchmarks
pwd

# Ensure fixtures exist
ls -la benchmarks/testdata/

# Run benchmarks from project root
cd /path/to/go-reddit-api-wrapper
go test -bench=. ./reddit ./benchmarks/comparative
```

**Advanced**: Fixture loader tries multiple paths automatically:
```go
candidatePaths := []string{
    "benchmarks/testdata/file.json",      // From project root
    "../testdata/file.json",              // From benchmarks/comparative/
    "../../benchmarks/testdata/file.json", // From nested dirs
}
```

### Race Detector Failures

**Error**: `WARNING: DATA RACE`

**Cause**: Concurrent access to shared state without synchronization

**Solution**:
```bash
# Run with race detector to identify exact location
go test -bench=BenchmarkName -race ./reddit

# Common causes:
# 1. Missing mutex around shared map/slice
# 2. Goroutine accessing parent scope variable
# 3. Multiple goroutines writing to same field
```

**Prevention**: All concurrent code uses proper synchronization:
- `sync.Mutex` for shared state
- Atomic operations for counters
- Channels for communication

### Benchmark Hangs or Timeouts

**Error**: Benchmark doesn't complete, stuck waiting

**Cause**: Goroutine deadlock or context not respecting cancellation

**Solution**:
```bash
# Add timeout to prevent infinite hangs
go test -bench=. -timeout=5m ./reddit

# Debug with prints (temporarily):
# 1. Add log.Printf before/after goroutine spawn
# 2. Add defer log.Printf in goroutines
# 3. Check for missing channel receives/sends
```

**Common Issues**:
- Goroutine waiting on never-sent channel
- Semaphore acquired but not released (use `defer`)
- Context cancellation not checked in tight loops

### Unexpected Performance Results

**Issue**: Benchmark shows unexpected slowness or high allocations

**Investigation Steps**:

1. **Check fixture size**:
   ```bash
   ls -lh benchmarks/testdata/
   # Ensure you're using the expected fixture size
   ```

2. **Profile the benchmark**:
   ```bash
   go test -bench=BenchmarkName -cpuprofile=cpu.prof ./reddit
   go tool pprof -http=:8080 cpu.prof
   # Look for unexpected hot spots
   ```

3. **Check allocations**:
   ```bash
   go test -bench=BenchmarkName -memprofile=mem.prof -benchmem ./reddit
   go tool pprof -http=:8080 -alloc_space mem.prof
   # Identify allocation sources
   ```

4. **Isolate the issue**:
   ```bash
   # Run just the slow benchmark
   go test -bench=BenchmarkName -benchtime=1x ./reddit

   # Compare with baseline
   go test -bench=BenchmarkComparison_GetPosts_RawHTTP -benchmem ./benchmarks/comparative
   ```

### Context Cancellation Issues

**Issue**: Benchmark doesn't respect context cancellation

**Symptoms**:
- Goroutines continue after context cancelled
- High `cancelledCount` but operations still complete
- Deadlock in cancellation scenarios

**Solution**:
```go
// Always check context in goroutines:
select {
case sem <- struct{}{}:
    // Acquired semaphore, proceed
case <-ctx.Done():
    // Context cancelled, exit immediately
    return
}

// Check context before expensive operations:
if ctx.Err() != nil {
    return ctx.Err()
}
```

**Testing**:
```bash
# Run context cancellation benchmarks specifically
go test -bench=ContextCancellation -v ./reddit
```

## CI/CD Integration

### GitHub Actions Workflow

Benchmarks run automatically in GitHub Actions with two separate jobs:

**Unit Benchmarks** (`benchmark-unit` job):
- Runs fast unit-level benchmarks from internal packages
- Excludes slow HTTP integration benchmarks
- Command:
  ```bash
  go test -bench=. -benchmem -benchtime=3s \
    ./pkg/validation \
    ./reddit/internal/auth \
    ./reddit/internal/parse \
    -run=^$

  go test -bench="^Benchmark(BufferPool|RateLimit|Client_NewRequest|ResponseBody|JSON|Extract|Truncate|BuildLimiter|DeferRequests)" \
    -benchmem -benchtime=3s \
    ./reddit/internal/client \
    -run=^$
  ```

**E2E Scenario Benchmarks** (`benchmark-e2e` job):
- Runs comprehensive scenario benchmarks
- Only includes fast scenario variants
- Command:
  ```bash
  go test -bench="BenchmarkScenario_(MonitorSubreddit/(fast_poll|medium_poll)|AnalyzeThread/(shallow|deep_no_more)|BulkFetch/small|UserActivityTracking/recent|TrendingTopics/3subs.*sequential|ConcurrentFetch/(3subs|5subs_25posts_sequential)|ContextCancellation/(immediate|no_cancel))" \
    -benchmem -benchtime=1s \
    ./reddit \
    -run=^$
  ```

### Performance Tracking with GitHub Pages

Benchmark results are automatically tracked over time and published to GitHub Pages:

**Setup (One-time)**:
1. Go to repository **Settings** > **Pages**
2. Under **Source**, select:
   - Branch: `gh-pages`
   - Folder: `/ (root)`
3. Click **Save**

**Viewing Results**:
- Visit: `https://YOUR_USERNAME.github.io/go-reddit-api-wrapper/`
- Two separate dashboards:
  - `/benchmarks/unit/` - Unit benchmark trends
  - `/benchmarks/e2e/` - E2E scenario benchmark trends

**Features**:
- 📊 Interactive charts showing performance over time
- 📈 Trend visualization for ns/op, B/op, and allocs/op metrics
- 🔔 Automated alerts when performance degrades >10% (unit) or >15% (E2E)
- 💬 PR comments when significant regressions are detected
- 📥 Raw benchmark results available as GitHub Actions artifacts (30-day retention)

### Performance Gates

**CI Fails If**:
- Any benchmark returns an error
- Race detector finds data races (when enabled)
- Build fails with benchmark code

**CI Warns (but doesn't fail) If**:
- Unit benchmarks show >10% performance degradation
- E2E benchmarks show >15% performance degradation
- Allocation counts increase significantly

**Not Enforced** (by design):
- Absolute performance numbers (hardware varies between CI runs)
- Minor allocation variations (<5%)

### Downloading Benchmark Results

Raw benchmark output is available as CI artifacts:

1. Go to **Actions** tab in GitHub
2. Click on the workflow run
3. Scroll to **Artifacts** section
4. Download:
   - `benchmark-unit-results` - Unit benchmark output
   - `benchmark-e2e-results` - E2E benchmark output

Artifacts are retained for 30 days.

### Local Pre-Push Checks

Run these before pushing to ensure CI will pass:

```bash
# Run full test suite with race detector
go test -race ./...

# Run unit benchmarks (as CI does)
go test -bench=. -benchmem -benchtime=3s \
  ./pkg/validation \
  ./reddit/internal/auth \
  ./reddit/internal/parse \
  -run=^$

# Run E2E benchmarks (fast variants)
go test -bench="BenchmarkScenario_(MonitorSubreddit/(fast_poll|medium_poll)|AnalyzeThread/shallow)" \
  -benchmem -benchtime=1s \
  ./reddit \
  -run=^$

# Run with timeout to catch hangs
go test -bench=. -benchmem -timeout=10m ./reddit
```

## Best Practices

### Writing New Benchmarks

Follow these patterns when adding benchmarks:

#### 1. Load Fixtures Before Timing

```go
func BenchmarkMyFeature(b *testing.B) {
    // Load fixture BEFORE b.ResetTimer()
    fixture := loadScenarioFixture(b, "medium_posts.json")
    server := setupMockServer(fixture)
    defer server.Close()

    client := createScenarioClient(b, server.URL, clock.NewMockClock(...))

    // Start timing AFTER all setup
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Benchmark code here
    }
}
```

#### 2. Use Table-Driven Tests

```go
func BenchmarkMyFeature(b *testing.B) {
    tests := []struct {
        name     string
        fixture  string
        limit    int
    }{
        {"small_10items", "small_posts.json", 10},
        {"medium_100items", "medium_posts.json", 100},
        {"large_1000items", "large_posts.json", 1000},
    }

    for _, tt := range tests {
        b.Run(tt.name, func(b *testing.B) {
            // Benchmark code
        })
    }
}
```

#### 3. Prevent Compiler Optimizations

```go
func BenchmarkParsing(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        result, err := Parse(data)
        if err != nil {
            b.Fatal(err)
        }

        // Access result to prevent dead code elimination
        _ = result.Posts
        _ = result.AfterFullname
    }
}
```

#### 4. Use MockClock for Time-Dependent Code

```go
func BenchmarkRateLimiting(b *testing.B) {
    mockClock := clock.NewMockClock(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

    // No real time delays - MockClock advances instantly
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Rate limiting happens but time advances instantly
        client.GetHot(ctx, req)

        // Advance mock clock instead of time.Sleep()
        mockClock.Advance(1 * time.Second)
    }
}
```

#### 5. Report Throughput When Appropriate

```go
func BenchmarkDownload(b *testing.B) {
    fixture := loadFixture(b, "medium_posts.json")

    // Report bytes processed for MB/s calculation
    b.SetBytes(int64(len(fixture)))

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Download and process fixture
    }
}
```

#### 6. Handle Errors Properly

```go
func BenchmarkConcurrent(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var wg sync.WaitGroup
        var mu sync.Mutex
        var firstErr error

        for j := 0; j < 10; j++ {
            wg.Add(1)
            go func() {
                defer wg.Done()

                if err := DoWork(); err != nil {
                    mu.Lock()
                    if firstErr == nil {
                        firstErr = err
                    }
                    mu.Unlock()
                }
            }()
        }

        wg.Wait()

        if firstErr != nil {
            b.Fatalf("operation failed: %v", firstErr)
        }
    }
}
```

### Comparative Benchmark Guidelines

When writing comparative benchmarks:

1. **Use Identical Fixtures**: Ensure both implementations process the same data
2. **Isolate Variables**: Change only one thing between comparisons
3. **Document Tradeoffs**: Explain what each approach sacrifices
4. **Realistic Workloads**: Use patterns that real applications would use
5. **Consider All Metrics**: Time, allocations, and throughput all matter

## Known Limitations

### Mock Servers vs Real API

**Limitation**: Benchmarks use mock HTTP servers, not real Reddit API

**Implications**:
- Network latency: 0 (real: 50-500ms depending on location)
- Server processing: Instant (real: 100-1000ms depending on query complexity)
- Rate limiting: Simulated (real: enforced by Reddit with 429 errors)
- Auth token refresh: Mocked (real: requires OAuth2 flow)

**Why Mock**: Benchmarks must be:
- Reproducible (same results every run)
- Fast (run in CI in <5 minutes)
- Isolated (no external dependencies)

**Real-World Testing**: For production validation, use integration tests with actual API.

### Fixture Size Constraints

**Limitation**: Fixtures are static snapshots with fixed sizes

**Constraints**:
- Max fixture size: ~5 MB (for reasonable git repo size)
- Comment depth: 50 levels (deep_comments.json)
- Post count: 1000 max (large_posts.json)

**Why Constrain**:
- Git repository size
- Benchmark runtime (must complete in reasonable time)
- Memory usage on CI runners

**Testing Larger Datasets**: For massive scale testing, generate fixtures programmatically in benchmarks.

### Time Simulation

**Limitation**: MockClock advances instantly, not real-time

**Implications**:
- Rate limiting: No actual delays (clock jumps forward)
- Timeouts: Instantaneous (no waiting for time.After channels)
- Polling intervals: Immediate (no real sleep)

**Why Mock Time**: Enables testing time-dependent behavior without delays:
- Rate limiting tests run in microseconds instead of seconds
- Timeout tests don't need to wait for actual timeouts
- Polling scenarios complete instantly

**Real-Time Testing**: For actual timing behavior, use integration tests with real clock.

### Concurrency Patterns

**Limitation**: Benchmarks use controlled concurrency levels

**Constraints**:
- Max concurrent requests: 20 (in most benchmarks)
- Semaphore limits: 10 (worker pool pattern)
- Goroutine count: Bounded to prevent resource exhaustion

**Why Limit**: Ensures benchmarks:
- Complete in reasonable time
- Don't exhaust file descriptors
- Produce consistent results

**Higher Concurrency**: For stress testing, increase limits in custom benchmarks.

## Additional Resources

### Related Documentation

- [CLAUDE.md](/home/jamesprial/code/go-reddit-api-wrapper/CLAUDE.md) - Development workflow and testing strategy
- [testdata/README.md](/home/jamesprial/code/go-reddit-api-wrapper/benchmarks/testdata/README.md) - Detailed fixture documentation
- [Go Testing Package](https://pkg.go.dev/testing) - Official benchmark documentation
- [Go Blog: Profiling Go Programs](https://go.dev/blog/pprof) - CPU and memory profiling guide

### Performance Analysis Tools

```bash
# CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./reddit
go tool pprof -http=:8080 cpu.prof

# Memory profiling
go test -bench=. -memprofile=mem.prof ./reddit
go tool pprof -http=:8080 mem.prof

# Allocation profiling
go test -bench=. -memprofile=mem.prof ./reddit
go tool pprof -http=:8080 -alloc_space mem.prof

# Benchmark comparison
go test -bench=. ./reddit > old.txt
# Make changes
go test -bench=. ./reddit > new.txt
benchcmp old.txt new.txt  # Requires golang.org/x/tools/cmd/benchcmp
```

### Example Analysis Workflow

```bash
# 1. Establish baseline
go test -bench=BenchmarkScenario_MonitorSubreddit -benchmem ./reddit > baseline.txt

# 2. Make optimization
# (edit code)

# 3. Measure impact
go test -bench=BenchmarkScenario_MonitorSubreddit -benchmem ./reddit > optimized.txt

# 4. Compare results
benchcmp baseline.txt optimized.txt

# 5. Profile to find bottlenecks
go test -bench=BenchmarkScenario_MonitorSubreddit -cpuprofile=cpu.prof ./reddit
go tool pprof -http=:8080 cpu.prof

# 6. Verify no races introduced
go test -bench=BenchmarkScenario_MonitorSubreddit -race ./reddit
```

---

**Last Updated**: 2025-01-23

**Maintained By**: Go Reddit API Wrapper Contributors

**Questions?** See [CLAUDE.md](/home/jamesprial/code/go-reddit-api-wrapper/CLAUDE.md) for development guidance or open an issue on GitHub.
