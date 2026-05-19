package transport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/account/app"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
)

func newAuthHandlerWithPolicy(auth *stubAccountService, sess *stubSessionService, allowTempBannedLogin bool) *Handler {
	return &Handler{
		svc:                  auth,
		sessSvc:              sess,
		logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		secure:               false,
		allowTempBannedLogin: allowTempBannedLogin,
	}
}

func TestDoLogin_PermaBanned_NoSession_RendersBannedMessage(t *testing.T) {
	t.Parallel()

	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return &app.GetDTO{ID: 5, Username: "testuser"}, app.TierPermaBanned, nil
		},
	}
	sess := &stubSessionService{}
	h := newAuthHandlerWithPolicy(auth, sess, true)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
	h.doLogin(rr, req)

	if len(sess.createCalls) != 0 {
		t.Errorf("session.Create must not be called for perma-banned login; got %v", sess.createCalls)
	}
	if findSetCookie(rr, middleware.SessionCookieName) != nil {
		t.Errorf("cookie must not be set for perma-banned login")
	}
	if !strings.Contains(rr.Body.String(), "permanently banned") {
		t.Errorf("body missing permanently banned message: %s", rr.Body.String())
	}
}

func TestDoLogin_TempBanned_AllowTempBannedLogin_CreatesSessionAndRedirects(t *testing.T) {
	t.Parallel()

	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return &app.GetDTO{ID: 7, Username: "testuser"}, app.TierTempBanned, nil
		},
	}
	sess := &stubSessionService{
		createFn: func(_ context.Context, userID int) (string, *domain.Session, error) {
			return "issued-token", &domain.Session{UserID: userID}, nil
		},
	}
	h := newAuthHandlerWithPolicy(auth, sess, true)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
	h.doLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", rr.Header().Get("Location"))
	}
	if len(sess.createCalls) != 1 || sess.createCalls[0] != 7 {
		t.Errorf("createCalls = %v, want [7]", sess.createCalls)
	}
	if findSetCookie(rr, middleware.SessionCookieName) == nil {
		t.Errorf("expected session cookie to be set")
	}
}

func TestDoLogin_TempBanned_NotAllowed_NoSession_RendersRestrictedMessage(t *testing.T) {
	t.Parallel()

	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return &app.GetDTO{ID: 7, Username: "testuser"}, app.TierTempBanned, nil
		},
	}
	sess := &stubSessionService{}
	h := newAuthHandlerWithPolicy(auth, sess, false)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
	h.doLogin(rr, req)

	if len(sess.createCalls) != 0 {
		t.Errorf("session.Create must not be called when temp-ban login is forbidden; got %v", sess.createCalls)
	}
	if findSetCookie(rr, middleware.SessionCookieName) != nil {
		t.Errorf("cookie must not be set when temp-ban login is forbidden")
	}
	if !strings.Contains(rr.Body.String(), "restricted") {
		t.Errorf("body missing restricted message: %s", rr.Body.String())
	}
}

func TestDoLogin_Unverified_CreatesSession_RedirectsToVerifyAccount(t *testing.T) {
	t.Parallel()

	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return &app.GetDTO{ID: 11, Username: "testuser"}, app.TierUnverified, nil
		},
	}
	sess := &stubSessionService{
		createFn: func(_ context.Context, userID int) (string, *domain.Session, error) {
			return "issued-token", &domain.Session{UserID: userID}, nil
		},
	}
	h := newAuthHandlerWithPolicy(auth, sess, true)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
	h.doLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/verify-account" {
		t.Errorf("Location = %q, want /verify-account", rr.Header().Get("Location"))
	}
	if len(sess.createCalls) != 1 {
		t.Errorf("expected session.Create to be called once for unverified; got %v", sess.createCalls)
	}
	if findSetCookie(rr, middleware.SessionCookieName) == nil {
		t.Errorf("expected session cookie for unverified tier")
	}
}

func TestDoLogin_Active_CreatesSession_RedirectsToHome(t *testing.T) {
	t.Parallel()

	auth := &stubAccountService{
		authNFn: func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error) {
			return &app.GetDTO{ID: 13, Username: "testuser"}, app.TierActive, nil
		},
	}
	sess := &stubSessionService{
		createFn: func(_ context.Context, userID int) (string, *domain.Session, error) {
			return "issued-token", &domain.Session{UserID: userID}, nil
		},
	}
	h := newAuthHandlerWithPolicy(auth, sess, true)

	rr := httptest.NewRecorder()
	req := postForm("/login", map[string]string{"username": "testuser", "password": "Test1234!"})
	h.doLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if rr.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", rr.Header().Get("Location"))
	}
	if len(sess.createCalls) != 1 || sess.createCalls[0] != 13 {
		t.Errorf("createCalls = %v, want [13]", sess.createCalls)
	}
}
