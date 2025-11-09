/**
 * Reddit API Client for HTTP Server
 *
 * A production-ready JavaScript API client for the reddit-server HTTP server.
 * Provides functions for authentication, posts, comments, and user data retrieval.
 * Designed to work with vanilla JavaScript and Alpine.js.
 *
 * All functions are globally accessible and use async/await for API calls.
 * Errors are thrown with user-friendly messages that can be displayed directly.
 */

// Configuration
const CONFIG = {
  API_BASE_URL: window.location.origin,
  JWT_TOKEN_STORAGE: 'jwt_token',
  JWT_EXPIRES_STORAGE: 'jwt_expires_at',
  USER_INFO_STORAGE: 'user_info',
  REQUEST_TIMEOUT: 30000, // 30 seconds
  MAX_RETRIES: 3,
};

/**
 * Authentication State Management
 */
const auth = {
  token: localStorage.getItem(CONFIG.JWT_TOKEN_STORAGE),
  expiresAt: localStorage.getItem(CONFIG.JWT_EXPIRES_STORAGE),
  user: JSON.parse(localStorage.getItem(CONFIG.USER_INFO_STORAGE) || 'null'),

  /**
   * Check if the user is authenticated with a valid token
   * @returns {boolean} True if authenticated and token not expired
   */
  isAuthenticated() {
    if (!this.token || !this.expiresAt) return false;
    return new Date(this.expiresAt) > new Date();
  },

  /**
   * Set authentication data in memory and localStorage
   * @param {string} token - JWT token
   * @param {string} expiresAt - ISO 8601 expiration timestamp
   * @param {object} user - User information object
   */
  setAuth(token, expiresAt, user) {
    this.token = token;
    this.expiresAt = expiresAt;
    this.user = user;
    localStorage.setItem(CONFIG.JWT_TOKEN_STORAGE, token);
    localStorage.setItem(CONFIG.JWT_EXPIRES_STORAGE, expiresAt);
    localStorage.setItem(CONFIG.USER_INFO_STORAGE, JSON.stringify(user));
  },

  /**
   * Clear authentication data from memory and localStorage
   */
  clearAuth() {
    this.token = null;
    this.expiresAt = null;
    this.user = null;
    localStorage.removeItem(CONFIG.JWT_TOKEN_STORAGE);
    localStorage.removeItem(CONFIG.JWT_EXPIRES_STORAGE);
    localStorage.removeItem(CONFIG.USER_INFO_STORAGE);
  }
};

/**
 * HTTP Request Handling
 */

/**
 * Creates an AbortController with automatic timeout.
 * @param {number} timeout - Timeout in milliseconds
 * @returns {object} Object with controller and promise
 */
function createAbortTimeout(timeout) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeout);
  return { controller, timeoutId };
}

/**
 * Makes an HTTP request to the API server with automatic error handling.
 * Includes authorization header, timeout, and retry logic for network errors.
 *
 * @param {string} url - The endpoint URL (relative to API base)
 * @param {object} options - Request options
 * @param {string} options.method - HTTP method (GET, POST, etc.)
 * @param {object} options.body - Request body (will be JSON encoded)
 * @param {boolean} options.retry - Whether to retry on network errors (default: true)
 * @param {object} options.headers - Custom headers to include in the request
 * @returns {Promise<object>} Parsed JSON response
 * @throws {Error} User-friendly error message
 */
