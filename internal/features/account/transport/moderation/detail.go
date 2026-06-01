package moderation

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
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
		Location:     h.general.Location(),
		AllowedRoles: state.BuildRoleOptions(h.svc.AllowedRoles()),
	}

	if snap, ok := middleware.SnapshotFromContext(r.Context()); ok && snap != nil {
		s.ViewerIsAdmin = snap.IsAdmin()
	}

	if s.ViewerIsAdmin {
		h.loadDetailHistory(r, &s, detail.User.ID)
	}

	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailPage(h.layout(), detail.User.Username, s))
}

func (h *Handler) loadDetailHistory(r *http.Request, s *state.DetailState, accountID int) {
	if h.currency == nil {
		return
	}

	dpage := httpx.ParsePositiveInt(r.URL.Query().Get("dpage"), 1)
	wpage := httpx.ParsePositiveInt(r.URL.Query().Get("wpage"), 1)

	deposits, err := h.currency.DepositHistoryByAccount(r.Context(), accountID, dpage, detailHistoryPerPage)
	if err != nil {
		h.logger.Warn("users: deposit history failed", "id", accountID, "err", err)
		s.DepositsFailed = true
	} else {
		s.Deposits = deposits
	}

	withdraws, err := h.currency.WithdrawHistoryByAccount(r.Context(), accountID, wpage, detailHistoryPerPage)
	if err != nil {
		h.logger.Warn("users: withdraw history failed", "id", accountID, "err", err)
		s.WithdrawsFailed = true
	} else {
		s.Withdraws = withdraws
	}
}
