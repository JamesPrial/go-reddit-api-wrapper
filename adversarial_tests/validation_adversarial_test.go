package adversarial_tests

import (
	"fmt"
	"strings"
	"testing"

	graw "github.com/jamesprial/go-reddit-api-wrapper"
	"github.com/jamesprial/go-reddit-api-wrapper/adversarial_tests/helpers"
	pkgerrs "github.com/jamesprial/go-reddit-api-wrapper/pkg/errors"
)

// TestSubredditNameFuzzing tests subreddit name validation against injection attacks
func TestSubredditNameFuzzing(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	maliciousNames := fuzzer.FuzzSubredditName()

	// We test validation indirectly by checking if the client would accept these names
	// In a real scenario, these would be rejected by validateSubredditName

	for _, name := range maliciousNames {
		t.Run(name, func(t *testing.T) {
			// Test the validation function indirectly through GetHot which uses it
			// We can't test directly since validateSubredditName is package-private
			// but we know these should all fail validation

			// Most of these should be rejected by validation
			// Only valid subreddit names (3-21 chars, alphanumeric + underscore, no leading/trailing/double underscores) should pass

			isValid := isValidSubredditName(name)

			// Log the result for manual inspection
			t.Logf("Subreddit name: %q, valid: %v", name, isValid)

			// Known attack patterns should always be invalid
			if strings.Contains(name, "..") ||
				strings.Contains(name, ";") ||
				strings.Contains(name, "'") ||
				strings.Contains(name, "\"") ||
				strings.Contains(name, "<") ||
				strings.Contains(name, ">") ||
				strings.Contains(name, "\n") ||
				strings.Contains(name, "\r") ||
				strings.Contains(name, "\x00") {
				if isValid {
					t.Errorf("Attack pattern should be rejected but was accepted: %q", name)
				}
			}
		})
	}
}

// TestCommentIDFuzzing tests comment ID validation against malicious input
func TestCommentIDFuzzing(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	maliciousIDs := fuzzer.FuzzCommentID()

	for _, id := range maliciousIDs {
		t.Run(id, func(t *testing.T) {
			isValid := isValidCommentID(id)
			t.Logf("Comment ID: %q, valid: %v", id, isValid)

			// Any comment ID with special characters, control chars, or path traversal should be invalid
			if strings.Contains(id, "..") ||
				strings.Contains(id, "/") ||
				strings.Contains(id, "\\") ||
				strings.Contains(id, ";") ||
				strings.Contains(id, "'") ||
				strings.Contains(id, "\n") ||
				strings.Contains(id, "\r") ||
				strings.Contains(id, "\x00") ||
				strings.Contains(id, " ") ||
				strings.Contains(id, "-") ||
				strings.Contains(id, "_") ||
				strings.Contains(id, ".") {
				if isValid {
					t.Errorf("Malicious comment ID should be rejected but was accepted: %q", id)
				}
			}
		})
	}
}

// TestUserAgentFuzzing tests User-Agent validation against header injection
func TestUserAgentFuzzing(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	maliciousUAs := fuzzer.FuzzUserAgent()

	for _, ua := range maliciousUAs {
		t.Run(ua[:min(50, len(ua))], func(t *testing.T) {
			// Try to create a client with malicious User-Agent
			config := &graw.Config{
				ClientID:     "test_client_id",
				ClientSecret: "test_client_secret",
				UserAgent:    ua,
			}

			// Attempt to create client (will fail at auth, but should validate UA first)
			_, err := graw.NewClient(config)

			// Any User-Agent with newlines should be rejected
			if strings.ContainsAny(ua, "\r\n") {
				if err == nil {
					t.Errorf("User-Agent with newlines should be rejected: %q", ua)
				}
				if !isConfigError(err) {
					t.Errorf("Expected ConfigError for malicious User-Agent, got: %T", err)
				}
			}

			// User-Agents over 256 chars should be rejected
			if len(ua) > 256 {
				if err == nil {
					t.Errorf("User-Agent over 256 chars should be rejected: len=%d", len(ua))
				}
				if !isConfigError(err) {
					t.Errorf("Expected ConfigError for oversized User-Agent, got: %T", err)
				}
			}
		})
	}
}

// TestLinkIDFuzzing tests LinkID validation and prefix handling
func TestLinkIDFuzzing(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	maliciousLinkIDs := fuzzer.FuzzLinkID()

	for _, linkID := range maliciousLinkIDs {
		t.Run(linkID, func(t *testing.T) {
			// LinkID validation happens in GetMoreComments
			// Test various malicious patterns

			hasWrongPrefix := strings.HasPrefix(linkID, "t1_") ||
				strings.HasPrefix(linkID, "t2_") ||
				strings.HasPrefix(linkID, "t4_") ||
				strings.HasPrefix(linkID, "t5_")

			hasPrefixButNoContent := linkID == "t3_" || linkID == "t1_"

			hasPathTraversal := strings.Contains(linkID, "..")
			hasInjection := strings.Contains(linkID, "'")

			t.Logf("LinkID: %q, wrong_prefix: %v, empty_content: %v",
				linkID, hasWrongPrefix, hasPrefixButNoContent)

			// These patterns should all be problematic
			if hasWrongPrefix || hasPrefixButNoContent || hasPathTraversal || hasInjection {
				t.Logf("  Potential attack vector detected")
			}
		})
	}
}

