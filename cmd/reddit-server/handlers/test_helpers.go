package handlers

import (
	"encoding/json"
)

// parseJSON is a test helper to parse JSON bytes into a struct.
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
