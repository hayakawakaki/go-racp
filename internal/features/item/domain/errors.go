package domain

import "errors"

var (
	ErrNotFound      = errors.New("item: not found")
	ErrEmptySnapshot = errors.New("item: database not loaded")
)
