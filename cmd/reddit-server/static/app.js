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
  API_KEY_STORAGE: 'reddit_api_key',
  REQUEST_TIMEOUT: 30000, // 30 seconds
  MAX_RETRIES: 3,
};

/**
 * Storage Management
 */

/**
 * Saves the API key to localStorage.
 * @param {string} key - The API key to save
 */
function saveApiKey(key) {
  if (!key || typeof key !== 'string') {
    throw new Error('Invalid API key');
  }
  localStorage.setItem(CONFIG.API_KEY_STORAGE, key.trim());
}

/**
 * Retrieves the API key from localStorage.
 * @returns {string|null} The stored API key, or null if not found
 */
function getApiKey() {
  return localStorage.getItem(CONFIG.API_KEY_STORAGE);
}

/**
 * Clears the API key from localStorage.
 */
function clearApiKey() {
  localStorage.removeItem(CONFIG.API_KEY_STORAGE);
}

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
 * @returns {Promise<object>} Parsed JSON response
 * @throws {Error} User-friendly error message
 */
async function makeRequest(url, options = {}) {
  const {
    method = 'GET',
    body = null,
    retry = true,
  } = options;

  const apiKey = getApiKey();
  const fullUrl = CONFIG.API_BASE_URL + url;

  const { controller, timeoutId } = createAbortTimeout(CONFIG.REQUEST_TIMEOUT);

  try {
    const headers = {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
    };

    // Add authorization header if API key is available
    if (apiKey) {
      headers['Authorization'] = 'Bearer ' + apiKey;
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

    // Handle 401 - clear stored API key for re-authentication
    if (response.status === 401) {
      clearApiKey();
      throw new Error('Authentication required. Please provide a valid API key.');
    }

    // Handle rate limiting
    if (response.status === 429) {
      throw new Error('Rate limited. Please wait a moment before trying again.');
    }

    // Handle not found
    if (response.status === 404) {
      throw new Error('Resource not found.');
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
 * Checks if the provided API key is valid by making a test request.
 * @param {string} apiKey - The API key to validate
 * @returns {Promise<boolean>} True if the key is valid, false otherwise
 */
async function checkAuth(apiKey) {
  try {
    // Temporarily set the key to test it
    const originalKey = getApiKey();
    saveApiKey(apiKey);

    // Try to fetch user info to validate the key
    const response = await makeRequest('/api/v1/user/me', {
      method: 'GET',
    });

    return !!response;
  } catch (error) {
    // Restore original key on failure
    if (originalKey) {
      saveApiKey(originalKey);
    } else {
      clearApiKey();
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
  // Storage
  saveApiKey: saveApiKey,
  getApiKey: getApiKey,
  clearApiKey: clearApiKey,

  // Request utilities
  makeRequest: makeRequest,

  // Authentication
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
  console.log('- api.formatTimestamp(unixTime)');
  console.log('- api.formatScore(score)');
  console.log('- api.truncateText(text, maxLength)');
  console.log('- api.escapeHtml(text)');
  console.log('- api.markdownToHtml(markdown)');
  console.log('- api.getThumbnailClass(thumbnail)');
  console.log('- createState(initialValue)');
}
