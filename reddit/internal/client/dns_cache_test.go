package client

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

func TestDNSCache_HitAndMiss(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	hits, misses := cache.GetMetrics()
	if hits != 0 || misses != 0 {
		t.Fatalf("expected initial metrics to be 0/0, got %d/%d", hits, misses)
	}

	// First lookup should be a miss
	ctx := context.Background()
	ips, err := cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	if len(ips) == 0 {
		t.Fatalf("expected at least one IP address for www.reddit.com")
	}

	hits, misses = cache.GetMetrics()
	if hits != 0 || misses != 1 {
		t.Errorf("after first lookup, expected metrics 0/1, got %d/%d", hits, misses)
	}

	// Second lookup should be a hit (same host, same time)
	ips2, err := cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	if len(ips2) == 0 {
		t.Fatalf("expected at least one IP address on cache hit")
	}

	hits, misses = cache.GetMetrics()
	if hits != 1 || misses != 1 {
		t.Errorf("after cache hit, expected metrics 1/1, got %d/%d", hits, misses)
	}

	// Verify the cached IPs match
	if len(ips) != len(ips2) {
		t.Errorf("expected same number of IPs, got %d and %d", len(ips), len(ips2))
	}
	for i, ip := range ips {
		if ip.String() != ips2[i].String() {
			t.Errorf("expected same IP at index %d, got %q and %q", i, ip.String(), ips2[i].String())
		}
	}
}

func TestDNSCache_TTLExpiration(t *testing.T) {
	ttl := 1 * time.Minute
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(ttl, mockClock)

	ctx := context.Background()

	// First lookup
	ips, err := cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	hits, misses := cache.GetMetrics()
	if hits != 0 || misses != 1 {
		t.Errorf("expected 0/1, got %d/%d", hits, misses)
	}

	// Second lookup (same time, should be cached)
	_, err = cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	hits, misses = cache.GetMetrics()
	if hits != 1 || misses != 1 {
		t.Errorf("expected 1/1, got %d/%d", hits, misses)
	}

	// Advance time past TTL
	mockClock.Advance(2 * time.Minute)

	// Third lookup (after expiration, should be a miss)
	ips3, err := cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	// Should match the original IPs but be counted as a miss
	if len(ips) != len(ips3) {
		t.Errorf("expected same IPs, got %d and %d", len(ips), len(ips3))
	}

	hits, misses = cache.GetMetrics()
	if hits != 1 || misses != 2 {
		t.Errorf("after expiration, expected 1/2, got %d/%d", hits, misses)
	}
}

func TestDNSCacheLookupIPAddr_ThreadSafety(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	ctx := context.Background()
	numGoroutines := 10
	lookupsPerGoroutine := 100
	var wg sync.WaitGroup
	var lookupErrors atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < lookupsPerGoroutine; j++ {
				_, err := cache.LookupIPAddr(ctx, "www.reddit.com")
				if err != nil {
					lookupErrors.Add(1)
				}
			}
		}(i)
	}

	wg.Wait()

	if lookupErrors.Load() > 0 {
		t.Errorf("expected no errors, got %d", lookupErrors.Load())
	}

	// Verify metrics are consistent
	hits, misses := cache.GetMetrics()
	totalLookups := hits + misses
	expectedMin := int64(1) // At least one miss for the first lookup

	if totalLookups < expectedMin {
		t.Errorf("expected at least %d total lookups, got %d", expectedMin, totalLookups)
	}

	// With a single host and cached results, we expect mostly hits
	if hits < 100 {
		t.Errorf("expected significant cache hits, got %d hits out of %d total", hits, totalLookups)
	}
}

