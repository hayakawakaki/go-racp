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
		Methods:   h.paymentMethods(),
		Available: available,
	}
	switch notice {
	case noticeSuccess:
		if purchased, ok := h.confirmPurchase(r); ok {
			st.Purchased = &purchased
		} else {
			st.NotCompleted = true
		}
	case noticeCancel:
		st.NotCompleted = true
	default:
		st.Notice = noticeMessage(notice)
	}

	httpx.RenderHTML(w, r, h.logger, h.theme.StorePage(h.layout(), st))
}

func (h *Handler) confirmPurchase(r *http.Request) (domain.Package, bool) {
	provider := r.URL.Query().Get(fieldProvider)
	if !h.svc.ProviderEnabled(provider) {
		return domain.Package{}, false
	}

	snapshot, ok := middleware.SnapshotFromContext(r.Context())
	if !ok {
		return domain.Package{}, false
	}

	purchased, ok, err := h.svc.ConfirmCheckout(r.Context(), provider, r.URL.Query(), snapshot.UserID)
	if err != nil {
		h.logger.Error("billing: confirm checkout", "err", err)
		return domain.Package{}, false
	}

	return purchased, ok
}

func (h *Handler) paymentMethods() []state.PaymentMethod {
	methods := []state.PaymentMethod{
		{Key: providerStripe, Label: "Stripe", Enabled: h.svc.ProviderEnabled(providerStripe)},
		{Key: "paypal", Label: "PayPal", Enabled: h.svc.ProviderEnabled("paypal")},
		{Key: "crypto", Label: "Crypto", Enabled: h.svc.ProviderEnabled("crypto")},
	}
	for i := range methods {
		if methods[i].Enabled {
			methods[i].Checked = true
			break
		}
	}

	return methods
}

func (h *Handler) startCheckout(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		http.Redirect(w, r, "/store?notice="+noticeUnavailable, http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCheckoutFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/store?notice="+noticeInvalid, http.StatusSeeOther)
		return
	}

	snapshot, ok := middleware.SnapshotFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/store?notice="+noticeInvalid, http.StatusSeeOther)
		return
	}

	packageKey := r.FormValue(fieldPackage)
	provider := r.FormValue(fieldProvider)
	if !h.svc.ProviderEnabled(provider) {
		http.Redirect(w, r, "/store?notice="+noticeInvalid, http.StatusSeeOther)
		return
	}

	successURL := h.appURL + "/store?notice=" + noticeSuccess + "&" + fieldProvider + "=" + provider
	cancelURL := h.appURL + "/store?notice=" + noticeCancel

	redirectURL, err := h.svc.StartCheckout(r.Context(), snapshot.UserID, provider, packageKey, successURL, cancelURL)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnknownPackage):
			http.Redirect(w, r, "/store?notice="+noticeInvalid, http.StatusSeeOther)
		case errors.Is(err, domain.ErrProviderUnavailable):
			http.Redirect(w, r, "/store?notice="+noticeUnavailable, http.StatusSeeOther)
		default:
			h.logger.Error("billing: start checkout", "err", err)
			http.Redirect(w, r, "/store?notice="+noticeInvalid, http.StatusSeeOther)
		}
		return
	}

	h.redirectToCheckout(w, r, redirectURL)
}

func (h *Handler) redirectToCheckout(w http.ResponseWriter, r *http.Request, redirectURL string) {
	parsed, parseErr := url.Parse(redirectURL)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" {
		h.logger.Error("billing: provider returned an unexpected redirect url", "err", parseErr)
		http.Redirect(w, r, "/store?notice="+noticeInvalid, http.StatusSeeOther)
		return
	}

	//nolint:gosec // redirect is by payment provider
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
