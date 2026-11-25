package redirection

import "errors"

// Errors returned by the redirection service.
var (
	ErrRedirectionNotFound = errors.New("redirection not found")
	ErrInvalidSort         = errors.New("invalid sort")
)
