package app

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeRepo struct {
	createFn            func(ctx context.Context, purchase domain.Purchase) (int64, error)
	setProviderRefFn    func(ctx context.Context, id int64, ref string) error
	getByIDFn           func(ctx context.Context, id int64) (domain.Purchase, error)
	getByPaymentFn      func(ctx context.Context, provider, paymentID string) (domain.Purchase, error)
	completeFn          func(ctx context.Context, id int64, providerPaymentID string, now time.Time) (bool, int, int, error)
	markDisputedFn      func(ctx context.Context, id int64, now time.Time) (bool, error)
	markRefundedFn      func(ctx context.Context, id int64, now time.Time) (bool, error)
	markFailedFn        func(ctx context.Context, id int64, now time.Time) (bool, error)
	listPaidByAccountFn func(ctx context.Context, accountID, limit int) ([]domain.Purchase, error)
	listFilteredFn      func(ctx context.Context, filter domain.PurchaseFilter, limit, offset int) ([]domain.Purchase, int, error)
	earningsFn          func(ctx context.Context, dayStart, weekStart, monthStart time.Time) (domain.EarningsSummary, error)
}

var _ Repository = (*fakeRepo)(nil)

func (f *fakeRepo) Create(ctx context.Context, purchase domain.Purchase) (int64, error) {
	if f.createFn != nil {
		return f.createFn(ctx, purchase)
	}

	return 1, nil
}

func (f *fakeRepo) SetProviderRef(ctx context.Context, id int64, ref string) error {
	if f.setProviderRefFn != nil {
		return f.setProviderRefFn(ctx, id, ref)
	}

	return nil
}

func (f *fakeRepo) GetByID(ctx context.Context, id int64) (domain.Purchase, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}

	return domain.Purchase{}, nil
}

func (f *fakeRepo) GetByPaymentID(ctx context.Context, provider, paymentID string) (domain.Purchase, error) {
	if f.getByPaymentFn != nil {
		return f.getByPaymentFn(ctx, provider, paymentID)
	}

	return domain.Purchase{}, nil
}

func (f *fakeRepo) Complete(ctx context.Context, id int64, providerPaymentID string, now time.Time) (credited bool, accountID, cashPoints int, err error) {
	if f.completeFn != nil {
		return f.completeFn(ctx, id, providerPaymentID, now)
	}

	return true, 0, 0, nil
}

func (f *fakeRepo) MarkDisputed(ctx context.Context, id int64, now time.Time) (bool, error) {
	if f.markDisputedFn != nil {
		return f.markDisputedFn(ctx, id, now)
	}

	return true, nil
}

func (f *fakeRepo) MarkRefunded(ctx context.Context, id int64, now time.Time) (bool, error) {
	if f.markRefundedFn != nil {
		return f.markRefundedFn(ctx, id, now)
	}

	return true, nil
}

func (f *fakeRepo) MarkFailed(ctx context.Context, id int64, now time.Time) (bool, error) {
	if f.markFailedFn != nil {
		return f.markFailedFn(ctx, id, now)
	}

	return true, nil
}

func (f *fakeRepo) ListPaidByAccount(ctx context.Context, accountID, limit int) ([]domain.Purchase, error) {
	if f.listPaidByAccountFn != nil {
		return f.listPaidByAccountFn(ctx, accountID, limit)
	}

	return nil, nil
}

func (f *fakeRepo) ListFiltered(ctx context.Context, filter domain.PurchaseFilter, limit, offset int) ([]domain.Purchase, int, error) {
	if f.listFilteredFn != nil {
		return f.listFilteredFn(ctx, filter, limit, offset)
	}

	return nil, 0, nil
}

func (f *fakeRepo) Earnings(ctx context.Context, dayStart, weekStart, monthStart time.Time) (domain.EarningsSummary, error) {
	if f.earningsFn != nil {
		return f.earningsFn(ctx, dayStart, weekStart, monthStart)
	}

	return domain.EarningsSummary{}, nil
}

type fakeProvider struct {
	createErr   error
	lastRequest domain.CheckoutRequest
}

var _ domain.Provider = (*fakeProvider)(nil)

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) CreateCheckout(_ context.Context, request domain.CheckoutRequest) (domain.CheckoutResult, error) {
	f.lastRequest = request
	if f.createErr != nil {
		return domain.CheckoutResult{}, f.createErr
	}

	return domain.CheckoutResult{RedirectURL: "https://pay.test/x", Reference: "ref_1"}, nil
}

type fakeBanner struct {
	banErr      error
	bannedID    int
	bannedCount int
}

var _ AccountBanner = (*fakeBanner)(nil)

func (f *fakeBanner) BanForChargeback(_ context.Context, accountID int, _ string) error {
	f.bannedCount++
	if f.banErr != nil {
		return f.banErr
	}
	f.bannedID = accountID

	return nil
}
