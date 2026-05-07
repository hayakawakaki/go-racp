package transport

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

const maxRegisterFormBytes = 4 << 10

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
	if err := registerPage(RegisterFormState{}).Render(r.Context(), w); err != nil {
		h.logger.Error("render register", "err", err)
	}
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

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) renderRegister(w http.ResponseWriter, r *http.Request, state RegisterFormState) {
	if r.Header.Get("HX-Request") == "true" {
		if err := registerForm(state).Render(r.Context(), w); err != nil {
			h.logger.Error("render register form", "err", err)
		}
		return
	}
	if err := registerPage(state).Render(r.Context(), w); err != nil {
		h.logger.Error("render register page", "err", err)
	}
}

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) doLogin(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) doLogout(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
