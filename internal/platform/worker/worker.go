package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Job struct {
	Fn       func(ctx context.Context) (int64, error)
	Name     string
	Interval time.Duration
}

func Run(ctx context.Context, logger *slog.Logger, jobs ...Job) {
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Go(func() {
			runJob(ctx, logger, job)
		})
	}
	wg.Wait()
}

func runJob(ctx context.Context, logger *slog.Logger, job Job) {
	tick := func() {
		count, err := job.Fn(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("worker job failed", "job", job.Name, "err", err)
			return
		}
		if count > 0 {
			logger.Info("worker job ran", "job", job.Name, "count", count)
		}
	}

	tick()

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}
