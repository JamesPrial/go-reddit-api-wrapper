<script lang="ts">
  import { getPostComments } from '../api/client';
  import type { Comment } from '../api/types';

  export let postFullname: string;
  export let postTitle: string = '';

  let comments: Comment[] = [];
  let loading = false;
  let error: string | null = null;
  let isOpen = false;

  async function loadComments() {
    if (comments.length > 0) {
      isOpen = !isOpen;
      return;
    }

    loading = true;
    error = null;
    isOpen = true;

    try {
      comments = await getPostComments(postFullname);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load comments';
      console.error('Failed to load comments:', err);
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
</script>

<div class="border-t border-gray-200 pt-3">
  <button
    on:click={loadComments}
    class="flex w-full items-center justify-between text-left text-sm font-medium text-blue-600 hover:text-blue-800"
  >
    <span>
      {isOpen ? 'Hide' : 'Show'} Comments ({comments.length || '?'})
    </span>
    <svg
      class="h-5 w-5 transition-transform {isOpen ? 'rotate-180' : ''}"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
    </svg>
  </button>

  {#if isOpen}
    <div class="mt-3 space-y-3">
      {#if loading}
        <div class="p-4 text-center text-gray-500">
          <div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-solid border-current border-r-transparent"></div>
          <p class="mt-2 text-sm">Loading comments...</p>
        </div>
      {:else if error}
        <div class="rounded-md bg-red-50 p-3">
          <div class="text-sm text-red-700">{error}</div>
        </div>
      {:else if comments.length === 0}
        <div class="p-4 text-center text-sm text-gray-500">
          No comments yet
        </div>
      {:else}
        {#each comments as comment}
          <div class="rounded-md bg-gray-50 p-3">
            <div class="mb-2 flex items-center gap-2 text-xs text-gray-500">
              <span class="font-medium text-gray-700">u/{comment.author}</span>
              <span>{formatDate(comment.created_utc)}</span>
              <span class="flex items-center gap-1">
                <svg class="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M10 3l2.5 5.5L18 9l-4.5 4.5L14 19l-4-2.5L6 19l.5-5.5L2 9l5.5-.5L10 3z"/>
                </svg>
                {formatScore(comment.score)}
              </span>
            </div>
            <div class="text-sm text-gray-800 whitespace-pre-wrap">{comment.body}</div>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>