async function makeRequest(url, options = {}) {
  const {
    method = 'GET',
    body = null,
    retry = true,
    headers: customHeaders = {},
  } = options;

  // Auto-refresh token if expiring soon
  if (shouldRefreshToken()) {
    try {
      await refreshToken();
    } catch (error) {
      console.error('Auto token refresh failed:', error);
      // Continue with existing token, will fail with 401 if truly expired
    }
  }

  const fullUrl = CONFIG.API_BASE_URL + url;

  const { controller, timeoutId } = createAbortTimeout(CONFIG.REQUEST_TIMEOUT);

  try {
    const headers = {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      ...customHeaders,
    };

    // Add JWT token header if authenticated and not overridden
    const token = auth.token;
    if (token && auth.isAuthenticated()) {
      headers['Authorization'] = 'Bearer ' + token;
    }

    const fetchOptions = {
      method,
      headers,
      signal: controller.signal,
    };

    if (body) {
      fetchOptions.body = JSON.stringify(body);
    }

    const response = await fetch(fullUrl, fetchOptions);

    // Handle 401 - clear auth and require login
    if (response.status === 401) {
      auth.clearAuth();
      throw new Error('Authentication required. Please log in again.');
    }

    // Handle rate limiting
    if (response.status === 429) {
      throw new Error('Rate limited. Please wait a moment before trying again.');
    }

    // Handle not found
    if (response.status === 404) {
      throw new Error('Resource not found.');
    }

    // Handle conflict (e.g., monitor already running)
    if (response.status === 409) {
      const error = await response.json().catch(() => null);
      const message = error?.error || 'Conflict: resource already exists.';
      throw new Error(message);
    }

    // Handle bad request
    if (response.status === 400) {
      const error = await response.json().catch(() => null);
      const message = error?.error || 'Invalid request parameters.';
      throw new Error(message);
    }

    // Handle server errors
    if (response.status >= 500) {
      throw new Error('Server error. Please try again later.');
    }

    // Handle other error status codes
    if (!response.ok) {
      const error = await response.json().catch(() => null);
      const message = error?.error || ('Request failed with status ' + response.status);
      throw new Error(message);
    }

    // Parse and return response
    try {
      return await response.json();
    } catch (e) {
      throw new Error('Invalid response from server');
    }
  } catch (error) {
    clearTimeout(timeoutId);

    // Handle abort (timeout)
    if (error.name === 'AbortError') {
      throw new Error('Request timed out. Please check your connection and try again.');
    }

    // Handle network errors
    if (error instanceof TypeError && error.message.includes('fetch')) {
      if (retry && Math.random() < 0.5) {
        // Simple exponential backoff: retry once with delay
        await new Promise(resolve => setTimeout(resolve, 1000));
        return makeRequest(url, Object.assign({}, options, { retry: false }));
      }
      throw new Error('Network error. Please check your internet connection.');
    }

    // Re-throw known errors
    throw error;
  }
}

/**
 * Authentication Functions
 */

/**
 * Login with username and password to obtain JWT token.
 * @param {string} username - The username
 * @param {string} password - The password
 * @returns {Promise<object>} Authentication response with token, expires_at, and user info
 * @throws {Error} If login fails
 */
async function login(username, password) {
  try {
    const response = await fetch(CONFIG.API_BASE_URL + '/api/v1/auth/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => null);
      throw new Error(error?.error || 'Login failed');
    }

    const data = await response.json();
    auth.setAuth(data.token, data.expires_at, data.user);

    return data;
  } catch (error) {
    console.error('Login error:', error);
    throw error;
  }
}

/**
 * Logout and clear authentication data.
 * @returns {Promise<void>}
 */
async function logout() {
  try {
    if (auth.isAuthenticated()) {
      await fetch(CONFIG.API_BASE_URL + '/api/v1/auth/logout', {
        method: 'POST',
        headers: {
          'Authorization': 'Bearer ' + auth.token,
        },
      });
    }
  } catch (error) {
    console.error('Logout error:', error);
  } finally {
    auth.clearAuth();
  }
}

/**
 * Refresh the JWT token to extend the session.
 * @returns {Promise<object>} New authentication data
 * @throws {Error} If token refresh fails
 */
async function refreshToken() {
  try {
    const response = await fetch(CONFIG.API_BASE_URL + '/api/v1/auth/refresh', {
      method: 'POST',
      headers: {
        'Authorization': 'Bearer ' + auth.token,
      },
    });

    if (!response.ok) {
      throw new Error('Token refresh failed');
    }

    const data = await response.json();
    auth.setAuth(data.token, data.expires_at, auth.user);

    return data;
  } catch (error) {
    console.error('Token refresh error:', error);
    auth.clearAuth();
    throw error;
  }
}

/**
 * Check if token needs refresh (expires in <5 minutes)
 * @returns {boolean} True if token should be refreshed
 */
