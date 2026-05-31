package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

func testCatalog() domain.Catalog {
	return domain.NewCatalog([]domain.Package{
		{Key: "starter", Name: "Starter Pack", Currency: "USD", Price: 5, CashPoints: 500},
	})
}

func TestService_StartCheckout_HappyPath(t *testing.T) {
	t.Parallel()

	var createdStatus int
	var setRefID int64
	var setRef string
	repo := &fakeRepo{
		createFn: func(_ context.Context, purchase domain.Purchase) (int64, error) {
			createdStatus = purchase.Status
			return 42, nil
		},
		setProviderRefFn: func(_ context.Context, id int64, ref string) error {
			setRefID = id
			setRef = ref
			return nil
		},
	}
	provider := &fakeProvider{}
	svc := NewService(repo, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	redirectURL, err := svc.StartCheckout(context.Background(), 7, "starter", "https://app.test/ok", "https://app.test/cancel")
	if err != nil {
		t.Fatalf("StartCheckout: %v", err)
	}
	if redirectURL != "https://pay.test/x" {
		t.Errorf("redirectURL = %q, want https://pay.test/x", redirectURL)
	}
	if createdStatus != domain.StatusPending {
		t.Errorf("created status = %d, want %d", createdStatus, domain.StatusPending)
	}
	if setRefID != 42 || setRef != "ref_1" {
		t.Errorf("SetProviderRef(%d, %q), want (42, ref_1)", setRefID, setRef)
	}
	if provider.lastRequest.PurchaseID != 42 || provider.lastRequest.Amount != 5 {
		t.Errorf("checkout request = %+v, want PurchaseID 42 Amount 5", provider.lastRequest)
	}
}

func TestService_StartCheckout_UnknownPackage(t *testing.T) {
	t.Parallel()

	called := false
	repo := &fakeRepo{
		createFn: func(context.Context, domain.Purchase) (int64, error) {
			called = true
			return 1, nil
		},
	}
	svc := NewService(repo, testCatalog(), WithProvider(&fakeProvider{}), WithLogger(discardLogger()))

	_, err := svc.StartCheckout(context.Background(), 7, "missing", "", "")
	if !errors.Is(err, domain.ErrUnknownPackage) {
		t.Fatalf("err = %v, want ErrUnknownPackage", err)
	}
	if called {
		t.Errorf("repo.Create must not be called for unknown package")
	}
}

func TestService_StartCheckout_NoProvider(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepo{}, testCatalog(), WithLogger(discardLogger()))

	_, err := svc.StartCheckout(context.Background(), 7, "starter", "", "")
	if !errors.Is(err, domain.ErrProviderUnavailable) {
		t.Fatalf("err = %v, want ErrProviderUnavailable", err)
	}
}

func TestService_CompletePurchase_CreditsOnMatch(t *testing.T) {
	t.Parallel()

	completed := false
	repo := &fakeRepo{
		completeFn: func(context.Context, int64, string, time.Time) (bool, int, int, error) {
			completed = true
			return true, 7, 500, nil
		},
	}
	svc := NewService(repo, testCatalog(), WithLogger(discardLogger()))

	if err := svc.CompletePurchase(context.Background(), 9, "pay_1"); err != nil {
		t.Fatalf("CompletePurchase: %v", err)
	}
	if !completed {
		t.Errorf("repo.Complete was not called")
	}
}

func TestService_DisputePurchase_BansThenMarks(t *testing.T) {
	t.Parallel()

	marked := false
	repo := &fakeRepo{
		getByPaymentFn: func(context.Context, string, string) (domain.Purchase, error) {
			return domain.Purchase{ID: 9, AccountID: 7, Status: domain.StatusCompleted}, nil
		},
		markDisputedFn: func(context.Context, int64, time.Time) (bool, error) {
			marked = true
			return true, nil
		},
	}
	banner := &fakeBanner{}
	svc := NewService(repo, testCatalog(), WithBanner(banner), WithLogger(discardLogger()))

	if err := svc.DisputePurchase(context.Background(), "fake", "pay_1"); err != nil {
		t.Fatalf("DisputePurchase: %v", err)
	}
	if banner.bannedID != 7 {
		t.Errorf("bannedID = %d, want 7", banner.bannedID)
	}
	if !marked {
		t.Errorf("repo.MarkDisputed was not called")
	}
}

