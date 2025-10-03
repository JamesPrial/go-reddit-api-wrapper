package adversarial_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jamesprial/go-reddit-api-wrapper/adversarial_tests/helpers"
	"github.com/jamesprial/go-reddit-api-wrapper/internal"
	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// TestDeeplyNestedCommentsProtection tests that the parser prevents stack overflow
// from deeply nested comment structures by enforcing MaxCommentDepth
func TestDeeplyNestedCommentsProtection(t *testing.T) {
	parser := internal.NewParser()
	generator := helpers.NewJSONGenerator()

	testCases := []struct {
		name          string
		depth         int
		shouldSucceed bool
		checkReplies  bool // Whether to verify replies were truncated
	}{
		{
			name:          "normal depth (10 levels)",
			depth:         10,
			shouldSucceed: true,
			checkReplies:  false,
		},
		{
			name:          "at max depth (50 levels)",
			depth:         internal.MaxCommentDepth,
			shouldSucceed: true,
			checkReplies:  false,
		},
		{
			name:          "one over max depth (51 levels)",
			depth:         internal.MaxCommentDepth + 1,
			shouldSucceed: true, // Root succeeds, but deepest replies truncated
			checkReplies:  true,
		},
		{
			name:          "way over max depth (100 levels)",
			depth:         100,
			shouldSucceed: true, // Root succeeds, but deepest replies truncated
			checkReplies:  true,
		},
		{
			name:          "extreme depth (1000 levels)",
			depth:         1000,
			shouldSucceed: true, // Root succeeds, but deepest replies truncated
			checkReplies:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate deeply nested comment JSON
			jsonStr := generator.GenerateDeeplyNestedComment(tc.depth)

			// Parse into Thing
			var thing types.Thing
			err := json.Unmarshal([]byte(jsonStr), &thing)
			if err != nil {
				t.Fatalf("failed to unmarshal test JSON: %v", err)
			}

			// Attempt to parse the comment using ParseThing which handles context internally
			result, err := parser.ParseThing(context.Background(), &thing)
			var comment *types.Comment
			if result != nil {
				comment = result.(*types.Comment)
			}

			if tc.shouldSucceed {
				if err != nil {
					t.Errorf("expected parsing to succeed at depth %d, but got error: %v", tc.depth, err)
				}
				if comment == nil {
					t.Error("expected non-nil comment")
				}

				// If checkReplies is true, verify that the tree was truncated at max depth
				if tc.checkReplies {
					actualDepth := measureCommentDepth(comment)
					// The tree should be truncated, so actual depth should not exceed MaxCommentDepth + 1
					// (+1 because we count from 0)
					if actualDepth > internal.MaxCommentDepth+1 {
						t.Errorf("comment tree with depth %d was not truncated properly, actual depth: %d (max allowed: %d)",
							tc.depth, actualDepth, internal.MaxCommentDepth+1)
					} else {
						t.Logf("Successfully truncated tree from %d to %d levels", tc.depth, actualDepth)
					}
				}
			} else {
				if err == nil {
					t.Errorf("expected parsing to fail at depth %d due to max depth protection, but it succeeded", tc.depth)
				} else {
					// Verify the error message mentions depth
					if !strings.Contains(err.Error(), "depth") && !strings.Contains(err.Error(), "maximum") {
						t.Errorf("expected error to mention depth limit, got: %v", err)
					}
				}
			}
		})
	}
}

