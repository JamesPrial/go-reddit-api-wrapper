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
    // ========== Configuration ==========
    PAGE_SIZE: 25,

    // ========== Authentication State ==========
    username: '',
    password: '',
    authenticated: false,
    user: null,

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
      author: '',
      minScore: null,
      sortBy: 'created_utc',
      subreddit: '',
    },
    savedPagination: {
      offset: 0,
      limit: 25,
      total: 0,
    },
    savedComments: [],
    savedCurrentPost: null,

    // ========== Bulk Operations State ==========
    bulkSubreddit: '',
    bulkSort: 'hot',
    bulkLimit: 25,
    bulkProgress: {
      saved: 0,
      total: 0,
    },

    // ========== Statistics State ==========
    stats: {
      postCount: 0,
      commentCount: 0,
    },

    // ========== Monitor State ==========
    monitorStatus: 'stopped', // 'stopped' | 'running'
    monitorId: '',
    monitorStartedAt: null,
    monitorSubreddits: [], // Array of subreddit names for tag-style input
    currentSubredditInput: '', // Temporary input field value
    monitorInterval: '30s', // Selected interval
    monitorLimit: 25, // Posts per fetch
    monitorFetchComments: true,
    monitorStats: {
      totalFetches: 0,
      totalPosts: 0,
      totalComments: 0,
      lastFetchTime: null,
      lastError: '',
    },
    monitorRefreshInterval: null, // Timer ID for auto-refresh
    monitorStatusLoading: false,  // Prevents concurrent status loads

    // ========== UI State ==========
    loading: false,
    error: '',
    success: '',
    _messageTimeoutId: null,

    // ========== LIFECYCLE ==========

    /**
     * Initialize reactive watchers
     * Called automatically by Alpine.js when component mounts
     */
    init() {
      // Clean up auto-refresh when leaving monitor view
      this.$watch('view', (newView, oldView) => {
        if (oldView === 'monitor' && newView !== 'monitor') {
          this.clearAutoRefresh();
        }
      });
    },

    // ========== INITIALIZATION ==========

    /**
     * Initialize the application
     * Checks for existing JWT token, validates it, and loads statistics
     */
    async initApp() {
      try {
        // Check if user has valid JWT token
        if (window.api.auth.isAuthenticated()) {
          // Token exists and not expired - verify it's still valid
          try {
            const user = await window.api.getCurrentUser();
            this.authenticated = true;
            this.user = window.api.auth.user || user;
            await this.loadStorageStats();
            // Load monitor status on startup (show errors if it fails)
            await this.loadMonitorStatus(false);
          } catch (err) {
            // Token invalid or expired - clear auth
            window.api.auth.clearAuth();
            this.authenticated = false;
            this.user = null;
            this.showError('Your session has expired. Please log in again.');
          }
        }
      } catch (err) {
        this.showError('Failed to initialize application: ' + err.message);
      }
    },

    // ========== AUTHENTICATION ==========

    /**
     * Authenticate with username and password to obtain JWT token
     */
    async authenticate() {
      if (!this.username || !this.username.trim()) {
        this.showError('Please enter a username');
        return;
      }

      if (!this.password || !this.password.trim()) {
        this.showError('Please enter a password');
        return;
      }

      this.loading = true;
      this.error = '';
      this.success = '';

      try {
        const result = await window.api.login(this.username.trim(), this.password);

        this.authenticated = true;
        this.user = result.user;
        this.password = ''; // Clear password from memory
        this.showSuccess('Login successful!');
        await this.loadStorageStats();
      } catch (err) {
        this.showError('Login failed: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Logout and clear all authentication data
     */
    async logout() {
      if (confirm('Are you sure you want to logout?')) {
        await window.api.logout();
        this.username = '';
        this.password = '';
        this.authenticated = false;
        this.user = null;
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

      this.selectedPosts.clear();
      await this._fetchPostsWithParams({});

      if (this.posts.length === 0) {
        this.showError('No posts found. Try a different subreddit.');
      } else {
        this.showSuccess(`Loaded ${this.posts.length} posts`);
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

      await this._fetchPostsWithParams({ after: this.pagination.after });
    },

    /**
     * Load the previous page of posts
     */
    async previousPage() {
      if (!this.pagination.before) {
        this.showError('No previous posts available');
        return;
      }

      await this._fetchPostsWithParams({ before: this.pagination.before });
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
        const subreddit = post.subreddit;
        const postId = post.id;

        const result = await window.api.fetchComments(subreddit, postId, {
          limit: this.PAGE_SIZE,
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
     * Internal helper to fetch posts with pagination parameters
     * Reduces duplication across fetchPosts(), nextPage(), and previousPage()
     * @private
     */
    async _fetchPostsWithParams(params) {
      this.loading = true;
      this.error = '';

      try {
        const sortFunc = this.sortBy === 'hot' ? window.api.fetchHotPosts : window.api.fetchNewPosts;
        const result = await sortFunc({
          subreddit: this.subreddit.trim(),
          limit: this.PAGE_SIZE,
          ...params
        });

        this.posts = result.posts || [];
        this.pagination.after = result.after || '';
        this.pagination.before = result.before || '';
        this.selectedPosts.clear();
      } catch (err) {
        this.showError('Failed to load posts: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Load more comments (expand comment thread)
     *
     * @param {object} commentData - The comment data object containing more_replies info
     */
    async loadMoreComments(commentData) {
      if (!this.currentPost) {
        this.showError('No post is currently being viewed');
        return;
      }

      if (!commentData.more_replies || !Array.isArray(commentData.more_children)) {
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const linkId = this.currentPost.name; // e.g., "t3_abc123"
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
      // Force Alpine.js reactivity: Set mutations don't trigger updates automatically
      this.selectedPosts = new Set(this.selectedPosts);
    },

    /**
     * Select all currently visible posts
     */
    selectAllPosts() {
      this.posts.forEach(post => {
        this.selectedPosts.add(post.id);
      });
      // Force Alpine.js reactivity: Set mutations don't trigger updates automatically
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
        await window.api.savePost(post);
        await this.loadStorageStats();
        this.showSuccess(`Saved post: ${post.title.substring(0, 50)}...`);
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
          this.selectedPosts.has(post.id)
        );

        for (const post of postsToSave) {
          await window.api.savePost(post);
        }
        await this.loadStorageStats();

        const count = postsToSave.length;
        this.showSuccess(`Saved ${count} post${this.pluralize(count)}`);
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
        await window.api.savePost(this.currentPost);
        if (this.comments.length > 0) {
          await window.api.saveComments(this.currentPost.id, this.comments);
        }
        await this.loadStorageStats();

        this.showSuccess(
          `Saved post with ${this.comments.length} comment${this.pluralize(this.comments.length)}`
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
      this.bulkProgress.saved = 0;
      this.bulkProgress.total = this.bulkLimit;

      try {
        const result = await window.api.bulkSaveFromSubreddit(
          this.bulkSubreddit.trim(),
          this.bulkSort,
          this.bulkLimit
        );

        await this.loadStorageStats();

        this.showSuccess(
          `Bulk saved ${result.saved} post${this.pluralize(result.saved)}`
        );
        this.bulkProgress.saved = 0;
        this.bulkProgress.total = 0;
      } catch (err) {
        this.showError('Bulk save failed: ' + err.message);
        this.bulkProgress.saved = 0;
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
        const result = await window.api.listSavedPosts(
          {
            subreddit: this.savedFilters.subreddit || undefined,
            author: this.savedFilters.author || undefined,
            min_score: this.savedFilters.minScore || undefined,
            sort_by: this.savedFilters.sortBy,
            sort_dir: 'desc'
          },
          {
            limit: this.savedPagination.limit,
            offset: this.savedPagination.offset
          }
        );
        this.savedPosts = result.posts || [];
        this.savedPagination.total = result.total || 0;

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
        const result = await window.api.getSavedComments(post.id, {});
        this.savedCurrentPost = post;
        this.savedComments = result.comments || [];
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
        await window.api.deleteSavedPost(postId);
        await this.loadStorageStats();

        this.savedPosts = this.savedPosts.filter(p => p.id !== postId);
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
      if (this.savedPagination.offset + this.savedPagination.limit >= this.savedPagination.total) {
        this.showError('No more posts available');
        return;
      }

      this.savedPagination.offset += this.savedPagination.limit;
      await this.loadSavedPosts();
    },

    /**
     * Load previous page of saved posts (stub implementation)
     */
    async savedPreviousPage() {
      if (this.savedPagination.offset === 0) {
        this.showError('No previous posts available');
        return;
      }

      this.savedPagination.offset = Math.max(0, this.savedPagination.offset - this.savedPagination.limit);
      await this.loadSavedPosts();
    },

    // ========== STATISTICS ==========

    /**
     * Load storage statistics
     * Updates stats about saved posts and comments
     */
    async loadStorageStats() {
      try {
        const result = await window.api.getStorageStats();
        this.stats.postCount = result.post_count || 0;
        this.stats.commentCount = result.comment_count || 0;
      } catch (err) {
        // Silently log stats errors to avoid disrupting save operations
        console.error('Failed to load storage stats:', err.message);
        // Keep existing stats on failure
      }
    },

    // ========== MONITOR OPERATIONS ==========

    /**
     * Add a subreddit to the monitor list
     * Validates and adds the subreddit from the input field
     */
    addMonitorSubreddit() {
      const subreddit = this.currentSubredditInput.trim();

      // Validation
      if (!subreddit) {
        this.error = 'Subreddit name cannot be empty';
        return;
      }

      if (subreddit.length > 21) {
        this.error = 'Subreddit name cannot exceed 21 characters';
        return;
      }

      // Add format validation - must start with letter, only letters/numbers/underscores
      const subredditRegex = /^[a-zA-Z][a-zA-Z0-9_]*$/;
      if (!subredditRegex.test(subreddit)) {
        this.error = 'Invalid subreddit name format (must start with letter, contain only letters, numbers, and underscores)';
        return;
      }

      // Normalize to lowercase for duplicate checking (subreddits are case-insensitive)
      const normalizedSubreddit = subreddit.toLowerCase();
      if (this.monitorSubreddits.map(s => s.toLowerCase()).includes(normalizedSubreddit)) {
        this.error = 'Subreddit already added';
        return;
      }

      if (this.monitorSubreddits.length >= 10) {
        this.error = 'Maximum 10 subreddits allowed';
        return;
      }

      // Add to list
      this.monitorSubreddits.push(subreddit);
      this.currentSubredditInput = '';
      this.error = '';  // Clear any previous errors
    },

    /**
     * Remove a subreddit from the monitor list
     *
     * @param {number} index - The index of the subreddit to remove
     */
    removeMonitorSubreddit(index) {
      this.monitorSubreddits.splice(index, 1);
    },

    /**
     * Start the monitor with current configuration
     * Validates settings and starts monitoring the configured subreddits
     */
    async startMonitor() {
      // Validate at least one subreddit
      if (this.monitorSubreddits.length === 0) {
        this.error = 'Please add at least one subreddit to monitor';
        return;
      }

      this.loading = true;
      this.error = '';

      // Reset stats for new monitor (prevents showing old stats)
      this.monitorStats = {
        totalFetches: 0,
        totalPosts: 0,
        totalComments: 0,
        lastFetchTime: null,
        lastError: '',
      };

      try {
        // Build config object
        const config = {
          subreddits: this.monitorSubreddits,
          interval: this.monitorInterval,
          limit: this.monitorLimit,
          fetch_comments: this.monitorFetchComments,
        };

        const result = await window.api.startMonitor(config);

        // Update state on success
        this.monitorStatus = 'running';
        this.monitorId = result.id || '';
        this.monitorStartedAt = result.started_at || null;

        // Start auto-refresh
        this.pollMonitorStatus();

        this.showSuccess('Monitor started successfully');
      } catch (err) {
        this.showError('Failed to start monitor: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Stop the currently running monitor
     */
    async stopMonitor() {
      // Add confirmation dialog
      if (!confirm('Are you sure you want to stop the monitor? This will end the current monitoring session.')) {
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const result = await window.api.stopMonitor();

        // Update state on success
        this.monitorStatus = 'stopped';

        // Save final stats
        if (result.stats) {
          this.monitorStats = {
            totalFetches: result.stats.total_fetches || 0,
            totalPosts: result.stats.total_posts || 0,
            totalComments: result.stats.total_comments || 0,
            lastFetchTime: result.stats.last_fetch_time || null,
            lastError: result.stats.last_error || '',
          };
        }

        // Clear refresh interval
        if (this.monitorRefreshInterval) {
          clearInterval(this.monitorRefreshInterval);
          this.monitorRefreshInterval = null;
        }

        this.showSuccess('Monitor stopped successfully');
      } catch (err) {
        this.showError('Failed to stop monitor: ' + err.message);
      } finally {
        this.loading = false;
      }
    },

    /**
     * Load the current monitor status from the server
     * Updates monitor state with the latest information
     */
    async loadMonitorStatus(silent = true) {
      // Prevent concurrent loads
      if (this.monitorStatusLoading) {
        return;
      }

      this.monitorStatusLoading = true;
      try {
        const result = await window.api.getMonitorStatus();

        if (result.status === 'running') {
          // Update all monitor state
          this.monitorStatus = 'running';
          this.monitorId = result.id || '';
          this.monitorStartedAt = result.started_at || null;

          if (result.config) {
            this.monitorSubreddits = result.config.subreddits || [];
            this.monitorInterval = result.config.interval || '30s';
            this.monitorLimit = result.config.limit || 25;
            this.monitorFetchComments = result.config.fetch_comments !== false;
          }

          if (result.stats) {
            this.monitorStats = {
              totalFetches: result.stats.total_fetches || 0,
              totalPosts: result.stats.total_posts || 0,
              totalComments: result.stats.total_comments || 0,
              lastFetchTime: result.stats.last_fetch_time || null,
              lastError: result.stats.last_error || '',
            };
          }

          // Start auto-refresh if not already running
          if (!this.monitorRefreshInterval) {
            this.pollMonitorStatus();
          }
        } else {
          // Not running - update status and clear refresh
          this.monitorStatus = 'stopped';
          this.clearAutoRefresh();
        }
      } catch (err) {
        if (silent) {
          console.error('Failed to load monitor status:', err.message);
        } else {
          this.error = 'Failed to load monitor status: ' + err.message;
        }
      } finally {
        this.monitorStatusLoading = false;
      }
    },

    /**
     * Auto-refresh monitor status
     * Polls the server every 5 seconds to update monitor stats
     */
    pollMonitorStatus() {
      // Clear any existing interval first
      if (this.monitorRefreshInterval) {
        clearInterval(this.monitorRefreshInterval);
      }

      // Poll status every 5 seconds
      this.monitorRefreshInterval = setInterval(() => {
        this.loadMonitorStatus(true);  // Silent for auto-refresh
      }, 5000);
    },

    /**
     * Clear auto-refresh interval (prevents memory leaks)
     */
    clearAutoRefresh() {
      if (this.monitorRefreshInterval) {
        clearInterval(this.monitorRefreshInterval);
        this.monitorRefreshInterval = null;
      }
    },

    // ========== SERVER MANAGEMENT ==========

    /**
     * Shutdown the server
     * Displays a confirmation dialog and initiates graceful server shutdown
     *
     * Expected flow:
     * 1. Client sends POST /api/v1/shutdown
     * 2. Server returns 202 Accepted with {"message": "server shutdown initiated"}
     * 3. Client shows success message
     * 4. Server completes graceful shutdown (30s timeout)
     * 5. Connection drops (client may be disconnected by then)
     */
    async shutdown() {
      if (!confirm('Are you sure you want to shutdown the server? This will terminate all active connections.')) {
        return;
      }

      this.loading = true;
      this.error = '';

      try {
        const result = await window.api.shutdownServer();
        // Server returned 202 Accepted - shutdown initiated successfully
        this.success = result.message || 'Server shutdown initiated';
        this.loading = false;
      } catch (err) {
        // True errors: 401 auth, 503 service unavailable, network failure
        // Check for authentication failure (401)
        if (err.message.includes('Authentication required')) {
          this.error = err.message;
        } else {
          // All other errors are real failures (network timeout, 503, etc)
          this.error = 'Failed to shutdown server: ' + err.message;
        }
        this.loading = false;
      }
    },

    // ========== UI HELPERS ==========

    /**
     * Helper for grammatically correct pluralization
     * @param {number} count - The count to check
     * @param {string} singular - Singular suffix (default: '')
     * @param {string} plural - Plural suffix (default: 's')
     * @returns {string} Appropriate suffix
     */
    pluralize(count, singular = '', plural = 's') {
      return count === 1 ? singular : plural;
    },

    /**
     * Display an error message
     * Auto-clears after 3 seconds
     *
     * @param {string} message - The error message to display
     */
    showError(message) {
      this.error = message;
      this.success = '';

      // Clear any existing timeout
      if (this._messageTimeoutId) {
        clearTimeout(this._messageTimeoutId);
      }

      this._messageTimeoutId = setTimeout(() => {
        this.error = '';
        this._messageTimeoutId = null;
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

      // Clear any existing timeout
      if (this._messageTimeoutId) {
        clearTimeout(this._messageTimeoutId);
      }

      this._messageTimeoutId = setTimeout(() => {
        this.success = '';
        this._messageTimeoutId = null;
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
      return this.posts.every(post => this.selectedPosts.has(post.id));
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
