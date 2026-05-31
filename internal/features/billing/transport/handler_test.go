package transport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/features/billing/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/server/config"
	billingtpl "github.com/hayakawakaki/go-racp/themes/default/features/billing/transport"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

type stubTheme struct{}

func (stubTheme) StorePage(layout httpx.Layout, st state.StoreState) templ.Component {
	return billingtpl.StorePage(layout, st)
}

func (stubTheme) PurchaseHistoryPage(layout httpx.Layout, st state.PurchaseHistoryState) templ.Component {
	return billingtpl.PurchaseHistoryPage(layout, st)
}

func (stubTheme) PurchaseHistoryContent(st state.PurchaseHistoryState) templ.Component {
	return billingtpl.PurchaseHistoryContent(st)
}

type stubService struct {
	checkoutURL string
	checkoutErr error
	historyErr  error
	packages    []domain.Package
	history     []domain.Purchase
	available   bool
}

func (s *stubService) Packages() []domain.Package { return s.packages }

func (s *stubService) Available() bool { return s.available }

func (s *stubService) StartCheckout(context.Context, int, string, string, string) (string, error) {
	return s.checkoutURL, s.checkoutErr
}

func (s *stubService) HistoryByAccount(context.Context, int, int) ([]domain.Purchase, error) {
	return s.history, s.historyErr
}

func (s *stubService) CompletePurchase(context.Context, int64, string) error {
	return nil
}

func (s *stubService) RefundPurchase(context.Context, string, string) error { return nil }

func (s *stubService) DisputePurchase(context.Context, string, string) error { return nil }

func (s *stubService) FailPurchase(context.Context, int64) error { return nil }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newHandler(svc billingService) *Handler {
	return NewHandler(svc, HandlerConfig{
		Logger:   discardLogger(),
		Theme:    stubTheme{},
		Currency: "USD",
		AppURL:   "https://panel.example.com",
		General:  config.GeneralConfig{ServerName: "Test", Timezone: "UTC"},
	})
}

func TestHandler_ShowStore_Available(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		available: true,
		packages: []domain.Package{
			{Key: "starter", Name: "Starter Pack", Currency: "USD", Price: 5, CashPoints: 500},
		},
	}
	h := newHandler(svc)

	rr := httptest.NewRecorder()
	h.showStore(rr, httptest.NewRequest(http.MethodGet, "/store", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Starter Pack") {
		t.Errorf("body does not contain package name")
	}
}

func TestHandler_ShowStore_Unavailable(t *testing.T) {
	t.Parallel()
	h := newHandler(&stubService{available: false})

	rr := httptest.NewRecorder()
	h.showStore(rr, httptest.NewRequest(http.MethodGet, "/store", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unavailable") {
		t.Errorf("body does not contain unavailable text")
	}
}

func TestHandler_StartCheckout_KnownPackage(t *testing.T) {
	t.Parallel()
	svc := &stubService{checkoutURL: "https://pay.test/session/1"}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/store/checkout", strings.NewReader("package=starter"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 42, Username: "kaki"}))

	rr := httptest.NewRecorder()
	h.startCheckout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "https://pay.test/session/1" {
		t.Errorf("Location = %q, want provider URL", got)
	}
}

func TestHandler_StartCheckout_UnknownPackage(t *testing.T) {
	t.Parallel()
	svc := &stubService{checkoutErr: domain.ErrUnknownPackage}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/store/checkout", strings.NewReader("package=bogus"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 7, Username: "testuser"}))

	rr := httptest.NewRecorder()
	h.startCheckout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/store?notice=invalid" {
		t.Errorf("Location = %q, want /store?notice=invalid", got)
	}
}

func TestHandler_ShowHistory_WithPurchases(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		history: []domain.Purchase{
			{PackageKey: "starter", Amount: 500, CashPoints: 500, Status: domain.StatusCompleted},
			{PackageKey: "bundle", Amount: 1000, CashPoints: 1200, Status: domain.StatusRefunded},
		},
	}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/store/history", http.NoBody)
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 42, Username: "kaki"}))

	rr := httptest.NewRecorder()
	h.showHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "starter") {
		t.Errorf("body does not contain package key")
	}
	if !strings.Contains(body, "Completed") {
		t.Errorf("body does not contain status label")
	}
}

func TestHandler_ShowHistory_HTMXFragment(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		history: []domain.Purchase{
			{PackageKey: "starter", Amount: 500, CashPoints: 500, Status: domain.StatusCompleted},
		},
	}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/store/history", http.NoBody)
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 42, Username: "kaki"}))

	rr := httptest.NewRecorder()
	h.showHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "starter") {
		t.Errorf("fragment does not contain package key")
	}
	if strings.Contains(body, "<html") {
		t.Errorf("HTMX fragment must not include the full page shell")
	}
}

func TestHandler_ShowHistory_Empty(t *testing.T) {
	t.Parallel()
	h := newHandler(&stubService{})

	req := httptest.NewRequest(http.MethodGet, "/store/history", http.NoBody)
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 7, Username: "testuser"}))

	rr := httptest.NewRecorder()
	h.showHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "no purchases yet") {
		t.Errorf("body does not contain empty-state text")
	}
}

func TestHandler_StartCheckout_StripeProvider(t *testing.T) {
	t.Parallel()
	svc := &stubService{checkoutURL: "https://pay.test/session/9"}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/store/checkout", strings.NewReader("package=starter&provider=stripe"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 42, Username: "kaki"}))

	rr := httptest.NewRecorder()
	h.startCheckout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "https://pay.test/session/9" {
		t.Errorf("Location = %q, want provider URL", got)
	}
}

func TestHandler_StartCheckout_UnsupportedProvider(t *testing.T) {
	t.Parallel()
	svc := &stubService{checkoutURL: "https://pay.test/session/9"}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/store/checkout", strings.NewReader("package=starter&provider=paypal"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 42, Username: "kaki"}))

	rr := httptest.NewRecorder()
	h.startCheckout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/store?notice=invalid" {
		t.Errorf("Location = %q, want /store?notice=invalid", got)
	}
}

func TestHandler_StartCheckout_EmptyProviderFallsBack(t *testing.T) {
	t.Parallel()
	svc := &stubService{checkoutURL: "https://pay.test/session/9"}
	h := newHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/store/checkout", strings.NewReader("package=starter"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(middleware.ContextWithSnapshot(req.Context(), &middleware.AccountSnapshot{UserID: 42, Username: "kaki"}))

	rr := httptest.NewRecorder()
	h.startCheckout(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "https://pay.test/session/9" {
		t.Errorf("Location = %q, want provider URL", got)
	}
}

func TestHandler_ShowStore_RendersMethods(t *testing.T) {
	t.Parallel()
	svc := &stubService{
		available: true,
		packages: []domain.Package{
			{Key: "starter", Name: "Starter Pack", Currency: "USD", Price: 5, CashPoints: 500},
		},
	}
	h := newHandler(svc)

	rr := httptest.NewRecorder()
	h.showStore(rr, httptest.NewRequest(http.MethodGet, "/store", http.NoBody))

	body := rr.Body.String()
	if !strings.Contains(body, "Stripe") {
		t.Errorf("body does not contain the stripe method label")
	}
	if !strings.Contains(body, "Coming soon") {
		t.Errorf("body does not contain a coming soon row")
	}
	if !strings.Contains(body, "$5 USD") {
		t.Errorf("body does not contain the sign-and-code price")
	}
}