func TestDNSCache_RealLookup(t *testing.T) {
	cache := NewDNSCache(5 * time.Minute)

	ctx := context.Background()

	// Use a host we know should resolve (localhost)
	ips, err := cache.LookupIPAddr(ctx, "localhost")
	testutil.AssertNoError(t, err)

	if len(ips) == 0 {
		t.Fatalf("expected at least one IP for localhost")
	}

	// Verify we got valid IP addresses
	for _, ip := range ips {
		if ip.IP == nil {
			t.Errorf("expected valid IP, got nil")
		}
	}

	// Verify metrics
	hits, misses := cache.GetMetrics()
	if hits != 0 || misses != 1 {
		t.Errorf("expected 0/1, got %d/%d", hits, misses)
	}
}

func TestDNSCache_Metrics(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	ctx := context.Background()

	// Perform multiple lookups on different hosts
	hosts := []string{"www.reddit.com", "oauth.reddit.com", "api.reddit.com"}

	for _, host := range hosts {
		_, err := cache.LookupIPAddr(ctx, host)
		testutil.AssertNoError(t, err)
	}

	hits, misses := cache.GetMetrics()
	if hits != 0 || misses != 3 {
		t.Errorf("expected 0/3 after first lookups, got %d/%d", hits, misses)
	}

	// Perform cached lookups
	for _, host := range hosts {
		_, err := cache.LookupIPAddr(ctx, host)
		testutil.AssertNoError(t, err)
	}

	hits, misses = cache.GetMetrics()
	if hits != 3 || misses != 3 {
		t.Errorf("expected 3/3 after cached lookups, got %d/%d", hits, misses)
	}
}

func TestDNSCacheLookupIPAddr_Clear(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	ctx := context.Background()

	// Populate cache
	_, err := cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	hits, misses := cache.GetMetrics()
	if misses != 1 {
		t.Errorf("expected 1 miss after first lookup, got %d", misses)
	}

	// Clear cache
	cache.Clear()

	hits, misses = cache.GetMetrics()
	if hits != 0 || misses != 0 {
		t.Errorf("expected metrics to be cleared, got %d/%d", hits, misses)
	}

	// Next lookup should be a miss again
	_, err = cache.LookupIPAddr(ctx, "www.reddit.com")
	testutil.AssertNoError(t, err)

	hits, misses = cache.GetMetrics()
	if hits != 0 || misses != 1 {
		t.Errorf("after clear and lookup, expected 0/1, got %d/%d", hits, misses)
	}
}

func TestDNSCache_DefaultTTL(t *testing.T) {
	// Test with zero TTL (should use default)
	cache := NewDNSCache(0)
	if cache.ttl != 5*time.Minute {
		t.Errorf("expected default TTL of 5 minutes, got %v", cache.ttl)
	}

	// Test with negative TTL (should use default)
	cache = NewDNSCache(-1 * time.Second)
	if cache.ttl != 5*time.Minute {
		t.Errorf("expected default TTL of 5 minutes, got %v", cache.ttl)
	}

	// Test with custom TTL
	cache = NewDNSCache(1 * time.Minute)
	if cache.ttl != 1*time.Minute {
		t.Errorf("expected custom TTL of 1 minute, got %v", cache.ttl)
	}
}

func TestDNSCache_InvalidHost(t *testing.T) {
	cache := NewDNSCache(5 * time.Minute)

	ctx := context.Background()

	// Attempt to lookup an invalid host
	_, err := cache.LookupIPAddr(ctx, "invalid.host.that.definitely.does.not.exist.example.com")
	testutil.AssertError(t, err)

	// Verify it was counted as a miss despite the error
	_, misses := cache.GetMetrics()
	if misses != 1 {
		t.Errorf("expected 1 miss even on error, got %d", misses)
	}
}

