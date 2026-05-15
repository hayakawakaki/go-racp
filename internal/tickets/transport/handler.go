package transport

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	accountdomain "github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/routes"
	ticketsapp "github.com/hayakawakaki/go-racp/internal/tickets/app"
	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	maxOpenFormBytes    = 8 << 10
	maxReplyFormBytes   = 8 << 10
	maxNoteFormBytes    = 8 << 10
	maxSubjectFormBytes = 1 << 10
)

const (
	fieldCategory = "category"
	fieldSubject  = "subject"
	fieldBody     = "body"
)

type ticketService interface {
	Now() time.Time
	OpenTicket(ctx context.Context, accountID int, category, subject, body string) (int64, error)
	PlayerReply(ctx context.Context, accountID int, ticketID int64, body string) error
	StaffReply(ctx context.Context, staffID int, ticketID int64, body string) error
	StaffNote(ctx context.Context, staffID int, ticketID int64, body string) error
	StaffRecategorize(ctx context.Context, staffID int, ticketID int64, newCategory string) error
	StaffEditSubject(ctx context.Context, staffID int, ticketID int64, newSubject string) error
	StaffResolve(ctx context.Context, staffID int, ticketID int64) error
	StaffClose(ctx context.Context, staffID int, ticketID int64) error
	GetTicketForPlayer(ctx context.Context, accountID int, ticketID int64) (ticketsapp.TicketDetailDTO, error)
	GetTicketForStaff(ctx context.Context, ticketID int64) (ticketsapp.TicketDetailDTO, error)
	ListForPlayer(ctx context.Context, accountID, offset, limit int) ([]ticketsapp.TicketListItem, int, error)
	ListForStaff(ctx context.Context, staffID int, tab domain.StaffTab, categoryKeys []string, offset, limit int) ([]ticketsapp.TicketListItem, int, error)
	MarkViewed(ctx context.Context, accountID int, ticketID int64)
	UnreadCountForPlayer(ctx context.Context, accountID int) int
	UnreadCountForStaff(ctx context.Context, accountID int, categoryKeys []string) int
	Categories() domain.CategoryResolver
}

type userLookup interface {
	GetByID(ctx context.Context, id int) (*accountdomain.User, error)
}

type HandlerConfig struct {
	Logger       *slog.Logger
	Users        userLookup
	Roles        accountdomain.RoleResolver
	General      config.GeneralConfig
	PollInterval time.Duration
}

type Handler struct {
	svc     ticketService
	users   userLookup
	logger  *slog.Logger
	roles   accountdomain.RoleResolver
	general config.GeneralConfig
	poll    time.Duration
}

func NewHandler(service ticketService, cfg HandlerConfig) *Handler {
	return &Handler{
		svc:     service,
		users:   cfg.Users,
		logger:  cfg.Logger,
		general: cfg.General,
		roles:   cfg.Roles,
		poll:    cfg.PollInterval,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) currentUser(r *http.Request) (*accountdomain.User, accountdomain.Role, bool) {
	session, ok := middleware.SessionFromContext(r.Context())
	if !ok {
		return nil, accountdomain.Role{}, false
	}
	user, err := h.users.GetByID(r.Context(), session.UserID)
	if err != nil {
		h.logger.Error("tickets: load user", "err", err, "userID", session.UserID)
		return nil, accountdomain.Role{}, false
	}

	return user, h.roles.Resolve(user.GroupID), true
}

func (h *Handler) categoryAllowed(role accountdomain.Role, categoryKey string) bool {
	return h.svc.Categories().Permits(categoryKey, role.Name, role == accountdomain.RoleAdmin)
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Tickets.View", "GET /tickets", http.HandlerFunc(h.playerList))
	reg.Wrap(mux, "Tickets.Open", "GET /tickets/new", http.HandlerFunc(h.playerNewForm))
	reg.Wrap(mux, "Tickets.Open", "POST /tickets/new", http.HandlerFunc(h.playerCreate))
	reg.Wrap(mux, "Tickets.View", "GET /tickets/{id}", http.HandlerFunc(h.playerDetail))
	reg.Wrap(mux, "Tickets.Reply", "POST /tickets/{id}/reply", http.HandlerFunc(h.playerReply))

	reg.Wrap(mux, "Tickets.StaffAccess", "GET /admin/tickets", http.HandlerFunc(h.staffList))
	reg.Wrap(mux, "Tickets.StaffAccess", "GET /admin/tickets/{id}", http.HandlerFunc(h.staffDetail))
	reg.Wrap(mux, "Tickets.StaffAccess", "POST /admin/tickets/{id}/reply", http.HandlerFunc(h.staffReply))
	reg.Wrap(mux, "Tickets.StaffAccess", "POST /admin/tickets/{id}/note", http.HandlerFunc(h.staffNote))
	reg.Wrap(mux, "Tickets.StaffAccess", "POST /admin/tickets/{id}/resolve", http.HandlerFunc(h.staffResolve))
	reg.Wrap(mux, "Tickets.StaffAccess", "POST /admin/tickets/{id}/close", http.HandlerFunc(h.staffClose))
	reg.Wrap(mux, "Tickets.StaffAccess", "POST /admin/tickets/{id}/category", http.HandlerFunc(h.staffRecategorize))
	reg.Wrap(mux, "Tickets.StaffAccess", "POST /admin/tickets/{id}/subject", http.HandlerFunc(h.staffEditSubject))
}
