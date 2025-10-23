package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// commentScanDest holds the destination pointers for scanning a Comment from SQL.
// This struct maintains the nullable SQL types that need to be converted after scanning.
type commentScanDest struct {
	// Direct fields
	id             string
	name           string
	score          int
	ups            int
	downs          int
	created        float64
	createdUTC     float64
	author         string
	body           string
	bodyHTML       string
	editedIsEdited bool
	editedTs       float64
	gilded         int
	linkID         string
	parentID       string
	saved          bool
	scoreHidden    bool
	subreddit      string
	subredditID    string

	// Nullable fields
	likes           sql.NullInt64
	approvedBy      sql.NullString
	authorFlairCSS  sql.NullString
	authorFlairText sql.NullString
	bannedBy        sql.NullString
	linkAuthor      sql.NullString
	linkTitle       sql.NullString
	linkURL         sql.NullString
	numReports      sql.NullInt64
	distinguished   sql.NullString
	depth           sql.NullInt64
}

// dest returns a slice of pointers to scan into, in the correct column order.
func (d *commentScanDest) dest() []interface{} {
	return []interface{}{
		&d.id,
		&d.name,
		&d.score,
		&d.ups,
		&d.downs,
		&d.likes,
		&d.created,
		&d.createdUTC,
		&d.approvedBy,
		&d.author,
		&d.authorFlairCSS,
		&d.authorFlairText,
		&d.bannedBy,
		&d.body,
		&d.bodyHTML,
		&d.editedIsEdited,
		&d.editedTs,
		&d.gilded,
		&d.linkAuthor,
		&d.linkID,
		&d.linkTitle,
		&d.linkURL,
		&d.numReports,
		&d.parentID,
		&d.saved,
		&d.scoreHidden,
		&d.subreddit,
		&d.subredditID,
		&d.distinguished,
		&d.depth,
	}
}

// toComment converts the scanned values to a Comment struct.
func (d *commentScanDest) toComment() *types.Comment {
	c := &types.Comment{
		ThingData: types.ThingData{
			ID:   d.id,
			Name: d.name,
		},
		Votable: types.Votable{
			Score: d.score,
			Ups:   d.ups,
			Downs: d.downs,
		},
		Created: types.Created{
			Created:    d.created,
			CreatedUTC: d.createdUTC,
		},
		Author:      d.author,
		Body:        d.body,
		BodyHTML:    d.bodyHTML,
		Gilded:      d.gilded,
		LinkID:      d.linkID,
		ParentID:    d.parentID,
		Saved:       d.saved,
		ScoreHidden: d.scoreHidden,
		Subreddit:   d.subreddit,
		SubredditID: d.subredditID,
		Edited: types.Edited{
			IsEdited:  d.editedIsEdited,
			Timestamp: d.editedTs,
		},
		Replies:         []*types.Comment{},
		MoreChildrenIDs: []string{},
	}

	// Convert nullable fields
	if d.likes.Valid {
		b := d.likes.Int64 != 0
		c.Likes = &b
	}
	if d.approvedBy.Valid {
		c.ApprovedBy = &d.approvedBy.String
	}
	if d.authorFlairCSS.Valid {
		c.AuthorFlairCSSClass = &d.authorFlairCSS.String
	}
	if d.authorFlairText.Valid {
		c.AuthorFlairText = &d.authorFlairText.String
	}
	if d.bannedBy.Valid {
		c.BannedBy = &d.bannedBy.String
	}
	if d.linkAuthor.Valid {
		c.LinkAuthor = d.linkAuthor.String
	}
	if d.linkTitle.Valid {
		c.LinkTitle = d.linkTitle.String
	}
	if d.linkURL.Valid {
		c.LinkURL = d.linkURL.String
	}
	if d.numReports.Valid {
		val := int(d.numReports.Int64)
		c.NumReports = &val
	}
	if d.distinguished.Valid {
		c.Distinguished = &d.distinguished.String
	}

	return c
}

// newCommentScanDest creates a new commentScanDest for scanning a Comment from SQL.
// Usage:
//
//	dest := newCommentScanDest()
//	err := row.Scan(dest.dest()...)
//	if err != nil { return err }
//	comment := dest.toComment()
func newCommentScanDest() *commentScanDest {
	return &commentScanDest{}
}

