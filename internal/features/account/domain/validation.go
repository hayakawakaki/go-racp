package domain

import (
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	usernameMaxLen = 23
	emailMaxLen    = 39
	passwordMaxLen = 32
)

var usernameCharset = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func ValidateUsername(s string) error {
	if s == "" {
		return ErrUsernameEmpty
	}

	if utf8.RuneCountInString(s) > usernameMaxLen {
		return ErrUsernameTooLong
	}

	if !usernameCharset.MatchString(s) {
		return ErrUsernameBadCharset
	}

	return nil
}

func ValidateEmail(s string) (string, error) {
	if s == "" {
		return "", ErrEmailEmpty
	}

	lowered := strings.ToLower(s)
	if utf8.RuneCountInString(lowered) > emailMaxLen {
		return "", ErrEmailTooLong
	}

	if _, err := mail.ParseAddress(lowered); err != nil {
		return "", ErrEmailShape
	}

	return lowered, nil
}

func ValidatePassword(s string) error {
	if s == "" {
		return ErrPasswordEmpty
	}

	if utf8.RuneCountInString(s) > passwordMaxLen {
		return ErrPasswordTooLong
	}

	return nil
}

func ValidateGender(s string) error {
	if s != "M" && s != "F" {
		return ErrGenderInvalid
	}
	return nil
}

const BirthdateMaxAgeYears = 120

func ValidateBirthdate(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, ErrBirthdateEmpty
	}

	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, ErrBirthdateShape
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if parsed.After(today) {
		return time.Time{}, ErrBirthdateFuture
	}

	if parsed.Before(today.AddDate(-BirthdateMaxAgeYears, 0, 0)) {
		return time.Time{}, ErrBirthdateTooOld
	}

	return parsed, nil
}
