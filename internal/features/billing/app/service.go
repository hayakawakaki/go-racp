package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	ListPaidByAccount(ctx context.Context, accountID, limit int) ([]domain.Purchase, error)
	ListFiltered(ctx context.Context, filter domain.PurchaseFilter, limit, offset int) (rows []domain.Purchase, total int, err error)
	Earnings(ctx context.Context, dayStart, weekStart, monthStart time.Time) (domain.EarningsSummary, error)
}

type AccountBanner interface {
	BanForChargeback(ctx context.Context, accountID int, reason string) error
}

type Service struct {
	repo     Repository
	provider domain.Provider
	banner   AccountBanner
	logger   *slog.Logger
	loc      *time.Location
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
func WithLocation(loc *time.Location) Option {
	return func(s *Service) {
		if loc != nil {
			s.loc = loc
		}
	}
}

func NewService(repo Repository, catalog domain.Catalog, opts ...Option) *Service {
	s := &Service{repo: repo, catalog: catalog, now: time.Now, loc: time.UTC, logger: slog.Default()}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Packages() []domain.Package { return s.catalog.List() }

func (s *Service) Available() bool { return s.provider != nil }

func (s *Service) SetProvider(provider domain.Provider) { s.provider = provider }

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

func (s *Service) ConfirmCheckout(ctx context.Context, sessionID string, accountID int) (domain.Package, bool, error) {
	if s.provider == nil {
		return domain.Package{}, false, nil
	}

	confirmation, err := s.provider.RetrieveCheckout(ctx, sessionID)
	if err != nil {
		s.logger.Warn("billing: confirm checkout retrieve failed", "session_id", sessionID, "err", err)
		return domain.Package{}, false, nil
	}
	if !confirmation.Paid {
		return domain.Package{}, false, nil
	}

	purchase, err := s.repo.GetByID(ctx, confirmation.PurchaseID)
	if err != nil {
		if errors.Is(err, domain.ErrPurchaseNotFound) {
			return domain.Package{}, false, nil
		}
		return domain.Package{}, false, fmt.Errorf("billing.Service.ConfirmCheckout: %w", err)
	}
	if purchase.AccountID != accountID {
		return domain.Package{}, false, nil
	}

	pkg, ok := s.catalog.Lookup(purchase.PackageKey)
	if !ok {
		return domain.Package{}, false, nil
	}

	return pkg, true, nil
}

func (s *Service) CompletePurchase(ctx context.Context, purchaseID int64, providerPaymentID string) error {
	credited, accountID, cashPoints, err := s.repo.Complete(ctx, purchaseID, providerPaymentID, s.now())
	if err != nil {
		return fmt.Errorf("billing.Service.CompletePurchase: %w", err)
	}
	if credited {
		s.logger.Info("billing: purchase credited",
			"purchase_id", purchaseID,
			"account_id", accountID,
			"cash_points", cashPoints,
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
	if purchase.Status != domain.StatusCompleted {
		s.logger.Warn("billing: dispute on non-completed purchase, skipping ban",
			"purchase_id", purchase.ID,
			"account_id", purchase.AccountID,
			"status", purchase.Status,
			"provider_payment_id", providerPaymentID,
		)
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
	rows, err := s.repo.ListPaidByAccount(ctx, accountID, clampHistoryLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("billing.Service.HistoryByAccount: %w", err)
	}

	return rows, nil
}

func (s *Service) AdminHistory(ctx context.Context, filter domain.PurchaseFilter, page, pageSize int) (rows []domain.Purchase, total int, err error) {
	pageSize = clampHistoryLimit(pageSize)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, total, err = s.repo.ListFiltered(ctx, filter, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("billing.Service.AdminHistory: %w", err)
	}

	return rows, total, nil
}

func (s *Service) Earnings(ctx context.Context) (domain.EarningsSummary, error) {
	now := s.now().In(s.loc)
	year, month, day := now.Date()
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, s.loc)
	weekStart := dayStart.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, s.loc)

	summary, err := s.repo.Earnings(ctx, dayStart, weekStart, monthStart)
	if err != nil {
		return domain.EarningsSummary{}, fmt.Errorf("billing.Service.Earnings: %w", err)
	}

	return summary, nil
}

func clampHistoryLimit(limit int) int {
	if limit <= 0 || limit > maxHistoryLimit {
		return maxHistoryLimit
	}

	return limit
}
