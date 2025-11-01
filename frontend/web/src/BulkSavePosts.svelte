<script>
  import { onDestroy } from 'svelte';
  import { bulkSavePosts, getBulkSaveProgress } from './api.js';

  // Props
  export let token;
  export let onSuccess = () => {};

  // Form state
  let subreddit = '';
  let sort = 'hot';
  let count = 100;
  let includeComments = false;

  // Component state
  let state = 'idle'; // idle | saving | completed | error
  let touched = false;
  let error = '';

  // Progress state
  let jobId = '';
  let postsSaved = 0;
  let postsTotal = 0;
  let commentsSaved = 0;
  let statusMessage = '';
  let progressPercent = 0;

  // Polling
  let pollInterval = null;
  let abortController = null;
  let pollStartTime = null;
  let pollAttempts = 0;

  // Maximum polling duration: 5 minutes (300000ms)
  const MAX_POLL_DURATION = 300000;

  /**
   * Validate subreddit name: 3-21 chars, alphanumeric + underscores + hyphens
   */
  function validateSubreddit(value) {
    if (!value) return 'Subreddit name is required';
    const trimmed = value.trim();
    if (trimmed.length < 3) return 'Name must be at least 3 characters';
    if (trimmed.length > 21) return 'Name must be at most 21 characters';
    if (!/^[a-zA-Z0-9_-]+$/.test(trimmed)) {
      return 'Only letters, numbers, underscores, and hyphens allowed';
    }
    return '';
  }

  /**
   * Validate count: 1-2000
   */
  function validateCount(value) {
    // Handle empty string
    if (value === '' || value === null || value === undefined) {
      return 'Count is required';
    }

    // Convert to string and check for decimal
    const stringValue = String(value);
    if (stringValue.includes('.')) {
      return 'Count must be a whole number';
    }

    const num = parseInt(value, 10);
    if (isNaN(num)) return 'Count must be a number';
    if (num < 1) return 'Count must be at least 1';
    if (num > 2000) return 'Count must be at most 2000';
    return '';
  }

  // Reactive validation
  $: subredditError = touched ? validateSubreddit(subreddit) : '';
  $: countError = touched ? validateCount(count) : '';
  $: hasValidationErrors = !!subredditError || !!countError;

  /**
   * Start polling for progress
   */
  function startPolling() {
    if (pollInterval) return;

    // Initialize polling tracking
    pollStartTime = Date.now();
    pollAttempts = 0;

    pollInterval = setInterval(async () => {
      try {
        if (!jobId) return;

        // Check if we've exceeded maximum polling duration
        pollAttempts++;
        const elapsedTime = Date.now() - pollStartTime;
        if (elapsedTime > MAX_POLL_DURATION) {
          const timeoutError = 'Operation timed out after 5 minutes';
          statusMessage = timeoutError;
          state = 'error';
          error = timeoutError;
          stopPolling();
          return;
        }

        const progress = await getBulkSaveProgress(token, jobId, abortController?.signal);

        postsSaved = progress.posts_saved || 0;
        postsTotal = progress.posts_total || 0;
        commentsSaved = progress.comments_saved || 0;

        // Calculate progress percentage
        if (postsTotal > 0) {
          progressPercent = Math.round((postsSaved / postsTotal) * 100);
        }

        // Update status message based on state
        if (progress.status === 'fetching_posts') {
          statusMessage = `Fetching posts... (${postsSaved} of ${postsTotal})`;
        } else if (progress.status === 'fetching_comments') {
          statusMessage = `Fetching comments... (${commentsSaved} comments saved)`;
        } else if (progress.status === 'saving') {
          statusMessage = `Saved ${postsSaved} of ${postsTotal} posts...`;
        } else if (progress.status === 'completed') {
          statusMessage = 'Complete!';
          state = 'completed';
          stopPolling();
          onSuccess();
        } else if (progress.status === 'error') {
          const errorMessage = progress.error || 'Bulk save operation failed';
          statusMessage = errorMessage;
          state = 'error';
          error = errorMessage;
          stopPolling();
        }
      } catch (err) {
        // Ignore abort errors
        if (err.name === 'AbortError') {
          return;
        }

        console.error('Error polling progress:', err);
        state = 'error';
        error = err.message || 'Failed to fetch progress';
        stopPolling();
      }
    }, 500);
  }

  /**
   * Stop polling
   */
  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
    pollStartTime = null;
    pollAttempts = 0;
  }

  /**
   * Handle form submission
   */
  async function handleSubmit(event) {
    event.preventDefault();
    touched = true;
    error = '';

    if (hasValidationErrors) {
      return;
    }

    state = 'saving';
    statusMessage = 'Starting bulk save...';
    postsSaved = 0;
    postsTotal = 0;
    commentsSaved = 0;
    progressPercent = 0;

    // Create abort controller for cancellation
    abortController = new AbortController();

    try {
      const response = await bulkSavePosts(
        token,
        subreddit.trim(),
        sort,
        count,
        includeComments,
        abortController.signal
      );

      jobId = response.job_id;
      postsTotal = count;
      statusMessage = 'Bulk save started...';

      // Start polling for progress
      startPolling();
    } catch (err) {
      // Ignore abort errors
      if (err.name === 'AbortError') {
        return;
      }

      state = 'error';
      error = err.message || 'Failed to start bulk save';
      stopPolling();
    }
  }

  /**
   * Handle cancel button
   */
  function handleCancel() {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
    stopPolling();
    state = 'idle';
    statusMessage = '';
    postsSaved = 0;
    postsTotal = 0;
    commentsSaved = 0;
    progressPercent = 0;
    jobId = '';
  }

  /**
   * Handle input changes
   */
  function handleInput() {
    if (touched && error) {
      error = '';
    }
  }

  /**
   * Reset form
   */
  function handleReset() {
    subreddit = '';
    sort = 'hot';
    count = 100;
    includeComments = false;
    touched = false;
    error = '';
    state = 'idle';
    statusMessage = '';
    postsSaved = 0;
    postsTotal = 0;
    commentsSaved = 0;
    progressPercent = 0;
    jobId = '';
  }

  /**
   * Cleanup on component unmount
   */
  onDestroy(() => {
    stopPolling();
    if (abortController) {
      abortController.abort();
    }
  });
