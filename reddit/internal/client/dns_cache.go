package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/clock"
)

// DNSCache provides application-level DNS caching to reduce lookups to Reddit API endpoints.
// This improves connection establishment time for repeated requests to the same hosts.
// DNSCache is thread-safe and uses minimal locking with read-heavy workloads in mind.
type DNSCache struct {
	cache    map[string]*dnsCacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
	resolver *net.Resolver
	clock    clock.Clock
	group    singleflight.Group

	// Metrics tracked with atomic operations for minimal lock contention
	hits   atomic.Int64
	misses atomic.Int64
}

// dnsCacheEntry holds cached IP addresses and their expiration time.
type dnsCacheEntry struct {
	ips       []net.IPAddr
	expiresAt time.Time
}

// NewDNSCache returns a new DNS cache with the specified TTL.
// The cache will be empty initially and populated on demand during lookups.
// If ttl is zero or negative, a default TTL of 5 minutes is used.
func NewDNSCache(ttl time.Duration) *DNSCache {
	return NewDNSCacheWithClock(ttl, nil)
}

// NewDNSCacheWithClock returns a new DNS cache with the specified TTL and clock.
// The clock is used for time operations and can be overridden for testing.
// If ttl is zero or negative, a default TTL of 5 minutes is used.
// If clock is nil, a real clock is used.
// A background cleanup goroutine is started for production use (non-mock clocks).
func NewDNSCacheWithClock(ttl time.Duration, clk clock.Clock) *DNSCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}

	c := &DNSCache{
		cache:    make(map[string]*dnsCacheEntry),
		ttl:      ttl,
		resolver: net.DefaultResolver,
		clock:    clk,
	}

	// Start cleanup goroutine for non-mock clocks (production use)
	// Mock clocks skip this to keep tests deterministic
	if _, isMock := clk.(*clock.MockClock); !isMock {
		go c.cleanupExpired()
	}

	return c
}

// LookupIPAddr performs a DNS lookup for the given host, using the cache if available.
// Multiple concurrent lookups for the same host are deduplicated (singleflight pattern).
// Returns a defensive copy of the cached IPs to prevent external mutation.
// Thread-safe.
func (c *DNSCache) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	// Fast path: check cache with read lock
	c.mu.RLock()
	if entry, ok := c.cache[host]; ok {
		if c.clock.Now().Before(entry.expiresAt) {
			// Cache hit - return defensive copy
			ipsCopy := make([]net.IPAddr, len(entry.ips))
			copy(ipsCopy, entry.ips)
			c.mu.RUnlock()
			c.hits.Add(1)
			return ipsCopy, nil
		}
		// Entry expired - clean it up while we have the lock
		c.mu.RUnlock()
		c.mu.Lock()
		delete(c.cache, host)
		c.mu.Unlock()
	} else {
		c.mu.RUnlock()
	}

	// Cache miss or expired - use singleflight to deduplicate concurrent lookups
	v, err, _ := c.group.Do(host, func() (interface{}, error) {
		// Perform actual DNS lookup
		ips, err := c.resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("dns lookup failed for %q: %w", host, err)
		}

		// Store defensive copy in cache
		ipsCopy := make([]net.IPAddr, len(ips))
		copy(ipsCopy, ips)

		c.mu.Lock()
		c.cache[host] = &dnsCacheEntry{
			ips:       ipsCopy,
			expiresAt: c.clock.Now().Add(c.ttl),
		}
		c.mu.Unlock()

		return ips, nil
	})

	if err != nil {
		c.misses.Add(1)
		return nil, err
	}

	c.misses.Add(1)
	return v.([]net.IPAddr), nil
}

// GetMetrics returns the current cache hit and miss counts.
// These counters are tracked using atomic operations and are safe to read concurrently.
func (c *DNSCache) GetMetrics() (hits, misses int64) {
	return c.hits.Load(), c.misses.Load()
}

// Clear removes all entries from the DNS cache and resets metrics.
// This is useful for testing or forcing a full refresh of cached DNS entries.
// Thread-safe.
func (c *DNSCache) Clear() {
	c.mu.Lock()
	c.cache = make(map[string]*dnsCacheEntry)
	c.mu.Unlock()

	c.hits.Store(0)
	c.misses.Store(0)
}

// cleanupExpired periodically removes expired entries from the cache.
// Runs in a background goroutine for production (non-mock clock) use.
// This prevents the cache from accumulating stale entries indefinitely.
func (c *DNSCache) cleanupExpired() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		now := c.clock.Now()
		c.mu.Lock()
		for host, entry := range c.cache {
			if now.After(entry.expiresAt) {
				delete(c.cache, host)
			}
		}
		c.mu.Unlock()
	}
}

// DialContext returns a DialContext function that uses cached DNS lookups.
// This function can be used with http.Transport's DialContext field to integrate
// DNS caching into the HTTP client's connection establishment.
func (c *DNSCache) DialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	// Create a dialer that will be used for actual connections
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			// If we can't split the host and port, fall back to standard dialing
			return baseDialer.DialContext(ctx, network, addr)
		}

		// Look up IPs using our cache
		ips, err := c.LookupIPAddr(ctx, host)
		if err != nil {
			// Fall back to standard dialing if cache lookup fails
			return baseDialer.DialContext(ctx, network, addr)
		}

		// Try each IP address until one succeeds
		var lastErr error
		for _, ip := range ips {
			addr := net.JoinHostPort(ip.String(), port)
			conn, err := baseDialer.DialContext(ctx, network, addr)
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}

		// All IPs failed, return the last error
		if lastErr != nil {
			return nil, fmt.Errorf("dial failed for %s: %w", host, lastErr)
		}
		return nil, fmt.Errorf("dial failed for %s: no addresses available", host)
	}
}
