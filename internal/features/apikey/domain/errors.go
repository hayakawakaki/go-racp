package domain

import (
	"errors"
	"sort"
	"strings"
)

var (
	ErrKeyNotFound  = errors.New("apikey: key not found")
	ErrKeyRevoked   = errors.New("apikey: key has been revoked")
	ErrNameRequired = errors.New("apikey: name is required")
	ErrUnknownTier  = errors.New("apikey: unknown rate tier")
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
