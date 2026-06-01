package middleware

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/security"
	"github.com/hayakawakaki/go-racp/server/config"
)

type fakeValidator struct {
	key *domain.APIKey
	err error
}

func (f fakeValidator) Validate(context.Context, string) (*domain.APIKey, error) {
	return f.key, f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newLimiter(t *testing.T, ratePerMinute, burst int) *security.RateLimiter {
	t.Helper()

	limiter, err := security.NewRateLimiter(security.RateLimiterOptions{
		Name: "test",
		Rule: config.RateLimitRule{RatePerMinute: ratePerMinute, Burst: burst},
	})
	if err != nil {
		t.Fatalf("NewRateLimiter: %v", err)
	}

	return limiter
}

func newTestGate(t *testing.T, validator Validator) func(http.Handler) http.Handler {
	t.Helper()

	return APIKeyGate(GateConfig{
		Validator: validator,
		Limiters:  map[string]*security.RateLimiter{"Standard": newLimiter(t, 600, 600)},
		Fallback:  newLimiter(t, 600, 600),
		Invalid:   newLimiter(t, 600, 600),
		Logger:    discardLogger(),
	})
}

func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func bodyError(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
	}

	return body.Error
}

func TestAPIKeyGate(t *testing.T) {
	t.Parallel()

	validKey := &domain.APIKey{ID: 1, Name: "deploy bot", RateTier: "Standard"}

	tests := []struct {
		validator  fakeValidator
		name       string
		authHeader string
		wantBody   string
		wantStatus int
		setHeader  bool
		wantCalled bool
	}{
		{
			name:       "missing authorization header rejected",
			setHeader:  false,
			validator:  fakeValidator{key: validKey},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid api key",
		},
		{
			name:       "non bearer scheme rejected",
			setHeader:  true,
			authHeader: "Token abc123",
			validator:  fakeValidator{key: validKey},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid api key",
		},
		{
			name:       "empty bearer token rejected",
			setHeader:  true,
			authHeader: "Bearer    ",
			validator:  fakeValidator{key: validKey},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid api key",
		},
		{
			name:       "validator rejects key",
			setHeader:  true,
			authHeader: "Bearer badkey",
			validator:  fakeValidator{err: domain.ErrKeyNotFound},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "invalid api key",
		},
		{
			name:       "valid key with known tier passes through",
			setHeader:  true,
			authHeader: "Bearer goodkey",
			validator:  fakeValidator{key: validKey},
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "valid key with unknown tier falls back and passes through",
			setHeader:  true,
			authHeader: "Bearer goodkey",
			validator:  fakeValidator{key: &domain.APIKey{ID: 2, RateTier: "Platinum"}},
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "lowercase bearer scheme accepted",
			setHeader:  true,
			authHeader: "bearer goodkey",
			validator:  fakeValidator{key: validKey},
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called bool
			gate := newTestGate(t, tt.validator)
			handler := gate(okHandler(&called))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", http.NoBody)
			if tt.setHeader {
				req.Header.Set("Authorization", tt.authHeader)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Errorf("downstream called = %v, want %v", called, tt.wantCalled)
			}
			if tt.wantBody != "" {
				if got := bodyError(t, recorder); got != tt.wantBody {
					t.Errorf("body error = %q, want %q", got, tt.wantBody)
				}
			}
		})
	}
}

func TestAPIKeyGate_RateLimitsValidKey(t *testing.T) {
	t.Parallel()

	gate := APIKeyGate(GateConfig{
		Validator: fakeValidator{key: &domain.APIKey{ID: 1, RateTier: "Standard"}},
		Limiters:  map[string]*security.RateLimiter{"Standard": newLimiter(t, 60, 1)},
		Fallback:  newLimiter(t, 60, 1),
		Invalid:   newLimiter(t, 600, 600),
		Logger:    discardLogger(),
	})

	var called bool
	handler := gate(okHandler(&called))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, gateRequest("Bearer goodkey"))
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}

	called = false
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, gateRequest("Bearer goodkey"))
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want 429", second.Code)
	}
	if called {
		t.Errorf("downstream called after rate limit")
	}
	if got := second.Header().Get("Retry-After"); got == "" {
		t.Errorf("Retry-After header missing on 429")
	}
	if got := bodyError(t, second); got != "rate limit exceeded" {
		t.Errorf("body error = %q, want %q", got, "rate limit exceeded")
	}
}

func TestAPIKeyGate_RateLimitsInvalidKey(t *testing.T) {
	t.Parallel()

	gate := APIKeyGate(GateConfig{
		Validator: fakeValidator{err: domain.ErrKeyNotFound},
		Limiters:  map[string]*security.RateLimiter{},
		Fallback:  newLimiter(t, 600, 600),
		Invalid:   newLimiter(t, 60, 1),
		Logger:    discardLogger(),
	})

	var called bool
	handler := gate(okHandler(&called))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, gateRequest("Bearer badkey"))
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first invalid request status = %d, want 401", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, gateRequest("Bearer badkey"))
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second invalid request status = %d, want 429", second.Code)
	}
	if got := second.Header().Get("Retry-After"); got == "" {
		t.Errorf("Retry-After header missing on 429")
	}
}

func TestBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOk    bool
	}{
		{name: "valid bearer", header: "Bearer abc123", wantToken: "abc123", wantOk: true},
		{name: "lowercase scheme", header: "bearer abc123", wantToken: "abc123", wantOk: true},
		{name: "trims surrounding space", header: "Bearer   abc123  ", wantToken: "abc123", wantOk: true},
		{name: "missing header", header: "", wantOk: false},
		{name: "wrong scheme", header: "Token abc123", wantOk: false},
		{name: "scheme only", header: "Bearer ", wantOk: false},
		{name: "scheme with only spaces", header: "Bearer    ", wantOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, ok := bearerToken(req)
			if ok != tt.wantOk {
				t.Errorf("bearerToken ok = %v, want %v", ok, tt.wantOk)
			}
			if token != tt.wantToken {
				t.Errorf("bearerToken token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func gateRequest(authHeader string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", http.NoBody)
	req.RemoteAddr = "1.2.3.4:5000"
	req.Header.Set("Authorization", authHeader)

	return req
}
