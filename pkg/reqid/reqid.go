package reqid

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// ctxKey is an unexported type used as the key for storing request IDs in context.
// Using an unexported type prevents accidental collisions with keys from other packages.
type ctxKey struct{}

// Generate creates a new ULID using cryptographic randomness and the current time.
// The returned ULID is sortable by timestamp and cryptographically unique.
// Panics if ULID generation fails, which indicates a critical system issue (e.g., entropy exhaustion).
func Generate() string {
	u, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		// Entropy failure is a critical system issue that should not be silently ignored.
		panic(fmt.Sprintf("reqid: failed to generate ULID (system entropy exhausted?): %v", err))
	}
	return u.String()
}

// FromContext extracts the request ID from the context.
// Returns an empty string if the context is nil or no request ID is found.
//
// Example:
//
//	id := FromContext(ctx)
//	if id != "" {
//		log.Printf("Request ID: %s", id)
//	}
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// WithRequestID returns a new context with the given request ID attached.
// If the provided ID is empty, the original context is returned unchanged.
// The request ID can be retrieved later using FromContext.
//
// Example:
//
//	ctx = WithRequestID(ctx, "01ARZ3NDEKTSV4RRFFQ69G5FAV")
//	// Later retrieve it:
//	id := FromContext(ctx)
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		// Don't attach empty IDs to context
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// Ensure returns the request ID from context if it exists, otherwise generates a new one.
// If the context is nil, a new background context is created.
// This is useful for ensuring every request has an ID without explicit generation.
//
// Example:
//
//	ctx = Ensure(ctx)
//	id := FromContext(ctx)
//	// id is guaranteed to be non-empty
func Ensure(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id := FromContext(ctx); id != "" {
		return ctx
	}
	return WithRequestID(ctx, Generate())
}
