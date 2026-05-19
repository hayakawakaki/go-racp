package domain

import "errors"

var (
	ErrNotFound        = errors.New("news: not found")
	ErrInvalidCategory = errors.New("news: invalid category")
	ErrTitleEmpty      = errors.New("news: title is required")
	ErrTitleTooLong    = errors.New("news: title is too long")
	ErrBodyEmpty       = errors.New("news: body is required")
	ErrBodyTooLong     = errors.New("news: body is too long")
)
