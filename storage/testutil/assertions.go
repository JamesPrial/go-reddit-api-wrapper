package testutil

import "testing"

// AssertNoError fails the test if err is not nil.
// This is a test helper for integration tests to verify no errors occurred.
//
// Example:
//
//	err := someFunction()
//	testutil.AssertNoError(t, err)
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
