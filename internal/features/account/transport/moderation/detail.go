package moderation

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

type roleOption struct {
	Name    string
	GroupID int
}

type DetailState struct {
	Now          time.Time
	Detail       app.UserDetail
	AllowedRoles []roleOption
}

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

	state := DetailState{
		Detail:       detail,
		Now:          time.Now(),
		AllowedRoles: buildRoleOptions(h.svc.AllowedRoles()),
	}

	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailContent(state))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailPage(h.layout(), detail.User.Username, state))
}

func buildRoleOptions(allowed map[int]string) []roleOption {
	out := make([]roleOption, 0, len(allowed))
	for id, name := range allowed {
		out = append(out, roleOption{GroupID: id, Name: name})
	}
	slices.SortFunc(out, func(a, b roleOption) int { return a.GroupID - b.GroupID })

	return out
}

func roleNameFor(state DetailState, groupID int) string {
	for _, opt := range state.AllowedRoles {
		if opt.GroupID == groupID {
			return opt.Name
		}
	}
	if groupID == accdomain.RoleAdmin.GroupID {
		return "Admin"
	}

	return fmt.Sprintf("group_%d", groupID)
}
