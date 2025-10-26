-- ============================================================================
-- Migration: 003_create_comment_closures.up.sql
-- Purpose: Create closure table for efficient comment tree queries
-- ============================================================================
--
-- CLOSURE TABLE PATTERN EXPLAINED:
--
-- The closure table pattern precomputes and stores all ancestor-descendant
-- relationships in a comment tree. This enables O(1) tree queries without
-- recursive CTEs or multiple database round-trips.
--
-- TRADITIONAL APPROACH (Slow):
--   To get all descendants of a comment, you'd need recursive CTEs:
--     WITH RECURSIVE tree AS (
--       SELECT id FROM comments WHERE id = 'C1'
--       UNION ALL
--       SELECT c.id FROM comments c JOIN tree t ON c.parent_id = t.id
--     )
--     SELECT * FROM tree;
--
--   This requires:
--   - Multiple iterations (one per tree level)
--   - Complex query planning
--   - Performance degrades with tree depth
--
-- CLOSURE TABLE APPROACH (Fast):
--   Store all ancestor-descendant pairs upfront:
--     SELECT descendant FROM comment_closures WHERE ancestor = 'C1';
--
--   This requires:
--   - Single index lookup
--   - Simple query plan
--   - Constant time regardless of tree depth
--
-- EXAMPLE TREE STRUCTURE:
--
--   C1 (top-level comment)
--   └─ C2 (child of C1)
--      └─ C3 (child of C2, grandchild of C1)
--
-- CLOSURE TABLE ENTRIES:
--
--   ancestor | descendant | depth | Explanation
--   ---------+------------+-------+-------------------------------------------
--   C1       | C1         | 0     | Self-reference (every node is ancestor of itself)
--   C1       | C2         | 1     | C1 is immediate parent of C2
--   C1       | C3         | 2     | C1 is grandparent of C3 (2 levels up)
--   C2       | C2         | 0     | Self-reference
--   C2       | C3         | 1     | C2 is immediate parent of C3
--   C3       | C3         | 0     | Self-reference
--
-- QUERY EXAMPLES:
--
--   1. Get all descendants of C1 (the entire subtree):
--      SELECT descendant FROM comment_closures WHERE ancestor = 'C1' AND depth > 0;
--      Result: C2, C3
--
--   2. Get all ancestors of C3 (the path to root):
--      SELECT ancestor FROM comment_closures WHERE descendant = 'C3' AND depth > 0;
--      Result: C2, C1
--
--   3. Get immediate children of C1:
--      SELECT descendant FROM comment_closures WHERE ancestor = 'C1' AND depth = 1;
--      Result: C2
--
--   4. Get depth of a comment in the tree:
--      SELECT MAX(depth) FROM comment_closures WHERE descendant = 'C3';
--      Result: 2 (C3 is 2 levels deep from root C1)
--
--   5. Count total descendants of C1:
--      SELECT COUNT(*) FROM comment_closures WHERE ancestor = 'C1' AND depth > 0;
--      Result: 2 (C2 and C3)
--
-- INSERTION PATTERN:
--
--   When inserting a new comment C4 as child of C3:
--   1. Add self-reference: (C4, C4, 0)
--   2. Copy all C3's ancestors and add C4 as descendant:
--      INSERT INTO comment_closures (ancestor, descendant, depth)
--      SELECT ancestor, 'C4', depth + 1
--      FROM comment_closures
--      WHERE descendant = 'C3';
--
--   This creates:
--   - (C4, C4, 0)  -- self-reference
--   - (C3, C4, 1)  -- immediate parent
--   - (C2, C4, 2)  -- grandparent
--   - (C1, C4, 3)  -- great-grandparent
--
-- DELETION PATTERN:
--
--   When a comment is deleted, CASCADE DELETE automatically removes:
--   - All rows where the comment is an ancestor (deleting subtree)
--   - All rows where the comment is a descendant (removing from ancestry)
--
--   For example, deleting C2 removes:
--   - (C1, C2, 1), (C2, C2, 0), (C2, C3, 1) -- C2 as ancestor
--   - (C1, C2, 1), (C2, C2, 0)              -- C2 as descendant
--
--   This automatically orphans C3 and all its descendants.
--
-- TRADE-OFFS:
--
--   Pros:
--   - O(1) tree queries (single index lookup)
--   - No recursive CTEs needed
--   - Simple query patterns
--   - Excellent read performance
--
--   Cons:
--   - Extra storage (O(n²) worst case for deep trees)
--   - INSERT complexity (must copy ancestor paths)
--   - UPDATE complexity (moving nodes requires rebuilding paths)
--
--   Reddit's comment trees are typically read-heavy (many viewers, few posters),
--   making this pattern ideal for our use case.
--
-- ============================================================================

CREATE TABLE IF NOT EXISTS comment_closures (
    -- The comment ID that is an ancestor in the tree
    -- This can be any comment up the parent chain, including the comment itself
    ancestor TEXT NOT NULL,

    -- The comment ID that is a descendant in the tree
    -- This can be any comment down the child chain, including the comment itself
    descendant TEXT NOT NULL,

    -- The number of edges (hops) between ancestor and descendant
    -- 0 = self-reference (every comment is its own ancestor)
    -- 1 = immediate parent-child relationship
    -- 2+ = grandparent, great-grandparent, etc.
    depth INTEGER NOT NULL,

    -- Composite primary key ensures no duplicate relationships
    -- Also creates an implicit index on (ancestor, descendant) for fast lookups
    PRIMARY KEY (ancestor, descendant),

    -- Foreign key to comments table (ancestor)
    -- ON DELETE CASCADE: When an ancestor is deleted, remove all its closure entries
    -- This automatically cleans up the entire subtree
    FOREIGN KEY (ancestor) REFERENCES comments(id) ON DELETE CASCADE,

    -- Foreign key to comments table (descendant)
    -- ON DELETE CASCADE: When a descendant is deleted, remove all its closure entries
    -- This automatically removes it from all ancestry paths
    FOREIGN KEY (descendant) REFERENCES comments(id) ON DELETE CASCADE
);

-- Index for querying descendants by ancestor
-- Example: "Find all comments in this thread" = WHERE ancestor = 'C1'
-- This is covered by the PRIMARY KEY (ancestor, descendant), so not needed separately

-- Index for querying ancestors by descendant
-- Example: "Find all parent comments of C3" = WHERE descendant = 'C3'
-- The PRIMARY KEY doesn't help with descendant-first queries, so we need this index
CREATE INDEX idx_closures_descendant ON comment_closures(descendant);

-- Composite index for depth-based queries on a specific ancestor
-- Example: "Get immediate children of C1" = WHERE ancestor = 'C1' AND depth = 1
-- Example: "Get all descendants within 2 levels of C1" = WHERE ancestor = 'C1' AND depth <= 2
CREATE INDEX idx_closures_ancestor_depth ON comment_closures(ancestor, depth);
