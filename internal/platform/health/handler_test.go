package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePinger struct {
	err error
}

func (f fakePinger) PingContext(ctx context.Context) error {
	return f.err
}

func TestHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		main       error
		log        error
		cp         error
		wantBody   string
		name       string
		wantStatus int
	}{
		{name: "all ok", main: nil, log: nil, cp: nil, wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "main fails", main: errors.New("boom"), log: nil, cp: nil, wantStatus: http.StatusServiceUnavailable, wantBody: "main db unavailable"},
		{name: "log fails", main: nil, log: errors.New("boom"), cp: nil, wantStatus: http.StatusServiceUnavailable, wantBody: "log db unavailable"},
		{name: "cp fails", main: nil, log: nil, cp: errors.New("boom"), wantStatus: http.StatusServiceUnavailable, wantBody: "cp db unavailable"},
		{name: "main+log fail returns main first", main: errors.New("main boom"), log: errors.New("log boom"), cp: nil, wantStatus: http.StatusServiceUnavailable, wantBody: "main db unavailable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := New(fakePinger{err: tc.main}, fakePinger{err: tc.log}, fakePinger{err: tc.cp}, logger)
			req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
			body := strings.TrimSpace(rec.Body.String())
			if !strings.Contains(body, tc.wantBody) {
				t.Fatalf("body: got %q, want contains %q", body, tc.wantBody)
			}
		})
	}
}

func TestHandlerPingTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	blocker := blockingPinger{}
	h := New(blocker, fakePinger{}, fakePinger{}, logger)

	req := httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

type blockingPinger struct{}

func (blockingPinger) PingContext(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
