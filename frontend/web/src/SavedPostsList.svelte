<script>
  import { onMount, onDestroy } from 'svelte';
  import { fetchSavedPosts, APIError } from './api.js';
  import { sanitizePost, sanitizeText } from './utils/sanitize.js';

  // Props
  export let token = '';
  export let onSelectPost = () => {};

  // Constants for pagination and debouncing
  const PAGE_SIZE = 25;
  const DEBOUNCE_DELAY_MS = 500;

  // State
  let posts = [];
  let loading = false;
  let error = '';
  let offset = 0;
  let hasMore = false;
  let totalPosts = 0;

  // Filters
  let subredditFilter = '';
  let sortBy = 'created_utc';

  // Debounce timeout
  let debounceTimeout = null;

  // Request cancellation
  let searchAbortController = null;
  let loadMoreAbortController = null;

  /**
   * Format relative time (e.g., "2 hours ago")
   */
  function formatRelativeTime(unixTimestamp) {
    const now = Math.floor(Date.now() / 1000);
    const diff = now - unixTimestamp;

    if (diff < 60) return 'just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    if (diff < 604800) return `${Math.floor(diff / 86400)}d ago`;
    if (diff < 2592000) return `${Math.floor(diff / 604800)}w ago`;

    return `${Math.floor(diff / 2592000)}mo ago`;
  }

  /**
   * Format large numbers (1000 -> 1k, 1000000 -> 1m)
   */
  function formatNumber(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'm';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'k';
    return num.toString();
  }

  /**
   * Truncate text to max length
   */
  function truncateText(text, maxLength = 200) {
    if (!text) return '';
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
  }

  /**
   * Load saved posts with current filters
   */
  async function loadSavedPosts() {
    // Cancel previous request if one is in progress
    if (searchAbortController) {
      searchAbortController.abort();
    }

    // Create new abort controller for this request
    searchAbortController = new AbortController();

    error = '';
    loading = true;
    offset = 0;
    posts = [];

    try {
      const response = await fetchSavedPosts(
        token,
        {
          subreddit: subredditFilter,
          limit: PAGE_SIZE,
          offset: 0,
          sort: sortBy,
        },
        searchAbortController.signal
      );

      // Sanitize all posts from the response
      const sanitizedPosts = (response.posts || []).map(sanitizePost).filter(Boolean);

      posts = sanitizedPosts;
      totalPosts = response.total || 0;
      hasMore = (response.total || 0) > PAGE_SIZE;
    } catch (err) {
      // Ignore abort errors (user-initiated cancellation)
      if (err.name === 'AbortError') {
        return;
      }

      if (err instanceof APIError) {
        if (err.status === 404) {
          error = 'No saved posts found';
        } else if (err.status === 403) {
          error = 'Unable to load saved posts';
        } else {
          error = err.message || 'Failed to load saved posts';
        }
      } else {
        error = 'Network error. Please try again.';
      }
      posts = [];
    } finally {
      loading = false;
    }
  }

  /**
   * Handle subreddit filter change with debounce
   */
  function handleSubredditChange(value) {
    subredditFilter = value;

    // Clear existing debounce
    if (debounceTimeout) {
      clearTimeout(debounceTimeout);
    }

    // Debounce filter change to avoid excessive API calls
    debounceTimeout = setTimeout(() => {
      loadSavedPosts();
    }, DEBOUNCE_DELAY_MS);
  }

  /**
   * Handle sort change
   */
  function handleSortChange(value) {
    sortBy = value;
    loadSavedPosts();
  }

  /**
   * Handle load more posts with request cancellation to prevent race conditions
   */
  async function handleLoadMore() {
    if (!hasMore) return;

    // Cancel previous load more request if one is in progress
    if (loadMoreAbortController) {
      loadMoreAbortController.abort();
    }

    // Create new abort controller for this load more request
    loadMoreAbortController = new AbortController();

    const newOffset = offset + PAGE_SIZE;
    error = '';
    loading = true;

    try {
      const response = await fetchSavedPosts(
        token,
        {
          subreddit: subredditFilter,
          limit: PAGE_SIZE,
          offset: newOffset,
          sort: sortBy,
        },
        loadMoreAbortController.signal
      );

      // Sanitize all posts from the response
      const sanitizedPosts = (response.posts || []).map(sanitizePost).filter(Boolean);

      posts = [...posts, ...sanitizedPosts];
      offset = newOffset;
      hasMore = (response.total || 0) > offset + PAGE_SIZE;
    } catch (err) {
      // Ignore abort errors (user-initiated cancellation)
      if (err.name === 'AbortError') {
        return;
      }

      error = 'Failed to load more posts';
      console.error('Load more error:', err);
    } finally {
      loading = false;
    }
  }

  /**
   * Retry loading posts after an error
   */
  function retryLoadPosts() {
    loadSavedPosts();
  }

  /**
   * Load initial saved posts on component mount
   */
  onMount(() => {
    loadSavedPosts();
  });

  /**
   * Cleanup on component destroy
   * Clear debounce timeout and abort any in-flight requests
   */
  onDestroy(() => {
    // Clear debounce timeout
    if (debounceTimeout) {
      clearTimeout(debounceTimeout);
      debounceTimeout = null;
    }

    // Abort any in-flight search requests
    if (searchAbortController) {
      searchAbortController.abort();
      searchAbortController = null;
    }

    // Abort any in-flight load more requests
    if (loadMoreAbortController) {
      loadMoreAbortController.abort();
      loadMoreAbortController = null;
    }
  });
