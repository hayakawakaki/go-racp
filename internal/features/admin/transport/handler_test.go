package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	currency "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	modstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/moderation/state"
	adminstate "github.com/hayakawakaki/go-racp/internal/features/admin/transport/state"
	billingdomain "github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	guildstate "github.com/hayakawakaki/go-racp/internal/features/guild/transport/state"
	itemapp "github.com/hayakawakaki/go-racp/internal/features/item/app"
	mobapp "github.com/hayakawakaki/go-racp/internal/features/mob/app"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	metricdomain "github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
	accountmoderation "github.com/hayakawakaki/go-racp/themes/default/features/account/transport/moderation"
	admin "github.com/hayakawakaki/go-racp/themes/default/features/admin/transport"
	guildtpl "github.com/hayakawakaki/go-racp/themes/default/features/guild/transport"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

type stubTheme struct{}

func (stubTheme) AdminLayout(layout httpx.Layout, pageTitle string, content templ.Component) templ.Component {
	return admin.AdminLayout(layout, pageTitle, content)
}

func (stubTheme) DashboardContent(state adminstate.DashboardState) templ.Component {
	return admin.DashboardContent(state)
}

func (stubTheme) DatabaseContent(state adminstate.DatabaseState) templ.Component {
	return admin.DatabaseContent(state)
}

func (stubTheme) UsersListPage(layout httpx.Layout, state modstate.ListState) templ.Component {
	return accountmoderation.UsersListPage(layout, state)
}

func (stubTheme) UsersListContent(state modstate.ListState) templ.Component {
	return accountmoderation.UsersListContent(state)
}

func (stubTheme) GuildListPage(layout httpx.Layout, state guildstate.ListState) templ.Component {
	return guildtpl.GuildListPage(layout, state)
}

func (stubTheme) GuildListContent(state guildstate.ListState) templ.Component {
	return guildtpl.GuildListContent(state)
}

func (stubTheme) EconomyContent(state adminstate.EconomyState) templ.Component {
	return admin.EconomyContent(state)
}

type stubEconomyReader struct {
	totalsFn    func(context.Context) (currency.TotalsDTO, error)
	depositsFn  func(context.Context, int, int) (currency.DepositPage, error)
	withdrawsFn func(context.Context, int, int) (currency.WithdrawHistoryPage, error)
	stuckFn     func(context.Context) ([]currency.AdminWithdrawDTO, error)
}

func (s *stubEconomyReader) Totals(ctx context.Context) (currency.TotalsDTO, error) {
	if s.totalsFn != nil {
		return s.totalsFn(ctx)
	}
	return currency.TotalsDTO{}, nil
}

func (s *stubEconomyReader) DepositHistory(ctx context.Context, page, perPage int) (currency.DepositPage, error) {
	if s.depositsFn != nil {
		return s.depositsFn(ctx, page, perPage)
	}
	return currency.DepositPage{}, nil
}

func (s *stubEconomyReader) WithdrawHistory(ctx context.Context, page, perPage int) (currency.WithdrawHistoryPage, error) {
	if s.withdrawsFn != nil {
		return s.withdrawsFn(ctx, page, perPage)
	}
	return currency.WithdrawHistoryPage{}, nil
}

func (s *stubEconomyReader) StuckWithdraws(ctx context.Context) ([]currency.AdminWithdrawDTO, error) {
	if s.stuckFn != nil {
		return s.stuckFn(ctx)
	}
	return nil, nil
}

type stubPurchasesReader struct {
	historyFn  func(context.Context, billingdomain.PurchaseFilter, int, int) ([]billingdomain.Purchase, int, error)
	earningsFn func(context.Context) (billingdomain.EarningsSummary, error)
}

func (s *stubPurchasesReader) AdminHistory(ctx context.Context, filter billingdomain.PurchaseFilter, page, pageSize int) ([]billingdomain.Purchase, int, error) {
	if s.historyFn != nil {
		return s.historyFn(ctx, filter, page, pageSize)
	}
	return nil, 0, nil
}

