-- ============================================================================
-- Migration: 003_create_comment_closures.down.sql
-- Purpose: Rollback closure table creation
-- ============================================================================
--
-- This migration reverses the changes made by 003_create_comment_closures.up.sql
--
-- ROLLBACK ORDER:
--   1. Drop indexes first (they depend on the table)
--   2. Drop the table last (foreign keys are automatically dropped with the table)
--
-- SAFETY NOTES:
--   - Dropping this table removes all precomputed tree relationships
--   - Comments themselves are NOT deleted (they're in the comments table)
--   - The closure table can be rebuilt by re-running queries against parent_id
--   - If you roll back this migration, tree queries will need recursive CTEs
--
-- ============================================================================

-- Drop composite index for depth-based queries
DROP INDEX IF EXISTS idx_closures_ancestor_depth;

-- Drop index for descendant lookups
DROP INDEX IF EXISTS idx_closures_descendant;

-- Drop the closure table
-- This also removes the foreign key constraints automatically
DROP TABLE IF EXISTS comment_closures;
