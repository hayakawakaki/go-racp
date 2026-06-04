package transport

import (
	"fmt"
	"net/http"
	"time"
)

const ssePingInterval = 25 * time.Second

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	accountID, ok := h.accountID(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		h.logger.Warn("notification: clear sse write deadline", "err", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	events, unsubscribe := h.svc.Subscribe(accountID)
	defer unsubscribe()

	h.svc.PublishUnread(r.Context(), accountID)

	ping := time.NewTicker(ssePingInterval)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event := <-events:
			if _, err := fmt.Fprintf(w, "event: notification\ndata: %d\n\n", event.Unread); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
