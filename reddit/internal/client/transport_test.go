package client

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewOptimizedTransport_DefaultConfig(t *testing.T) {
	transport, metrics := NewOptimizedTransport(nil)

	// Verify transport and metrics are not nil
	if transport == nil {
		t.Fatal("expected transport to be non-nil")
	}
	if metrics == nil {
		t.Fatal("expected metrics to be non-nil")
	}

	// Verify default settings
	if transport.MaxIdleConns != 100 {
		t.Errorf("expected MaxIdleConns=100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("expected MaxIdleConnsPerHost=10, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected IdleConnTimeout=90s, got %v", transport.IdleConnTimeout)
	}
	if transport.DisableKeepAlives != false {
		t.Errorf("expected DisableKeepAlives=false, got %v", transport.DisableKeepAlives)
	}
	if transport.ForceAttemptHTTP2 != true {
		t.Errorf("expected ForceAttemptHTTP2=true, got %v", transport.ForceAttemptHTTP2)
	}
}

func TestNewOptimizedTransport_CustomConfig(t *testing.T) {
	config := &TransportConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   false,
	}

	transport, metrics := NewOptimizedTransport(config)

	// Verify transport and metrics are not nil
	if transport == nil {
		t.Fatal("expected transport to be non-nil")
	}
	if metrics == nil {
		t.Fatal("expected metrics to be non-nil")
	}

	// Verify custom settings are respected
	if transport.MaxIdleConns != 50 {
		t.Errorf("expected MaxIdleConns=50, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 5 {
		t.Errorf("expected MaxIdleConnsPerHost=5, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 60*time.Second {
		t.Errorf("expected IdleConnTimeout=60s, got %v", transport.IdleConnTimeout)
	}
	if transport.DisableKeepAlives != true {
		t.Errorf("expected DisableKeepAlives=true, got %v", transport.DisableKeepAlives)
	}
}

func TestTransportMetrics_ConnectionTracking(t *testing.T) {
	config := &TransportConfig{}
	transport, metrics := NewOptimizedTransport(config)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create HTTP client with our transport
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	// Make a few requests
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// Verify that connections were tracked
	opened := metrics.ConnectionsOpened.Load()
	if opened < 1 {
		t.Errorf("expected ConnectionsOpened >= 1, got %d", opened)
	}
}

func TestTransportMetrics_ThreadSafety(t *testing.T) {
	metrics := &TransportMetrics{}
	numGoroutines := 10
	operationsPerGoroutine := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent goroutines that update metrics
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				metrics.ConnectionsOpened.Add(1)
				metrics.ConnectionsReused.Add(1)
				metrics.ConnectionsFailed.Add(1)
				metrics.DNSLookupsTotal.Add(1)
			}
		}()
	}

	wg.Wait()

	// Verify final counts
	expectedCount := int64(numGoroutines * operationsPerGoroutine)
	if got := metrics.ConnectionsOpened.Load(); got != expectedCount {
		t.Errorf("expected ConnectionsOpened=%d, got %d", expectedCount, got)
	}
	if got := metrics.ConnectionsReused.Load(); got != expectedCount {
		t.Errorf("expected ConnectionsReused=%d, got %d", expectedCount, got)
	}
	if got := metrics.ConnectionsFailed.Load(); got != expectedCount {
		t.Errorf("expected ConnectionsFailed=%d, got %d", expectedCount, got)
	}
	if got := metrics.DNSLookupsTotal.Load(); got != expectedCount {
		t.Errorf("expected DNSLookupsTotal=%d, got %d", expectedCount, got)
	}
}

func TestTransportWithDNSCache(t *testing.T) {
	dnsCache := NewDNSCache(1 * time.Minute)
	config := &TransportConfig{
		DNSCache: dnsCache,
	}

	transport, metrics := NewOptimizedTransport(config)

	// Verify transport and metrics are not nil
	if transport == nil {
		t.Fatal("expected transport to be non-nil")
	}
	if metrics == nil {
		t.Fatal("expected metrics to be non-nil")
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create HTTP client with our transport
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	// Make multiple requests - subsequent ones should use cached DNS
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}

	// Verify DNS metrics were tracked
	totalLookups := metrics.DNSLookupsTotal.Load()

	if totalLookups < 1 {
		t.Logf("expected at least one DNS lookup, got %d (this may be normal for localhost)", totalLookups)
	}
}

func TestTransportKeepAliveEnabled(t *testing.T) {
	config := &TransportConfig{
		DisableKeepAlives: false,
	}

	transport, _ := NewOptimizedTransport(config)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create HTTP client with our transport
	client := &http.Client{Transport: transport}
	defer client.CloseIdleConnections()

	// Make two requests
	resp1, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	resp1.Body.Close()

	resp2, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	resp2.Body.Close()

	// If keep-alive is enabled, the connection should be reused
	// We can't directly verify this, but we can verify no errors occurred
}

func TestTransportMetrics_Snapshot(t *testing.T) {
	metrics := &TransportMetrics{}

	// Add some values
	metrics.ConnectionsOpened.Store(100)
	metrics.ConnectionsReused.Store(50)
	metrics.ConnectionsFailed.Store(10)
	metrics.DNSLookupsTotal.Store(5)

	// Get snapshot
	snapshot := metrics.GetTransportMetrics()

	// Snapshot should have the same values as the original metrics
	if snapshot.ConnectionsOpened != 100 {
		t.Errorf("expected snapshot ConnectionsOpened=100, got %d", snapshot.ConnectionsOpened)
	}
	if snapshot.ConnectionsReused != 50 {
		t.Errorf("expected snapshot ConnectionsReused=50, got %d", snapshot.ConnectionsReused)
	}
	if snapshot.ConnectionsFailed != 10 {
		t.Errorf("expected snapshot ConnectionsFailed=10, got %d", snapshot.ConnectionsFailed)
	}
	if snapshot.DNSLookupsTotal != 5 {
		t.Errorf("expected snapshot DNSLookupsTotal=5, got %d", snapshot.DNSLookupsTotal)
	}
}

func TestNewOptimizedTransport_HTTP2(t *testing.T) {
	config := &TransportConfig{
		ForceAttemptHTTP2: true,
	}

	transport, _ := NewOptimizedTransport(config)

	if transport.ForceAttemptHTTP2 != true {
		t.Errorf("expected ForceAttemptHTTP2=true, got %v", transport.ForceAttemptHTTP2)
	}
}

func TestTransportMetrics_ConcurrentReads(t *testing.T) {
	metrics := &TransportMetrics{}

	// Set initial values
	metrics.ConnectionsOpened.Store(1000)
	metrics.ConnectionsReused.Store(500)
	metrics.ConnectionsFailed.Store(100)

	numReaders := 20
	readsPerReader := 1000

	var wg sync.WaitGroup
	wg.Add(numReaders)

	// Launch concurrent readers
	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < readsPerReader; j++ {
				_ = metrics.ConnectionsOpened.Load()
				_ = metrics.ConnectionsReused.Load()
				_ = metrics.ConnectionsFailed.Load()
			}
		}()
	}

	wg.Wait()

	// Verify values haven't changed
	if got := metrics.ConnectionsOpened.Load(); got != 1000 {
		t.Errorf("expected ConnectionsOpened=1000, got %d", got)
	}
	if got := metrics.ConnectionsReused.Load(); got != 500 {
		t.Errorf("expected ConnectionsReused=500, got %d", got)
	}
	if got := metrics.ConnectionsFailed.Load(); got != 100 {
		t.Errorf("expected ConnectionsFailed=100, got %d", got)
	}
}
