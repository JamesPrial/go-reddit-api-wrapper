-- Migration: Create posts table
-- Description: Creates the posts table with all 36 Post struct fields plus fetched_at timestamp
--              for caching Reddit posts with proper indexing for common query patterns.
--
-- Schema Design:
--   - Primary key on id (Reddit post ID without "t3_" prefix)
--   - TEXT for strings and IDs (SQLite stores variable-length strings efficiently)
--   - INTEGER for ints and booleans (SQLite convention: 0=false, 1=true)
--   - REAL for float64 timestamps (Unix timestamps with fractional seconds)
--   - Edited field split into two columns: edited_is_edited (boolean) and edited_timestamp (float)
--   - Media/MediaEmbed stored as TEXT (JSON strings)
--   - Nullable fields use NULL for absent values (matches Go *string, *bool types)
--
-- Indexes:
--   - idx_posts_subreddit_created: Enables efficient sorting by recency within a subreddit
--   - idx_posts_author: Supports queries filtering by author
--   - idx_posts_fetched_at: Enables efficient cache eviction of stale posts

CREATE TABLE posts (
    -- ThingData fields (inherited from embedded struct)
    id TEXT PRIMARY KEY NOT NULL,           -- Reddit ID without prefix (e.g., "abc123")
    name TEXT NOT NULL,                     -- Full Reddit name with prefix (e.g., "t3_abc123")

    -- Votable fields (inherited from embedded struct)
    score INTEGER NOT NULL,                 -- Net vote count (upvotes - downvotes)
    ups INTEGER NOT NULL,                   -- Legacy field, equals score
    downs INTEGER NOT NULL,                 -- Legacy field, always 0
    likes INTEGER,                          -- User vote: 1=upvote, 0=downvote, NULL=no vote

    -- Created fields (inherited from embedded struct)
    created REAL NOT NULL,                  -- Creation time (local timezone, Unix timestamp)
    created_utc REAL NOT NULL,              -- Creation time (UTC, Unix timestamp)

    -- Post-specific fields
    author TEXT NOT NULL,                   -- Username of post author
    author_flair_css_class TEXT,            -- CSS class for author flair (nullable)
    author_flair_text TEXT,                 -- Text content of author flair (nullable)
    clicked INTEGER NOT NULL DEFAULT 0,     -- Whether user clicked the post (0=false, 1=true)
    domain TEXT NOT NULL,                   -- Domain of the linked URL
    hidden INTEGER NOT NULL DEFAULT 0,      -- Whether user hid the post (0=false, 1=true)
    is_self INTEGER NOT NULL DEFAULT 0,     -- Whether post is self-post/text-only (0=false, 1=true)
    link_flair_css_class TEXT,              -- CSS class for link flair (nullable)
    link_flair_text TEXT,                   -- Text content of link flair (nullable)
    locked INTEGER NOT NULL DEFAULT 0,      -- Whether post is locked (0=false, 1=true)
    media TEXT,                             -- JSON string of media object (nullable)
    media_embed TEXT,                       -- JSON string of media embed object (nullable)
    num_comments INTEGER NOT NULL,          -- Number of comments on the post
    over_18 INTEGER NOT NULL DEFAULT 0,     -- Whether post is NSFW (0=false, 1=true)
    permalink TEXT NOT NULL,                -- Relative URL to post (e.g., "/r/golang/comments/...")
    saved INTEGER NOT NULL DEFAULT 0,       -- Whether user saved the post (0=false, 1=true)
    selftext TEXT NOT NULL,                 -- Body text for self-posts (empty for link posts)
    selftext_html TEXT,                     -- HTML-rendered body text (nullable)
    subreddit TEXT NOT NULL,                -- Subreddit name without "/r/" prefix
    subreddit_id TEXT NOT NULL,             -- Subreddit fullname (e.g., "t5_xyz789")
    thumbnail TEXT NOT NULL,                -- URL to thumbnail image
    title TEXT NOT NULL,                    -- Post title
    url TEXT NOT NULL,                      -- URL of linked content

    -- Edited field (split into two columns for SQLite compatibility)
    edited_is_edited INTEGER NOT NULL DEFAULT 0,  -- Whether post was edited (0=false, 1=true)
    edited_timestamp REAL,                  -- Edit timestamp if available (nullable, Unix timestamp)

    -- Additional Post fields
    distinguished TEXT,                     -- Distinguishment status: "moderator", "admin", or NULL
    stickied INTEGER NOT NULL DEFAULT 0,    -- Whether post is stickied (0=false, 1=true)
    upvote_ratio REAL NOT NULL,             -- Ratio of upvotes to total votes (0.0-1.0)

    -- Cache management field
    fetched_at INTEGER NOT NULL             -- Unix timestamp when post was fetched (for cache eviction)
);

-- Index for common query: Get recent posts in a subreddit
-- Enables efficient "SELECT * FROM posts WHERE subreddit = ? ORDER BY created_utc DESC"
CREATE INDEX idx_posts_subreddit_created ON posts(subreddit, created_utc DESC);

-- Index for author-based queries
-- Enables efficient "SELECT * FROM posts WHERE author = ?"
CREATE INDEX idx_posts_author ON posts(author);

-- Index for cache eviction queries
-- Enables efficient "DELETE FROM posts WHERE fetched_at < ?" to remove stale posts
CREATE INDEX idx_posts_fetched_at ON posts(fetched_at);
