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

type stubService struct {
	checkoutURL string
	checkoutErr error
	packages    []domain.Package
	available   bool
}

func (s *stubService) Packages() []domain.Package { return s.packages }

func (s *stubService) Available() bool { return s.available }

func (s *stubService) StartCheckout(context.Context, int, string, string, string) (string, error) {
	return s.checkoutURL, s.checkoutErr
}

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
