<script>
  import { onMount, onDestroy } from 'svelte';

  // Props
  export let post = null;
  export let comments = [];
  export let loading = false;
  export let error = '';
  export let onClose = () => {};
  export let source = 'live';

  let previouslyFocusedElement = null;
  let modalContent = null;

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
   * Format large numbers
   */
  function formatNumber(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'm';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'k';
    return num.toString();
  }

  /**
   * Truncate text
   */
  function truncateText(text, maxLength = 500) {
    if (!text) return '';
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
  }

  /**
   * Close modal when clicking backdrop
   */
  function handleBackdropClick(event) {
    if (event.target === event.currentTarget) {
      onClose();
    }
  }

  /**
   * Close modal on Escape key
   */
  function handleKeydown(event) {
    if (event.key === 'Escape') {
      onClose();
    }
  }

  /**
   * Handle modal mount - prevent page scroll and manage focus
   */
  onMount(() => {
    // Prevent scroll on body element
    const body = document.body;
    previouslyFocusedElement = document.activeElement;

    // Add modal-open class to body and set fixed styles
    body.classList.add('modal-open');
    body.style.position = 'fixed';
    body.style.width = '100%';

    // Focus modal content for keyboard accessibility
    if (modalContent) {
      modalContent.focus();
    }
  });

  /**
   * Handle modal unmount - restore page scroll and focus
   */
  onDestroy(() => {
    // Remove modal-open class from body and restore styles
    const body = document.body;
    body.classList.remove('modal-open');
    body.style.position = '';
    body.style.width = '';

    // Restore focus to previously focused element
    if (previouslyFocusedElement && typeof previouslyFocusedElement.focus === 'function') {
      previouslyFocusedElement.focus();
    }
  });
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="modal-backdrop" on:click={handleBackdropClick} role="presentation">
  <div class="modal-content" role="dialog" aria-modal="true" bind:this={modalContent} tabindex="-1">
    <div class="modal-header">
      <h2>Comments</h2>
      <button class="close-button" on:click={onClose} aria-label="Close">
        ×
      </button>
    </div>

    {#if post}
      <div class="post-preview">
        <h3 class="post-title">{post.title}</h3>
        <div class="post-info">
          <span>u/{post.author}</span>
          <span>{formatRelativeTime(post.created_utc)}</span>
          <span>{formatNumber(post.score)} points</span>
        </div>
        {#if post.selftext}
          <p class="post-body">{truncateText(post.selftext, 300)}</p>
        {/if}
      </div>

      <div class="divider"></div>
    {/if}

    <div class="comments-section">
      {#if error}
        <div class="error-banner">
          <span class="error-icon">!</span>
          {error}
        </div>
      {/if}

      {#if loading}
        <div class="loading-container">
          <div class="spinner"></div>
          <p>Loading comments...</p>
        </div>
      {:else if comments.length === 0}
        <div class="empty-comments">
          {#if source === 'saved'}
            <p>Comments not cached</p>
            <p class="empty-details">This post was saved but comments weren't loaded.</p>
          {:else}
            <p>No comments found</p>
          {/if}
        </div>
      {:else}
        <div class="comments-list">
          {#each comments as comment (comment.id)}
            <article class="comment">
              <div class="comment-header">
                <span class="comment-author">u/{comment.author}</span>
                <span class="comment-time">{formatRelativeTime(comment.created_utc)}</span>
                <span class="comment-score">{formatNumber(comment.score)} points</span>
              </div>
              <p class="comment-body">{truncateText(comment.body, 500)}</p>
            </article>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  :global(body.modal-open) {
    overflow: hidden;
  }

  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
    z-index: 1000;
    animation: fadeIn 0.2s ease-out;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .modal-content {
    background: white;
    border-radius: 12px;
    width: 100%;
    max-width: 600px;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    animation: slideUp 0.3s ease-out;
  }

  @keyframes slideUp {
    from {
      transform: translateY(40px);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 24px;
    border-bottom: 1px solid #e0e0e0;
  }

  .modal-header h2 {
    margin: 0;
    font-size: 20px;
    color: #1a1a1a;
  }

  .close-button {
    background: none;
    border: none;
    font-size: 28px;
    color: #666;
    cursor: pointer;
    padding: 0;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s;
  }

  .close-button:hover {
    color: #1a1a1a;
  }

  .post-preview {
    padding: 20px 24px;
    background-color: #f8f9fa;
  }

  .post-title {
    margin: 0 0 12px 0;
    font-size: 16px;
    font-weight: 600;
    color: #1a1a1a;
    line-height: 1.4;
  }

  .post-info {
    display: flex;
    gap: 12px;
    font-size: 13px;
    color: #666;
    margin-bottom: 12px;
  }

  .post-info span {
    display: flex;
    align-items: center;
  }

  .post-body {
    margin: 0;
    font-size: 14px;
    color: #333;
    line-height: 1.5;
  }

  .divider {
    height: 1px;
    background-color: #e0e0e0;
  }

  .comments-section {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
  }

  .error-banner {
    display: flex;
    align-items: center;
    gap: 12px;
    background-color: #fee;
    border: 1px solid #fcc;
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 16px;
    color: #c33;
    font-size: 13px;
  }

  .error-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    background-color: #c33;
    color: white;
    border-radius: 50%;
    font-weight: bold;
    font-size: 11px;
    flex-shrink: 0;
  }

  .loading-container {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    padding: 40px 20px;
    color: #666;
  }

  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid #e0e0e0;
    border-top-color: #667eea;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-bottom: 12px;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .empty-comments {
    text-align: center;
    padding: 40px 20px;
    color: #666;
    font-size: 14px;
  }

  .empty-comments p {
    margin: 8px 0;
  }

  .empty-details {
    font-size: 13px;
    color: #999;
  }

  .comments-list {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .comment {
    padding: 16px;
    background-color: #f8f9fa;
    border-radius: 8px;
    border-left: 3px solid #667eea;
  }

  .comment-header {
    display: flex;
    gap: 12px;
    font-size: 12px;
    color: #666;
    margin-bottom: 8px;
  }

  .comment-author {
    font-weight: 600;
    color: #333;
  }

  .comment-time {
    color: #999;
  }

  .comment-score {
    color: #667eea;
    font-weight: 500;
  }

  .comment-body {
    margin: 0;
    font-size: 14px;
    color: #333;
    line-height: 1.5;
    word-break: break-word;
  }

  /* Scrollbar styling */
  .comments-section::-webkit-scrollbar {
    width: 8px;
  }

  .comments-section::-webkit-scrollbar-track {
    background: transparent;
  }

  .comments-section::-webkit-scrollbar-thumb {
    background: #ddd;
    border-radius: 4px;
  }

  .comments-section::-webkit-scrollbar-thumb:hover {
    background: #bbb;
  }

  /* Responsive design */
  @media (max-width: 768px) {
    .modal-backdrop {
      padding: 0;
    }

    .modal-content {
      max-width: 100%;
      max-height: 100vh;
      border-radius: 12px 12px 0 0;
    }

    .modal-header {
      padding: 20px;
    }

    .post-preview {
      padding: 16px;
    }

    .comments-section {
      padding: 16px;
    }
  }
</style>
