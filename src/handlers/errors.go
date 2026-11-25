package handlers

import (
	"net/http"

	"github.com/assaidy/moon"
)

var (
	ErrInvalidJson         = moon.NewError(http.StatusBadRequest, "invalid_json", "The request body contains invalid JSON.")
	ErrInvalidPayload      = moon.NewError(http.StatusUnprocessableEntity, "invalid_payload", "The request payload failed validation.")
	ErrUsernameConflict    = moon.NewError(http.StatusConflict, "username_conflict", "An account with this username already exists.")
	ErrInvalidOldPassword  = moon.NewError(http.StatusUnprocessableEntity, "invalid_old_password", "The current password is incorrect.")
	ErrInvalidCursor       = moon.NewError(http.StatusBadRequest, "invalid_cursor", "The provided cursor is invalid.")
	ErrUrlNotFound         = moon.NewError(http.StatusNotFound, "url_not_found", "The requested URL was not found.")
	ErrRedirectionNotFound = moon.NewError(http.StatusNotFound, "redirection_not_found", "The requested redirection was not found.")
)
