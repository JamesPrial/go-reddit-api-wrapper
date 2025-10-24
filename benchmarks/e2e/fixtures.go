package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const fixturesDir = "testdata"

// saveFixture saves the API response data to a JSON file for later inspection
// and validation. This is useful for capturing real Reddit API responses during
// benchmark runs, which can then be used for debugging, documentation, or
// creating mock data for unit tests.
//
// The data is marshaled to pretty-printed JSON and saved to:
// benchmarks/e2e/testdata/{name}.json
//
// If any error occurs during directory creation, marshaling, or file writing,
// the benchmark will fail immediately via b.Fatal().
func saveFixture(b *testing.B, name string, data interface{}) {
	b.Helper()

	// Create the testdata directory relative to the test execution directory
	dir := filepath.Join(fixturesDir)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		b.Fatalf("failed to create fixtures directory %s: %v", dir, err)
	}

	// Marshal data to pretty-printed JSON for human readability
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		b.Fatalf("failed to marshal fixture data for %s: %v", name, err)
	}

	// Write to file
	filename := filepath.Join(dir, name+".json")
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		b.Fatalf("failed to write fixture file %s: %v", filename, err)
	}

	b.Logf("saved fixture to %s (%d bytes)", filename, len(jsonData))
}
