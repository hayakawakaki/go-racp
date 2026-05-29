package currency

import (
	"context"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type WithdrawWorkerConfig struct {
	Logger   *slog.Logger
	Interval time.Duration
	Batch    int
}

type WithdrawWorker struct {
	repo  domain.CurrencyRepository
	queue domain.WithdrawQueue
	now   func() time.Time
	cfg   WithdrawWorkerConfig
}

func NewWithdrawWorker(repo domain.CurrencyRepository, queue domain.WithdrawQueue, cfg WithdrawWorkerConfig) *WithdrawWorker {
	if cfg.Batch <= 0 {
		cfg.Batch = 100
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &WithdrawWorker{repo: repo, queue: queue, cfg: cfg, now: time.Now}
}

func (w *WithdrawWorker) Run(ctx context.Context) {
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

func (w *WithdrawWorker) drainOnce(ctx context.Context) {
	requests, err := w.repo.PendingWithdraws(ctx, w.cfg.Batch)
	if err != nil {
		w.cfg.Logger.Error("currency: pending withdraws", "err", err)
		return
	}

	for _, withdrawRequest := range requests {
		if err := w.queue.Insert(ctx, withdrawRequest.ID, withdrawRequest.AccountID, withdrawRequest.Zeny, withdrawRequest.Cashpoint); err != nil {
			w.cfg.Logger.Error("currency: insert withdraw", "id", withdrawRequest.ID, "err", err)
			continue
		}
		if err := w.repo.MarkWithdrawSent(ctx, withdrawRequest.ID, w.now()); err != nil {
			w.cfg.Logger.Error("currency: mark withdraw sent", "id", withdrawRequest.ID, "err", err)
		}
	}
}
