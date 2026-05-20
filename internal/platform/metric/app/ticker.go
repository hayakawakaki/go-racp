package app

import (
	"context"
	"time"
)

func runTicker(ctx context.Context, interval time.Duration, refresh func(context.Context)) {
	refresh(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh(ctx)
		}
	}
}
