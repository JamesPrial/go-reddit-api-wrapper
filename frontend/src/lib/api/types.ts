// TypeScript types matching Go backend models

export interface Subreddit {
  id: number;
  fullname: string;
  name: string;
  description: string;
  subscribers: number;
  created_at: string;
  updated_at: string;
}

export interface Post {
  id: number;
  fullname: string;
  title: string;
  author: string;
  score: number;
  num_comments: number;
  url: string;
  selftext: string;
  created_utc: string;
  subreddit?: string;
}

export interface Comment {
  id: number;
  fullname: string;
  author: string;
  body: string;
  score: number;
  created_utc: string;
  post_fullname?: string;
}

// WebSocket message types
export interface WebSocketMessage {
  type: string;
  data: any;
}

export interface BenchmarkResult {
  name: string;
  iterations: number;
  ns_per_op: number;
  mb_per_sec: number;
  bytes_per_op: number;
  allocs_per_op: number;
}

// API Response types
export interface SubredditsResponse {
  subreddits: Subreddit[];
}

export interface PostsResponse {
  posts: Post[];
  total: number;
}

export interface CommentsResponse {
  comments: Comment[];
}

export interface CreateSubredditRequest {
  name: string;
  description?: string;
}

// Error response
export interface ErrorResponse {
  error: string;
}
