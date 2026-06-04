package self

import (
	"context"
	"errors"
	"net/http"

	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
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

func (h *Handler) requireUnverifiedSession(w http.ResponseWriter, r *http.Request) (*domain.User, bool) {
	cookie, err := r.Cookie(middleware.SessionCookieName)
	if err != nil || cookie.Value == "" {
		httpx.Redirect(w, r, "/login")
		return nil, false
	}

	sess, err := h.sessSvc.Validate(r.Context(), cookie.Value)
	if err != nil {
		httpx.Redirect(w, r, "/login")
		return nil, false
	}

	user, err := h.users.GetByID(r.Context(), sess.UserID)
	if err != nil {
		httpx.Redirect(w, r, "/login")
		return nil, false
	}
	if user.State != app.StateUnverified {
		httpx.Redirect(w, r, "/")
		return nil, false
	}

	return user, true
}

func (h *Handler) showVerifyAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireUnverifiedSession(w, r)
	if !ok {
		return
	}

	state := selfstate.VerifyAccountState{Email: user.Email}
	if notice, ok := resendNoticeText[r.URL.Query().Get("notice")]; ok {
		state.Notice = notice
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AccountVerifyAccountPage(h.layout(), state))
}

func (h *Handler) showVerify(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireUnverifiedSession(w, r); !ok {
		return
	}

	expired := h.theme.AccountVerifyResultPage(h.layout(), selfstate.VerifyResultState{Kind: selfstate.VerifyResultExpired})
	token, ok := h.validateTokenLink(w, r, actiontokendomain.EmailVerification, "verify peek", expired)
	if !ok {
		return
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.AccountVerifyConfirmPage(h.layout(), selfstate.VerifyConfirmState{Token: token}))
}

//nolint:cyclop // splitting would obscure the flow
func (h *Handler) doVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	if err := httpx.ParseForm(w, r, maxVerifyFormBytes); err != nil {
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountVerifyResultPage(h.layout(), selfstate.VerifyResultState{Kind: selfstate.VerifyResultInvalid}))
		return
	}

	token := r.PostFormValue(fieldToken)
	if token == "" {
		httpx.RenderHTML(w, r, h.logger, h.theme.AccountVerifyResultPage(h.layout(), selfstate.VerifyResultState{Kind: selfstate.VerifyResultInvalid}))
		return
	}

	err := h.svc.ConsumeVerification(r.Context(), token)
	if (err == nil || errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed)) && h.hasActiveSession(r) {
		httpx.Redirect(w, r, "/")
		return
	}

	state := selfstate.VerifyResultState{}
	switch {
	case err == nil:
		state.Kind = selfstate.VerifyResultSuccess
	case errors.Is(err, actiontokendomain.ErrTokenExpired):
		state.Kind = selfstate.VerifyResultExpired
	default:
		if !errors.Is(err, actiontokendomain.ErrTokenInvalid) && !errors.Is(err, actiontokendomain.ErrTokenAlreadyUsed) {
			h.logger.Error("verify consume", "err", err)
		}
		state.Kind = selfstate.VerifyResultInvalid
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.AccountVerifyResultPage(h.layout(), state))
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
		httpx.Redirect(w, r, "/login")
		return
	}

	sess, err := h.sessSvc.Validate(r.Context(), cookie.Value)
	if err != nil {
		httpx.Redirect(w, r, "/login")
		return
	}

	notice := resendNoticeSent
	if err := h.svc.ResendVerification(r.Context(), sess.UserID); err != nil {
		h.logger.Error("verify resend", "err", err)
		notice = resendNoticeFailed
	}

	httpx.Redirect(w, r, "/verify-account?notice="+notice)
}
