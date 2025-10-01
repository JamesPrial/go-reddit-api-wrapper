package helpers

import (
	"math/rand"
	"strings"
	"unicode"
)

// Fuzzer provides utilities for generating adversarial input strings
type Fuzzer struct {
	rnd *rand.Rand
}

// NewFuzzer creates a new Fuzzer with the given seed
func NewFuzzer(seed int64) *Fuzzer {
	return &Fuzzer{
		rnd: rand.New(rand.NewSource(seed)),
	}
}

// FuzzSubredditName generates malicious subreddit name test cases
func (f *Fuzzer) FuzzSubredditName() []string {
	return []string{
		// Empty and boundary cases
		"",
		"a",
		"ab",
		"abc",                         // Minimum valid length
		"abcdefghijklmnopqrstu",       // Maximum valid length (21 chars)
		"abcdefghijklmnopqrstuv",      // One char too long
		"a_very_long_subreddit_name_that_exceeds_the_maximum_allowed_length",

		// SQL injection attempts
		"golang'; DROP TABLE--",
		"golang' OR '1'='1",
		"golang\"; DELETE FROM users--",
		"'; UNION SELECT * FROM posts--",

		// Path traversal
		"../../etc/passwd",
		"..\\..\\windows\\system32",
		"/etc/passwd",
		"../../../../root",

		// Unicode normalization attacks
		"golang\u0000admin",
		"test\u202Eadmin",  // Right-to-left override
		"café",              // Unicode characters
		"тест",              // Cyrillic
		"测试",               // Chinese
		"🚀rocket",           // Emoji

		// Special characters and control characters
		"test\nsubname",
		"test\rsubname",
		"test\tsubname",
		"test\x00subname",
		"test\x1Bsubname",

		// Underscore edge cases
		"_test",             // Leading underscore
		"test_",             // Trailing underscore
		"__test",            // Double leading underscore
		"test__",            // Double trailing underscore
		"test__sub",         // Double underscore in middle
		"___",               // All underscores

		// Mixed attacks
		"test<script>alert('xss')</script>",
		"test%00admin",
		"test\\/admin",
		"test/../admin",
		"test' UNION SELECT password FROM users--",

		// Case sensitivity
		"GOLANG",
		"GoLang",
		"gOlAnG",

		// Numbers and special patterns
		"123",
		"test123",
		"123test",
		"test-sub",          // Hyphen (might not be allowed)
		"test.sub",          // Dot (might not be allowed)
		"test+sub",          // Plus sign
		"test sub",          // Space
		"test@sub",          // At sign
		"test#sub",          // Hash

		// XML/HTML injection
		"test<>sub",
		"test&lt;sub",
		"test&gt;sub",

		// Null bytes in various positions
		"\x00test",
		"test\x00",
		"te\x00st",

		// Very long repeated patterns
		strings.Repeat("a", 100),
		strings.Repeat("_", 50),
		strings.Repeat("test", 50),
	}
}

// FuzzCommentID generates malicious comment ID test cases
func (f *Fuzzer) FuzzCommentID() []string {
	return []string{
		// Empty and boundary cases
		"",
		strings.Repeat("a", 100),  // Exactly at max length
		strings.Repeat("a", 101),  // One over max length
		strings.Repeat("a", 1000), // Far over max length

		// Special characters
		"abc123\n",
		"abc123\r",
		"abc123\t",
		"abc123\x00",
		"abc-123",
		"abc_123",
		"abc.123",
		"abc/123",
		"abc\\123",
		"abc 123",

		// SQL injection
		"abc'; DROP TABLE--",
		"abc' OR '1'='1",

		// Path traversal
		"../../secret",

		// Unicode
		"abc\u0000123",
		"тест123",
		"测试123",

		// Control characters
		"\x00abc123",
		"abc\x1B123",
		"abc\x7F123",

		// Mixed valid/invalid
		"abc123!@#",
		"abc123<>",
		"abc123[]",
		"abc123{}",

		// Only special characters
		"!@#$%^&*()",
		"<>?:\"{}|",
		"../../../",
	}
}

// FuzzUserAgent generates malicious User-Agent test cases
func (f *Fuzzer) FuzzUserAgent() []string {
	return []string{
		// Empty
		"",

		// Header injection via newlines
		"MyApp/1.0\nX-Evil-Header: injected",
		"MyApp/1.0\rX-Evil-Header: injected",
		"MyApp/1.0\r\nX-Evil-Header: injected",
		"MyApp/1.0\n\nInjected Body",

		// Extremely long
		strings.Repeat("a", 256),  // Exactly at limit
		strings.Repeat("a", 257),  // One over limit
		strings.Repeat("a", 10000), // Way over limit

		// Control characters
		"MyApp\x00/1.0",
		"MyApp\x1B/1.0",
		"MyApp\x7F/1.0",
		"\x00MyApp/1.0",

		// Multiple newlines
		"MyApp/1.0\n\n\nEvil",
		"MyApp/1.0\r\r\rEvil",
		"\n\n\nMyApp/1.0",

		// Mixed injection attempts
		"MyApp/1.0\r\nContent-Length: 0\r\n\r\nGET /evil HTTP/1.1",
		"MyApp/1.0\nSet-Cookie: session=stolen",

		// Unicode
		"MyApp\u0000/1.0",
		"MyApp\u202E/1.0",  // Right-to-left override
	}
}