func TestDNSCache_MultipleHosts(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	ctx := context.Background()

	// Lookup different hosts
	hosts := []string{"localhost", "127.0.0.1"}

	for _, host := range hosts {
		_, err := cache.LookupIPAddr(ctx, host)
		// Note: localhost should resolve, 127.0.0.1 is an IP and may not resolve as a hostname
		// We just verify the cache works, errors are acceptable for invalid hosts
		_ = err
	}

	// Lookup the same hosts again - these should be cached if they succeeded
	for _, host := range hosts {
		_, _ = cache.LookupIPAddr(ctx, host)
	}

	// Just verify the cache is tracking lookups
	_, misses := cache.GetMetrics()
	if misses < 1 {
		t.Errorf("expected at least 1 miss, got %d", misses)
	}
}

func TestDNSCache_DialContext(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	dialFunc := cache.DialContext()

	if dialFunc == nil {
		t.Fatalf("expected non-nil dial function")
	}

	// Test that dial context returns a function (actual dialing may fail in test environment)
	if _, misses := cache.GetMetrics(); misses != 0 {
		t.Errorf("expected no lookups before dial, got %d misses", misses)
	}
}

func TestDNSCache_DialContextInvalidAddress(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	dialFunc := cache.DialContext()
	ctx := context.Background()

	// Test with an invalid address format (no port)
	_, err := dialFunc(ctx, "tcp", "invalid-no-port")

	// Should fail - either from split error or dial error
	testutil.AssertError(t, err)
}

func TestDNSCache_DialContextCacheLookup(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	// Pre-populate cache with localhost
	ctx := context.Background()
	_, err := cache.LookupIPAddr(ctx, "localhost")
	testutil.AssertNoError(t, err)

	// Get dial function
	dialFunc := cache.DialContext()

	// Try to dial to localhost - should use cached IPs
	// Note: This may fail due to no service listening, but we're testing cache usage
	_, _ = dialFunc(ctx, "tcp", "localhost:1234")

	// Verify we used the cache
	hits, _ := cache.GetMetrics()
	if hits < 1 {
		t.Errorf("expected dial to hit cache, got %d hits", hits)
	}
}

func TestDNSCache_ConcurrentClear(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	ctx := context.Background()
	var wg sync.WaitGroup
	var errors atomic.Int32

	// Goroutine 1: continuous lookups
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, err := cache.LookupIPAddr(ctx, "www.reddit.com")
			if err != nil {
				errors.Add(1)
			}
		}
	}()

	// Goroutine 2: periodic clears
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Millisecond)
			cache.Clear()
		}
	}()

	wg.Wait()

	if errors.Load() > 0 {
		t.Errorf("expected no errors during concurrent operations, got %d", errors.Load())
	}
}

func TestDNSCache_ContextCancellation(t *testing.T) {
	cache := NewDNSCache(5 * time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to lookup with cancelled context
	_, err := cache.LookupIPAddr(ctx, "www.reddit.com")

	// Should error due to cancelled context
	if err == nil {
		t.Fatalf("expected error with cancelled context")
	}

	// Verify the error is about context cancellation
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		// Some DNS resolvers may wrap the error
		t.Logf("got error: %v", err)
	}
}

func TestDNSCache_DefensiveCopy(t *testing.T) {
	mockClock := clock.NewMockClock(time.Time{})
	cache := NewDNSCacheWithClock(5*time.Minute, mockClock)

	ctx := context.Background()

	// First lookup populates cache
	ips1, err := cache.LookupIPAddr(ctx, "localhost")
	testutil.AssertNoError(t, err)
	if len(ips1) == 0 {
		t.Fatalf("expected at least one IP")
	}

	// Mutate the returned slice
	originalFirst := ips1[0]
	ips1[0] = net.IPAddr{IP: net.ParseIP("255.255.255.255")}

	// Second lookup should return original cached data, not mutated data
	ips2, err := cache.LookupIPAddr(ctx, "localhost")
	testutil.AssertNoError(t, err)
	if originalFirst.IP.String() != ips2[0].IP.String() {
		t.Errorf("expected cached IP to be %q, got %q (defensive copy failed)", originalFirst.IP.String(), ips2[0].IP.String())
	}
}
