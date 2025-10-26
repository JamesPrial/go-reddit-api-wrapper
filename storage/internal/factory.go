package internal

import (
	"context"
	"sync"
)

// StoreFactory is a function that creates a Store from Config.
// This type is exported within the storage package tree but hidden from external packages
// due to the internal/ directory.
type StoreFactory[C any, S any] func(context.Context, C) (S, error)

// Registry manages factory registration for storage backends.
// It uses generics to allow type-safe factory registration without circular dependencies.
type Registry[C any, S any] struct {
	factories map[string]StoreFactory[C, S]
	mu        sync.RWMutex
}

// NewRegistry creates a new factory registry.
func NewRegistry[C any, S any]() *Registry[C, S] {
	return &Registry[C, S]{
		factories: make(map[string]StoreFactory[C, S]),
	}
}

// Register registers a factory function for a storage driver.
// This is called by backend subpackages (like sqlite) in their init() function.
func (r *Registry[C, S]) Register(driver string, factory StoreFactory[C, S]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[driver] = factory
}

// Get retrieves a factory function for a storage driver.
// Returns the factory and true if found, nil and false otherwise.
func (r *Registry[C, S]) Get(driver string) (StoreFactory[C, S], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[driver]
	return factory, ok
}
