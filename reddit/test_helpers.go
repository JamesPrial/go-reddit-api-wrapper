package graw

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// mockTokenProvider is a simple test implementation of the TokenProvider interface.
// It returns a fixed token and expiry time, or an error if configured to do so.
type mockTokenProvider struct {
	token           string
	expiry          time.Time
	err             error
	getCalls        atomic.Int32
	invalidateCalls atomic.Int32
	mu              sync.Mutex
}

// GetToken implements the TokenProvider interface.
func (m *mockTokenProvider) GetToken(ctx context.Context) (string, time.Time, error) {
	m.getCalls.Add(1)
	if m.err != nil {
		return "", time.Time{}, m.err
	}
	m.mu.Lock()
	if m.expiry.IsZero() {
		m.expiry = time.Now().Add(1 * time.Hour)
	}
	expiry := m.expiry
	m.mu.Unlock()
	return m.token, expiry, nil
}

// InvalidateToken implements the TokenProvider interface.
func (m *mockTokenProvider) InvalidateToken(ctx context.Context) error {
	m.invalidateCalls.Add(1)
	return nil
}
