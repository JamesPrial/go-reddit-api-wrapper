# Reddit API Client (app.js)

A production-ready vanilla JavaScript API client for the reddit-server HTTP server. Works seamlessly with Alpine.js or any vanilla JavaScript framework.

## Features

- **Zero Dependencies**: Pure vanilla JavaScript, no external libraries required
- **Full API Coverage**: All reddit-server endpoints (posts, comments, subreddit, user)
- **Error Handling**: User-friendly error messages with automatic 401 cleanup
- **Rate Limiting**: Automatic rate limit detection and user feedback
- **LocalStorage Integration**: Persist API key across sessions
- **Timeout Protection**: 30-second request timeout with automatic abort
- **Retry Logic**: Simple exponential backoff for network errors
- **Utility Functions**: Timestamp formatting, score abbreviation, HTML escaping
- **State Management**: Simple createState() helper for Alpine.js
- **XSS Protection**: HTML escaping and input validation
- **Alpine.js Ready**: Global `window.api` object for easy integration

## Installation

Include the script in your HTML:

```html
<script src="/static/app.js"></script>
```

All functions are available via `window.api` and `window.createState`.

## Quick Start

### Authentication

```javascript
// Save an API key
api.saveApiKey('your-api-key');

// Validate an API key
const isValid = await api.checkAuth('your-api-key');
if (!isValid) {
  console.log('Invalid API key');
}

// Get current user
try {
  const user = await api.getCurrentUser();
  console.log('Username:', user.name);
  console.log('Karma:', user.link_karma + user.comment_karma);
} catch (error) {
  console.error('Error:', error.message);
}

// Clear stored key
api.clearApiKey();
```

### Fetching Posts

```javascript
// Get hot posts from frontpage
const hotPosts = await api.fetchHotPosts({ limit: 25 });
console.log('Posts:', hotPosts.posts);
console.log('Next page cursor:', hotPosts.after);

// Get new posts from specific subreddit
const newPosts = await api.fetchNewPosts({
  subreddit: 'golang',
  limit: 50,
});

// Pagination
const page2 = await api.fetchHotPosts({
  subreddit: 'golang',
  limit: 25,
  after: hotPosts.after,
});
```

### Fetching Comments

```javascript
// Get comments for a post
const result = await api.fetchComments('golang', 'abc123', {
  limit: 100,
});

console.log('Post:', result.post.title);
console.log('Comments:', result.comments);

// Expand collapsed comment threads
const moreComments = await api.fetchMoreComments('t3_abc123', [
  'comment_id_1',
  'comment_id_2',
]);
```

### Subreddit Information

```javascript
// Get subreddit details
const subreddit = await api.fetchSubreddit('golang');
console.log('r/' + subreddit.display_name);
console.log('Subscribers:', subreddit.subscribers);
console.log('Description:', subreddit.public_description);
```

### Utility Functions

```javascript
// Format timestamps
const posted = api.formatTimestamp(1234567890);
// Output: "2 hours ago"

// Format scores
const score = api.formatScore(1234567);
// Output: "1.2M"

// Truncate text
const text = api.truncateText('Long post content...', 50);
// Output: "Long post content that is very long and should..."

// Escape HTML
const safe = api.escapeHtml('<script>alert("xss")</script>');
// Output: "&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;"

// Markdown to HTML
const html = api.markdownToHtml('**bold** and *italic*');
// Output: "<strong>bold</strong> and <em>italic</em>"

// Get thumbnail class
const cssClass = api.getThumbnailClass('https://example.com/image.jpg');
// Output: "thumbnail-image"
```

## API Reference

### Storage Management

#### `saveApiKey(key)`
Saves an API key to localStorage.

**Parameters:**
- `key` (string): The API key to store

**Throws:** Error if key is invalid

**Example:**
```javascript
try {
  api.saveApiKey('my-api-key');
} catch (error) {
  console.error('Invalid key:', error.message);
}
```

#### `getApiKey()`
Retrieves the stored API key.

**Returns:** string | null - The API key or null if not found

**Example:**
```javascript
const key = api.getApiKey();
if (key) {
  console.log('Using key:', key);
}
```

#### `clearApiKey()`
Removes the stored API key.

**Example:**
```javascript
api.clearApiKey();
console.log('Key cleared');
```

