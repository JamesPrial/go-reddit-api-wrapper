/**
 * Alpine.js Application State
 *
 * Centralized state management for the Reddit API Browser application.
 * Provides reactive state properties and methods for posts, comments, authentication,
 * and bulk operations using Alpine.js patterns.
 *
 * All methods use async/await and integrate with the window.api global functions
 * from app.js for API calls.
 */

/**
 * Main application state factory function for Alpine.js
 * Returns a reactive state object with all required properties and methods.
 *
 * @returns {object} Alpine.js reactive state object
 */
function appState() {
  return {
    // ========== Authentication State ==========
    apiKey: '',
    authenticated: false,

    // ========== Navigation State ==========
    view: 'posts', // 'posts' | 'comments' | 'saved' | 'bulk'

    // ========== Posts Browsing State ==========
    subreddit: '',
    sortBy: 'hot', // 'hot' | 'new'
    posts: [],
    pagination: {
      after: '',
      before: '',
    },
    selectedPosts: new Set(),

    // ========== Comments State ==========
    comments: [],
    currentPost: null,
    commentPagination: {
      after: '',
      before: '',
    },

    // ========== Saved Posts State ==========
    savedPosts: [],
    savedFilters: {
      sortBy: 'hot',
      subreddit: '',
    },
    savedPagination: {
      after: '',
      before: '',
    },
    savedComments: [],
    savedCurrentPost: null,

    // ========== Bulk Operations State ==========
    bulkSubreddit: '',
    bulkSort: 'hot',
    bulkLimit: 25,
    bulkProgress: {
      current: 0,
      total: 0,
    },

    // ========== Statistics State ==========
    stats: {
      postCount: 0,
      commentCount: 0,
    },

    // ========== UI State ==========
    loading: false,
    error: '',
    success: '',

    // ========== INITIALIZATION ==========

    /**
     * Initialize the application
     * Loads API key from localStorage, checks authentication, and loads statistics
     */
    async initApp() {
      try {
        const savedKey = window.api.getApiKey();
        if (savedKey) {
          this.apiKey = savedKey;
          // Check if the saved key is still valid
          const isValid = await window.api.checkAuth(savedKey);
          if (isValid) {
            this.authenticated = true;
            await this.loadStorageStats();
          } else {
            this.authenticated = false;
            window.api.clearApiKey();
            this.showError('Saved API key is no longer valid. Please authenticate again.');
          }
        }
      } catch (err) {
        this.showError('Failed to initialize application: ' + err.message);
      }
    },

    // ========== AUTHENTICATION ==========

    /**
     * Authenticate with the provided API key
     * Saves key to localStorage and validates it with the server
     */
    async authenticate() {
      if (!this.apiKey || !this.apiKey.trim()) {
        this.showError('Please enter an API key');
        return;
      }

      this.loading = true;
      this.error = '';
      this.success = '';

      try {
        const trimmedKey = this.apiKey.trim();
        const isValid = await window.api.checkAuth(trimmedKey);

        if (!isValid) {
          this.showError('Invalid API key. Please check and try again.');
          return;
        }

        window.api.saveApiKey(trimmedKey);
        this.authenticated = true;
        this.showSuccess('Authentication successful!');
        await this.loadStorageStats();
      } catch (err) {
        this.showError('Authentication failed: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Logout and clear all authentication data
     */
    logout() {
      if (confirm('Are you sure you want to logout? Your API key will be removed.')) {
        window.api.clearApiKey();
        this.apiKey = '';
        this.authenticated = false;
        this.posts = [];
        this.comments = [];
        this.selectedPosts.clear();
        this.subreddit = '';
        this.view = 'posts';
        this.showSuccess('Logged out successfully');
      }
    },

    // ========== POSTS BROWSING ==========

    /**
     * Fetch posts from Reddit based on current subreddit and sort settings
     */
    async fetchPosts() {
      if (!this.subreddit || !this.subreddit.trim()) {
        this.showError('Please enter a subreddit name');
        return;
      }

      this.loading = true;
      this.error = '';
      this.selectedPosts.clear();

      try {
        const sortFunc = this.sortBy === 'hot' ? window.api.fetchHotPosts : window.api.fetchNewPosts;
        const result = await sortFunc({
          subreddit: this.subreddit.trim(),
          limit: 25,
        });

        this.posts = result.posts || [];
        this.pagination.after = result.after || '';
        this.pagination.before = result.before || '';

        if (this.posts.length === 0) {
          this.showError('No posts found. Try a different subreddit.');
        } else {
          this.showSuccess(`Loaded ${this.posts.length} posts`);
        }
      } catch (err) {
        this.showError('Failed to fetch posts: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Load the next page of posts
     */
    async nextPage() {
      if (!this.pagination.after) {
        this.showError('No more posts available');
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const sortFunc = this.sortBy === 'hot' ? window.api.fetchHotPosts : window.api.fetchNewPosts;
        const result = await sortFunc({
          subreddit: this.subreddit.trim(),
          limit: 25,
          after: this.pagination.after,
        });

        this.posts = result.posts || [];
        this.pagination.after = result.after || '';
        this.pagination.before = result.before || '';
        this.selectedPosts.clear();
      } catch (err) {
        this.showError('Failed to load next page: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Load the previous page of posts
     */
    async previousPage() {
      if (!this.pagination.before) {
        this.showError('No previous posts available');
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const sortFunc = this.sortBy === 'hot' ? window.api.fetchHotPosts : window.api.fetchNewPosts;
        const result = await sortFunc({
          subreddit: this.subreddit.trim(),
          limit: 25,
          before: this.pagination.before,
        });

        this.posts = result.posts || [];
        this.pagination.after = result.after || '';
        this.pagination.before = result.before || '';
        this.selectedPosts.clear();
      } catch (err) {
        this.showError('Failed to load previous page: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * View comments for a specific post
     * Switches view to comments and fetches comment data
     *
     * @param {object} post - The post object to view comments for
     */
    async viewComments(post) {
      this.loading = true;
      this.error = '';

      try {
        // Extract subreddit and post ID from the post
        const subreddit = post.data.subreddit;
        const postId = post.data.id;

        const result = await window.api.fetchComments(subreddit, postId, {
          limit: 25,
        });

        this.currentPost = result.post;
        this.comments = result.comments || [];
        this.commentPagination.after = result.after || '';
        this.commentPagination.before = result.before || '';
        this.view = 'comments';
      } catch (err) {
        this.showError('Failed to load comments: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Return to posts list from comments view
     */
    backToPostsList() {
      this.view = 'posts';
      this.currentPost = null;
      this.comments = [];
      this.commentPagination = { after: '', before: '' };
    },

    /**
     * Load more comments (expand comment thread)
     *
     * @param {object} commentData - The comment data object containing more_replies info
     */
    async loadMoreComments(commentData) {
      if (!commentData.more_replies || !Array.isArray(commentData.more_children)) {
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const linkId = this.currentPost.data.name; // e.g., "t3_abc123"
        const children = commentData.more_children.slice(0, 100);

        const result = await window.api.fetchMoreComments(linkId, children);
        if (result.comments && result.comments.length > 0) {
          this.comments = this.comments.concat(result.comments);
          commentData.more_replies = false;
        }
      } catch (err) {
        this.showError('Failed to load more comments: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    // ========== POST SELECTION ==========

    /**
     * Toggle selection state for a post
     *
     * @param {string} postId - The ID of the post to toggle
     */
    togglePostSelection(postId) {
      if (this.selectedPosts.has(postId)) {
        this.selectedPosts.delete(postId);
      } else {
        this.selectedPosts.add(postId);
      }
      // Force Alpine.js reactivity by triggering a state change
      this.selectedPosts = new Set(this.selectedPosts);
    },

    /**
     * Select all currently visible posts
     */
    selectAllPosts() {
      this.posts.forEach(post => {
        this.selectedPosts.add(post.data.id);
      });
      this.selectedPosts = new Set(this.selectedPosts);
    },

    /**
     * Deselect all posts
     */
    deselectAllPosts() {
      this.selectedPosts.clear();
      this.selectedPosts = new Set();
    },

    /**
     * Check if a post is selected
     *
     * @param {string} postId - The ID of the post to check
     * @returns {boolean} True if the post is selected
     */
    isPostSelected(postId) {
      return this.selectedPosts.has(postId);
    },

    /**
     * Get count of selected posts
     *
     * @returns {number} Number of selected posts
     */
    getSelectedCount() {
      return this.selectedPosts.size;
    },

    // ========== SAVE OPERATIONS ==========

    /**
     * Save a single post (stub implementation)
     * In a real implementation, this would persist to a backend storage
     *
     * @param {object} post - The post to save
     */
    async savePost(post) {
      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, this would make an API call
        // await window.api.savePost(post);
        this.showSuccess(`Saved post: ${post.data.title.substring(0, 50)}...`);
      } catch (err) {
        this.showError('Failed to save post: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Save all selected posts (stub implementation)
     * Saves posts that are currently selected in the UI
     */
    async saveSelectedPosts() {
      if (this.selectedPosts.size === 0) {
        this.showError('No posts selected');
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const postsToSave = this.posts.filter(post =>
          this.selectedPosts.has(post.data.id)
        );

        // Stub: In production, would batch save to backend
        // await window.api.saveMultiplePosts(postsToSave);

        const count = postsToSave.length;
        this.showSuccess(`Saved ${count} post${count !== 1 ? 's' : ''}`);
        this.selectedPosts.clear();
      } catch (err) {
        this.showError('Failed to save posts: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Save current post along with its comments (stub implementation)
     */
    async saveCurrentPostWithComments() {
      if (!this.currentPost) {
        this.showError('No post is currently being viewed');
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, would persist post + comments to backend
        // await window.api.savePostWithComments(this.currentPost, this.comments);

        this.showSuccess(
          `Saved post with ${this.comments.length} comment${this.comments.length !== 1 ? 's' : ''}`
        );
      } catch (err) {
        this.showError('Failed to save post with comments: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Bulk download and save posts from a subreddit (stub implementation)
     * Fetches multiple pages of posts and saves them
     */
    async bulkSaveFromSubreddit() {
      if (!this.bulkSubreddit || !this.bulkSubreddit.trim()) {
        this.showError('Please enter a subreddit name');
        return;
      }

      this.loading = true;
      this.error = '';
      this.bulkProgress.current = 0;
      this.bulkProgress.total = this.bulkLimit;

      try {
        const sortFunc = this.bulkSort === 'hot' ? window.api.fetchHotPosts : window.api.fetchNewPosts;
        const allPosts = [];
        let after = '';
        let remaining = this.bulkLimit;

        while (remaining > 0) {
          const batchSize = Math.min(remaining, 25);
          const result = await sortFunc({
            subreddit: this.bulkSubreddit.trim(),
            limit: batchSize,
            after: after,
          });

          if (!result.posts || result.posts.length === 0) {
            break;
          }

          allPosts.push(...result.posts);
          this.bulkProgress.current += result.posts.length;
          remaining -= result.posts.length;

          if (!result.after) {
            break;
          }
          after = result.after;

          // Small delay to avoid rate limiting
          await new Promise(resolve => setTimeout(resolve, 100));
        }

        // Stub: In production, would save all posts to backend
        // await window.api.saveBulkPosts(allPosts);

        this.showSuccess(
          `Bulk saved ${allPosts.length} post${allPosts.length !== 1 ? 's' : ''}`
        );
        this.bulkProgress.current = 0;
        this.bulkProgress.total = 0;
      } catch (err) {
        this.showError('Bulk save failed: ' + err.message);
        this.bulkProgress.current = 0;
        this.bulkProgress.total = 0;
      } finally {
        this.loading = false;
      }
    },

    // ========== SAVED POSTS ==========

    /**
     * Load saved posts (stub implementation)
     * In production, would fetch from backend storage
     */
    async loadSavedPosts() {
      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, would fetch from backend
        // const result = await window.api.getSavedPosts({
        //   sortBy: this.savedFilters.sortBy,
        //   subreddit: this.savedFilters.subreddit,
        // });
        // this.savedPosts = result.posts || [];
        // this.savedPagination = { after: result.after, before: result.before };

        this.showSuccess('Loaded saved posts');
        this.view = 'saved';
      } catch (err) {
        this.showError('Failed to load saved posts: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * View comments for a saved post (stub implementation)
     *
     * @param {object} post - The saved post to view
     */
    async viewSavedComments(post) {
      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, would fetch from backend storage
        // const result = await window.api.getSavedComments(post.id);
        // this.savedCurrentPost = post;
        // this.savedComments = result.comments || [];

        this.savedCurrentPost = post;
      } catch (err) {
        this.showError('Failed to load comments: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Delete a saved post (stub implementation)
     *
     * @param {string} postId - The ID of the post to delete
     */
    async deleteSavedPost(postId) {
      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, would delete from backend
        // await window.api.deleteSavedPost(postId);

        this.savedPosts = this.savedPosts.filter(p => p.data.id !== postId);
        this.showSuccess('Post deleted');
      } catch (err) {
        this.showError('Failed to delete post: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Load next page of saved posts (stub implementation)
     */
    async savedNextPage() {
      if (!this.savedPagination.after) {
        this.showError('No more posts available');
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, would fetch next page from backend
        // const result = await window.api.getSavedPosts({
        //   sortBy: this.savedFilters.sortBy,
        //   subreddit: this.savedFilters.subreddit,
        //   after: this.savedPagination.after,
        // });
        // this.savedPosts = result.posts || [];
        // this.savedPagination = { after: result.after, before: result.before };
      } catch (err) {
        this.showError('Failed to load next page: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Load previous page of saved posts (stub implementation)
     */
    async savedPreviousPage() {
      if (!this.savedPagination.before) {
        this.showError('No previous posts available');
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        // Stub: In production, would fetch previous page from backend
        // const result = await window.api.getSavedPosts({
        //   sortBy: this.savedFilters.sortBy,
        //   subreddit: this.savedFilters.subreddit,
        //   before: this.savedPagination.before,
        // });
        // this.savedPosts = result.posts || [];
        // this.savedPagination = { after: result.after, before: result.before };
      } catch (err) {
        this.showError('Failed to load previous page: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    // ========== STATISTICS ==========

    /**
     * Load storage statistics
     * Updates stats about saved posts and comments
     */
    async loadStorageStats() {
      try {
        // Stub: In production, would fetch from backend
        // const result = await window.api.getStorageStats();
        // this.stats.postCount = result.postCount || 0;
        // this.stats.commentCount = result.commentCount || 0;

        // For now, initialize with defaults
        this.stats.postCount = 0;
        this.stats.commentCount = 0;
      } catch (err) {
        console.error('Failed to load storage stats:', err.message);
      }
    },

    // ========== UI HELPERS ==========

    /**
     * Display an error message
     * Auto-clears after 3 seconds
     *
     * @param {string} message - The error message to display
     */
    showError(message) {
      this.error = message;
      this.success = '';
      setTimeout(() => {
        if (this.error === message) {
          this.error = '';
        }
      }, 3000);
    },

    /**
     * Display a success message
     * Auto-clears after 3 seconds
     *
     * @param {string} message - The success message to display
     */
    showSuccess(message) {
      this.success = message;
      this.error = '';
      setTimeout(() => {
        if (this.success === message) {
          this.success = '';
        }
      }, 3000);
    },

    /**
     * Clear all messages
     */
    clearMessages() {
      this.error = '';
      this.success = '';
    },

    /**
     * Check if any posts are selected
     *
     * @returns {boolean} True if at least one post is selected
     */
    hasSelectedPosts() {
      return this.selectedPosts.size > 0;
    },

    /**
     * Check if all visible posts are selected
     *
     * @returns {boolean} True if all visible posts are selected
     */
    areAllPostsSelected() {
      if (this.posts.length === 0) return false;
      return this.posts.every(post => this.selectedPosts.has(post.data.id));
    },

    /**
     * Format a number for display
     *
     * @param {number} num - The number to format
     * @returns {string} Formatted number
     */
    formatNumber(num) {
      return window.api.formatScore(num);
    },

    /**
     * Format a timestamp for display
     *
     * @param {number} timestamp - Unix timestamp
     * @returns {string} Human-readable relative time
     */
    formatTime(timestamp) {
      return window.api.formatTimestamp(timestamp);
    },

    /**
     * Truncate text for display
     *
     * @param {string} text - The text to truncate
     * @param {number} maxLength - Maximum length
     * @returns {string} Truncated text
     */
    truncate(text, maxLength) {
      return window.api.truncateText(text, maxLength);
    },
  };
}

// Make appState globally available for Alpine.js
if (typeof window !== 'undefined') {
  window.appState = appState;
}
