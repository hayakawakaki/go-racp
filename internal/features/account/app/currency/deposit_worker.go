package currency

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type DepositWorkerConfig struct {
	Logger       *slog.Logger
	Interval     time.Duration
	Cooldown     time.Duration
	MaxZeny      int64
	MaxCashpoint int
	Batch        int
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
	w.drainOnce(ctx)
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.drainOnce(ctx)
		}
	}
}

func (w *DepositWorker) drainOnce(ctx context.Context) {
	rows, err := w.queue.Batch(ctx, w.cfg.Batch)
	if err != nil {
		w.cfg.Logger.Error("currency: deposit batch", "err", err)
		return
	}

	for _, depositRow := range rows {
		if !w.valid(depositRow) {
			w.cfg.Logger.Error("currency: invalid deposit row, deleting", "id", depositRow.ID, "account_id", depositRow.AccountID, "zeny", depositRow.Zeny, "points", depositRow.Points)
			if err := w.queue.Delete(ctx, depositRow.ID); err != nil {
				w.cfg.Logger.Error("currency: delete invalid deposit", "id", depositRow.ID, "err", err)
			}
			continue
		}

		now := w.now()
		_, err := w.repo.CreditDeposit(ctx, depositRow.ID, depositRow.AccountID, depositRow.Zeny, depositRow.Points, now.Add(w.cfg.Cooldown), now)
		if errors.Is(err, domain.ErrDepositLocked) {
			continue
		}
		if err != nil {
			w.cfg.Logger.Error("currency: credit deposit", "id", depositRow.ID, "err", err)
			continue
		}
		if err := w.queue.Delete(ctx, depositRow.ID); err != nil {
			w.cfg.Logger.Error("currency: delete credited deposit", "id", depositRow.ID, "err", err)
		}
	}
}

func (w *DepositWorker) valid(depositRow domain.DepositRow) bool {
	if depositRow.Zeny < 0 || depositRow.Points < 0 {
		return false
	}
	if depositRow.Zeny == 0 && depositRow.Points == 0 {
		return false
	}

	return depositRow.Zeny <= w.cfg.MaxZeny && depositRow.Points <= w.cfg.MaxCashpoint
}
