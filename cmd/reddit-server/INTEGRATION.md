# HTTP Server Integration Guide

This guide shows how to integrate the Reddit HTTP server with your applications.

## Quick Start

### 1. Start the Server

```bash
export REDDIT_CLIENT_ID="your-client-id"
export REDDIT_CLIENT_SECRET="your-client-secret"
cd cmd/reddit-server
go build
./reddit-server
```

The server runs on `http://localhost:8080` by default.

### 2. Test the Server

```bash
# Health check (no auth required)
curl http://localhost:8080/health

# Get user info (requires auth)
curl http://localhost:8080/api/v1/user/me \
  -H "X-Reddit-Client-ID: your-id" \
  -H "X-Reddit-Client-Secret: your-secret"
```

## JavaScript/TypeScript Integration

### Using Fetch API

```javascript
class RedditAPIClient {
  constructor(clientId, clientSecret, baseUrl = 'http://localhost:8080') {
    this.clientId = clientId;
    this.clientSecret = clientSecret;
    this.baseUrl = baseUrl;
  }

  private async request(path, options = {}) {
    const headers = {
      'X-Reddit-Client-ID': this.clientId,
      'X-Reddit-Client-Secret': this.clientSecret,
      'Content-Type': 'application/json',
      ...options.headers,
    };

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error.message);
    }

    return response.json();
  }

  async getUser() {
    return this.request('/api/v1/user/me');
  }

  async getSubreddit(name) {
    return this.request(`/api/v1/subreddit/${name}`);
  }

  async getHotPosts(subreddit = '', limit = 25, after = '') {
    const params = new URLSearchParams({ limit });
    if (subreddit) params.append('subreddit', subreddit);
    if (after) params.append('after', after);
    return this.request(`/api/v1/posts/hot?${params}`);
  }

  async getNewPosts(subreddit = '', limit = 25, after = '') {
    const params = new URLSearchParams({ limit });
    if (subreddit) params.append('subreddit', subreddit);
    if (after) params.append('after', after);
    return this.request(`/api/v1/posts/new?${params}`);
  }

  async getComments(subreddit, postId, limit = 25, after = '') {
    const params = new URLSearchParams({ limit });
    if (after) params.append('after', after);
    return this.request(
      `/api/v1/posts/${subreddit}/${postId}/comments?${params}`
    );
  }

  async getMoreComments(linkId, commentIds) {
    return this.request(`/api/v1/posts/${linkId}/more-comments`, {
      method: 'POST',
      body: JSON.stringify({
        link_id: linkId,
        comment_ids: commentIds,
      }),
    });
  }
}

// Usage
const client = new RedditAPIClient('your-id', 'your-secret');

// Get user info
const user = await client.getUser();
console.log(`Logged in as: ${user.data.name}`);

// Get hot posts from /r/golang
const posts = await client.getHotPosts('golang', 25);
console.log(`Found ${posts.data.length} hot posts`);

// Get first 10 comments on a post
const comments = await client.getComments('golang', 'abc123', 10);
console.log(`Post: ${comments.data.post.title}`);
console.log(`Comments: ${comments.data.comments.length}`);
```

### Using Axios

```typescript
import axios, { AxiosInstance } from 'axios';

interface UserData {
  name: string;
  link_karma: number;
  comment_karma: number;
}

interface PostData {
  id: string;
  title: string;
  author: string;
  score: number;
  num_comments: number;
}

interface APIResponse<T> {
  data: T;
  pagination?: {
    after?: string;
    before?: string;
  };
}

class RedditAPI {
  private client: AxiosInstance;

  constructor(clientId: string, clientSecret: string, baseURL = 'http://localhost:8080') {
    this.client = axios.create({
      baseURL,
      headers: {
        'X-Reddit-Client-ID': clientId,
        'X-Reddit-Client-Secret': clientSecret,
      },
    });
  }

  async getUser(): Promise<APIResponse<UserData>> {
    const { data } = await this.client.get('/api/v1/user/me');
    return data;
  }

  async getSubreddit(name: string) {
    const { data } = await this.client.get(`/api/v1/subreddit/${name}`);
    return data;
  }

  async getHotPosts(
    subreddit = '',
    limit = 25,
    after = ''
  ): Promise<APIResponse<PostData[]>> {
    const params = new URLSearchParams({ limit: limit.toString() });
    if (subreddit) params.append('subreddit', subreddit);
    if (after) params.append('after', after);
    const { data } = await this.client.get(`/api/v1/posts/hot?${params}`);
    return data;
  }

  async getNewPosts(
    subreddit = '',
    limit = 25,
    after = ''
  ): Promise<APIResponse<PostData[]>> {
    const params = new URLSearchParams({ limit: limit.toString() });
    if (subreddit) params.append('subreddit', subreddit);
    if (after) params.append('after', after);
    const { data } = await this.client.get(`/api/v1/posts/new?${params}`);
    return data;
  }

  async getComments(
    subreddit: string,
    postId: string,
    limit = 25,
    after = ''
  ) {
    const params = new URLSearchParams({ limit: limit.toString() });
    if (after) params.append('after', after);
    const { data } = await this.client.get(
      `/api/v1/posts/${subreddit}/${postId}/comments?${params}`
    );
    return data;
  }
}

// Usage
const api = new RedditAPI('your-id', 'your-secret');
const user = await api.getUser();
console.log(`User: ${user.data.name}`);
```

