// API client for communicating with the backend server
import { sanitizeText } from './utils/sanitize.js';

const API_BASE_URL = '/api';

// Allowed sort values for validation
const ALLOWED_SORTS = ['hot', 'new', 'top', 'rising', 'created_utc', 'score', 'num_comments'];

// Maximum reasonable limit value (Reddit API max is 100)
const MAX_LIMIT = 100;
const MIN_LIMIT = 1;

// Maximum reasonable offset value
const MAX_OFFSET = 10000;

// Bulk save count limits
const MIN_BULK_SAVE_COUNT = 1;
const MAX_BULK_SAVE_COUNT = 2000;

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
 * Validate and sanitize a subreddit name
 * @param {string} subreddit - Subreddit name to validate
 * @returns {string} Sanitized subreddit name
 * @throws {APIError} If subreddit is invalid
 */
function validateSubreddit(subreddit) {
  if (!subreddit || typeof subreddit !== 'string') {
    throw new APIError('Invalid subreddit name', 400, { field: 'subreddit' });
  }

  const sanitized = sanitizeText(subreddit, 100).trim();
  if (!sanitized || sanitized.length === 0) {
    throw new APIError('Subreddit name cannot be empty', 400, { field: 'subreddit' });
  }

  // Subreddit names can only contain alphanumeric, underscores, and hyphens
  if (!/^[a-zA-Z0-9_-]+$/.test(sanitized)) {
    throw new APIError('Invalid subreddit name format', 400, { field: 'subreddit' });
  }

  return sanitized;
}

/**
 * Validate and sanitize a post ID
 * @param {string} postId - Post ID to validate
 * @returns {string} Sanitized post ID
 * @throws {APIError} If post ID is invalid
 */
function validatePostId(postId) {
  if (!postId || typeof postId !== 'string') {
    throw new APIError('Invalid post ID', 400, { field: 'post_id' });
  }

  const sanitized = sanitizeText(postId, 100).trim();
  if (!sanitized || sanitized.length === 0) {
    throw new APIError('Post ID cannot be empty', 400, { field: 'post_id' });
  }

  // Post IDs are alphanumeric
  if (!/^[a-zA-Z0-9_]+$/.test(sanitized)) {
    throw new APIError('Invalid post ID format', 400, { field: 'post_id' });
  }

  return sanitized;
}

/**
 * Validate and clamp pagination parameters
 * @param {string} sort - Sort type to validate
 * @param {number} limit - Limit to clamp
 * @param {number} offset - Offset to clamp
 * @returns {object} Validated parameters {sort, limit, offset}
 */
