package internal

import (
	"database/sql"
	"time"
)

// StringToNullString converts a string to sql.NullString.
// Empty strings are converted to NULL, non-empty strings to valid values.
func StringToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// NullStringToString converts sql.NullString to string.
// Returns the string value if Valid is true, or an empty string if NULL.
func NullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// Int64ToNullInt64 converts int64 to sql.NullInt64.
// Zero values are converted to NULL (following SQL conventions for absence of integer data).
func Int64ToNullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

// NullInt64ToInt64 converts sql.NullInt64 to int64.
// Returns the int64 value if Valid is true, or 0 if NULL.
func NullInt64ToInt64(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// IntToNullInt64 converts int to sql.NullInt64.
// Zero values are converted to NULL.
func IntToNullInt64(i int) sql.NullInt64 {
	return Int64ToNullInt64(int64(i))
}

// NullInt64ToInt converts sql.NullInt64 to int.
// Returns the int value if Valid is true, or 0 if NULL.
func NullInt64ToInt(ni sql.NullInt64) int {
	return int(NullInt64ToInt64(ni))
}

// BoolToNullBool converts bool to sql.NullBool.
// False values are converted to valid FALSE, true values to valid TRUE.
// There is no "null" representation for booleans in this conversion
// since we always have a boolean value.
func BoolToNullBool(b bool) sql.NullBool {
	return sql.NullBool{Bool: b, Valid: true}
}

// NullBoolToBool converts sql.NullBool to bool.
// Returns the bool value if Valid is true, or false if NULL.
func NullBoolToBool(nb sql.NullBool) bool {
	if nb.Valid {
		return nb.Bool
	}
	return false
}

// Float64ToNullFloat64 converts float64 to sql.NullFloat64.
// Zero values are converted to NULL.
func Float64ToNullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: f, Valid: true}
}

// NullFloat64ToFloat64 converts sql.NullFloat64 to float64.
// Returns the float64 value if Valid is true, or 0 if NULL.
func NullFloat64ToFloat64(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// TimeToNullTime converts time.Time to sql.NullTime.
// Zero-value times are converted to NULL.
func TimeToNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// NullTimeToTime converts sql.NullTime to time.Time.
// Returns the time value if Valid is true, or zero-value time if NULL.
func NullTimeToTime(nt sql.NullTime) time.Time {
	if nt.Valid {
		return nt.Time
	}
	return time.Time{}
}
