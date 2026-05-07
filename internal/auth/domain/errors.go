// Package domain defines the core user entity, its repository interface, and
// domain-level sentinel errors for the auth feature.
package domain

import "errors"

var (
	// ErrUserNotFound is returned when a requested user does not exist in the
	// repository.
	ErrUserNotFound = errors.New("user not found")

	// ErrUsernameConflict is returned when a username is already registered.
	ErrUsernameConflict = errors.New("username already taken")

	// ErrEmailConflict is returned when an email address is already registered.
	ErrEmailConflict = errors.New("email already in use")
)
