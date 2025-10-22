package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestMockHTTPClient_DefaultBehavior verifies the mock's default behavior.
func TestMockHTTPClient_DefaultBehavior(t *testing.T) {
	mock := NewMockHTTPClient()
	ctx := context.Background()

	// NewRequest should create a valid request by default
	req, err := mock.NewRequest(ctx, "GET", "/r/golang/hot", nil)
	if err != nil {
		t.Errorf("NewRequest failed: %v", err)
	}
	if req == nil {
		t.Error("NewRequest returned nil request")
	}
	if mock.NewRequestCalls() != 1 {
		t.Errorf("expected 1 NewRequest call, got %d", mock.NewRequestCalls())
	}

	// Do should succeed by default
	var thing types.Thing
	if err := mock.Do(req, &thing); err != nil {
		t.Errorf("Do failed: %v", err)
	}
	if mock.DoCalls() != 1 {
		t.Errorf("expected 1 Do call, got %d", mock.DoCalls())
	}

	// DoThingArray should return nil by default
	things, err := mock.DoThingArray(req)
	if err != nil {
		t.Errorf("DoThingArray failed: %v", err)
	}
	if things != nil {
		t.Error("DoThingArray should return nil by default")
	}
	if mock.DoThingArrayCalls() != 1 {
		t.Errorf("expected 1 DoThingArray call, got %d", mock.DoThingArrayCalls())
	}

	// DoMoreChildren should return nil by default
	moreThings, err := mock.DoMoreChildren(req)
	if err != nil {
		t.Errorf("DoMoreChildren failed: %v", err)
	}
	if moreThings != nil {
		t.Error("DoMoreChildren should return nil by default")
	}
	if mock.DoMoreChildrenCalls() != 1 {
		t.Errorf("expected 1 DoMoreChildren call, got %d", mock.DoMoreChildrenCalls())
	}
}

// TestMockHTTPClient_WithError verifies error injection.
func TestMockHTTPClient_WithError(t *testing.T) {
	expectedErr := errors.New("test error")
	mock := NewMockHTTPClient().WithError(expectedErr)
	ctx := context.Background()

	req, _ := mock.NewRequest(ctx, "GET", "/test", nil)

	// All Do methods should return the error
	var thing types.Thing
	if err := mock.Do(req, &thing); err != expectedErr {
		t.Errorf("Do error = %v, want %v", err, expectedErr)
	}

	if _, err := mock.DoThingArray(req); err != expectedErr {
		t.Errorf("DoThingArray error = %v, want %v", err, expectedErr)
	}

	if _, err := mock.DoMoreChildren(req); err != expectedErr {
		t.Errorf("DoMoreChildren error = %v, want %v", err, expectedErr)
	}
}

// TestMockHTTPClient_ResetCalls verifies call counter reset.
func TestMockHTTPClient_ResetCalls(t *testing.T) {
	mock := NewMockHTTPClient()
	ctx := context.Background()

	// Make some calls
	req, _ := mock.NewRequest(ctx, "GET", "/test", nil)
	mock.Do(req, &types.Thing{})
	mock.DoThingArray(req)

	// Verify counters are non-zero
	if mock.NewRequestCalls() == 0 || mock.DoCalls() == 0 {
		t.Error("expected non-zero call counts before reset")
	}

	// Reset
	mock.ResetCalls()

	// Verify all counters are zero
	if mock.NewRequestCalls() != 0 {
		t.Errorf("NewRequestCalls after reset = %d, want 0", mock.NewRequestCalls())
	}
	if mock.DoCalls() != 0 {
		t.Errorf("DoCalls after reset = %d, want 0", mock.DoCalls())
	}
	if mock.DoThingArrayCalls() != 0 {
		t.Errorf("DoThingArrayCalls after reset = %d, want 0", mock.DoThingArrayCalls())
	}
	if mock.DoMoreChildrenCalls() != 0 {
		t.Errorf("DoMoreChildrenCalls after reset = %d, want 0", mock.DoMoreChildrenCalls())
	}
}

