# Parse Package

Internal package for parsing Reddit API responses into typed Go structures.

## Purpose

The `parse` package handles the conversion of Reddit's Thing wrapper format (with `kind` and `data` fields) into concrete Go types. Reddit returns all API responses wrapped in Thing objects with type discriminators (`t1` for comments, `t3` for posts, etc.), and this package unwraps them into usable structs.

## Key Features

- **Multi-Type Parsing**: Supports all Reddit Thing types
  - `t1` - Comments (with nested reply trees)
  - `t2` - Account data
  - `t3` - Posts/Links
  - `t4` - Messages
  - `t5` - Subreddit data
  - `Listing` - Paginated collections
  - `more` - Continuation tokens for deferred loading

- **Comment Tree Construction**: Builds proper hierarchical structures where each comment's `Replies` field contains only direct children, not all descendants

- **Security Protections**:
  - Maximum depth limit (`MaxCommentDepth = 50`) prevents stack overflow attacks
  - Loop detection using `seenIDs` map prevents infinite recursion
  - Input validation for all parsed fields (IDs, subreddit names, timestamps, etc.)

- **Performance Optimizations**:
  - `sync.Pool` for reusing parse contexts reduces allocations
  - Single JSON unmarshal per comment (unified structure)
  - Efficient buffer management

- **Pagination Support**: Extracts `AfterFullname` and `BeforeFullname` tokens for cursor-based pagination

- **"More" ID Collection**: Recursively collects all continuation IDs from comment trees for deferred loading

## Usage

```go
package main

import (
    "context"
    "log/slog"

    "github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
    "github.com/jamesprial/go-reddit-api-wrapper/reddit/internal/parse"
)

func main() {
    // Create parser (optionally with logger)
    logger := slog.Default()
    parser := parse.NewParser(logger)

    // Parse a single Thing
    thing := &types.Thing{
        Kind: "t3",
        Data: []byte(`{"id":"abc123","title":"Test Post",...}`),
    }

    result, err := parser.ParseThing(context.Background(), thing)
    if err != nil {
        log.Fatal(err)
    }

    // Type assert to specific type
    post := result.(*types.Post)

    // Extract posts from a listing
    posts, err := parser.ExtractPosts(ctx, listingThing)

    // Extract comments with tree structure and "more" IDs
    comments, moreIDs, err := parser.ExtractComments(ctx, commentsThing)

    // Parse typical GetComments response: [post_listing, comments_listing]
    response, err := parser.ExtractPostAndComments(ctx, []*types.Thing{...})
}
```

## Error Types

The package defines five specific error types for different failure modes:

1. **`KindError`**: Wrong or unknown Thing kind (e.g., expected `t3` but got `t1`)
2. **`UnmarshalError`**: JSON unmarshaling failed (malformed data)
3. **`ValidationError`**: Data validation failed (invalid IDs, timestamps, formats, etc.)
4. **`DepthError`**: Comment tree exceeds `MaxCommentDepth` (security protection)
5. **`ExtractionError`**: High-level extraction operations failed (e.g., empty response, both post and comments missing)

All errors that wrap underlying errors implement `Unwrap()` for error chain inspection.

## Security Considerations

This package is designed with security in mind:

- **Depth Limiting**: The `MaxCommentDepth` constant (50) prevents stack overflow attacks from deeply nested comment structures
- **Loop Detection**: The `seenIDs` map in `parseContext` prevents infinite loops from circular references
- **Input Validation**: All parsed data is validated using `pkg/validation` to reject:
  - Invalid IDs (uppercase, special characters, SQL injection attempts)
  - Invalid subreddit names (too short, special characters)
  - Invalid timestamps (future dates, before Reddit existed)
  - Invalid fullnames (wrong format, uppercase)
  - Negative counts where not allowed
  - Out-of-range ratios

## Implementation Notes

- Parser uses `context.Context` for cancellation support
- Logging is optional; pass `nil` or omit the logger parameter for no logging
- Parse operations are safe for concurrent use (Parser is stateless except for the pool)
- The `parseContext` struct tracks depth and seen IDs per parse operation
- Comment trees maintain proper structure: `Replies` contains only direct children
