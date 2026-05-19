package transport

import (
	"context"
	"errors"
	"net/http"

	actiontokendomain "github.com/hayakawakaki/go-racp/internal/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/app"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/httpx"
)

type userLookup interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

const (
	resendNoticeSent   = "sent"
	resendNoticeFailed = "failed"

	maxVerifyFormBytes = 1 << 10
)

var resendNoticeText = map[string]string{
	resendNoticeSent:   "Verification email sent. Check your inbox.",
	resendNoticeFailed: "Couldn't send verification email. Please try again in a moment.",
}

func (h *Handler) showVerifyAccount(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
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
	if user.State != app.StateUnverified {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	state := VerifyAccountState{Email: user.Email}
	if notice, ok := resendNoticeText[r.URL.Query().Get("notice")]; ok {
		state.Notice = notice
	}
	httpx.RenderHTML(w, r, h.logger, verifyAccountPage(h.layout(), state))
}

func (h *Handler) showVerify(w http.ResponseWriter, r *http.Request) {
	expired := verifyResultPage(h.layout(), VerifyResultState{Kind: VerifyResultExpired})
	token, ok := h.validateTokenLink(w, r, actiontokendomain.EmailVerification, "verify peek", expired)
	if !ok {
		return
	}

	httpx.RenderHTML(w, r, h.logger, verifyConfirmPage(h.layout(), VerifyConfirmState{Token: token}))
}

//nolint:cyclop // splitting would obscure the flow
func (h *Handler) doVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := httpx.ParseForm(w, r, maxVerifyFormBytes); err != nil {
		httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), VerifyResultState{Kind: VerifyResultInvalid}))
		return
	}

	token := r.PostFormValue(fieldToken)
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), VerifyResultState{Kind: VerifyResultInvalid}))
		return
	}

	err := h.svc.ConsumeVerification(r.Context(), token)
	if (err == nil || errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed)) && h.hasActiveSession(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	state := VerifyResultState{}
	switch {
	case err == nil:
		state.Kind = VerifyResultSuccess
	case errors.Is(err, actiontokendomain.ErrTokenExpired):
		state.Kind = VerifyResultExpired
	default:
		if !errors.Is(err, actiontokendomain.ErrTokenInvalid) && !errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed) {
			h.logger.Error("verify consume", "err", err)
		}
		state.Kind = VerifyResultInvalid
	}
	httpx.RenderHTML(w, r, h.logger, verifyResultPage(h.layout(), state))
}

func (h *Handler) hasActiveSession(r *http.Request) bool {
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	_, err = h.sessSvc.Validate(r.Context(), cookie.Value)

	return err == nil
}

func (h *Handler) doResendVerification(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
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
	if err := h.svc.ResendVerification(r.Context(), sess.UserID); err != nil {
		h.logger.Error("verify resend", "err", err)
		notice = resendNoticeFailed
	}

	httpx.Redirect(w, r, "/verify-account?notice="+notice)
}
