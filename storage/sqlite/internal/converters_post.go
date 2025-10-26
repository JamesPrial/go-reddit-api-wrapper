package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jamesprial/go-reddit-api-wrapper/pkg/types"
)

// postScanDest holds intermediate values for scanning a Post from SQL.
// This struct maintains the nullable SQL types that need to be converted after scanning.
type postScanDest struct {
	// Direct fields
	id             string
	name           string
	score          int
	ups            int
	downs          int
	created        float64
	createdUTC     float64
	author         string
	clicked        bool
	domain         string
	hidden         bool
	isSelf         bool
	locked         bool
	numComments    int
	over18         bool
	permalink      string
	saved          bool
	selftext       string
	subreddit      string
	subredditID    string
	thumbnail      string
	title          string
	url            string
	editedIsEdited bool
	stickied       bool
	upvoteRatio    float64

	// Nullable fields
	likes           sql.NullInt64
	authorFlairCSS  sql.NullString
	authorFlairText sql.NullString
	linkFlairCSS    sql.NullString
	linkFlairText   sql.NullString
	media           sql.NullString
	mediaEmbed      sql.NullString
	selftextHTML    sql.NullString
	distinguished   sql.NullString
	editedTs        sql.NullFloat64
}

// dest returns a slice of pointers to scan into, in the correct column order.
func (d *postScanDest) dest() []interface{} {
	return []interface{}{
		&d.id,
		&d.name,
		&d.score,
		&d.ups,
		&d.downs,
		&d.likes,
		&d.created,
		&d.createdUTC,
		&d.author,
		&d.authorFlairCSS,
		&d.authorFlairText,
		&d.clicked,
		&d.domain,
		&d.hidden,
		&d.isSelf,
		&d.linkFlairCSS,
		&d.linkFlairText,
		&d.locked,
		&d.media,
		&d.mediaEmbed,
		&d.numComments,
		&d.over18,
		&d.permalink,
		&d.saved,
		&d.selftext,
		&d.selftextHTML,
		&d.subreddit,
		&d.subredditID,
		&d.thumbnail,
		&d.title,
		&d.url,
		&d.editedIsEdited,
		&d.editedTs,
		&d.distinguished,
		&d.stickied,
		&d.upvoteRatio,
	}
}

// toPost converts the scanned values to a Post struct.
func (d *postScanDest) toPost() *types.Post {
	p := &types.Post{
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
		Clicked:     d.clicked,
		Domain:      d.domain,
		Hidden:      d.hidden,
		IsSelf:      d.isSelf,
		Locked:      d.locked,
		NumComments: d.numComments,
		Over18:      d.over18,
		Permalink:   d.permalink,
		Saved:       d.saved,
		SelfText:    d.selftext,
		Subreddit:   d.subreddit,
		SubredditID: d.subredditID,
		Thumbnail:   d.thumbnail,
		Title:       d.title,
		URL:         d.url,
		Stickied:    d.stickied,
		UpvoteRatio: d.upvoteRatio,
	}

	// Set Edited field
	if d.editedTs.Valid {
		p.Edited = types.Edited{
			IsEdited:  d.editedIsEdited,
			Timestamp: d.editedTs.Float64,
		}
	} else {
		p.Edited = types.Edited{
			IsEdited:  d.editedIsEdited,
			Timestamp: 0,
		}
	}

	// Convert nullable fields
	if d.likes.Valid {
		b := d.likes.Int64 != 0
		p.Likes = &b
	}
	if d.authorFlairCSS.Valid {
		p.AuthorFlairCSSClass = &d.authorFlairCSS.String
	}
	if d.authorFlairText.Valid {
		p.AuthorFlairText = &d.authorFlairText.String
	}
	if d.linkFlairCSS.Valid {
		p.LinkFlairCSSClass = &d.linkFlairCSS.String
	}
	if d.linkFlairText.Valid {
		p.LinkFlairText = &d.linkFlairText.String
	}
	if d.media.Valid && d.media.String != "" {
		p.Media = json.RawMessage(d.media.String)
	}
	if d.mediaEmbed.Valid && d.mediaEmbed.String != "" {
		p.MediaEmbed = json.RawMessage(d.mediaEmbed.String)
	}
	if d.selftextHTML.Valid {
		p.SelfTextHTML = &d.selftextHTML.String
	}
	if d.distinguished.Valid {
		p.Distinguished = &d.distinguished.String
	}

	return p
}

// newPostScanDest creates a new postScanDest for scanning a Post from SQL.
// Usage:
//
//	dest := newPostScanDest()
//	err := row.Scan(dest.dest()...)
//	if err != nil { return err }
//	post := dest.toPost()
func newPostScanDest() *postScanDest {
	return &postScanDest{}
}

