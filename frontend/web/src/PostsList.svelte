<script>
  // Props
  export let posts = [];
  export let loading = false;
  export let error = '';
  export let onSelectPost = () => {};
  export let onLoadMore = () => {};
  export let afterFullname = '';
  export let hasMore = false;

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
</script>

{#if error}
  <div class="error-banner">
    <span class="error-icon">!</span>
    {error}
  </div>
{/if}

{#if loading && posts.length === 0}
  <div class="loading-container">
    <div class="spinner-large"></div>
    <p>Loading posts...</p>
  </div>
{:else if posts.length === 0}
  <div class="empty-state">
    <div class="empty-icon">empty</div>
    <p>No posts found</p>
    <p class="empty-subtext">Try searching for a different subreddit</p>
  </div>
{:else}
  <div class="posts-container">
    {#each posts as post (post.id)}
      <article class="post-card" on:click={() => onSelectPost(post)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && onSelectPost(post)}>
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
              <span class="meta-label">Author</span>
              <span class="meta-value">u/{post.author}</span>
            </span>
            <span class="meta-item">
              <span class="meta-label">Posted</span>
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
        on:click={onLoadMore}
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
    width: 20px;
    height: 20px;
    background-color: #c33;
    color: white;
    border-radius: 50%;
    font-weight: bold;
    font-size: 12px;
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

  .posts-container {
    display: grid;
    grid-template-columns: 1fr;
    gap: 16px;
    margin-bottom: 32px;
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
