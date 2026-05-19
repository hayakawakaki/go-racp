package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	domain2 "github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
)

type Mailer interface {
	SendAsync(to, subject, body string)
}

type UserLookup interface {
	GetByID(ctx context.Context, id int) (*accdomain.User, error)
}

type Config struct {
	AppURL             string
	ServerName         string
	MaxOpenPerPlayer   int
	TicketOpenCooldown time.Duration
}

type Service struct {
	tickets    domain2.Repository
	messages   domain2.MessageRepository
	views      domain2.ViewRepository
	users      UserLookup
	mailer     Mailer
	logger     *slog.Logger
	now        func() time.Time
	categories domain2.CategoryResolver
	config     Config
}

func NewService(
	tickets domain2.Repository,
	messages domain2.MessageRepository,
	views domain2.ViewRepository,
	categories domain2.CategoryResolver,
	users UserLookup,
	mailer Mailer,
	logger *slog.Logger,
	config Config,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		tickets:    tickets,
		messages:   messages,
		views:      views,
		categories: categories,
		users:      users,
		mailer:     mailer,
		logger:     logger,
		config:     config,
		now:        time.Now,
	}
}

func (s *Service) Now() time.Time { return s.now() }

func (s *Service) Categories() domain2.CategoryResolver { return s.categories }

func (s *Service) categoryDisplay(key string) string {
	if category, ok := s.categories.Get(key); ok {
		return category.Display
	}

	return key
}

// OpenTicket creates a new ticket after enforcing per-player open limits and cooldown.
func (s *Service) OpenTicket(ctx context.Context, accountID int, category, subject, body string) (int64, error) {
	if err := s.checkOpenGate(ctx, accountID, category); err != nil {
		return 0, err
	}

	user, err := s.users.GetByID(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("app.Service.OpenTicket: %w: %w", ErrPlayerLookupFailed, err)
	}

	ticket, message, err := domain2.NewTicket(accountID, user.Username, category, subject, body, s.now())
	if err != nil {
		return 0, fmt.Errorf("app.Service.OpenTicket: %w", err)
	}

	id, err := s.tickets.Create(ctx, ticket, message)
	if err != nil {
		return 0, fmt.Errorf("app.Service.OpenTicket: %w", err)
	}

	return id, nil
}

func (s *Service) checkOpenGate(ctx context.Context, accountID int, category string) error {
	if _, ok := s.categories.Get(category); !ok {
		return domain2.ErrUnknownCategory
	}

	openCount, err := s.tickets.CountOpenForPlayer(ctx, accountID)
	if err != nil {
		return fmt.Errorf("app.Service.checkOpenGate: %w", err)
	}
	if openCount >= s.config.MaxOpenPerPlayer {
		return domain2.ErrTooManyOpenTickets
	}

	if s.config.TicketOpenCooldown > 0 {
		last, lastErr := s.tickets.MostRecentOpenedAt(ctx, accountID)
		if lastErr != nil {
			return fmt.Errorf("app.Service.checkOpenGate: %w", lastErr)
		}
		if !last.IsZero() && s.now().Sub(last) < s.config.TicketOpenCooldown {
			return domain2.ErrTicketCooldown
		}
	}

	return nil
}

func (s *Service) PlayerReply(ctx context.Context, accountID int, ticketID int64, body string) error {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("app.Service.PlayerReply: %w", err)
	}
	if ticket.AccountID != accountID {
		return domain2.ErrNotTicketOwner
	}

	_, message, err := ticket.AppendPublic(accountID, domain2.ActorPlayer, body, s.now())
	if err != nil {
		return fmt.Errorf("app.Service.PlayerReply: %w", err)
	}

	if _, err := s.tickets.AppendPublicMessage(ctx, ticketID, message); err != nil {
		return fmt.Errorf("app.Service.PlayerReply: %w", err)
	}

	return nil
}