function shouldRefreshToken() {
  if (!auth.isAuthenticated()) return false;
  if (!auth.expiresAt) return false;

  const expiresAt = new Date(auth.expiresAt);
  const now = new Date();
  const expiresInMs = expiresAt - now;
  const fiveMinutes = 5 * 60 * 1000;

  return expiresInMs > 0 && expiresInMs < fiveMinutes;
}

/**
 * Checks if the provided API key is valid by making a test request.
 * @deprecated Use login() instead for JWT authentication
 * @param {string} apiKey - The API key to validate
 * @returns {Promise<boolean>} True if the key is valid, false otherwise
 */
async function checkAuth(apiKey) {
  try {
    // Temporarily set the key to test it
    const originalToken = auth.token;
    auth.token = apiKey;
    localStorage.setItem(CONFIG.JWT_TOKEN_STORAGE, apiKey);

    // Try to fetch user info to validate the key
    const response = await makeRequest('/api/v1/user/me', {
      method: 'GET',
    });

    return !!response;
  } catch (error) {
    // Restore original token on failure
    if (originalToken) {
      auth.token = originalToken;
      localStorage.setItem(CONFIG.JWT_TOKEN_STORAGE, originalToken);
    } else {
      auth.clearAuth();
    }
    return false;
  }
}

/**
 * Gets the current authenticated user information.
 * @returns {Promise<object>} User data (id, name, link_karma, comment_karma, etc.)
 * @throws {Error} If authentication fails or user not configured
 */
async function getCurrentUser() {
  return makeRequest('/api/v1/user/me', {
    method: 'GET',
  });
}

/**
 * Posts Functions
 */

/**
 * Fetches posts from a subreddit or the frontpage.
 * @param {string} sortBy - Sort order: 'hot' or 'new'
 * @param {object} options - Request options
 * @param {string} options.subreddit - Subreddit name (omit for frontpage)
 * @param {number} options.limit - Number of posts to fetch (1-100, default 25)
 * @param {string} options.after - Pagination cursor for next page
 * @param {string} options.before - Pagination cursor for previous page
 * @returns {Promise<object>} Object with posts array and pagination cursors
 * @throws {Error} If request fails
 */
async function fetchPosts(sortBy, options) {
  options = options || {};
  const subreddit = options.subreddit || '';
  const limit = options.limit || 25;
  const after = options.after || '';
  const before = options.before || '';

  if (!['hot', 'new'].includes(sortBy)) {
    throw new Error('Invalid sort order. Use "hot" or "new".');
  }

  if (limit < 1 || limit > 100) {
    throw new Error('Limit must be between 1 and 100.');
  }

  const params = new URLSearchParams();
  if (subreddit) params.append('subreddit', subreddit);
  params.append('limit', limit.toString());
  if (after) params.append('after', after);
  if (before) params.append('before', before);

  const url = '/api/v1/posts/' + sortBy + '?' + params.toString();

  const response = await makeRequest(url, {
    method: 'GET',
  });

  return {
    posts: response.posts || [],
    after: response.after || '',
    before: response.before || '',
  };
}

/**
 * Fetches hot posts from a subreddit or frontpage.
 * @param {object} options - Request options (subreddit, limit, pagination)
 * @returns {Promise<object>} Posts and pagination data
 */
async function fetchHotPosts(options) {
  return fetchPosts('hot', options);
}

/**
 * Fetches new posts from a subreddit or frontpage.
 * @param {object} options - Request options (subreddit, limit, pagination)
 * @returns {Promise<object>} Posts and pagination data
 */
async function fetchNewPosts(options) {
  return fetchPosts('new', options);
}

/**
 * Comments Functions
 */

/**
 * Fetches comments for a specific post.
 * @param {string} subreddit - Subreddit name
 * @param {string} postId - Post ID without prefix (e.g., "abc123", not "t3_abc123")
 * @param {object} options - Request options
 * @param {number} options.limit - Number of comments (1-100, default 25)
 * @param {string} options.after - Pagination cursor
 * @param {string} options.before - Pagination cursor
 * @returns {Promise<object>} Object with post, comments array, and pagination
 * @throws {Error} If request fails
 */
