package domain

import (
	"errors"
	"sort"
	"strings"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")

	ErrAccountUnverified       = errors.New("auth: account not verified")
	ErrEmailTaken              = errors.New("auth: email already in use")
	ErrPasswordRecentlyChanged = errors.New("auth: password was recently changed")
	ErrEmailRecentlyChanged    = errors.New("auth: email was recently changed")

	ErrInvalidCurrentSessionToken = errors.New("auth: current session token undecodable")

	ErrUsernameEmpty      = errors.New("username is required")
	ErrUsernameTooLong    = errors.New("username must be at most 23 characters")
	ErrUsernameBadCharset = errors.New("username may only contain letters, digits, and underscores")

	ErrEmailEmpty   = errors.New("email is required")
	ErrEmailTooLong = errors.New("email must be at most 39 characters")
	ErrEmailShape   = errors.New("email is not a valid address")

	ErrPasswordEmpty   = errors.New("password is required")
	ErrPasswordTooLong = errors.New("password must be at most 32 characters")

	ErrGenderInvalid = errors.New("gender must be M or F")

	ErrBirthdateEmpty  = errors.New("birthdate is required")
	ErrBirthdateShape  = errors.New("birthdate must be a valid date")
	ErrBirthdateFuture = errors.New("birthdate cannot be in the future")
	ErrBirthdateTooOld = errors.New("birthdate is unrealistic")

	ErrRegUsernameTooShort = errors.New("username must be at least 6 characters")
	ErrRegPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrRegPasswordNoUpper  = errors.New("password must contain an uppercase letter")
	ErrRegPasswordNoDigit  = errors.New("password must contain a digit")
	ErrRegPasswordNoSymbol = errors.New("password must contain a special character")
)

type FieldErrors map[string]string

func (fe FieldErrors) Has() bool { return len(fe) > 0 }

func (fe FieldErrors) Add(field, msg string) { fe[field] = msg }

type ValidationError struct {
	Fields FieldErrors
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}

	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e.Fields[k])
	}

	return "validation failed: " + strings.Join(parts, "; ")
}