</script>

<form on:submit={handleSubmit} class="bulk-save-form">
  <h2 class="form-title">Bulk Save Posts</h2>

  <div class="form-container">
    <div class="form-group">
      <label for="subreddit">Subreddit</label>
      <div class="input-wrapper">
        <span class="prefix">r/</span>
        <input
          id="subreddit"
          type="text"
          bind:value={subreddit}
          on:input={handleInput}
          on:blur={() => (touched = true)}
          disabled={state === 'saving'}
          class:error={subredditError}
          placeholder="javascript, programming, learnprogramming"
        />
      </div>
      {#if subredditError}
        <span class="field-error">{subredditError}</span>
      {/if}
    </div>

    <div class="form-row">
      <div class="form-group">
        <label for="sort">Sort By</label>
        <select id="sort" bind:value={sort} disabled={state === 'saving'}>
          <option value="hot">Hot</option>
          <option value="new">New</option>
        </select>
      </div>

      <div class="form-group">
        <label for="count">Count</label>
        <input
          id="count"
          type="number"
          bind:value={count}
          on:input={handleInput}
          on:blur={() => (touched = true)}
          disabled={state === 'saving'}
          class:error={countError}
          min="1"
          max="2000"
          placeholder="100"
        />
        {#if countError}
          <span class="field-error">{countError}</span>
        {/if}
      </div>
    </div>

    <div class="form-group checkbox-group">
      <label class="checkbox-label">
        <input
          type="checkbox"
          bind:checked={includeComments}
          disabled={state === 'saving'}
        />
        <span>Include Comments</span>
      </label>
    </div>
  </div>

  {#if error}
    <div class="error-message">
      {error}
    </div>
  {/if}

  {#if state === 'saving'}
    <div class="progress-container">
      <div class="progress-header">
        <span class="progress-text">{statusMessage}</span>
        <span class="progress-percentage">{progressPercent}%</span>
      </div>
      <div class="progress-bar">
        <div class="progress-fill" style="width: {progressPercent}%"></div>
      </div>
      <div class="progress-details">
        <span>Posts: {postsSaved}/{postsTotal}</span>
        {#if includeComments}
          <span>Comments: {commentsSaved}</span>
        {/if}
      </div>
    </div>
  {/if}

  {#if state === 'completed'}
    <div class="success-message">
      Successfully saved {postsSaved} posts{includeComments ? ` with ${commentsSaved} comments` : ''}!
    </div>
  {/if}

  <div class="button-group">
    {#if state === 'idle' || state === 'error'}
      <button
        type="submit"
        disabled={hasValidationErrors}
        class="save-button"
      >
        Save Posts
      </button>
    {/if}

    {#if state === 'saving'}
      <button
        type="button"
        on:click={handleCancel}
        class="cancel-button"
      >
        Cancel
      </button>
    {/if}

    {#if state === 'completed'}
      <button
        type="button"
        on:click={handleReset}
        class="reset-button"
      >
        Save More Posts
      </button>
    {/if}
  </div>
</form>

<style>
  .bulk-save-form {
    background: white;
    border-radius: 12px;
    padding: 28px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    margin-bottom: 32px;
  }

  .form-title {
    margin: 0 0 24px 0;
    font-size: 22px;
    font-weight: 600;
    color: #1a1a1a;
  }

  .form-container {
    display: flex;
    flex-direction: column;
    gap: 20px;
    margin-bottom: 20px;
  }

  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
  }

  label {
    display: block;
    margin-bottom: 8px;
    font-weight: 500;
    color: #333;
    font-size: 14px;
  }

  .input-wrapper {
    display: flex;
    align-items: center;
    border: 2px solid #e0e0e0;
    border-radius: 8px;
    transition: border-color 0.2s;
    background-color: white;
  }

  .input-wrapper:focus-within {
    border-color: #667eea;
  }

  .prefix {
    padding-left: 12px;
    color: #666;
    font-weight: 500;
    font-size: 16px;
  }

  input[type="text"],
  input[type="number"] {
    flex: 1;
    padding: 12px 16px;
    border: none;
    font-size: 16px;
    background-color: transparent;
  }

  input[type="number"] {
    border: 2px solid #e0e0e0;
    border-radius: 8px;
    transition: border-color 0.2s;
  }

  input:focus {
    outline: none;
  }

  input[type="number"]:focus {
    border-color: #667eea;
  }

  input:disabled {
    background-color: transparent;
    cursor: not-allowed;
    opacity: 0.6;
  }

  input.error {
    color: #ef4444;
  }

  input[type="number"].error {
    border-color: #ef4444;
  }

  select {
    padding: 12px 16px;
    border: 2px solid #e0e0e0;
    border-radius: 8px;
    font-size: 16px;
    background-color: white;
    transition: border-color 0.2s;
  }

  select:focus {
    outline: none;
    border-color: #667eea;
  }

  select:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
    opacity: 0.6;
  }

  .checkbox-group {
    margin-top: 4px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    font-size: 15px;
    color: #333;
    font-weight: 400;
  }

  input[type="checkbox"] {
    width: 18px;
    height: 18px;
    cursor: pointer;
    accent-color: #667eea;
  }

  input[type="checkbox"]:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }

  .field-error {
    display: block;
    margin-top: 6px;
    color: #ef4444;
    font-size: 13px;
  }

  .error-message {
    background-color: #fee;
    border: 1px solid #fcc;
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 20px;
    color: #c33;
    font-size: 14px;
  }

  .success-message {
    background-color: #efe;
    border: 1px solid #cfc;
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 20px;
    color: #3a3;
    font-size: 14px;
  }

  .progress-container {
    background-color: #f9f9f9;
    border: 1px solid #e0e0e0;
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 20px;
  }

  .progress-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
  }

  .progress-text {
    font-size: 14px;
    color: #333;
    font-weight: 500;
  }

  .progress-percentage {
    font-size: 14px;
    color: #667eea;
    font-weight: 600;
  }

  .progress-bar {
    width: 100%;
    height: 8px;
    background-color: #e0e0e0;
    border-radius: 4px;
    overflow: hidden;
    margin-bottom: 10px;
  }

  .progress-fill {
    height: 100%;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    border-radius: 4px;
    transition: width 0.3s ease;
  }

  .progress-details {
    display: flex;
    gap: 16px;
    font-size: 13px;
    color: #666;
  }

  .button-group {
    display: flex;
    gap: 12px;
  }

  button {
    padding: 12px 24px;
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

  .save-button,
  .reset-button {
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: white;
    flex: 1;
  }

  .save-button:hover:not(:disabled),
  .reset-button:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
  }

  .save-button:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .cancel-button {
    background-color: #ef4444;
    color: white;
    flex: 1;
  }

  .cancel-button:hover {
    background-color: #dc2626;
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(239, 68, 68, 0.4);
  }

  /* Responsive design */
  @media (max-width: 768px) {
    .bulk-save-form {
      padding: 20px;
      margin-bottom: 24px;
    }

    .form-title {
      font-size: 20px;
      margin-bottom: 20px;
    }

    .form-row {
      grid-template-columns: 1fr;
      gap: 16px;
    }

    .button-group {
      flex-direction: column;
      width: 100%;
    }

    button {
      width: 100%;
    }

    .progress-header {
      flex-direction: column;
      align-items: flex-start;
      gap: 6px;
    }
  }
</style>