func (s *stubPurchasesReader) Earnings(ctx context.Context) (billingdomain.EarningsSummary, error) {
	if s.earningsFn != nil {
		return s.earningsFn(ctx)
	}
	return billingdomain.EarningsSummary{}, nil
}

type stubMetric struct {
	peaksFn func(context.Context) ([]metricdomain.PeakRow, error)
}

func (s *stubMetric) Online(context.Context) metricdomain.OnlineSnapshot {
	return metricdomain.OnlineSnapshot{}
}

func (s *stubMetric) Peaks(ctx context.Context) ([]metricdomain.PeakRow, error) {
	if s.peaksFn != nil {
		return s.peaksFn(ctx)
	}
	return nil, nil
}

func (s *stubMetric) General(context.Context) (metricdomain.GeneralSnapshot, error) {
	return metricdomain.GeneralSnapshot{}, nil
}

type stubEmailResolver struct {
	emailsFn func(context.Context, []int) (map[int]string, error)
}

func (s *stubEmailResolver) EmailsByIDs(ctx context.Context, ids []int) (map[int]string, error) {
	if s.emailsFn != nil {
		return s.emailsFn(ctx, ids)
	}
	return map[int]string{}, nil
}

type stubItemStatus struct {
	status itemapp.ServiceStatus
}

func (s *stubItemStatus) Status() itemapp.ServiceStatus { return s.status }

type stubMobStatus struct {
	status mobapp.ServiceStatus
}

func (s *stubMobStatus) Status() mobapp.ServiceStatus { return s.status }

type stubSession struct {
	validateFn func(context.Context, string) (*accdomain.Session, error)
}

func (s *stubSession) Validate(ctx context.Context, token string) (*accdomain.Session, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, token)
	}
	return nil, accdomain.ErrSessionNotFound
}

func (s *stubSession) Destroy(_ context.Context, _ string) error {
	return nil
}

func (s *stubSession) TTL() time.Duration { return time.Hour }

type stubUsers struct {
	getFn func(context.Context, int) (*accdomain.User, error)
}

func (s *stubUsers) GetByID(ctx context.Context, id int) (*accdomain.User, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return &accdomain.User{ID: id}, nil
}

func newStubSession() *stubSession  { return &stubSession{} }
func newStubUserLookup() *stubUsers { return &stubUsers{} }

func newTestHandler() *Handler {
	return NewHandler(HandlerConfig{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		General:    config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		ItemStatus: &stubItemStatus{status: itemapp.ServiceStatus{ItemsLoaded: 42, LastReload: "2026-05-18T00:00:00Z"}},
		MobStatus:  &stubMobStatus{status: mobapp.ServiceStatus{MobsLoaded: 7, LastReload: "2026-05-18T00:00:00Z"}},
		Purchases:  &stubPurchasesReader{},
		Theme:      stubTheme{},
	})
}

func TestHandler_ShowDashboard_FullPage(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<title>Test CP / Admin / Dashboard</title>") {
		t.Errorf("full page must include layout title; body:\n%s", body)
	}
	if !strings.Contains(body, `x-data="adminDashboard"`) {
		t.Errorf("full page must include dashboard content; body:\n%s", body)
	}
	if !strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("full page must include admin layout shell; body:\n%s", body)
	}
}

func TestHandler_ShowDashboard_HTMXFragment(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.Header.Set("HX-Request", "true")
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `x-data="adminDashboard"`) {
		t.Errorf("HTMX fragment must include dashboard content; body:\n%s", body)
	}
	if strings.Contains(body, "<title>") {
		t.Errorf("HTMX fragment must not include layout chrome; body:\n%s", body)
	}
	if strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("HTMX fragment must not include admin shell; body:\n%s", body)
	}
}

