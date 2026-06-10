package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accountdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	"github.com/hayakawakaki/go-racp/internal/features/market/app"
	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

var errStashTransport = errors.New("transport: stash test error")

type fakeStashRepository struct {
	listErr error
	items   []domain.StashItem
	locked  bool
}

func (f *fakeStashRepository) ListByAccount(_ context.Context, _ int) ([]domain.StashItem, error) {
	return f.items, f.listErr
}

func (f *fakeStashRepository) IsLocked(_ context.Context, _ int) (bool, error) {
	return f.locked, nil
}

func (f *fakeStashRepository) SlotsUsed(_ context.Context, _ int) (int, error) {
	return len(f.items), nil
}

func (f *fakeStashRepository) MergeableAmount(_ context.Context, _ int, _ domain.StashItem) (existingAmount int, found bool, err error) {
	return 0, false, nil
}

func newTestHandler(repo domain.StashRepository) *Handler {
	service := app.NewStashService(repo, 600)
	return NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestStashJSON_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeStashRepository{})
	request := httptest.NewRequest(http.MethodGet, "/market/stash", http.NoBody)
	recorder := httptest.NewRecorder()

	handler.stashJSON(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cacheControl)
	}
	if !strings.Contains(recorder.Body.String(), "error") {
		t.Errorf("body = %q, want it to contain error", recorder.Body.String())
	}
}

func TestStashJSON_OK(t *testing.T) {
	t.Parallel()

	repo := &fakeStashRepository{
		items:  []domain.StashItem{{NameID: 501, Amount: 10}, {NameID: 502, Amount: 3}},
		locked: true,
	}
	handler := newTestHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/market/stash", http.NoBody)
	ctx := middleware.ContextWithSession(request.Context(), &accountdomain.Session{UserID: 9900001})
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.stashJSON(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Items      []map[string]any `json:"items"`
		SlotsUsed  int              `json:"slots_used"`
		SlotsTotal int              `json:"slots_total"`
		Locked     bool             `json:"locked"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if len(body.Items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(body.Items))
	}
	if !body.Locked {
		t.Errorf("locked = false, want true")
	}
	if body.SlotsUsed != 2 {
		t.Errorf("slots_used = %d, want 2", body.SlotsUsed)
	}
	if body.SlotsTotal != 600 {
		t.Errorf("slots_total = %d, want 600", body.SlotsTotal)
	}
}

func TestStashJSON_ServiceError(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeStashRepository{listErr: errStashTransport})

	request := httptest.NewRequest(http.MethodGet, "/market/stash", http.NoBody)
	ctx := middleware.ContextWithSession(request.Context(), &accountdomain.Session{UserID: 9900001})
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.stashJSON(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), "error") {
		t.Errorf("body = %q, want it to contain error", recorder.Body.String())
	}
}
