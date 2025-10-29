package client

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// TransportConfig holds configuration for optimized HTTP transport.
type TransportConfig struct {
	// MaxIdleConns sets the maximum number of idle connections to keep open
	MaxIdleConns int
	// MaxIdleConnsPerHost sets the maximum idle connections per host
	MaxIdleConnsPerHost int
	// IdleConnTimeout sets how long an idle connection is kept alive
	IdleConnTimeout time.Duration
	// DisableKeepAlives disables HTTP keep-alive
	DisableKeepAlives bool
	// ForceAttemptHTTP2 attempts to negotiate HTTP/2 even without TLS
	ForceAttemptHTTP2 bool
	// DNSCache is optional DNS cache for reducing lookups
	DNSCache *DNSCache
}

// TransportMetrics tracks metrics about HTTP transport usage.
// All fields are protected by atomic operations for thread-safe access.
type TransportMetrics struct {
	ConnectionsOpened atomic.Int64
	ConnectionsReused atomic.Int64
	ConnectionsFailed atomic.Int64
	DNSLookupsTotal   atomic.Int64
}

// TransportMetricsSnapshot is a non-atomic snapshot of transport metrics.
type TransportMetricsSnapshot struct {
	ConnectionsOpened int64
	ConnectionsReused int64
	ConnectionsFailed int64
	DNSLookupsTotal   int64
}

// GetTransportMetrics returns a snapshot of the current transport metrics.
func (tm *TransportMetrics) GetTransportMetrics() TransportMetricsSnapshot {
	return TransportMetricsSnapshot{
		ConnectionsOpened: tm.ConnectionsOpened.Load(),
		ConnectionsReused: tm.ConnectionsReused.Load(),
		ConnectionsFailed: tm.ConnectionsFailed.Load(),
		DNSLookupsTotal:   tm.DNSLookupsTotal.Load(),
	}
}

// NewOptimizedTransport creates an HTTP transport optimized for the Reddit API.
// If config is nil, sensible defaults optimized for Reddit are used.
// Returns the transport and a metrics structure for monitoring performance.
func NewOptimizedTransport(config *TransportConfig) (*http.Transport, *TransportMetrics) {
	// Use defaults if no config provided
	if config == nil {
		config = &TransportConfig{}
	}

	metrics := &TransportMetrics{}

	// Set Reddit-optimized defaults
	maxIdleConns := config.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 100
	}

	maxIdleConnsPerHost := config.MaxIdleConnsPerHost
	if maxIdleConnsPerHost == 0 {
		maxIdleConnsPerHost = 10
	}

	idleConnTimeout := config.IdleConnTimeout
	if idleConnTimeout == 0 {
		idleConnTimeout = 90 * time.Second
	}

	// Create the base transport with optimized settings
	// Default ForceAttemptHTTP2 to true for HTTP/2 support
	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		IdleConnTimeout:     idleConnTimeout,
		DisableKeepAlives:   config.DisableKeepAlives,
		ForceAttemptHTTP2:   true,
	}

	// Configure custom dialer with DNS cache if provided
	if config.DNSCache != nil {
		transport.DialContext = createDialContextWithDNSCache(config.DNSCache, metrics)
	} else {
		// Use default dialer with metrics tracking
		transport.DialContext = createDialContextWithMetrics(metrics)
	}

	return transport, metrics
}

// createDialContextWithDNSCache creates a DialContext function that uses DNS cache.
func createDialContextWithDNSCache(dnsCache *DNSCache, metrics *TransportMetrics) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Track connection attempt
		metrics.ConnectionsOpened.Add(1)

		// Use the DNS cache's DialContext function which handles lookups
		dialFunc := dnsCache.DialContext()
		conn, err := dialFunc(ctx, network, addr)
		if err != nil {
			// Failed to establish connection
			metrics.ConnectionsOpened.Add(-1)
			metrics.ConnectionsFailed.Add(1)
			return nil, err
		}

		// Update DNS metrics from cache
		dnsHits, dnsMisses := dnsCache.GetMetrics()
		metrics.DNSLookupsTotal.Store(dnsHits + dnsMisses)

		return conn, nil
	}
}

// createDialContextWithMetrics creates a DialContext function that tracks basic metrics.
func createDialContextWithMetrics(metrics *TransportMetrics) func(ctx context.Context, network, addr string) (net.Conn, error) {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Track connection attempt
		metrics.ConnectionsOpened.Add(1)
		conn, err := baseDialer.DialContext(ctx, network, addr)
		if err != nil {
			// Failed to establish connection
			metrics.ConnectionsOpened.Add(-1)
			metrics.ConnectionsFailed.Add(1)
			return nil, err
		}

		return conn, nil
	}
}
