package internal

import (
	"sync"
	"time"
)

// Clock provides an abstraction over time operations to enable testing.
// This interface allows tests to control time progression without actual delays.
type Clock interface {
	// Now returns the current time.
	Now() time.Time

	// Since returns the time elapsed since t.
	Since(t time.Time) time.Duration

	// Until returns the duration until t.
	Until(t time.Time) time.Duration

	// Sleep pauses execution for the specified duration.
	Sleep(d time.Duration)

	// After returns a channel that receives the current time after the duration.
	After(d time.Duration) <-chan time.Time
}

// realClock implements Clock using the standard time package.
type realClock struct{}

// NewRealClock returns a Clock that uses real system time.
func NewRealClock() Clock {
	return &realClock{}
}

func (c *realClock) Now() time.Time {
	return time.Now()
}

func (c *realClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func (c *realClock) Until(t time.Time) time.Duration {
	return time.Until(t)
}

func (c *realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (c *realClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// MockClock implements Clock with controllable time for testing.
// It allows tests to advance time instantly without actual delays.
type MockClock struct {
	mu      sync.RWMutex
	current time.Time
}

// NewMockClock returns a Clock that starts at the specified time.
// If zero time is provided, it starts at a fixed reference time.
func NewMockClock(start time.Time) *MockClock {
	if start.IsZero() {
		start = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return &MockClock{
		current: start,
	}
}

// Now returns the current mock time.
func (c *MockClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// Since returns the duration since t based on mock time.
func (c *MockClock) Since(t time.Time) time.Duration {
	return c.Now().Sub(t)
}

// Until returns the duration until t based on mock time.
func (c *MockClock) Until(t time.Time) time.Duration {
	return t.Sub(c.Now())
}

// Sleep advances mock time by the specified duration without actual delay.
func (c *MockClock) Sleep(d time.Duration) {
	c.Advance(d)
}

// After returns a channel that immediately receives the time after advancing by d.
// Note: This is a simplified implementation for testing. The channel receives immediately.
func (c *MockClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.Advance(d)
	ch <- c.Now()
	return ch
}

// Advance moves mock time forward by the specified duration.
// This is a test helper method to explicitly advance time.
func (c *MockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.current.Add(d)
}

// Set sets the mock time to a specific value.
// This is a test helper method to set absolute time.
func (c *MockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = t
}
