package transport

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/character/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showDetail(w http.ResponseWriter, r *http.Request) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	charID, ok := parseCharID(r)
	if !ok {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	dto, err := h.svc.Get(r.Context(), sess.UserID, charID)
	if errors.Is(err, domain.ErrCharacterNotFound) || errors.Is(err, domain.ErrNotOwner) {
		http.Redirect(w, r, "/account?notice="+noticeNotFound, http.StatusSeeOther)
		return
	}
	if err != nil {
		h.logger.Error("character: detail", "err", err, "char_id", charID)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := DetailState{Char: dto, Now: time.Now()}
	if notice, ok := noticeText[r.URL.Query().Get("notice")]; ok {
		state.Notice = notice
	}
	httpx.RenderHTML(w, r, h.logger, detailPage(h.layout(), state))
}

func (h *Handler) doResetLook(w http.ResponseWriter, r *http.Request) {
	h.runReset(w, r, func(accountID, charID int) error {
		return h.svc.ResetLook(r.Context(), accountID, charID)
	}, noticeLookReset)
}

func (h *Handler) doResetLocation(w http.ResponseWriter, r *http.Request) {
	h.runReset(w, r, func(accountID, charID int) error {
		return h.svc.ResetLocation(r.Context(), accountID, charID)
	}, noticeLocationReset)
}

func (h *Handler) runReset(w http.ResponseWriter, r *http.Request, op func(accountID, charID int) error, success string) {
	sess, ok := middleware.SessionFromContext(r.Context())
	if !ok || sess == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	charID, ok := parseCharID(r)
	if !ok {
		httpx.Render404(w, r, h.logger, h.layout())
		return
	}

	err := op(sess.UserID, charID)
	notice := success
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrCharacterNotFound), errors.Is(err, domain.ErrNotOwner):
		http.Redirect(w, r, "/account?notice="+noticeNotFound, http.StatusSeeOther)
		return
	case errors.Is(err, domain.ErrCharacterOnline):
		notice = noticeOnlineLocked
	case errors.Is(err, domain.ErrCooldown):
		notice = noticeCooldownActive
	default:
		h.logger.Error("character: reset", "err", err, "char_id", charID)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/characters/"+strconv.Itoa(charID)+"?notice="+notice, http.StatusSeeOther)
}

func parseCharID(r *http.Request) (int, bool) {
	raw := r.PathValue("charID")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}
