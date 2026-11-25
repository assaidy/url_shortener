package utils

import (
	"database/sql"
	"time"
	"uuid"
)

// UuidPtr returns a pointer to id, or nil when id is the nil UUID. It is used
// to pass an optional uuid to sqlc queries.
func UuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil() {
		return nil
	}
	return &id
}

// NullTime returns a valid sql.NullTime holding t, or an invalid one when t is
// the zero time.
func NullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}

// NullString returns a valid sql.NullString holding s, or an invalid one when s
// is empty.
func NullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
