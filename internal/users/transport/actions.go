package transport

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/users/app"
	"github.com/hayakawakaki/go-racp/internal/users/domain"
)

const maxUserActionFormBytes = 4 << 10

func (h *Handler) targetAndActor(w http.ResponseWriter, r *http.Request) (targetID, actorID int, ok bool) {
	targetID, ok = pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return 0, 0, false
	}
	snap, snapOK := middleware.SnapshotFromContext(r.Context())
	if !snapOK || snap == nil {
		h.writeActionError(w, r, "Session missing.", http.StatusUnauthorized)

		return 0, 0, false
	}

	return targetID, snap.UserID, true
}

func (h *Handler) renderDetail(w http.ResponseWriter, r *http.Request, detail app.UserDetail) {
	state := detailState{
		Detail:       detail,
		Now:          time.Now(),
		AllowedRoles: buildRoleOptions(h.svc.AllowedRoles()),
	}
	httpx.RenderHTML(w, r, h.logger, detailContent(state))
}

func (h *Handler) doBan(w http.ResponseWriter, r *http.Request) {
	targetID, actorID, ok := h.targetAndActor(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUserActionFormBytes)
	if err := r.ParseForm(); err != nil {
		h.writeActionError(w, r, "Invalid form data.", http.StatusBadRequest)

		return
	}

	days, _ := strconv.Atoi(r.FormValue("days"))
	detail, err := h.svc.Ban(r.Context(), app.BanCommand{
		ActorUserID:  actorID,
		TargetUserID: targetID,
		Permanent:    r.FormValue("permanent") != "",
		Days:         days,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		h.writeActionErrorFromDomain(w, r, err)

		return
	}
	h.renderDetail(w, r, detail)
}

func (h *Handler) doUnban(w http.ResponseWriter, r *http.Request) {
	targetID, actorID, ok := h.targetAndActor(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUserActionFormBytes)
	if err := r.ParseForm(); err != nil {
		h.writeActionError(w, r, "Invalid form data.", http.StatusBadRequest)

		return
	}

	detail, err := h.svc.Unban(r.Context(), app.UnbanCommand{
		ActorUserID:  actorID,
		TargetUserID: targetID,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		h.writeActionErrorFromDomain(w, r, err)

		return
	}
	h.renderDetail(w, r, detail)
}

func (h *Handler) doSetRole(w http.ResponseWriter, r *http.Request) {
	targetID, actorID, ok := h.targetAndActor(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUserActionFormBytes)
	if err := r.ParseForm(); err != nil {
		h.writeActionError(w, r, "Invalid form data.", http.StatusBadRequest)

		return
	}

	groupID, err := strconv.Atoi(r.FormValue("group_id"))
	if err != nil {
		h.writeActionError(w, r, "Invalid role selection.", http.StatusBadRequest)

		return
	}

	detail, err := h.svc.SetRole(r.Context(), app.SetRoleCommand{
		ActorUserID:  actorID,
		TargetUserID: targetID,
		NewGroupID:   groupID,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		h.writeActionErrorFromDomain(w, r, err)

		return
	}
	h.renderDetail(w, r, detail)
}

func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}

func (h *Handler) writeActionError(w http.ResponseWriter, r *http.Request, message string, status int) {
	w.WriteHeader(status)
	httpx.RenderHTML(w, r, h.logger, actionError(message))
}

func (h *Handler) writeActionErrorFromDomain(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrSelfAction):
		h.writeActionError(w, r, "You can't perform admin actions on your own account.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrTargetIsAdmin):
		h.writeActionError(w, r, "Admin-on-admin actions must go through the database.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrEmptyReason):
		h.writeActionError(w, r, "Reason is required.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidDuration):
		h.writeActionError(w, r, "Invalid ban duration.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidRole):
		h.writeActionError(w, r, "Selected role is not allowed.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidState):
		h.writeActionError(w, r, "Action not allowed in the current state.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrNotFound):
		h.writeActionError(w, r, "Account not found.", http.StatusNotFound)
	default:
		h.logger.Error("users: action failed", "err", err)
		h.writeActionError(w, r, "Action failed. Check server logs.", http.StatusInternalServerError)
	}
}