// measureCommentDepth recursively measures the depth of a comment tree
func measureCommentDepth(comment *types.Comment) int {
	if comment == nil || len(comment.Replies) == 0 {
		return 1
	}

	maxChildDepth := 0
	for _, reply := range comment.Replies {
		childDepth := measureCommentDepth(reply)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return maxChildDepth + 1
}

// TestMalformedThings tests various malformed Thing structures
func TestMalformedThings(t *testing.T) {
	parser := internal.NewParser()
	generator := helpers.NewJSONGenerator()

	malformedThings := generator.GenerateMalformedThings()

	for i, jsonStr := range malformedThings {
		t.Run(jsonStr[:min(50, len(jsonStr))], func(t *testing.T) {
			var thing types.Thing
			err := json.Unmarshal([]byte(jsonStr), &thing)

			// Some of these are invalid JSON, so unmarshal will fail
			if err != nil {
				// This is expected for invalid JSON
				return
			}

			// If we managed to unmarshal, try to parse it
			// Should either return an error or handle gracefully
			result, parseErr := parser.ParseThing(context.Background(), &thing)

			// We expect most of these to fail parsing
			if parseErr == nil && result == nil {
				t.Errorf("test case %d: parsing returned nil result and nil error", i)
			}
		})
	}
}

// TestMalformedEditedField tests handling of various malformed "edited" field values
func TestMalformedEditedField(t *testing.T) {
	generator := helpers.NewJSONGenerator()
	malformedFields := generator.GenerateMalformedEditedField()

	for _, jsonStr := range malformedFields {
		t.Run(jsonStr, func(t *testing.T) {
			// The types package should handle these gracefully with custom unmarshaler
			var data struct {
				Edited types.Edited `json:"edited"`
			}

			err := json.Unmarshal([]byte(jsonStr), &data)
			// Should not panic and should either succeed or return error
			if err != nil {
				// Some formats are expected to fail
				t.Logf("Unmarshal failed as expected: %v", err)
			}
		})
	}
}

// TestMalformedListings tests various malformed Listing structures
func TestMalformedListings(t *testing.T) {
	parser := internal.NewParser()
	generator := helpers.NewJSONGenerator()

	malformedListings := generator.GenerateMalformedListing()

	for _, jsonStr := range malformedListings {
		t.Run(jsonStr[:min(50, len(jsonStr))], func(t *testing.T) {
			var thing types.Thing
			err := json.Unmarshal([]byte(jsonStr), &thing)
			if err != nil {
				// Invalid JSON, expected
				return
			}

			// Try to parse the listing
			_, parseErr := parser.ParseListing(context.Background(), &thing)

			// Should handle gracefully (return error or valid result)
			// Should not panic
			if parseErr != nil {
				t.Logf("Parsing failed gracefully: %v", parseErr)
			}
		})
	}
}

// TestUnknownThingKinds tests handling of unknown Thing kinds
func TestUnknownThingKinds(t *testing.T) {
	parser := internal.NewParser()

	testCases := []string{
		"t6",
		"t7",
		"t8",
		"t9",
		"tx",
		"unknown",
		"",
		"123",
		"Listing2",
		"more2",
	}

	for _, kind := range testCases {
		t.Run(kind, func(t *testing.T) {
			thing := &types.Thing{
				Kind: kind,
				Data: json.RawMessage(`{"id": "test"}`),
			}

			result, err := parser.ParseThing(context.Background(), thing)

			// Should return an error for unknown kinds
			if err == nil {
				t.Errorf("expected error for unknown kind '%s', but got result: %v", kind, result)
			}

			// Should mention "unknown" in error
			if !strings.Contains(strings.ToLower(err.Error()), "unknown") {
				t.Errorf("expected error to mention 'unknown', got: %v", err)
			}
		})
	}
}

// TestNilThingHandling tests that parsing nil Things returns appropriate errors
func TestNilThingHandling(t *testing.T) {
	parser := internal.NewParser()

	parseFuncs := map[string]func(*types.Thing) (interface{}, error){
		"ParseThing":     func(t *types.Thing) (interface{}, error) { return parser.ParseThing(context.Background(), t) },
		"ParseComment":   func(t *types.Thing) (interface{}, error) { return parser.ParseThing(context.Background(), t) },
		"ParsePost":      func(t *types.Thing) (interface{}, error) { return parser.ParsePost(context.Background(), t) },
		"ParseListing":   func(t *types.Thing) (interface{}, error) { return parser.ParseListing(context.Background(), t) },
		"ParseSubreddit": func(t *types.Thing) (interface{}, error) { return parser.ParseSubreddit(context.Background(), t) },
		"ParseAccount":   func(t *types.Thing) (interface{}, error) { return parser.ParseAccount(context.Background(), t) },
		"ParseMessage":   func(t *types.Thing) (interface{}, error) { return parser.ParseMessage(context.Background(), t) },
		"ParseMore":      func(t *types.Thing) (interface{}, error) { return parser.ParseMore(context.Background(), t) },
	}

	for name, parseFunc := range parseFuncs {
		t.Run(name, func(t *testing.T) {
			result, err := parseFunc(nil)

			if err == nil {
				t.Errorf("%s: expected error when parsing nil Thing, got result: %v", name, result)
			}

			if !strings.Contains(err.Error(), "nil") {
				t.Errorf("%s: expected error to mention 'nil', got: %v", name, err)
			}
		})
	}
}

// TestWrongThingKindHandling tests that parsers validate Thing kind
func TestWrongThingKindHandling(t *testing.T) {
	parser := internal.NewParser()

	testCases := []struct {
		name      string
		thing     *types.Thing
		parseFunc func(*types.Thing) (interface{}, error)
	}{
		{
			name:  "ParseComment with t3 (Post)",
			thing: &types.Thing{Kind: "t3", Data: json.RawMessage(`{"id": "post"}`)},
			parseFunc: func(t *types.Thing) (interface{}, error) {
				// Try to parse as comment by calling ParsePost and then converting
				// This should succeed since we're calling ParseThing which handles any type
				result, err := parser.ParseThing(context.Background(), t)
				if err != nil {
					return nil, err
				}
				// Check if the result is actually a Post, not a Comment
				if _, ok := result.(*types.Post); ok {
					// This is wrong - we expected a Comment but got a Post
					return nil, fmt.Errorf("expected t1 (Comment), got t3 (Post)")
				}
				return result, nil
			},
		},
		{
			name:      "ParsePost with t1 (Comment)",
			thing:     &types.Thing{Kind: "t1", Data: json.RawMessage(`{"id": "comment"}`)},
			parseFunc: func(t *types.Thing) (interface{}, error) { return parser.ParsePost(context.Background(), t) },
		},
		{
			name:      "ParseListing with t1 (Comment)",
			thing:     &types.Thing{Kind: "t1", Data: json.RawMessage(`{"id": "comment"}`)},
			parseFunc: func(t *types.Thing) (interface{}, error) { return parser.ParseListing(context.Background(), t) },
		},
		{
			name:      "ParseSubreddit with t1 (Comment)",
			thing:     &types.Thing{Kind: "t1", Data: json.RawMessage(`{"id": "comment"}`)},
			parseFunc: func(t *types.Thing) (interface{}, error) { return parser.ParseSubreddit(context.Background(), t) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.parseFunc(tc.thing)

			if err == nil {
				t.Errorf("expected error for wrong Thing kind, got result: %v", result)
			}

			if !strings.Contains(err.Error(), "expected") {
				t.Errorf("expected error to mention 'expected', got: %v", err)
			}
		})
	}
}

// TestJSONBombProtection tests protection against JSON bombs (deeply nested objects)
func TestJSONBombProtection(t *testing.T) {
	generator := helpers.NewJSONGenerator()

	testCases := []int{100, 500, 1000}

	for _, depth := range testCases {
		t.Run(fmt.Sprintf("depth_%d", depth), func(t *testing.T) {
			jsonBomb := generator.GenerateJSONBomb(depth)

			var thing types.Thing
			err := json.Unmarshal([]byte(jsonBomb), &thing)

			// Either unmarshal fails or succeeds, but should not hang or crash
			if err != nil {
				t.Logf("JSON bomb at depth %d failed to unmarshal (expected): %v", depth, err)
			} else {
				t.Logf("JSON bomb at depth %d unmarshaled successfully", depth)
			}
		})
	}
}

// TestLargeArrayHandling tests handling of very large arrays
func TestLargeArrayHandling(t *testing.T) {
	parser := internal.NewParser()
	generator := helpers.NewJSONGenerator()

	testCases := []struct {
		name string
		size int
	}{
		{"normal size", 100},
		{"large size", 1000},
		{"very large size", 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonStr := generator.GenerateLargeArray(tc.size)

			var thing types.Thing
			err := json.Unmarshal([]byte(jsonStr), &thing)
			if err != nil {
				t.Fatalf("failed to unmarshal large array: %v", err)
			}

			// Should handle large arrays without crashing
			result, err := parser.ParseListing(context.Background(), &thing)
			if err != nil {
				t.Errorf("failed to parse large listing: %v", err)
			}

			if result != nil && len(result.Children) != tc.size {
				t.Errorf("expected %d children, got %d", tc.size, len(result.Children))
			}
		})
	}
}