### HTTP Requests

#### `makeRequest(url, options)`
Low-level HTTP request function. Automatically includes API key in Authorization header.

**Parameters:**
- `url` (string): Relative API endpoint URL
- `options` (object):
  - `method` (string): HTTP method, default "GET"
  - `body` (object): Request body (auto-JSON encoded)
  - `retry` (boolean): Retry on network errors, default true

**Returns:** Promise<object> - Parsed JSON response

**Throws:** Error with user-friendly message

**Example:**
```javascript
try {
  const data = await api.makeRequest('/api/v1/user/me');
  console.log('User data:', data);
} catch (error) {
  console.error('Request failed:', error.message);
}
```

### Authentication

#### `checkAuth(apiKey)`
Validates an API key by making a test request.

**Parameters:**
- `apiKey` (string): The API key to validate

**Returns:** Promise<boolean> - true if valid, false otherwise

**Example:**
```javascript
const isValid = await api.checkAuth('test-key');
if (isValid) {
  console.log('Key is valid!');
  api.saveApiKey('test-key');
} else {
  console.log('Invalid key');
}
```

#### `getCurrentUser()`
Fetches the current authenticated user information.

**Returns:** Promise<object> - User data object

**Throws:** Error if authentication fails

**Example:**
```javascript
try {
  const user = await api.getCurrentUser();
  console.log('Logged in as:', user.name);
} catch (error) {
  console.error('Auth failed:', error.message);
}
```

### Posts

#### `fetchHotPosts(options)`
Fetches hot posts from frontpage or a subreddit.

**Parameters:**
- `options` (object):
  - `subreddit` (string): Subreddit name, omit for frontpage
  - `limit` (number): Posts to fetch, 1-100, default 25
  - `after` (string): Pagination cursor for next page
  - `before` (string): Pagination cursor for previous page

**Returns:** Promise<object>
- `posts` (array): Post objects
- `after` (string): Cursor for next page
- `before` (string): Cursor for previous page

**Example:**
```javascript
const result = await api.fetchHotPosts({
  subreddit: 'golang',
  limit: 50,
});
result.posts.forEach(post => {
  console.log(post.title, '(' + post.score + ' points)');
});
```

#### `fetchNewPosts(options)`
Fetches new posts from frontpage or a subreddit.

**Parameters:** Same as fetchHotPosts

**Example:**
```javascript
const result = await api.fetchNewPosts({
  subreddit: 'programming',
  limit: 25,
});
```

#### `fetchPosts(sortBy, options)`
Generic post fetching function.

**Parameters:**
- `sortBy` (string): "hot" or "new"
- `options` (object): Same as fetchHotPosts

**Returns:** Same as fetchHotPosts

### Comments

#### `fetchComments(subreddit, postId, options)`
Fetches comments for a specific post.

**Parameters:**
- `subreddit` (string): Subreddit name
- `postId` (string): Post ID (with or without t3_ prefix)
- `options` (object):
  - `limit` (number): Comments to fetch, 1-100, default 25
  - `after` (string): Pagination cursor
  - `before` (string): Pagination cursor

**Returns:** Promise<object>
- `post` (object): The post object
- `comments` (array): Comment tree objects
- `after` (string): Cursor for next page
- `before` (string): Cursor for previous page

**Example:**
```javascript
const result = await api.fetchComments('golang', 'abc123', {
  limit: 100,
});
console.log('Post:', result.post.title);
console.log('Top comment:', result.comments[0].body);
```

#### `fetchMoreComments(linkId, children)`
Loads more comments from a collapsed comment thread.

**Parameters:**
- `linkId` (string): Post link ID with t3_ prefix
- `children` (string[]): Array of comment IDs to load (1-100)

**Returns:** Promise<object>
- `comments` (array): Loaded comment objects

**Throws:** Error if validation fails

**Example:**
```javascript
const result = await api.fetchMoreComments('t3_abc123', [
  'comment1',
  'comment2',
  'comment3',
]);
console.log('Loaded', result.comments.length, 'comments');
```

### Subreddit

#### `fetchSubreddit(subreddit)`
Fetches information about a subreddit.

**Parameters:**
- `subreddit` (string): Subreddit name

**Returns:** Promise<object> - Subreddit data

**Throws:** Error if subreddit not found

