package transport

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/features/billing/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

var storeNoticeText = map[string]string{
	"unavailable": "The store is currently unavailable. Please try again later.",
	"invalid":     "That request could not be processed. Please try again.",
	"cancel":      "Checkout was cancelled. No payment was taken.",
	"success":     "Payment received. Your cash points will be credited shortly.",
}

func noticeMessage(code string) string {
	return storeNoticeText[code]
}

func (h *Handler) showStore(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	st := state.StoreState{
		Packages:  h.svc.Packages(),
		Currency:  h.currency,
		Notice:    noticeMessage(r.URL.Query().Get("notice")),
		Available: h.svc.Available(),
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.StorePage(h.layout(), st))
}

func (h *Handler) startCheckout(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Redirect(w, r, "/store?notice=unavailable", http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCheckoutFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/store?notice=invalid", http.StatusSeeOther)
		return
	}

	snapshot, ok := middleware.SnapshotFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/store?notice=invalid", http.StatusSeeOther)
		return
	}

	packageKey := r.FormValue(fieldPackage)
	successURL := h.appURL + "/store?notice=success"
	cancelURL := h.appURL + "/store?notice=cancel"

	redirectURL, err := h.svc.StartCheckout(r.Context(), snapshot.UserID, packageKey, successURL, cancelURL)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnknownPackage):
			http.Redirect(w, r, "/store?notice=invalid", http.StatusSeeOther)
		case errors.Is(err, domain.ErrProviderUnavailable):
			http.Redirect(w, r, "/store?notice=unavailable", http.StatusSeeOther)
		default:
			h.logger.Error("billing: start checkout", "err", err)
			http.Redirect(w, r, "/store?notice=invalid", http.StatusSeeOther)
		}
		return
	}

	h.redirectToCheckout(w, r, redirectURL)
}

func (h *Handler) redirectToCheckout(w http.ResponseWriter, r *http.Request, redirectURL string) {
	parsed, parseErr := url.Parse(redirectURL)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" {
		h.logger.Error("billing: provider returned an unexpected redirect url", "err", parseErr)
		http.Redirect(w, r, "/store?notice=invalid", http.StatusSeeOther)
		return
	}

	//nolint:gosec // redirect is by payment provider
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
