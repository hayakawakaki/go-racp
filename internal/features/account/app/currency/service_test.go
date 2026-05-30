package currency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func TestService_RequestWithdraw_RejectsInvalidAmounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		zeny      int64
		cashpoint int
	}{
		{name: "both zero", zeny: 0, cashpoint: 0},
		{name: "negative zeny", zeny: -1, cashpoint: 0},
		{name: "negative cashpoint", zeny: 0, cashpoint: -1},
		{name: "zeny over limit", zeny: 2001, cashpoint: 0},
		{name: "cashpoint over limit", zeny: 0, cashpoint: 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			repo := &fakeCurrencyRepo{
				requestWithdrawFn: func(context.Context, int, int64, int, time.Time, time.Time) (int64, error) {
					called = true
					return 1, nil
				},
			}
			svc := NewService(repo, WithLimits(2000, 100), WithCooldown(time.Minute))

			err := svc.RequestWithdraw(context.Background(), 1, tt.zeny, tt.cashpoint)
			if !errors.Is(err, domain.ErrInvalidAmount) {
				t.Fatalf("err = %v, want ErrInvalidAmount", err)
			}
			if called {
				t.Errorf("repo.RequestWithdraw must not be called for invalid input")
			}
		})
	}
}

func TestService_RequestWithdraw_PassesSharedCooldownDeadline(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	cooldown := 5 * time.Minute

	var gotLockUntil, gotNow time.Time
	repo := &fakeCurrencyRepo{
		requestWithdrawFn: func(_ context.Context, _ int, _ int64, _ int, lockUntil, callNow time.Time) (int64, error) {
			gotLockUntil = lockUntil
			gotNow = callNow
			return 7, nil
		},
	}
	svc := NewService(repo, WithLimits(2000, 100), WithCooldown(cooldown), WithNow(func() time.Time { return now }))

	if err := svc.RequestWithdraw(context.Background(), 1, 100, 5); err != nil {
		t.Fatalf("RequestWithdraw: %v", err)
	}
	if !gotNow.Equal(now) {
		t.Errorf("now = %v, want %v", gotNow, now)
	}
	if want := now.Add(cooldown); !gotLockUntil.Equal(want) {
		t.Errorf("lockUntil = %v, want %v", gotLockUntil, want)
	}
}

func TestService_RequestWithdraw_WrapsRepoError(t *testing.T) {
	t.Parallel()

	repo := &fakeCurrencyRepo{
		requestWithdrawFn: func(context.Context, int, int64, int, time.Time, time.Time) (int64, error) {
			return 0, domain.ErrWithdrawLocked
		},
	}
	svc := NewService(repo, WithLimits(2000, 100), WithCooldown(time.Minute))

	err := svc.RequestWithdraw(context.Background(), 1, 100, 0)
	if !errors.Is(err, domain.ErrWithdrawLocked) {
		t.Fatalf("err = %v, want ErrWithdrawLocked in chain", err)
	}
}

type fakeBridge struct {
	err error
}

func (f fakeBridge) PingContext(context.Context) error { return f.err }

func TestService_RequestWithdraw_RejectsWhenBridgeDown(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeCurrencyRepo{}, WithLimits(1_000_000, 1000), WithBridge(fakeBridge{err: errors.New("mariadb down")}))

	err := svc.RequestWithdraw(context.Background(), 1, 100, 0)
	if !errors.Is(err, domain.ErrBridgeUnavailable) {
		t.Errorf("err = %v, want ErrBridgeUnavailable", err)
	}
}

func TestService_RequestWithdraw_ProceedsWhenBridgeUp(t *testing.T) {
	t.Parallel()

	called := false
	repo := &fakeCurrencyRepo{
		requestWithdrawFn: func(context.Context, int, int64, int, time.Time, time.Time) (int64, error) {
			called = true
			return 1, nil
		},
	}
	svc := NewService(repo, WithLimits(1_000_000, 1000), WithBridge(fakeBridge{err: nil}))

	if err := svc.RequestWithdraw(context.Background(), 1, 100, 0); err != nil {
		t.Fatalf("RequestWithdraw: %v", err)
	}
	if !called {
		t.Errorf("repo.RequestWithdraw must be called when the bridge is up")
	}
}

func TestService_Balance_MapsDTO(t *testing.T) {
	t.Parallel()

	repo := &fakeCurrencyRepo{
		balanceFn: func(context.Context, int) (domain.Balance, error) {
			return domain.Balance{AccountID: 1, Zeny: 123, Cashpoint: 45}, nil
		},
	}
	svc := NewService(repo)

	got, err := svc.Balance(context.Background(), 1)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got.Zeny != 123 || got.Cashpoint != 45 {
		t.Errorf("Balance = %+v, want {Zeny:123 Cashpoint:45}", got)
	}
}

func TestService_RecentWithdraws_MapsDTOs(t *testing.T) {
	t.Parallel()

	repo := &fakeCurrencyRepo{
		recentFn: func(context.Context, int, int) ([]domain.WithdrawRequest, error) {
			return []domain.WithdrawRequest{
				{ID: 2, Zeny: 200, Cashpoint: 20},
				{ID: 1, Zeny: 100, Cashpoint: 10},
			}, nil
		},
	}
	svc := NewService(repo)

	got, err := svc.RecentWithdraws(context.Background(), 1, 5)
	if err != nil {
		t.Fatalf("RecentWithdraws: %v", err)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Errorf("RecentWithdraws = %+v, want ids [2 1]", got)
	}
}
