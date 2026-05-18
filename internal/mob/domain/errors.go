package domain

import "errors"

var (
	ErrNotFound       = errors.New("mob: not found")
	ErrEmptySnapshot  = errors.New("mob: database not loaded")
	ErrParseFailed    = errors.New("mob: parse failed")
	ErrReloadConflict = errors.New("mob: reload in progress")
)