async function fetchComments(subreddit, postId, options) {
  if (!subreddit || typeof subreddit !== 'string') {
    throw new Error('Subreddit name is required.');
  }

  if (!postId || typeof postId !== 'string') {
    throw new Error('Post ID is required.');
  }

  options = options || {};
  const limit = options.limit || 25;
  const after = options.after || '';
  const before = options.before || '';

  // Remove prefix if present (t3_abc123 -> abc123)
  const cleanPostId = postId.replace(/^t3_/, '');

  if (limit < 1 || limit > 100) {
    throw new Error('Limit must be between 1 and 100.');
  }

  const params = new URLSearchParams();
  params.append('limit', limit.toString());
  if (after) params.append('after', after);
  if (before) params.append('before', before);

  const url = '/api/v1/posts/' + subreddit + '/' + cleanPostId + '/comments?' + params.toString();

  const response = await makeRequest(url, {
    method: 'GET',
  });

  return {
    post: response.post || {},
    comments: response.comments || [],
    after: response.after || '',
    before: response.before || '',
  };
}

/**
 * Expands collapsed comment threads by loading more comments.
 * Used when Reddit returns "load more comments" placeholders.
 *
 * @param {string} linkId - Post link ID with prefix (e.g., "t3_abc123")
 * @param {string[]} children - Array of comment IDs to load (1-100 IDs)
 * @returns {Promise<object>} Object with loaded comments array
 * @throws {Error} If request fails or parameters are invalid
 */
async function fetchMoreComments(linkId, children) {
  if (!linkId || typeof linkId !== 'string') {
    throw new Error('Link ID is required.');
  }

  if (!Array.isArray(children) || children.length === 0) {
    throw new Error('Children array is required and must not be empty.');
  }

  if (children.length > 100) {
    throw new Error('Maximum 100 comment IDs per request.');
  }

  // Validate each child ID
  const uniqueIds = new Set();
  for (let i = 0; i < children.length; i++) {
    const id = children[i];
    if (typeof id !== 'string' || id.length === 0 || id.length > 100) {
      throw new Error('Each comment ID must be a non-empty string (max 100 chars).');
    }
    if (uniqueIds.has(id)) {
      throw new Error('Duplicate comment IDs not allowed.');
    }
    uniqueIds.add(id);
  }

  const url = '/api/v1/posts/' + linkId + '/more-comments';

  const response = await makeRequest(url, {
    method: 'POST',
    body: {
      children: Array.from(uniqueIds),
    },
  });

  return {
    comments: response.comments || [],
  };
}

/**
 * Subreddit Functions
 */

/**
 * Fetches information about a specific subreddit.
 * @param {string} subreddit - Subreddit name
 * @returns {Promise<object>} Subreddit data (name, subscribers, description, etc.)
 * @throws {Error} If subreddit not found or request fails
 */
async function fetchSubreddit(subreddit) {
  if (!subreddit || typeof subreddit !== 'string') {
    throw new Error('Subreddit name is required.');
  }

  const url = '/api/v1/subreddit/' + subreddit;

  return makeRequest(url, {
    method: 'GET',
  });
}

/**
 * Utility Functions
 */

/**
 * Formats a Unix timestamp to a human-readable relative time string.
 * Examples: "2 hours ago", "3 days ago", "1 year ago"
 *
 * @param {number} unixTime - Unix timestamp in seconds
 * @returns {string} Human-readable relative time
 */
function formatTimestamp(unixTime) {
  if (typeof unixTime !== 'number' || unixTime <= 0) {
    return 'unknown time';
  }

  const now = Date.now() / 1000;
  const secondsAgo = now - unixTime;

  if (secondsAgo < 60) {
    return 'just now';
  }

  const minutesAgo = Math.floor(secondsAgo / 60);
  if (minutesAgo < 60) {
    return minutesAgo + ' minute' + (minutesAgo === 1 ? '' : 's') + ' ago';
  }

  const hoursAgo = Math.floor(secondsAgo / 3600);
  if (hoursAgo < 24) {
    return hoursAgo + ' hour' + (hoursAgo === 1 ? '' : 's') + ' ago';
  }

  const daysAgo = Math.floor(secondsAgo / 86400);
  if (daysAgo < 7) {
    return daysAgo + ' day' + (daysAgo === 1 ? '' : 's') + ' ago';
  }

  const weeksAgo = Math.floor(secondsAgo / 604800);
  if (weeksAgo < 4) {
    return weeksAgo + ' week' + (weeksAgo === 1 ? '' : 's') + ' ago';
  }

  const monthsAgo = Math.floor(secondsAgo / 2592000);
  if (monthsAgo < 12) {
    return monthsAgo + ' month' + (monthsAgo === 1 ? '' : 's') + ' ago';
  }

  const yearsAgo = Math.floor(secondsAgo / 31536000);
  return yearsAgo + ' year' + (yearsAgo === 1 ? '' : 's') + ' ago';
}

