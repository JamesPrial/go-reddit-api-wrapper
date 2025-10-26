<script lang="ts">
  import { onMount } from 'svelte';
  import { getSubreddits, createSubreddit, deleteSubreddit } from '../api/client';
  import type { Subreddit } from '../api/types';

  let subreddits: Subreddit[] = [];
  let loading = false;
  let error: string | null = null;

  // Form state
  let newSubredditName = '';
  let newSubredditDescription = '';
  let submitting = false;

  onMount(async () => {
    await loadSubreddits();
  });

  async function loadSubreddits() {
    loading = true;
    error = null;

    try {
      subreddits = await getSubreddits();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load subreddits';
      console.error('Failed to load subreddits:', err);
    } finally {
      loading = false;
    }
  }

  async function handleSubmit() {
    if (!newSubredditName.trim()) {
      error = 'Subreddit name is required';
      return;
    }

    submitting = true;
    error = null;

    try {
      await createSubreddit(newSubredditName.trim(), newSubredditDescription.trim() || undefined);
      newSubredditName = '';
      newSubredditDescription = '';
      await loadSubreddits();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to create subreddit';
      console.error('Failed to create subreddit:', err);
    } finally {
      submitting = false;
    }
  }

  async function handleDelete(name: string) {
    if (!confirm(`Are you sure you want to stop tracking r/${name}?`)) {
      return;
    }

    error = null;

    try {
      await deleteSubreddit(name);
      await loadSubreddits();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to delete subreddit';
      console.error('Failed to delete subreddit:', err);
    }
  }

  function formatNumber(num: number): string {
    if (num >= 1000000) {
      return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
      return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
  }

  function formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    return date.toLocaleDateString();
  }
</script>

<div class="space-y-6">
  <!-- Add Subreddit Form -->
  <div class="rounded-lg bg-white p-6 shadow">
    <h2 class="mb-4 text-xl font-bold text-gray-900">Track New Subreddit</h2>

    <form on:submit|preventDefault={handleSubmit} class="space-y-4">
      <div>
        <label for="name" class="block text-sm font-medium text-gray-700">
          Subreddit Name
        </label>
        <div class="mt-1 flex rounded-md shadow-sm">
          <span class="inline-flex items-center rounded-l-md border border-r-0 border-gray-300 bg-gray-50 px-3 text-gray-500 sm:text-sm">
            r/
          </span>
          <input
            type="text"
            id="name"
            bind:value={newSubredditName}
            disabled={submitting}
            class="block w-full flex-1 rounded-none rounded-r-md border-gray-300 focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            placeholder="worldnews"
          />
        </div>
      </div>

      <div>
        <label for="description" class="block text-sm font-medium text-gray-700">
          Description (optional)
        </label>
        <input
          type="text"
          id="description"
          bind:value={newSubredditDescription}
          disabled={submitting}
          class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
          placeholder="World news and current events"
        />
      </div>

      <button
        type="submit"
        disabled={submitting || !newSubredditName.trim()}
        class="inline-flex justify-center rounded-md border border-transparent bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {submitting ? 'Adding...' : 'Add Subreddit'}
      </button>
    </form>
  </div>

  <!-- Error Message -->
  {#if error}
    <div class="rounded-md bg-red-50 p-4">
      <div class="flex">
        <div class="ml-3">
          <h3 class="text-sm font-medium text-red-800">Error</h3>
          <div class="mt-2 text-sm text-red-700">
            <p>{error}</p>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- Subreddits List -->
  <div class="rounded-lg bg-white shadow">
    <div class="px-6 py-4 border-b border-gray-200">
      <h2 class="text-xl font-bold text-gray-900">Tracked Subreddits</h2>
    </div>

    {#if loading}
      <div class="p-8 text-center text-gray-500">
        <div class="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent"></div>
        <p class="mt-2">Loading subreddits...</p>
      </div>
    {:else if subreddits.length === 0}
      <div class="p-8 text-center text-gray-500">
        No subreddits tracked yet. Add one above to get started!
      </div>
    {:else}
      <ul class="divide-y divide-gray-200">
        {#each subreddits as subreddit}
          <li class="px-6 py-4 hover:bg-gray-50">
            <div class="flex items-center justify-between">
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <h3 class="text-lg font-semibold text-gray-900">r/{subreddit.name}</h3>
                  <span class="rounded-full bg-blue-100 px-2 py-1 text-xs font-medium text-blue-800">
                    {formatNumber(subreddit.subscribers)} subscribers
                  </span>
                </div>
                {#if subreddit.description}
                  <p class="mt-1 text-sm text-gray-600">{subreddit.description}</p>
                {/if}
                <p class="mt-1 text-xs text-gray-500">
                  Added {formatDate(subreddit.created_at)}
                </p>
              </div>
              <button
                on:click={() => handleDelete(subreddit.name)}
                class="ml-4 rounded-md bg-red-600 px-3 py-2 text-sm font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2"
              >
                Remove
              </button>
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</div>
