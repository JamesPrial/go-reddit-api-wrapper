// API client for communicating with the backend server

const API_BASE_URL = '/api';

/**
 * Custom error class for API errors
 */
export class APIError extends Error {
  constructor(message, status, details) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.details = details;
  }
}

/**
 * Make an HTTP request to the backend API
 * @param {string} endpoint - API endpoint path (e.g., '/auth/login')
 * @param {object} options - Fetch options (can include signal for abort control)
 * @returns {Promise<object>} - Response data
 */
async function apiRequest(endpoint, options = {}) {
  const url = `${API_BASE_URL}${endpoint}`;

  try {
    const response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });

    // Parse response body
    let data;
    const contentType = response.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      data = await response.json();
    } else {
      data = await response.text();
    }

    // Handle error responses
    if (!response.ok) {
      const message = data?.error || data?.message || 'Request failed';
      throw new APIError(message, response.status, data);
    }

    return data;
  } catch (error) {
    // Ignore abort errors (user-initiated cancellation)
    if (error.name === 'AbortError') {
      throw error;
    }

    // Re-throw APIErrors as-is
    if (error instanceof APIError) {
      throw error;
    }

    // Network errors or other failures
    throw new APIError(
      error.message || 'Network error',
      0,
      { originalError: error }
    );
  }
}

/**
 * Login with username and password
 * @param {string} username - Reddit username
 * @param {string} password - Reddit password
 * @returns {Promise<{success: boolean, token: string, username: string}>}
 */
export async function login(username, password) {
  const data = await apiRequest('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });

  return data;
}

/**
 * Check authentication status
 * @param {string} token - JWT token
 * @returns {Promise<{authenticated: boolean, username: string, link_karma: number, comment_karma: number}>}
 */
export async function checkAuth(token) {
  const data = await apiRequest('/auth/status', {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return data;
}

/**
 * Logout and invalidate session
 * @param {string} token - JWT token
 * @returns {Promise<{success: boolean}>}
 */
export async function logout(token) {
  const data = await apiRequest('/auth/logout', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return data;
}

/**
 * Fetch posts from a subreddit
 * @param {string} token - JWT token
 * @param {string} subreddit - Subreddit name (e.g., 'javascript')
 * @param {string} sort - Sort type: 'hot', 'new', 'top', 'rising'
 * @param {string} after - Pagination fullname (optional)
 * @param {number} limit - Number of posts to fetch (default: 25)
 * @param {AbortSignal} signal - Abort signal for request cancellation (optional)
 * @returns {Promise<{posts: Array, after_fullname: string, before_fullname: string}>}
 */
export async function fetchSubredditPosts(token, subreddit, sort = 'hot', after = '', limit = 25, signal = undefined) {
  const params = new URLSearchParams({
    subreddit,
    sort,
    limit: limit.toString(),
  });

  if (after) {
    params.append('after', after);
  }

  const data = await apiRequest(`/reddit/posts?${params.toString()}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    signal,
  });

  return data;
}

/**
 * Fetch comments for a post
 * @param {string} token - JWT token
 * @param {string} postId - Post ID (e.g., 'abc123' without the t3_ prefix)
 * @param {string} subreddit - Subreddit name (e.g., 'javascript')
 * @returns {Promise<{post: object, comments: Array, more_ids: Array}>}
 */
export async function fetchPostComments(token, postId, subreddit) {
  const params = new URLSearchParams({
    post_id: postId,
    subreddit,
  });

  const data = await apiRequest(`/reddit/comments?${params.toString()}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return data;
}
