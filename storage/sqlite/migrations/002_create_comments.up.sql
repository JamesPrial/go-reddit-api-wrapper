-- Create comments table for storing Reddit comment data
-- This migration creates the comments table with all fields from the Comment struct,
-- plus additional denormalized fields (depth, post_id) for efficient querying.

CREATE TABLE IF NOT EXISTS comments (
    -- Column order matches converters_comment.go dest() function exactly
    -- This ensures SQL SELECT statements can scan directly into the struct

    -- Core identity fields (ThingData)
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,

    -- Votable fields
    score INTEGER NOT NULL DEFAULT 0,
    ups INTEGER NOT NULL DEFAULT 0,
    downs INTEGER NOT NULL DEFAULT 0,
    likes INTEGER,  -- NULL for no vote, 0 for downvote, 1 for upvote

    -- Created fields
    created REAL NOT NULL,
    created_utc REAL NOT NULL,

    -- Moderation and author fields
    approved_by TEXT,
    author TEXT NOT NULL,
    author_flair_css_class TEXT,
    author_flair_text TEXT,
    banned_by TEXT,

    -- Content fields
    body TEXT NOT NULL,
    body_html TEXT NOT NULL,

    -- Edited field (split into two columns)
    -- edited_is_edited stores whether the comment was edited (0 = false, 1 = true)
    -- edited_timestamp stores the edit timestamp (NULL if never edited or old edit without timestamp)
    edited_is_edited INTEGER NOT NULL DEFAULT 0,
    edited_timestamp REAL,

    -- Reddit metadata
    gilded INTEGER NOT NULL DEFAULT 0,

    -- Link/Post reference fields
    link_author TEXT,
    link_id TEXT NOT NULL,  -- Reddit fullname like "t3_abc123"
    link_title TEXT,
    link_url TEXT,

    -- Reports and threading
    num_reports INTEGER,
    parent_id TEXT NOT NULL,  -- Reddit fullname like "t1_xyz789" or "t3_abc123" for top-level

    -- User interaction fields
    saved INTEGER NOT NULL DEFAULT 0,  -- 0 = false, 1 = true
    score_hidden INTEGER NOT NULL DEFAULT 0,  -- 0 = false, 1 = true

    -- Community fields
    subreddit TEXT NOT NULL,
    subreddit_id TEXT NOT NULL,

    -- Moderation status
    distinguished TEXT,  -- NULL, "moderator", "admin", "special"

    -- Denormalized field for efficient tree queries
    -- depth is calculated from the comment tree structure:
    --   - Top-level comments (ParentID starts with "t3_") have depth = 0
    --   - Child comments have depth = parent depth + 1
    -- Denormalized here to enable efficient tree queries without recursive lookups
    depth INTEGER NOT NULL,

    -- Additional storage-specific fields
    -- post_id is extracted from link_id for foreign key relationship
    -- Example: link_id "t3_abc123" → post_id "abc123"
    -- This enables CASCADE delete when posts are removed
    post_id TEXT NOT NULL,

    -- fetched_at is a Unix timestamp (seconds since epoch) for cache eviction
    -- Used by EvictStale to remove old comments based on fetch time, not creation time
    fetched_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),

    -- Foreign key constraint to ensure comments belong to valid posts
    -- ON DELETE CASCADE ensures orphaned comments are automatically removed when posts are deleted
    FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
);

-- Create indexes for efficient queries

-- Index for fetching comments by post with optimal tree traversal
-- Composite index on (post_id, depth) enables:
--   1. Fast retrieval of all comments for a post
--   2. Efficient ordering by depth for tree reconstruction
--   3. Quick filtering by depth (e.g., MaxDepth option)
CREATE INDEX idx_comments_post_id_depth ON comments(post_id, depth);

-- Index for fetching child comments by parent
-- Used for building comment trees when constructing Reply relationships
-- Enables fast lookups like: SELECT * FROM comments WHERE parent_id = 't1_xyz789'
CREATE INDEX idx_comments_parent_id ON comments(parent_id);

-- Index for filtering comments by author
-- Useful for author-specific queries and user activity tracking
CREATE INDEX idx_comments_author ON comments(author);

-- Index for cache eviction based on fetch time
-- Used by EvictStale to efficiently remove old entries
-- Enables fast queries like: DELETE FROM comments WHERE fetched_at < ?
CREATE INDEX idx_comments_fetched_at ON comments(fetched_at);