// StaffReply appends a public staff reply and emails the ticket owner.
func (s *Service) StaffReply(ctx context.Context, staffID int, ticketID int64, body string) error {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("app.Service.StaffReply: %w", err)
	}

	_, message, err := ticket.AppendPublic(staffID, domain2.ActorStaff, body, s.now())
	if err != nil {
		return fmt.Errorf("app.Service.StaffReply: %w", err)
	}

	updated, err := s.tickets.AppendPublicMessage(ctx, ticketID, message)
	if err != nil {
		return fmt.Errorf("app.Service.StaffReply: %w", err)
	}
	s.notifyStaffReply(ctx, updated)

	return nil
}

func (s *Service) StaffNote(ctx context.Context, staffID int, ticketID int64, body string) error {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("app.Service.StaffNote: %w", err)
	}
	message, err := ticket.AppendInternalNote(staffID, body, s.now())
	if err != nil {
		return fmt.Errorf("app.Service.StaffNote: %w", err)
	}
	if err := s.tickets.AppendInternalNote(ctx, ticketID, message); err != nil {
		return fmt.Errorf("app.Service.StaffNote: %w", err)
	}

	return nil
}

func (s *Service) StaffRecategorize(ctx context.Context, staffID int, ticketID int64, newCategory string) error {
	if _, ok := s.categories.Get(newCategory); !ok {
		return domain2.ErrUnknownCategory
	}
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("app.Service.StaffRecategorize: %w", err)
	}
	updated, message, err := ticket.Recategorize(staffID, newCategory, s.now())
	if err != nil {
		return fmt.Errorf("app.Service.StaffRecategorize: %w", err)
	}
	if err := s.tickets.AppendSystemEvent(ctx, ticketID, updated, message); err != nil {
		return fmt.Errorf("app.Service.StaffRecategorize: %w", err)
	}

	return nil
}

func (s *Service) StaffEditSubject(ctx context.Context, staffID int, ticketID int64, newSubject string) error {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("app.Service.StaffEditSubject: %w", err)
	}
	updated, message, err := ticket.EditSubject(staffID, newSubject, s.now())
	if err != nil {
		return fmt.Errorf("app.Service.StaffEditSubject: %w", err)
	}
	if err := s.tickets.AppendSystemEvent(ctx, ticketID, updated, message); err != nil {
		return fmt.Errorf("app.Service.StaffEditSubject: %w", err)
	}

	return nil
}

// StaffResolve marks the ticket resolved and emails the owner.
func (s *Service) StaffResolve(ctx context.Context, staffID int, ticketID int64) error {
	return s.setTerminal(ctx, staffID, ticketID, domain2.StatusResolved)
}

// StaffClose marks the ticket closed and emails the owner.
func (s *Service) StaffClose(ctx context.Context, staffID int, ticketID int64) error {
	return s.setTerminal(ctx, staffID, ticketID, domain2.StatusClosed)
}

func (s *Service) setTerminal(ctx context.Context, staffID int, ticketID int64, status domain2.Status) error {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("app.Service.setTerminal: %w", err)
	}
	if ticket.IsTerminal() {
		return domain2.ErrTicketTerminal
	}
	updated, _, err := s.tickets.SetTerminal(ctx, ticketID, status, staffID, s.now())
	if err != nil {
		return fmt.Errorf("app.Service.setTerminal: %w", err)
	}
	s.notifyTerminal(ctx, updated)

	return nil
}

func (s *Service) GetTicketForPlayer(ctx context.Context, accountID int, ticketID int64) (TicketDetailDTO, error) {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return TicketDetailDTO{}, fmt.Errorf("app.Service.GetTicketForPlayer: %w", err)
	}
	if ticket.AccountID != accountID {
		return TicketDetailDTO{}, domain2.ErrNotTicketOwner
	}
	messages, err := s.messages.List(ctx, ticketID, false)
	if err != nil {
		return TicketDetailDTO{}, fmt.Errorf("app.Service.GetTicketForPlayer: %w", err)
	}
	otherSeen, err := s.views.OtherSeenAt(ctx, ticketID, ticket.AccountID, false)
	if err != nil {
		s.logger.Error("tickets.GetTicketForPlayer: other-seen", "err", err)
	}

	return TicketDetailDTO{Ticket: ticket, Messages: messages, OtherSeenAt: otherSeen}, nil
}

