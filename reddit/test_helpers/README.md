# Test Helpers

This directory provides enhanced testing utilities for the go-reddit-api-wrapper project.

## Overview

The test helpers package provides comprehensive utilities for testing Reddit API wrapper functionality, including:

- **Mock Server**: Configurable mock Reddit API server
- **Test Client**: Wrapper around Reddit client for testing
- **Concurrent Testing**: Utilities for testing concurrent scenarios
- **Performance Testing**: Load testing and performance measurement tools
- **Authentication Testing**: Helpers for auth scenario testing

## Components

### Mock Server (`mock_server.go`)

Provides a configurable mock Reddit API server that can simulate various API responses and behaviors.

#### Key Features:
- Configurable responses for different endpoints
- Request logging and call counting
- Rate limiting simulation
- Error injection
- Response delays
- Reddit-specific endpoints pre-configured

#### Usage:
```go
// Create a new mock server
server := NewRedditMockServer()
defer server.Close()

// Configure responses
server.SetupSubreddit("test", posts, comments)
server.SetupRateLimit(100, 1, time.Now().Add(time.Hour))

// Use the server URL for testing
client := NewTestClient(&MockClientConfig{
    BaseURL: server.URL(),
})
```

### Test Client (`test_client.go`)

Provides a wrapper around the Reddit client specifically designed for testing.

#### Key Features:
- Integration with mock server
- Request assertion utilities
- Performance measurement
- Concurrent testing support
- Authentication scenario testing

#### Usage:
```go
// Create a test client
client := NewTestClient(nil)
defer client.Close()

// Configure mock server behavior
client.SetupRateLimit(100, 1, time.Now().Add(time.Hour))

// Make requests and assert behavior
err := client.SomeAPICall()
assert.NoError(t, err)

// Assert request patterns
err = client.AssertRequestCount("/api/v1/access_token", 1)
assert.NoError(t, err)
```

### Concurrent Testing

Utilities for testing concurrent client usage scenarios.

#### Features:
- Multiple client management
- Concurrent test execution
- Error collection and reporting
- Resource cleanup

#### Usage:
```go
helper := NewConcurrentTestHelper(5) // 5 concurrent clients
defer helper.Close()

errors := helper.RunConcurrentTest(func(client *TestClient) error {
    return client.SomeAPICall()
})

// Check for errors
for i, err := range errors {
    assert.NoError(t, err, "Client %d failed", i)
}
```

### Performance Testing

Comprehensive performance testing and measurement tools.

#### Features:
- Latency measurement
- Request throughput tracking
- Error rate monitoring
- Load testing with configurable concurrency
- Memory usage tracking

#### Usage:
```go
helper := NewPerformanceTestHelper()
defer helper.Close()

// Run a load test
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

metrics, err := helper.RunLoadTest(ctx, 10, 10*time.Second, func(client *TestClient) error {
    return client.SomeAPICall()
})

assert.NoError(t, err)
assert.Greater(t, metrics.RequestCount, int64(0))
assert.Less(t, metrics.AverageLatency, time.Second)
```

### Authentication Testing

Specialized helpers for testing authentication scenarios.

#### Features:
- Valid authentication setup
- Expired token simulation
- Rate limiting scenarios
- Token refresh testing

#### Usage:
```go
helper := NewAuthTestHelper()
defer helper.Close()

// Setup valid authentication
helper.SetupValidAuth()

// Test with valid auth
err := helper.Client().SomeAuthenticatedAPICall()
assert.NoError(t, err)

// Setup expired token scenario
helper.SetupExpiredToken()
err = helper.Client().SomeAuthenticatedAPICall()
assert.Error(t, err)
```

## Configuration

### Mock Client Configuration

```go
type MockClientConfig struct {
    BaseURL        string        // Mock server URL
    UserAgent      string        // User agent string
    Timeout        time.Duration // Request timeout
    RetryAttempts  int           // Number of retry attempts
    RetryDelay     time.Duration // Delay between retries
    RateLimitDelay time.Duration // Rate limit delay
    AuthHeader     string        // Authorization header
}
```

### Default Configuration

```go
config := DefaultMockClientConfig()
// Returns:
// {
//     UserAgent: "test-client/1.0",
//     Timeout: 30 * time.Second,
//     RetryAttempts: 3,
//     RetryDelay: 100 * time.Millisecond,
//     RateLimitDelay: 100 * time.Millisecond,
//     AuthHeader: "Bearer mock_token",
// }
```

## Best Practices

### 1. Resource Management
Always close test clients and mock servers to prevent resource leaks:
```go
client := NewTestClient(nil)
defer client.Close()
```

### 2. Test Isolation
Reset mock server state between tests:
```go
func TestSomething(t *testing.T) {
    client := NewTestClient(nil)
    defer client.Close()
    
    // Reset before each test
    client.Reset()
    
    // Test logic here
}
```

### 3. Concurrent Testing
Use proper synchronization and cleanup for concurrent tests:
```go
helper := NewConcurrentTestHelper(10)
defer helper.Close() // Ensures all clients are closed
```

### 4. Performance Testing
Use context for timeout control in performance tests:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

metrics, err := helper.RunLoadTest(ctx, 5, 10*time.Second, testFunc)
```

### 5. Error Handling
Use provided assertion utilities for consistent error checking:
```go
err := client.SomeAPICall()
assert.NoError(t, AssertNoError(err))
```

## Integration with Existing Tests

These helpers are designed to integrate seamlessly with the existing test suite:

1. **Backward Compatibility**: Existing tests continue to work without changes
2. **Gradual Adoption**: Can be adopted incrementally in new tests
3. **Consistent Interface**: Follows Go testing conventions
4. **Comprehensive Coverage**: Covers all major testing scenarios

## Examples

See the test files in the parent directory for comprehensive examples of using these helpers:

- `workflows_test.go`: Complete workflow testing
- `concurrency_test.go`: Concurrent usage patterns
- `performance_logic_test.go`: Performance testing
- `auth_lifecycle_test.go`: Authentication scenarios
- `network_recovery_test.go`: Network failure testing

## Troubleshooting

### Common Issues

1. **Import Errors**: Ensure the module path is correct in `go.mod`
2. **Resource Leaks**: Always close clients and servers
3. **Race Conditions**: Use proper synchronization in concurrent tests
4. **Timeout Issues**: Configure appropriate timeouts for your environment

### Debug Mode

Enable detailed logging by setting the log level:
```go
// Add debug logging to mock server
server.SetDelay(0) // Remove delays for debugging
server.SetErrorRate(0) // Disable random errors
```

## Contributing

When adding new test helpers:

1. Follow the existing patterns and conventions
2. Add comprehensive documentation
3. Include usage examples
4. Ensure proper resource cleanup
5. Add tests for the helpers themselves
6. Update this README

## License

These test helpers are part of the go-reddit-api-wrapper project and follow the same license terms.
