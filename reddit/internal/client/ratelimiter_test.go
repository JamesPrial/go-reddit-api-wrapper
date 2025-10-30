package client

import (
	"context"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
	"golang.org/x/time/rate"
)

// TestClockAwareRateLimiter_WithMockClock verifies that the rate limiter respects MockClock.
func TestClockAwareRateLimiter_WithMockClock(t *testing.T) {
	// Create MockClock starting at a fixed time
	mockClock := clock.NewMockClock(time.Time{})

	// Create rate limiter with 60 req/min (1 per second), burst=10
	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	// Verify that Allow() respects burst capacity
	for i := 0; i < DefaultRateLimitBurst; i++ {
		if !limiter.Allow() {
			t.Fatalf("Allow %d should succeed within burst", i)
		}
	}

	// After burst exhausted, Allow should return false
	if limiter.Allow() {
		t.Fatal("Allow should return false after burst exhausted")
	}

	// But if we advance time, more tokens should be available
	mockClock.Advance(time.Second)

	// Now Allow should succeed
	if !limiter.Allow() {
		t.Fatal("Allow should return true after advancing clock 1 second")
	}
}

// TestClockAwareRateLimiter_ContextCancellation verifies that Wait() respects context cancellation.
func TestClockAwareRateLimiter_ContextCancellation(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	// Create rate limiter with very low rate to ensure blocking when burst exhausted
	limiter := NewClockAwareRateLimiter(mockClock, 1, 0) // 1 request per minute

	ctx := context.Background()

	// Exhaust burst capacity
	for i := 0; i < DefaultRateLimitBurst; i++ {
		_ = limiter.Wait(ctx)
	}

	// Create a cancellable context
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel it immediately

	// Try to wait with cancelled context - should return immediately with error
	err := limiter.Wait(cancelCtx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestClockAwareRateLimiter_Allow verifies that Allow() reports rate limit status without blocking.
func TestClockAwareRateLimiter_Allow(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	// Create rate limiter with 60 req/min (1 per second)
	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	// Should be able to use burst capacity immediately
	for i := 0; i < DefaultRateLimitBurst; i++ {
		if !limiter.Allow() {
			t.Fatalf("Allow %d should return true within burst capacity", i)
		}
	}

	// After burst is exhausted, Allow should return false
	if limiter.Allow() {
		t.Fatal("Allow should return false after burst exhausted")
	}

	// Allow should return false immediately without blocking
	start := time.Now()
	if limiter.Allow() {
		t.Fatal("Allow should still return false")
	}
	elapsed := time.Since(start)

	// This should be nearly instant, not waiting at all
	if elapsed > 50*time.Millisecond {
		t.Errorf("Allow took too long: %v", elapsed)
	}
}

// TestClockAwareRateLimiter_Limit verifies that Limit() returns the correct rate.
func TestClockAwareRateLimiter_Limit(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	limiter := NewClockAwareRateLimiter(mockClock, 120, 0)

	expectedLimit := rate.Limit(120.0 / 60.0) // 2 requests per second
	if got := limiter.Limit(); got != expectedLimit {
		t.Errorf("expected limit %v, got %v", expectedLimit, got)
	}
}

// TestClockAwareRateLimiter_Burst verifies that Burst() returns the correct capacity.
func TestClockAwareRateLimiter_Burst(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	expectedBurst := DefaultRateLimitBurst
	if got := limiter.Burst(); got != expectedBurst {
		t.Errorf("expected burst %d, got %d", expectedBurst, got)
	}
}

// TestClockAwareRateLimiter_BurstCapacity verifies burst behavior.
func TestClockAwareRateLimiter_BurstCapacity(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	// Create limiter with default burst capacity (10)
	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	// Should be able to use all burst capacity immediately
	for i := 0; i < DefaultRateLimitBurst; i++ {
		if !limiter.Allow() {
			t.Fatalf("Allow %d should succeed within burst capacity", i)
		}
	}

	// After burst, Allow should return false until time advances
	if limiter.Allow() {
		t.Fatal("Allow should return false after burst exhausted")
	}

	// Advance time slightly - not enough for a full token
	mockClock.Advance(500 * time.Millisecond)
	if limiter.Allow() {
		t.Fatal("Allow should still return false after 500ms (need 1 second for 1 token)")
	}

	// Advance time enough for another token (1 second total, 0.5 added)
	mockClock.Advance(600 * time.Millisecond)
	if !limiter.Allow() {
		t.Fatal("Allow should return true after accumulating 1+ seconds")
	}
}

// TestZeroRateLimiter verifies behavior when rate is 0 (uses default).
func TestZeroRateLimiter(t *testing.T) {
	// Create limiter with 0 rate (should use DefaultRequestsPerMinute)
	limiter := NewClockAwareRateLimiter(nil, 0, 0)

	// Should use DefaultRequestsPerMinute (100)
	expectedLimit := rate.Limit(float64(DefaultRequestsPerMinute) / SecondsPerMinute)
	if got := limiter.Limit(); got != expectedLimit {
		t.Errorf("expected default limit %v, got %v", expectedLimit, got)
	}

	// Wait should return immediately (within burst)
	start := time.Now()
	err := limiter.Wait(context.Background())
	elapsed := time.Since(start)

	testutil.AssertNoError(t, err)
	if elapsed > 50*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}

	// Multiple waits should succeed for burst capacity
	for i := 0; i < DefaultRateLimitBurst; i++ {
		err := limiter.Wait(context.Background())
		testutil.AssertNoError(t, err)
	}
}

// TestZeroRateLimiter_ContextCancellation verifies context cancellation works.
func TestZeroRateLimiter_ContextCancellation(t *testing.T) {
	limiter := NewClockAwareRateLimiter(nil, 0, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Cancelled context should be caught immediately
	err := limiter.Wait(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestClockAwareRateLimiter_NilClock verifies default clock behavior.
func TestClockAwareRateLimiter_NilClock(t *testing.T) {
	// When nil clock is provided, should use real clock
	limiter := NewClockAwareRateLimiter(nil, 60, 0)

	// Should not panic
	ctx := context.Background()
	err := limiter.Wait(ctx)
	testutil.AssertNoError(t, err)

	// Should allow immediate use within burst
	if !limiter.Allow() {
		t.Fatal("Allow should return true within burst capacity")
	}
}

// TestClockAwareRateLimiter_NegativeRate verifies behavior with negative rate.
func TestClockAwareRateLimiter_NegativeRate(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	// Negative rate should apply default
	limiter := NewClockAwareRateLimiter(mockClock, -1, 0)

	// Should have default limit (100 req/min)
	expectedLimit := rate.Limit(float64(DefaultRequestsPerMinute) / SecondsPerMinute)
	if got := limiter.Limit(); got != expectedLimit {
		t.Errorf("negative rate should apply default, expected %v, got %v", expectedLimit, got)
	}
}

// TestClockAwareRateLimiter_RapidRequests verifies behavior under rapid requests.
func TestClockAwareRateLimiter_RapidRequests(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	// Create rate limiter with 60 req/min (1 per second)
	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	// Use up the burst capacity (DefaultRateLimitBurst)
	ctx := context.Background()
	for i := 0; i < DefaultRateLimitBurst; i++ {
		if err := limiter.Wait(ctx); err != nil {
			t.Fatalf("Wait %d should succeed: %v", i, err)
		}
	}

	// Track how many requests can be processed as we advance time
	requestCount := 0
	for t := time.Duration(0); t < 5*time.Second; t += 500 * time.Millisecond {
		mockClock.Advance(500 * time.Millisecond)

		// Try to make a request
		if limiter.Allow() {
			requestCount++
		}
	}

	// With 500ms intervals, we should get roughly 2-3 requests in 5 seconds
	// (rate is 1 request/second, so about 5 total, but burst and timing affect this)
	if requestCount < 2 {
		t.Errorf("expected at least 2 requests to be allowed, got %d", requestCount)
	}
}

// TestClockAwareRateLimiter_MultipleWaiters verifies behavior with multiple concurrent goroutines.
func TestClockAwareRateLimiter_MultipleWaiters(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	// Create rate limiter with 60 req/min (1 per second, burst of 10)
	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	ctx := context.Background()
	done := make(chan error, 20)

	// Start 20 concurrent wait requests
	for i := 0; i < 20; i++ {
		go func() {
			done <- limiter.Wait(ctx)
		}()
	}

	// Some should succeed within burst (10 requests)
	successCount := 0
	for i := 0; i < 20; i++ {
		select {
		case err := <-done:
			if err == nil {
				successCount++
			}
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	// Should have used up the burst capacity (10 concurrent waits)
	if successCount < 5 {
		t.Errorf("expected at least 5 concurrent requests to succeed, got %d", successCount)
	}

	// Advance time and remaining requests should complete
	mockClock.Advance(5 * time.Second)

	// Wait for remaining goroutines
	remaining := 20 - successCount
	for i := 0; i < remaining; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("expected request to complete after time advance")
		}
	}
}

// TestClockAwareRateLimiter_Reserve verifies Reserve() behavior.
func TestClockAwareRateLimiter_Reserve(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})

	limiter := NewClockAwareRateLimiter(mockClock, 60, 0)

	// Reservations can be made, underlying rate limiter handles them
	reservation := limiter.Reserve()
	if reservation == nil {
		t.Fatal("expected non-nil reservation")
	}

	// Reserve should return a valid reservation
	if !reservation.OK() {
		t.Fatal("reservation should be OK")
	}

	// Multiple reservations should work
	reservation2 := limiter.Reserve()
	if reservation2 == nil {
		t.Fatal("expected non-nil reservation")
	}

	// Both should have non-negative delays
	delay1 := reservation.DelayFrom(mockClock.Now())
	delay2 := reservation2.DelayFrom(mockClock.Now())

	if delay1 < 0 || delay2 < 0 {
		t.Errorf("delays should be non-negative, got %v and %v", delay1, delay2)
	}
}