// scanToPost reconstructs a Post from scanned SQL values. It handles all type conversions
// and null values, returning an error if any value is invalid.
//
// The values slice must contain all Post fields in the same order as postToScanDest.
// This function is the inverse of postToScanDest and postToInsertArgs.
func scanToPost(values []interface{}) (*types.Post, error) {
	if len(values) != 36 {
		return nil, fmt.Errorf("expected 36 values for Post, got %d", len(values))
	}

	p := &types.Post{}

	// Helper to safely extract values with type checking
	var ok bool

	// ThingData fields
	if p.ID, ok = values[0].(string); !ok {
		return nil, fmt.Errorf("field 0 (ID) is not a string")
	}
	if p.Name, ok = values[1].(string); !ok {
		return nil, fmt.Errorf("field 1 (Name) is not a string")
	}

	// Votable fields
	if p.Score, ok = values[2].(int); !ok {
		return nil, fmt.Errorf("field 2 (Score) is not an int")
	}
	if p.Ups, ok = values[3].(int); !ok {
		return nil, fmt.Errorf("field 3 (Ups) is not an int")
	}
	if p.Downs, ok = values[4].(int); !ok {
		return nil, fmt.Errorf("field 4 (Downs) is not an int")
	}
	// Likes is a pointer, handle null
	if likes, ok := values[5].(*bool); ok {
		p.Likes = likes
	} else if values[5] != nil {
		return nil, fmt.Errorf("field 5 (Likes) is not a *bool or nil")
	}

	// Created fields (from embedded Created struct)
	var created, createdUTC float64
	if created, ok = values[6].(float64); !ok {
		return nil, fmt.Errorf("field 6 (Created) is not a float64")
	}
	if createdUTC, ok = values[7].(float64); !ok {
		return nil, fmt.Errorf("field 7 (CreatedUTC) is not a float64")
	}
	p.Created.Created = created
	p.Created.CreatedUTC = createdUTC

	// Post-specific fields
	if p.Author, ok = values[8].(string); !ok {
		return nil, fmt.Errorf("field 8 (Author) is not a string")
	}
	if authorFlairCSSClass, ok := values[9].(*string); ok {
		p.AuthorFlairCSSClass = authorFlairCSSClass
	} else if values[9] != nil {
		return nil, fmt.Errorf("field 9 (AuthorFlairCSSClass) is not a *string or nil")
	}
	if authorFlairText, ok := values[10].(*string); ok {
		p.AuthorFlairText = authorFlairText
	} else if values[10] != nil {
		return nil, fmt.Errorf("field 10 (AuthorFlairText) is not a *string or nil")
	}
	if p.Clicked, ok = values[11].(bool); !ok {
		return nil, fmt.Errorf("field 11 (Clicked) is not a bool")
	}
	if p.Domain, ok = values[12].(string); !ok {
		return nil, fmt.Errorf("field 12 (Domain) is not a string")
	}
	if p.Hidden, ok = values[13].(bool); !ok {
		return nil, fmt.Errorf("field 13 (Hidden) is not a bool")
	}
	if p.IsSelf, ok = values[14].(bool); !ok {
		return nil, fmt.Errorf("field 14 (IsSelf) is not a bool")
	}
	if linkFlairCSSClass, ok := values[15].(*string); ok {
		p.LinkFlairCSSClass = linkFlairCSSClass
	} else if values[15] != nil {
		return nil, fmt.Errorf("field 15 (LinkFlairCSSClass) is not a *string or nil")
	}
	if linkFlairText, ok := values[16].(*string); ok {
		p.LinkFlairText = linkFlairText
	} else if values[16] != nil {
		return nil, fmt.Errorf("field 16 (LinkFlairText) is not a *string or nil")
	}
	if p.Locked, ok = values[17].(bool); !ok {
		return nil, fmt.Errorf("field 17 (Locked) is not a bool")
	}

	// JSON fields (Media, MediaEmbed)
	if media, ok := values[18].(json.RawMessage); ok {
		p.Media = media
	} else if values[18] != nil {
		return nil, fmt.Errorf("field 18 (Media) is not json.RawMessage or nil")
	}
	if mediaEmbed, ok := values[19].(json.RawMessage); ok {
		p.MediaEmbed = mediaEmbed
	} else if values[19] != nil {
		return nil, fmt.Errorf("field 19 (MediaEmbed) is not json.RawMessage or nil")
	}

	if p.NumComments, ok = values[20].(int); !ok {
		return nil, fmt.Errorf("field 20 (NumComments) is not an int")
	}
	if p.Over18, ok = values[21].(bool); !ok {
		return nil, fmt.Errorf("field 21 (Over18) is not a bool")
	}
	if p.Permalink, ok = values[22].(string); !ok {
		return nil, fmt.Errorf("field 22 (Permalink) is not a string")
	}
	if p.Saved, ok = values[23].(bool); !ok {
		return nil, fmt.Errorf("field 23 (Saved) is not a bool")
	}
	if p.SelfText, ok = values[24].(string); !ok {
		return nil, fmt.Errorf("field 24 (SelfText) is not a string")
	}
	if selfTextHTML, ok := values[25].(*string); ok {
		p.SelfTextHTML = selfTextHTML
	} else if values[25] != nil {
		return nil, fmt.Errorf("field 25 (SelfTextHTML) is not a *string or nil")
	}
	if p.Subreddit, ok = values[26].(string); !ok {
		return nil, fmt.Errorf("field 26 (Subreddit) is not a string")
	}
	if p.SubredditID, ok = values[27].(string); !ok {
		return nil, fmt.Errorf("field 27 (SubredditID) is not a string")
	}
	if p.Thumbnail, ok = values[28].(string); !ok {
		return nil, fmt.Errorf("field 28 (Thumbnail) is not a string")
	}
	if p.Title, ok = values[29].(string); !ok {
		return nil, fmt.Errorf("field 29 (Title) is not a string")
	}
	if p.URL, ok = values[30].(string); !ok {
		return nil, fmt.Errorf("field 30 (URL) is not a string")
	}

	// Edited field (special handling)
	if isEdited, ok := values[31].(bool); ok {
		p.Edited.IsEdited = isEdited
	} else {
		return nil, fmt.Errorf("field 31 (Edited.IsEdited) is not a bool")
	}
	if timestamp, ok := values[32].(float64); ok {
		p.Edited.Timestamp = timestamp
	} else {
		return nil, fmt.Errorf("field 32 (Edited.Timestamp) is not a float64")
	}

	if distinguished, ok := values[33].(*string); ok {
		p.Distinguished = distinguished
	} else if values[33] != nil {
		return nil, fmt.Errorf("field 33 (Distinguished) is not a *string or nil")
	}
	if p.Stickied, ok = values[34].(bool); !ok {
		return nil, fmt.Errorf("field 34 (Stickied) is not a bool")
	}
	if p.UpvoteRatio, ok = values[35].(float64); !ok {
		return nil, fmt.Errorf("field 35 (UpvoteRatio) is not a float64")
	}

	return p, nil
}

