package transport

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/server/config"
)

func newTestHandler() *Handler {
	return NewHandler(HandlerConfig{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		General: config.GeneralConfig{ServerName: "Test CP", Timezone: "UTC"},
	})
}

func TestHandler_ShowDashboard_FullPage(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<title>Test CP / Admin / Dashboard</title>") {
		t.Errorf("full page must include layout title; body:\n%s", body)
	}
	if !strings.Contains(body, "Welcome to the admin panel") {
		t.Errorf("full page must include dashboard content; body:\n%s", body)
	}
	if !strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("full page must include admin layout shell; body:\n%s", body)
	}
}

func TestHandler_ShowDashboard_HTMXFragment(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	req.Header.Set("HX-Request", "true")
	h.showDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Welcome to the admin panel") {
		t.Errorf("HTMX fragment must include dashboard content; body:\n%s", body)
	}
	if strings.Contains(body, "<title>") {
		t.Errorf("HTMX fragment must not include layout chrome; body:\n%s", body)
	}
	if strings.Contains(body, `id="admin-shell"`) {
		t.Errorf("HTMX fragment must not include admin shell; body:\n%s", body)
	}
}

func TestHandler_RegisterRoutes_WrapsAdminRouteInMiddleware(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	wrapped := false
	requireAdmin := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped = true
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireAdmin)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)

	if !wrapped {
		t.Errorf("requireAdmin middleware must wrap /admin")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestHandler_RegisterRoutes_RejectsNonGet(t *testing.T) {
	t.Parallel()
	h := newTestHandler()

	requireAdmin := func(next http.Handler) http.Handler { return next }

	mux := http.NewServeMux()
	h.RegisterRoutes(mux, requireAdmin)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin", http.NoBody)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}
