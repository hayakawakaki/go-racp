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

	dpage := httpx.ParsePositiveInt(r.URL.Query().Get("dpage"), 1)
	wpage := httpx.ParsePositiveInt(r.URL.Query().Get("wpage"), 1)

	s := state.DetailState{
		Detail:       detail,
		Now:          time.Now(),
		Location:     h.general.Location(),
		AllowedRoles: state.BuildRoleOptions(h.svc.AllowedRoles()),
	}

	if h.currency != nil {
		deposits, depositErr := h.currency.DepositHistoryByAccount(r.Context(), detail.User.ID, dpage, detailHistoryPerPage)
		if depositErr != nil {
			h.logger.Warn("users: deposit history failed", "id", id, "err", depositErr)
		} else {
			s.Deposits = deposits
		}

		withdraws, withdrawErr := h.currency.WithdrawHistoryByAccount(r.Context(), detail.User.ID, wpage, detailHistoryPerPage)
		if withdrawErr != nil {
			h.logger.Warn("users: withdraw history failed", "id", id, "err", withdrawErr)
		} else {
			s.Withdraws = withdraws
		}
	}

	if httpx.IsHTMX(r) {
		httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailContent(s))
		return
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.UsersDetailPage(h.layout(), detail.User.Username, s))
}