</script>

{#if error}
  <div class="error-banner">
    <span class="error-icon">!</span>
    <span class="error-message">{error}</span>
    <button
      class="error-retry-button"
      on:click={retryLoadPosts}
      aria-label="Retry loading posts"
    >
      Retry
    </button>
  </div>
{/if}

{#if loading && posts.length === 0}
  <div class="loading-container">
    <div class="spinner-large"></div>
    <p>Loading saved posts...</p>
  </div>
{:else if posts.length === 0}
  <div class="empty-state">
    <div class="empty-icon">bookmark</div>
    <p>No saved posts yet</p>
    <p class="empty-subtext">
      Browse some subreddits to build your cache!
    </p>
  </div>
{:else}
  <div class="saved-posts-wrapper">
    <!-- Filters Section -->
    <div class="filters-section">
      <div class="filter-group">
        <label for="subreddit-filter" class="filter-label">Subreddit Filter</label>
        <input
          id="subreddit-filter"
          type="text"
          class="filter-input"
          placeholder="Filter by subreddit (optional)"
          value={subredditFilter}
          aria-label="Filter saved posts by subreddit name"
          on:input={(e) => handleSubredditChange(sanitizeText(e.target.value, 100))}
        />
      </div>

      <div class="filter-group">
        <label for="sort-by" class="filter-label">Sort By</label>
        <select
          id="sort-by"
          class="filter-select"
          value={sortBy}
          aria-label="Sort saved posts by date, score, or comment count"
          on:change={(e) => handleSortChange(e.target.value)}
        >
          <option value="created_utc">Latest</option>
          <option value="score">Highest Score</option>
          <option value="num_comments">Most Comments</option>
        </select>
      </div>

      {#if totalPosts > 0}
        <div class="filter-info">
          Showing {posts.length} of {totalPosts} saved posts
        </div>
      {/if}
    </div>

    <!-- Posts List -->
    <div class="posts-container">
      {#each posts as post (post.id)}
        <article
          class="post-card"
          on:click={() => onSelectPost(post)}
          role="button"
          tabindex="0"
          aria-label="Click to view comments for {post.title}"
          on:keydown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              onSelectPost(post);
            }
          }}
        >
          <div class="post-content">
            <h3 class="post-title">{post.title}</h3>

            {#if post.selftext}
              <p class="post-body">
                {truncateText(post.selftext, 200)}
              </p>
            {/if}

            <div class="post-meta">
              <span class="meta-item">
                <span class="meta-label">Score</span>
                <span class="meta-value">{formatNumber(post.score)}</span>
              </span>
              <span class="meta-item">
                <span class="meta-label">Comments</span>
                <span class="meta-value">{formatNumber(post.num_comments)}</span>
              </span>
              <span class="meta-item">
                <span class="meta-label">Subreddit</span>
                <span class="meta-value">r/{post.subreddit}</span>
              </span>
              <span class="meta-item">
                <span class="meta-label">Saved</span>
                <span class="meta-value">{formatRelativeTime(post.created_utc)}</span>
              </span>
            </div>
          </div>

          <div class="post-arrow">
            →
          </div>
        </article>
      {/each}

      {#if hasMore}
        <button
          class="load-more-button"
          on:click={handleLoadMore}
          disabled={loading}
        >
          {#if loading}
            <span class="spinner"></span>
            Loading...
          {:else}
            Load More Posts
          {/if}
        </button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .error-banner {
    display: flex;
    align-items: center;
    gap: 12px;
    background-color: #fee;
    border: 1px solid #fcc;
    border-radius: 8px;
    padding: 14px 16px;
    margin-bottom: 24px;
    color: #c33;
    font-size: 14px;
  }

  .error-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    width: 20px;
    height: 20px;
    background-color: #c33;
    color: white;
    border-radius: 50%;
    font-weight: bold;
    font-size: 12px;
  }

  .error-message {
    flex: 1;
  }

  .error-retry-button {
    flex-shrink: 0;
    padding: 6px 12px;
    background-color: #c33;
    color: white;
    border: none;
    border-radius: 4px;
    font-size: 13px;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  .error-retry-button:hover {
    background-color: #a22;
  }

  .error-retry-button:focus {
    outline: 2px solid #c33;
    outline-offset: 2px;
  }

  .loading-container {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    padding: 60px 20px;
    color: #666;
  }

  .spinner-large {
    width: 40px;
    height: 40px;
    border: 4px solid #e0e0e0;
    border-top-color: #667eea;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-bottom: 16px;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .empty-state {
    text-align: center;
    padding: 60px 20px;
    color: #666;
  }

  .empty-icon {
    font-size: 48px;
    margin-bottom: 16px;
  }

  .empty-state p {
    margin: 8px 0;
    font-size: 16px;
  }

  .empty-subtext {
    color: #999;
    font-size: 14px;
  }

  .saved-posts-wrapper {
    display: flex;
    flex-direction: column;
    gap: 24px;
  }

  /* Filters Section */
  .filters-section {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
    padding: 20px;
    background: white;
    border-radius: 10px;
    border: 1px solid #e0e0e0;
  }

  .filter-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .filter-label {
    font-size: 13px;
    font-weight: 600;
    color: #333;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .filter-input,
  .filter-select {
    padding: 10px 12px;
    border: 1px solid #d0d0d0;
    border-radius: 6px;
    font-size: 14px;
    font-family: inherit;
    transition: border-color 0.2s;
  }

  .filter-input:focus,
  .filter-select:focus {
    outline: none;
    border-color: #667eea;
    box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
  }

  .filter-select {
    cursor: pointer;
    background-color: white;
  }

  .filter-info {
    display: flex;
    align-items: center;
    padding: 10px 12px;
    font-size: 13px;
    color: #666;
    background-color: #f8f9fa;
    border-radius: 6px;
  }

  /* Posts Container */
  .posts-container {
    display: grid;
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .post-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: white;
    border: 1px solid #e0e0e0;
    border-radius: 10px;
    padding: 20px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .post-card:hover {
    border-color: #667eea;
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.15);
    transform: translateY(-2px);
  }

  .post-card:focus {
    outline: none;
    box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.2);
  }

  .post-content {
    flex: 1;
    min-width: 0;
  }

  .post-title {
    margin: 0 0 8px 0;
    font-size: 18px;
    font-weight: 600;
    color: #1a1a1a;
    line-height: 1.4;
    word-break: break-word;
  }

  .post-body {
    margin: 0 0 12px 0;
    font-size: 14px;
    color: #666;
    line-height: 1.5;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .post-meta {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
    font-size: 13px;
    color: #666;
  }

  .meta-item {
    display: flex;
    gap: 4px;
  }

  .meta-label {
    color: #999;
  }

  .meta-value {
    color: #333;
    font-weight: 500;
  }

  .post-arrow {
    flex-shrink: 0;
    margin-left: 16px;
    font-size: 20px;
    color: #667eea;
    transition: transform 0.2s;
  }

  .post-card:hover .post-arrow {
    transform: translateX(4px);
  }

  .load-more-button {
    width: 100%;
    padding: 14px 20px;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;
    transition: transform 0.2s, box-shadow 0.2s;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .load-more-button:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
  }

  .load-more-button:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .spinner {
    width: 14px;
    height: 14px;
    border: 2px solid #ffffff;
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  /* Responsive design */
  @media (max-width: 768px) {
    .filters-section {
      grid-template-columns: 1fr;
    }

    .post-card {
      padding: 16px;
      flex-direction: column;
      align-items: flex-start;
    }

    .post-title {
      font-size: 16px;
    }

    .post-meta {
      gap: 12px;
      font-size: 12px;
    }

    .post-arrow {
      display: none;
    }

    .loading-container {
      padding: 40px 20px;
    }

    .empty-state {
      padding: 40px 20px;
    }
  }
</style>
