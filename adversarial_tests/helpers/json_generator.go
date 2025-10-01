package helpers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONGenerator creates malicious and malformed JSON for testing
type JSONGenerator struct{}

// NewJSONGenerator creates a new JSON generator
func NewJSONGenerator() *JSONGenerator {
	return &JSONGenerator{}
}

// GenerateDeeplyNestedComment creates a JSON comment with deeply nested replies
func (g *JSONGenerator) GenerateDeeplyNestedComment(depth int) string {
	var build func(currentDepth int) string
	build = func(currentDepth int) string {
		if currentDepth >= depth {
			return `{
				"kind": "t1",
				"data": {
					"id": "comment` + fmt.Sprintf("%d", currentDepth) + `",
					"body": "This is comment at depth ` + fmt.Sprintf("%d", currentDepth) + `",
					"author": "testuser",
					"replies": ""
				}
			}`
		}

		return `{
			"kind": "t1",
			"data": {
				"id": "comment` + fmt.Sprintf("%d", currentDepth) + `",
				"body": "This is comment at depth ` + fmt.Sprintf("%d", currentDepth) + `",
				"author": "testuser",
				"replies": {
					"kind": "Listing",
					"data": {
						"children": [` + build(currentDepth+1) + `]
					}
				}
			}
		}`
	}

	return build(0)
}

// GenerateMalformedThings creates various malformed Thing objects
func (g *JSONGenerator) GenerateMalformedThings() []string {
	return []string{
		// Missing kind
		`{"data": {"id": "test123"}}`,

		// Missing data
		`{"kind": "t1"}`,

		// Null data
		`{"kind": "t1", "data": null}`,

		// Data as array instead of object
		`{"kind": "t1", "data": ["test"]}`,

		// Data as string
		`{"kind": "t1", "data": "invalid"}`,

		// Data as number
		`{"kind": "t1", "data": 12345}`,

		// Unknown kind
		`{"kind": "t9", "data": {"id": "test"}}`,
		`{"kind": "tx", "data": {"id": "test"}}`,
		`{"kind": "unknown", "data": {"id": "test"}}`,

		// Empty kind
		`{"kind": "", "data": {"id": "test"}}`,

		// Null kind
		`{"kind": null, "data": {"id": "test"}}`,

		// Kind as number
		`{"kind": 123, "data": {"id": "test"}}`,

		// Completely empty
		`{}`,

		// Just opening brace
		`{`,

		// Unclosed object
		`{"kind": "t1", "data": {"id": "test"`,

		// Invalid JSON
		`{"kind": "t1" "data": {"id": "test"}}`,

		// Trailing comma
		`{"kind": "t1", "data": {"id": "test"},}`,

		// Double commas
		`{"kind": "t1",, "data": {"id": "test"}}`,

		// Missing quotes
		`{kind: "t1", data: {id: "test"}}`,

		// Single quotes (invalid JSON)
		`{'kind': 't1', 'data': {'id': 'test'}}`,
	}
}

// GenerateMalformedEditedField creates various malformed values for the edited field
func (g *JSONGenerator) GenerateMalformedEditedField() []string {
	return []string{
		`{"edited": []}`,
		`{"edited": {}}`,
		`{"edited": "true"}`,
		`{"edited": "false"}`,
		`{"edited": "not_a_number"}`,
		`{"edited": -1}`,
		`{"edited": 999999999999999}`, // Far future timestamp
		`{"edited": null}`,
		`{"edited": [1234567890]}`,
		`{"edited": {"timestamp": 1234567890}}`,
	}
}

// GenerateMalformedListing creates various malformed Listing objects
func (g *JSONGenerator) GenerateMalformedListing() []string {
	return []string{
		// Empty children array
		`{
			"kind": "Listing",
			"data": {
				"children": []
			}
		}`,

		// Null children
		`{
			"kind": "Listing",
			"data": {
				"children": null
			}
		}`,

		// Children as object instead of array
		`{
			"kind": "Listing",
			"data": {
				"children": {"test": "invalid"}
			}
		}`,

		// Children as string
		`{
			"kind": "Listing",
			"data": {
				"children": "invalid"
			}
		}`,

		// Missing children field
		`{
			"kind": "Listing",
			"data": {}
		}`,

		// Mixed types in children
		`{
			"kind": "Listing",
			"data": {
				"children": [
					{"kind": "t1", "data": {"id": "comment"}},
					"invalid",
					123,
					null,
					{"kind": "t3", "data": {"id": "post"}}
				]
			}
		}`,

		// Invalid pagination values
		`{
			"kind": "Listing",
			"data": {
				"children": [],
				"after": 12345,
				"before": true
			}
		}`,
	}
}

