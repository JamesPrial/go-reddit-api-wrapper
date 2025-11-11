-- ============================================================================
-- Migration: 006_create_monitor_state.up.sql
-- Purpose: Create monitor_state table for persisting monitor instance state
-- ============================================================================
--
-- MONITOR STATE PERSISTENCE:
--
-- This table stores the state of monitor instances that continuously poll
-- Reddit for new posts and comments. It enables monitors to survive restarts
-- by persisting configuration, status, statistics, and last seen post IDs.
--
-- ============================================================================
-- SCHEMA DESIGN
-- ============================================================================
--
-- Primary Key:
--   - id: UUID uniquely identifying each monitor instance
--
-- Configuration Fields:
--   - subreddits: JSON array of subreddit names being monitored
--   - interval_seconds: Polling interval (minimum 10 seconds per Reddit API guidelines)
--   - post_limit: Number of posts to fetch per poll (1-100 per Reddit API limits)
--   - fetch_comments: Boolean flag indicating whether to fetch comments (0=no, 1=yes)
--
-- State Management:
--   - status: Current monitor state - "active", "paused", or "stopped"
--   - last_post_ids: JSON object mapping subreddit names to last seen post IDs
--                    Example: {"golang": "t3_abc123", "programming": "t3_xyz789"}
--                    Used to detect new posts and avoid reprocessing
--
-- Statistics Fields:
--   - total_fetches: Total number of successful fetch operations
--   - total_posts: Total number of posts processed
--   - total_comments: Total number of comments processed
--   - failed_fetches: Total number of failed fetch attempts
--   - consecutive_errors: Count of consecutive errors (resets on success)
--   - last_error: Text description of most recent error (empty string if none)
--
-- Timestamp Fields (Unix timestamps, INTEGER for SQLite efficiency):
--   - created_at: When the monitor was created
--   - started_at: When the monitor was last started/resumed
--   - last_fetch_time: When the last successful fetch occurred (NULL if never)
--   - stopped_at: When the monitor was stopped (NULL if active)
--
-- ============================================================================
-- CONSTRAINTS
-- ============================================================================
--
-- CHECK Constraints:
--   1. interval_seconds >= 10
--      Enforces Reddit API rate limit guidelines (no more than 6 requests/min)
--
--   2. post_limit >= 1 AND post_limit <= 100
--      Enforces Reddit API limits for posts per request
--
--   3. status IN ('active', 'paused', 'stopped')
--      Ensures valid state transitions
--
-- ============================================================================
-- INDEXES
-- ============================================================================
--
-- idx_monitor_state_status:
--   - Enables efficient queries for active monitors
--   - Query pattern: SELECT * FROM monitor_state WHERE status = 'active'
--   - Critical for monitor management operations (resume all, stop all, etc.)
--
-- ============================================================================

CREATE TABLE monitor_state (
    -- Identity
    id TEXT PRIMARY KEY NOT NULL,                   -- UUID of monitor instance

    -- Configuration
    subreddits TEXT NOT NULL,                       -- JSON array of subreddit names
    interval_seconds INTEGER NOT NULL,              -- Polling interval in seconds
    post_limit INTEGER NOT NULL,                    -- Posts to fetch per poll
    fetch_comments INTEGER NOT NULL,                -- Boolean: 1=fetch comments, 0=don't

    -- State
    status TEXT NOT NULL,                           -- "active", "paused", or "stopped"
    last_post_ids TEXT NOT NULL DEFAULT '{}',       -- JSON map: subreddit -> last post ID

    -- Statistics
    total_fetches INTEGER NOT NULL DEFAULT 0,       -- Total successful fetches
    total_posts INTEGER NOT NULL DEFAULT 0,         -- Total posts processed
    total_comments INTEGER NOT NULL DEFAULT 0,      -- Total comments processed
    failed_fetches INTEGER NOT NULL DEFAULT 0,      -- Total failed fetch attempts
    consecutive_errors INTEGER NOT NULL DEFAULT 0,  -- Consecutive error count
    last_error TEXT NOT NULL DEFAULT '',            -- Most recent error message

    -- Timestamps (Unix timestamps)
    created_at INTEGER NOT NULL,                    -- When monitor was created
    started_at INTEGER NOT NULL,                    -- When monitor was started/resumed
    last_fetch_time INTEGER,                        -- Last successful fetch (nullable)
    stopped_at INTEGER,                             -- When monitor was stopped (nullable)

    -- Constraints
    CHECK (interval_seconds >= 10),
    CHECK (post_limit >= 1 AND post_limit <= 100),
    CHECK (status IN ('active', 'paused', 'stopped'))
);

-- Index for efficient queries of active monitors
-- Covers: SELECT * FROM monitor_state WHERE status = 'active'
CREATE INDEX idx_monitor_state_status ON monitor_state(status);
