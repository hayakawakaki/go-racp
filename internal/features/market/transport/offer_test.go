package transport

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

func discardOfferHandler() *Handler {
	return NewHandler(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestAtoiForm(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/?num=5&bad=abc", http.NoBody)
	if got := atoiForm(req, "num"); got != 5 {
		t.Errorf("atoiForm(num) = %d, want 5", got)
	}
	if got := atoiForm(req, "bad"); got != 0 {
		t.Errorf("atoiForm(bad) = %d, want 0 on parse failure", got)
	}
	if got := atoiForm(req, "missing"); got != 0 {
		t.Errorf("atoiForm(missing) = %d, want 0", got)
	}
}

func TestAtoi64Form(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/?id=9007199254740993&bad=x", http.NoBody)
	if got := atoi64Form(req, "id"); got != 9007199254740993 {
		t.Errorf("atoi64Form(id) = %d, want 9007199254740993", got)
	}
	if got := atoi64Form(req, "bad"); got != 0 {
		t.Errorf("atoi64Form(bad) = %d, want 0 on parse failure", got)
	}
}

func TestHandler_WriteOfferError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
		want int
	}{
		{name: "not found", err: domain.ErrListingNotFound, want: http.StatusNotFound},
		{name: "storage unlocked", err: domain.ErrStorageUnlocked, want: http.StatusConflict},
		{name: "insufficient funds", err: domain.ErrInsufficientFunds, want: http.StatusConflict},
		{name: "storage full", err: domain.ErrStorageFull, want: http.StatusConflict},
		{name: "invalid offer defaults to bad request", err: domain.ErrInvalidOffer, want: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := discardOfferHandler()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/market/listings/1/take", http.NoBody)

			h.writeOfferError(rr, req, tt.err)
			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d", rr.Code, tt.want)
			}
		})
	}
}

func TestHandler_OfferRoutes_Unauthorized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		handler func(*Handler) http.HandlerFunc
		name    string
	}{
		{handler: func(h *Handler) http.HandlerFunc { return h.create }, name: "create"},
		{handler: func(h *Handler) http.HandlerFunc { return h.take }, name: "take"},
		{handler: func(h *Handler) http.HandlerFunc { return h.cancel }, name: "cancel"},
		{handler: func(h *Handler) http.HandlerFunc { return h.mine }, name: "mine"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := discardOfferHandler()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/market/listings", http.NoBody)

			tt.handler(h)(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401 (no session)", rr.Code)
			}
		})
	}
}