func TestService_DisputePurchase_AlreadyDisputedNoOp(t *testing.T) {
	t.Parallel()

	marked := false
	repo := &fakeRepo{
		getByPaymentFn: func(context.Context, string, string) (domain.Purchase, error) {
			return domain.Purchase{ID: 9, AccountID: 7, Status: domain.StatusDisputed}, nil
		},
		markDisputedFn: func(context.Context, int64, time.Time) (bool, error) {
			marked = true
			return true, nil
		},
	}
	banner := &fakeBanner{}
	svc := NewService(repo, testCatalog(), WithBanner(banner), WithLogger(discardLogger()))

	if err := svc.DisputePurchase(context.Background(), "fake", "pay_1"); err != nil {
		t.Fatalf("DisputePurchase: %v", err)
	}
	if banner.bannedCount != 0 {
		t.Errorf("bannedCount = %d, want 0 for already-disputed redelivery", banner.bannedCount)
	}
	if marked {
		t.Errorf("repo.MarkDisputed must not be called for already-disputed redelivery")
	}
}

func TestService_DisputePurchase_BanErrorAbortsBeforeMark(t *testing.T) {
	t.Parallel()

	marked := false
	repo := &fakeRepo{
		getByPaymentFn: func(context.Context, string, string) (domain.Purchase, error) {
			return domain.Purchase{ID: 9, AccountID: 7, Status: domain.StatusCompleted}, nil
		},
		markDisputedFn: func(context.Context, int64, time.Time) (bool, error) {
			marked = true
			return true, nil
		},
	}
	banErr := errors.New("ban failed")
	banner := &fakeBanner{banErr: banErr}
	svc := NewService(repo, testCatalog(), WithBanner(banner), WithLogger(discardLogger()))

	err := svc.DisputePurchase(context.Background(), "fake", "pay_1")
	if !errors.Is(err, banErr) {
		t.Fatalf("err = %v, want ban error in chain", err)
	}
	if marked {
		t.Errorf("repo.MarkDisputed must not be called when ban fails")
	}
}

func TestService_DisputePurchase_SkipsNonCompleted(t *testing.T) {
	t.Parallel()

	marked := false
	repo := &fakeRepo{
		getByPaymentFn: func(context.Context, string, string) (domain.Purchase, error) {
			return domain.Purchase{ID: 9, AccountID: 7, Status: domain.StatusPending}, nil
		},
		markDisputedFn: func(context.Context, int64, time.Time) (bool, error) {
			marked = true
			return true, nil
		},
	}
	banner := &fakeBanner{}
	svc := NewService(repo, testCatalog(), WithBanner(banner), WithLogger(discardLogger()))

	if err := svc.DisputePurchase(context.Background(), "fake", "pay_1"); err != nil {
		t.Fatalf("DisputePurchase: %v", err)
	}
	if banner.bannedCount != 0 {
		t.Errorf("bannedCount = %d, want 0 for a non-completed purchase", banner.bannedCount)
	}
	if marked {
		t.Errorf("MarkDisputed must not be called for a non-completed purchase")
	}
}

func TestService_RefundPurchase_MarksAndNeverBans(t *testing.T) {
	t.Parallel()

	refunded := false
	repo := &fakeRepo{
		getByPaymentFn: func(context.Context, string, string) (domain.Purchase, error) {
			return domain.Purchase{ID: 9, AccountID: 7, Status: domain.StatusCompleted}, nil
		},
		markRefundedFn: func(context.Context, int64, time.Time) (bool, error) {
			refunded = true
			return true, nil
		},
	}
	banner := &fakeBanner{}
	svc := NewService(repo, testCatalog(), WithBanner(banner), WithLogger(discardLogger()))

	if err := svc.RefundPurchase(context.Background(), "fake", "pay_1"); err != nil {
		t.Fatalf("RefundPurchase: %v", err)
	}
	if !refunded {
		t.Errorf("repo.MarkRefunded was not called")
	}
	if banner.bannedCount != 0 {
		t.Errorf("bannedCount = %d, want 0 for refund", banner.bannedCount)
	}
}

