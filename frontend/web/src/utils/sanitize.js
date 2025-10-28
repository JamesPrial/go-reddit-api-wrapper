/**
 * Sanitization utilities for API response data
 * Prevents XSS and injection attacks by removing control characters
 * and limiting input sizes
 */

/**
 * Sanitize text by removing control characters and limiting length
 * Prevents XSS attacks and ensures reasonable data sizes
 *
 * @param {string} text - Text to sanitize
 * @param {number} maxLength - Maximum allowed length (default: 10000)
 * @returns {string} Sanitized text
 */
export function sanitizeText(text, maxLength = 10000) {
  if (typeof text !== 'string') {
    return '';
  }

  // Remove control characters (ASCII 0-31, except tab/newline/carriage return)
  let sanitized = text.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, '');

  // Limit length
  if (sanitized.length > maxLength) {
    sanitized = sanitized.substring(0, maxLength);
  }

  return sanitized;
}

/**
 * Sanitize number by ensuring it's a valid number type
 * Prevents NaN and Infinity from causing display issues
 *
 * @param {any} num - Value to sanitize
 * @returns {number} Valid number or 0 if invalid
 */
export function sanitizeNumber(num) {
  const parsed = Number(num);

  // Check for valid finite number
  if (Number.isFinite(parsed)) {
    return parsed;
  }

  return 0;
}

/**
 * Sanitize a post object from API response
 * Applied to all posts to ensure data safety
 *
 * @param {object} post - Post object to sanitize
 * @returns {object} Sanitized post
 */
export function sanitizePost(post) {
  if (!post || typeof post !== 'object') {
    return null;
  }

  return {
    ...post,
    title: sanitizeText(post.title, 300),
    selftext: sanitizeText(post.selftext, 10000),
    author: sanitizeText(post.author, 100),
    subreddit: sanitizeText(post.subreddit, 100),
    score: sanitizeNumber(post.score),
    num_comments: sanitizeNumber(post.num_comments),
    created_utc: sanitizeNumber(post.created_utc),
  };
}

/**
 * Sanitize a comment object from API response
 * Applied to all comments to ensure data safety
 *
 * @param {object} comment - Comment object to sanitize
 * @returns {object} Sanitized comment
 */
export function sanitizeComment(comment) {
  if (!comment || typeof comment !== 'object') {
    return null;
  }

  return {
    ...comment,
    body: sanitizeText(comment.body, 5000),
    author: sanitizeText(comment.author, 100),
    score: sanitizeNumber(comment.score),
    created_utc: sanitizeNumber(comment.created_utc),
  };
}
