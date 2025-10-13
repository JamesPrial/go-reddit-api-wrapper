# Testutil Refactoring Patterns Guide

This guide provides standardized patterns for refactoring tests to use the testutil infrastructure. Follow these patterns for consistent, maintainable test code across the entire codebase.

## Table of Contents

1. [Core Principles](#core-principles)
2. [Pattern 1: httptest.NewServer → MockServer](#pattern-1-httptestnewserver--mockserver)
3. [Pattern 2: JSON Strings → Builders](#pattern-2-json-strings--builders)
4. [Pattern 3: Manual Error Checks → Assertions](#pattern-3-manual-error-checks--assertions)
5. [Pattern 4: Table-Driven Test Structure](#pattern-4-table-driven-test-structure)
6. [Pattern 5: Complex Nested Data](#pattern-5-complex-nested-data)
7. [Common Mistakes to Avoid](#common-mistakes-to-avoid)
8. [Checklist for Refactoring](#checklist-for-refactoring)

---

## Core Principles

When refactoring tests, follow these principles:

1. **Preserve Test Coverage**: Refactored tests must verify the exact same behavior
2. **Improve Readability**: Code should be self-documenting and clear
3. **Type Safety First**: Prefer compile-time checks over runtime JSON validation
4. **DRY (Don't Repeat Yourself)**: Reuse builders and helpers
5. **Fail Fast**: Use assertions that provide clear error messages

---

## Pattern 1: httptest.NewServer → MockServer

### When to Apply
- Any test using `httptest.NewServer` with manual request handling
- Tests that need to mock multiple Reddit API endpoints
- Integration-style tests that simulate full API responses

### Before
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.URL.Path == "/r/golang/hot" {
		// Manually construct JSON response
		postData := map[string]interface{}{
			"id": "post1",
			"title": "Test Post",
			"score": 100,
			// ... 15+ more fields
		}
		postJSON, _ := json.Marshal(postData)
		child := map[string]interface{}{
			"kind": "t3",
			"data": json.RawMessage(postJSON),
		}
		listing := map[string]interface{}{
			"kind": "Listing",
			"data": map[string]interface{}{
				"children": []interface{}{child},
			},
		}
		json.NewEncoder(w).Encode(listing)
	} else if r.URL.Path == "/api/v1/me" {
		// More manual JSON construction...
	}
}))
defer server.Close()
```

### After
```go
post := testutil.NewPostBuilder().
	WithID("post1").
	WithTitle("Test Post").
	WithScore(100).
	Build()

account := testutil.NewAccount("testuser").
	WithLinkKarma(1000).
	Build()

server := testutil.NewMockServer().
	WithPosts("golang", "hot", post).
	WithAccount(account).
	Start()
defer server.Close()
```

### Benefits
- **90% less code**: MockServer handles routing and response formatting
- **Type-safe**: Builders ensure correct data structure
- **Declarative**: Clear intent - "this endpoint returns these posts"
- **Consistent**: All tests use same server configuration pattern

### Migration Steps
1. Identify all endpoints the test uses
2. Convert JSON construction to builders
3. Replace server setup with `NewMockServer()`
4. Chain `With*()` methods for each endpoint
5. Call `.Start()` and add `defer .Close()`

---

## Pattern 2: JSON Strings → Builders

### When to Apply
- Any test with raw JSON strings
- Tests using `json.Marshal()` with map literals
- Tests constructing `types.Thing` manually

### Before: Inline JSON Strings
```go
postData := `{"id":"abc123","title":"Test Post","score":100,"name":"t3_abc123","created_utc":1609459200.0,"created":1609459200.0,"permalink":"/r/golang/comments/abc123/test_post/","subreddit":"golang","author":"testuser","url":"https://reddit.com/r/golang/","num_comments":0,"upvote_ratio":0.95}`

*v = types.Thing{
	Kind: "t3",
	Data: json.RawMessage(postData),
}
```

### After: Fluent Builders
```go
post := testutil.NewPostBuilder().
	WithID("abc123").
	WithTitle("Test Post").
	WithScore(100).
	WithSubreddit("golang").
	WithAuthor("testuser").
	Build()

*v = *testutil.NewPostBuilder().
	WithID("abc123").
	WithTitle("Test Post").
	// ... same fields
	ToThing()
```

### Before: Map Literals + Marshal
```go
postData := map[string]interface{}{
	"id":           "post1",
	"title":        "Test Post",
	"score":        100,
	"ups":          100,
	"downs":        0,
	"name":         "t3_post1",
	"created_utc":  1609459200.0,
	"created":      1609459200.0,
	"permalink":    "/r/golang/comments/post1/test_post/",
	"subreddit":    "golang",
	"author":       "testuser",
	"url":          "https://reddit.com/r/golang/",
	"num_comments": 0,
	"upvote_ratio": 0.95,
}
data, _ := json.Marshal(postData)
```

### After: Clean Builders
```go
post := testutil.NewPostBuilder().
	WithID("post1").
	WithTitle("Test Post").
	WithScore(100).
	WithSubreddit("golang").
	WithAuthor("testuser").
	Build()
```

### Builder Selection Guide

| Data Type | Builder | Example |
|-----------|---------|---------|
| Post | `NewPostBuilder()` | `NewPostBuilder().WithTitle("Post").Build()` |
| Comment | `NewCommentBuilder()` | `NewCommentBuilder().WithBody("Text").Build()` |
| Subreddit | `NewSubreddit(name)` | `NewSubreddit("golang").WithSubscribers(10000).Build()` |
| Account | `NewAccount(username)` | `NewAccount("user").WithLinkKarma(100).Build()` |
| More | `NewMore()` | `NewMore().WithChildren([]string{"id1", "id2"}).Build()` |

### Thing Conversion

All builders provide `ToThing()` method:
```go
postThing := testutil.NewPostBuilder().WithID("abc").ToThing()
// postThing.Kind == "t3"
// postThing.Data contains marshaled post

commentThing := testutil.NewCommentBuilder().WithID("def").ToThing()
// commentThing.Kind == "t1"
```

### Benefits
- **Type Safety**: Typos caught at compile time
- **Defaults**: Sensible defaults for all required fields
- **Consistency**: Reddit naming conventions (t3_ prefix, etc.) handled automatically
- **Readability**: Clear field names vs JSON keys
- **Maintenance**: Change struct = update one builder, not 50 tests

---

## Pattern 3: Manual Error Checks → Assertions

### When to Apply
- Any test with `if err == nil { t.Error(...) }` blocks
- Tests checking error types with type assertions
- Tests verifying response counts or field values

### Before: Manual Error Checking
```go
account, err := client.Me(ctx)

if err != nil {
	t.Errorf("unexpected error: %v", err)
}
if account == nil {
	t.Error("expected account but got nil")
}
if account.LinkKarma != 100 {
	t.Errorf("expected link karma 100, got %d", account.LinkKarma)
}
```

### After: Clean Assertions
```go
account, err := client.Me(ctx)

testutil.AssertNoError(t, err)
if account == nil {
	t.Fatal("expected account but got nil")
}
if account.LinkKarma != 100 {
	t.Errorf("expected link karma 100, got %d", account.LinkKarma)
}
```

### Before: Error Type Checking
```go
err := client.GetHot(ctx, invalidRequest)

if err == nil {
	t.Error("expected error but got none")
}

var configErr *pkgerrs.ConfigError
if !errors.As(err, &configErr) {
	t.Errorf("expected ConfigError, got %T", err)
}
```

### After: Type-Safe Assertion
```go
err := client.GetHot(ctx, invalidRequest)

testutil.AssertError(t, err)
testutil.AssertErrorType(t, err, &pkgerrs.ConfigError{})
```

### Before: API Error Checking
```go
err := client.GetSubreddit(ctx, "nonexistent")

if err == nil {
	t.Fatal("expected error but got none")
}

var apiErr *pkgerrs.APIError
if !errors.As(err, &apiErr) {
	t.Fatalf("expected APIError, got %T", err)
}

if apiErr.StatusCode != 404 {
	t.Fatalf("expected status 404, got %d", apiErr.StatusCode)
}
```

### After: Concise API Error Check
```go
err := client.GetSubreddit(ctx, "nonexistent")

testutil.AssertAPIError(t, err, 404)
```

### Before: Count Verification
```go
posts, err := client.GetHot(ctx, request)

if err != nil {
	t.Fatalf("unexpected error: %v", err)
}

if posts == nil {
	t.Fatal("expected posts response but got nil")
}

if len(posts.Posts) != 5 {
	t.Errorf("expected 5 posts, got %d", len(posts.Posts))
}
```

### After: Dedicated Count Assertion
```go
posts, err := client.GetHot(ctx, request)

testutil.AssertNoError(t, err)
testutil.AssertPostCount(t, posts, 5)
```

### Assertion Reference

| Scenario | Assertion | Usage |
|----------|-----------|-------|
| No error expected | `AssertNoError(t, err)` | Success cases |
| Error expected | `AssertError(t, err)` | Failure cases |
| Specific error type | `AssertErrorType(t, err, &Type{})` | ConfigError, AuthError, etc. |
| API error with status | `AssertAPIError(t, err, 404)` | HTTP status validation |
| Post count | `AssertPostCount(t, response, 5)` | After GetHot/GetNew |
| Comment count | `AssertCommentCount(t, response, 3)` | After GetComments |
| String contains | `AssertStringContains(t, str, substr)` | Error messages |
| Post equality | `AssertPostEqual(t, expected, actual)` | Field comparison |
| Comment equality | `AssertCommentEqual(t, expected, actual)` | Field comparison |

### Benefits
- **Consistency**: All tests check errors the same way
- **Clarity**: Intent is obvious (`AssertNoError` vs nested if statements)
- **Helper Support**: `t.Helper()` gives accurate stack traces
- **Readability**: Tests focus on behavior, not error handling boilerplate

---

## Pattern 4: Table-Driven Test Structure

### Standard Table Test Structure

```go
func TestClient_Operation_Refactored(t *testing.T) {
	tests := []struct {
		name      string
		// INPUT: Request configuration
		request   *types.Request
		// SETUP: Test data using builders
		setupData *TestData
		// EXPECT: Expected results
		wantError bool
		want      *ExpectedResult
	}{
		{
			name: "descriptive test case name",
			request: &types.Request{
				Field: "value",
			},
			setupData: &TestData{
				Posts: []*types.Post{
					testutil.NewPostBuilder().WithID("1").Build(),
				},
			},
			wantError: false,
			want: &ExpectedResult{
				Count: 1,
			},
		},
		// More test cases...
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE: Setup mock server/client
			server := setupMockServer(tt.setupData)
			defer server.Close()
			client := setupClient(server)

			// ACT: Execute the operation
			result, err := client.Operation(context.Background(), tt.request)

			// ASSERT: Verify results
			if tt.wantError {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
				verifyResult(t, tt.want, result)
			}
		})
	}
}
```

### Table Test Guidelines

1. **Name Tests Descriptively**: Use clear names that explain scenario
2. **Separate Concerns**: Input, setup, and expectations in distinct fields
3. **Use Builders in Setup**: Never construct JSON in table definitions
4. **Keep Cases Independent**: Each case should be self-contained
5. **Group Related Cases**: Organize by success/error/edge cases

### Example: Success and Error Cases

```go
tests := []struct {
	name      string
	request   *types.PostsRequest
	posts     []*types.Post
	wantError bool
	wantCount int
}{
	// SUCCESS CASES
	{
		name: "returns posts from hot feed",
		request: &types.PostsRequest{
			Subreddit: "golang",
			Pagination: types.Pagination{Limit: 10},
		},
		posts: []*types.Post{
			testutil.NewPostBuilder().WithID("1").Build(),
			testutil.NewPostBuilder().WithID("2").Build(),
		},
		wantError: false,
		wantCount: 2,
	},
	{
		name: "empty subreddit returns front page",
		request: &types.PostsRequest{
			Subreddit: "",
		},
		posts: []*types.Post{
			testutil.NewPostBuilder().WithID("front1").Build(),
		},
		wantError: false,
		wantCount: 1,
	},

	// ERROR CASES
	{
		name: "invalid subreddit name",
		request: &types.PostsRequest{
			Subreddit: "ab", // Too short
		},
		posts:     nil,
		wantError: true,
	},
	{
		name: "private subreddit returns error",
		request: &types.PostsRequest{
			Subreddit: "private",
		},
		posts:     nil,
		wantError: true,
	},
}
```

---

## Pattern 5: Complex Nested Data

### Handling Nested Comments

#### Before: Embedded JSON Hell
```go
commentData := `{"id":"cnested","body":"Test comment","author":"user1","link_id":"t3_abc123","parent_id":"t3_abc123","name":"t1_cnested","created_utc":1609459200.0,"created":1609459200.0,"permalink":"/r/golang/comments/abc123/test_post/cnested/","subreddit":"golang","score":10,"ups":10,"downs":0,"replies":{"kind":"Listing","data":{"children":[{"kind":"more","data":{"id":"moreid1","name":"t1_moreid1","children":["more1","more2"]}}]}}}`
```

#### After: Structured with Builders
```go
// Build child comments first
reply1 := testutil.NewCommentBuilder().
	WithID("reply1").
	WithBody("First reply").
	WithParentID("t1_parent").
	Build()

reply2 := testutil.NewCommentBuilder().
	WithID("reply2").
	WithBody("Second reply").
	WithParentID("t1_parent").
	Build()

// Build parent with nested replies
parent := testutil.NewCommentBuilder().
	WithID("parent").
	WithBody("Parent comment").
	WithReplies(reply1, reply2).
	Build()
```

### Handling "More" Comments

```go
// Create More data for collapsed comment thread
more := testutil.NewMore().
	WithID("more123").
	WithChildren([]string{"comment1", "comment2", "comment3"}).
	Build()

// Or use ToThing() for API responses
moreThing := testutil.NewMore().
	WithChildren([]string{"id1", "id2"}).
	ToThing()
```

### Complex Comment Thread Example

```go
// Three-level comment thread:
// - Top-level comment
//   - Reply 1
//     - Nested reply
//   - Reply 2

nestedReply := testutil.NewCommentBuilder().
	WithID("nested").
	WithBody("Nested reply").
	Build()

reply1 := testutil.NewCommentBuilder().
	WithID("reply1").
	WithBody("First reply").
	WithReplies(nestedReply).
	Build()

reply2 := testutil.NewCommentBuilder().
	WithID("reply2").
	WithBody("Second reply").
	Build()

topLevel := testutil.NewCommentBuilder().
	WithID("top").
	WithBody("Top-level comment").
	WithReplies(reply1, reply2).
	Build()

// Use in test
server := testutil.NewMockServer().
	WithComments("golang", "post123", post, topLevel).
	Start()
```

### Benefits
- **Visual Structure**: Nesting is explicit and readable
- **Flexibility**: Easy to modify depth or add branches
- **Type Safety**: Compiler ensures Comment types are correct
- **Reusability**: Build sub-trees once, use in multiple tests

---

## Common Mistakes to Avoid

### ❌ Mistake 1: Mixing JSON and Builders

**Bad:**
```go
post := testutil.NewPostBuilder().WithID("abc").Build()
postData := `{"id":"abc","title":"Test",...}` // Mixing approaches!
```

**Good:**
```go
post := testutil.NewPostBuilder().
	WithID("abc").
	WithTitle("Test").
	Build()
```

### ❌ Mistake 2: Not Using Defaults

**Bad:**
```go
post := testutil.NewPostBuilder().
	WithID("1").
	WithName("t3_1").
	WithTitle("Title").
	WithScore(100).
	WithUps(100).
	WithDowns(0).
	WithAuthor("user").
	WithSubreddit("golang").
	WithPermalink("/r/golang/...").
	WithURL("https://...").
	WithCreated(1234567890).
	WithNumComments(0).
	WithUpvoteRatio(0.95).
	Build()
```

**Good:**
```go
// Defaults handle most fields! Only customize what matters.
post := testutil.NewPostBuilder().
	WithID("1").
	WithTitle("Title").
	Build()
```

### ❌ Mistake 3: Ignoring MockServer Capabilities

**Bad:**
```go
// Manually mocking each endpoint
mock := &mockHTTPClient{
	doFunc: func(req *http.Request, v *types.Thing) error {
		if strings.Contains(req.URL.Path, "/hot") {
			// Manual JSON construction
		} else if strings.Contains(req.URL.Path, "/about") {
			// More manual JSON
		}
		return nil
	},
}
```

**Good:**
```go
// Let MockServer handle routing
server := testutil.NewMockServer().
	WithPosts("golang", "hot", posts...).
	WithSubreddit("golang", subreddit).
	Start()
```

### ❌ Mistake 4: Over-Asserting

**Bad:**
```go
testutil.AssertNoError(t, err)
if posts == nil {
	t.Fatal("posts nil")
}
if posts.Posts == nil {
	t.Fatal("posts.Posts nil")
}
if len(posts.Posts) == 0 {
	t.Fatal("no posts")
}
if len(posts.Posts) != 5 {
	t.Errorf("wrong count")
}
```

**Good:**
```go
testutil.AssertNoError(t, err)
testutil.AssertPostCount(t, posts, 5) // Checks nil and count
```

### ❌ Mistake 5: Not Using t.Helper()

**Bad:**
```go
func verifyPost(t *testing.T, post *types.Post) {
	if post.ID == "" {
		t.Error("post ID empty") // Line number will be wrong!
	}
}
```

**Good:**
```go
func verifyPost(t *testing.T, post *types.Post) {
	t.Helper() // Correct line numbers in failures
	if post.ID == "" {
		t.Error("post ID empty")
	}
}
```

---

## Checklist for Refactoring

Use this checklist when refactoring each test:

### Pre-Refactoring
- [ ] Read original test and understand what it verifies
- [ ] Identify all test cases (table rows or separate tests)
- [ ] Note any special assertions or edge cases
- [ ] Check if test uses httptest.NewServer or mockHTTPClient

### During Refactoring
- [ ] Convert JSON strings to builders
- [ ] Replace httptest setup with MockServer where applicable
- [ ] Change manual error checks to assertions
- [ ] Use table-driven format for multiple cases
- [ ] Preserve all original test coverage
- [ ] Add comments explaining complex scenarios

### Post-Refactoring
- [ ] Run refactored test: `go test -run TestName`
- [ ] Verify test passes with same behavior
- [ ] Check test coverage is maintained
- [ ] Review code for clarity and readability
- [ ] Ensure no JSON strings remain
- [ ] Verify proper use of `t.Helper()` in helper functions

### Before Committing
- [ ] Run full test suite: `go test ./...`
- [ ] Check for consistent style across refactored tests
- [ ] Update test documentation if needed
- [ ] Add test to refactoring tracking document

---

## Summary

Follow these patterns for consistent, maintainable tests:

1. **MockServer over httptest**: Declarative endpoint configuration
2. **Builders over JSON**: Type-safe, clear, reusable test data
3. **Assertions over manual checks**: Consistent, clear error messages
4. **Table-driven structure**: Organize cases logically
5. **Compose complexity**: Build nested structures step-by-step

**Result**: Tests that are shorter, clearer, and easier to maintain while preserving full test coverage.

---

## Quick Reference Card

```go
// 1. Create test data
post := testutil.NewPostBuilder().WithID("1").Build()
comment := testutil.NewCommentBuilder().WithBody("Hi").Build()
account := testutil.NewAccount("user").WithLinkKarma(100).Build()

// 2. Setup mock server
server := testutil.NewMockServer().
	WithPosts("golang", "hot", post).
	WithAccount(account).
	Start()
defer server.Close()

// 3. Execute test
result, err := client.Operation(ctx, request)

// 4. Assert results
testutil.AssertNoError(t, err)
testutil.AssertPostCount(t, result, 1)
```

This is the pattern. Repeat for every test. Keep it simple.
