package currency

import (
	"context"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type BalanceDTO struct {
	Zeny      int64
	Cashpoint int
}

type WithdrawDTO struct {
	ID        int64
	Zeny      int64
	Cashpoint int
}

type BridgePinger interface {
	PingContext(ctx context.Context) error
}

type Service struct {
	repo         domain.CurrencyRepository
	now          func() time.Time
	bridge       BridgePinger
	cooldown     time.Duration
	maxZeny      int64
	maxCashpoint int
}

type Option func(*Service)

func WithNow(fn func() time.Time) Option {
	return func(s *Service) {
		if fn != nil {
			s.now = fn
		}
	}
}

func WithCooldown(d time.Duration) Option {
	return func(s *Service) { s.cooldown = d }
}

func WithLimits(maxZeny int64, maxCashpoint int) Option {
	return func(s *Service) {
		s.maxZeny = maxZeny
		s.maxCashpoint = maxCashpoint
	}
}

func WithBridge(pinger BridgePinger) Option {
	return func(s *Service) { s.bridge = pinger }
}

func NewService(repo domain.CurrencyRepository, opts ...Option) *Service {
	s := &Service{repo: repo, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Service) Balance(ctx context.Context, accountID int) (BalanceDTO, error) {
	balance, err := s.repo.Balance(ctx, accountID)
	if err != nil {
		return BalanceDTO{}, fmt.Errorf("currency.Service.Balance: %w", err)
	}

	return BalanceDTO{Zeny: balance.Zeny, Cashpoint: balance.Cashpoint}, nil
}

func (s *Service) RequestWithdraw(ctx context.Context, accountID int, zeny int64, cashpoint int) error {
	if s.bridge != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := s.bridge.PingContext(pingCtx); err != nil {
			return fmt.Errorf("currency.Service.RequestWithdraw: %w", domain.ErrBridgeUnavailable)
		}
	}
	if zeny < 0 || cashpoint < 0 {
		return domain.ErrInvalidAmount
	}
	if zeny == 0 && cashpoint == 0 {
		return domain.ErrInvalidAmount
	}
	if zeny > s.maxZeny || cashpoint > s.maxCashpoint {
		return domain.ErrInvalidAmount
	}

	now := s.now()
	if _, err := s.repo.RequestWithdraw(ctx, accountID, zeny, cashpoint, now.Add(s.cooldown), now); err != nil {
		return fmt.Errorf("currency.Service.RequestWithdraw: %w", err)
	}

	return nil
}

func (s *Service) RecentWithdraws(ctx context.Context, accountID, limit int) ([]WithdrawDTO, error) {
	requests, err := s.repo.RecentWithdraws(ctx, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("currency.Service.RecentWithdraws: %w", err)
	}

	out := make([]WithdrawDTO, 0, len(requests))
	for _, request := range requests {
		out = append(out, WithdrawDTO{ID: request.ID, Zeny: request.Zeny, Cashpoint: request.Cashpoint})
	}

	return out, nil
}
