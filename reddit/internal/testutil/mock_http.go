// Package testutil provides reusable test helpers and assertions for the Reddit API wrapper.
package testutil

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// MockHTTPClient is a thread-safe mock implementation of the HTTPClient interface for testing.
// It provides configurable behavior for all HTTP client operations and tracks call counts
// for verification in tests.
//
// The mock supports three patterns for customization:
//  1. Set function fields directly to customize specific method behavior
//  2. Use helper methods (WithDoFunc, WithError, etc.) for common scenarios
//  3. Use default behavior (returns nil/empty values) when no customization is needed
//
// All call counters use atomic operations for thread-safety, making this mock suitable
// for testing concurrent operations.
//
// Example:
//
//	// Custom behavior
//	mock := &testutil.MockHTTPClient{
//		DoFunc: func(req *http.Request, v *types.Thing) error {
//			// Fill v with test data
//			return nil
//		},
//	}
//
//	// Helper methods
//	mock := testutil.NewMockHTTPClient().WithError(errors.New("network error"))
//
//	// Verify calls
//	if mock.DoCalls() != 1 {
//		t.Errorf("expected 1 Do call, got %d", mock.DoCalls())
//	}
type MockHTTPClient struct {
	// NewRequestFunc is called by NewRequest if set. If nil, a default implementation is used
	// that creates a valid request with the given parameters.
	NewRequestFunc func(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error)

	// DoFunc is called by Do if set. If nil, returns nil (success with no data).
	DoFunc func(req *http.Request, v *types.Thing) error

	// DoThingArrayFunc is called by DoThingArray if set. If nil, returns nil, nil.
	DoThingArrayFunc func(req *http.Request) ([]*types.Thing, error)

	// DoMoreChildrenFunc is called by DoMoreChildren if set. If nil, returns nil, nil.
	DoMoreChildrenFunc func(req *http.Request) ([]*types.Thing, error)

	// Call counters (atomic for thread-safety)
	newRequestCalls     atomic.Int32
	doCalls             atomic.Int32
	doThingArrayCalls   atomic.Int32
	doMoreChildrenCalls atomic.Int32
}

// NewMockHTTPClient creates a new MockHTTPClient with default behavior.
// All methods return success with empty/nil values unless customized.
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{}
}

// NewRequest creates a new HTTP request. If NewRequestFunc is set, it delegates to that function.
// Otherwise, creates a valid request with the given parameters pointing to oauth.reddit.com.
func (m *MockHTTPClient) NewRequest(ctx context.Context, method, path string, body io.Reader, params ...url.Values) (*http.Request, error) {
	m.newRequestCalls.Add(1)

	if m.NewRequestFunc != nil {
		return m.NewRequestFunc(ctx, method, path, body, params...)
	}

	// Default implementation: create a valid request
	req, err := http.NewRequestWithContext(ctx, method, "https://oauth.reddit.com/"+path, body)
	if err != nil {
		return nil, err
	}

	if len(params) > 0 && params[0] != nil {
		req.URL.RawQuery = params[0].Encode()
	}

	return req, nil
}

// Do executes an HTTP request. If DoFunc is set, it delegates to that function.
// Otherwise, returns nil (success with no error).
func (m *MockHTTPClient) Do(req *http.Request, v *types.Thing) error {
	m.doCalls.Add(1)

	if m.DoFunc != nil {
		return m.DoFunc(req, v)
	}

	return nil
}

// DoThingArray executes an HTTP request and returns Things. If DoThingArrayFunc is set,
// it delegates to that function. Otherwise, returns nil, nil.
func (m *MockHTTPClient) DoThingArray(req *http.Request) ([]*types.Thing, error) {
	m.doThingArrayCalls.Add(1)

	if m.DoThingArrayFunc != nil {
		return m.DoThingArrayFunc(req)
	}

	return nil, nil
}

// DoMoreChildren executes a morechildren request. If DoMoreChildrenFunc is set,
// it delegates to that function. Otherwise, returns nil, nil.
func (m *MockHTTPClient) DoMoreChildren(req *http.Request) ([]*types.Thing, error) {
	m.doMoreChildrenCalls.Add(1)

	if m.DoMoreChildrenFunc != nil {
		return m.DoMoreChildrenFunc(req)
	}

	return nil, nil
}

