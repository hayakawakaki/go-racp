package domain

import "errors"

var (
	ErrNotFound        = errors.New("users: account not found")
	ErrSelfAction      = errors.New("users: cannot perform action on own account")
	ErrTargetIsAdmin   = errors.New("users: cannot act on another admin account through the UI")
	ErrInvalidRole     = errors.New("users: invalid role / group_id")
	ErrInvalidDuration = errors.New("users: invalid ban duration")
	ErrEmptyReason     = errors.New("users: reason is required")
	ErrInvalidState    = errors.New("users: action not allowed in current state")
)
