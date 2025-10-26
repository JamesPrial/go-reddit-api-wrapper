<script lang="ts">
	import ConnectionIndicator from '$lib/components/ConnectionIndicator.svelte';
	import PostList from '$lib/components/PostList.svelte';
</script>

<div class="space-y-6">
	<!-- Connection Status -->
	<ConnectionIndicator />

	<!-- Page Header -->
	<div class="rounded-lg bg-white p-6 shadow">
		<h2 class="text-2xl font-bold text-gray-900">Live Post Feed</h2>
		<p class="mt-2 text-sm text-gray-600">
			Real-time posts from all tracked subreddits. New posts appear automatically via WebSocket.
		</p>
	</div>

	<!-- Post List -->
	<PostList />
</div>