## Python Integration

```python
import requests
from typing import Dict, List, Any, Optional

class RedditHTTPClient:
    def __init__(self, client_id: str, client_secret: str, base_url: str = "http://localhost:8080"):
        self.client_id = client_id
        self.client_secret = client_secret
        self.base_url = base_url
        self.session = requests.Session()
        self.session.headers.update({
            "X-Reddit-Client-ID": client_id,
            "X-Reddit-Client-Secret": client_secret,
        })

    def _request(self, method: str, path: str, **kwargs) -> Dict[str, Any]:
        url = f"{self.base_url}{path}"
        response = self.session.request(method, url, **kwargs)
        response.raise_for_status()
        return response.json()

    def get_user(self) -> Dict[str, Any]:
        return self._request("GET", "/api/v1/user/me")

    def get_subreddit(self, name: str) -> Dict[str, Any]:
        return self._request("GET", f"/api/v1/subreddit/{name}")

    def get_hot_posts(self, subreddit: str = "", limit: int = 25, after: Optional[str] = None) -> Dict[str, Any]:
        params = {"limit": limit}
        if subreddit:
            params["subreddit"] = subreddit
        if after:
            params["after"] = after
        return self._request("GET", "/api/v1/posts/hot", params=params)

    def get_new_posts(self, subreddit: str = "", limit: int = 25, after: Optional[str] = None) -> Dict[str, Any]:
        params = {"limit": limit}
        if subreddit:
            params["subreddit"] = subreddit
        if after:
            params["after"] = after
        return self._request("GET", "/api/v1/posts/new", params=params)

    def get_comments(self, subreddit: str, post_id: str, limit: int = 25, after: Optional[str] = None) -> Dict[str, Any]:
        params = {"limit": limit}
        if after:
            params["after"] = after
        return self._request("GET", f"/api/v1/posts/{subreddit}/{post_id}/comments", params=params)

    def get_more_comments(self, link_id: str, comment_ids: List[str]) -> Dict[str, Any]:
        data = {
            "link_id": link_id,
            "comment_ids": comment_ids,
        }
        return self._request("POST", f"/api/v1/posts/{link_id}/more-comments", json=data)

# Usage
client = RedditHTTPClient("your-id", "your-secret")

# Get user info
user = client.get_user()
print(f"User: {user['data']['name']}")

# Get hot posts
posts = client.get_hot_posts("golang", limit=10)
for post in posts['data']:
    print(f"{post['title']} ({post['score']} upvotes)")
```

## Frontend Integration (Svelte/Vue/React)

### Svelte Component Example

```svelte
<script>
  import { onMount } from 'svelte';

  let user = null;
  let posts = [];
  let loading = false;
  let error = null;

  const API_BASE = 'http://localhost:8080';
  const headers = {
    'X-Reddit-Client-ID': import.meta.env.VITE_REDDIT_CLIENT_ID,
    'X-Reddit-Client-Secret': import.meta.env.VITE_REDDIT_CLIENT_SECRET,
  };

  async function fetchUser() {
    try {
      loading = true;
      const res = await fetch(`${API_BASE}/api/v1/user/me`, { headers });
      if (!res.ok) throw new Error('Failed to fetch user');
      const data = await res.json();
      user = data.data;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function fetchPosts(subreddit) {
    try {
      loading = true;
      const url = new URL(`${API_BASE}/api/v1/posts/hot`);
      if (subreddit) url.searchParams.append('subreddit', subreddit);
      url.searchParams.append('limit', '25');

      const res = await fetch(url, { headers });
      if (!res.ok) throw new Error('Failed to fetch posts');
      const data = await res.json();
      posts = data.data;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    fetchUser();
    fetchPosts('golang');
  });
</script>

<div>
  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if user}
    <div class="user-info">
      <h2>{user.name}</h2>
      <p>Link Karma: {user.link_karma}</p>
      <p>Comment Karma: {user.comment_karma}</p>
    </div>
  {/if}

  {#if loading}
    <p>Loading...</p>
  {:else if posts.length > 0}
    <div class="posts">
      {#each posts as post (post.id)}
        <div class="post">
          <h3>{post.title}</h3>
          <p>by {post.author}</p>
          <p>{post.score} upvotes, {post.num_comments} comments</p>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .user-info {
    background: #f0f0f0;
    padding: 1rem;
    border-radius: 4px;
  }

  .posts {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .post {
    border: 1px solid #ddd;
    padding: 1rem;
    border-radius: 4px;
  }
</style>
```

