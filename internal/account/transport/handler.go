package transport

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	maxAccountFormBytes = 4 << 10

	genericErrorMessage = "Something went wrong. Please try again."
	invalidFormDataMsg  = "Invalid form data."
	sessionCookieName   = "racp_session"

	fieldCurrentPassword    = "current_password"
	fieldNewPassword        = "new_password"
	fieldNewPasswordConfirm = "new_password_confirm"
	fieldNewEmail           = "new_email"
)

type accountService interface {
	GetAccount(ctx context.Context, userID int) (*accountapp.AccountDTO, error)
	UpdatePassword(ctx context.Context, userID int, currentRawToken, currentPassword, newPassword, confirmPassword string) error
	RequestEmailChange(ctx context.Context, userID int, currentPassword, newEmail string) error
	ConsumeEmailChange(ctx context.Context, rawToken string) (*authdomain.User, error)
}

type sessionService interface {
	Validate(ctx context.Context, rawToken string) (*authdomain.Session, error)
	TTL() time.Duration
}

type HandlerConfig struct {
	Logger  *slog.Logger
	General config.GeneralConfig
	Secure  bool
}

type Handler struct {
	svc     accountService
	sessSvc sessionService
	logger  *slog.Logger
	general config.GeneralConfig
	secure  bool
}

func NewHandler(svc accountService, sessSvc sessionService, cfg HandlerConfig) *Handler {
	return &Handler{
		svc:     svc,
		sessSvc: sessSvc,
		logger:  cfg.Logger,
		general: cfg.General,
		secure:  cfg.Secure,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireLogin func(http.Handler) http.Handler) {
	mux.Handle("GET /account", requireLogin(http.HandlerFunc(h.showAccount)))
	mux.Handle("GET /account/password", requireLogin(http.HandlerFunc(h.showChangePassword)))
	mux.Handle("POST /account/password", requireLogin(http.HandlerFunc(h.doChangePassword)))
	mux.Handle("GET /account/email", requireLogin(http.HandlerFunc(h.showChangeEmail)))
	mux.Handle("POST /account/email", requireLogin(http.HandlerFunc(h.doChangeEmail)))
	mux.HandleFunc("GET /verify-email-change", h.doVerifyEmailChange)
}
