package sqlite

// SQLite-specific SQL queries
//
// This file centralizes all SQL query strings used by the SQLite storage backend.
// Queries use ? placeholders for parameters (SQLite syntax).
// All timestamp operations use strftime for consistency with Unix timestamps.

const (
	// ============================================================================
	// Post Queries
	// ============================================================================

	// queryUpsertPost inserts a new post or updates an existing post if it already exists.
	// Uses INSERT ... ON CONFLICT to handle upserts atomically.
	// The post ID is the unique identifier and primary key.
	// Sets fetched_at to current Unix timestamp on both insert and update.
	// Parameters: 35 values for the post data (fetched_at is set by strftime in the query)
	queryUpsertPost = `
		INSERT INTO posts (
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio, fetched_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, strftime('%s', 'now')
		)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			score = excluded.score,
			ups = excluded.ups,
			downs = excluded.downs,
			likes = excluded.likes,
			created = excluded.created,
			created_utc = excluded.created_utc,
			author = excluded.author,
			author_flair_css_class = excluded.author_flair_css_class,
			author_flair_text = excluded.author_flair_text,
			clicked = excluded.clicked,
			domain = excluded.domain,
			hidden = excluded.hidden,
			is_self = excluded.is_self,
			link_flair_css_class = excluded.link_flair_css_class,
			link_flair_text = excluded.link_flair_text,
			locked = excluded.locked,
			media = excluded.media,
			media_embed = excluded.media_embed,
			num_comments = excluded.num_comments,
			over_18 = excluded.over_18,
			permalink = excluded.permalink,
			saved = excluded.saved,
			selftext = excluded.selftext,
			selftext_html = excluded.selftext_html,
			subreddit = excluded.subreddit,
			subreddit_id = excluded.subreddit_id,
			thumbnail = excluded.thumbnail,
			title = excluded.title,
			url = excluded.url,
			edited_is_edited = excluded.edited_is_edited,
			edited_timestamp = excluded.edited_timestamp,
			distinguished = excluded.distinguished,
			stickied = excluded.stickied,
			upvote_ratio = excluded.upvote_ratio,
			fetched_at = strftime('%s', 'now')
	`

	// queryGetPost retrieves a post by its ID.
	// Parameters: 1 (post id)
	queryGetPost = `
		SELECT
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio
		FROM posts
		WHERE id = ?
	`

	// queryListPostsBase is the base SELECT query for listing posts.
	// WHERE, ORDER BY, LIMIT, and OFFSET clauses are added dynamically in ListPosts.
	// SECURITY: Sort field and direction are validated against whitelists before string interpolation.
	queryListPostsBase = `
		SELECT
			id, name, score, ups, downs, likes, created, created_utc,
			author, author_flair_css_class, author_flair_text, clicked, domain,
			hidden, is_self, link_flair_css_class, link_flair_text, locked,
			media, media_embed, num_comments, over_18, permalink, saved,
			selftext, selftext_html, subreddit, subreddit_id, thumbnail,
			title, url, edited_is_edited, edited_timestamp, distinguished,
			stickied, upvote_ratio
		FROM posts
	`

	// queryDeletePost removes a post by its ID.
	// Comments cascade automatically via foreign key constraints.
	// Parameters: 1 (post id)
	queryDeletePost = `DELETE FROM posts WHERE id = ?`

	// ============================================================================
	// Comment Queries
	// ============================================================================

	// queryUpsertComment inserts a new comment or updates an existing comment if it already exists.
	// Uses INSERT ... ON CONFLICT to handle upserts atomically.
	// Maintains comment ID as the unique identifier.
	// Sets fetched_at to current Unix timestamp on both insert and update.
	// Depth field is calculated by the application before calling this query.
	// Parameters: 32 values for the comment data (fetched_at is set by strftime in the query)
	queryUpsertComment = `
		INSERT INTO comments (
			id, name, score, ups, downs, likes, created, created_utc,
			approved_by, author, author_flair_css_class, author_flair_text, banned_by,
			body, body_html, edited_is_edited, edited_timestamp, gilded,
			link_author, link_id, link_title, link_url, num_reports, parent_id,
			saved, score_hidden, subreddit, subreddit_id, distinguished, depth, post_id, fetched_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now')
		)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			score = excluded.score,
			ups = excluded.ups,
			downs = excluded.downs,
			likes = excluded.likes,
			created = excluded.created,
			created_utc = excluded.created_utc,
			approved_by = excluded.approved_by,
			author = excluded.author,
			author_flair_css_class = excluded.author_flair_css_class,
			author_flair_text = excluded.author_flair_text,
			banned_by = excluded.banned_by,
			body = excluded.body,
			body_html = excluded.body_html,
			edited_is_edited = excluded.edited_is_edited,
			edited_timestamp = excluded.edited_timestamp,
			gilded = excluded.gilded,
			link_author = excluded.link_author,
			link_id = excluded.link_id,
			link_title = excluded.link_title,
			link_url = excluded.link_url,
			num_reports = excluded.num_reports,
			parent_id = excluded.parent_id,
			saved = excluded.saved,
			score_hidden = excluded.score_hidden,
			subreddit = excluded.subreddit,
			subreddit_id = excluded.subreddit_id,
			distinguished = excluded.distinguished,
			depth = excluded.depth,
			post_id = excluded.post_id,
			fetched_at = strftime('%s', 'now')
	`

	// queryGetComment retrieves a comment by its ID.
	// Parameters: 1 (comment id)
	queryGetComment = `
		SELECT
			id, name, score, ups, downs, likes, created, created_utc,
			approved_by, author, author_flair_css_class, author_flair_text, banned_by,
			body, body_html, edited_is_edited, edited_timestamp, gilded,
			link_author, link_id, link_title, link_url, num_reports, parent_id,
			saved, score_hidden, subreddit, subreddit_id, distinguished, depth
		FROM comments
		WHERE id = ?
	`

	// queryGetCommentTree retrieves all comments for a specific post.
	// WHERE clause filters by post_id and optional depth constraint are added dynamically.
	// ORDER BY clause is added dynamically (validates sort field before string interpolation).
	// SECURITY: Sort field and direction are validated against whitelists before string interpolation.
	// Parameters: 1 (post id) + optional 1 (max_depth if specified)
	queryGetCommentTree = `
		SELECT DISTINCT
			c.id, c.name, c.score, c.ups, c.downs, c.likes, c.created, c.created_utc,
			c.approved_by, c.author, c.author_flair_css_class, c.author_flair_text, c.banned_by,
			c.body, c.body_html, c.edited_is_edited, c.edited_timestamp, c.gilded,
			c.link_author, c.link_id, c.link_title, c.link_url, c.num_reports, c.parent_id,
			c.saved, c.score_hidden, c.subreddit, c.subreddit_id, c.distinguished, c.depth
		FROM comments c
		WHERE c.post_id = ?
	`

	// queryDeleteComment removes a comment by its ID.
	// Closure table entries are automatically deleted via CASCADE foreign keys.
	// Parameters: 1 (comment id)
	queryDeleteComment = `DELETE FROM comments WHERE id = ?`

	// queryGetCommentDepth retrieves the depth of a comment by its ID.
	// Used to calculate the depth of child comments.
	// Parameters: 1 (comment id)
	queryGetCommentDepth = `SELECT depth FROM comments WHERE id = ?`

	// queryDeleteCommentClosures removes all closure table entries for a specific comment.
	// Used when reparenting a comment to rebuild its closure entries.
	// Parameters: 1 (comment id)
	queryDeleteCommentClosures = `DELETE FROM comment_closures WHERE descendant = ?`

	// queryInsertCommentClosure inserts a self-reference closure entry for a new comment.
	// Every comment has a self-reference with depth 0.
	// Parameters: 2 (comment id, comment id)
	queryInsertCommentClosure = `INSERT INTO comment_closures (ancestor, descendant, depth) VALUES (?, ?, 0)`

	// queryInsertCommentClosureCopyAncestry copies all ancestor relationships from a parent comment.
	// Inserts entries for the new comment as descendant of all parent's ancestors.
	// Increments depth by 1 for each ancestor (depth increases as we go up the tree).
	// Parameters: 2 (comment id, parent comment id)
	queryInsertCommentClosureCopyAncestry = `
		INSERT INTO comment_closures (ancestor, descendant, depth)
		SELECT ancestor, ?, depth + 1
		FROM comment_closures
		WHERE descendant = ?
	`

	// queryCheckCommentExists checks if a comment with the given ID exists.
	// Parameters: 1 (comment id)
	queryCheckCommentExists = `SELECT EXISTS(SELECT 1 FROM comments WHERE id = ?)`

	// ============================================================================
	// Statistics Queries
	// ============================================================================

	// queryGetPostCount retrieves the total number of posts in the database.
	queryGetPostCount = `SELECT COUNT(*) FROM posts`

	// queryGetCommentCount retrieves the total number of comments in the database.
	queryGetCommentCount = `SELECT COUNT(*) FROM comments`

	// queryGetOldestEntry retrieves the oldest fetched_at timestamp across all posts and comments.
	// Returns NULL if tables are empty.
	queryGetOldestEntry = `
		SELECT MIN(fetched_at) FROM (
			SELECT fetched_at FROM posts
			UNION ALL
			SELECT fetched_at FROM comments
		)
	`

	// queryGetNewestEntry retrieves the newest fetched_at timestamp across all posts and comments.
	// Returns NULL if tables are empty.
	queryGetNewestEntry = `
		SELECT MAX(fetched_at) FROM (
			SELECT fetched_at FROM posts
			UNION ALL
			SELECT fetched_at FROM comments
		)
	`

	// queryGetDatabaseSize calculates the total database size in bytes.
	// Multiplies page count by page size (SQLite storage model).
	queryGetDatabaseSize = `
		SELECT page_count * page_size as size
		FROM pragma_page_count(), pragma_page_size()
	`

	// ============================================================================
	// Eviction Queries
	// ============================================================================

	// queryDeleteStalePosts removes all posts where fetched_at is earlier than the specified cutoff timestamp.
	// Uses <= for the comparison to include the cutoff time itself.
	// Parameters: 1 (cutoff timestamp)
	queryDeleteStalePosts = `DELETE FROM posts WHERE fetched_at <= ?`

	// queryDeleteStaleComments removes all comments where fetched_at is earlier than the specified cutoff timestamp.
	// Closure table entries are automatically deleted via CASCADE foreign keys.
	// Uses <= for the comparison to include the cutoff time itself.
	// Parameters: 1 (cutoff timestamp)
	queryDeleteStaleComments = `DELETE FROM comments WHERE fetched_at <= ?`

	// ============================================================================
	// SQLite-specific expressions
	// ============================================================================

	// exprCurrentUnixTimestamp returns the current Unix timestamp in SQLite.
	// Used in INSERT/UPDATE queries to set the fetched_at field.
	exprCurrentUnixTimestamp = `strftime('%s', 'now')`
)
