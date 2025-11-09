-- Migration: Reverse post snapshots and comment change events tables
-- Description: Removes the post_snapshots and comment_change_events tables created in the
--              005_add_post_snapshots.up.sql migration. This reversal is safe as it only
--              drops tables; any dependent code will need to handle the absence of these
--              tables gracefully or use conditional logic based on migration state.

DROP TABLE IF EXISTS comment_change_events;

DROP TABLE IF EXISTS post_snapshots;
