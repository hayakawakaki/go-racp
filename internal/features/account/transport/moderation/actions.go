package moderation

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

const maxUserActionFormBytes = 4 << 10

func (h *Handler) targetAndActor(w http.ResponseWriter, r *http.Request) (targetID, actorID int, actorIsAdmin, ok bool) {
	targetID, ok = pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return 0, 0, false, false
	}
	snap, snapOK := middleware.SnapshotFromContext(r.Context())
	if !snapOK || snap == nil {
		h.writeActionError(w, r, "Session missing.", http.StatusUnauthorized)
		return 0, 0, false, false
	}

	return targetID, snap.UserID, snap.IsAdmin(), true
}

func (h *Handler) renderDetail(w http.ResponseWriter, r *http.Request, detail app.UserDetail, viewerIsAdmin bool) {
	s := state.DetailState{
		Detail:        detail,
		Now:           time.Now(),
		Location:      h.general.Location(),
		AllowedRoles:  state.BuildRoleOptions(h.svc.AllowedRoles()),
		ViewerIsAdmin: viewerIsAdmin,
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailContent(s))
}

func (h *Handler) modalState(r *http.Request, detail app.UserDetail) state.DetailState {
	s := state.DetailState{
		Detail:       detail,
		Now:          time.Now(),
		Location:     h.general.Location(),
		AllowedRoles: state.BuildRoleOptions(h.svc.AllowedRoles()),
	}
	if snap, ok := middleware.SnapshotFromContext(r.Context()); ok && snap != nil {
		s.ViewerIsAdmin = snap.IsAdmin()
	}

	return s
}

func (h *Handler) showBanModal(w http.ResponseWriter, r *http.Request) {
	h.showActionModal(w, r, func(s state.DetailState) templ.Component { return h.theme.UsersBanModal(s) })
}

func (h *Handler) showUnbanModal(w http.ResponseWriter, r *http.Request) {
	h.showActionModal(w, r, func(s state.DetailState) templ.Component { return h.theme.UsersUnbanModal(s) })
}

func (h *Handler) showRoleModal(w http.ResponseWriter, r *http.Request) {
	h.showActionModal(w, r, func(s state.DetailState) templ.Component { return h.theme.UsersRoleModal(s) })
}

func (h *Handler) showActionModal(w http.ResponseWriter, r *http.Request, render func(state.DetailState) templ.Component) {
	id, ok := pathID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if !httpx.IsHTMX(r) {
		httpx.Redirect(w, r, "/users/"+strconv.Itoa(id))
		return
	}

	user, err := h.svc.GetUser(r.Context(), id)
	if errors.Is(err, accdomain.ErrUserNotFound) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("users: modal get failed", "id", id, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	httpx.RenderHTML(w, r, h.logger, render(h.modalState(r, app.UserDetail{User: user})))
}

func (h *Handler) doBan(w http.ResponseWriter, r *http.Request) {
	targetID, actorID, actorIsAdmin, ok := h.targetAndActor(w, r)
	if !ok {
		return
	}
	banModal := func(s state.DetailState) templ.Component { return h.theme.UsersBanModal(s) }
	if err := httpx.ParseForm(w, r, maxUserActionFormBytes); err != nil {
		h.actionModalError(w, r, targetID, "Invalid form data.", banModal)
		return
	}

	days, _ := strconv.Atoi(r.FormValue("days"))
	detail, err := h.svc.Ban(r.Context(), app.BanCommand{
		ActorUserID:  actorID,
		ActorIsAdmin: actorIsAdmin,
		TargetUserID: targetID,
		Permanent:    r.FormValue("permanent") != "",
		Days:         days,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		message, expected := actionErrorMessage(err)
		if !expected {
			h.logger.Error("users: action failed", "err", err)
		}
		h.actionModalError(w, r, targetID, message, banModal)
		return
	}

	h.renderDetail(w, r, detail, actorIsAdmin)
}

func (h *Handler) doUnban(w http.ResponseWriter, r *http.Request) {
	targetID, actorID, actorIsAdmin, ok := h.targetAndActor(w, r)
	if !ok {
		return
	}
	unbanModal := func(s state.DetailState) templ.Component { return h.theme.UsersUnbanModal(s) }
	if err := httpx.ParseForm(w, r, maxUserActionFormBytes); err != nil {
		h.actionModalError(w, r, targetID, "Invalid form data.", unbanModal)
		return
	}

	detail, err := h.svc.Unban(r.Context(), app.UnbanCommand{
		ActorUserID:  actorID,
		ActorIsAdmin: actorIsAdmin,
		TargetUserID: targetID,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		message, expected := actionErrorMessage(err)
		if !expected {
			h.logger.Error("users: action failed", "err", err)
		}
		h.actionModalError(w, r, targetID, message, unbanModal)
		return
	}

	h.renderDetail(w, r, detail, actorIsAdmin)
}

func (h *Handler) doSetRole(w http.ResponseWriter, r *http.Request) {
	targetID, actorID, actorIsAdmin, ok := h.targetAndActor(w, r)
	if !ok {
		return
	}
	roleModal := func(s state.DetailState) templ.Component { return h.theme.UsersRoleModal(s) }
	if err := httpx.ParseForm(w, r, maxUserActionFormBytes); err != nil {
		h.actionModalError(w, r, targetID, "Invalid form data.", roleModal)
		return
	}

	groupID, err := strconv.Atoi(r.FormValue("group_id"))
	if err != nil {
		h.actionModalError(w, r, targetID, "Invalid role selection.", roleModal)
		return
	}

	detail, err := h.svc.SetRole(r.Context(), app.SetRoleCommand{
		ActorUserID:  actorID,
		ActorIsAdmin: actorIsAdmin,
		TargetUserID: targetID,
		NewGroupID:   groupID,
		Reason:       r.FormValue("reason"),
	})
	if err != nil {
		message, expected := actionErrorMessage(err)
		if !expected {
			h.logger.Error("users: action failed", "err", err)
		}
		h.actionModalError(w, r, targetID, message, roleModal)
		return
	}

	h.renderDetail(w, r, detail, actorIsAdmin)
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
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersActionError(message))
}

func (h *Handler) actionModalError(w http.ResponseWriter, r *http.Request, targetID int, message string, render func(state.DetailState) templ.Component) {
	user, err := h.svc.GetUser(r.Context(), targetID)
	if err != nil {
		h.writeActionError(w, r, message, http.StatusBadRequest)
		return
	}

	s := h.modalState(r, app.UserDetail{User: user})
	s.ActionError = message
	w.Header().Set("HX-Retarget", "#modal")
	w.Header().Set("HX-Reswap", "innerHTML")

	httpx.RenderHTML(w, r, h.logger, render(s))
}

func actionErrorMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, accdomain.ErrSelfAction):
		return "You can't perform admin actions on your own account.", true
	case errors.Is(err, accdomain.ErrTargetIsAdmin):
		return "Admin-on-admin actions must go through the database.", true
	case errors.Is(err, accdomain.ErrTargetProtected):
		return "You can only act on player accounts.", true
	case errors.Is(err, accdomain.ErrEmptyReason):
		return "Reason is required.", true
	case errors.Is(err, accdomain.ErrInvalidDuration):
		return "Invalid ban duration.", true
	case errors.Is(err, accdomain.ErrInvalidRole):
		return "Selected role is not allowed.", true
	case errors.Is(err, accdomain.ErrInvalidState):
		return "Action not allowed in the current state.", true
	case errors.Is(err, accdomain.ErrUserNotFound):
		return "Account not found.", true
	default:
		return "Action failed. Check server logs.", false
	}
}