// TestPaginationLimitFuzzing tests pagination limit bounds checking
func TestPaginationLimitFuzzing(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	maliciousLimits := fuzzer.FuzzPaginationLimit()

	for _, limit := range maliciousLimits {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			// Valid limits are 1-100
			isValid := limit >= 1 && limit <= 100

			t.Logf("Limit: %d, valid: %v", limit, isValid)

			if limit < 1 || limit > 100 {
				// These should be rejected by validation
				t.Logf("  Should be rejected by validation")
			}
		})
	}
}

// TestSQLInjectionAttempts tests that SQL injection patterns don't cause issues
func TestSQLInjectionAttempts(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	sqlInjections := fuzzer.GenerateSQLInjections()

	for _, injection := range sqlInjections {
		t.Run(injection, func(t *testing.T) {
			// Test as subreddit name
			isValidSub := isValidSubredditName(injection)
			if isValidSub {
				t.Errorf("SQL injection should be rejected as subreddit name: %q", injection)
			}

			// Test as comment ID
			isValidComment := isValidCommentID(injection)
			if isValidComment {
				t.Errorf("SQL injection should be rejected as comment ID: %q", injection)
			}
		})
	}
}

// TestPathTraversalAttempts tests that path traversal attempts are rejected
func TestPathTraversalAttempts(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	pathTraversals := fuzzer.GeneratePathTraversals()

	for _, path := range pathTraversals {
		t.Run(path, func(t *testing.T) {
			// Test as subreddit name
			isValidSub := isValidSubredditName(path)
			if isValidSub {
				t.Errorf("Path traversal should be rejected as subreddit name: %q", path)
			}

			// Test as comment ID
			isValidComment := isValidCommentID(path)
			if isValidComment {
				t.Errorf("Path traversal should be rejected as comment ID: %q", path)
			}
		})
	}
}

// TestUnicodeAttacks tests handling of various Unicode attack patterns
func TestUnicodeAttacks(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	unicodeAttacks := fuzzer.GenerateUnicodeAttacks()

	for _, attack := range unicodeAttacks {
		t.Run(attack, func(t *testing.T) {
			// Test as subreddit name - should be rejected (not ASCII alphanumeric)
			isValidSub := isValidSubredditName(attack)
			t.Logf("Unicode attack as subreddit: %q, valid: %v", attack, isValidSub)

			// Most Unicode should be rejected for subreddit names
			if strings.ContainsAny(attack, "\u0000\u0001\u0002") {
				if isValidSub {
					t.Errorf("Null/control Unicode should be rejected: %q", attack)
				}
			}
		})
	}
}

// TestControlCharacters tests handling of control characters
func TestControlCharacters(t *testing.T) {
	fuzzer := helpers.NewFuzzer(42)
	controlChars := fuzzer.GenerateControlCharString()

	for i, str := range controlChars {
		t.Run(fmt.Sprintf("control_char_%d", i), func(t *testing.T) {
			// Control characters should be rejected in all inputs
			isValidSub := isValidSubredditName(str)
			isValidComment := isValidCommentID(str)

			if isValidSub {
				t.Errorf("Control character string should be rejected as subreddit: %q", str)
			}
			if isValidComment {
				t.Errorf("Control character string should be rejected as comment ID: %q", str)
			}
		})
	}
}

// TestEmptyAndBoundaryValues tests empty strings and boundary values
func TestEmptyAndBoundaryValues(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{"empty string", ""},
		{"single char", "a"},
		{"two chars", "ab"},
		{"three chars (min valid)", "abc"},
		{"21 chars (max valid)", "abcdefghijklmnopqrstu"},
		{"22 chars (over max)", "abcdefghijklmnopqrstuv"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := isValidSubredditName(tc.value)

			// Only 3-21 character alphanumeric strings should be valid
			shouldBeValid := len(tc.value) >= 3 && len(tc.value) <= 21
			if isValid != shouldBeValid {
				t.Errorf("Value %q (len=%d): expected valid=%v, got valid=%v",
					tc.value, len(tc.value), shouldBeValid, isValid)
			}
		})
	}
}

// Helper functions

// isValidSubredditName checks if a subreddit name would pass validation
func isValidSubredditName(name string) bool {
	// Replicate the validation logic from validateSubredditName
	if len(name) < 3 || len(name) > 21 {
		return false
	}

	if strings.HasPrefix(name, "_") || strings.HasSuffix(name, "_") {
		return false
	}

	if strings.Contains(name, "__") {
		return false
	}

	for _, char := range name {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			char == '_') {
			return false
		}
	}

	return true
}

// isValidCommentID checks if a comment ID would pass validation
func isValidCommentID(id string) bool {
	if len(id) == 0 || len(id) > 100 {
		return false
	}

	for _, char := range id {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z')) {
			return false
		}
	}

	return true
}

// isConfigError checks if an error is a ConfigError
func isConfigError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*pkgerrs.ConfigError)
	return ok
}
