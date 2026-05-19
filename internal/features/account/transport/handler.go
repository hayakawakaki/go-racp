package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	actiontokendomain "github.com/hayakawakaki/go-racp/internal/actiontoken/domain"
	charapp "github.com/hayakawakaki/go-racp/internal/character/app"
	"github.com/hayakawakaki/go-racp/internal/features/account/app"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	maxRegisterFormBytes = 4 << 10
	maxLoginFormBytes    = 2 << 10
	maxAccountFormBytes  = 4 << 10

	genericErrorMessage = "Something went wrong. Please try again."
	invalidFormDataMsg  = "Invalid form data."

	fieldUsername           = "username"
	fieldEmail              = "email"
	fieldPassword           = "password"
	fieldPasswordConfirm    = "password_confirm"
	fieldGender             = "gender"
	fieldBirthdate          = "birthdate"
	fieldToken              = "token"
	fieldCurrentPassword    = "current_password"
	fieldNewPassword        = "new_password"
	fieldNewPasswordConfirm = "new_password_confirm"
	fieldNewEmail           = "new_email"
)

type accountService interface {
	Now() time.Time
	Create(ctx context.Context, cmd app.CreateCommand) (*app.GetDTO, error)
	Authenticate(ctx context.Context, cmd app.LoginCommand) (*app.GetDTO, app.Tier, error)
	GetAccount(ctx context.Context, userID int) (*app.AccountDTO, error)
	IssueVerification(ctx context.Context, accountID int, email, username string) error
	ConsumeVerification(ctx context.Context, rawToken string) error
	ResendVerification(ctx context.Context, accountID int) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConsumePasswordReset(ctx context.Context, rawToken, newPassword string) error
	Peek(ctx context.Context, kind actiontokendomain.Action, rawToken string) (*actiontokendomain.ActionToken, error)
	UpdatePassword(ctx context.Context, userID int, currentRawToken, currentPassword, newPassword, confirmPassword string) error
	RequestEmailChange(ctx context.Context, userID int, currentPassword, newEmail string) error
	ConsumeEmailChange(ctx context.Context, rawToken string) (*domain.User, error)
}

type sessionService interface {
	Create(ctx context.Context, userID int) (string, *domain.Session, error)
	Validate(ctx context.Context, rawToken string) (*domain.Session, error)
	Destroy(ctx context.Context, rawToken string) error
	TTL() time.Duration
}

type characterLister interface {
	List(ctx context.Context, accountID int) ([]charapp.CharacterDTO, error)
}

type HandlerConfig struct {
	Logger               *slog.Logger
	Users                userLookup
	Characters           characterLister
	General              config.GeneralConfig
	Secure               bool
	AllowTempBannedLogin bool
}

type Handler struct {
	svc                  accountService
	sessSvc              sessionService
	users                userLookup
	characters           characterLister
	logger               *slog.Logger
	general              config.GeneralConfig
	secure               bool
	allowTempBannedLogin bool
}

func NewHandler(svc accountService, sessSvc sessionService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		svc:                  svc,
		sessSvc:              sessSvc,
		users:                cfg.Users,
		characters:           cfg.Characters,
		logger:               logger,
		general:              cfg.General,
		secure:               cfg.Secure,
		allowTempBannedLogin: cfg.AllowTempBannedLogin,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Account.Register", "GET /register", http.HandlerFunc(h.showRegister))
	reg.Wrap(mux, "Account.Register", "POST /register", http.HandlerFunc(h.doRegister))
	reg.Wrap(mux, "Account.Login", "GET /login", http.HandlerFunc(h.showLogin))
	reg.Wrap(mux, "Account.Login", "POST /login", http.HandlerFunc(h.doLogin))
	reg.Wrap(mux, "Account.Logout", "POST /logout", http.HandlerFunc(h.doLogout))
	reg.Wrap(mux, "Account.ForgotPassword", "GET /forgot-password", http.HandlerFunc(h.showForgotPassword))
	reg.Wrap(mux, "Account.ForgotPassword", "POST /forgot-password", http.HandlerFunc(h.doForgotPassword))
	reg.Wrap(mux, "Account.ResetPassword", "GET /reset-password", http.HandlerFunc(h.showResetPassword))
	reg.Wrap(mux, "Account.ResetPassword", "POST /reset-password", http.HandlerFunc(h.doResetPassword))
	reg.Wrap(mux, "Account.Verify", "GET /verify-account", http.HandlerFunc(h.showVerifyAccount))
	reg.Wrap(mux, "Account.Verify", "GET /verify", http.HandlerFunc(h.showVerify))
	reg.Wrap(mux, "Account.Verify", "POST /verify", http.HandlerFunc(h.doVerify))
	reg.Wrap(mux, "Account.Verify", "POST /verify/resend", http.HandlerFunc(h.doResendVerification))
	reg.Wrap(mux, "Account.VerifyEmailChange", "GET /verify-email-change", http.HandlerFunc(h.showVerifyEmailChange))
	reg.Wrap(mux, "Account.VerifyEmailChange", "POST /verify-email-change", http.HandlerFunc(h.doVerifyEmailChange))

	reg.Wrap(mux, "Account.View", "GET /account", http.HandlerFunc(h.showAccount))
	reg.Wrap(mux, "Account.ChangePassword", "GET /account/password", http.HandlerFunc(h.showChangePassword))
	reg.Wrap(mux, "Account.ChangePassword", "POST /account/password", http.HandlerFunc(h.doChangePassword))
	reg.Wrap(mux, "Account.ChangeEmail", "GET /account/email", http.HandlerFunc(h.showChangeEmail))
	reg.Wrap(mux, "Account.ChangeEmail", "POST /account/email", http.HandlerFunc(h.doChangeEmail))
}

