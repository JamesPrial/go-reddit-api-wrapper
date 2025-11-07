# API Client Usage Examples

Practical examples for using the Reddit API client in your frontend.

## Table of Contents

1. [Basic Setup](#basic-setup)
2. [Authentication](#authentication)
3. [Fetching Posts](#fetching-posts)
4. [Working with Comments](#working-with-comments)
5. [Subreddit Information](#subreddit-information)
6. [Error Handling](#error-handling)
7. [Alpine.js Integration](#alpinejs-integration)
8. [Complete Application](#complete-application)

## Basic Setup

Include the script in your HTML:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Reddit Browser</title>
</head>
<body>
  <script src="/static/app.js"></script>
  <script>
    // All functions available via window.api
    console.log(window.api);
  </script>
</body>
</html>
```

## Authentication

### Save and Retrieve API Key

```javascript
// Save the API key when user logs in
function saveUserKey(key) {
  api.saveApiKey(key);
  console.log('API key saved');
}

// Retrieve stored key on page load
function loadStoredKey() {
  const key = api.getApiKey();
  if (key) {
    console.log('Found stored key');
    return true;
  }
  return false;
}

// Clear key on logout
function logout() {
  api.clearApiKey();
  console.log('Logged out');
}
```

### Validate API Key

```javascript
async function validateAndLogin(providedKey) {
  try {
    // Check if the key is valid
    const isValid = await api.checkAuth(providedKey);
    
    if (!isValid) {
      console.log('Invalid API key');
      return false;
    }
    
    // Save the key
    api.saveApiKey(providedKey);
    
    // Fetch user info to confirm
    const user = await api.getCurrentUser();
    console.log('Logged in as:', user.name);
    console.log('Link karma:', user.link_karma);
    console.log('Comment karma:', user.comment_karma);
    
    return true;
  } catch (error) {
    console.error('Login failed:', error.message);
    return false;
  }
}
```

## Fetching Posts

### Get Hot Posts from Frontpage

```javascript
async function getHotPosts() {
  try {
    const result = await api.fetchHotPosts({
      limit: 25,
    });
    
    console.log('Found', result.posts.length, 'posts');
    
    result.posts.forEach(post => {
      console.log({
        title: post.title,
        subreddit: post.subreddit,
        score: api.formatScore(post.score),
        comments: post.num_comments,
        posted: api.formatTimestamp(post.created_utc),
        author: post.author,
      });
    });
    
    return result;
  } catch (error) {
    console.error('Failed to fetch posts:', error.message);
  }
}
```

### Get Posts from Specific Subreddit

```javascript
async function getSubredditPosts(subredditName) {
  try {
    const result = await api.fetchHotPosts({
      subreddit: subredditName,
      limit: 50,
    });
    
    console.log('Posts from r/' + subredditName + ':');
    result.posts.forEach(post => {
      console.log('- ' + post.title);
    });
    
    return result;
  } catch (error) {
    console.error('Failed to fetch r/' + subredditName + ':', error.message);
  }
}

// Usage
getSubredditPosts('golang');
getSubredditPosts('programming');
getSubredditPosts('learnprogramming');
```

### Paginate Through Posts

```javascript
async function getAllPostsFromSubreddit(subredditName, maxPages = 3) {
  const allPosts = [];
  let after = '';
  let pageCount = 0;
  
  try {
    while (pageCount < maxPages) {
      console.log('Fetching page', pageCount + 1);
      
      const result = await api.fetchHotPosts({
        subreddit: subredditName,
        limit: 25,
        after: after,
      });
      
      allPosts.push(...result.posts);
      
      if (!result.after) {
        console.log('Reached end of results');
        break;
      }
      
      after = result.after;
      pageCount++;
      
      // Be nice to the server - wait a bit between requests
      await new Promise(resolve => setTimeout(resolve, 500));
    }
    
    console.log('Total posts:', allPosts.length);
    return allPosts;
  } catch (error) {
    console.error('Error fetching posts:', error.message);
  }
}
```

### Compare Hot vs New Posts

```javascript
async function comparePostSorts(subredditName) {
  try {
    const [hotResult, newResult] = await Promise.all([
      api.fetchHotPosts({ subreddit: subredditName, limit: 10 }),
      api.fetchNewPosts({ subreddit: subredditName, limit: 10 }),
    ]);
    
    console.log('Hot posts from r/' + subredditName + ':');
    hotResult.posts.forEach(post => {
      console.log('- ' + post.title + ' (' + post.score + ' points)');
    });
    
    console.log('\nNew posts from r/' + subredditName + ':');
    newResult.posts.forEach(post => {
      console.log('- ' + post.title + ' (' + post.score + ' points)');
    });
  } catch (error) {
    console.error('Error:', error.message);
  }
}
```

## Working with Comments

### Get Comments for a Post

```javascript
async function getPostComments(subredditName, postId) {
  try {
    const result = await api.fetchComments(subredditName, postId, {
      limit: 100,
    });
    
    console.log('Post: ' + result.post.title);
    console.log('Posted: ' + api.formatTimestamp(result.post.created_utc));
    console.log('Score: ' + api.formatScore(result.post.score));
    console.log('\nComments:');
    
    function printComments(comments, depth = 0) {
      const indent = '  '.repeat(depth);
      comments.forEach(comment => {
        console.log(
          indent + '- ' +
          comment.author + ' (' + api.formatScore(comment.score) + ' points)\n' +
          indent + '  ' + api.truncateText(comment.body, 60)
        );
        if (comment.replies && comment.replies.length > 0) {
          printComments(comment.replies, depth + 1);
        }
      });
    }
    
    printComments(result.comments);
    return result;
  } catch (error) {
    console.error('Failed to fetch comments:', error.message);
  }
}

// Usage
getPostComments('golang', 'abc123');
```

### Load More Comments

```javascript
async function expandCommentThread(postLinkId, commentIds) {
  try {
    console.log('Loading', commentIds.length, 'more comments...');
    
    const result = await api.fetchMoreComments(postLinkId, commentIds);
    
    console.log('Loaded', result.comments.length, 'comments');
    result.comments.forEach(comment => {
      console.log(comment.author + ': ' + api.truncateText(comment.body, 50));
    });
    
    return result;
  } catch (error) {
    console.error('Failed to load more comments:', error.message);
  }
}

// Usage - load 3 specific comments
expandCommentThread('t3_abc123', ['comment1', 'comment2', 'comment3']);
```

### Find Comments by Author

```javascript
async function findCommentsByAuthor(subredditName, postId, authorName) {
  try {
    const result = await api.fetchComments(subredditName, postId, {
      limit: 100,
    });
    
    const authorComments = [];
    
    function searchComments(comments) {
      comments.forEach(comment => {
        if (comment.author === authorName) {
          authorComments.push(comment);
        }
        if (comment.replies && comment.replies.length > 0) {
          searchComments(comment.replies);
        }
      });
    }
    
    searchComments(result.comments);
    
    console.log('Found', authorComments.length, 'comments by', authorName);
    authorComments.forEach(comment => {
      console.log(
        '- ' + api.formatScore(comment.score) + ' points\n' +
        api.truncateText(comment.body, 70)
      );
    });
    
    return authorComments;
  } catch (error) {
    console.error('Error searching comments:', error.message);
  }
}
```

## Subreddit Information

### Get Subreddit Details

```javascript
async function getSubredditInfo(subredditName) {
  try {
    const subreddit = await api.fetchSubreddit(subredditName);
    
    console.log('r/' + subreddit.display_name);
    console.log('Title: ' + subreddit.title);
    console.log('Description: ' + subreddit.public_description);
    console.log('Subscribers: ' + api.formatScore(subreddit.subscribers));
    console.log('Active now: ' + subreddit.active_user_count);
    console.log('Created: ' + api.formatTimestamp(subreddit.created_utc));
    console.log('NSFW: ' + subreddit.over18);
    
    return subreddit;
  } catch (error) {
    console.error('Failed to fetch subreddit:', error.message);
  }
}

// Usage
getSubredditInfo('golang');
getSubredditInfo('programming');
```

### Compare Multiple Subreddits

```javascript
async function compareSubreddits(subredditNames) {
  try {
    const subreddits = await Promise.all(
      subredditNames.map(name => api.fetchSubreddit(name))
    );
    
    console.log('Subreddit Comparison:');
    console.log('| Subreddit | Subscribers | Active |');
    console.log('|-----------|-------------|--------|');
    
    subreddits.forEach(sub => {
      console.log(
        '| r/' + sub.display_name + 
        ' | ' + api.formatScore(sub.subscribers) + 
        ' | ' + sub.active_user_count + ' |'
      );
    });
    
    return subreddits;
  } catch (error) {
    console.error('Error comparing subreddits:', error.message);
  }
}

// Usage
compareSubreddits(['golang', 'programming', 'learnprogramming', 'javascript']);
```

## Error Handling

### Comprehensive Error Handling

```javascript
async function robustApiCall(operation, operationName) {
  const maxRetries = 3;
  let attempt = 0;
  
  while (attempt < maxRetries) {
    try {
      console.log('Attempt', attempt + 1, 'of', maxRetries);
      return await operation();
    } catch (error) {
      attempt++;
      
      if (error.message.includes('Rate limited')) {
        const waitTime = 2000 * attempt; // Exponential backoff
        console.log('Rate limited. Waiting', waitTime, 'ms before retry...');
        await new Promise(resolve => setTimeout(resolve, waitTime));
      } else if (error.message.includes('Network error')) {
        const waitTime = 1000 * attempt;
        console.log('Network error. Waiting', waitTime, 'ms before retry...');
        await new Promise(resolve => setTimeout(resolve, waitTime));
      } else if (error.message.includes('Authentication required')) {
        console.error('Authentication failed. Please log in again.');
        return null;
      } else if (error.message.includes('Resource not found')) {
        console.error('Resource not found. Please check the ID/name.');
        return null;
      } else {
        console.error('Error:', error.message);
        if (attempt >= maxRetries) {
          console.error('Max retries reached. Giving up.');
          return null;
        }
      }
    }
  }
}

// Usage
async function safeGetPosts() {
  return robustApiCall(
    () => api.fetchHotPosts({ subreddit: 'golang', limit: 25 }),
    'fetch hot posts from golang'
  );
}
```

## Alpine.js Integration

### Simple Alpine.js Example

```html
<div x-data="redditApp()">
  <!-- Login Section -->
  <div x-show="!loggedIn">
    <h2>Login</h2>
    <input x-model="apiKey" type="password" placeholder="Enter API Key">
    <button @click="login()">Login</button>
    <p x-show="error" style="color: red;" x-text="error"></p>
  </div>
  
  <!-- Main App -->
  <div x-show="loggedIn">
    <h2>Welcome, <span x-text="user.name"></span>!</h2>
    
    <div>
      <input x-model="subreddit" placeholder="Enter subreddit name">
      <button @click="loadPosts()">Load Posts</button>
      <button @click="logout()">Logout</button>
    </div>
    
    <p x-show="loading">Loading...</p>
    <p x-show="error" style="color: red;" x-text="error"></p>
    
    <div x-show="posts.length > 0">
      <h3 x-text="'Posts from r/' + subreddit"></h3>
      <ul>
        <template x-for="post in posts" :key="post.id">
          <li style="margin: 20px 0; padding: 10px; border: 1px solid #ccc;">
            <h4 x-text="post.title"></h4>
            <p>
              <span x-text="'By ' + post.author"></span> •
              <span x-text="api.formatScore(post.score) + ' points'"></span> •
              <span x-text="api.formatTimestamp(post.created_utc)"></span>
            </p>
            <p x-text="post.num_comments + ' comments'"></p>
          </li>
        </template>
      </ul>
    </div>
  </div>
</div>

<script>
function redditApp() {
  return {
    apiKey: '',
    loggedIn: false,
    loading: false,
    error: '',
    user: {},
    subreddit: 'golang',
    posts: [],
    
    async login() {
      try {
        this.loading = true;
        this.error = '';
        
        const isValid = await api.checkAuth(this.apiKey);
        if (!isValid) {
          this.error = 'Invalid API key';
          return;
        }
        
        api.saveApiKey(this.apiKey);
        this.user = await api.getCurrentUser();
        this.loggedIn = true;
        
        await this.loadPosts();
      } catch (error) {
        this.error = error.message;
      } finally {
        this.loading = false;
      }
    },
    
    async loadPosts() {
      try {
        this.loading = true;
        this.error = '';
        
        const result = await api.fetchHotPosts({
          subreddit: this.subreddit,
          limit: 25,
        });
        
        this.posts = result.posts;
      } catch (error) {
        this.error = error.message;
      } finally {
        this.loading = false;
      }
    },
    
    logout() {
      api.clearApiKey();
      this.loggedIn = false;
      this.posts = [];
      this.user = {};
    },
  };
}
</script>
```

## Complete Application

### Full Reddit Browser Application

```html
<!DOCTYPE html>
<html>
<head>
  <title>Reddit Browser</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
    .container { max-width: 1000px; margin: 0 auto; padding: 20px; }
    
    .header {
      background: #0b1419;
      color: white;
      padding: 20px;
      margin-bottom: 20px;
      border-radius: 4px;
    }
    
    .login-form {
      max-width: 400px;
      margin: 0 auto;
    }
    
    .input-group {
      margin: 10px 0;
      display: flex;
      gap: 10px;
    }
    
    input, button {
      padding: 10px;
      border: 1px solid #ccc;
      border-radius: 4px;
      font-size: 14px;
    }
    
    button {
      background: #0079d3;
      color: white;
      cursor: pointer;
      border: none;
    }
    
    button:hover { background: #0066b3; }
    
    .error { color: #d32f2f; margin: 10px 0; }
    .success { color: #388e3c; margin: 10px 0; }
    
    .posts { display: grid; gap: 15px; }
    
    .post {
      border: 1px solid #ccc;
      border-radius: 4px;
      padding: 15px;
      transition: box-shadow 0.2s;
    }
    
    .post:hover { box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
    
    .post-title {
      font-size: 18px;
      font-weight: bold;
      margin-bottom: 10px;
      color: #0079d3;
    }
    
    .post-meta {
      font-size: 12px;
      color: #999;
      margin-bottom: 10px;
    }
    
    .post-meta span { margin-right: 15px; }
    
    .controls {
      margin: 20px 0;
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
    }
    
    input[type="text"] { flex: 1; }
    button { min-width: 100px; }
    
    .loading { text-align: center; padding: 20px; color: #999; }
  </style>
</head>
<body>
  <div x-data="redditBrowser()" @load="init()" class="container">
    <!-- Header -->
    <div class="header">
      <h1>Reddit Browser</h1>
      <p x-show="loggedIn" x-text="'Logged in as ' + user.name"></p>
    </div>
    
    <!-- Login Section -->
    <div x-show="!loggedIn" class="login-form">
      <h2>Sign In</h2>
      <div x-show="error" class="error" x-text="error"></div>
      <div class="input-group">
        <input x-model="apiKey" type="password" placeholder="API Key">
        <button @click="login()" x-bind:disabled="loading">Sign In</button>
      </div>
      <p style="font-size: 12px; color: #999; margin-top: 10px;">
        Get your API key from the reddit-server API endpoint.
      </p>
    </div>
    
    <!-- Main App -->
    <div x-show="loggedIn">
      <!-- Controls -->
      <div class="controls">
        <input x-model="subreddit" placeholder="Enter subreddit (or leave empty for frontpage)">
        <button @click="loadHotPosts()" x-bind:disabled="loading">Hot Posts</button>
        <button @click="loadNewPosts()" x-bind:disabled="loading">New Posts</button>
        <button @click="logout()">Logout</button>
      </div>
      
      <!-- Loading and Error States -->
      <div x-show="loading" class="loading">
        Loading posts...
      </div>
      <div x-show="error" class="error" x-text="error"></div>
      
      <!-- Posts -->
      <div x-show="!loading && posts.length > 0" class="posts">
        <template x-for="post in posts" :key="post.id">
          <div class="post">
            <div class="post-title" x-text="post.title"></div>
            <div class="post-meta">
              <span x-text="'r/' + post.subreddit"></span>
              <span x-text="'by ' + post.author"></span>
              <span x-text="api.formatScore(post.score) + ' points'"></span>
              <span x-text="post.num_comments + ' comments'"></span>
              <span x-text="api.formatTimestamp(post.created_utc)"></span>
            </div>
            <button @click="viewComments(post)" style="width: auto;">
              View Comments
            </button>
          </div>
        </template>
      </div>
      
      <div x-show="!loading && posts.length === 0" style="text-align: center; color: #999;">
        No posts found. Try a different subreddit or sort.
      </div>
    </div>
  </div>
  
  <script src="/static/app.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js" defer></script>
  
  <script>
    function redditBrowser() {
      return {
        apiKey: '',
        loggedIn: false,
        loading: false,
        error: '',
        user: {},
        subreddit: '',
        posts: [],
        sortBy: 'hot',
        
        async init() {
          // Check if user is already logged in
          const key = api.getApiKey();
          if (key) {
            this.apiKey = key;
            try {
              this.user = await api.getCurrentUser();
              this.loggedIn = true;
              await this.loadHotPosts();
            } catch (error) {
              this.error = 'Session expired. Please log in again.';
              api.clearApiKey();
            }
          }
        },
        
        async login() {
          try {
            this.loading = true;
            this.error = '';
            
            const isValid = await api.checkAuth(this.apiKey);
            if (!isValid) {
              this.error = 'Invalid API key. Please try again.';
              return;
            }
            
            api.saveApiKey(this.apiKey);
            this.user = await api.getCurrentUser();
            this.loggedIn = true;
            
            await this.loadHotPosts();
          } catch (error) {
            this.error = error.message;
          } finally {
            this.loading = false;
          }
        },
        
        async loadHotPosts() {
          this.sortBy = 'hot';
          await this.loadPosts();
        },
        
        async loadNewPosts() {
          this.sortBy = 'new';
          await this.loadPosts();
        },
        
        async loadPosts() {
          try {
            this.loading = true;
            this.error = '';
            
            const method = this.sortBy === 'hot' 
              ? api.fetchHotPosts 
              : api.fetchNewPosts;
            
            const result = await method({
              subreddit: this.subreddit,
              limit: 25,
            });
            
            this.posts = result.posts;
            
            if (this.posts.length === 0) {
              this.error = 'No posts found';
            }
          } catch (error) {
            this.error = error.message;
          } finally {
            this.loading = false;
          }
        },
        
        async viewComments(post) {
          try {
            this.loading = true;
            const result = await api.fetchComments(
              post.subreddit,
              post.id,
              { limit: 50 }
            );
            
            console.log('Post:', result.post.title);
            console.log('Comments:', result.comments);
            alert('Comments logged to console. Check browser developer tools.');
          } catch (error) {
            this.error = error.message;
          } finally {
            this.loading = false;
          }
        },
        
        logout() {
          api.clearApiKey();
          this.loggedIn = false;
          this.posts = [];
          this.user = {};
          this.error = '';
        },
      };
    }
  </script>
</body>
</html>
```

## Tips and Best Practices

1. **Always use try/catch** with async API calls
2. **Implement exponential backoff** for rate limiting
3. **Cache results** to reduce API calls
4. **Use pagination** for large result sets
5. **Validate input** before making API calls
6. **Show loading states** to users during API calls
7. **Handle errors gracefully** with user-friendly messages
8. **Test in development** before deploying to production
9. **Monitor rate limits** and adjust request frequency
10. **Use HTTPS** in production

