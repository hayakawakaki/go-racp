package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	accountapp "github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/httpx"
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
	Create(ctx context.Context, cmd accountapp.CreateCommand) (*accountapp.GetDTO, error)
	Authenticate(ctx context.Context, cmd accountapp.LoginCommand) (*accountapp.GetDTO, error)
	GetAccount(ctx context.Context, userID int) (*accountapp.AccountDTO, error)
	IssueVerification(ctx context.Context, accountID int, email, username string) error
	ConsumeVerification(ctx context.Context, rawToken string) error
	ResendVerification(ctx context.Context, accountID int) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConsumePasswordReset(ctx context.Context, rawToken, newPassword string) error
	PeekPasswordReset(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error)
	PeekVerification(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error)
	PeekEmailChange(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error)
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

type HandlerConfig struct {
	Logger  *slog.Logger
	Users   userLookup
	General config.GeneralConfig
	Secure  bool
}

type Handler struct {
	svc     accountService
	sessSvc sessionService
	users   userLookup
	logger  *slog.Logger
	general config.GeneralConfig
	secure  bool
}

func NewHandler(svc accountService, sessSvc sessionService, cfg HandlerConfig) *Handler {
	return &Handler{
		svc:     svc,
		sessSvc: sessSvc,
		users:   cfg.Users,
		logger:  cfg.Logger,
		general: cfg.General,
		secure:  cfg.Secure,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireLogin func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /register", h.showRegister)
	mux.HandleFunc("POST /register", h.doRegister)
	mux.HandleFunc("GET /login", h.showLogin)
	mux.HandleFunc("POST /login", h.doLogin)
	mux.HandleFunc("POST /logout", h.doLogout)
	mux.HandleFunc("GET /forgot-password", h.showForgotPassword)
	mux.HandleFunc("POST /forgot-password", h.doForgotPassword)
	mux.HandleFunc("GET /reset-password", h.showResetPassword)
	mux.HandleFunc("POST /reset-password", h.doResetPassword)
	mux.HandleFunc("GET /verify-account", h.showVerifyAccount)
	mux.HandleFunc("GET /verify", h.showVerify)
	mux.HandleFunc("POST /verify", h.doVerify)
	mux.HandleFunc("POST /verify/resend", h.doResendVerification)
	mux.HandleFunc("GET /verify-email-change", h.showVerifyEmailChange)
	mux.HandleFunc("POST /verify-email-change", h.doVerifyEmailChange)

	mux.Handle("GET /account", requireLogin(http.HandlerFunc(h.showAccount)))
	mux.Handle("GET /account/password", requireLogin(http.HandlerFunc(h.showChangePassword)))
	mux.Handle("POST /account/password", requireLogin(http.HandlerFunc(h.doChangePassword)))
	mux.Handle("GET /account/email", requireLogin(http.HandlerFunc(h.showChangeEmail)))
	mux.Handle("POST /account/email", requireLogin(http.HandlerFunc(h.doChangeEmail)))
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
	r.Body = http.MaxBytesReader(w, r.Body, maxRegisterFormBytes)
	minDate, maxDate := h.birthdateBounds()

	if err := r.ParseForm(); err != nil {
		h.renderRegister(w, r, RegisterFormState{
			FormError:    invalidFormDataMsg,
			BirthdateMin: minDate,
			BirthdateMax: maxDate,
		})
		return
	}

	cmd := accountapp.CreateCommand{
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

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	if h.hasActiveSession(r) {
		httpx.Redirect(w, r, "/")
		return
	}

	httpx.RenderHTML(w, r, h.logger, loginPage(h.layout(), LoginFormState{}))
}

func (h *Handler) doLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, r, LoginFormState{Error: invalidFormDataMsg})
		return
	}

	cmd := accountapp.LoginCommand{
		Username: r.PostFormValue(fieldUsername),
		Password: r.PostFormValue(fieldPassword),
	}

	user, err := h.svc.Authenticate(r.Context(), cmd)
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
	if c, err := r.Cookie(sessionCookieName); err == nil {
		_ = h.sessSvc.Destroy(r.Context(), c.Value)
	}
	clearSessionCookie(w, h.secure)

	httpx.Redirect(w, r, "/login")
}
