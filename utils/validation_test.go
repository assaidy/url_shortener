package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testUser struct {
	Username string `validate:"required,customUsername"`
}

type testContent struct {
	Title string `validate:"required,customNoOuterSpaces"`
}

type testUrl struct {
	ShortCode string `validate:"customShortUrl"`
}

type testMultiField struct {
	Username  string `validate:"customUsername"`
	Title     string `validate:"customNoOuterSpaces"`
	ShortCode string `validate:"customShortUrl"`
}

func TestCustomUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{"valid simple", "user123", true},
		{"valid with underscore", "user_name", true},
		{"valid numbers only", "12345", true},
		{"invalid with space", "user name", false},
		{"invalid special char", "user@name", false},
		{"invalid empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := testUser{Username: tt.username}
			err := ValidateStruct(u)
			assert.Equal(t, tt.want, err == nil)
		})
	}
}

func TestCustomNoOuterSpaces(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"valid no spaces", "title", true},
		{"valid with inner spaces", "title with spaces", true},
		{"valid single char", "a", true},
		{"invalid empty", "", false},
		{"invalid leading space", " title", false},
		{"invalid trailing space", "title ", false},
		{"invalid only space", " ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testContent{Title: tt.title}
			err := ValidateStruct(c)
			assert.Equal(t, tt.want, err == nil)
		})
	}
}

func TestCustomShortUrl(t *testing.T) {
	tests := []struct {
		name      string
		shortCode string
		want      bool
	}{
		{"valid alphanumeric", "abc123", true},
		{"valid letters only", "ABC", true},
		{"valid numbers only", "123", true},
		{"valid empty", "", true},
		{"invalid with hyphen", "abc-123", false},
		{"invalid with space", "abc def", false},
		{"invalid with slash", "abc/def", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := testUrl{ShortCode: tt.shortCode}
			err := ValidateStruct(u)
			assert.Equal(t, tt.want, err == nil)
		})
	}
}

func TestValidateStruct(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		s := testMultiField{
			Username:  "valid_user",
			Title:     "valid title",
			ShortCode: "abc123",
		}
		err := ValidateStruct(s)
		assert.NoError(t, err)
	})

	t.Run("invalid struct multiple fields", func(t *testing.T) {
		s := testMultiField{
			Username:  "invalid user",
			Title:     " invalid title ",
			ShortCode: "invalid!",
		}
		err := ValidateStruct(s)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Username: violation in constraint 'customUsername'")
		assert.Contains(t, err.Error(), "Title: violation in constraint 'customNoOuterSpaces'")
		assert.Contains(t, err.Error(), "ShortCode: violation in constraint 'customShortUrl'")
	})
}