// Call count accessors

// NewRequestCalls returns the number of times NewRequest was called.
func (m *MockHTTPClient) NewRequestCalls() int32 {
	return m.newRequestCalls.Load()
}

// DoCalls returns the number of times Do was called.
func (m *MockHTTPClient) DoCalls() int32 {
	return m.doCalls.Load()
}

// DoThingArrayCalls returns the number of times DoThingArray was called.
func (m *MockHTTPClient) DoThingArrayCalls() int32 {
	return m.doThingArrayCalls.Load()
}

// DoMoreChildrenCalls returns the number of times DoMoreChildren was called.
func (m *MockHTTPClient) DoMoreChildrenCalls() int32 {
	return m.doMoreChildrenCalls.Load()
}

// ResetCalls resets all call counters to zero. Useful when reusing a mock across multiple test cases.
func (m *MockHTTPClient) ResetCalls() {
	m.newRequestCalls.Store(0)
	m.doCalls.Store(0)
	m.doThingArrayCalls.Store(0)
	m.doMoreChildrenCalls.Store(0)
}

// Helper methods for common test scenarios

// WithDoFunc sets the DoFunc and returns the mock for method chaining.
//
// Example:
//
//	mock := NewMockHTTPClient().WithDoFunc(func(req *http.Request, v *types.Thing) error {
//		v.Kind = "Listing"
//		return nil
//	})
func (m *MockHTTPClient) WithDoFunc(fn func(req *http.Request, v *types.Thing) error) *MockHTTPClient {
	m.DoFunc = fn
	return m
}

// WithDoThingArrayFunc sets the DoThingArrayFunc and returns the mock for method chaining.
//
// Example:
//
//	mock := NewMockHTTPClient().WithDoThingArrayFunc(func(req *http.Request) ([]*types.Thing, error) {
//		return []*types.Thing{{Kind: "Listing"}}, nil
//	})
func (m *MockHTTPClient) WithDoThingArrayFunc(fn func(req *http.Request) ([]*types.Thing, error)) *MockHTTPClient {
	m.DoThingArrayFunc = fn
	return m
}

// WithDoMoreChildrenFunc sets the DoMoreChildrenFunc and returns the mock for method chaining.
//
// Example:
//
//	mock := NewMockHTTPClient().WithDoMoreChildrenFunc(func(req *http.Request) ([]*types.Thing, error) {
//		return []*types.Thing{{Kind: "t1"}}, nil
//	})
func (m *MockHTTPClient) WithDoMoreChildrenFunc(fn func(req *http.Request) ([]*types.Thing, error)) *MockHTTPClient {
	m.DoMoreChildrenFunc = fn
	return m
}

// WithError configures all Do* methods to return the given error.
// This is useful for testing error handling paths.
//
// Example:
//
//	mock := NewMockHTTPClient().WithError(errors.New("network timeout"))
func (m *MockHTTPClient) WithError(err error) *MockHTTPClient {
	m.DoFunc = func(req *http.Request, v *types.Thing) error {
		return err
	}
	m.DoThingArrayFunc = func(req *http.Request) ([]*types.Thing, error) {
		return nil, err
	}
	m.DoMoreChildrenFunc = func(req *http.Request) ([]*types.Thing, error) {
		return nil, err
	}
	return m
}

// WithSuccess configures the Do method to populate the Thing with test data.
// This is useful for testing successful API responses.
//
// Example:
//
//	mock := NewMockHTTPClient().WithSuccess(&types.Thing{Kind: "Listing"})
func (m *MockHTTPClient) WithSuccess(thing *types.Thing) *MockHTTPClient {
	m.DoFunc = func(req *http.Request, v *types.Thing) error {
		*v = *thing
		return nil
	}
	return m
}

// WithThingArraySuccess configures DoThingArray to return the given Things.
// This is useful for testing successful comments endpoint responses.
//
// Example:
//
//	mock := NewMockHTTPClient().WithThingArraySuccess([]*types.Thing{
//		{Kind: "Listing"},
//		{Kind: "Listing"},
//	})
func (m *MockHTTPClient) WithThingArraySuccess(things []*types.Thing) *MockHTTPClient {
	m.DoThingArrayFunc = func(req *http.Request) ([]*types.Thing, error) {
		return things, nil
	}
	return m
}
