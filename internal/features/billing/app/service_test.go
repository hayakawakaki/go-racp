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
		getByIDFn: func(context.Context, int64) (domain.Purchase, error) {
			return domain.Purchase{ID: 9, AccountID: 7, Amount: 5, Currency: "USD", CashPoints: 500}, nil
		},
		completeFn: func(context.Context, int64, string, time.Time) (bool, int, int, error) {
			completed = true
			return true, 7, 500, nil
		},
	}
	svc := NewService(repo, testCatalog(), WithLogger(discardLogger()))

	if err := svc.CompletePurchase(context.Background(), 9, "pay_1", 5, "usd"); err != nil {
		t.Fatalf("CompletePurchase: %v", err)
	}
	if !completed {
		t.Errorf("repo.Complete was not called")
	}
}

func TestService_CompletePurchase_AmountMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		paidCurrency string
		paidAmount   int64
	}{
		{name: "amount mismatch", paidCurrency: "USD", paidAmount: 4},
		{name: "currency mismatch", paidCurrency: "EUR", paidAmount: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			repo := &fakeRepo{
				getByIDFn: func(context.Context, int64) (domain.Purchase, error) {
					return domain.Purchase{ID: 9, AccountID: 7, Amount: 5, Currency: "USD", CashPoints: 500}, nil
				},
				completeFn: func(context.Context, int64, string, time.Time) (bool, int, int, error) {
					called = true
					return true, 7, 500, nil
				},
			}
			svc := NewService(repo, testCatalog(), WithLogger(discardLogger()))

			err := svc.CompletePurchase(context.Background(), 9, "pay_1", tt.paidAmount, tt.paidCurrency)
			if !errors.Is(err, domain.ErrAmountMismatch) {
				t.Fatalf("err = %v, want ErrAmountMismatch", err)
			}
			if called {
				t.Errorf("repo.Complete must not be called on mismatch")
			}
		})
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
