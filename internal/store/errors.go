// internal/store/errors.go
package store

import "errors"

var (
	ErrNotFound       = errors.New("store: not found")
	ErrAlreadyExists  = errors.New("store: already exists")
	ErrInvalidConfig  = errors.New("store: invalid config")
	ErrInvalidSession = errors.New("store: invalid session")
	ErrInvalidTag     = errors.New("invalid tag")
	ErrInvalidTemplate = errors.New("invalid template")
)
