<script>
  // Props
  export let onSearch;
  export let loading = false;

  // State
  let subreddit = '';
  let sort = 'hot';
  let error = '';
  let touched = false;

  // Validate subreddit name: 3-21 chars, alphanumeric + underscores
  function validateSubreddit(value) {
    if (!value) return 'Subreddit name is required';
    const trimmed = value.trim();
    if (trimmed.length < 3) return 'Name must be at least 3 characters';
    if (trimmed.length > 21) return 'Name must be at most 21 characters';
    if (!/^[a-zA-Z0-9_]+$/.test(trimmed)) {
      return 'Only letters, numbers, and underscores allowed';
    }
    return '';
  }

  $: validationError = touched ? validateSubreddit(subreddit) : '';

  /**
   * Handle form submission
   */
  async function handleSubmit(event) {
    event.preventDefault();
    touched = true;
    error = '';

    if (validationError) {
      return;
    }

    try {
      await onSearch(subreddit.trim(), sort);
    } catch (err) {
      error = err.message || 'Failed to search subreddit';
    }
  }

  /**
   * Handle input changes to clear error
   */
  function handleInput() {
    if (touched && error) {
      error = '';
    }
  }

  /**
   * Clear search
   */
  function handleClear() {
    subreddit = '';
    sort = 'hot';
    touched = false;
    error = '';
  }
</script>

<form on:submit={handleSubmit} class="search-form">
  <div class="search-container">
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
          disabled={loading}
          class:error={validationError}
          placeholder="javascript, programming, learnprogramming"
        />
      </div>
      {#if validationError}
        <span class="field-error">{validationError}</span>
      {/if}
    </div>

    <div class="form-group">
      <label for="sort">Sort By</label>
      <select id="sort" bind:value={sort} disabled={loading}>
        <option value="hot">Hot</option>
        <option value="new">New</option>
        <option value="top">Top</option>
        <option value="rising">Rising</option>
      </select>
    </div>
  </div>

  {#if error}
    <div class="error-message">
      {error}
    </div>
  {/if}

  <div class="button-group">
    <button type="submit" disabled={loading || !!validationError} class="search-button">
      {#if loading}
        <span class="spinner"></span>
        Searching...
      {:else}
        Search
      {/if}
    </button>
    {#if subreddit}
      <button type="button" on:click={handleClear} disabled={loading} class="clear-button">
        Clear
      </button>
    {/if}
  </div>
</form>

<style>
  .search-form {
    background: white;
    border-radius: 12px;
    padding: 28px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    margin-bottom: 32px;
  }

  .search-container {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 20px;
    margin-bottom: 20px;
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

  input {
    flex: 1;
    padding: 12px 16px;
    border: none;
    font-size: 16px;
    background-color: transparent;
  }

  input:focus {
    outline: none;
  }

  input:disabled {
    background-color: transparent;
    cursor: not-allowed;
    opacity: 0.6;
  }

  input.error {
    color: #ef4444;
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

  .search-button {
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: white;
    flex: 1;
  }

  .search-button:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
  }

  .search-button:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .clear-button {
    background-color: #f0f0f0;
    color: #333;
  }

  .clear-button:hover:not(:disabled) {
    background-color: #e0e0e0;
  }

  .clear-button:disabled {
    opacity: 0.6;
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

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Responsive design */
  @media (max-width: 768px) {
    .search-form {
      padding: 20px;
      margin-bottom: 24px;
    }

    .search-container {
      grid-template-columns: 1fr;
      gap: 16px;
    }

    .button-group {
      width: 100%;
    }

    .search-button {
      width: 100%;
    }

    .clear-button {
      flex: 1;
    }
  }
</style>