func TestService_FailPurchase_MarksFailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		transitioned bool
	}{
		{name: "real transition", transitioned: true},
		{name: "no transition", transitioned: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotID int64
			repo := &fakeRepo{
				markFailedFn: func(_ context.Context, id int64, _ time.Time) (bool, error) {
					gotID = id
					return tt.transitioned, nil
				},
			}
			svc := NewService(repo, testCatalog(), WithLogger(discardLogger()))

			if err := svc.FailPurchase(context.Background(), 9); err != nil {
				t.Fatalf("FailPurchase: %v", err)
			}
			if gotID != 9 {
				t.Errorf("MarkFailed id = %d, want 9", gotID)
			}
		})
	}
}

func TestService_HistoryByAccount_UsesPaidAndClamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{name: "zero clamps to max", limit: 0, wantLimit: maxHistoryLimit},
		{name: "over max clamps", limit: 5000, wantLimit: maxHistoryLimit},
		{name: "in range passes through", limit: 25, wantLimit: 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotAccountID, gotLimit int
			repo := &fakeRepo{
				listPaidByAccountFn: func(_ context.Context, accountID, limit int) ([]domain.Purchase, error) {
					gotAccountID = accountID
					gotLimit = limit
					return []domain.Purchase{{ID: 1}}, nil
				},
			}
			svc := NewService(repo, testCatalog(), WithLogger(discardLogger()))

			rows, err := svc.HistoryByAccount(context.Background(), 7, tt.limit)
			if err != nil {
				t.Fatalf("HistoryByAccount: %v", err)
			}
			if len(rows) != 1 {
				t.Errorf("rows = %d, want 1", len(rows))
			}
			if gotAccountID != 7 {
				t.Errorf("accountID = %d, want 7", gotAccountID)
			}
			if gotLimit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", gotLimit, tt.wantLimit)
			}
		})
	}
}

func TestService_AdminHistory_OffsetAndTotal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		page       int
		pageSize   int
		wantLimit  int
		wantOffset int
	}{
		{name: "page below one becomes one", page: 0, pageSize: 20, wantLimit: 20, wantOffset: 0},
		{name: "negative page becomes one", page: -3, pageSize: 20, wantLimit: 20, wantOffset: 0},
		{name: "third page offset", page: 3, pageSize: 20, wantLimit: 20, wantOffset: 40},
		{name: "oversized page size clamps", page: 1, pageSize: 5000, wantLimit: maxHistoryLimit, wantOffset: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotLimit, gotOffset int
			repo := &fakeRepo{
				listFilteredFn: func(_ context.Context, _ domain.PurchaseFilter, limit, offset int) ([]domain.Purchase, int, error) {
					gotLimit = limit
					gotOffset = offset
					return []domain.Purchase{{ID: 1}}, 137, nil
				},
			}
			svc := NewService(repo, testCatalog(), WithLogger(discardLogger()))

			rows, total, err := svc.AdminHistory(context.Background(), domain.PurchaseFilter{}, tt.page, tt.pageSize)
			if err != nil {
				t.Fatalf("AdminHistory: %v", err)
			}
			if len(rows) != 1 {
				t.Errorf("rows = %d, want 1", len(rows))
			}
			if total != 137 {
				t.Errorf("total = %d, want 137", total)
			}
			if gotLimit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", gotLimit, tt.wantLimit)
			}
			if gotOffset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", gotOffset, tt.wantOffset)
			}
		})
	}
}

func TestService_Earnings_WindowStarts(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, time.May, 27, 13, 45, 0, 0, time.UTC)
	var gotDay, gotWeek, gotMonth time.Time
	repo := &fakeRepo{
		earningsFn: func(_ context.Context, dayStart, weekStart, monthStart time.Time) (domain.EarningsSummary, error) {
			gotDay = dayStart
			gotWeek = weekStart
			gotMonth = monthStart
			return domain.EarningsSummary{Today: 1, Week: 2, Month: 3, AllTime: 4}, nil
		},
	}
	svc := NewService(repo, testCatalog(),
		WithLogger(discardLogger()),
		WithNow(func() time.Time { return fixedNow }),
		WithLocation(time.UTC),
	)

	summary, err := svc.Earnings(context.Background())
	if err != nil {
		t.Fatalf("Earnings: %v", err)
	}
	if summary.AllTime != 4 {
		t.Errorf("AllTime = %d, want 4", summary.AllTime)
	}

	wantDay := time.Date(2026, time.May, 27, 0, 0, 0, 0, time.UTC)
	wantWeek := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	wantMonth := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	if !gotDay.Equal(wantDay) {
		t.Errorf("dayStart = %s, want %s", gotDay, wantDay)
	}
	if !gotWeek.Equal(wantWeek) {
		t.Errorf("weekStart = %s, want %s (Monday)", gotWeek, wantWeek)
	}
	if !gotMonth.Equal(wantMonth) {
		t.Errorf("monthStart = %s, want %s", gotMonth, wantMonth)
	}
}

