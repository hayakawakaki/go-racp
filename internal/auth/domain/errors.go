package domain

import "errors"

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameConflict = errors.New("username already taken")
	ErrEmailConflict    = errors.New("email already in use")
)