function validatePaginationParams(sort, limit, offset) {
  // Validate sort parameter
  const validatedSort = ALLOWED_SORTS.includes(sort) ? sort : 'created_utc';

  // Clamp limit between MIN_LIMIT and MAX_LIMIT
  const validatedLimit = Math.max(MIN_LIMIT, Math.min(MAX_LIMIT, Math.floor(limit || 25)));

  // Clamp offset between 0 and MAX_OFFSET
  const validatedOffset = Math.max(0, Math.min(MAX_OFFSET, Math.floor(offset || 0)));

  return { sort: validatedSort, limit: validatedLimit, offset: validatedOffset };
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
  // Validate inputs
  const validatedSubreddit = validateSubreddit(subreddit);
  const { sort: validatedSort, limit: validatedLimit } = validatePaginationParams(sort, limit, undefined);
  const sanitizedAfter = sanitizeText(after || '', 100);

  const params = new URLSearchParams({
    subreddit: validatedSubreddit,
    sort: validatedSort,
    limit: validatedLimit.toString(),
  });

  if (sanitizedAfter) {
    params.append('after', sanitizedAfter);
  }

  const data = await apiRequest(`/posts?${params.toString()}`, {
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
  // Validate inputs
  const validatedPostId = validatePostId(postId);
  const validatedSubreddit = validateSubreddit(subreddit);

  const params = new URLSearchParams({
    post_id: validatedPostId,
    subreddit: validatedSubreddit,
  });

  const data = await apiRequest(`/comments?${params.toString()}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return data;
}

/**
 * Fetch saved posts from cache
 * @param {string} token - JWT token
 * @param {object} options - Query options
 * @param {string} options.subreddit - Filter by subreddit (optional)
 * @param {number} options.limit - Number of posts to fetch (default: 25)
 * @param {number} options.offset - Pagination offset (default: 0)
 * @param {string} options.sort - Sort by: 'created_utc', 'score', 'num_comments' (default: 'created_utc')
 * @param {AbortSignal} signal - Abort signal for request cancellation (optional)
 * @returns {Promise<{posts: Array, total: number, offset: number}>}
 */
export async function fetchSavedPosts(token, options = {}, signal = undefined) {
  const { subreddit = '', limit = 25, offset = 0, sort = 'created_utc' } = options;

  // Validate pagination parameters
  const { sort: validatedSort, limit: validatedLimit, offset: validatedOffset } = validatePaginationParams(sort, limit, offset);

  // Validate and sanitize subreddit filter if provided
  let validatedSubreddit = '';
  if (subreddit && subreddit.trim()) {
    try {
      validatedSubreddit = validateSubreddit(subreddit);
    } catch (err) {
      // If subreddit validation fails, ignore the filter (don't throw)
      console.warn('Invalid subreddit filter:', err.message);
      validatedSubreddit = '';
    }
  }

  const params = new URLSearchParams();
  if (validatedSubreddit) params.append('subreddit', validatedSubreddit);
  params.append('limit', validatedLimit.toString());
  params.append('offset', validatedOffset.toString());
  params.append('sort', validatedSort);

  const data = await apiRequest(`/saved/posts?${params.toString()}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    signal,
  });

  return data;
}

/**
 * Fetch comments for a saved post
 * @param {string} token - JWT token
 * @param {string} postId - Post ID (e.g., 'abc123' without the t3_ prefix)
 * @param {string} subreddit - Subreddit name (e.g., 'javascript')
 * @returns {Promise<{comments: Array}>}
 */
export async function fetchSavedComments(token, postId, subreddit = '') {
  // Validate post ID
  const validatedPostId = validatePostId(postId);

  // Validate and sanitize subreddit if provided
  let validatedSubreddit = '';
  if (subreddit && subreddit.trim()) {
    try {
      validatedSubreddit = validateSubreddit(subreddit);
    } catch (err) {
      // If subreddit validation fails, ignore it (don't throw)
      console.warn('Invalid subreddit:', err.message);
      validatedSubreddit = '';
    }
  }

  const params = new URLSearchParams({ post_id: validatedPostId });
  if (validatedSubreddit) params.append('subreddit', validatedSubreddit);

  const data = await apiRequest(`/saved/comments?${params.toString()}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return data;
}

/**
 * Start a bulk save operation for posts
 * @param {string} token - JWT token
 * @param {string} subreddit - Subreddit name (e.g., 'javascript')
 * @param {string} sort - Sort type: 'hot' or 'new'
 * @param {number} count - Number of posts to save (1-2000)
 * @param {boolean} includeComments - Whether to include comments
 * @param {AbortSignal} signal - Abort signal for request cancellation (optional)
 * @returns {Promise<{jobId: string, message: string}>}
 */
export async function bulkSavePosts(token, subreddit, sort, count, includeComments, signal = undefined) {
  // Validate subreddit
  const validatedSubreddit = validateSubreddit(subreddit);

  // Validate sort parameter
  const validatedSort = ['hot', 'new'].includes(sort) ? sort : 'hot';

  // Clamp count to 1-2000 range
  const validatedCount = Math.max(MIN_BULK_SAVE_COUNT, Math.min(MAX_BULK_SAVE_COUNT, Math.floor(count || 100)));

  // Validate includeComments is boolean
  const validatedIncludeComments = Boolean(includeComments);

  const data = await apiRequest('/bulk-save/posts', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({
      subreddit: validatedSubreddit,
      sort: validatedSort,
      count: validatedCount,
      include_comments: validatedIncludeComments,
    }),
    signal,
  });

  return data;
}

/**
 * Get progress of a bulk save operation
 * @param {string} token - JWT token
 * @param {string} jobId - Job ID from bulkSavePosts
 * @param {AbortSignal} signal - Abort signal for request cancellation (optional)
 * @returns {Promise<{status: string, postsSaved: number, postsTotal: number, commentsSaved: number, error: string}>}
 */
export async function getBulkSaveProgress(token, jobId, signal = undefined) {
  // Validate jobId
  if (!jobId || typeof jobId !== 'string' || jobId.trim().length === 0) {
    throw new APIError('Job ID must be a non-empty string', 400, { field: 'jobId' });
  }

  const sanitizedJobId = sanitizeText(jobId, 100).trim();

  // Validate jobId format (UUID v4 or hexadecimal string)
  if (!/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/i.test(sanitizedJobId) &&
      !/^[a-f0-9]+$/i.test(sanitizedJobId)) {
    throw new APIError('Invalid job ID format', 400, { field: 'jobId' });
  }

  const data = await apiRequest(`/bulk-save/progress/${sanitizedJobId}`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    signal,
  });

  return data;
}