func TestService_ConfirmCheckout_PaidAndOwned(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		getByIDFn: func(_ context.Context, id int64) (domain.Purchase, error) {
			return domain.Purchase{ID: id, AccountID: 7, PackageKey: "starter"}, nil
		},
	}
	provider := &fakeProvider{confirm: domain.CheckoutConfirmation{PurchaseID: 9, Paid: true}}
	svc := NewService(repo, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	pkg, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if !ok || pkg.Key != "starter" {
		t.Fatalf("ok=%v pkg=%q, want true/starter", ok, pkg.Key)
	}
}

func TestService_ConfirmCheckout_WrongAccount(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		getByIDFn: func(_ context.Context, id int64) (domain.Purchase, error) {
			return domain.Purchase{ID: id, AccountID: 99, PackageKey: "starter"}, nil
		},
	}
	provider := &fakeProvider{confirm: domain.CheckoutConfirmation{PurchaseID: 9, Paid: true}}
	svc := NewService(repo, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if ok {
		t.Fatal("ok = true for a session owned by another account, want false")
	}
}

func TestService_ConfirmCheckout_NotPaid(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{confirm: domain.CheckoutConfirmation{PurchaseID: 9, Paid: false}}
	svc := NewService(&fakeRepo{}, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if ok {
		t.Fatal("ok = true for an unpaid session, want false")
	}
}

func TestService_ConfirmCheckout_NoProvider(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepo{}, testCatalog(), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if ok {
		t.Fatal("ok = true with no provider, want false")
	}
}

func TestService_ConfirmCheckout_RetrieveError(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{confirmErr: errors.New("stripe down")}
	svc := NewService(&fakeRepo{}, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if ok {
		t.Fatal("ok = true when retrieve failed, want false")
	}
}

func TestService_ConfirmCheckout_PurchaseNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		getByIDFn: func(_ context.Context, _ int64) (domain.Purchase, error) {
			return domain.Purchase{}, domain.ErrPurchaseNotFound
		},
	}
	provider := &fakeProvider{confirm: domain.CheckoutConfirmation{PurchaseID: 9, Paid: true}}
	svc := NewService(repo, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if ok {
		t.Fatal("ok = true for a missing purchase, want false")
	}
}

func TestService_ConfirmCheckout_RepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		getByIDFn: func(_ context.Context, _ int64) (domain.Purchase, error) {
			return domain.Purchase{}, errors.New("db unavailable")
		},
	}
	provider := &fakeProvider{confirm: domain.CheckoutConfirmation{PurchaseID: 9, Paid: true}}
	svc := NewService(repo, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err == nil {
		t.Fatal("ConfirmCheckout err = nil for a repo failure, want non-nil")
	}
	if ok {
		t.Fatal("ok = true for a repo failure, want false")
	}
}

func TestService_ConfirmCheckout_UnknownPackage(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{
		getByIDFn: func(_ context.Context, id int64) (domain.Purchase, error) {
			return domain.Purchase{ID: id, AccountID: 7, PackageKey: "ghost"}, nil
		},
	}
	provider := &fakeProvider{confirm: domain.CheckoutConfirmation{PurchaseID: 9, Paid: true}}
	svc := NewService(repo, testCatalog(), WithProvider(provider), WithLogger(discardLogger()))

	_, ok, err := svc.ConfirmCheckout(context.Background(), "cs_1", 7)
	if err != nil {
		t.Fatalf("ConfirmCheckout: %v", err)
	}
	if ok {
		t.Fatal("ok = true for a package missing from the catalog, want false")
	}
}
