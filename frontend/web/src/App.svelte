<script>
  import { onMount, onDestroy } from 'svelte';
  import Login from './Login.svelte';
  import BulkSavePosts from './BulkSavePosts.svelte';
  import SubredditSearch from './SubredditSearch.svelte';
  import PostsList from './PostsList.svelte';
  import SavedPostsList from './SavedPostsList.svelte';
  import CommentsView from './CommentsView.svelte';
  import { checkAuth, logout, fetchSubredditPosts, fetchPostComments, fetchSavedComments, APIError } from './api.js';
  import { sanitizePost, sanitizeComment, sanitizeText, sanitizeNumber } from './utils/sanitize.js';

  // Authentication state
  let authenticated = false;
  let token = null;
  let username = '';
  let userInfo = null;
  let loading = true;
  let error = '';

  // Posts and subreddit browsing state
  let currentSubreddit = '';
  let currentSort = 'hot';
  let posts = [];
  let afterFullname = '';
  let beforeFullname = '';
  let postsLoading = false;
  let postsError = '';

  // Comments state
  let selectedPost = null;
  let comments = [];
  let commentsLoading = false;
  let commentsError = '';
  let showCommentsModal = false;
  let commentSource = 'live'; // 'live' or 'saved'
  let currentlyFetchingPostId = null; // Track which post we're currently fetching comments for

  // View state
  let currentView = 'browse'; // 'browse' or 'saved'

  // SavedPostsList component reference for triggering reloads
  let savedPostsListKey = 0;

  // Request cancellation
  let searchAbortController = null;
  let commentsAbortController = null;

  /**
   * Check if user is already authenticated on mount
   */
  onMount(async () => {
    // Check if we have a token in memory
    // (In a real app, you might use sessionStorage or localStorage)
    loading = false;
  });

  /**
   * Cleanup on component destroy
   * Abort any in-flight requests and prevent state updates after unmount
   */
  onDestroy(() => {
    // Abort any in-flight search requests
    if (searchAbortController) {
      searchAbortController.abort();
      searchAbortController = null;
    }

    // Abort any in-flight comments requests
    if (commentsAbortController) {
      commentsAbortController.abort();
      commentsAbortController = null;
    }
  });

  /**
   * Handle successful login
   */
  async function handleLoginSuccess(authToken, authUsername) {
    token = authToken;
    username = authUsername;

    // Fetch user info
    try {
      userInfo = await checkAuth(token);
      authenticated = true;
    } catch (err) {
      console.error('Failed to fetch user info:', err);
      error = 'Failed to load user information';
      // Still mark as authenticated since we have a valid token
      authenticated = true;
    }
  }

  /**
   * Handle logout
   */
  async function handleLogout() {
    try {
      if (token) {
        await logout(token);
      }
    } catch (err) {
      console.error('Logout error:', err);
    } finally {
      // Clear state regardless of logout API success
      token = null;
      username = '';
      userInfo = null;
      authenticated = false;
      currentSubreddit = '';
      posts = [];
      selectedPost = null;
      comments = [];
      showCommentsModal = false;
      currentView = 'browse';
    }
  }

  /**
   * Handle subreddit search with request cancellation
   */
  async function handleSearch(subreddit, sort) {
    // Cancel previous search if one is in progress
    if (searchAbortController) {
      searchAbortController.abort();
    }

    // Create new abort controller for this search
    searchAbortController = new AbortController();

    currentSubreddit = subreddit;
    currentSort = sort;
    postsError = '';
    postsLoading = true;
    posts = [];
    afterFullname = '';
    selectedPost = null;
    showCommentsModal = false;

    try {
      const response = await fetchSubredditPosts(token, subreddit, sort, '', 25, searchAbortController.signal);

      // Sanitize all posts from the response
      const sanitizedPosts = (response.posts || []).map(sanitizePost).filter(Boolean);

      posts = sanitizedPosts;
      afterFullname = sanitizeText(response.after_fullname || '', 100);
      beforeFullname = sanitizeText(response.before_fullname || '', 100);
    } catch (err) {
      // Ignore abort errors (user-initiated cancellation)
      if (err.name === 'AbortError') {
        return;
      }

      if (err instanceof APIError) {
        if (err.status === 404) {
          postsError = 'Subreddit not found';
        } else if (err.status === 403) {
          postsError = 'Subreddit is private or banned';
        } else {
          postsError = err.message || 'Failed to load posts';
        }
      } else {
        postsError = 'Network error. Please try again.';
      }
      posts = [];
    } finally {
      postsLoading = false;
    }
  }

  /**
   * Handle load more posts
   */
  async function handleLoadMore() {
    if (!afterFullname || !currentSubreddit) return;

    postsError = '';
    postsLoading = true;

    try {
      const response = await fetchSubredditPosts(token, currentSubreddit, currentSort, afterFullname, 25);

      // Sanitize all posts from the response
      const sanitizedPosts = (response.posts || []).map(sanitizePost).filter(Boolean);

      posts = [...posts, ...sanitizedPosts];
      afterFullname = sanitizeText(response.after_fullname || '', 100);
      beforeFullname = sanitizeText(response.before_fullname || '', 100);
    } catch (err) {
      postsError = 'Failed to load more posts';
      console.error('Load more error:', err);
    } finally {
      postsLoading = false;
    }
  }

  /**
   * Handle post selection to view comments
   * Tracks which post is being fetched to prevent race conditions from rapid clicks
   */
  async function handleSelectPost(post, source = 'live') {
    // Sanitize the selected post before displaying
    const sanitizedPost = sanitizePost(post);
    const postId = sanitizedPost.id;

    // Track which post we're currently fetching comments for
    currentlyFetchingPostId = postId;

    // Cancel previous comments fetch if one is in progress
    if (commentsAbortController) {
      commentsAbortController.abort();
    }

    // Create new abort controller for this comments fetch
    commentsAbortController = new AbortController();

    selectedPost = sanitizedPost;
    commentSource = source;
    commentsError = '';
    commentsLoading = true;
    comments = [];
    showCommentsModal = true;

    try {
      let response;

      if (source === 'saved') {
        // For saved posts, try to fetch cached comments
        response = await fetchSavedComments(token, postId, post.subreddit || '');
      } else {
        // For live posts, fetch comments from the API
        response = await fetchPostComments(token, postId, currentSubreddit);
      }

      // Only update state if we're still fetching this specific post
      if (currentlyFetchingPostId === postId) {
        const sanitizedComments = (response.comments || []).map(sanitizeComment).filter(Boolean);
        comments = sanitizedComments;
      }
    } catch (err) {
      // Ignore abort errors (user clicked a different post while this was loading)
      if (err.name === 'AbortError') {
        return;
      }

      // Only update error state if we're still fetching this specific post
      if (currentlyFetchingPostId === postId) {
        if (err instanceof APIError) {
          commentsError = err.message || 'Failed to load comments';
        } else {
          commentsError = 'Network error. Please try again.';
        }
        console.error('Comments fetch error:', err);
      }
    } finally {
      // Only clear loading state if we're still fetching this specific post
      if (currentlyFetchingPostId === postId) {
        commentsLoading = false;
      }
    }
  }

  /**
   * Handle closing comments modal
   */
  function handleCloseComments() {
    showCommentsModal = false;
    selectedPost = null;
    comments = [];
  }

  /**
   * Handle successful bulk save operation
   * Switch to saved posts tab and trigger reload
   */
  function handleBulkSaveSuccess() {
    // Switch to saved posts tab
    currentView = 'saved';

    // Clear any existing browse results
    posts = [];
    currentSubreddit = '';

    // Add a short delay to ensure backend persistence is complete
    // before triggering the saved posts reload
    setTimeout(() => {
      savedPostsListKey += 1;
    }, 100);
  }

  // Computed property for checking if there are more posts
  $: hasMore = !!afterFullname && posts.length > 0;
