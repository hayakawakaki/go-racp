package domain

import (
	"errors"
	"sort"
	"strings"
)

const (
	fieldSubject = "subject"
	fieldBody    = "body"
)

var (
	ErrTicketNotFound       = errors.New("tickets: ticket not found")
	ErrNotTicketOwner       = errors.New("tickets: not ticket owner")
	ErrTicketTerminal       = errors.New("tickets: ticket is closed or resolved")
	ErrPlayerCannotReply    = errors.New("tickets: player cannot reply now")
	ErrUnknownCategory      = errors.New("tickets: unknown category")
	ErrCategoryNotPermitted = errors.New("tickets: category not permitted for this staff role")
	ErrTooManyOpenTickets   = errors.New("tickets: too many open tickets")
	ErrTicketCooldown       = errors.New("tickets: cooldown active")

	ErrSubjectEmpty      = errors.New("subject is required")
	ErrSubjectTooLong    = errors.New("subject is too long")
	ErrSubjectUnchanged  = errors.New("subject is unchanged")
	ErrBodyEmpty         = errors.New("message body is required")
	ErrBodyTooLong       = errors.New("message body is too long")
	ErrCategoryUnchanged = errors.New("category is unchanged")
)

type FieldErrors map[string]string

func (fe FieldErrors) Has() bool { return len(fe) > 0 }

func (fe FieldErrors) Add(field, message string) { fe[field] = message }

type ValidationError struct {
	Fields FieldErrors
}

func (e *ValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "validation failed"
	}

	keys := make([]string, 0, len(e.Fields))
	for key := range e.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+e.Fields[key])
	}

	return "validation failed: " + strings.Join(parts, "; ")
}
