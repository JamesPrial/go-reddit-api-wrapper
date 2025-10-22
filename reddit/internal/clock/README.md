# Clock Package

**Internal package** providing time abstraction for testable time-dependent code.

## Purpose

The `clock` package enables writing time-dependent code that can be tested deterministically without actual delays. This is critical for testing features like:
- Token expiration and refresh
- Rate limiting and throttling
- Retry backoff logic
- Time-based caching

## Usage

### Production Code

Use `NewRealClock()` for production code that needs real system time:

```go
import "github.com/yourusername/go-reddit-api-wrapper/reddit/internal/clock"

type Service struct {
    clock clock.Clock
}

func NewService() *Service {
    return &Service{
        clock: clock.NewRealClock(),
    }
}

func (s *Service) IsExpired(expiresAt time.Time) bool {
    return s.clock.Now().After(expiresAt)
}
```

### Testing

Use `NewMockClock()` in tests to control time progression instantly:

```go
func TestTokenExpiry(t *testing.T) {
    mockClock := clock.NewMockClock(time.Now())
    service := &Service{clock: mockClock}

    expiresAt := mockClock.Now().Add(1 * time.Hour)

    // Time hasn't passed yet
    assert.False(t, service.IsExpired(expiresAt))

    // Advance time instantly (no actual delay)
    mockClock.Advance(2 * time.Hour)

    // Token is now expired
    assert.True(t, service.IsExpired(expiresAt))
}
```

## Interface

The `Clock` interface provides five methods matching the `time` package:

- `Now()` - Current time
- `Since(t)` - Duration since time `t`
- `Until(t)` - Duration until time `t`
- `Sleep(d)` - Pause for duration `d` (instant in mock)
- `After(d)` - Channel that receives after duration `d` (instant in mock)

## MockClock Helpers

- `Advance(d)` - Move time forward by duration `d`
- `Set(t)` - Set absolute time to `t`

## Thread Safety

`MockClock` is thread-safe and can be used in concurrent tests. All operations are protected by `sync.RWMutex`.
