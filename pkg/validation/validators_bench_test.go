package validation

import (
	"strings"
	"testing"
)

// Benchmarks for security-critical validators to ensure they perform well
// even with malicious inputs designed to cause DoS.

func BenchmarkIsValidBase36(b *testing.B) {
	tests := []struct {
		name  string
		input string
	}{
		{"short valid", "abc123"},
		{"medium valid", strings.Repeat("a", 50)},
		{"max length valid", strings.Repeat("a", 100)},
		{"short invalid", "ABC123"},
		{"exceeds max length", strings.Repeat("a", 200)},
		{"malicious very long", strings.Repeat("X", 10000)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = IsValidBase36(tt.input)
			}
		})
	}
}

func BenchmarkIsValidFullname(b *testing.B) {
	tests := []struct {
		name  string
		input string
	}{
		{"valid short", "t3_abc123"},
		{"valid medium", "t3_" + strings.Repeat("a", 50)},
		{"valid max length", "t3_" + strings.Repeat("a", 107)},
		{"exceeds max length", "t3_" + strings.Repeat("a", 200)},
		{"malicious very long", "t3_" + strings.Repeat("a", 10000)},
		{"wrong format", "invalid_fullname_" + strings.Repeat("x", 100)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = IsValidFullname(tt.input)
			}
		})
	}
}

func BenchmarkIsValidPermalink(b *testing.B) {
	tests := []struct {
		name  string
		input string
	}{
		{"valid short", "/r/golang/comments/abc123/test/"},
		{"valid with comment", "/r/golang/comments/abc123/test_post/def456/"},
		{"long title slug", "/r/golang/comments/abc123/" + strings.Repeat("a", 150) + "/"},
		{"exceeds max length", "/r/golang/comments/abc123/" + strings.Repeat("a", 600) + "/"},
		// ReDoS attack patterns - test backtracking behavior
		{"many slashes", strings.Repeat("/", 1000)},
		{"alternating pattern", "/r/" + strings.Repeat("a/", 500)},
		{"malicious nested", "/r/golang/comments/" + strings.Repeat("abc/", 100)},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = IsValidPermalink(tt.input)
			}
		})
	}
}

func BenchmarkIsValidSubreddit(b *testing.B) {
	tests := []struct {
		name  string
		input string
	}{
		{"valid short", "golang"},
		{"valid with underscore", "ask_reddit"},
		{"valid max length", strings.Repeat("a", 21)},
		{"invalid too long", strings.Repeat("a", 100)},
		{"invalid characters", "ask-reddit-test"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = IsValidSubreddit(tt.input)
			}
		})
	}
}

func BenchmarkIsValidUsername(b *testing.B) {
	tests := []struct {
		name  string
		input string
	}{
		{"valid short", "user123"},
		{"valid with hyphen", "test-user"},
		{"valid max length", strings.Repeat("a", 20)},
		{"invalid too long", strings.Repeat("a", 100)},
		{"invalid characters", "test@user"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = IsValidUsername(tt.input)
			}
		})
	}
}
