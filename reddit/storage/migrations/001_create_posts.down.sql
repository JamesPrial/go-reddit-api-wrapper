-- Migration: Drop posts table
-- Description: Reverses the 001_create_posts.up.sql migration by dropping all indexes and the posts table.
--              This removes all cached post data from the database.
--
-- WARNING: This operation is destructive and will delete all cached posts.

-- Drop indexes first (must be dropped before the table)
DROP INDEX IF EXISTS idx_posts_fetched_at;
DROP INDEX IF EXISTS idx_posts_author;
DROP INDEX IF EXISTS idx_posts_subreddit_created;

-- Drop the posts table
DROP TABLE IF EXISTS posts;