// FuzzLinkID generates malicious LinkID test cases
func (f *Fuzzer) FuzzLinkID() []string {
	return []string{
		// Empty
		"",

		// Wrong prefixes
		"t1_abc123",  // Comment prefix
		"t2_abc123",  // Account prefix
		"t4_abc123",  // Message prefix
		"t5_abc123",  // Subreddit prefix

		// Multiple prefixes
		"t3_t3_abc123",
		"t1_t3_abc123",

		// Prefix only
		"t3_",
		"t1_",

		// No content after underscore
		"t3_\x00",

		// Special characters after prefix
		"t3_../../../etc/passwd",
		"t3_'; DROP TABLE--",

		// Random underscores
		"_abc123",
		"abc_123_",
		"_t3_abc123",

		// Unicode and control chars
		"t3_тест123",
		"t3_\x00abc123",
		"t3_abc\n123",
	}
}

// FuzzPaginationLimit generates adversarial pagination limit values
func (f *Fuzzer) FuzzPaginationLimit() []int {
	return []int{
		-1,
		-100,
		-2147483648, // int32 min
		0,
		101,         // One over max
		1000,
		2147483647,  // int32 max
	}
}

// GenerateRandomString generates a random string of the given length with specified character types
func (f *Fuzzer) GenerateRandomString(length int, includeSpecial bool) string {
	const (
		letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		special = "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	)

	charset := letters
	if includeSpecial {
		charset += special
	}

	result := make([]byte, length)
	for i := range result {
		result[i] = charset[f.rnd.Intn(len(charset))]
	}
	return string(result)
}

// GenerateControlCharString generates a string with various control characters
func (f *Fuzzer) GenerateControlCharString() []string {
	controlChars := []rune{}
	for i := 0; i < 32; i++ {
		controlChars = append(controlChars, rune(i))
	}
	controlChars = append(controlChars, 127) // DEL

	var results []string
	for _, char := range controlChars {
		if unicode.IsControl(char) {
			results = append(results, string(char))
			results = append(results, "test"+string(char)+"string")
			results = append(results, string(char)+"teststring")
			results = append(results, "teststring"+string(char))
		}
	}
	return results
}

// GenerateUnicodeAttacks generates strings with various Unicode attack patterns
func (f *Fuzzer) GenerateUnicodeAttacks() []string {
	return []string{
		// Zero-width characters
		"test\u200Bstring",      // Zero-width space
		"test\u200Cstring",      // Zero-width non-joiner
		"test\u200Dstring",      // Zero-width joiner
		"test\uFEFFstring",      // Zero-width no-break space

		// Direction overrides
		"test\u202Estring",      // Right-to-left override
		"test\u202Dstring",      // Left-to-right override

		// Combining characters
		"test\u0301string",      // Combining acute accent
		"a\u0301\u0302\u0303",   // Multiple combining marks

		// Normalization attacks
		"café",                  // é as single character
		"café",                  // é as e + combining accent

		// Homoglyphs
		"gооgle",                // Cyrillic о instead of Latin o
		"аpple",                 // Cyrillic а instead of Latin a

		// Null and control in Unicode
		"test\u0000string",      // Null character
		"test\u0001string",      // Start of heading
	}
}

// GenerateSQLInjections generates common SQL injection patterns
func (f *Fuzzer) GenerateSQLInjections() []string {
	return []string{
		"'; DROP TABLE users--",
		"' OR '1'='1",
		"' OR 1=1--",
		"admin'--",
		"' UNION SELECT NULL--",
		"' UNION SELECT password FROM users--",
		"1' AND '1'='1",
		"'; EXEC sp_MSForEachTable 'DROP TABLE ?'--",
		"'; SHUTDOWN--",
		"' OR 'x'='x",
		"\"; DROP TABLE users--",
		"\" OR \"1\"=\"1",
		"') OR ('1'='1",
		"1' ORDER BY 1--",
		"1' UNION SELECT NULL, NULL--",
	}
}

// GeneratePathTraversals generates path traversal attack patterns
func (f *Fuzzer) GeneratePathTraversals() []string {
	return []string{
		"../../etc/passwd",
		"../../../etc/passwd",
		"../../../../etc/passwd",
		"..\\..\\windows\\system32\\config\\sam",
		"..%2F..%2F..%2Fetc%2Fpasswd",
		"....//....//....//etc/passwd",
		"..;/..;/..;/etc/passwd",
		"/etc/passwd",
		"\\windows\\system32",
		"/.../../etc/passwd",
	}
}
