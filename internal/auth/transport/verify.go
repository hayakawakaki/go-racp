package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type userLookup interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

type verificationService interface {
	ConsumeVerification(ctx context.Context, rawToken string) error
	ResendVerification(ctx context.Context, accountID int) error
}

func (h *Handler) showVerifyAccount(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	sess, err := h.sessSvc.Validate(r.Context(), cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.users.GetByID(r.Context(), sess.UserID)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if user.GroupID != 5 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	httpx.RenderHTML(w, r, h.logger, verifyAccountPage(h.layout(), VerifyAccountState{Email: user.Email}))
}

func (h *Handler) doVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), VerifyResultState{Kind: VerifyResultInvalid}))
		return
	}
	err := h.verifySvc.ConsumeVerification(r.Context(), token)
	state := VerifyResultState{}
	switch {
	case err == nil:
		state.Kind = VerifyResultSuccess
	case errors.Is(err, domain.ErrTokenAlreadyUsed):
		state.Kind = VerifyResultAlready
	case errors.Is(err, domain.ErrTokenExpired):
		state.Kind = VerifyResultExpired
	case errors.Is(err, domain.ErrTokenInvalid):
		state.Kind = VerifyResultInvalid
	default:
		h.logger.Error("verify consume", "err", err)
		state.Kind = VerifyResultInvalid
	}
	httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), state))
}

func (h *Handler) doResendVerification(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	sess, err := h.sessSvc.Validate(r.Context(), cookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := h.verifySvc.ResendVerification(r.Context(), sess.UserID); err != nil {
		h.logger.Error("verify resend", "err", err)
	}
	if httpx.IsHTMX(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/verify-account", http.StatusSeeOther)
}
