package transport

import (
	"context"
	"errors"
	"net/http"

	"github.com/hayakawakaki/go-racp/internal/actiontoken"
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

const (
	resendNoticeSent   = "sent"
	resendNoticeFailed = "failed"
)

var resendNoticeText = map[string]string{
	resendNoticeSent:   "Verification email sent. Check your inbox.",
	resendNoticeFailed: "Couldn't send verification email. Please try again in a moment.",
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
	state := VerifyAccountState{Email: user.Email}
	if notice, ok := resendNoticeText[r.URL.Query().Get("notice")]; ok {
		state.Notice = notice
	}
	httpx.RenderHTML(w, r, h.logger, verifyAccountPage(h.layout(), state))
}

func (h *Handler) doVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), VerifyResultState{Kind: VerifyResultInvalid}))
		return
	}
	err := h.verifySvc.ConsumeVerification(r.Context(), token)

	if (err == nil || errors.Is(err, actiontoken.ErrTokenAlreadyUsed)) && h.hasActiveSession(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	state := VerifyResultState{}
	switch {
	case err == nil:
		state.Kind = VerifyResultSuccess
	case errors.Is(err, actiontoken.ErrTokenExpired):
		state.Kind = VerifyResultExpired
	default:
		if !errors.Is(err, actiontoken.ErrTokenInvalid) && !errors.Is(err, actiontoken.ErrTokenAlreadyUsed) {
			h.logger.Error("verify consume", "err", err)
		}
		state.Kind = VerifyResultInvalid
	}
	httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), state))
}

func (h *Handler) hasActiveSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	_, err = h.sessSvc.Validate(r.Context(), cookie.Value)
	return err == nil
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
	notice := resendNoticeSent
	if err := h.verifySvc.ResendVerification(r.Context(), sess.UserID); err != nil {
		h.logger.Error("verify resend", "err", err)
		notice = resendNoticeFailed
	}
	target := "/verify-account?notice=" + notice
	if httpx.IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