**Example:**
```javascript
try {
  const sub = await api.fetchSubreddit('golang');
  console.log('r/' + sub.display_name);
  console.log('Members:', sub.subscribers);
  console.log('About:', sub.public_description);
} catch (error) {
  console.error('Not found:', error.message);
}
```

### Utilities

#### `formatTimestamp(unixTime)`
Converts Unix timestamp to human-readable relative time.

**Parameters:**
- `unixTime` (number): Unix timestamp in seconds

**Returns:** string - Relative time string

**Examples:**
```javascript
api.formatTimestamp(Date.now() / 1000 - 3600); // "1 hour ago"
api.formatTimestamp(Date.now() / 1000 - 86400); // "1 day ago"
api.formatTimestamp(Date.now() / 1000 - 604800); // "1 week ago"
```

#### `formatScore(score)`
Formats a score number with abbreviations.

**Parameters:**
- `score` (number): The score value

**Returns:** string - Formatted score

**Examples:**
```javascript
api.formatScore(5);          // "5"
api.formatScore(1000);       // "1k"
api.formatScore(1234567);    // "1.2M"
api.formatScore(1234567890); // "1.2B"
```

#### `truncateText(text, maxLength)`
Truncates text to maximum length with ellipsis.

**Parameters:**
- `text` (string): Text to truncate
- `maxLength` (number): Maximum length, default 100

**Returns:** string - Truncated text

**Example:**
```javascript
api.truncateText('This is a very long text', 10);
// Output: "This is a..."
```

#### `escapeHtml(text)`
Escapes HTML special characters to prevent XSS.

**Parameters:**
- `text` (string): Text to escape

**Returns:** string - HTML-escaped text

**Example:**
```javascript
api.escapeHtml('<script>alert("xss")</script>');
// Output: "&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;"
```

#### `markdownToHtml(markdown)`
Converts simple Reddit markdown to HTML.

**Parameters:**
- `markdown` (string): Markdown text

**Returns:** string - HTML string

**Supports:**
- **bold** -> <strong>bold</strong>
- *italic* -> <em>italic</em>
- `code` -> <code>code</code>
- Line breaks -> <br/>

**Example:**
```javascript
api.markdownToHtml('**Bold** and *italic* with `code`');
// Output: "<strong>Bold</strong> and <em>italic</em> with <code>code</code>"
```

#### `getThumbnailClass(thumbnail)`
Returns CSS class name for thumbnail styling.

**Parameters:**
- `thumbnail` (string): Thumbnail URL or type

**Returns:** string - CSS class name

**Returns:**
- "thumbnail-none" for empty, "self", or missing thumbnails
- "thumbnail-default" for "default" type
- "thumbnail-nsfw" for "nsfw" type
- "thumbnail-image" for image URLs

**Example:**
```javascript
api.getThumbnailClass('https://example.com/image.jpg');
// Output: "thumbnail-image"
```

### State Management

#### `createState(initialValue)`
Creates a simple reactive state object for Alpine.js integration.

**Parameters:**
- `initialValue` (*): Initial state value

**Returns:** object with methods:
- `get()`: Get current value
- `set(newValue)`: Update value and notify subscribers
- `subscribe(callback)`: Watch for changes

**Example:**
```javascript
const state = createState({ posts: [], loading: false });

// Subscribe to changes
const unsubscribe = state.subscribe((newValue) => {
  console.log('State changed:', newValue);
});

// Update state
state.set({ posts: [...], loading: true });

// Get state
console.log('Current:', state.get());

// Unsubscribe
unsubscribe();
```

## Error Handling

All API functions throw errors with user-friendly messages. Use try/catch:

```javascript
try {
  const posts = await api.fetchHotPosts({ subreddit: 'golang' });
  console.log('Got posts:', posts.posts.length);
} catch (error) {
  // Common error messages:
  // - "Authentication required. Please provide a valid API key."
  // - "Rate limited. Please wait a moment before trying again."
  // - "Resource not found."
  // - "Request timed out. Please check your connection and try again."
  // - "Network error. Please check your internet connection."
  // - "Server error. Please try again later."
  console.error(error.message);
}
```

### Error Types

