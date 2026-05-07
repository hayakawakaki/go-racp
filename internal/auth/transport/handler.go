package transport

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

const (
	maxRegisterFormBytes = 4 << 10
	maxLoginFormBytes    = 2 << 10
)

type Handler struct {
	svc    *app.Service
	logger *slog.Logger
}

func NewHandler(svc *app.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /register", h.showRegister)
	mux.HandleFunc("POST /register", h.doRegister)
	mux.HandleFunc("GET /login", h.showLogin)
	mux.HandleFunc("POST /login", h.doLogin)
	mux.HandleFunc("POST /logout", h.doLogout)
}

func (h *Handler) showRegister(w http.ResponseWriter, r *http.Request) {
	httpx.RenderHTML(w, r, h.logger, registerPage(RegisterFormState{}))
}

func (h *Handler) doRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRegisterFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderRegister(w, r, RegisterFormState{Error: "Invalid form data."})
		return
	}

	cmd := app.CreateCommand{
		Username: r.PostFormValue("username"),
		Email:    r.PostFormValue("email"),
		Password: r.PostFormValue("password"),
		Gender:   r.PostFormValue("gender"),
	}

	_, err := h.svc.Create(r.Context(), cmd)
	if err != nil {
		state := RegisterFormState{
			Username: cmd.Username,
			Email:    cmd.Email,
			Gender:   cmd.Gender,
		}
		switch {
		case errors.Is(err, domain.ErrUsernameConflict):
			state.Error = "Username already taken."
		case errors.Is(err, domain.ErrEmailConflict):
			state.Error = "Email already in use."
		default:
			h.logger.Error("register", "err", err)
			state.Error = "Something went wrong. Please try again."
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
	httpx.RenderHTML(w, r, h.logger, registerPage(state))
}

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	httpx.RenderHTML(w, r, h.logger, loginPage(LoginFormState{}))
}

func (h *Handler) doLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginFormBytes)
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, r, LoginFormState{Error: "Invalid form data."})
		return
	}

	cmd := app.LoginCommand{
		Username: r.PostFormValue("username"),
		Password: r.PostFormValue("password"),
	}

	_, err := h.svc.Authenticate(r.Context(), cmd)
	if err != nil {
		state := LoginFormState{Username: cmd.Username}
		if errors.Is(err, domain.ErrInvalidCredentials) {
			state.Error = "Invalid username or password."
		} else {
			h.logger.Error("login", "err", err)
			state.Error = "Something went wrong. Please try again."
		}
		h.renderLogin(w, r, state)
		return
	}

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
	httpx.RenderHTML(w, r, h.logger, loginPage(state))
}

func (h *Handler) doLogout(w http.ResponseWriter, r *http.Request) {
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
