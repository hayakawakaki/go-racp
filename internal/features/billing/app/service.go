package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

const maxHistoryLimit = 100

type Repository interface {
	Create(ctx context.Context, purchase domain.Purchase) (int64, error)
	SetProviderRef(ctx context.Context, id int64, ref string) error
	GetByID(ctx context.Context, id int64) (domain.Purchase, error)
	GetByPaymentID(ctx context.Context, provider, paymentID string) (domain.Purchase, error)
	Complete(ctx context.Context, id int64, providerPaymentID string, now time.Time) (credited bool, accountID, cashPoints int, err error)
	MarkDisputed(ctx context.Context, id int64, now time.Time) (transitioned bool, err error)
	MarkRefunded(ctx context.Context, id int64, now time.Time) (transitioned bool, err error)
	MarkFailed(ctx context.Context, id int64, now time.Time) (transitioned bool, err error)
	ListByAccount(ctx context.Context, accountID, limit int) ([]domain.Purchase, error)
	ListRecent(ctx context.Context, limit int) ([]domain.Purchase, error)
}

type AccountBanner interface {
	BanForChargeback(ctx context.Context, accountID int, reason string) error
}

type Service struct {
	repo     Repository
	provider domain.Provider
	banner   AccountBanner
	logger   *slog.Logger
	now      func() time.Time
	catalog  domain.Catalog
}

type Option func(*Service)

func WithProvider(provider domain.Provider) Option { return func(s *Service) { s.provider = provider } }
func WithBanner(banner AccountBanner) Option       { return func(s *Service) { s.banner = banner } }
func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}
func WithNow(fn func() time.Time) Option {
	return func(s *Service) {
		if fn != nil {
			s.now = fn
		}
	}
}

func NewService(repo Repository, catalog domain.Catalog, opts ...Option) *Service {
	s := &Service{repo: repo, catalog: catalog, now: time.Now, logger: slog.Default()}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Packages() []domain.Package { return s.catalog.List() }

func (s *Service) StartCheckout(ctx context.Context, accountID int, packageKey, successURL, cancelURL string) (string, error) {
	pkg, ok := s.catalog.Lookup(packageKey)
	if !ok {
		return "", domain.ErrUnknownPackage
	}
	if s.provider == nil {
		return "", domain.ErrProviderUnavailable
	}

	now := s.now()
	purchase := domain.Purchase{
		AccountID:  accountID,
		PackageKey: pkg.Key,
		Provider:   s.provider.Name(),
		Amount:     pkg.Price,
		Currency:   pkg.Currency,
		CashPoints: pkg.CashPoints,
		Status:     domain.StatusPending,
		CreatedAt:  now,
	}

	id, err := s.repo.Create(ctx, purchase)
	if err != nil {
		return "", fmt.Errorf("billing.Service.StartCheckout: %w", err)
	}

	result, err := s.provider.CreateCheckout(ctx, domain.CheckoutRequest{
		PurchaseID:  id,
		PackageKey:  pkg.Key,
		Description: pkg.Name,
		Amount:      pkg.Price,
		Currency:    pkg.Currency,
		SuccessURL:  successURL,
		CancelURL:   cancelURL,
	})
	if err != nil {
		return "", fmt.Errorf("billing.Service.StartCheckout: %w", err)
	}

	if err := s.repo.SetProviderRef(ctx, id, result.Reference); err != nil {
		return "", fmt.Errorf("billing.Service.StartCheckout: %w", err)
	}

	return result.RedirectURL, nil
}

func (s *Service) CompletePurchase(ctx context.Context, purchaseID int64, providerPaymentID string, paidAmount int64, paidCurrency string) error {
	purchase, err := s.repo.GetByID(ctx, purchaseID)
	if err != nil {
		return fmt.Errorf("billing.Service.CompletePurchase: %w", err)
	}
	if paidAmount != purchase.Amount || !strings.EqualFold(paidCurrency, purchase.Currency) {
		s.logger.Error("billing: paid amount mismatch, refusing credit",
			"purchase_id", purchaseID,
			"account_id", purchase.AccountID,
			"expected_amount", purchase.Amount,
			"paid_amount", paidAmount,
			"expected_currency", purchase.Currency,
			"paid_currency", paidCurrency,
		)
		return fmt.Errorf("billing.Service.CompletePurchase: %w", domain.ErrAmountMismatch)
	}

	credited, accountID, cashPoints, err := s.repo.Complete(ctx, purchaseID, providerPaymentID, s.now())
	if err != nil {
		return fmt.Errorf("billing.Service.CompletePurchase: %w", err)
	}
	if credited {
		s.logger.Info("billing: purchase credited",
			"purchase_id", purchaseID,
			"account_id", accountID,
			"cash_points", cashPoints,
			"amount", purchase.Amount,
			"currency", purchase.Currency,
			"provider_payment_id", providerPaymentID,
		)
	}

	return nil
}

func (s *Service) DisputePurchase(ctx context.Context, provider, providerPaymentID string) error {
	purchase, err := s.repo.GetByPaymentID(ctx, provider, providerPaymentID)
	if err != nil {
		return fmt.Errorf("billing.Service.DisputePurchase: %w", err)
	}
	if purchase.Status == domain.StatusDisputed {
		return nil
	}

	if s.banner != nil {
		if err := s.banner.BanForChargeback(ctx, purchase.AccountID, "payment chargeback"); err != nil {
			return fmt.Errorf("billing.Service.DisputePurchase: %w", err)
		}
	}

	if _, err := s.repo.MarkDisputed(ctx, purchase.ID, s.now()); err != nil {
		return fmt.Errorf("billing.Service.DisputePurchase: %w", err)
	}

	return nil
}

func (s *Service) RefundPurchase(ctx context.Context, provider, providerPaymentID string) error {
	purchase, err := s.repo.GetByPaymentID(ctx, provider, providerPaymentID)
	if err != nil {
		return fmt.Errorf("billing.Service.RefundPurchase: %w", err)
	}

	transitioned, err := s.repo.MarkRefunded(ctx, purchase.ID, s.now())
	if err != nil {
		return fmt.Errorf("billing.Service.RefundPurchase: %w", err)
	}
	if transitioned {
		s.logger.Info("billing: purchase refunded",
			"purchase_id", purchase.ID,
			"account_id", purchase.AccountID,
			"provider_payment_id", providerPaymentID,
		)
	}

	return nil
}

func (s *Service) FailPurchase(ctx context.Context, purchaseID int64) error {
	transitioned, err := s.repo.MarkFailed(ctx, purchaseID, s.now())
	if err != nil {
		return fmt.Errorf("billing.Service.FailPurchase: %w", err)
	}
	if transitioned {
		s.logger.Info("billing: purchase failed", "purchase_id", purchaseID)
	}

	return nil
}

func (s *Service) HistoryByAccount(ctx context.Context, accountID, limit int) ([]domain.Purchase, error) {
	rows, err := s.repo.ListByAccount(ctx, accountID, clampHistoryLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("billing.Service.HistoryByAccount: %w", err)
	}

	return rows, nil
}

func (s *Service) RecentForAdmin(ctx context.Context, limit int) ([]domain.Purchase, error) {
	rows, err := s.repo.ListRecent(ctx, clampHistoryLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("billing.Service.RecentForAdmin: %w", err)
	}

	return rows, nil
}

func clampHistoryLimit(limit int) int {
	if limit <= 0 || limit > maxHistoryLimit {
		return maxHistoryLimit
	}

	return limit
}
