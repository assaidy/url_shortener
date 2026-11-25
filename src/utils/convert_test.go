package utils_test

import (
	"database/sql"
	"testing"
	"time"
	"uuid"

	"github.com/assaidy/url_shortener/src/utils"
	"github.com/stretchr/testify/require"
)

func TestUuidPtr(t *testing.T) {
	t.Parallel()

	id := uuid.NewV7()
	require.Equal(t, &id, utils.UuidPtr(id))
	require.Nil(t, utils.UuidPtr(uuid.Nil()))
}

func TestNullTime(t *testing.T) {
	t.Parallel()

	now := time.Now()
	require.Equal(t, sql.NullTime{Time: now, Valid: true}, utils.NullTime(now))
	require.Equal(t, sql.NullTime{}, utils.NullTime(time.Time{}))
}

func TestNullString(t *testing.T) {
	t.Parallel()

	require.Equal(t, sql.NullString{String: "value", Valid: true}, utils.NullString("value"))
	require.Equal(t, sql.NullString{}, utils.NullString(""))
}
