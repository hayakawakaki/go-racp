package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/account/app"
	"github.com/hayakawakaki/go-racp/internal/account/domain"
	accounttransport "github.com/hayakawakaki/go-racp/internal/account/transport"
)

type stubUserService struct {
	getByIDFn func(context.Context, int) (*app.GetDTO, error)
}

func (s *stubUserService) GetByID(ctx context.Context, id int) (*app.GetDTO, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &app.GetDTO{ID: id, Username: "stub-user"}, nil
}

func newTestHandler(svc *stubUserService, logBuf io.Writer) *Handler {
	if logBuf == nil {
		logBuf = io.Discard
	}
	return &Handler{
		userSvc: svc,
		logger:  slog.New(slog.NewTextHandler(logBuf, nil)),
	}
}

func TestHome_ShowAnonymous(t *testing.T) {
	t.Parallel()
	h := newTestHandler(&stubUserService{}, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	h.show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Welcome, stranger") {
		t.Errorf("anonymous body missing 'Welcome, stranger': %s", body)
	}
	if !strings.Contains(body, `href="/login"`) {
		t.Errorf("anonymous body missing login link: %s", body)
	}
	if strings.Contains(body, `action="/logout"`) {
		t.Errorf("anonymous body should not contain logout form")
	}
}

func TestHome_ShowAuthenticated(t *testing.T) {
	t.Parallel()
	svc := &stubUserService{
		getByIDFn: func(_ context.Context, id int) (*app.GetDTO, error) {
			return &app.GetDTO{ID: id, Username: "alice"}, nil
		},
	}
	h := newTestHandler(svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	ctx := accounttransport.ContextWithSession(req.Context(), &domain.Session{UserID: 7})
	h.show(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Welcome, alice") {
		t.Errorf("authenticated body missing 'Welcome, alice': %s", body)
	}
	if !strings.Contains(body, `action="/logout"`) {
		t.Errorf("authenticated body missing logout form: %s", body)
	}
}

func TestHome_ShowAuthenticated_UserFetchFails(t *testing.T) {
	t.Parallel()
	svc := &stubUserService{
		getByIDFn: func(context.Context, int) (*app.GetDTO, error) {
			return nil, errors.New("db down")
		},
	}
	logs := &strings.Builder{}
	h := newTestHandler(svc, logs)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	ctx := accounttransport.ContextWithSession(req.Context(), &domain.Session{UserID: 7})
	h.show(rr, req.WithContext(ctx))

	// Falls back to anonymous render rather than 500.
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (fallback render)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Welcome, stranger") {
		t.Errorf("expected anonymous fallback, got: %s", rr.Body.String())
	}
	if !strings.Contains(logs.String(), "home: fetch user") {
		t.Errorf("expected fetch-user error logged, got: %s", logs.String())
	}
}
