package domain

import "strings"

const (
	MaxSubjectLen = 150
	MaxBodyLen    = 5000
	PageSize      = 20
)

func ValidateSubject(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrSubjectEmpty
	}
	if len(trimmed) > MaxSubjectLen {
		return "", ErrSubjectTooLong
	}

	return trimmed, nil
}

func ValidateBody(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrBodyEmpty
	}
	if len(trimmed) > MaxBodyLen {
		return "", ErrBodyTooLong
	}

	return trimmed, nil
}
