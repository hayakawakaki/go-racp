package currency

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func pendingOne() func(context.Context, int) ([]domain.WithdrawRequest, error) {
	return func(context.Context, int) ([]domain.WithdrawRequest, error) {
		return []domain.WithdrawRequest{{ID: 9, AccountID: 1, Zeny: 100, Cashpoint: 5}}, nil
	}
}

func TestWithdrawWorker_DrainOnce_ClaimsBeforeSending(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		pendingFn:     pendingOne(),
		markSentFn:    func(context.Context, int64, time.Time) error { order = append(order, "mark_sent"); return nil },
		markPendingFn: func(context.Context, int64) error { order = append(order, "mark_pending"); return nil },
	}
	queue := &fakeWithdrawQueue{
		insertFn: func(context.Context, int64, int, int64, int) error { order = append(order, "insert"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.drainOnce(context.Background())

	if want := []string{"mark_sent", "insert"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v", order, want)
	}
}

func TestWithdrawWorker_DrainOnce_RevertsOnInsertFailure(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		pendingFn:     pendingOne(),
		markSentFn:    func(context.Context, int64, time.Time) error { order = append(order, "mark_sent"); return nil },
		markPendingFn: func(context.Context, int64) error { order = append(order, "mark_pending"); return nil },
	}
	queue := &fakeWithdrawQueue{
		insertFn: func(context.Context, int64, int, int64, int) error {
			order = append(order, "insert")
			return errors.New("mariadb down")
		},
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.drainOnce(context.Background())

	if want := []string{"mark_sent", "insert", "mark_pending"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v", order, want)
	}
}

func TestWithdrawWorker_DrainOnce_SkipsSendWhenClaimFails(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		pendingFn: pendingOne(),
		markSentFn: func(context.Context, int64, time.Time) error {
			order = append(order, "mark_sent")
			return errors.New("cp db down")
		},
		markPendingFn: func(context.Context, int64) error { order = append(order, "mark_pending"); return nil },
	}
	queue := &fakeWithdrawQueue{
		insertFn: func(context.Context, int64, int, int64, int) error { order = append(order, "insert"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.drainOnce(context.Background())

	if want := []string{"mark_sent"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v (claim failure must not send or revert)", order, want)
	}
}
