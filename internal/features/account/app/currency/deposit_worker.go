package currency

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type DepositWorkerConfig struct {
	Logger   *slog.Logger
	Interval time.Duration
	Cooldown time.Duration
	Batch    int
}

type DepositWorker struct {
	repo  domain.CurrencyRepository
	queue domain.DepositQueue
	now   func() time.Time
	cfg   DepositWorkerConfig
}

func NewDepositWorker(repo domain.CurrencyRepository, queue domain.DepositQueue, cfg DepositWorkerConfig) *DepositWorker {
	if cfg.Batch <= 0 {
		cfg.Batch = 100
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &DepositWorker{repo: repo, queue: queue, cfg: cfg, now: time.Now}
}

func (w *DepositWorker) Run(ctx context.Context) {
	runLoop(ctx, w.cfg.Interval, w.drainOnce)
}

func (w *DepositWorker) drainOnce(ctx context.Context) {
	rows, err := w.queue.Batch(ctx, w.cfg.Batch)
	if err != nil {
		w.cfg.Logger.Error("currency: deposit batch", "err", err)
		return
	}

	for _, depositRow := range rows {
		if !validDepositRow(depositRow) {
			w.cfg.Logger.Error("currency: invalid deposit row, deleting", "id", depositRow.ID, "account_id", depositRow.AccountID, "zeny", depositRow.Zeny, "points", depositRow.Points)
			w.deleteDeposit(ctx, depositRow.ID)
			continue
		}

		if w.applyDeposit(ctx, depositRow) {
			w.deleteDeposit(ctx, depositRow.ID)
		}
	}
}

func (w *DepositWorker) applyDeposit(ctx context.Context, depositRow domain.DepositRow) (remove bool) {
	now := w.now()
	_, err := w.repo.CreditDeposit(ctx, depositRow.ID, depositRow.AccountID, depositRow.Zeny, depositRow.Points, now.Add(w.cfg.Cooldown), now)
	switch {
	case err == nil:
		return true
	case errors.Is(err, domain.ErrDepositLocked):
		return false
	default:
		w.cfg.Logger.Error("currency: credit deposit", "id", depositRow.ID, "err", err)
		return false
	}
}

func (w *DepositWorker) deleteDeposit(ctx context.Context, id int64) {
	if err := w.queue.Delete(ctx, id); err != nil {
		w.cfg.Logger.Error("currency: delete deposit", "id", id, "err", err)
	}
}

func validDepositRow(depositRow domain.DepositRow) bool {
	if depositRow.Zeny < 0 || depositRow.Points < 0 {
		return false
	}

	return depositRow.Zeny > 0 || depositRow.Points > 0
}

func runLoop(ctx context.Context, interval time.Duration, tick func(context.Context)) {
	tick(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick(ctx)
		}
	}
}
