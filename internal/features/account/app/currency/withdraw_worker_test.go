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

func TestWithdrawWorker_Enqueue_InsertsBeforeMarking(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		pendingFn:  pendingOne(),
		markSentFn: func(context.Context, int64, time.Time) error { order = append(order, "mark_sent"); return nil },
	}
	queue := &fakeWithdrawQueue{
		insertFn: func(context.Context, int64, int, int64, int) error { order = append(order, "insert"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.enqueue(context.Background())

	if want := []string{"insert", "mark_sent"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v", order, want)
	}
}

func TestWithdrawWorker_Enqueue_SkipsMarkWhenInsertFails(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		pendingFn:  pendingOne(),
		markSentFn: func(context.Context, int64, time.Time) error { order = append(order, "mark_sent"); return nil },
	}
	queue := &fakeWithdrawQueue{
		insertFn: func(context.Context, int64, int, int64, int) error {
			order = append(order, "insert")
			return errors.New("mariadb down")
		},
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.enqueue(context.Background())

	if want := []string{"insert"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v (insert failure must not mark sent)", order, want)
	}
}

func TestWithdrawWorker_Confirm_MarksDeliveredBeforeDelete(t *testing.T) {
	t.Parallel()

	var order []string
	var gotDelivered time.Time
	repo := &fakeCurrencyRepo{
		markDeliveredFn: func(_ context.Context, _ int64, deliveredAt time.Time) (bool, error) {
			order = append(order, "mark_delivered")
			gotDelivered = deliveredAt
			return true, nil
		},
	}
	queue := &fakeWithdrawQueue{
		deliveredFn: func(context.Context, int) ([]domain.DeliveredWithdraw, error) {
			return []domain.DeliveredWithdraw{{ID: 9, DeliveredAt: 1700000000}}, nil
		},
		deleteFn: func(context.Context, int64) error { order = append(order, "delete"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.confirm(context.Background())

	if want := []string{"mark_delivered", "delete"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v (must mark status=3 before deleting MariaDB row)", order, want)
	}
	if !gotDelivered.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Errorf("delivered_at = %v, want %v", gotDelivered, time.Unix(1700000000, 0).UTC())
	}
}

func TestWithdrawWorker_Confirm_SkipsDeleteWhenMarkFails(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		markDeliveredFn: func(context.Context, int64, time.Time) (bool, error) {
			order = append(order, "mark_delivered")
			return false, errors.New("cp db down")
		},
	}
	queue := &fakeWithdrawQueue{
		deliveredFn: func(context.Context, int) ([]domain.DeliveredWithdraw, error) {
			return []domain.DeliveredWithdraw{{ID: 9, DeliveredAt: 1700000000}}, nil
		},
		deleteFn: func(context.Context, int64) error { order = append(order, "delete"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.confirm(context.Background())

	if want := []string{"mark_delivered"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v (a failed mark must not delete the MariaDB row)", order, want)
	}
}

func TestWithdrawWorker_Confirm_SkipsDeleteWhenNotAdvanced(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		markDeliveredFn: func(context.Context, int64, time.Time) (bool, error) {
			order = append(order, "mark_delivered")
			return false, nil
		},
	}
	queue := &fakeWithdrawQueue{
		deliveredFn: func(context.Context, int) ([]domain.DeliveredWithdraw, error) {
			return []domain.DeliveredWithdraw{{ID: 9, DeliveredAt: 1700000000}}, nil
		},
		deleteFn: func(context.Context, int64) error { order = append(order, "delete"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.confirm(context.Background())

	if want := []string{"mark_delivered"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v (an unadvanced ledger row must not delete the MariaDB row)", order, want)
	}
}

func TestWithdrawWorker_Confirm_RequeuesWhenNotDrained(t *testing.T) {
	t.Parallel()

	var order []string
	repo := &fakeCurrencyRepo{
		markDeliveredFn: func(context.Context, int64, time.Time) (bool, error) {
			order = append(order, "mark_delivered")
			return true, nil
		},
	}
	queue := &fakeWithdrawQueue{
		deliveredFn: func(context.Context, int) ([]domain.DeliveredWithdraw, error) {
			return []domain.DeliveredWithdraw{{ID: 9, DeliveredAt: 1700000000, Zeny: 100, Points: 0}}, nil
		},
		resetDeliveredFn: func(context.Context, int64) error { order = append(order, "reset"); return nil },
		deleteFn:         func(context.Context, int64) error { order = append(order, "delete"); return nil },
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.confirm(context.Background())

	if want := []string{"reset"}; !slices.Equal(order, want) {
		t.Errorf("call order = %v, want %v (a delivered row with non-zero zeny must be reset, not confirmed or deleted)", order, want)
	}
}

func TestWithdrawWorker_Reap_ReinsertsStaleSentRows(t *testing.T) {
	t.Parallel()

	var inserted []int64
	repo := &fakeCurrencyRepo{
		sentBeforeFn: func(context.Context, time.Time, int) ([]domain.WithdrawRecord, error) {
			return []domain.WithdrawRecord{{ID: 42, AccountID: 1, Zeny: 100, Cashpoint: 5}}, nil
		},
	}
	queue := &fakeWithdrawQueue{
		insertFn: func(_ context.Context, id int64, _ int, _ int64, _ int) error {
			inserted = append(inserted, id)
			return nil
		},
	}
	worker := NewWithdrawWorker(repo, queue, WithdrawWorkerConfig{Logger: discardLogger(), ReapAfter: time.Minute})

	worker.reap(context.Background())

	if want := []int64{42}; !slices.Equal(inserted, want) {
		t.Errorf("reaped re-inserts = %v, want %v", inserted, want)
	}
}

func TestWithdrawWorker_Reap_DisabledWhenReapAfterZero(t *testing.T) {
	t.Parallel()

	called := false
	repo := &fakeCurrencyRepo{
		sentBeforeFn: func(context.Context, time.Time, int) ([]domain.WithdrawRecord, error) {
			called = true
			return nil, nil
		},
	}
	worker := NewWithdrawWorker(repo, &fakeWithdrawQueue{}, WithdrawWorkerConfig{Logger: discardLogger()})

	worker.reap(context.Background())

	if called {
		t.Errorf("reap must be a no-op when ReapAfter is 0")
	}
}