## Docker Integration

### Dockerfile

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY . .
RUN cd cmd/reddit-server && go build -o reddit-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/cmd/reddit-server/reddit-server .

EXPOSE 8080
ENV SERVER_PORT=8080
CMD ["./reddit-server"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  reddit-server:
    build: .
    ports:
      - "8080:8080"
    environment:
      REDDIT_CLIENT_ID: ${REDDIT_CLIENT_ID}
      REDDIT_CLIENT_SECRET: ${REDDIT_CLIENT_SECRET}
      SERVER_PORT: 8080
      CORS_ALLOWED_ORIGINS: "http://localhost:5173,http://localhost:3000"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  frontend:
    image: node:latest
    working_dir: /app
    volumes:
      - ./frontend/web:/app
    ports:
      - "5173:5173"
    environment:
      VITE_REDDIT_CLIENT_ID: ${REDDIT_CLIENT_ID}
      VITE_REDDIT_CLIENT_SECRET: ${REDDIT_CLIENT_SECRET}
    command: npm run dev
```

Run with:
```bash
export REDDIT_CLIENT_ID="your-id"
export REDDIT_CLIENT_SECRET="your-secret"
docker-compose up
```

## Error Handling

All client implementations should handle errors appropriately:

```javascript
async function safeRequest(path, options = {}) {
  try {
    const response = await fetch(`http://localhost:8080${path}`, {
      headers: {
        'X-Reddit-Client-ID': clientId,
        'X-Reddit-Client-Secret': clientSecret,
      },
      ...options,
    });

    if (!response.ok) {
      const error = await response.json();
      switch (response.status) {
        case 400:
          console.error('Validation error:', error.error.message);
          break;
        case 401:
          console.error('Authentication failed:', error.error.message);
          break;
        case 404:
          console.error('Not found:', error.error.message);
          break;
        case 429:
          console.error('Rate limited. Please wait before retrying.');
          break;
        default:
          console.error('Server error:', error.error.message);
      }
      throw new Error(error.error.message);
    }

    return response.json();
  } catch (error) {
    console.error('Request failed:', error);
    throw error;
  }
}
```

## Best Practices

1. **Use Environment Variables**: Store credentials in environment variables, not in code
2. **Handle Pagination**: Implement proper pagination using `after`/`before` tokens
3. **Respect Rate Limits**: The server respects Reddit's rate limits; implement exponential backoff
4. **Connection Pooling**: Use persistent connections when possible
5. **Error Handling**: Implement proper error handling for all network requests
6. **Caching**: Cache responses appropriately to reduce API calls
7. **Timeouts**: Set reasonable timeouts on client requests
8. **User Agent**: Always provide a meaningful user agent string

## Troubleshooting Integration Issues

### CORS Errors
Configure `CORS_ALLOWED_ORIGINS` when starting the server:
```bash
CORS_ALLOWED_ORIGINS="http://localhost:5173,https://myapp.com" ./reddit-server
```

### Authentication Failures
Verify credentials are correctly set in headers or environment:
```javascript
// Check that headers are being sent
console.log('Headers:', {
  'X-Reddit-Client-ID': clientId,
  'X-Reddit-Client-Secret': clientSecret,
});
```

### Timeout Issues
Increase request timeout in your client configuration:
```javascript
fetch(url, {
  signal: AbortSignal.timeout(60000), // 60 second timeout
})
```

### Rate Limiting
Implement exponential backoff:
```javascript
async function retryWithBackoff(fn, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (error) {
      if (error.response?.status === 429) {
        const delay = Math.pow(2, i) * 1000;
        await new Promise(resolve => setTimeout(resolve, delay));
      } else {
        throw error;
      }
    }
  }
}
```