</script>

<main>
  {#if loading}
    <div class="loading-container">
      <div class="spinner-large"></div>
      <p>Loading...</p>
    </div>
  {:else if !authenticated}
    <!-- Show login screen -->
    <Login onLoginSuccess={handleLoginSuccess} />
  {:else}
    <!-- Show authenticated view -->
    <div class="dashboard">
      <header>
        <div class="header-content">
          <h1>Reddit Dashboard</h1>
          <div class="user-section">
            <div class="user-info">
              <span class="username">u/{username}</span>
              {#if userInfo}
                <span class="karma">
                  {userInfo.link_karma} link · {userInfo.comment_karma} comment
                </span>
              {/if}
            </div>
            <button class="logout-button" on:click={handleLogout}>
              Logout
            </button>
          </div>
        </div>
      </header>

      <!-- Navigation Tabs -->
      <div class="nav-tabs">
        <button
          class="nav-tab"
          class:active={currentView === 'browse'}
          on:click={() => currentView = 'browse'}
        >
          Browse Subreddits
        </button>
        <button
          class="nav-tab"
          class:active={currentView === 'saved'}
          on:click={() => currentView = 'saved'}
        >
          Saved Posts
        </button>
      </div>

      <div class="content">
        {#if currentView === 'browse'}
          <!-- Browse View -->
          {#if !currentSubreddit}
            <!-- Initial welcome state -->
            <div class="welcome-card">
              <h2>Welcome, {username}!</h2>
              <p>You're successfully logged in to Reddit.</p>

              {#if userInfo}
                <div class="stats">
                  <div class="stat">
                    <div class="stat-value">{userInfo.link_karma?.toLocaleString() || 0}</div>
                    <div class="stat-label">Link Karma</div>
                  </div>
                  <div class="stat">
                    <div class="stat-value">{userInfo.comment_karma?.toLocaleString() || 0}</div>
                    <div class="stat-label">Comment Karma</div>
                  </div>
                  <div class="stat">
                    <div class="stat-value">{(userInfo.link_karma + userInfo.comment_karma)?.toLocaleString() || 0}</div>
                    <div class="stat-label">Total Karma</div>
                  </div>
                </div>
              {/if}

              {#if error}
                <div class="error-banner">
                  {error}
                </div>
              {/if}
            </div>
          {/if}

          <!-- Bulk save posts component -->
          <BulkSavePosts token={token} onSuccess={handleBulkSaveSuccess} />

          <!-- Subreddit search component -->
          <SubredditSearch onSearch={handleSearch} loading={postsLoading} />

          <!-- Posts list component -->
          {#if currentSubreddit}
            <PostsList
              posts={posts}
              loading={postsLoading}
              error={postsError}
              onSelectPost={(post) => handleSelectPost(post, 'live')}
              onLoadMore={handleLoadMore}
              afterFullname={afterFullname}
              hasMore={hasMore}
            />
          {/if}
        {:else if currentView === 'saved'}
          <!-- Saved View -->
          {#key savedPostsListKey}
            <SavedPostsList
              token={token}
              onSelectPost={(post) => handleSelectPost(post, 'saved')}
            />
          {/key}
        {/if}

        <!-- Comments modal component -->
        {#if showCommentsModal && selectedPost}
          <CommentsView
            post={selectedPost}
            comments={comments}
            loading={commentsLoading}
            error={commentsError}
            onClose={handleCloseComments}
            source={commentSource}
          />
        {/if}
      </div>
    </div>
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  }

  main {
    min-height: 100vh;
  }

  .loading-container {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    color: #666;
  }

  .spinner-large {
    width: 48px;
    height: 48px;
    border: 4px solid #e0e0e0;
    border-top-color: #667eea;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-bottom: 16px;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .dashboard {
    min-height: 100vh;
    background-color: #f5f7fa;
  }

  header {
    background: white;
    border-bottom: 1px solid #e0e0e0;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);
  }

  .nav-tabs {
    background: white;
    border-bottom: 1px solid #e0e0e0;
    display: flex;
    gap: 0;
  }

  .nav-tab {
    flex: 1;
    padding: 16px 24px;
    border: none;
    background: none;
    color: #666;
    font-size: 15px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
    border-bottom: 3px solid transparent;
    position: relative;
  }

  .nav-tab:hover {
    color: #333;
    background-color: #f9f9f9;
  }

  .nav-tab.active {
    color: #667eea;
    border-bottom-color: #667eea;
  }

  .header-content {
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px 24px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  header h1 {
    margin: 0;
    font-size: 24px;
    color: #1a1a1a;
  }

  .user-section {
    display: flex;
    align-items: center;
    gap: 20px;
  }

  .user-info {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .username {
    font-weight: 600;
    color: #333;
    font-size: 16px;
  }

  .karma {
    font-size: 13px;
    color: #666;
  }

  .logout-button {
    padding: 8px 20px;
    background-color: #ef4444;
    color: white;
    border: none;
    border-radius: 6px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  .logout-button:hover {
    background-color: #dc2626;
  }

  .content {
    max-width: 1200px;
    margin: 0 auto;
    padding: 40px 24px;
  }

  .welcome-card {
    background: white;
    border-radius: 12px;
    padding: 40px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  }

  .welcome-card h2 {
    margin: 0 0 8px 0;
    font-size: 28px;
    color: #1a1a1a;
  }

  .welcome-card > p {
    margin: 0 0 32px 0;
    color: #666;
    font-size: 16px;
  }

  .stats {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 20px;
    margin-bottom: 32px;
  }

  .stat {
    text-align: center;
    padding: 20px;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    border-radius: 10px;
    color: white;
  }

  .stat-value {
    font-size: 32px;
    font-weight: 700;
    margin-bottom: 4px;
  }

  .stat-label {
    font-size: 14px;
    opacity: 0.9;
  }

  .error-banner {
    background-color: #fee;
    border: 1px solid #fcc;
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 24px;
    color: #c33;
    font-size: 14px;
  }

  .placeholder-message {
    background-color: #f9fafb;
    border: 2px dashed #d1d5db;
    border-radius: 10px;
    padding: 32px;
    text-align: center;
  }

  .placeholder-message p {
    margin: 0 0 12px 0;
    font-size: 18px;
    color: #374151;
  }

  .sub-message {
    font-size: 14px !important;
    color: #6b7280 !important;
    margin-bottom: 20px !important;
  }

  .placeholder-message ul {
    text-align: left;
    display: inline-block;
    margin: 0;
    padding-left: 24px;
    color: #4b5563;
  }

  .placeholder-message li {
    margin: 8px 0;
  }

  /* Responsive design */
  @media (max-width: 768px) {
    .header-content {
      flex-direction: column;
      gap: 16px;
      align-items: flex-start;
    }

    .user-section {
      width: 100%;
      justify-content: space-between;
    }

    .stats {
      grid-template-columns: 1fr;
    }

    .welcome-card {
      padding: 24px;
    }

    .nav-tab {
      padding: 14px 16px;
      font-size: 13px;
    }
  }
</style>
