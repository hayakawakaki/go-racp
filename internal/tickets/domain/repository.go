package domain

import (
	"context"
	"time"
)

type StaffTab int

const (
	TabOpenNoResponse StaffTab = iota
	TabActive
	TabTerminal
)

type Page struct {
	Limit  int
	Offset int
}

type Repository interface {
	Create(ctx context.Context, ticket Ticket, initial Message) (int64, error)
	Get(ctx context.Context, id int64) (Ticket, error)

	ListForPlayer(ctx context.Context, accountID int, page Page) ([]Ticket, int, error)
	ListForStaff(ctx context.Context, tab StaffTab, categoryKeys []string, page Page) ([]Ticket, int, error)

	CountOpenForPlayer(ctx context.Context, accountID int) (int, error)
	MostRecentOpenedAt(ctx context.Context, accountID int) (time.Time, error)

	AppendPublicMessage(ctx context.Context, ticketID int64, message Message) (Ticket, error)
	AppendInternalNote(ctx context.Context, ticketID int64, message Message) error
	AppendSystemEvent(ctx context.Context, ticketID int64, updated Ticket, message Message) error

	SetTerminal(ctx context.Context, ticketID int64, status Status, staffID int, at time.Time) (Ticket, Message, error)
}

type MessageRepository interface {
	List(ctx context.Context, ticketID int64, includeInternal bool) ([]Message, error)
}

type ViewRepository interface {
	Get(ctx context.Context, accountID int, ticketID int64) (time.Time, error)
	Upsert(ctx context.Context, accountID int, ticketID int64, at time.Time) error
	UnreadCountForPlayer(ctx context.Context, accountID int) (int, error)
	UnreadCountForStaff(ctx context.Context, accountID int, categoryKeys []string) (int, error)
	OtherSeenAt(ctx context.Context, ticketID int64, ownerID int, viewerIsStaff bool) (time.Time, error)
}