// scanToComment reconstructs a Comment from scanned SQL values.
// It handles all type conversions and null values, returning an error if any value is invalid.
//
// The values slice must contain fields in this exact order:
//
//	id, name, score, ups, downs, likes, created, created_utc,
//	approved_by, author, author_flair_css_class, author_flair_text, banned_by,
//	body, body_html, edited_is_edited, edited_timestamp, gilded,
//	link_author, link_id, link_title, link_url, num_reports, parent_id,
//	saved, score_hidden, subreddit, subreddit_id, distinguished, depth
//
// Note: Replies and MoreChildrenIDs are NOT stored in the database and will be empty.
func scanToComment(values []interface{}) (*types.Comment, error) {
	if len(values) != 30 {
		return nil, fmt.Errorf("expected 30 values, got %d", len(values))
	}

	c := &types.Comment{}

	// Extract ThingData
	if id, ok := values[0].(string); ok {
		c.ID = id
	} else {
		return nil, fmt.Errorf("invalid type for ID: expected string")
	}

	if name, ok := values[1].(string); ok {
		c.Name = name
	} else {
		return nil, fmt.Errorf("invalid type for Name: expected string")
	}

	// Extract Votable
	if score, ok := values[2].(int); ok {
		c.Score = score
	} else {
		return nil, fmt.Errorf("invalid type for Score: expected int")
	}

	if ups, ok := values[3].(int); ok {
		c.Ups = ups
	} else {
		return nil, fmt.Errorf("invalid type for Ups: expected int")
	}

	if downs, ok := values[4].(int); ok {
		c.Downs = downs
	} else {
		return nil, fmt.Errorf("invalid type for Downs: expected int")
	}

	if likes, ok := values[5].(sql.NullInt64); ok && likes.Valid {
		b := likes.Int64 != 0
		c.Likes = &b
	}

	// Extract Created
	if created, ok := values[6].(float64); ok {
		c.Created.Created = created
	} else {
		return nil, fmt.Errorf("invalid type for Created: expected float64")
	}

	if createdUTC, ok := values[7].(float64); ok {
		c.Created.CreatedUTC = createdUTC
	} else {
		return nil, fmt.Errorf("invalid type for CreatedUTC: expected float64")
	}

	// Extract Comment-specific fields
	if approvedBy, ok := values[8].(sql.NullString); ok && approvedBy.Valid {
		c.ApprovedBy = &approvedBy.String
	}

	if author, ok := values[9].(string); ok {
		c.Author = author
	} else {
		return nil, fmt.Errorf("invalid type for Author: expected string")
	}

	if authorFlairCSSClass, ok := values[10].(sql.NullString); ok && authorFlairCSSClass.Valid {
		c.AuthorFlairCSSClass = &authorFlairCSSClass.String
	}

	if authorFlairText, ok := values[11].(sql.NullString); ok && authorFlairText.Valid {
		c.AuthorFlairText = &authorFlairText.String
	}

	if bannedBy, ok := values[12].(sql.NullString); ok && bannedBy.Valid {
		c.BannedBy = &bannedBy.String
	}

	if body, ok := values[13].(string); ok {
		c.Body = body
	} else {
		return nil, fmt.Errorf("invalid type for Body: expected string")
	}

	if bodyHTML, ok := values[14].(string); ok {
		c.BodyHTML = bodyHTML
	} else {
		return nil, fmt.Errorf("invalid type for BodyHTML: expected string")
	}

	if isEdited, ok := values[15].(bool); ok {
		c.Edited.IsEdited = isEdited
	} else {
		return nil, fmt.Errorf("invalid type for Edited.IsEdited: expected bool")
	}

	if timestamp, ok := values[16].(float64); ok {
		c.Edited.Timestamp = timestamp
	} else {
		return nil, fmt.Errorf("invalid type for Edited.Timestamp: expected float64")
	}

	if gilded, ok := values[17].(int); ok {
		c.Gilded = gilded
	} else {
		return nil, fmt.Errorf("invalid type for Gilded: expected int")
	}

	if linkAuthor, ok := values[18].(sql.NullString); ok && linkAuthor.Valid {
		c.LinkAuthor = linkAuthor.String
	}

	if linkID, ok := values[19].(string); ok {
		c.LinkID = linkID
	} else {
		return nil, fmt.Errorf("invalid type for LinkID: expected string")
	}

	if linkTitle, ok := values[20].(sql.NullString); ok && linkTitle.Valid {
		c.LinkTitle = linkTitle.String
	}

	if linkURL, ok := values[21].(sql.NullString); ok && linkURL.Valid {
		c.LinkURL = linkURL.String
	}

	if numReports, ok := values[22].(sql.NullInt64); ok && numReports.Valid {
		val := int(numReports.Int64)
		c.NumReports = &val
	}

	if parentID, ok := values[23].(string); ok {
		c.ParentID = parentID
	} else {
		return nil, fmt.Errorf("invalid type for ParentID: expected string")
	}

	if saved, ok := values[24].(bool); ok {
		c.Saved = saved
	} else {
		return nil, fmt.Errorf("invalid type for Saved: expected bool")
	}

	if scoreHidden, ok := values[25].(bool); ok {
		c.ScoreHidden = scoreHidden
	} else {
		return nil, fmt.Errorf("invalid type for ScoreHidden: expected bool")
	}

	if subreddit, ok := values[26].(string); ok {
		c.Subreddit = subreddit
	} else {
		return nil, fmt.Errorf("invalid type for Subreddit: expected string")
	}

	if subredditID, ok := values[27].(string); ok {
		c.SubredditID = subredditID
	} else {
		return nil, fmt.Errorf("invalid type for SubredditID: expected string")
	}

	if distinguished, ok := values[28].(sql.NullString); ok && distinguished.Valid {
		c.Distinguished = &distinguished.String
	}

	// depth (values[29]) is not stored in the Comment struct itself,
	// it's calculated and used only for storage/retrieval

	// Replies and MoreChildrenIDs are initialized as empty slices
	// They are not stored in the database
	c.Replies = []*types.Comment{}
	c.MoreChildrenIDs = []string{}

	return c, nil
}

