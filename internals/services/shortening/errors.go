package shortening

import "errors"

// Errors returned by the shortening service.
var (
	ErrUrlNotFound = errors.New("url not found")
	ErrInvalidSort = errors.New("invalid sort")
)
