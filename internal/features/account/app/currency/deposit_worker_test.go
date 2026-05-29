package currency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func TestValidDepositRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  domain.DepositRow
		want bool
	}{
		{name: "both positive", row: domain.DepositRow{Zeny: 10, Points: 5}, want: true},
		{name: "zeny only", row: domain.DepositRow{Zeny: 10}, want: true},
		{name: "points only", row: domain.DepositRow{Points: 5}, want: true},
		{name: "large amounts uncapped", row: domain.DepositRow{Zeny: 5_000_000_000, Points: 5_000_000}, want: true},
		{name: "both zero", row: domain.DepositRow{}, want: false},
		{name: "negative zeny", row: domain.DepositRow{Zeny: -1, Points: 5}, want: false},
		{name: "negative points", row: domain.DepositRow{Zeny: 10, Points: -1}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := validDepositRow(tt.row); got != tt.want {
				t.Errorf("validDepositRow(%+v) = %v, want %v", tt.row, got, tt.want)
			}
		})
	}
}

func TestDepositWorker_DrainOnce_DeletesByOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		creditErr   error
		name        string
		credited    bool
		wantDeleted bool
	}{
		{name: "credited is removed", credited: true, wantDeleted: true},
		{name: "already processed is removed", credited: false, wantDeleted: true},
		{name: "cooldown locked is kept", creditErr: domain.ErrDepositLocked, wantDeleted: false},
		{name: "transient error is kept", creditErr: errors.New("db down"), wantDeleted: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var deleted []int64
			repo := &fakeCurrencyRepo{
				creditDepositFn: func(context.Context, int64, int, int64, int, time.Time, time.Time) (bool, error) {
					return tt.credited, tt.creditErr
				},
			}
			queue := &fakeDepositQueue{
				batchFn: func(context.Context, int) ([]domain.DepositRow, error) {
					return []domain.DepositRow{{ID: 7, AccountID: 1, Zeny: 10, Points: 5}}, nil
				},
				deleteFn: func(_ context.Context, id int64) error {
					deleted = append(deleted, id)
					return nil
				},
			}
			worker := NewDepositWorker(repo, queue, DepositWorkerConfig{Logger: discardLogger()})

			worker.drainOnce(context.Background())

			gotDeleted := len(deleted) == 1 && deleted[0] == 7
			if gotDeleted != tt.wantDeleted {
				t.Errorf("deleted = %v, wantDeleted = %v", deleted, tt.wantDeleted)
			}
		})
	}
}

func TestDepositWorker_DrainOnce_InvalidRowSkipsCredit(t *testing.T) {
	t.Parallel()

	credited := false
	var deleted []int64
	repo := &fakeCurrencyRepo{
		creditDepositFn: func(context.Context, int64, int, int64, int, time.Time, time.Time) (bool, error) {
			credited = true
			return true, nil
		},
	}
	queue := &fakeDepositQueue{
		batchFn: func(context.Context, int) ([]domain.DepositRow, error) {
			return []domain.DepositRow{{ID: 3, AccountID: 1, Zeny: 0, Points: 0}}, nil
		},
		deleteFn: func(_ context.Context, id int64) error {
			deleted = append(deleted, id)
			return nil
		},
	}
	worker := NewDepositWorker(repo, queue, DepositWorkerConfig{Logger: discardLogger()})

	worker.drainOnce(context.Background())

	if credited {
		t.Errorf("CreditDeposit must not be called for an invalid row")
	}
	if len(deleted) != 1 || deleted[0] != 3 {
		t.Errorf("invalid row should be deleted, deleted = %v", deleted)
	}
}
