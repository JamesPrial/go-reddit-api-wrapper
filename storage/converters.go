package storage

import (
	"database/sql"
	"time"
)

// stringToNullString converts a string to sql.NullString.
// Empty strings are converted to NULL, non-empty strings to valid values.
func stringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullStringToString converts sql.NullString to string.
// Returns the string value if Valid is true, or an empty string if NULL.
func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// int64ToNullInt64 converts int64 to sql.NullInt64.
// Zero values are converted to NULL (following SQL conventions for absence of integer data).
func int64ToNullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

// nullInt64ToInt64 converts sql.NullInt64 to int64.
// Returns the int64 value if Valid is true, or 0 if NULL.
func nullInt64ToInt64(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// intToNullInt64 converts int to sql.NullInt64.
// Zero values are converted to NULL.
func intToNullInt64(i int) sql.NullInt64 {
	return int64ToNullInt64(int64(i))
}

// nullInt64ToInt converts sql.NullInt64 to int.
// Returns the int value if Valid is true, or 0 if NULL.
func nullInt64ToInt(ni sql.NullInt64) int {
	return int(nullInt64ToInt64(ni))
}

// boolToNullBool converts bool to sql.NullBool.
// False values are converted to valid FALSE, true values to valid TRUE.
// There is no "null" representation for booleans in this conversion
// since we always have a boolean value.
func boolToNullBool(b bool) sql.NullBool {
	return sql.NullBool{Bool: b, Valid: true}
}

// nullBoolToBool converts sql.NullBool to bool.
// Returns the bool value if Valid is true, or false if NULL.
func nullBoolToBool(nb sql.NullBool) bool {
	if nb.Valid {
		return nb.Bool
	}
	return false
}

// float64ToNullFloat64 converts float64 to sql.NullFloat64.
// Zero values are converted to NULL.
func float64ToNullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

// nullFloat64ToFloat64 converts sql.NullFloat64 to float64.
// Returns the float64 value if Valid is true, or 0 if NULL.
func nullFloat64ToFloat64(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// timeToNullTime converts time.Time to sql.NullTime.
// Zero-value times are converted to NULL.
func timeToNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// nullTimeToTime converts sql.NullTime to time.Time.
// Returns the time value if Valid is true, or zero-value time if NULL.
func nullTimeToTime(nt sql.NullTime) time.Time {
	if nt.Valid {
		return nt.Time
	}
	return time.Time{}
}
