import type {
  Subreddit,
  Post,
  Comment,
  SubredditsResponse,
  PostsResponse,
  CommentsResponse,
  CreateSubredditRequest,
  ErrorResponse
} from './types';

// Get API base URL from environment or use default
const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

/**
 * API client for Reddit tracker backend
 */
class APIClient {
  private baseUrl: string;

  constructor(baseUrl: string = API_BASE) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options?: RequestInit
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;

    try {
      const response = await fetch(url, {
        ...options,
        headers: {
          'Content-Type': 'application/json',
          ...options?.headers,
        },
      });

      if (!response.ok) {
        const error: ErrorResponse = await response.json().catch(() => ({
          error: `HTTP ${response.status}: ${response.statusText}`,
        }));
        throw new Error(error.error || `Request failed with status ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      if (error instanceof Error) {
        throw error;
      }
      throw new Error('An unknown error occurred');
    }
  }

  /**
   * Get all tracked subreddits
   */
  async getSubreddits(): Promise<Subreddit[]> {
    const response = await this.request<SubredditsResponse>('/api/subreddits');
    return response.subreddits || [];
  }

  /**
   * Create a new tracked subreddit
   */
  async createSubreddit(name: string, description?: string): Promise<Subreddit> {
    const body: CreateSubredditRequest = {
      name,
      description,
    };

    return await this.request<Subreddit>('/api/subreddits', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  /**
   * Delete a tracked subreddit
   */
  async deleteSubreddit(name: string): Promise<void> {
    await this.request<void>(`/api/subreddits/${name}`, {
      method: 'DELETE',
    });
  }

  /**
   * Get posts from a subreddit
   */
  async getPosts(subreddit: string, limit: number = 50, offset: number = 0): Promise<PostsResponse> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      offset: offset.toString(),
    });

    return await this.request<PostsResponse>(
      `/api/subreddits/${subreddit}/posts?${params}`
    );
  }

  /**
   * Get all posts from all tracked subreddits
   */
  async getAllPosts(limit: number = 50, offset: number = 0): Promise<PostsResponse> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      offset: offset.toString(),
    });

    return await this.request<PostsResponse>(`/api/posts?${params}`);
  }

  /**
   * Get comments for a specific post
   */
  async getPostComments(fullname: string): Promise<Comment[]> {
    const response = await this.request<CommentsResponse>(
      `/api/posts/${fullname}/comments`
    );
    return response.comments || [];
  }
}

// Export singleton instance
export const apiClient = new APIClient();

// Export named functions for convenience
export const getSubreddits = () => apiClient.getSubreddits();
export const createSubreddit = (name: string, description?: string) =>
  apiClient.createSubreddit(name, description);
export const deleteSubreddit = (name: string) => apiClient.deleteSubreddit(name);
export const getPosts = (subreddit: string, limit?: number, offset?: number) =>
  apiClient.getPosts(subreddit, limit, offset);
export const getAllPosts = (limit?: number, offset?: number) =>
  apiClient.getAllPosts(limit, offset);
export const getPostComments = (fullname: string) => apiClient.getPostComments(fullname);
