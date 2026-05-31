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
	"cancel":      "Your payment was not completed.",
}

func noticeMessage(code string) string {
	return storeNoticeText[code]
}

func (h *Handler) showStore(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	available := h.svc.Available()
	packages := h.svc.Packages()
	notice := r.URL.Query().Get("notice")

	st := state.StoreState{
		Packages:  packages,
		Currency:  h.currency,
		Methods:   paymentMethods(available),
		Available: available,
	}
	if notice == "success" {
		if purchased, ok := h.confirmPurchase(r); ok {
			st.Success = true
			st.Purchased = &purchased
		} else {
			st.Notice = noticeMessage("cancel")
		}
	} else {
		st.Notice = noticeMessage(notice)
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.StorePage(h.layout(), st))
}

func (h *Handler) confirmPurchase(r *http.Request) (domain.Package, bool) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		return domain.Package{}, false
	}

	snapshot, ok := middleware.SnapshotFromContext(r.Context())
	if !ok {
		return domain.Package{}, false
	}

	purchased, ok, err := h.svc.ConfirmCheckout(r.Context(), sessionID, snapshot.UserID)
	if err != nil {
		h.logger.Error("billing: confirm checkout", "err", err)
		return domain.Package{}, false
	}

	return purchased, ok
}

func paymentMethods(stripeReady bool) []state.PaymentMethod {
	return []state.PaymentMethod{
		{Key: providerStripe, Label: "Stripe", Enabled: stripeReady},
		{Key: "paypal", Label: "PayPal", Enabled: false},
		{Key: "crypto", Label: "Crypto", Enabled: false},
	}
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
	provider := r.FormValue(fieldProvider)
	if provider != "" && provider != providerStripe {
		http.Redirect(w, r, "/store?notice=invalid", http.StatusSeeOther)
		return
	}

	successURL := h.appURL + "/store?notice=success&session_id={CHECKOUT_SESSION_ID}"
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