func TestHandler_RegisterRoutes_WrapsAdminRouteInRegistry(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	reg := routes.NewRegistry(
		config.AccessConfig{},
		nil,
		accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2}),
		newStubSession(),
		newStubUserLookup(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false,
		true,
		httpx.Layout{},
	)
	mux := http.NewServeMux()
	h.RegisterRoutes(reg, mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("anonymous to admin must 404 (hidden); status = %d", rr.Code)
	}
}

func TestHandler_RegisterRoutes_RejectsNonGet(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	reg := routes.NewRegistry(
		config.AccessConfig{},
		nil,
		accdomain.NewRoleResolver(config.RolesConfig{"Moderator": 20, "Enforcer": 10, "Event": 2}),
		newStubSession(),
		newStubUserLookup(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		false,
		true,
		httpx.Layout{},
	)
	mux := http.NewServeMux()
	h.RegisterRoutes(reg, mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandler_ShowEconomy(t *testing.T) {
	t.Parallel()

	gotDepositPage := 0
	economy := &stubEconomyReader{
		totalsFn: func(context.Context) (currency.TotalsDTO, error) {
			return currency.TotalsDTO{Zeny: 100, Cashpoint: 10}, nil
		},
		depositsFn: func(_ context.Context, page, _ int) (currency.DepositPage, error) {
			gotDepositPage = page
			return currency.DepositPage{Page: page, PerPage: 15}, nil
		},
	}
	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Economy: economy,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/economy?dpage=2&wpage=1", http.NoBody)
	h.showEconomy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotDepositPage != 2 {
		t.Errorf("deposit page = %d, want 2", gotDepositPage)
	}
}

func TestHandler_ShowEconomy_ResolvesEmails(t *testing.T) {
	t.Parallel()

	economy := &stubEconomyReader{
		depositsFn: func(_ context.Context, page, _ int) (currency.DepositPage, error) {
			return currency.DepositPage{
				Rows: []currency.DepositDTO{{DepositID: 1, AccountID: 7, Zeny: 100}},
				Page: page,
			}, nil
		},
		withdrawsFn: func(_ context.Context, page, _ int) (currency.WithdrawHistoryPage, error) {
			return currency.WithdrawHistoryPage{
				Rows: []currency.AdminWithdrawDTO{{ID: 1, AccountID: 9, Zeny: 50}},
				Page: page,
			}, nil
		},
	}
	emails := &stubEmailResolver{
		emailsFn: func(_ context.Context, _ []int) (map[int]string, error) {
			return map[int]string{7: "a@example.com", 9: "b@example.com"}, nil
		},
	}
	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Economy: economy,
		Emails:  emails,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/economy", http.NoBody)
	h.showEconomy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "a@example.com") || !strings.Contains(body, "b@example.com") {
		t.Errorf("body missing resolved emails:\n%s", body)
	}
}

func TestHandler_ShowEconomy_BlankEmailShowsPlaceholder(t *testing.T) {
	t.Parallel()

	economy := &stubEconomyReader{
		depositsFn: func(_ context.Context, page, _ int) (currency.DepositPage, error) {
			return currency.DepositPage{
				Rows: []currency.DepositDTO{{DepositID: 1, AccountID: 7, Zeny: 100}},
				Page: page,
			}, nil
		},
		withdrawsFn: func(_ context.Context, page, _ int) (currency.WithdrawHistoryPage, error) {
			return currency.WithdrawHistoryPage{
				Rows: []currency.AdminWithdrawDTO{{ID: 1, AccountID: 9, Zeny: 50}},
				Page: page,
			}, nil
		},
	}
	emails := &stubEmailResolver{
		emailsFn: func(_ context.Context, _ []int) (map[int]string, error) {
			return map[int]string{7: "kaki@example.com"}, nil
		},
	}
	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Economy: economy,
		Emails:  emails,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/economy", http.NoBody)
	h.showEconomy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "kaki@example.com") {
		t.Errorf("resolved deposit email must still render:\n%s", body)
	}
	if !strings.Contains(body, "No email") {
		t.Errorf("an account with no email on file must render the placeholder:\n%s", body)
	}
}

func TestHandler_ShowEconomy_PartialReadFailure(t *testing.T) {
	t.Parallel()

	economy := &stubEconomyReader{
		totalsFn: func(context.Context) (currency.TotalsDTO, error) {
			return currency.TotalsDTO{}, errors.New("totals db down")
		},
		depositsFn: func(_ context.Context, page, _ int) (currency.DepositPage, error) {
			return currency.DepositPage{
				Rows: []currency.DepositDTO{{DepositID: 1, AccountID: 7, Zeny: 100}},
				Page: page,
			}, nil
		},
		withdrawsFn: func(_ context.Context, page, _ int) (currency.WithdrawHistoryPage, error) {
			return currency.WithdrawHistoryPage{
				Rows: []currency.AdminWithdrawDTO{{ID: 1, AccountID: 9, Zeny: 50}},
				Page: page,
			}, nil
		},
	}
	emails := &stubEmailResolver{
		emailsFn: func(_ context.Context, _ []int) (map[int]string, error) {
			return map[int]string{7: "kaki@example.invalid", 9: "crazyarashi@example.invalid"}, nil
		},
	}
	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Economy: economy,
		Emails:  emails,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/economy", http.NoBody)
	h.showEconomy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "kaki@example.invalid") || !strings.Contains(body, "crazyarashi@example.invalid") {
		t.Errorf("a failed totals read must not blank the deposit/withdraw tables:\n%s", body)
	}
	if !strings.Contains(body, "Unavailable") {
		t.Errorf("a failed totals read must surface an unavailable marker, not 0:\n%s", body)
	}
	if strings.Contains(body, "Unable to load this right now.") {
		t.Errorf("deposit/withdraw tables succeeded and must not show the table unavailable snippet:\n%s", body)
	}
}