func (h *Handler) birthdateBounds() (minDate, maxDate string) {
	today := h.svc.Now().In(h.general.Location())
	maxDate = today.Format("2006-01-02")
	minDate = today.AddDate(-domain.BirthdateMaxAgeYears, 0, 0).Format("2006-01-02")

	return minDate, maxDate
}

func (h *Handler) showRegister(w http.ResponseWriter, r *http.Request) {
	if h.hasActiveSession(r) {
		httpx.Redirect(w, r, "/")
		return
	}

	minDate, maxDate := h.birthdateBounds()
	httpx.RenderHTML(w, r, h.logger, registerPage(h.layout(), RegisterFormState{
		BirthdateMin: minDate,
		BirthdateMax: maxDate,
	}))
}

func (h *Handler) doRegister(w http.ResponseWriter, r *http.Request) {
	minDate, maxDate := h.birthdateBounds()

	if err := httpx.ParseForm(w, r, maxRegisterFormBytes); err != nil {
		h.renderRegister(w, r, RegisterFormState{
			FormError:    invalidFormDataMsg,
			BirthdateMin: minDate,
			BirthdateMax: maxDate,
		})
		return
	}

	cmd := app.CreateCommand{
		Username:        r.PostFormValue(fieldUsername),
		Email:           r.PostFormValue(fieldEmail),
		Password:        r.PostFormValue(fieldPassword),
		PasswordConfirm: r.PostFormValue(fieldPasswordConfirm),
		Gender:          r.PostFormValue(fieldGender),
		Birthdate:       r.PostFormValue(fieldBirthdate),
	}

	_, err := h.svc.Create(r.Context(), cmd)
	if err != nil {
		state := RegisterFormState{
			Username:     cmd.Username,
			Email:        cmd.Email,
			Gender:       cmd.Gender,
			Birthdate:    cmd.Birthdate,
			BirthdateMin: minDate,
			BirthdateMax: maxDate,
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			state.Errors = ve.Fields
		} else {
			h.logger.Error("register", "err", err)
			state.FormError = genericErrorMessage
		}
		h.renderRegister(w, r, state)
		return
	}

	httpx.Redirect(w, r, "/login")
}

func (h *Handler) renderRegister(w http.ResponseWriter, r *http.Request, state RegisterFormState) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, registerForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, registerPage(h.layout(), state))
}

var loginNoticeText = map[string]string{
	middleware.NoticeBanned:  "You were signed out. This account is permanently banned.",
	middleware.NoticeDeleted: "You were signed out. This account no longer exists.",
}

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	if h.hasActiveSession(r) {
		httpx.Redirect(w, r, "/")
		return
	}

	state := LoginFormState{}
	if notice, ok := loginNoticeText[r.URL.Query().Get("notice")]; ok {
		state.Notice = notice
	}

	httpx.RenderHTML(w, r, h.logger, loginPage(h.layout(), state))
}

func (h *Handler) doLogin(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseForm(w, r, maxLoginFormBytes); err != nil {
		h.renderLogin(w, r, LoginFormState{Error: invalidFormDataMsg})
		return
	}

	cmd := app.LoginCommand{
		Username: r.PostFormValue(fieldUsername),
		Password: r.PostFormValue(fieldPassword),
	}

	user, tier, err := h.svc.Authenticate(r.Context(), cmd)
	if err != nil {
		state := LoginFormState{Username: cmd.Username}
		if errors.Is(err, domain.ErrInvalidCredentials) {
			state.Error = "Invalid username or password."
		} else {
			h.logger.Error("login", "err", err)
			state.Error = genericErrorMessage
		}
		h.renderLogin(w, r, state)
		return
	}

	if tier == app.TierPermaBanned {
		h.renderLogin(w, r, LoginFormState{
			Username: cmd.Username,
			Error:    "This account is permanently banned.",
		})
		return
	}

	if tier == app.TierTempBanned && !h.allowTempBannedLogin {
		h.renderLogin(w, r, LoginFormState{
			Username: cmd.Username,
			Error:    "This account is currently restricted. Please try again later.",
		})
		return
	}

	token, _, err := h.sessSvc.Create(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("session create", "err", err)
		h.renderLogin(w, r, LoginFormState{
			Username: cmd.Username,
			Error:    genericErrorMessage,
		})
		return
	}
	setSessionCookie(w, token, h.sessSvc.TTL(), h.secure)

	if tier == app.TierUnverified {
		httpx.Redirect(w, r, "/verify-account")
		return
	}

	httpx.Redirect(w, r, "/")
}

func (h *Handler) renderLogin(w http.ResponseWriter, r *http.Request, state LoginFormState) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, loginForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, loginPage(h.layout(), state))
}

func (h *Handler) doLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(middleware.SessionCookieName); err == nil {
		_ = h.sessSvc.Destroy(r.Context(), c.Value)
	}
	middleware.ClearSessionCookie(w, h.secure)

	httpx.Redirect(w, r, "/login")
}
