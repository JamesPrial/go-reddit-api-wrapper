package client

import (
	"context"
	"sync"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
	"golang.org/x/time/rate"
)

// ClockAwareRateLimiter is an interface for rate limiting that respects the injected Clock.
// This enables time-dependent tests to run without delays by using MockClock.
type ClockAwareRateLimiter interface {
	// Wait blocks until a token is available or ctx is cancelled.
	Wait(ctx context.Context) error

	// Allow reports whether an event may happen now without blocking.
	Allow() bool

	// Reserve reserves an event for a future time.
	Reserve() *rate.Reservation

	// Limit returns the current rate limit.
	Limit() rate.Limit

	// Burst returns the current burst capacity.
	Burst() int
}

// clockAwareRateLimiter wraps a rate.Limiter and uses the injected Clock interface.
// It implements a custom token bucket algorithm that respects the injected Clock,
// allowing it to work correctly with MockClock in tests.
type clockAwareRateLimiter struct {
	mu              sync.Mutex
	limiter         *rate.Limiter
	clock           clock.Clock
	lastAllowTime   time.Time
	tokensAvailable float64
}

// Wait blocks until a token is available or ctx is cancelled.
// It uses the injected Clock for timing, making it compatible with MockClock.
func (c *clockAwareRateLimiter) Wait(ctx context.Context) error {
	for {
		// Check context to allow fast cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to allow the request
		if c.Allow() {
			return nil
		}

		// Calculate how long to wait for a token to become available
		c.mu.Lock()
		if c.tokensAvailable >= 1.0 {
			c.mu.Unlock()
			continue // Try again immediately (shouldn't happen but be safe)
		}

		// How many tokens do we need to generate to have 1 token?
		tokensNeeded := 1.0 - c.tokensAvailable

		// How long until we accumulate that many tokens?
		limit := c.limiter.Limit()
		tokensPerSecond := float64(limit)
		if tokensPerSecond <= 0 {
			// Shouldn't happen, but protect against division
			c.mu.Unlock()
			c.clock.Sleep(10 * time.Millisecond)
			continue
		}

		secondsNeeded := tokensNeeded / tokensPerSecond
		waitDuration := time.Duration(secondsNeeded * float64(time.Second))

		// Cap wait to prevent extremely long sleeps in tests
		if waitDuration > 100*time.Millisecond {
			waitDuration = 100 * time.Millisecond
		}

		// Minimum wait to make progress
		if waitDuration < 1*time.Millisecond {
			waitDuration = 1 * time.Millisecond
		}

		c.mu.Unlock()

		// Sleep for the calculated duration
		c.clock.Sleep(waitDuration)
	}
}

// Allow reports whether an event may happen now without blocking.
// It uses the injected Clock to calculate elapsed time, making it work with MockClock.
func (c *clockAwareRateLimiter) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()

	// Initialize on first call
	if c.lastAllowTime.IsZero() {
		c.lastAllowTime = now
		c.tokensAvailable = float64(c.limiter.Burst())
	}

	// Calculate time elapsed and refill tokens
	elapsed := now.Sub(c.lastAllowTime)
	c.lastAllowTime = now

	// Calculate tokens per duration based on the limiter's rate
	limit := c.limiter.Limit()
	tokensPerSecond := float64(limit)
	tokensToAdd := tokensPerSecond * elapsed.Seconds()
	c.tokensAvailable += tokensToAdd

	// Cap at burst capacity
	if c.tokensAvailable > float64(c.limiter.Burst()) {
		c.tokensAvailable = float64(c.limiter.Burst())
	}

	// Check if we have tokens available
	if c.tokensAvailable >= 1.0 {
		c.tokensAvailable -= 1.0
		return true
	}

	return false
}

// Reserve reserves an event for a future time.
//
// WARNING: Reserve uses the underlying rate.Limiter which does not respect
// the injected Clock interface. The reservation's Delay() method will use
// real system time, not the injected clock time. For clock-aware rate limiting,
// use Wait() or Allow() instead.
//
// This method is provided for API compatibility with rate.Limiter but should
// be avoided when using MockClock in tests as it may produce unexpected results.
func (c *clockAwareRateLimiter) Reserve() *rate.Reservation {
	return c.limiter.Reserve()
}

// Limit returns the current rate limit.
func (c *clockAwareRateLimiter) Limit() rate.Limit {
	return c.limiter.Limit()
}

// Burst returns the current burst capacity.
func (c *clockAwareRateLimiter) Burst() int {
	return c.limiter.Burst()
}

// NewClockAwareRateLimiter creates a rate limiter that respects the injected Clock.
// If requestsPerMinute is 0, applies the default rate limit.
// If burst is 0 or negative, applies the default burst capacity.
// If clk is nil, a real clock is used.
func NewClockAwareRateLimiter(clk clock.Clock, requestsPerMinute float64, burst int) ClockAwareRateLimiter {
	if clk == nil {
		clk = clock.NewRealClock()
	}

	// Apply default rate limit if not specified
	rpm := requestsPerMinute
	if rpm <= 0 {
		rpm = DefaultRequestsPerMinute
	}

	// Apply default burst if not specified
	b := burst
	if b <= 0 {
		b = DefaultRateLimitBurst
	}

	limitPerSecond := rate.Limit(rpm / SecondsPerMinute)
	if limitPerSecond <= 0 {
		limitPerSecond = rate.Limit(MinRateLimitPerSecond)
	}

	return &clockAwareRateLimiter{
		limiter:         rate.NewLimiter(limitPerSecond, b),
		clock:           clk,
		tokensAvailable: float64(b),
	}
}
