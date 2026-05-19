package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const pingTimeout = 2 * time.Second

type Pinger interface {
	PingContext(ctx context.Context) error
}

// New returns a handler that reports 503 unless all three databases respond to a ping within pingTimeout.
func New(mainDB, logDB, cpDB Pinger, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := mainDB.PingContext(ctx); err != nil {
			logger.Error("healthz: main db ping", "err", err)
			http.Error(w, "main db unavailable", http.StatusServiceUnavailable)
			return
		}

		if err := logDB.PingContext(ctx); err != nil {
			logger.Error("healthz: log db ping", "err", err)
			http.Error(w, "log db unavailable", http.StatusServiceUnavailable)
			return
		}

		if err := cpDB.PingContext(ctx); err != nil {
			logger.Error("healthz: cp db ping", "err", err)
			http.Error(w, "cp db unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	}
}
