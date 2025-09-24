package internal

import (
	"context"
	"sync"
)

// ConnectionManager handles thread-safe connection initialization for the Reddit client.
// It ensures that initialization happens exactly once, even when called concurrently
// from multiple goroutines.
type ConnectionManager struct {
	once  sync.Once
	err   error
	ready chan struct{}
}

// NewConnectionManager creates a new ConnectionManager instance ready for use.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		ready: make(chan struct{}),
	}
}

// Initialize runs the provided initialization function exactly once.
// If called multiple times concurrently, only the first call will execute the function,
// and all calls will wait for the initialization to complete before returning.
//
// The context passed to the first call will be used for initialization.
// Subsequent calls will return the result of the first initialization attempt.
func (cm *ConnectionManager) Initialize(ctx context.Context, fn func(context.Context) error) error {
	cm.once.Do(func() {
		cm.err = fn(ctx)
		close(cm.ready)
	})

	// Wait for initialization to complete if called concurrently
	<-cm.ready
	return cm.err
}

// Error returns the error from the initialization attempt, if any.
// This can be called to check the initialization status without triggering it.
func (cm *ConnectionManager) Error() error {
	select {
	case <-cm.ready:
		return cm.err
	default:
		return nil
	}
}

// IsInitialized returns true if the initialization has been attempted.
func (cm *ConnectionManager) IsInitialized() bool {
	select {
	case <-cm.ready:
		return true
	default:
		return false
	}
}