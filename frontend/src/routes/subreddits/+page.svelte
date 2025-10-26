<script lang="ts">
	import SubredditManager from '$lib/components/SubredditManager.svelte';
</script>

<div class="space-y-6">
	<!-- Page Header -->
	<div class="rounded-lg bg-white p-6 shadow">
		<h2 class="text-2xl font-bold text-gray-900">Manage Subreddits</h2>
		<p class="mt-2 text-sm text-gray-600">
			Track new subreddits or remove existing ones. Changes will be reflected in the live feed.
		</p>
	</div>

	<!-- Subreddit Manager -->
	<SubredditManager />
</div>
