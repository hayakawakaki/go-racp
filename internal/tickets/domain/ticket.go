package domain

import (
	"encoding/json"
	"time"
)

type Status string

const (
	StatusOpen     Status = "open"
	StatusResolved Status = "resolved"
	StatusClosed   Status = "closed"
)

type Actor string

const (
	ActorPlayer Actor = "player"
	ActorStaff  Actor = "staff"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityInternal Visibility = "internal"
	VisibilitySystem   Visibility = "system"
)

type Ticket struct {
	ClosedBy       *int
	LastActivity   time.Time
	CreatedAt      time.Time
	AuthorUsername string
	Category       string
	Subject        string
	Status         Status
	LastActor      Actor
	ID             int64
	AccountID      int
	MessageCount   int
}

type Message struct {
	CreatedAt  time.Time
	Body       string
	AuthorRole Actor
	Visibility Visibility
	Event      json.RawMessage
	ID         int64
	TicketID   int64
	AuthorID   int
}

func (t Ticket) IsTerminal() bool {
	return t.Status == StatusResolved || t.Status == StatusClosed
}

func (t Ticket) CanPlayerReply() bool {
	return !t.IsTerminal() && t.LastActor == ActorStaff
}

func (t Ticket) IsUnreadFor(lastViewed time.Time) bool {
	return t.LastActivity.After(lastViewed)
}
