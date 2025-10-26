<script lang="ts">
	import '../app.css';

	let { children } = $props();
</script>

<div class="min-h-screen bg-gray-100">
	<!-- Header -->
	<header class="bg-white shadow">
		<div class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
			<div class="flex items-center justify-between">
				<h1 class="text-3xl font-bold text-gray-900">Reddit Tracker</h1>
				<nav class="flex gap-4">
					<a
						href="/"
						class="rounded-md px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
					>
						Dashboard
					</a>
					<a
						href="/subreddits"
						class="rounded-md px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
					>
						Manage Subreddits
					</a>
				</nav>
			</div>
		</div>
	</header>

	<!-- Main Content -->
	<main class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
		{@render children?.()}
	</main>

	<!-- Footer -->
	<footer class="mt-8 bg-white shadow">
		<div class="mx-auto max-w-7xl px-4 py-4 sm:px-6 lg:px-8">
			<p class="text-center text-sm text-gray-500">
				Reddit Tracker - Real-time Reddit monitoring with WebSocket updates
			</p>
		</div>
	</footer>
</div>
