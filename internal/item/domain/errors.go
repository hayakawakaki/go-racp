package domain

import "errors"

var (
	ErrNotFound       = errors.New("item: not found")
	ErrEmptySnapshot  = errors.New("item: database not loaded")
	ErrParseFailed    = errors.New("item: parse failed")
	ErrReloadConflict = errors.New("item: reload in progress")
)
