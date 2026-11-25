package auth

import "errors"

// Errors returned by the auth service.
var (
	ErrUsernameConflict   = errors.New("username conflict")
	ErrUserNotFound       = errors.New("user not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSessionNotFound    = errors.New("session not found")
	ErrInvalidOldPassword = errors.New("invalid old password")
)
