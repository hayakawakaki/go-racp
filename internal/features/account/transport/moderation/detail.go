package moderation

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	detail, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, accdomain.ErrUserNotFound) {
		w.WriteHeader(http.StatusNotFound)
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersNotFoundPage(h.layout(), strconv.Itoa(id)))
		return
	}
	if err != nil {
		h.logger.Error("users: get failed", "id", id, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	s := state.DetailState{
		Detail:       detail,
		Now:          time.Now(),
		AllowedRoles: state.BuildRoleOptions(h.svc.AllowedRoles()),
	}

	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailPage(h.layout(), detail.User.Username, s))
}
