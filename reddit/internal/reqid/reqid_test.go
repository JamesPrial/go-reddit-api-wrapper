package reqid

import (
	"context"
	"strings"
	"testing"
)

func TestGenerate_ReturnsValidULID(t *testing.T) {
	id := Generate()
	if id == "" {
		t.Fatal("Generate() returned empty string, expected valid ULID")
	}
	if len(id) != 26 {
		t.Fatalf("ULID length = %d, want 26", len(id))
	}
}

func TestGenerate_ReturnsDifferentIDs(t *testing.T) {
	id1 := Generate()
	id2 := Generate()
	if id1 == id2 {
		t.Fatalf("Generate() returned same ID twice: %s", id1)
	}
}

func TestFromContext_WithNilContext(t *testing.T) {
	id := FromContext(nil)
	if id != "" {
		t.Fatalf("FromContext(nil) = %q, want empty string", id)
	}
}

func TestFromContext_WithoutRequestID(t *testing.T) {
	ctx := context.Background()
	id := FromContext(ctx)
	if id != "" {
		t.Fatalf("FromContext(ctx) without ID = %q, want empty string", id)
	}
}

func TestFromContext_WithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-id-123")
	id := FromContext(ctx)
	if id != "test-id-123" {
		t.Fatalf("FromContext(ctx) = %q, want test-id-123", id)
	}
}

func TestWithRequestID_RejectsEmptyString(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "initial-id")
	// Try to overwrite with empty string
	ctx2 := WithRequestID(ctx, "")
	// Should return the original context unchanged
	if ctx2 != ctx {
		t.Fatal("WithRequestID(ctx, \"\") should return original ctx unchanged")
	}
	// Original ID should still be there
	if id := FromContext(ctx2); id != "initial-id" {
		t.Fatalf("ID after rejecting empty string = %q, want initial-id", id)
	}
}

func TestWithRequestID_AttachesNonEmptyID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "my-request-id")
	if id := FromContext(ctx); id != "my-request-id" {
		t.Fatalf("FromContext(ctx) = %q, want my-request-id", id)
	}
}

func TestWithRequestID_CanOverwriteExistingID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "old-id")
	ctx = WithRequestID(ctx, "new-id")
	if id := FromContext(ctx); id != "new-id" {
		t.Fatalf("FromContext(ctx) = %q, want new-id", id)
	}
}

func TestEnsure_WithNilContext(t *testing.T) {
	ctx := Ensure(nil)
	if ctx == nil {
		t.Fatal("Ensure(nil) returned nil, expected valid context")
	}
	id := FromContext(ctx)
	if id == "" {
		t.Fatal("Ensure(nil) context has no ID, expected generated ID")
	}
}

func TestEnsure_WithExistingID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "existing-id")
	ctx = Ensure(ctx)
	if id := FromContext(ctx); id != "existing-id" {
		t.Fatalf("Ensure(ctx) changed ID from existing-id to %q", id)
	}
}

func TestEnsure_GeneratesNewIDWhenMissing(t *testing.T) {
	ctx := context.Background()
	ctx = Ensure(ctx)
	id := FromContext(ctx)
	if id == "" {
		t.Fatal("Ensure(ctx) failed to generate ID")
	}
	// Verify it's a valid ULID
	if len(id) != 26 {
		t.Fatalf("Generated ID length = %d, want 26", len(id))
	}
}

func TestEnsure_IDIsNonEmpty(t *testing.T) {
	ctx := context.Background()
	ctx = Ensure(ctx)
	if FromContext(ctx) == "" {
		t.Fatal("Ensure() guarantee violated: ID is empty")
	}
}

func TestGeneratePanicsOnError(t *testing.T) {
	// This test documents the expected behavior when ULID generation fails.
	// Under normal circumstances, this won't happen, but if it does, we expect a panic.
	// We can't directly force rand.Reader to fail in this test, but we document the behavior.
	// In real scenarios, if entropy is exhausted, Generate() will panic.

	id := Generate()
	if id == "" {
		t.Fatal("Generate() should panic on error, not return empty string")
	}
}

func TestIDFormat_IsValidULID(t *testing.T) {
	id := Generate()
	// ULID format: 26 characters, uppercase alphanumeric
	if len(id) != 26 {
		t.Fatalf("ID length = %d, want 26", len(id))
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			t.Fatalf("ID contains invalid character: %c", r)
		}
	}
}

func TestContextIsolation_DifferentContexts(t *testing.T) {
	ctx1 := context.Background()
	ctx1 = WithRequestID(ctx1, "id-1")

	ctx2 := context.Background()
	ctx2 = WithRequestID(ctx2, "id-2")

	if FromContext(ctx1) != "id-1" {
		t.Fatalf("ctx1 ID changed unexpectedly")
	}
	if FromContext(ctx2) != "id-2" {
		t.Fatalf("ctx2 ID changed unexpectedly")
	}
}

func TestContextIsolation_CancelledContext(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ctx = WithRequestID(ctx, "test-id")

	if id := FromContext(ctx); id != "test-id" {
		t.Fatalf("ID not preserved in cancelled context: %q", id)
	}

	cancel()
	// ID should still be accessible even after cancellation
	if id := FromContext(ctx); id != "test-id" {
		t.Fatalf("ID lost after context cancellation: %q", id)
	}
}
