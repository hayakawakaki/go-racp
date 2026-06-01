package self

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func TestParseAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "empty is zero", raw: "", want: 0},
		{name: "zero", raw: "0", want: 0},
		{name: "positive", raw: "42", want: 42},
		{name: "non-numeric", raw: "abc", wantErr: true},
		{name: "float", raw: "1.5", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAmount(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAmount(%q) err = nil, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAmount(%q) unexpected err: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("parseAmount(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseAmount64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{name: "empty is zero", raw: "", want: 0},
		{name: "positive", raw: "2000000000", want: 2000000000},
		{name: "non-numeric", raw: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAmount64(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAmount64(%q) err = nil, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAmount64(%q) unexpected err: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("parseAmount64(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestHandler_DoWithdraw_Routing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		form         map[string]string
		repoErr      error
		wantLocation string
		wantCalled   bool
	}{
		{name: "success", form: map[string]string{"cashpoint": "100", "zeny": "0"}, wantLocation: "/account?notice=withdraw_ok", wantCalled: true},
		{name: "locked", form: map[string]string{"cashpoint": "100"}, repoErr: domain.ErrWithdrawLocked, wantLocation: "/account?notice=withdraw_locked", wantCalled: true},
		{name: "insufficient", form: map[string]string{"cashpoint": "100"}, repoErr: domain.ErrInsufficientBalance, wantLocation: "/account?notice=withdraw_insufficient", wantCalled: true},
		{name: "invalid amount", form: map[string]string{"cashpoint": "100"}, repoErr: domain.ErrInvalidAmount, wantLocation: "/account?notice=withdraw_invalid", wantCalled: true},
		{name: "bridge unavailable", form: map[string]string{"cashpoint": "100"}, repoErr: domain.ErrBridgeUnavailable, wantLocation: "/account?notice=withdraw_bridge", wantCalled: true},
		{name: "parse error skips service", form: map[string]string{"cashpoint": "abc"}, wantLocation: "/account?notice=withdraw_invalid", wantCalled: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			stub := &stubCurrencyService{
				requestWithdrawFn: func(context.Context, int, int64, int) error {
					called = true
					return tt.repoErr
				},
			}
			h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)
			h.currency = stub

			rec := httptest.NewRecorder()
			h.doWithdraw(rec, postWithSession("/account/withdraw", 7, tt.form))

			if rec.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
			}
			if loc := rec.Header().Get("Location"); loc != tt.wantLocation {
				t.Errorf("Location = %q, want %q", loc, tt.wantLocation)
			}
			if called != tt.wantCalled {
				t.Errorf("service called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}

func TestHandler_DoWithdraw_NilCurrencyRedirects(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)
	h.currency = nil

	rec := httptest.NewRecorder()
	h.doWithdraw(rec, postWithSession("/account/withdraw", 7, map[string]string{"cashpoint": "100"}))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account" {
		t.Errorf("code=%d loc=%q, want 303 /account", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHandler_DoWithdraw_NoSessionRedirectsLogin(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)
	h.currency = &stubCurrencyService{}

	rec := httptest.NewRecorder()
	h.doWithdraw(rec, httptest.NewRequest(http.MethodPost, "/account/withdraw", http.NoBody))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("code=%d loc=%q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHandler_DoWithdraw_HTMXSuccessFiresToast(t *testing.T) {
	t.Parallel()

	stub := &stubCurrencyService{
		requestWithdrawFn: func(context.Context, int, int64, int) error {
			return nil
		},
	}
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)
	h.currency = stub

	req := postWithSession("/account/withdraw", 7, map[string]string{"cashpoint": "100", "zeny": "0"})
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	h.doWithdraw(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("Location = %q, want empty (no redirect)", loc)
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "toast") {
		t.Errorf("HX-Trigger = %q, want it to contain a toast", trigger)
	}
}

func TestHandler_DoWithdraw_HTMXErrorRendersForm(t *testing.T) {
	t.Parallel()

	stub := &stubCurrencyService{
		requestWithdrawFn: func(context.Context, int, int64, int) error {
			return domain.ErrInsufficientBalance
		},
	}
	h := newTestHandler(&stubAccountService{}, &stubSessionService{}, nil)
	h.currency = stub

	req := postWithSession("/account/withdraw", 7, map[string]string{"cashpoint": "100", "zeny": "0"})
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	h.doWithdraw(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Errorf("Location = %q, want empty (no redirect)", loc)
	}
	if !strings.Contains(rec.Body.String(), "You do not have enough balance") {
		t.Errorf("body should surface the insufficient notice; got %s", rec.Body.String())
	}
}
