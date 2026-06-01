package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
	"github.com/hayakawakaki/go-racp/internal/features/apikey/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

const maxFormBytes = 16 << 10

const (
	fieldName = "name"
	fieldTier = "tier"
)

type apiKeyService interface {
	Generate(ctx context.Context, name, tier string) (string, *domain.APIKey, error)
	List(ctx context.Context) ([]domain.APIKey, error)
	Revoke(ctx context.Context, id int64) error
	Tiers() []domain.Tier
}

type Renderer interface {
	APIKeysPage(layout httpx.Layout, state state.ListState) templ.Component
	APIKeysContent(state state.ListState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	General config.GeneralConfig
	Theme   Renderer
	Logger  *slog.Logger
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	general config.GeneralConfig
	svc     apiKeyService
	theme   Renderer
	logger  *slog.Logger
}

func NewHandler(service apiKeyService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{svc: service, theme: cfg.Theme, logger: logger, general: cfg.General}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Admin.ApiKeys", "GET /admin/api-keys", http.HandlerFunc(h.showList))
	reg.Wrap(mux, "Admin.ApiKeys", "POST /admin/api-keys", http.HandlerFunc(h.create))
	reg.Wrap(mux, "Admin.ApiKeys", "POST /admin/api-keys/{id}/revoke", http.HandlerFunc(h.revoke))
}

func (h *Handler) showList(w http.ResponseWriter, r *http.Request) {
	s, err := h.listState(r.Context())
	if err != nil {
		h.logger.Error("apikey: list", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.render(w, r, s)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if err := httpx.ParseForm(w, r, maxFormBytes); err != nil {
		http.Error(w, "request too large or malformed", http.StatusBadRequest)
		return
	}

	name := r.PostFormValue(fieldName)
	tier := r.PostFormValue(fieldTier)

	raw, _, err := h.svc.Generate(r.Context(), name, tier)
	if err != nil {
		h.renderCreateError(w, r, name, tier, err)
		return
	}

	s, err := h.listState(r.Context())
	if err != nil {
		h.logger.Error("apikey: list after create", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.RevealedKey = raw
	s.RevealedName = name

	h.render(w, r, s)
}

func (h *Handler) renderCreateError(w http.ResponseWriter, r *http.Request, name, tier string, err error) {
	var validation *domain.ValidationError
	if !errors.As(err, &validation) {
		h.logger.Error("apikey: create", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s, listErr := h.listState(r.Context())
	if listErr != nil {
		h.logger.Error("apikey: list after validation", "err", listErr)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.Errors = validation.Fields
	s.FormName = name
	s.FormTier = tier

	h.render(w, r, s)
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err = h.svc.Revoke(r.Context(), id); err != nil && !errors.Is(err, domain.ErrKeyNotFound) {
		h.logger.Error("apikey: revoke", "err", err, "id", id)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s, err := h.listState(r.Context())
	if err != nil {
		h.logger.Error("apikey: list after revoke", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.render(w, r, s)
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, s state.ListState) {
	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.APIKeysContent(s))
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.APIKeysPage(h.layout(), s))
}

func (h *Handler) listState(ctx context.Context) (state.ListState, error) {
	keys, err := h.svc.List(ctx)
	if err != nil {
		return state.ListState{}, fmt.Errorf("transport.Handler.listState: %w", err)
	}

	return state.ListState{
		Keys:  keys,
		Tiers: h.svc.Tiers(),
		Now:   time.Now(),
	}, nil
}