/**
 * Formats a score number with abbreviations for readability.
 * Examples: 1000 -> "1.0k", 1234567 -> "1.2M", 5 -> "5"
 *
 * @param {number} score - The score to format
 * @returns {string} Formatted score
 */
function formatScore(score) {
  if (typeof score !== 'number' || score < 0) {
    return '0';
  }

  if (score < 1000) {
    return score.toString();
  }

  if (score < 1000000) {
    const thousands = (score / 1000).toFixed(1);
    return thousands.replace(/\.0$/, '') + 'k';
  }

  if (score < 1000000000) {
    const millions = (score / 1000000).toFixed(1);
    return millions.replace(/\.0$/, '') + 'M';
  }

  const billions = (score / 1000000000).toFixed(1);
  return billions.replace(/\.0$/, '') + 'B';
}

/**
 * Truncates text to a maximum length and adds ellipsis.
 * @param {string} text - The text to truncate
 * @param {number} maxLength - Maximum length (default 100)
 * @returns {string} Truncated text with ellipsis if needed
 */
function truncateText(text, maxLength) {
  maxLength = maxLength || 100;
  if (!text || typeof text !== 'string') {
    return '';
  }

  if (text.length <= maxLength) {
    return text;
  }

  return text.substring(0, maxLength) + '...';
}

/**
 * Escapes HTML special characters to prevent XSS attacks.
 * @param {string} text - The text to escape
 * @returns {string} HTML-escaped text
 */
