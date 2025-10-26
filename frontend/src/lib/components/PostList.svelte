<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { realtimeData } from '../stores/realtimeData';
  import { getAllPosts } from '../api/client';
  import type { Post } from '../api/types';

  let loading = false;
  let error: string | null = null;
  let selectedPost: Post | null = null;

  // Filters
  let filterSubreddit = '';
  let sortBy: 'score' | 'time' | 'comments' = 'time';

  // Subscribe to real-time posts
  $: posts = $realtimeData.posts;

  // Derived filtered and sorted posts
  $: filteredPosts = posts
    .filter(post => {
      if (!filterSubreddit) return true;
      return post.subreddit?.toLowerCase() === filterSubreddit.toLowerCase();
    })
    .sort((a, b) => {
      switch (sortBy) {
        case 'score':
          return b.score - a.score;
        case 'comments':
          return b.num_comments - a.num_comments;
        case 'time':
        default:
          return new Date(b.created_utc).getTime() - new Date(a.created_utc).getTime();
      }
    });

  onMount(async () => {
    // Connect to WebSocket
    realtimeData.connect();

    // Load initial posts from REST API
    await loadPosts();
  });

  onDestroy(() => {
    // Disconnect WebSocket when component unmounts
    realtimeData.disconnect();
  });

  async function loadPosts() {
    loading = true;
    error = null;

    try {
      const response = await getAllPosts(100, 0);
      realtimeData.setPosts(response.posts || []);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load posts';
      console.error('Failed to load posts:', err);
    } finally {
      loading = false;
    }
  }

  function formatScore(score: number): string {
    if (score >= 1000) {
      return (score / 1000).toFixed(1) + 'k';
    }
    return score.toString();
  }

  function formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 60) {
      return `${diffMins}m ago`;
    }
    if (diffHours < 24) {
      return `${diffHours}h ago`;
    }
    if (diffDays < 7) {
      return `${diffDays}d ago`;
    }
    return date.toLocaleDateString();
  }

  function truncateText(text: string, maxLength: number): string {
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
  }

  function getUniqueSubreddits(posts: Post[]): string[] {
    const subreddits = new Set<string>();
    posts.forEach(post => {
      if (post.subreddit) {
        subreddits.add(post.subreddit);
      }
    });
    return Array.from(subreddits).sort();
  }

  $: uniqueSubreddits = getUniqueSubreddits(posts);
</script>

<div class="space-y-4">
  <!-- Filters and Sort -->
  <div class="flex flex-wrap items-center gap-4 rounded-lg bg-white p-4 shadow">
    <div class="flex-1">
      <label for="subreddit-filter" class="block text-sm font-medium text-gray-700">
        Filter by Subreddit
      </label>
      <select
        id="subreddit-filter"
        bind:value={filterSubreddit}
        class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
      >
        <option value="">All Subreddits</option>
        {#each uniqueSubreddits as subreddit}
          <option value={subreddit}>r/{subreddit}</option>
        {/each}
      </select>
    </div>

    <div class="flex-1">
      <label for="sort-by" class="block text-sm font-medium text-gray-700">
        Sort By
      </label>
      <select
        id="sort-by"
        bind:value={sortBy}
        class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
      >
        <option value="time">Newest First</option>
        <option value="score">Highest Score</option>
        <option value="comments">Most Comments</option>
      </select>
    </div>

    <div class="flex items-end">
      <button
        on:click={loadPosts}
        disabled={loading}
        class="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {loading ? 'Loading...' : 'Refresh'}
      </button>
    </div>
  </div>

  <!-- Error Message -->
  {#if error}
    <div class="rounded-md bg-red-50 p-4">
      <div class="text-sm text-red-700">{error}</div>
    </div>
  {/if}

  <!-- Posts List -->
  <div class="space-y-3">
    {#if loading && posts.length === 0}
      <div class="rounded-lg bg-white p-8 text-center text-gray-500 shadow">
        <div class="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent"></div>
        <p class="mt-2">Loading posts...</p>
      </div>
    {:else if filteredPosts.length === 0}
      <div class="rounded-lg bg-white p-8 text-center text-gray-500 shadow">
        {#if filterSubreddit}
          No posts found in r/{filterSubreddit}
        {:else}
          No posts available. Add some subreddits to get started!
        {/if}
      </div>
    {:else}
      {#each filteredPosts as post (post.fullname)}
        <div class="rounded-lg bg-white p-4 shadow hover:shadow-md transition-shadow">
          <div class="flex gap-4">
            <!-- Score -->
            <div class="flex flex-col items-center justify-start">
              <div class="text-xs text-gray-500">
                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M10 3l2.5 5.5L18 9l-4.5 4.5L14 19l-4-2.5L6 19l.5-5.5L2 9l5.5-.5L10 3z"/>
                </svg>
              </div>
              <div class="font-bold text-gray-900">{formatScore(post.score)}</div>
            </div>

            <!-- Content -->
            <div class="flex-1">
              <div class="mb-2">
                <a
                  href={post.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-lg font-semibold text-gray-900 hover:text-blue-600"
                >
                  {post.title}
                </a>
              </div>

              {#if post.selftext}
                <p class="mb-2 text-sm text-gray-600">
                  {truncateText(post.selftext, 200)}
                </p>
              {/if}

              <div class="flex flex-wrap items-center gap-3 text-xs text-gray-500">
                {#if post.subreddit}
                  <span class="font-medium text-blue-600">r/{post.subreddit}</span>
                {/if}
                <span>by u/{post.author}</span>
                <span>{formatDate(post.created_utc)}</span>
                <span>{post.num_comments} comments</span>
                <a
                  href={post.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-blue-600 hover:underline"
                >
                  View on Reddit
                </a>
              </div>
            </div>
          </div>
        </div>
      {/each}
    {/if}
  </div>

  <!-- Post Count -->
  {#if filteredPosts.length > 0}
    <div class="text-center text-sm text-gray-500">
      Showing {filteredPosts.length} of {posts.length} posts
    </div>
  {/if}
</div>
