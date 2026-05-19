package app

import (
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
)

type TicketListItem struct {
	LastActivity    time.Time
	AuthorUsername  string
	Category        string
	CategoryDisplay string
	Subject         string
	Status          domain.Status
	LastActor       domain.Actor
	ID              int64
	MessageCount    int
	Unread          bool
}

type TicketDetailDTO struct {
	OtherSeenAt time.Time
	Messages    []domain.Message
	Ticket      domain.Ticket
}
