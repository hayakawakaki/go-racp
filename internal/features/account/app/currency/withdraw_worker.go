package currency

import (
	"context"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type WithdrawWorkerConfig struct {
	Logger    *slog.Logger
	Interval  time.Duration
	ReapAfter time.Duration
	Batch     int
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
	runLoop(ctx, w.cfg.Interval, w.drainOnce)
}

func (w *WithdrawWorker) drainOnce(ctx context.Context) {
	w.enqueue(ctx)
	w.confirm(ctx)
	w.reap(ctx)
}

func (w *WithdrawWorker) enqueue(ctx context.Context) {
	requests, err := w.repo.PendingWithdraws(ctx, w.cfg.Batch)
	if err != nil {
		w.cfg.Logger.Error("currency: pending withdraws", "err", err)
		return
	}

	for _, request := range requests {
		if err := w.queue.Insert(ctx, request.ID, request.AccountID, request.Zeny, request.Cashpoint); err != nil {
			w.cfg.Logger.Error("currency: insert withdraw", "id", request.ID, "err", err)
			continue
		}
		if err := w.repo.MarkWithdrawSent(ctx, request.ID, w.now()); err != nil {
			w.cfg.Logger.Error("currency: mark withdraw sent", "id", request.ID, "err", err)
		}
	}
}

func (w *WithdrawWorker) confirm(ctx context.Context) {
	delivered, err := w.queue.Delivered(ctx, w.cfg.Batch)
	if err != nil {
		w.cfg.Logger.Error("currency: delivered withdraws", "err", err)
		return
	}

	for _, row := range delivered {
		if row.Zeny != 0 || row.Points != 0 {
			w.cfg.Logger.Warn("currency: delivered row not drained, requeueing", "id", row.ID, "zeny", row.Zeny, "points", row.Points)
			if err := w.queue.ResetDelivered(ctx, row.ID); err != nil {
				w.cfg.Logger.Error("currency: reset delivered", "id", row.ID, "err", err)
			}
			continue
		}
		if err := w.repo.MarkWithdrawDelivered(ctx, row.ID, time.Unix(row.DeliveredAt, 0).UTC()); err != nil {
			w.cfg.Logger.Error("currency: mark withdraw delivered", "id", row.ID, "err", err)
			continue
		}
		if err := w.queue.Delete(ctx, row.ID); err != nil {
			w.cfg.Logger.Error("currency: delete withdraw", "id", row.ID, "err", err)
		}
	}
}

func (w *WithdrawWorker) reap(ctx context.Context) {
	if w.cfg.ReapAfter <= 0 {
		return
	}

	stale, err := w.repo.SentBefore(ctx, w.now().Add(-w.cfg.ReapAfter), w.cfg.Batch)
	if err != nil {
		w.cfg.Logger.Error("currency: stale withdraws", "err", err)
		return
	}

	for _, record := range stale {
		if err := w.queue.Insert(ctx, record.ID, record.AccountID, record.Zeny, record.Cashpoint); err != nil {
			w.cfg.Logger.Error("currency: reap reinsert withdraw", "id", record.ID, "err", err)
		}
	}
}
