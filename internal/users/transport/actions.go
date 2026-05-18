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

func actorIDFromContext(r *http.Request) (int, bool) {
	snap, ok := middleware.SnapshotFromContext(r.Context())
	if !ok || snap == nil {
		return 0, false
	}

	return snap.UserID, true
}

func (h *Handler) now() time.Time { return time.Now() }

func (h *Handler) doBan(w http.ResponseWriter, r *http.Request) {
	targetID, ok := pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUserActionFormBytes)
	if err := r.ParseForm(); err != nil {
		writeActionError(w, r, h, "Invalid form data.", http.StatusBadRequest)

		return
	}
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeActionError(w, r, h, "Session missing.", http.StatusUnauthorized)

		return
	}

	cmd := app.BanCommand{
		ActorUserID:  actorID,
		TargetUserID: targetID,
		Permanent:    r.FormValue("permanent") != "",
		Days:         atoiOrZero(r.FormValue("days")),
		Reason:       r.FormValue("reason"),
	}

	detail, err := h.svc.Ban(r.Context(), cmd)
	if err != nil {
		writeActionErrorFromDomain(w, r, h, err)

		return
	}

	state := detailState{
		Detail:       detail,
		Now:          h.now(),
		AllowedRoles: buildRoleOptions(h.svc.AllowedRoles()),
	}
	httpx.RenderHTML(w, r, h.logger, detailContent(state))
}

func (h *Handler) doUnban(w http.ResponseWriter, r *http.Request) {
	targetID, ok := pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUserActionFormBytes)
	if err := r.ParseForm(); err != nil {
		writeActionError(w, r, h, "Invalid form data.", http.StatusBadRequest)

		return
	}
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeActionError(w, r, h, "Session missing.", http.StatusUnauthorized)

		return
	}

	detail, err := h.svc.Unban(r.Context(), app.UnbanCommand{
		ActorUserID:  actorID,
		TargetUserID: targetID,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		writeActionErrorFromDomain(w, r, h, err)

		return
	}

	state := detailState{
		Detail:       detail,
		Now:          h.now(),
		AllowedRoles: buildRoleOptions(h.svc.AllowedRoles()),
	}
	httpx.RenderHTML(w, r, h.logger, detailContent(state))
}

func (h *Handler) doSetRole(w http.ResponseWriter, r *http.Request) {
	targetID, ok := pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUserActionFormBytes)
	if err := r.ParseForm(); err != nil {
		writeActionError(w, r, h, "Invalid form data.", http.StatusBadRequest)

		return
	}
	actorID, ok := actorIDFromContext(r)
	if !ok {
		writeActionError(w, r, h, "Session missing.", http.StatusUnauthorized)

		return
	}

	groupID, err := strconv.Atoi(r.FormValue("group_id"))
	if err != nil {
		writeActionError(w, r, h, "Invalid role selection.", http.StatusBadRequest)

		return
	}

	detail, err := h.svc.SetRole(r.Context(), app.SetRoleCommand{
		ActorUserID:  actorID,
		TargetUserID: targetID,
		NewGroupID:   groupID,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		writeActionErrorFromDomain(w, r, h, err)

		return
	}

	state := detailState{
		Detail:       detail,
		Now:          h.now(),
		AllowedRoles: buildRoleOptions(h.svc.AllowedRoles()),
	}
	httpx.RenderHTML(w, r, h.logger, detailContent(state))
}

func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}

func atoiOrZero(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return n
}

func writeActionError(w http.ResponseWriter, r *http.Request, h *Handler, message string, status int) {
	w.WriteHeader(status)
	httpx.RenderHTML(w, r, h.logger, actionError(message))
}

func writeActionErrorFromDomain(w http.ResponseWriter, r *http.Request, h *Handler, err error) {
	switch {
	case errors.Is(err, domain.ErrSelfAction):
		writeActionError(w, r, h, "You can't perform admin actions on your own account.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrTargetIsAdmin):
		writeActionError(w, r, h, "Admin-on-admin actions must go through the database.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrEmptyReason):
		writeActionError(w, r, h, "Reason is required.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidDuration):
		writeActionError(w, r, h, "Invalid ban duration.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidRole):
		writeActionError(w, r, h, "Selected role is not allowed.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidState):
		writeActionError(w, r, h, "Action not allowed in the current state.", http.StatusBadRequest)
	case errors.Is(err, domain.ErrNotFound):
		writeActionError(w, r, h, "Account not found.", http.StatusNotFound)
	default:
		h.logger.Error("users: action failed", "err", err)
		writeActionError(w, r, h, "Action failed. Check server logs.", http.StatusInternalServerError)
	}
}