// GenerateTokenResponse creates various malformed token responses
func (g *JSONGenerator) GenerateTokenResponse() map[string]string {
	return map[string]string{
		"empty_access_token": `{
			"access_token": "",
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"missing_access_token": `{
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"null_access_token": `{
			"access_token": null,
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"access_token_as_number": `{
			"access_token": 12345,
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"access_token_as_array": `{
			"access_token": ["token"],
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"negative_expires_in": `{
			"access_token": "valid_token",
			"token_type": "bearer",
			"expires_in": -3600
		}`,

		"zero_expires_in": `{
			"access_token": "valid_token",
			"token_type": "bearer",
			"expires_in": 0
		}`,

		"huge_expires_in": `{
			"access_token": "valid_token",
			"token_type": "bearer",
			"expires_in": 999999999999
		}`,

		"expires_in_as_string": `{
			"access_token": "valid_token",
			"token_type": "bearer",
			"expires_in": "3600"
		}`,

		"expires_in_as_float": `{
			"access_token": "valid_token",
			"token_type": "bearer",
			"expires_in": 3600.5
		}`,

		"missing_expires_in": `{
			"access_token": "valid_token",
			"token_type": "bearer"
		}`,

		"null_expires_in": `{
			"access_token": "valid_token",
			"token_type": "bearer",
			"expires_in": null
		}`,

		"invalid_json": `{
			"access_token": "valid_token",
			"token_type": "bearer"
			"expires_in": 3600
		}`,

		"empty_object": `{}`,

		"null": `null`,

		"array": `[]`,

		"string": `"not an object"`,

		"very_long_token": `{
			"access_token": "` + strings.Repeat("A", 100000) + `",
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"token_with_nullbytes": `{
			"access_token": "valid\u0000token",
			"token_type": "bearer",
			"expires_in": 3600
		}`,

		"token_with_newlines": `{
			"access_token": "valid\ntoken\r\nhere",
			"token_type": "bearer",
			"expires_in": 3600
		}`,
	}
}

// GenerateRateLimitHeaders creates various malformed rate limit header combinations
func (g *JSONGenerator) GenerateRateLimitHeaders() map[string]map[string]string {
	return map[string]map[string]string{
		"negative_remaining": {
			"X-Ratelimit-Remaining": "-1",
			"X-Ratelimit-Reset":     "1234567890",
		},
		"negative_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "-1",
		},
		"zero_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "0",
		},
		"huge_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "9999999999",
		},
		"invalid_float_remaining": {
			"X-Ratelimit-Remaining": "not_a_number",
			"X-Ratelimit-Reset":     "1234567890",
		},
		"invalid_int_reset": {
			"X-Ratelimit-Remaining": "50",
			"X-Ratelimit-Reset":     "not_a_number",
		},
		"nan_remaining": {
			"X-Ratelimit-Remaining": "NaN",
			"X-Ratelimit-Reset":     "1234567890",
		},
		"inf_remaining": {
			"X-Ratelimit-Remaining": "Inf",
			"X-Ratelimit-Reset":     "1234567890",
		},
		"empty_headers": {},
		"only_remaining": {
			"X-Ratelimit-Remaining": "50",
		},
		"only_reset": {
			"X-Ratelimit-Reset": "1234567890",
		},
		"huge_retry_after": {
			"Retry-After": "999999999",
		},
		"negative_retry_after": {
			"Retry-After": "-1",
		},
	}
}

// GenerateJSONBomb creates a "JSON bomb" - deeply nested objects designed to exhaust parsers
func (g *JSONGenerator) GenerateJSONBomb(depth int) string {
	opening := strings.Repeat(`{"a":`, depth)
	closing := strings.Repeat(`}`, depth)
	return opening + `"value"` + closing
}

// GenerateLargeArray creates a JSON array with many elements
func (g *JSONGenerator) GenerateLargeArray(size int) string {
	elements := make([]string, size)
	for i := 0; i < size; i++ {
		elements[i] = fmt.Sprintf(`{"kind":"t1","data":{"id":"comment%d"}}`, i)
	}
	return `{"kind":"Listing","data":{"children":[` + strings.Join(elements, ",") + `]}}`
}

// GenerateCircularReference attempts to create a structure that might cause circular parsing
// Note: JSON doesn't support true circular references, but we can simulate problematic structures
func (g *JSONGenerator) GenerateCircularReference() string {
	return `{
		"kind": "t1",
		"data": {
			"id": "parent",
			"parent_id": "t1_child",
			"replies": {
				"kind": "Listing",
				"data": {
					"children": [{
						"kind": "t1",
						"data": {
							"id": "child",
							"parent_id": "t1_parent"
						}
					}]
				}
			}
		}
	}`
}