func (s *Service) GetTicketForStaff(ctx context.Context, ticketID int64) (TicketDetailDTO, error) {
	ticket, err := s.tickets.Get(ctx, ticketID)
	if err != nil {
		return TicketDetailDTO{}, fmt.Errorf("app.Service.GetTicketForStaff: %w", err)
	}
	messages, err := s.messages.List(ctx, ticketID, true)
	if err != nil {
		return TicketDetailDTO{}, fmt.Errorf("app.Service.GetTicketForStaff: %w", err)
	}
	otherSeen, err := s.views.OtherSeenAt(ctx, ticketID, ticket.AccountID, true)
	if err != nil {
		s.logger.Error("tickets.GetTicketForStaff: other-seen", "err", err)
	}

	return TicketDetailDTO{Ticket: ticket, Messages: messages, OtherSeenAt: otherSeen}, nil
}

func (s *Service) ListForPlayer(ctx context.Context, accountID, offset, limit int) ([]TicketListItem, int, error) {
	tickets, total, err := s.tickets.ListForPlayer(ctx, accountID, domain2.Page{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("app.Service.ListForPlayer: %w", err)
	}

	return s.toListItems(ctx, accountID, tickets), total, nil
}

func (s *Service) ListForStaff(
	ctx context.Context,
	staffID int,
	tab domain2.StaffTab,
	categoryKeys []string,
	offset, limit int,
) ([]TicketListItem, int, error) {
	tickets, total, err := s.tickets.ListForStaff(ctx, tab, categoryKeys, domain2.Page{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, fmt.Errorf("app.Service.ListForStaff: %w", err)
	}

	return s.toListItems(ctx, staffID, tickets), total, nil
}

func (s *Service) toListItems(ctx context.Context, viewerID int, tickets []domain2.Ticket) []TicketListItem {
	out := make([]TicketListItem, 0, len(tickets))
	for _, ticket := range tickets {
		lastViewed, err := s.views.Get(ctx, viewerID, ticket.ID)
		if err != nil {
			s.logger.Error("tickets.toListItems: view lookup", "err", err, "ticket", ticket.ID)
		}
		out = append(out, TicketListItem{
			ID:              ticket.ID,
			AuthorUsername:  ticket.AuthorUsername,
			Category:        ticket.Category,
			CategoryDisplay: s.categoryDisplay(ticket.Category),
			Subject:         ticket.Subject,
			Status:          ticket.Status,
			LastActor:       ticket.LastActor,
			MessageCount:    ticket.MessageCount,
			LastActivity:    ticket.LastActivity,
			Unread:          ticket.IsUnreadFor(lastViewed),
		})
	}

	return out
}

func (s *Service) MarkViewed(ctx context.Context, accountID int, ticketID int64) {
	if err := s.views.Upsert(ctx, accountID, ticketID, s.now()); err != nil {
		s.logger.Error("tickets.MarkViewed", "err", err, "accountID", accountID, "ticketID", ticketID)
	}
}

func (s *Service) UnreadCountForPlayer(ctx context.Context, accountID int) int {
	count, err := s.views.UnreadCountForPlayer(ctx, accountID)
	if err != nil {
		s.logger.Error("tickets.UnreadCountForPlayer", "err", err)
		return 0
	}

	return count
}

func (s *Service) UnreadCountForStaff(ctx context.Context, accountID int, categoryKeys []string) int {
	count, err := s.views.UnreadCountForStaff(ctx, accountID, categoryKeys)
	if err != nil {
		s.logger.Error("tickets.UnreadCountForStaff", "err", err)
		return 0
	}

	return count
}
