# Benchmark Test Fixtures

This directory contains realistic Reddit API response fixtures for benchmarking and performance testing.

## Files

### Posts Fixtures

#### small_posts.json (17 KB)
- **Content**: 10 posts from r/golang
- **Use case**: Basic parsing benchmarks, quick tests
- **Structure**: Reddit Listing response with pagination
- **Features**:
  - Mix of self-posts and link posts
  - Various post types: text, links, images
  - Realistic scores, comment counts, and timestamps
  - Different flair types and authors

#### medium_posts.json (138 KB)
- **Content**: 100 posts across multiple subreddits
- **Use case**: Medium-scale parsing benchmarks
- **Structure**: Reddit Listing response with pagination
- **Features**:
  - Diverse subreddits: golang, programming, learnprogramming, webdev, devops
  - Mix of domains: github.com, medium.com, dev.to, go.dev
  - Various post flairs and author roles
  - Includes stickied, edited, and saved posts

#### large_posts.json (1.4 MB)
- **Content**: 1000 posts across 10 subreddits
- **Use case**: High-volume parsing performance tests
- **Structure**: Reddit Listing response with pagination
- **Features**:
  - 10 different subreddits
  - Multiple domains and content types
  - Comprehensive variation in post attributes
  - Tests memory allocation and parsing efficiency at scale

### Comments Fixtures

#### deep_comments.json (506 KB)
- **Content**: Single comment thread with 50 levels of nesting
- **Use case**: Maximum depth parsing tests
- **Structure**: Reddit comments endpoint response (array with post and comments)
- **Features**:
  - Tests MaxCommentDepth = 50
  - Single thread going maximum depth
  - Each level has one reply
  - Total: 51 comments (1 top-level + 50 nested)
  - Tests recursive parsing performance
  - Note: Exceeds jq's depth limit but valid JSON (verified with Go)

#### wide_comments.json (716 KB)
- **Content**: Wide comment tree with breadth-first structure
- **Use case**: Breadth-first parsing performance tests
- **Structure**: Reddit comments endpoint response (array with post and comments)
- **Features**:
  - 100 top-level comments
  - Each top-level comment has 5 direct replies
  - Total: 600 comments
  - Tests horizontal tree traversal
  - Simulates popular posts with many discussions

## Structure Details

### Posts Response Format
```json
{
  "kind": "Listing",
  "data": {
    "modhash": "...",
    "dist": N,
    "after": "t3_...",
    "before": null,
    "children": [
      {
        "kind": "t3",
        "data": {
          "id": "...",
          "name": "t3_...",
          "title": "...",
          "author": "...",
          ...
        }
      }
    ]
  }
}
```

### Comments Response Format
```json
[
  {
    "kind": "Listing",
    "data": {
      "children": [
        {
          "kind": "t3",
          "data": { /* post data */ }
        }
      ]
    }
  },
  {
    "kind": "Listing",
    "data": {
      "children": [
        {
          "kind": "t1",
          "data": {
            "id": "...",
            "name": "t1_...",
            "body": "...",
            "replies": {
              "kind": "Listing",
              "data": {
                "children": [ /* nested comments */ ]
              }
            }
          }
        }
      ]
    }
  }
]
```

## Usage in Benchmarks

These fixtures can be used in Go benchmarks like this:

```go
func BenchmarkParseSmallPosts(b *testing.B) {
    data, _ := os.ReadFile("testdata/small_posts.json")
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var listing Listing
        json.Unmarshal(data, &listing)
    }
}
```

## Validation

All fixtures are:
- Valid JSON (verified with Go's encoding/json)
- Conform to Reddit's API structure
- Include realistic field values and data types
- Ready for use in production-like benchmarks

Note: deep_comments.json exceeds some tools' JSON depth limits (like jq's default 128) but is valid and parseable by Go's standard library.
