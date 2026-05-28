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
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

type fakeReader struct {
	peaksErr     error
	generalErr   error
	peaks        []domain.PeakRow
	serverStatus domain.ServerStatusSnapshot
	general      domain.GeneralSnapshot
	online       domain.OnlineSnapshot
}

func (f *fakeReader) Online(context.Context) domain.OnlineSnapshot {
	return f.online
}

func (f *fakeReader) ServerStatus(context.Context) domain.ServerStatusSnapshot {
	return f.serverStatus
}

func (f *fakeReader) Peaks(context.Context) ([]domain.PeakRow, error) {
	return f.peaks, f.peaksErr
}

func (f *fakeReader) General(context.Context) (domain.GeneralSnapshot, error) {
	return f.general, f.generalErr
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandler_Online_ReturnsJSONSnapshot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	svc := &fakeReader{online: domain.OnlineSnapshot{
		UpdatedAt: now, Total: 100, Vendor: 30, NonVendor: 70, Unique: 80, HasUnique: true,
	}}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.online(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/online", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}

	var got domain.OnlineSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 100 || got.Vendor != 30 || got.NonVendor != 70 || got.Unique != 80 || !got.HasUnique {
		t.Errorf("body = %+v", got)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", got.UpdatedAt, now)
	}
}

func TestHandler_Status_ReturnsJSONSnapshot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	svc := &fakeReader{serverStatus: domain.ServerStatusSnapshot{
		CheckedAt: now, Login: true, Char: false, Map: true, Web: false,
	}}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.status(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/status", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}

	var got domain.ServerStatusSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Login || got.Char || !got.Map || got.Web {
		t.Errorf("body flags = %+v, want login=true char=false map=true web=false", got)
	}
	if !got.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt = %v, want %v", got.CheckedAt, now)
	}
}

func TestHandler_Peaks_ReturnsJSONRows(t *testing.T) {
	t.Parallel()
	svc := &fakeReader{peaks: []domain.PeakRow{
		{Metric: domain.MetricOnlineTotal, Window: domain.WindowDaily, Value: 100},
		{Metric: domain.MetricOnlineVendor, Window: domain.WindowDaily, Value: 30},
	}}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.peaks(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/peaks", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got []domain.PeakRow
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestHandler_Peaks_NilReturnsEmptyArrayNotNull(t *testing.T) {
	t.Parallel()
	svc := &fakeReader{peaks: nil}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.peaks(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/peaks", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
		t.Errorf("body = %q, want []", body)
	}
}

func TestHandler_Peaks_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()
	svc := &fakeReader{peaksErr: errors.New("boom")}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.peaks(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/peaks", http.NoBody))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] == "" {
		t.Errorf("expected error message in body, got %v", body)
	}
}

func TestHandler_General_ReturnsJSONSnapshot(t *testing.T) {
	t.Parallel()
	captured := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	svc := &fakeReader{general: domain.GeneralSnapshot{
		CapturedAt: captured, TotalAccounts: 10, TotalCharacters: 50, TotalGuilds: 3,
	}}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.general(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/general", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got domain.GeneralSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.TotalAccounts != 10 || got.TotalCharacters != 50 || got.TotalGuilds != 3 {
		t.Errorf("body = %+v", got)
	}
	if !got.CapturedAt.Equal(captured) {
		t.Errorf("CapturedAt = %v, want %v", got.CapturedAt, captured)
	}
}

func TestHandler_General_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()
	svc := &fakeReader{generalErr: errors.New("db down")}
	h := NewHandler(svc, discardLogger())

	rr := httptest.NewRecorder()
	h.general(rr, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/general", http.NoBody))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] == "" {
		t.Errorf("expected error message in body, got %v", body)
	}
}

func TestNewHandler_DefaultsLogger(t *testing.T) {
	t.Parallel()
	h := NewHandler(&fakeReader{}, nil)
	if h.logger == nil {
		t.Errorf("logger not defaulted")
	}
}
