-- Migration: Drop performance indexes
-- Description: Reverses the 004_add_performance_indexes.up.sql migration by dropping
--              the score and subreddit+score indexes to clean up the database schema.
--
-- This migration safely removes indexes that were added to optimize score filtering
-- and subreddit-based queries. The PostOperations interface will continue to function,
-- but queries using score filtering (MinScore) and subreddit+score combinations may
-- perform slower (resorting to table scans or less optimal index usage).
--
-- Use this migration only during development or if the performance indexes are no
-- longer beneficial for your query patterns.

-- Drop the score filtering index
DROP INDEX IF EXISTS idx_posts_score;

-- Drop the composite subreddit+score index
DROP INDEX IF EXISTS idx_posts_subreddit_score;
