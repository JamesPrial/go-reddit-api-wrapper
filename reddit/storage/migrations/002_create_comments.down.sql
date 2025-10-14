-- Rollback migration for comments table
-- This migration removes the comments table and all associated indexes.

-- Drop indexes first (SQLite requires explicit index drops)
DROP INDEX IF EXISTS idx_comments_fetched_at;
DROP INDEX IF EXISTS idx_comments_author;
DROP INDEX IF EXISTS idx_comments_parent_id;
DROP INDEX IF EXISTS idx_comments_post_id_depth;

-- Drop the comments table
-- This will cascade delete any data stored in the table
DROP TABLE IF EXISTS comments;