function escapeHtml(text) {
  if (!text || typeof text !== 'string') {
    return '';
  }

  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

/**
 * Converts Reddit's selftext markdown to basic HTML (simple implementation).
 * Handles: bold, italic, code blocks, line breaks
 *
 * @param {string} markdown - The markdown text to convert
 * @returns {string} HTML string
 */
function markdownToHtml(markdown) {
  if (!markdown || typeof markdown !== 'string') {
    return '';
  }

  let html = escapeHtml(markdown);

  // Bold: **text** -> <strong>text</strong>
  html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');

  // Italic: *text* -> <em>text</em>
  html = html.replace(/\*(.*?)\*/g, '<em>$1</em>');

  // Code: `text` -> <code>text</code>
  html = html.replace(/`(.*?)`/g, '<code>$1</code>');

  // Line breaks
  html = html.replace(/\n/g, '<br/>');

  return html;
}

/**
 * Gets the appropriate CSS class for a post's thumbnail.
 * @param {string} thumbnail - The thumbnail URL or type
 * @returns {string} CSS class name for styling
 */
function getThumbnailClass(thumbnail) {
  if (!thumbnail || thumbnail === '' || thumbnail === 'self') {
    return 'thumbnail-none';
  }
  if (thumbnail === 'default') {
    return 'thumbnail-default';
  }
  if (thumbnail === 'nsfw') {
    return 'thumbnail-nsfw';
  }
  return 'thumbnail-image';
}

/**
 * Storage Functions
 */

/**
 * Saves a post to storage.
 * @param {object} post - The post object to save
 * @returns {Promise<object>} Result with success flag and ID
 * @throws {Error} If request fails
 */
async function savePost(post) {
  if (!post || typeof post !== 'object') {
    throw new Error('Post object is required.');
  }

  const response = await makeRequest('/api/v1/storage/posts', {
    method: 'POST',
    body: { post: post },
  });

  return {
    success: response.success || false,
    id: response.id || '',
  };
}

/**
 * Saves comments for a post to storage.
 * @param {string} postId - The post ID
 * @param {array} comments - Array of comment objects to save
 * @returns {Promise<object>} Result with success flag and count
 * @throws {Error} If request fails
 */
async function saveComments(postId, comments) {
  if (!postId || typeof postId !== 'string') {
    throw new Error('Post ID is required.');
  }

  if (!Array.isArray(comments)) {
    throw new Error('Comments must be an array.');
  }

  const response = await makeRequest('/api/v1/storage/posts/' + postId + '/comments', {
    method: 'POST',
    body: {
      comments: comments,
    },
  });

  return {
    success: response.success || false,
    count: response.count || 0,
  };
}

/**
 * Lists saved posts with optional filtering and pagination.
 * @param {object} filters - Filter options
 * @param {string} filters.subreddit - Filter by subreddit
 * @param {string} filters.author - Filter by author
 * @param {number} filters.min_score - Minimum score filter
 * @param {string} filters.sort_by - Sort field (score, timestamp, title)
 * @param {string} filters.sort_dir - Sort direction (asc, desc)
 * @param {object} pagination - Pagination options
 * @param {number} pagination.limit - Results per page
 * @param {number} pagination.offset - Starting offset
 * @returns {Promise<object>} Object with posts array and total count
 * @throws {Error} If request fails
 */
async function listSavedPosts(filters, pagination) {
  filters = filters || {};
  pagination = pagination || {};

  const limit = pagination.limit || 25;
  const offset = pagination.offset || 0;

  if (limit < 1 || limit > 100) {
    throw new Error('Limit must be between 1 and 100.');
  }

  if (offset < 0) {
    throw new Error('Offset must be non-negative.');
  }

  const params = new URLSearchParams();
  if (filters.subreddit) params.append('subreddit', filters.subreddit);
  if (filters.author) params.append('author', filters.author);
  if (filters.min_score) params.append('min_score', filters.min_score.toString());
  if (filters.sort_by) params.append('sort_by', filters.sort_by);
  if (filters.sort_dir) params.append('sort_dir', filters.sort_dir);
  params.append('limit', limit.toString());
  params.append('offset', offset.toString());

  const url = '/api/v1/storage/posts?' + params.toString();

  const response = await makeRequest(url, {
    method: 'GET',
  });

  return {
    posts: response.posts || [],
    total: response.total || 0,
  };
}

/**
 * Gets a specific saved post by ID.
 * @param {string} postId - The post ID
 * @returns {Promise<object>} The post object
 * @throws {Error} If request fails or post not found
 */
async function getSavedPost(postId) {
  if (!postId || typeof postId !== 'string') {
    throw new Error('Post ID is required.');
  }

  return makeRequest('/api/v1/storage/posts/' + postId, {
    method: 'GET',
  });
}

/**
 * Deletes a saved post from storage.
 * @param {string} postId - The post ID to delete
 * @returns {Promise<object>} Result with success flag
 * @throws {Error} If request fails
 */
async function deleteSavedPost(postId) {
  if (!postId || typeof postId !== 'string') {
    throw new Error('Post ID is required.');
  }

  const response = await makeRequest('/api/v1/storage/posts/' + postId, {
    method: 'DELETE',
  });

  return {
    success: response.success || false,
  };
}

/**
 * Gets comments for a saved post with optional filtering.
 * @param {string} postId - The post ID
 * @param {object} options - Query options
 * @param {number} options.max_depth - Maximum comment tree depth
 * @param {string} options.sort_by - Sort field (score, timestamp)
 * @param {string} options.sort_dir - Sort direction (asc, desc)
 * @returns {Promise<object>} Object with comments array and count
 * @throws {Error} If request fails
 */
async function getSavedComments(postId, options) {
  if (!postId || typeof postId !== 'string') {
    throw new Error('Post ID is required.');
  }

  options = options || {};

  const params = new URLSearchParams();
  if (options.max_depth) params.append('max_depth', options.max_depth.toString());
  if (options.sort_by) params.append('sort_by', options.sort_by);
  if (options.sort_dir) params.append('sort_dir', options.sort_dir);

  const url = '/api/v1/storage/posts/' + postId + '/comments?' + params.toString();

  const response = await makeRequest(url, {
    method: 'GET',
  });

  return {
    comments: response.comments || [],
    count: response.count || 0,
  };
}

/**
 * Gets storage statistics.
 * @returns {Promise<object>} Statistics object with post_count, comment_count, etc.
 * @throws {Error} If request fails
 */
async function getStorageStats() {
  return makeRequest('/api/v1/storage/stats', {
    method: 'GET',
  });
}

/**
 * Bulk saves posts from a subreddit.
 * @param {string} subreddit - Subreddit name
 * @param {string} sort - Sort order (hot, new, top, controversial)
 * @param {number} limit - Number of posts to save (1-100)
 * @returns {Promise<object>} Result with success, count saved, and posts array
 * @throws {Error} If request fails or parameters are invalid
 */
async function bulkSaveFromSubreddit(subreddit, sort, limit) {
  if (!subreddit || typeof subreddit !== 'string') {
    throw new Error('Subreddit name is required.');
  }

  if (!sort || typeof sort !== 'string') {
    throw new Error('Sort order is required.');
  }

  if (!['hot', 'new', 'top', 'controversial'].includes(sort)) {
    throw new Error('Invalid sort order. Use "hot", "new", "top", or "controversial".');
  }

  if (typeof limit !== 'number' || limit < 1 || limit > 100) {
    throw new Error('Limit must be a number between 1 and 100.');
  }

  const response = await makeRequest('/api/v1/storage/bulk-save', {
    method: 'POST',
    body: {
      subreddit: subreddit,
      sort: sort,
      limit: limit,
    },
  });

  return {
    success: response.success || false,
    saved: response.saved || 0,
    posts: response.posts || [],
  };
}

/**
 * Monitor Functions
 */

/**
 * Start monitoring subreddits.
 * @param {Object} config - Monitor configuration
 * @param {string[]} config.subreddits - Array of subreddit names (1-10)
 * @param {string} config.interval - Poll interval (e.g., "30s", "1m")
 * @param {number} config.limit - Posts per fetch (1-100)
 * @param {boolean} config.fetch_comments - Whether to fetch comments
 * @returns {Promise<Object>} Monitor instance with id, status, started_at
 */
async function startMonitor(config) {
  // Validate config object
  if (!config || typeof config !== 'object') {
    throw new Error('Monitor configuration object is required.');
  }

  // Validate subreddits
  if (!Array.isArray(config.subreddits) || config.subreddits.length === 0) {
    throw new Error('At least one subreddit is required.');
  }

  if (config.subreddits.length > 10) {
    throw new Error('Maximum 10 subreddits allowed.');
  }

  // Validate each subreddit name
  for (let i = 0; i < config.subreddits.length; i++) {
    const sub = config.subreddits[i];
    if (typeof sub !== 'string' || sub.trim() === '') {
      throw new Error('Each subreddit must be a non-empty string.');
    }
    if (sub.length > 21) {
      throw new Error('Subreddit "' + sub + '" exceeds 21 character limit.');
    }
  }

  // Validate interval
  if (!config.interval || typeof config.interval !== 'string') {
    throw new Error('Interval is required (e.g., "30s", "1m").');
  }

  // Validate limit
  if (typeof config.limit !== 'number' || config.limit < 1 || config.limit > 100) {
    throw new Error('Limit must be a number between 1 and 100.');
  }

  // Validate fetch_comments
  if (typeof config.fetch_comments !== 'boolean') {
    throw new Error('fetch_comments must be a boolean value.');
  }

  return makeRequest('/api/v1/monitor/start', {
    method: 'POST',
    body: config,
  });
}

/**
 * Stop the currently running monitor.
 * @returns {Promise<Object>} Final status with stats
 */
async function stopMonitor() {
  return makeRequest('/api/v1/monitor/stop', {
    method: 'POST',
  });
}

/**
 * Get current monitor status and statistics.
 * @returns {Promise<Object>} Status object (running or stopped)
 */
async function getMonitorStatus() {
  return makeRequest('/api/v1/monitor/status', {
    method: 'GET',
  });
}

/**
 * Server Management Functions
 */

/**
 * Shuts down the HTTP server gracefully.
 * @requires Authentication - Must have valid API key
 * @returns {Promise<object>} Response with shutdown message
 * @throws {Error} If request fails or authentication invalid
 */
async function shutdownServer() {
  return makeRequest('/api/v1/server/shutdown', {
    method: 'POST',
  });
}

/**
 * State Management Helper
 */

/**
 * Creates a simple state object with getter/setter and callbacks.
 * Useful for Alpine.js integration without needing Vuex or similar.
 *
 * @param {*} initialValue - Initial state value
 * @returns {object} State object with get(), set(), and subscribe() methods
 */
function createState(initialValue) {
  let value = initialValue;
  const subscribers = [];

  return {
    get: function() {
      return value;
    },
    set: function(newValue) {
      value = newValue;
      for (let i = 0; i < subscribers.length; i++) {
        subscribers[i](value);
      }
    },
    subscribe: function(callback) {
      subscribers.push(callback);
      return function() {
        const index = subscribers.indexOf(callback);
        if (index > -1) {
          subscribers.splice(index, 1);
        }
      };
    },
  };
}

/**
 * Global API object for Alpine.js integration
 * All API functions are accessible as window.api.*
 */
window.api = {
  // Authentication state
  auth: auth,

  // Request utilities
  makeRequest: makeRequest,

  // JWT Authentication
  login: login,
  logout: logout,
  refreshToken: refreshToken,

  // Authentication (deprecated - use login instead)
  checkAuth: checkAuth,
  getCurrentUser: getCurrentUser,

  // Posts
  fetchPosts: fetchPosts,
  fetchHotPosts: fetchHotPosts,
  fetchNewPosts: fetchNewPosts,

  // Comments
  fetchComments: fetchComments,
  fetchMoreComments: fetchMoreComments,

  // Subreddit
  fetchSubreddit: fetchSubreddit,

  // Storage
  savePost: savePost,
  saveComments: saveComments,
  listSavedPosts: listSavedPosts,
  getSavedPost: getSavedPost,
  deleteSavedPost: deleteSavedPost,
  getSavedComments: getSavedComments,
  getStorageStats: getStorageStats,
  bulkSaveFromSubreddit: bulkSaveFromSubreddit,

  // Monitor operations
  startMonitor: startMonitor,
  stopMonitor: stopMonitor,
  getMonitorStatus: getMonitorStatus,

  // Server management
  shutdownServer: shutdownServer,

  // Utilities
  formatTimestamp: formatTimestamp,
  formatScore: formatScore,
  truncateText: truncateText,
  escapeHtml: escapeHtml,
  markdownToHtml: markdownToHtml,
  getThumbnailClass: getThumbnailClass,
  createState: createState,
};

// Make state management available globally
window.createState = createState;

// Initialize debug logging if in development
if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
  console.log('Reddit API Client loaded. Available functions:');
  console.log('- api.checkAuth(key)');
  console.log('- api.getCurrentUser()');
  console.log('- api.fetchHotPosts(options)');
  console.log('- api.fetchNewPosts(options)');
  console.log('- api.fetchComments(subreddit, postId, options)');
  console.log('- api.fetchMoreComments(linkId, children)');
  console.log('- api.fetchSubreddit(name)');
  console.log('- api.savePost(post)');
  console.log('- api.saveComments(postId, comments)');
  console.log('- api.listSavedPosts(filters, pagination)');
  console.log('- api.getSavedPost(postId)');
  console.log('- api.deleteSavedPost(postId)');
  console.log('- api.getSavedComments(postId, options)');
  console.log('- api.getStorageStats()');
  console.log('- api.bulkSaveFromSubreddit(subreddit, sort, limit)');
  console.log('- api.startMonitor(config)');
  console.log('- api.stopMonitor()');
  console.log('- api.getMonitorStatus()');
  console.log('- api.formatTimestamp(unixTime)');
  console.log('- api.formatScore(score)');
  console.log('- api.truncateText(text, maxLength)');
  console.log('- api.escapeHtml(text)');
  console.log('- api.markdownToHtml(markdown)');
  console.log('- api.getThumbnailClass(thumbnail)');
  console.log('- createState(initialValue)');
}