| Error Message | Status Code | Cause |
|---|---|---|
| "Authentication required..." | 401 | API key invalid, cleared from storage |
| "Rate limited..." | 429 | Too many requests, wait before retry |
| "Resource not found." | 404 | Subreddit/post doesn't exist |
| "Invalid request parameters." | 400 | Bad query parameters |
| "Request timed out..." | (timeout) | 30-second timeout exceeded |
| "Network error..." | (network) | Connection failed |
| "Server error..." | 500+ | Server-side error |

## Alpine.js Integration

The client is designed to work seamlessly with Alpine.js:

```html
<div x-data="reddit()" @load="init()">
  <input x-model="apiKey" placeholder="API Key">
  <button @click="loginWithKey()">Login</button>
  
  <div x-show="loggedIn">
    <p>Hello, <span x-text="user.name"></span>!</p>
    
    <button @click="loadHotPosts('golang')">Load Posts</button>
    <div x-show="loading">Loading...</div>
    
    <div x-show="error" class="error" x-text="error"></div>
    
    <ul>
      <template x-for="post in posts" :key="post.id">
        <li>
          <h3 x-text="post.title"></h3>
          <p x-text="api.formatScore(post.score) + ' points'"></p>
          <p x-text="api.formatTimestamp(post.created_utc)"></p>
        </li>
      </template>
    </ul>
  </div>
</div>

<script>
function reddit() {
  return {
    apiKey: '',
    loggedIn: false,
    loading: false,
    error: '',
    user: {},
    posts: [],
    
    async init() {
      const key = api.getApiKey();
      if (key) {
        this.apiKey = key;
        await this.checkLogin();
      }
    },
    
    async loginWithKey() {
      try {
        this.loading = true;
        const isValid = await api.checkAuth(this.apiKey);
        if (isValid) {
          api.saveApiKey(this.apiKey);
          this.user = await api.getCurrentUser();
          this.loggedIn = true;
          this.error = '';
        } else {
          this.error = 'Invalid API key';
        }
      } catch (err) {
        this.error = err.message;
      } finally {
        this.loading = false;
      }
    },
    
    async loadHotPosts(subreddit) {
      try {
        this.loading = true;
        this.error = '';
        const result = await api.fetchHotPosts({
          subreddit,
          limit: 25,
        });
        this.posts = result.posts;
      } catch (err) {
        this.error = err.message;
      } finally {
        this.loading = false;
      }
    },
  };
}
</script>
```

## Configuration

Modify the CONFIG object at the top of app.js to customize behavior:

```javascript
const CONFIG = {
  API_BASE_URL: window.location.origin,    // API server URL
  API_KEY_STORAGE: 'reddit_api_key',       // localStorage key
  REQUEST_TIMEOUT: 30000,                  // Request timeout (ms)
  MAX_RETRIES: 3,                          // Max retry attempts
};
```

## Performance Considerations

- **Caching**: Implement your own caching layer if needed
- **Pagination**: Use the `after`/`before` cursors for efficient pagination
- **Rate Limiting**: Respect Reddit's rate limits; monitor response headers
- **Bundle Size**: The unminified file is ~19KB (suitable for inlining)

## Browser Support

- Modern browsers (ES6+)
- Chrome 55+
- Firefox 52+
- Safari 10.1+
- Edge 15+

Note: Requires `fetch` API and `localStorage` support.

## Production Checklist

- [ ] Minify app.js before deploying
- [ ] Set ALLOWED_ORIGINS in server config for CORS
- [ ] Test error handling for all endpoints
- [ ] Monitor rate limit responses
- [ ] Implement proper error UI in your frontend
- [ ] Validate and sanitize user input before API calls
- [ ] Use HTTPS in production
- [ ] Set Content Security Policy headers
- [ ] Test on target browsers

## Troubleshooting

### "Authentication required" error after saving key
- The API key is invalid or expired
- Server responded with 401 status
- Key is automatically cleared from storage

### "Rate limited" error
- Too many requests to the API
- Wait a moment (usually 1-2 seconds)
- Implement exponential backoff in your app

### "Request timed out" error
- Server not responding within 30 seconds
- Check network connection
- Verify server is running

### Functions not available on window.api
- Ensure app.js is loaded before your code runs
- Check browser console for errors
- Verify script src path is correct

## License

Part of the go-reddit-api-wrapper project. See repository root for license.
