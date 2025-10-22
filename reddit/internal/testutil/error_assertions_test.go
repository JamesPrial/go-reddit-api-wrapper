package testutil_test

import (
	"errors"
	"testing"

	graw "github.com/jamesprial/go-reddit-api-wrapper/reddit"
	"github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/testutil"
)

// TestAssertAuthError verifies AuthError type checking
func TestAssertAuthError(t *testing.T) {
	// Test with actual AuthError
	err := &graw.AuthError{
		StatusCode: 401,
		Message:    "invalid credentials",
	}
	testutil.AssertAuthError(t, err, "")
	testutil.AssertAuthError(t, err, "invalid credentials")

	// Test with wrapped AuthError
	wrappedErr := &graw.ParseError{
		Operation: "auth",
		Err:       err,
	}
	testutil.AssertAuthError(t, wrappedErr, "")
}

// TestAssertValidationError verifies ValidationError type checking
func TestAssertValidationError(t *testing.T) {
	// Test with actual ValidationError
	err := &graw.ValidationError{
		Field:  "subreddit",
		Value:  "invalid!",
		Reason: "contains invalid characters",
	}
	testutil.AssertValidationError(t, err, "")
	testutil.AssertValidationError(t, err, "subreddit")

	// Test with wrapped ValidationError
	wrappedErr := &graw.ParseError{
		Operation: "validation",
		Err:       err,
	}
	testutil.AssertValidationError(t, wrappedErr, "")
}

// TestAssertParseError verifies ParseError type checking
func TestAssertParseError(t *testing.T) {
	// Test with actual ParseError
	err := &graw.ParseError{
		Operation: "GetHot",
		Message:   "invalid JSON",
	}
	testutil.AssertParseError(t, err, "")
	testutil.AssertParseError(t, err, "invalid JSON")

	// Test with wrapped ParseError
	innerErr := errors.New("unexpected end of JSON input")
	wrappedErr := &graw.ParseError{
		Operation: "GetHot",
		Err:       innerErr,
	}
	testutil.AssertParseError(t, wrappedErr, "")
}

// TestAssertNetworkError verifies NetworkError type checking
func TestAssertNetworkError(t *testing.T) {
	// Test with actual NetworkError
	err := &graw.NetworkError{
		Method: "GET",
		URL:    "https://oauth.reddit.com/r/golang/hot",
		Err:    errors.New("connection refused"),
	}
	testutil.AssertNetworkError(t, err, "")
	testutil.AssertNetworkError(t, err, "connection refused")

	// Test with wrapped NetworkError
	wrappedErr := &graw.ParseError{
		Operation: "network",
		Err:       err,
	}
	testutil.AssertNetworkError(t, wrappedErr, "")
}

// TestAssertRateLimitError verifies RateLimitError type checking
func TestAssertRateLimitError(t *testing.T) {
	// Test with actual RateLimitError
	err := &graw.RateLimitError{
		Reason: "context_cancelled",
		Err:    errors.New("context deadline exceeded"),
	}
	testutil.AssertRateLimitError(t, err)

	// Test with wrapped RateLimitError
	wrappedErr := &graw.ParseError{
		Operation: "ratelimit",
		Err:       err,
	}
	testutil.AssertRateLimitError(t, wrappedErr)
}

// TestAssertErrorChain verifies error chain validation
func TestAssertErrorChain(t *testing.T) {
	// Create a chain: ParseError -> NetworkError
	networkErr := &graw.NetworkError{
		Method: "GET",
		URL:    "https://oauth.reddit.com/r/golang/hot",
		Err:    errors.New("connection refused"),
	}
	parseErr := &graw.ParseError{
		Operation: "GetHot",
		Err:       networkErr,
	}

	// Test that we can find both errors in the chain
	testutil.AssertErrorChain(t, parseErr, "ParseError", "NetworkError")

	// Test with a more complex chain: AuthError -> NetworkError
	authErr := &graw.AuthError{
		StatusCode: 401,
		Err:        networkErr,
	}
	testutil.AssertErrorChain(t, authErr, "AuthError", "NetworkError")
}
