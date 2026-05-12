package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	maxRegisterFormBytes = 4 << 10
	maxLoginFormBytes    = 2 << 10

	genericErrorMessage = "Something went wrong. Please try again."
	invalidFormDataMsg  = "Invalid form data."
)

type authService interface {
	Create(ctx context.Context, cmd app.CreateCommand) (*app.GetDTO, error)
	Authenticate(ctx context.Context, cmd app.LoginCommand) (*app.GetDTO, error)
}

type sessionService interface {
	Create(ctx context.Context, userID int) (string, *domain.Session, error)
	Validate(ctx context.Context, rawToken string) (*domain.Session, error)
	Destroy(ctx context.Context, rawToken string) error
	TTL() time.Duration
}

type passwordResetService interface {
	RequestPasswordReset(ctx context.Context, email string) error
	ConsumePasswordReset(ctx context.Context, rawToken, newPassword string) error
	PeekPasswordReset(ctx context.Context, rawToken string) (*actiontoken.ActionToken, error)
}

type HandlerConfig struct {
	Logger    *slog.Logger
	Users     userLookup
	VerifySvc verificationService
	ResetSvc  passwordResetService
	General   config.GeneralConfig
	Secure    bool
}

type Handler struct {
	svc       authService
	sessSvc   sessionService
	users     userLookup
	verifySvc verificationService
	resetSvc  passwordResetService
	logger    *slog.Logger
	general   config.GeneralConfig
	secure    bool
}

func NewHandler(svc authService, sessSvc sessionService, cfg HandlerConfig) *Handler {
	return &Handler{
		svc:       svc,
		sessSvc:   sessSvc,
		users:     cfg.Users,
		verifySvc: cfg.VerifySvc,
		resetSvc:  cfg.ResetSvc,
		logger:    cfg.Logger,
		general:   cfg.General,
		secure:    cfg.Secure,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /register", h.showRegister)
	mux.HandleFunc("POST /register", h.doRegister)
	mux.HandleFunc("GET /login", h.showLogin)
	mux.HandleFunc("POST /login", h.doLogin)
	mux.HandleFunc("POST /logout", h.doLogout)
	mux.HandleFunc("GET /verify-account", h.showVerifyAccount)
	mux.HandleFunc("GET /verify", h.doVerify)
	mux.HandleFunc("POST /verify/resend", h.doResendVerification)
	mux.HandleFunc("GET /forgot-password", h.showForgotPassword)
	mux.HandleFunc("POST /forgot-password", h.doForgotPassword)
	mux.HandleFunc("GET /reset-password", h.showResetPassword)
	mux.HandleFunc("POST /reset-password", h.doResetPassword)
}

func (h *Handler) showRegister(w http.ResponseWriter, r *http.Request) {
	httpx.RenderHTML(w, r, h.logger, registerPage(h.layout(), RegisterFormState{}))
}

func (h *Handler) doRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRegisterFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderRegister(w, r, RegisterFormState{FormError: invalidFormDataMsg})
		return
	}

	cmd := app.CreateCommand{
		Username:        r.PostFormValue("username"),
		Email:           r.PostFormValue("email"),
		Password:        r.PostFormValue("password"),
		PasswordConfirm: r.PostFormValue("password_confirm"),
		Gender:          r.PostFormValue("gender"),
	}

	_, err := h.svc.Create(r.Context(), cmd)
	if err != nil {
		state := RegisterFormState{
			Username: cmd.Username,
			Email:    cmd.Email,
			Gender:   cmd.Gender,
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

	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) renderRegister(w http.ResponseWriter, r *http.Request, state RegisterFormState) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, registerForm(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, registerPage(h.layout(), state))
}

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	httpx.RenderHTML(w, r, h.logger, loginPage(h.layout(), LoginFormState{}))
}

func (h *Handler) doLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, r, LoginFormState{Error: invalidFormDataMsg})
		return
	}

	cmd := app.LoginCommand{
		Username: r.PostFormValue("username"),
		Password: r.PostFormValue("password"),
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

	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
