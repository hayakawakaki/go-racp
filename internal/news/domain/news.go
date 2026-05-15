package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxTitleLen = 200
	MaxBodyLen  = 50 * 1024
)

type News struct {
	CreatedAt time.Time
	Title     string
	Body      string
	Category  string
	ID        int64
}

func ValidateTitle(s string) error {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ErrTitleEmpty
	}
	if utf8.RuneCountInString(trimmed) > MaxTitleLen {
		return ErrTitleTooLong
	}

	return nil
}

func ValidateBody(s string) error {
	if strings.TrimSpace(s) == "" {
		return ErrBodyEmpty
	}
	if len(s) > MaxBodyLen {
		return ErrBodyTooLong
	}

	return nil
}