// GenerateMalformedMoreChildren creates malformed MoreChildren response
func (g *JSONGenerator) GenerateMalformedMoreChildren() []string {
	return []string{
		// Missing json.data.things
		`{
			"json": {
				"errors": [],
				"data": {}
			}
		}`,

		// things as null
		`{
			"json": {
				"errors": [],
				"data": {
					"things": null
				}
			}
		}`,

		// things as string
		`{
			"json": {
				"errors": [],
				"data": {
					"things": "invalid"
				}
			}
		}`,

		// errors as object instead of array
		`{
			"json": {
				"errors": {},
				"data": {
					"things": []
				}
			}
		}`,

		// Missing json field
		`{
			"errors": [],
			"data": {
				"things": []
			}
		}`,

		// Null json
		`{
			"json": null
		}`,

		// Array instead of object
		`[]`,

		// Completely invalid structure
		`{
			"random": "data",
			"that": "doesn't",
			"match": "expected structure"
		}`,
	}
}

// PrettyPrint formats JSON for debugging
func (g *JSONGenerator) PrettyPrint(jsonStr string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}
	return string(pretty), nil
}

// GenerateMalformedTokenResponses creates various malformed OAuth token responses
func (g *JSONGenerator) GenerateMalformedTokenResponses() []string {
	return []string{
		// Missing required fields
		`{}`,
		`{"access_token": ""}`,
		`{"expires_in": 3600}`,
		`{"token_type": "bearer"}`,

		// Invalid expiry values
		`{"access_token": "valid_token", "expires_in": -1}`,
		`{"access_token": "valid_token", "expires_in": 0}`,
		`{"access_token": "valid_token", "expires_in": -999999}`,
		`{"access_token": "valid_token", "expires_in": 999999999999}`, // Over 1 year

		// Invalid types
		`{"access_token": "valid_token", "expires_in": "not_a_number"}`,
		`{"access_token": "valid_token", "expires_in": null}`,
		`{"access_token": "valid_token", "expires_in": []}`,
		`{"access_token": "valid_token", "expires_in": {}}`,
		`{"access_token": 123, "expires_in": 3600}`,
		`{"access_token": null, "expires_in": 3600}`,
		`{"access_token": [], "expires_in": 3600}`,

		// Malformed JSON
		`{"access_token": "valid_token", "expires_in": 3600`,
		`{access_token: "valid_token", "expires_in": 3600}`,
		`{"access_token": "valid_token", "expires_in": 3600,}`,

		// Very large token values
		`{"access_token": "` + strings.Repeat("A", 10000) + `", "expires_in": 3600}`,

		// Special characters in token
		`{"access_token": "token\nwith\nnewlines", "expires_in": 3600}`,
		`{"access_token": "token\x00with\x00nulls", "expires_in": 3600}`,
		`{"access_token": "token'with'quotes", "expires_in": 3600}`,

		// Error responses
		`{"error": "invalid_grant"}`,
		`{"error": "invalid_client", "error_description": "Client authentication failed"}`,

		// Unexpected extra fields
		`{"access_token": "valid_token", "expires_in": 3600, "extra_field": "unexpected"}`,
	}
}

// GenerateOversizedTokenResponse creates an extremely large token response (15MB+)
func (g *JSONGenerator) GenerateOversizedTokenResponse() string {
	// Create a 15MB token
	largeToken := strings.Repeat("A", 15*1024*1024)
	return fmt.Sprintf(`{"access_token": "%s", "expires_in": 3600}`, largeToken)
}

// GenerateTokenResponseWithInvalidExpiry creates token responses with specific invalid expiry values
func (g *JSONGenerator) GenerateTokenResponseWithInvalidExpiry() []struct {
	Name     string
	Response string
} {
	return []struct {
		Name     string
		Response string
	}{
		{"negative", `{"access_token": "valid_token", "expires_in": -1}`},
		{"zero", `{"access_token": "valid_token", "expires_in": 0}`},
		{"max_int", `{"access_token": "valid_token", "expires_in": 2147483647}`},
		{"over_one_year", `{"access_token": "valid_token", "expires_in": 31536001}`},
		{"huge_value", `{"access_token": "valid_token", "expires_in": 999999999999}`},
		{"string", `{"access_token": "valid_token", "expires_in": "3600"}`},
		{"float", `{"access_token": "valid_token", "expires_in": 3600.5}`},
		{"nan", `{"access_token": "valid_token", "expires_in": NaN}`},
		{"infinity", `{"access_token": "valid_token", "expires_in": Infinity}`},
	}
}

// GenerateRaceConditionTokens creates multiple token responses for race condition testing
func (g *JSONGenerator) GenerateRaceConditionTokens(count int) []string {
	tokens := make([]string, count)
	for i := 0; i < count; i++ {
		// Each token has a unique value and expiry
		token := fmt.Sprintf("token_%d", i)
		expiry := 3600 + i
		tokens[i] = fmt.Sprintf(`{"access_token": "%s", "expires_in": %d}`, token, expiry)
	}
	return tokens
}
