// Package transport provides the HTTP handler and server-rendered templates
// for the auth feature.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/app"
)

// Handler is the HTTP handler for auth endpoints. It depends on an app.Service
// for business logic and a structured logger for error reporting.
type Handler struct {
	svc    *app.Service
	logger *slog.Logger
}

// NewHandler creates a Handler with the given service and logger.
func NewHandler(svc *app.Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// RegisterRoutes attaches all auth routes to mux:
//
//	GET  /register  – render the registration form
//	POST /register  – submit the registration form
//	GET  /login     – render the login form (not yet implemented)
//	POST /login     – submit the login form (not yet implemented)
//	POST /logout    – end the current session (not yet implemented)
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /register", h.showRegister)
	mux.HandleFunc("POST /register", h.doRegister)
	mux.HandleFunc("GET /login", h.showLogin)
	mux.HandleFunc("POST /login", h.doLogin)
	mux.HandleFunc("POST /logout", h.doLogout)
}

// showRegister renders the registration page with an empty form state.
func (h *Handler) showRegister(w http.ResponseWriter, r *http.Request) {
	if err := registerPage(RegisterFormState{}).Render(r.Context(), w); err != nil {
		h.logger.Error("render register", "err", err)
	}
}

// doRegister handles a POST /register submission. Not yet implemented.
func (h *Handler) doRegister(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// showLogin renders the login page. Not yet implemented.
func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// doLogin handles a POST /login submission. Not yet implemented.
func (h *Handler) doLogin(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// doLogout handles a POST /logout request and ends the user session.
// Not yet implemented.
func (h *Handler) doLogout(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
