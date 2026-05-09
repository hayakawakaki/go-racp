package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	regUsernameMinLen = 6
	regPasswordMinLen = 8

	//nolint:gosec // G101: charset of allowed special characters, not a credential.
	passwordSpecials = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
)

func CheckRegistrationUsername(username string) error {
	if utf8.RuneCountInString(username) < regUsernameMinLen {
		return ErrRegUsernameTooShort
	}
	return nil
}

func CheckRegistrationPassword(password string) error {
	if utf8.RuneCountInString(password) < regPasswordMinLen {
		return ErrRegPasswordTooShort
	}
	var hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case strings.ContainsRune(passwordSpecials, r):
			hasSymbol = true
		}
	}
	if !hasUpper {
		return ErrRegPasswordNoUpper
	}
	if !hasDigit {
		return ErrRegPasswordNoDigit
	}
	if !hasSymbol {
		return ErrRegPasswordNoSymbol
	}
	return nil
}
