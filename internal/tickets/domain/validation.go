package domain

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxSubjectLen = 150
	MaxBodyLen    = 1000
	PageSize      = 20
)

func ValidateSubject(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrSubjectEmpty
	}
	if utf8.RuneCountInString(trimmed) > MaxSubjectLen {
		return "", ErrSubjectTooLong
	}

	return trimmed, nil
}

func ValidateBody(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrBodyEmpty
	}
	if utf8.RuneCountInString(trimmed) > MaxBodyLen {
		return "", ErrBodyTooLong
	}

	return trimmed, nil
}