// commentToInsertArgs returns a slice of values for INSERT/UPDATE statements.
// It converts Go types to SQL-compatible types, handling null values appropriately.
//
// The depth parameter must be calculated by the caller:
//   - Top-level comments (ParentID starts with "t3_" or is empty) have depth = 0
//   - Child comments have depth = parent depth + 1
//
// The values are returned in this order:
//
//	id, name, score, ups, downs, likes, created, created_utc,
//	approved_by, author, author_flair_css_class, author_flair_text, banned_by,
//	body, body_html, edited_is_edited, edited_timestamp, gilded,
//	link_author, link_id, link_title, link_url, num_reports, parent_id,
//	saved, score_hidden, subreddit, subreddit_id, distinguished, depth
//
// Note: Replies and MoreChildrenIDs are NOT stored.
func commentToInsertArgs(c *types.Comment, depth int) []interface{} {
	return []interface{}{
		// ThingData
		c.ID,
		c.Name,
		// Votable
		c.Score,
		c.Ups,
		c.Downs,
		boolPtrToNullInt64(c.Likes),
		// Created
		c.Created.Created,
		c.Created.CreatedUTC,
		// Comment-specific fields
		stringPtrToNullString(c.ApprovedBy),
		c.Author,
		stringPtrToNullString(c.AuthorFlairCSSClass),
		stringPtrToNullString(c.AuthorFlairText),
		stringPtrToNullString(c.BannedBy),
		c.Body,
		c.BodyHTML,
		c.Edited.IsEdited,
		c.Edited.Timestamp,
		c.Gilded,
		stringToNullString(c.LinkAuthor),
		c.LinkID,
		stringToNullString(c.LinkTitle),
		stringToNullString(c.LinkURL),
		intPtrToNullInt64(c.NumReports),
		c.ParentID,
		c.Saved,
		c.ScoreHidden,
		c.Subreddit,
		c.SubredditID,
		stringPtrToNullString(c.Distinguished),
		depth,
	}
}

// Helper functions for type conversions

// boolPtrToNullInt64 converts a *bool to sql.NullInt64.
// True becomes 1, false becomes 0, nil becomes NULL.
func boolPtrToNullInt64(b *bool) sql.NullInt64 {
	if b == nil {
		return sql.NullInt64{Valid: false}
	}
	val := int64(0)
	if *b {
		val = 1
	}
	return sql.NullInt64{Int64: val, Valid: true}
}

// stringPtrToNullString converts a *string to sql.NullString.
// Non-nil strings become valid NullStrings, nil becomes NULL.
func stringPtrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// stringToNullString converts a string to sql.NullString.
// Empty strings become NULL, non-empty strings become valid NullStrings.
func stringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// intPtrToNullInt64 converts a *int to sql.NullInt64.
// Non-nil ints become valid NullInt64s, nil becomes NULL.
func intPtrToNullInt64(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}
