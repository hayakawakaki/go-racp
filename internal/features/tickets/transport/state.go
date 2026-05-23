package transport

import (
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/tickets/app"
	"github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
)

type PlayerDetailState struct {
	Categories []domain.Category
	Detail     app.TicketDetailDTO
}

type PlayerListState struct {
	Items []app.TicketListItem
	Page  int
	Total int
}

type PlayerNewState struct {
	Category   string
	Subject    string
	Body       string
	FormError  string
	Errors     domain.FieldErrors
	Categories []domain.Category
}

type StaffDetailState struct {
	Categories []domain.Category
	Detail     app.TicketDetailDTO
}

type StaffListState struct {
	Items        []app.TicketListItem
	PollInterval time.Duration
	Tab          domain.StaffTab
	Page         int
	Total        int
}
