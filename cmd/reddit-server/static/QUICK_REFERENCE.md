# Quick Reference Guide

## Installation

Include the script in your HTML:
```html
<script src="/static/app.js"></script>
```

All functions are available as `window.api.*` or `api.*`

## Authentication

```javascript
// Save API key
api.saveApiKey('your-key');

// Check if valid
const isValid = await api.checkAuth('key');

// Get current user
const user = await api.getCurrentUser();

// Logout
api.clearApiKey();
```

## Posts

```javascript
// Hot posts
const result = await api.fetchHotPosts({
  subreddit: 'golang',      // optional
  limit: 25,                // 1-100, default 25
  after: 'cursor',          // pagination
  before: 'cursor',
});

// New posts
const result = await api.fetchNewPosts({ subreddit: 'golang' });

// Generic
const result = await api.fetchPosts('hot', { subreddit: 'golang' });

// Returns: { posts: [], after: '', before: '' }
```

## Comments

```javascript
// Get comments for a post
const result = await api.fetchComments('golang', 'abc123', {
  limit: 25,
  after: 'cursor',
});

// Returns: { post: {}, comments: [], after: '', before: '' }

// Load more comments
const result = await api.fetchMoreComments('t3_abc123', [
  'comment1',
  'comment2',
  'comment3',
]);

// Returns: { comments: [] }
```

## Subreddit Info

```javascript
const sub = await api.fetchSubreddit('golang');

// Properties:
// - display_name (e.g., "golang")
// - display_name_prefixed (e.g., "r/golang")
// - title
// - public_description
// - subscribers
// - active_user_count
// - created_utc
// - over18
// - icon_img
// - banner_img
```

## Utilities

```javascript
// Format timestamp
api.formatTimestamp(1234567890);        // "2 hours ago"

// Format score
api.formatScore(1234567);               // "1.2M"

// Truncate text
api.truncateText('Long text...', 50);   // "Long text..."

// Escape HTML
api.escapeHtml('<script>...');          // "&lt;script&gt;..."

// Markdown to HTML
api.markdownToHtml('**bold** *italic*'); // "<strong>bold</strong> <em>italic</em>"

// Get thumbnail CSS class
api.getThumbnailClass('https://...');   // "thumbnail-image"
```

## State Management

```javascript
// Create state
const state = api.createState({ count: 0 });

// Get value
state.get();  // { count: 0 }

// Set value
state.set({ count: 1 });

// Subscribe to changes
const unsubscribe = state.subscribe(newValue => {
  console.log('Changed:', newValue);
});

// Unsubscribe
unsubscribe();
```

## Error Handling

```javascript
try {
  const posts = await api.fetchHotPosts({ subreddit: 'golang' });
} catch (error) {
  console.error(error.message);
  // Possible messages:
  // - "Authentication required. Please provide a valid API key."
  // - "Rate limited. Please wait a moment before trying again."
  // - "Resource not found."
  // - "Invalid request parameters."
  // - "Request timed out. Please check your connection and try again."
  // - "Network error. Please check your internet connection."
  // - "Server error. Please try again later."
}
```

## Alpine.js Example

```html
<div x-data="app()">
  <input x-model="apiKey" placeholder="API Key">
  <button @click="login()">Login</button>
  
  <div x-show="loggedIn">
    <p x-text="'Hello, ' + user.name"></p>
    <button @click="loadPosts()">Load Posts</button>
    
    <ul>
      <template x-for="post in posts" :key="post.id">
        <li x-text="post.title"></li>
      </template>
    </ul>
  </div>
</div>

<script>
function app() {
  return {
    apiKey: '',
    loggedIn: false,
    user: {},
    posts: [],
    
    async login() {
      const isValid = await api.checkAuth(this.apiKey);
      if (isValid) {
        api.saveApiKey(this.apiKey);
        this.user = await api.getCurrentUser();
        this.loggedIn = true;
      }
    },
    
    async loadPosts() {
      const result = await api.fetchHotPosts({ subreddit: 'golang' });
      this.posts = result.posts;
    },
  };
}
</script>
```

## API Endpoint Reference

| Function | Method | Endpoint | Parameters |
|----------|--------|----------|-----------|
| `fetchHotPosts` | GET | `/api/v1/posts/hot` | subreddit, limit, after, before |
| `fetchNewPosts` | GET | `/api/v1/posts/new` | subreddit, limit, after, before |
| `fetchComments` | GET | `/api/v1/posts/{sub}/{postId}/comments` | limit, after, before |
| `fetchMoreComments` | POST | `/api/v1/posts/{linkId}/more-comments` | children |
| `fetchSubreddit` | GET | `/api/v1/subreddit/{name}` | - |
| `getCurrentUser` | GET | `/api/v1/user/me` | - |

## Configuration

Edit CONFIG object in app.js:

```javascript
const CONFIG = {
  API_BASE_URL: window.location.origin,
  API_KEY_STORAGE: 'reddit_api_key',
  REQUEST_TIMEOUT: 30000,   // 30 seconds
  MAX_RETRIES: 3,
};
```

## Common Patterns

### Pagination
```javascript
let after = '';
const allPosts = [];

while (true) {
  const result = await api.fetchHotPosts({
    subreddit: 'golang',
    limit: 25,
    after,
  });
  
  allPosts.push(...result.posts);
  
  if (!result.after) break;
  after = result.after;
}
```

### Retry with Backoff
```javascript
async function retryFetch(fn, maxRetries = 3) {
  let attempt = 0;
  while (attempt < maxRetries) {
    try {
      return await fn();
    } catch (error) {
      if (error.message.includes('Rate limited')) {
        await new Promise(r => setTimeout(r, 1000 * (attempt + 1)));
        attempt++;
      } else {
        throw error;
      }
    }
  }
}

// Usage
const posts = await retryFetch(
  () => api.fetchHotPosts({ subreddit: 'golang' })
);
```

### Parallel Requests
```javascript
const [hot, new_posts, subs] = await Promise.all([
  api.fetchHotPosts({ limit: 25 }),
  api.fetchNewPosts({ limit: 25 }),
  Promise.all(['golang', 'python', 'javascript'].map(
    name => api.fetchSubreddit(name)
  )),
]);
```

## Debugging

### Enable Logging
App automatically logs to console on localhost:
```
Reddit API Client loaded. Available functions:
- api.checkAuth(key)
- api.getCurrentUser()
...
```

### Check Stored Key
```javascript
console.log('Stored key:', api.getApiKey());
```

### Check State
```javascript
const state = api.createState({ test: true });
state.subscribe(v => console.log('State changed:', v));
state.set({ test: false });
```

## Performance Tips

1. **Cache Results**: Don't fetch the same data repeatedly
2. **Paginate**: Use pagination for large result sets
3. **Lazy Load**: Load app.js asynchronously
4. **Minify**: Use minified version in production
5. **Debounce**: Rate limit user input

## Browser Support

- Chrome 55+
- Firefox 52+
- Safari 10.1+
- Edge 15+

Requires: Fetch API, localStorage, Promise, ES6+

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "api is undefined" | Ensure app.js is loaded before your code |
| 401 errors | API key is invalid. Clear and re-authenticate |
| 429 errors | Too many requests. Wait and retry |
| 404 errors | Resource doesn't exist. Check ID/name |
| Timeout errors | Server not responding. Check connection |

