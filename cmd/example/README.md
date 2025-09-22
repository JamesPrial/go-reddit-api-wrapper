# Reddit API Wrapper Example

This example demonstrates all features of the Reddit API wrapper, including the new pagination and tree traversal capabilities.

## Running the Example

```bash
# Set your Reddit API credentials
export REDDIT_CLIENT_ID="your_client_id"
export REDDIT_CLIENT_SECRET="your_client_secret"
export REDDIT_USERNAME="your_username"        # Optional for user auth
export REDDIT_PASSWORD="your_password"        # Optional for user auth

# Build and run
go build -o reddit-example ./cmd/example
./reddit-example
```

## Features Demonstrated

### Basic Operations
- Connecting to Reddit with OAuth2 authentication
- Fetching hot posts from a subreddit
- Getting subreddit information
- Loading comments for a post
- User authentication (if credentials provided)

### Advanced Pagination Features

#### 1. PostIterator
- Seamlessly paginate through posts without managing tokens
- Automatically fetches next batch when needed
- Configurable batch size (up to Reddit's limit of 100)

```go
iterator := client.NewHotIterator(ctx, "golang").WithLimit(10)
for iterator.HasNext() {
    post, err := iterator.Next()
    // Process post
}
```

#### 2. GetMoreComments
- Load comments that were truncated in initial response
- Uses Reddit's `/api/morechildren` endpoint
- Configurable sort order and depth

```go
moreComments, err := client.GetMoreComments(ctx, postID, commentIDs, &graw.MoreCommentsOptions{
    Sort:  "best",
    Limit: 10,
})
```

#### 3. Batch Operations
- Load comments for multiple posts in parallel
- More efficient than sequential requests
- Maintains result ordering

```go
requests := []graw.CommentRequest{
    {Subreddit: "golang", PostID: post1ID, Options: opts},
    {Subreddit: "golang", PostID: post2ID, Options: opts},
}
results, err := client.GetCommentsMultiple(ctx, requests)
```

#### 4. Bulk Collection
- Collect multiple pages of posts at once
- Useful for data analysis and aggregation

```go
collector := client.NewNewIterator(ctx, "golang").WithLimit(25)
allPosts, err := collector.Collect(50) // Get up to 50 posts
```

## Output

The example will display:
1. Connection status and user info (if authenticated)
2. Hot posts from r/golang with scores and comment counts
3. Subreddit information (subscribers, description)
4. Comments from the first hot post
5. Demonstration of all pagination and tree traversal features
6. Statistics like average post scores and comment tree depth

## Error Handling

The example includes comprehensive error handling and logging:
- Connection failures
- API rate limiting
- Malformed responses
- Missing data fields

## Performance Notes

- The PostIterator caches pages to minimize API calls
- Batch operations use goroutines for parallel fetching
- The iterative tree traversal avoids stack overflow on deep comment threads
- Rate limiting is handled automatically by the wrapper