-- Migration: Add post snapshots and comment change events tables
-- Description: Creates tables for tracking post metrics over time and detecting comment count changes.
--
-- Schema design rationale:
--   - post_snapshots: Immutable point-in-time records of post state (num_comments, score)
--   - comment_change_events: Records when new comments are detected between snapshots
--   - INTEGER timestamps for consistency with existing schema (Unix seconds)
--   - Composite indexes on (post_id, timestamp) for efficient time-series queries
--   - CHECK constraints ensure data validity at database level
--   - FOREIGN KEY with CASCADE ensures referential integrity
--   - UNIQUE constraint prevents duplicate snapshots at same timestamp

CREATE TABLE post_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id TEXT NOT NULL,
    fullname TEXT NOT NULL,
    num_comments INTEGER NOT NULL CHECK (num_comments >= 0),
    score INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),

    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
    UNIQUE(post_id, created_at)
);

-- Index for efficient post snapshot queries
-- Optimizes queries filtering by post_id and ordering by created_at
-- Examples:
--   - SELECT * FROM post_snapshots WHERE post_id = ? ORDER BY created_at DESC LIMIT 1
--   - SELECT * FROM post_snapshots WHERE post_id = ? AND created_at > ?
CREATE INDEX idx_post_snapshots_post_created ON post_snapshots(post_id, created_at);

CREATE TABLE comment_change_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id TEXT NOT NULL,
    fullname TEXT NOT NULL,
    detected_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    previous_count INTEGER NOT NULL CHECK (previous_count >= 0),
    new_count INTEGER NOT NULL CHECK (new_count >= previous_count),
    comments_added INTEGER NOT NULL CHECK (comments_added = new_count - previous_count),

    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
);

-- Index for efficient comment change event queries
-- Optimizes queries filtering by post_id and ordering by detected_at
-- Examples:
--   - SELECT * FROM comment_change_events WHERE post_id = ? ORDER BY detected_at DESC LIMIT 10
--   - SELECT COUNT(*) FROM comment_change_events WHERE post_id = ?
CREATE INDEX idx_comment_change_events_post_detected ON comment_change_events(post_id, detected_at);