// TestMockHTTPClient_CustomBehavior verifies custom function injection.
func TestMockHTTPClient_CustomBehavior(t *testing.T) {
	expectedThings := []*types.Thing{
		{Kind: "t3", Data: nil},
	}

	mock := NewMockHTTPClient().WithThingArraySuccess(expectedThings)
	ctx := context.Background()

	req, _ := mock.NewRequest(ctx, "GET", "/test", nil)
	things, err := mock.DoThingArray(req)

	if err != nil {
		t.Errorf("DoThingArray failed: %v", err)
	}

	if len(things) != len(expectedThings) {
		t.Errorf("got %d things, want %d", len(things), len(expectedThings))
	}
}

// TestMockParser_DefaultBehavior verifies the mock's default behavior.
func TestMockParser_DefaultBehavior(t *testing.T) {
	mock := NewMockParser()
	ctx := context.Background()
	thing := &types.Thing{Kind: "Listing"}

	// ParseThing should return nil by default
	result, err := mock.ParseThing(ctx, thing)
	if err != nil {
		t.Errorf("ParseThing failed: %v", err)
	}
	if result != nil {
		t.Error("ParseThing should return nil by default")
	}

	// ExtractPosts should return nil by default
	posts, err := mock.ExtractPosts(ctx, thing)
	if err != nil {
		t.Errorf("ExtractPosts failed: %v", err)
	}
	if posts != nil {
		t.Error("ExtractPosts should return nil by default")
	}

	// ExtractPostAndComments should return empty response by default
	resp, err := mock.ExtractPostAndComments(ctx, []*types.Thing{thing})
	if err != nil {
		t.Errorf("ExtractPostAndComments failed: %v", err)
	}
	if resp == nil {
		t.Error("ExtractPostAndComments should return empty response, not nil")
	}
}

// TestMockParser_WithError verifies error injection.
func TestMockParser_WithError(t *testing.T) {
	expectedErr := errors.New("parse error")
	mock := NewMockParser().WithError(expectedErr)
	ctx := context.Background()
	thing := &types.Thing{Kind: "Listing"}

	// All methods should return the error
	if _, err := mock.ParseThing(ctx, thing); err != expectedErr {
		t.Errorf("ParseThing error = %v, want %v", err, expectedErr)
	}

	if _, err := mock.ExtractPosts(ctx, thing); err != expectedErr {
		t.Errorf("ExtractPosts error = %v, want %v", err, expectedErr)
	}

	if _, err := mock.ExtractPostAndComments(ctx, []*types.Thing{thing}); err != expectedErr {
		t.Errorf("ExtractPostAndComments error = %v, want %v", err, expectedErr)
	}
}

// TestMockParser_WithPosts verifies post injection.
func TestMockParser_WithPosts(t *testing.T) {
	expectedPosts := []*types.Post{
		DefaultPost(),
	}

	mock := NewMockParser().WithPosts(expectedPosts)
	ctx := context.Background()

	posts, err := mock.ExtractPosts(ctx, &types.Thing{})
	if err != nil {
		t.Errorf("ExtractPosts failed: %v", err)
	}

	if len(posts) != len(expectedPosts) {
		t.Errorf("got %d posts, want %d", len(posts), len(expectedPosts))
	}
}

// TestMockParser_WithCommentsResponse verifies comments response injection.
func TestMockParser_WithCommentsResponse(t *testing.T) {
	expectedResp := &types.CommentsResponse{
		Post:     DefaultPost(),
		Comments: []*types.Comment{DefaultComment()},
	}

	mock := NewMockParser().WithCommentsResponse(expectedResp)
	ctx := context.Background()

	resp, err := mock.ExtractPostAndComments(ctx, []*types.Thing{})
	if err != nil {
		t.Errorf("ExtractPostAndComments failed: %v", err)
	}

	if resp != expectedResp {
		t.Error("got different response than expected")
	}
}

// TestMockHTTPClient_ThreadSafety verifies atomic counter thread-safety.
func TestMockHTTPClient_ThreadSafety(t *testing.T) {
	mock := NewMockHTTPClient()
	ctx := context.Background()

	// Make concurrent calls
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req, _ := mock.NewRequest(ctx, "GET", "/test", nil)
			mock.Do(req, &types.Thing{})
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	if mock.NewRequestCalls() != 10 {
		t.Errorf("NewRequestCalls = %d, want 10", mock.NewRequestCalls())
	}
	if mock.DoCalls() != 10 {
		t.Errorf("DoCalls = %d, want 10", mock.DoCalls())
	}
}