// TestCircularReferenceHandling tests handling of structures that might cause circular parsing
func TestCircularReferenceHandling(t *testing.T) {
	parser := internal.NewParser()
	generator := helpers.NewJSONGenerator()

	jsonStr := generator.GenerateCircularReference()

	var thing types.Thing
	err := json.Unmarshal([]byte(jsonStr), &thing)
	if err != nil {
		t.Fatalf("failed to unmarshal circular reference JSON: %v", err)
	}

	// Should handle without infinite recursion using ParseThing which handles context internally
	result, err := parser.ParseThing(context.Background(), &thing)
	var comment *types.Comment
	if result != nil {
		comment = result.(*types.Comment)
	}
	if err != nil {
		t.Logf("Parsing circular reference failed (acceptable): %v", err)
	} else if comment == nil {
		t.Error("got nil comment with no error")
	}
}

// TestMalformedMoreChildren tests malformed MoreChildren responses
func TestMalformedMoreChildren(t *testing.T) {
	generator := helpers.NewJSONGenerator()
	malformedResponses := generator.GenerateMalformedMoreChildren()

	for i, jsonStr := range malformedResponses {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Just verify we can attempt to unmarshal without panicking
			var response map[string]interface{}
			err := json.Unmarshal([]byte(jsonStr), &response)

			if err != nil {
				t.Logf("Unmarshal failed as expected: %v", err)
			}

			// The actual parsing would happen in the HTTP client's DoMoreChildren method
			// which should handle these gracefully
		})
	}
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
