package db

import (
	"database/sql"
	"time"
)

// TimeLayout is the canonical on-disk timestamp format for every TEXT
// timestamp column in the schema: always UTC, always nanosecond precision
// with a fixed width. A fixed width matters because filters and the
// occurred_at/id ordering index rely on plain lexical (byte) comparison
// matching chronological order — time.RFC3339Nano would trim trailing zero
// fractional digits, breaking that for rows whose timestamps differ only in
// precision.
const TimeLayout = "2006-01-02T15:04:05.000000000Z"

// FormatTime renders t in the canonical storage format, converting to UTC
// first so two equal instants in different zones always produce the same
// text (and therefore compare equal and sort identically).
func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeLayout)
}

// ParseTime parses a timestamp written by FormatTime.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(TimeLayout, s)
}

// FormatTimePtr is FormatTime for an optional column: a nil t stores SQL
// NULL rather than an empty string.
func FormatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return FormatTime(*t)
}

// ParseNullTime turns a nullable TEXT column read via sql.NullString into an
// optional time.Time.
func ParseNullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}
	t, err := ParseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
