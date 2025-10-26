import { writable, derived, type Writable, type Readable } from 'svelte/store';
import { createWebSocketStore, ConnectionStatus } from './websocket';
import type { Post, BenchmarkResult, WebSocketMessage } from '../api/types';

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080';

interface RealtimeDataState {
  posts: Post[];
  benchmarks: BenchmarkResult[];
  lastUpdate: Date | null;
}

/**
 * Application-specific real-time data store
 * Integrates WebSocket updates with application state
 */
function createRealtimeDataStore() {
  // WebSocket connection
  const wsUrl = `${WS_URL}/ws`;
  const ws = createWebSocketStore(wsUrl);

  // Application state
  const state = writable<RealtimeDataState>({
    posts: [],
    benchmarks: [],
    lastUpdate: null
  });

  // Subscribe to WebSocket messages
  ws.lastMessage.subscribe((message: WebSocketMessage | null) => {
    if (!message) return;

    state.update(current => {
      const updated = { ...current, lastUpdate: new Date() };

      switch (message.type) {
        case 'new_post':
          // Add new post to the beginning of the list
          const newPost = message.data as Post;
          updated.posts = [newPost, ...current.posts];
          break;

        case 'post_update':
          // Update existing post
          const updatedPost = message.data as Post;
          updated.posts = current.posts.map(p =>
            p.fullname === updatedPost.fullname ? updatedPost : p
          );
          break;

        case 'benchmark':
          // Add benchmark result
          const benchmark = message.data as BenchmarkResult;
          updated.benchmarks = [benchmark, ...current.benchmarks.slice(0, 99)]; // Keep last 100
          break;

        case 'posts_batch':
          // Batch update of posts
          updated.posts = message.data as Post[];
          break;

        default:
          console.warn('[RealtimeData] Unknown message type:', message.type);
      }

      return updated;
    });
  });

  // Derived stores
  const posts = derived(state, $state => $state.posts);
  const benchmarks = derived(state, $state => $state.benchmarks);
  const lastUpdate = derived(state, $state => $state.lastUpdate);

  // Derived store for posts grouped by subreddit
  const postsBySubreddit = derived(posts, $posts => {
    const grouped = new Map<string, Post[]>();
    $posts.forEach(post => {
      const subreddit = post.subreddit || 'unknown';
      if (!grouped.has(subreddit)) {
        grouped.set(subreddit, []);
      }
      grouped.get(subreddit)!.push(post);
    });
    return grouped;
  });

  // Derived store for post count
  const postCount = derived(posts, $posts => $posts.length);

  return {
    // WebSocket controls
    connect: ws.connect,
    disconnect: ws.disconnect,
    status: ws.status,
    error: ws.error,

    // Data stores
    posts,
    benchmarks,
    lastUpdate,
    postsBySubreddit,
    postCount,

    // Manual state updates (for REST API)
    setPosts: (newPosts: Post[]) => {
      state.update(current => ({
        ...current,
        posts: newPosts,
        lastUpdate: new Date()
      }));
    },

    addPost: (post: Post) => {
      state.update(current => ({
        ...current,
        posts: [post, ...current.posts],
        lastUpdate: new Date()
      }));
    },

    clearPosts: () => {
      state.update(current => ({
        ...current,
        posts: [],
        lastUpdate: new Date()
      }));
    }
  };
}

// Export singleton instance
export const realtimeData = createRealtimeDataStore();

// Export connection status enum for convenience
export { ConnectionStatus };
