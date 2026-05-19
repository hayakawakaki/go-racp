package domain

import "errors"

var (
	ErrNotFound      = errors.New("mob: not found")
	ErrEmptySnapshot = errors.New("mob: database not loaded")
)