func TestHandler_ShowDashboard_PeaksReadFailure(t *testing.T) {
	t.Parallel()

	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Metric: &stubMetric{
			peaksFn: func(context.Context) ([]metricdomain.PeakRow, error) {
				return nil, errors.New("peaks db down")
			},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Unable to load this right now.") {
		t.Errorf("failed peaks read must surface the unavailable snippet:\n%s", body)
	}
	if strings.Contains(body, "No peaks recorded yet.") {
		t.Errorf("failed peaks read must not look like genuinely empty:\n%s", body)
	}
}

func TestHandler_ShowEconomy_RendersStuckWithdraws(t *testing.T) {
	t.Parallel()

	economy := &stubEconomyReader{
		stuckFn: func(context.Context) ([]currency.AdminWithdrawDTO, error) {
			return []currency.AdminWithdrawDTO{{ID: 1, AccountID: 7, Zeny: 100, Status: 2}}, nil
		},
	}
	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Economy: economy,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/economy", http.NoBody)
	h.showEconomy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Stuck withdrawals") {
		t.Errorf("body must include the stuck-withdrawals section:\n%s", rr.Body.String())
	}
}

func TestHandler_ShowEconomy_StuckReadFailure(t *testing.T) {
	t.Parallel()

	economy := &stubEconomyReader{
		stuckFn: func(context.Context) ([]currency.AdminWithdrawDTO, error) {
			return nil, errors.New("stuck db down")
		},
	}
	h := NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
		Theme:   stubTheme{},
		Economy: economy,
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/economy", http.NoBody)
	h.showEconomy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Stuck withdrawals") {
		t.Errorf("a failed stuck read must still render the stuck section header:\n%s", body)
	}
	if !strings.Contains(body, "Unable to load this right now.") {
		t.Errorf("a failed stuck read must surface the unavailable marker, not look healthy:\n%s", body)
	}
}

func TestParsePurchaseFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		query        url.Values
		wantProvider string
		wantStatus   int
		wantAccount  int
	}{
		{
			name:  "empty query yields empty filter",
			query: url.Values{},
		},
		{
			name:       "valid status is kept",
			query:      url.Values{"status": {"3"}},
			wantStatus: 3,
		},
		{
			name:  "out-of-range status is ignored",
			query: url.Values{"status": {"9"}},
		},
		{
			name:  "zero status is ignored",
			query: url.Values{"status": {"0"}},
		},
		{
			name:  "non-numeric status is ignored",
			query: url.Values{"status": {"abc"}},
		},
		{
			name:        "valid account is kept",
			query:       url.Values{"account": {"42"}},
			wantAccount: 42,
		},
		{
			name:  "negative account is ignored",
			query: url.Values{"account": {"-3"}},
		},
		{
			name:         "provider is trimmed",
			query:        url.Values{"provider": {"  stripe  "}},
			wantProvider: "stripe",
		},
		{
			name:  "invalid date is ignored",
			query: url.Values{"from": {"nope"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filter, form := parsePurchaseFilter(tt.query, time.UTC)

			if filter.Status != tt.wantStatus {
				t.Errorf("Status = %d, want %d", filter.Status, tt.wantStatus)
			}
			if filter.AccountID != tt.wantAccount {
				t.Errorf("AccountID = %d, want %d", filter.AccountID, tt.wantAccount)
			}
			if filter.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", filter.Provider, tt.wantProvider)
			}
			if filter.From != nil || filter.To != nil {
				t.Errorf("From/To = %v/%v, want nil/nil for a dateless query", filter.From, filter.To)
			}
			if form.Status != tt.query.Get("status") {
				t.Errorf("form.Status = %q, want raw %q", form.Status, tt.query.Get("status"))
			}
		})
	}
}

func TestParsePurchaseFilter_DateRangeUsesLocation(t *testing.T) {
	t.Parallel()

	tokyo := time.FixedZone("UTC+9", 9*60*60)
	filter, _ := parsePurchaseFilter(url.Values{"from": {"2026-05-30"}, "to": {"2026-05-31"}}, tokyo)

	wantFrom := time.Date(2026, 5, 30, 0, 0, 0, 0, tokyo)
	wantTo := time.Date(2026, 6, 1, 0, 0, 0, 0, tokyo)

	if filter.From == nil || !filter.From.Equal(wantFrom) {
		t.Errorf("From = %v, want %v (midnight in the configured location)", filter.From, wantFrom)
	}
	if filter.To == nil || !filter.To.Equal(wantTo) {
		t.Errorf("To = %v, want %v (next-day midnight, half-open)", filter.To, wantTo)
	}
	if filter.From != nil && filter.From.Equal(time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("From was parsed as UTC midnight instead of the configured location")
	}
}

func TestPurchasesHrefPattern(t *testing.T) {
	t.Parallel()

	got := purchasesHrefPattern(url.Values{
		"status":  {"2"},
		"account": {"42"},
		"from":    {"2026-05-01"},
		"dpage":   {"3"},
		"ppage":   {"5"},
	})

	const prefix = "/admin/economy?"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("href = %q, want prefix %q", got, prefix)
	}

	parsed, err := url.ParseQuery(strings.TrimPrefix(got, prefix))
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", got, err)
	}
	if parsed.Get("ppage") != "__PAGE__" {
		t.Errorf("ppage = %q, want the __PAGE__ placeholder (input ppage must be overwritten)", parsed.Get("ppage"))
	}

	for _, want := range []struct{ key, value string }{
		{"status", "2"}, {"account", "42"}, {"from", "2026-05-01"}, {"dpage", "3"},
	} {
		if parsed.Get(want.key) != want.value {
			t.Errorf("%s = %q, want %q", want.key, parsed.Get(want.key), want.value)
		}
	}
	if _, ok := parsed["to"]; ok {
		t.Errorf("empty filter keys must be omitted, but 'to' is present")
	}
}