// postToInsertArgs returns a slice of values for INSERT/UPDATE statements.
// It converts Go types to SQL-compatible types and handles null values appropriately.
//
// The order must match the column order in INSERT/UPDATE statements and must be
// compatible with the table schema defined in migrations.
func postToInsertArgs(p *types.Post) []interface{} {
	// Convert Media and MediaEmbed to string for SQLite TEXT storage
	var mediaStr, mediaEmbedStr sql.NullString
	if p.Media != nil && len(p.Media) > 0 {
		mediaStr = sql.NullString{String: string(p.Media), Valid: true}
	}
	if p.MediaEmbed != nil && len(p.MediaEmbed) > 0 {
		mediaEmbedStr = sql.NullString{String: string(p.MediaEmbed), Valid: true}
	}

	return []interface{}{
		p.ID,
		p.Name,
		p.Score,
		p.Ups,
		p.Downs,
		boolPtrToNullInt64(p.Likes),
		p.Created.Created,
		p.Created.CreatedUTC,
		p.Author,
		stringPtrToNullString(p.AuthorFlairCSSClass),
		stringPtrToNullString(p.AuthorFlairText),
		p.Clicked,
		p.Domain,
		p.Hidden,
		p.IsSelf,
		stringPtrToNullString(p.LinkFlairCSSClass),
		stringPtrToNullString(p.LinkFlairText),
		p.Locked,
		mediaStr,
		mediaEmbedStr,
		p.NumComments,
		p.Over18,
		p.Permalink,
		p.Saved,
		p.SelfText,
		stringPtrToNullString(p.SelfTextHTML),
		p.Subreddit,
		p.SubredditID,
		p.Thumbnail,
		p.Title,
		p.URL,
		p.Edited.IsEdited,
		editedToNullFloat64(p.Edited),
		stringPtrToNullString(p.Distinguished),
		p.Stickied,
		p.UpvoteRatio,
	}
}

// editedToNullFloat64 converts types.Edited to sql.NullFloat64 for timestamp storage.
// If IsEdited is false, returns SQL NULL.
// If IsEdited is true and Timestamp > 0, returns the timestamp.
// If IsEdited is true and Timestamp == 0, returns SQL NULL (old edit without timestamp).
func editedToNullFloat64(e types.Edited) sql.NullFloat64 {
	if !e.IsEdited || e.Timestamp == 0 {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: e.Timestamp, Valid: true}
}

// nullFloat64ToEdited converts sql.NullFloat64 back to types.Edited.
// SQL NULL with IsEdited=false means not edited.
// SQL NULL with IsEdited=true means old edit without timestamp.
// Valid timestamp means modern edit with timestamp.
//
// Note: This function requires IsEdited to be set separately from the database.
func nullFloat64ToEdited(n sql.NullFloat64) types.Edited {
	if !n.Valid {
		return types.Edited{IsEdited: false, Timestamp: 0}
	}
	return types.Edited{IsEdited: true, Timestamp: n.Float64}
}
