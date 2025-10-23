# Comparative Benchmarks

This directory contains comparative benchmarks that measure the performance of our Reddit client against baseline implementations.

## Purpose

These benchmarks provide objective performance comparisons to help understand:
- The overhead/benefit of our abstractions versus raw stdlib usage
- The impact of specific optimizations (buffer pooling, rate limiting, etc.)
- The performance characteristics of our error handling approach
- How our library compares to alternative implementations

## Benchmark Categories

### 1. Our Client vs Raw HTTP + Manual JSON Parsing

Compares our full-featured Reddit client against a minimal implementation using only stdlib.

**Benchmarks:**
- `BenchmarkComparison_GetPosts_OurClient` - Full client with all features
- `BenchmarkComparison_GetPosts_RawHTTP` - Minimal stdlib-only implementation
- `BenchmarkComparison_ParseResponse_OurClient` - Type-safe parsing with custom unmarshalers
- `BenchmarkComparison_ParseResponse_RawJSON` - Generic JSON parsing
- `BenchmarkComparison_EndToEnd_OurClient` - Complete workflow with all features
- `BenchmarkComparison_EndToEnd_RawHTTP` - Complete workflow with minimal abstractions

**Expected Results:**
- Our client: ~20-30% slower due to additional features
- Our client: 40-60% fewer allocations due to buffer pooling
- Our client: Better type safety and validation

### 2. With vs Without Buffer Pooling

Compares the memory efficiency of using `sync.Pool` for response body buffering.

**Benchmarks:**
- `BenchmarkComparison_ResponseParsing_WithPool` - Uses sync.Pool for buffers
- `BenchmarkComparison_ResponseParsing_NoPool` - Allocates new buffer each request

**Expected Results:**
- With pool: 40-60% fewer allocations
- With pool: Lower GC pressure under load
- With pool: More consistent memory footprint

### 3. With vs Without Rate Limiting

Measures the throughput impact of rate limiting enforcement.

**Benchmarks:**
- `BenchmarkComparison_BurstRequests_WithRateLimit` - Enforces rate limits
- `BenchmarkComparison_BurstRequests_NoRateLimit` - No rate limiting

**Expected Results:**
- With rate limit: Slower throughput but prevents 429 errors
- With rate limit: Smooth request distribution over time
- Without rate limit: Maximum throughput but unsustainable in production

### 4. Typed Errors vs Generic Errors

Compares structured error types versus simple error strings.

**Benchmarks:**
- `BenchmarkComparison_ErrorHandling_TypedErrors` - Structured error types
- `BenchmarkComparison_ErrorHandling_GenericErrors` - fmt.Errorf strings

**Expected Results:**
- Typed errors: Slightly higher allocation cost
- Typed errors: Better debugging and error inspection
- Typed errors: Supports error chains with Unwrap()

### 5. Placeholder for Future Library Comparisons

Structure for comparing against other Go Reddit libraries when available.

**Future Comparisons:**
- github.com/vartanbeno/go-reddit
- github.com/turnage/graw
- Other implementations

## Running Benchmarks

### Run All Comparative Benchmarks

```bash
go test -bench=. -benchmem ./benchmarks/comparative/
```

### Run Specific Comparison Category

```bash
# Our client vs raw HTTP
go test -bench=BenchmarkComparison_GetPosts -benchmem ./benchmarks/comparative/

# Buffer pooling comparison
go test -bench=BenchmarkComparison_ResponseParsing -benchmem ./benchmarks/comparative/

# Rate limiting comparison
go test -bench=BenchmarkComparison_BurstRequests -benchmem ./benchmarks/comparative/

# Error handling comparison
go test -bench=BenchmarkComparison_ErrorHandling -benchmem ./benchmarks/comparative/
```

### Run with Different Iteration Counts

```bash
# Quick sanity check (5 iterations)
go test -bench=. -benchmem -benchtime=5x ./benchmarks/comparative/

# Standard run (default iterations based on time)
go test -bench=. -benchmem ./benchmarks/comparative/

# High-precision results (1000 iterations)
go test -bench=. -benchmem -benchtime=1000x ./benchmarks/comparative/
```

### Save Results for Analysis

```bash
# Save baseline results
go test -bench=. -benchmem ./benchmarks/comparative/ > baseline.txt

# Compare against previous results
go test -bench=. -benchmem ./benchmarks/comparative/ > current.txt
benchstat baseline.txt current.txt
```

## Interpreting Results

### Key Metrics

- **ns/op**: Nanoseconds per operation (lower is better)
- **MB/s**: Throughput in megabytes per second (higher is better)
- **B/op**: Bytes allocated per operation (lower is better)
- **allocs/op**: Number of allocations per operation (lower is better)

### Example Output Analysis

```
BenchmarkComparison_GetPosts_OurClient/small_10posts-24    5    838362 ns/op   20.28 MB/s   161193 B/op    546 allocs/op
BenchmarkComparison_GetPosts_RawHTTP/small_10posts-24      5    205343 ns/op   82.79 MB/s   172003 B/op   1080 allocs/op
```

**Analysis:**
- Raw HTTP is ~4x faster (205ms vs 838ms)
- Our client uses slightly fewer allocations (546 vs 1080)
- Raw HTTP has higher throughput but provides no abstractions
- Trade-off: Our client provides auth caching, rate limiting, type safety, validation

### Comparison Guidelines

When comparing benchmarks:

1. **Time overhead is acceptable** when it provides significant value:
   - Auth token caching (saves auth roundtrip on subsequent requests)
   - Rate limiting (prevents 429 errors in production)
   - Type safety (catches bugs at compile time)
   - Structured errors (better debugging)

2. **Memory overhead should be minimized**:
   - Buffer pooling reduces allocations significantly
   - Reuse of parse contexts reduces GC pressure
   - Struct packing minimizes memory footprint

3. **Consider production scenarios**:
   - Raw HTTP benchmark doesn't include auth logic
   - No rate limiting means potential API bans
   - Generic errors make debugging harder
   - Type assertions add runtime overhead and panic risk

## Test Data

All benchmarks use fixtures from `../testdata/`:

- `small_posts.json` - 10 posts (~17KB) - Quick comparisons
- `medium_posts.json` - 100 posts (~138KB) - Realistic workloads
- `large_posts.json` - 1000 posts (~1.4MB) - Stress testing

## Adding New Comparisons

To add a comparison against another library:

1. Add imports for the library
2. Create paired benchmarks following the naming convention:
   - `BenchmarkComparison_Operation_OurClient`
   - `BenchmarkComparison_Operation_OtherLib`
3. Use identical fixtures and mock servers
4. Document expected results
5. Add to this README

Example structure:

```go
func BenchmarkComparison_GetPosts_OtherLib(b *testing.B) {
    b.ReportAllocs()
    b.SetBytes(expectedSize)

    fixture := loadFixture(b, "medium_posts.json")
    server := setupMockServer(fixture)
    defer server.Close()

    // Setup other library client
    client := otherlib.NewClient(...)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        posts, err := client.GetPosts("golang")
        if err != nil {
            b.Fatal(err)
        }
        _ = posts
    }
}
```

## Best Practices

1. Always use `b.ReportAllocs()` to track memory usage
2. Use `b.ResetTimer()` after setup to exclude initialization
3. Use identical fixtures for paired comparisons
4. Use discard logger to minimize logging overhead
5. Run multiple iterations for statistical significance
6. Document what each comparison measures and why

## Continuous Integration

These benchmarks are run in CI to track performance over time:

```bash
# Run as part of CI pipeline
go test -bench=. -benchmem -benchtime=100x ./benchmarks/comparative/
```

Performance regressions are flagged when:
- Time per operation increases >20%
- Allocations per operation increase >30%
- Throughput decreases >20%
