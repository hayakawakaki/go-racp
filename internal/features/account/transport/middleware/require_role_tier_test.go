package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func newTierMiddleware(sess *stubSessionService, users *stubUserLookup, hidden bool, policy AuthPolicy, allowed ...domain.Role) func(http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	resolver := newTestRoleResolver()
	if hidden {
		return RequireRoleHidden(sess, users, resolver, logger, false, httpx.Layout{}, policy, allowed...)
	}
	return RequireRole(sess, users, resolver, logger, false, policy, allowed...)
}

func userWithState(state, groupID int, unbanTime time.Time) func(context.Context, int) (*domain.User, error) {
	return func(_ context.Context, id int) (*domain.User, error) {
		return &domain.User{ID: id, GroupID: groupID, State: state, UnbanTime: unbanTime}, nil
	}
}

func TestRequireRole_TierMatrix(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(2 * time.Hour)

	type want struct {
		location   string
		status     int
		called     bool
		destroyed  bool
		cookieGone bool
		snapshot   bool
	}

	tests := []struct {
		userFn  func(context.Context, int) (*domain.User, error)
		name    string
		allowed []domain.Role
		want    want
		policy  AuthPolicy
		hidden  bool
	}{
		{
			name:    "perma banned destroys session and redirects",
			userFn:  userWithState(5, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:     http.StatusSeeOther,
				location:   "/login?notice=banned",
				destroyed:  true,
				cookieGone: true,
			},
		},
		{
			name:    "perma banned hidden returns 404",
			userFn:  userWithState(5, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{domain.RoleAdmin},
			hidden:  true,
			want: want{
				status:     http.StatusNotFound,
				destroyed:  true,
				cookieGone: true,
			},
		},
		{
			name: "deleted account (user not found) destroys and redirects with deleted notice",
			userFn: func(context.Context, int) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:     http.StatusSeeOther,
				location:   "/login?notice=deleted",
				destroyed:  true,
				cookieGone: true,
			},
		},
		{
			name: "deleted hidden returns 404",
			userFn: func(context.Context, int) (*domain.User, error) {
				return nil, domain.ErrUserNotFound
			},
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{domain.RoleAdmin},
			hidden:  true,
			want: want{
				status:     http.StatusNotFound,
				destroyed:  true,
				cookieGone: true,
			},
		},
		{
			name:    "unverified on verified route redirects to verify-account and preserves session",
			userFn:  userWithState(1, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true, RequireVerified: true},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:    http.StatusSeeOther,
				location:  "/verify-account",
				destroyed: false,
			},
		},
		{
			name:    "unverified on verified route hidden returns 404",
			userFn:  userWithState(1, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true, RequireVerified: true},
			allowed: []domain.Role{domain.RoleAdmin},
			hidden:  true,
			want: want{
				status:    http.StatusNotFound,
				destroyed: false,
			},
		},
		{
			name:    "unverified on member route passes through with snapshot",
			userFn:  userWithState(1, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true, RequireVerified: false},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:   http.StatusOK,
				called:   true,
				snapshot: true,
			},
		},
		{
			name:    "temp banned on unrestricted route soft-blocks to account",
			userFn:  userWithState(0, 0, future),
			policy:  AuthPolicy{AllowTempBannedLogin: true, Unrestricted: true},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:    http.StatusSeeOther,
				location:  "/account?notice=ban_blocked",
				destroyed: false,
			},
		},
		{
			name:    "temp banned on unrestricted route hidden returns 404",
			userFn:  userWithState(0, 0, future),
			policy:  AuthPolicy{AllowTempBannedLogin: true, Unrestricted: true},
			allowed: []domain.Role{domain.RoleAdmin},
			hidden:  true,
			want: want{
				status:    http.StatusNotFound,
				destroyed: false,
			},
		},
		{
			name:    "temp banned passes through when route does not require unrestricted",
			userFn:  userWithState(0, 0, future),
			policy:  AuthPolicy{AllowTempBannedLogin: true, Unrestricted: false},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:   http.StatusOK,
				called:   true,
				snapshot: true,
			},
		},
		{
			name:    "temp banned with AllowTempBannedLogin=false is treated as banned",
			userFn:  userWithState(0, 0, future),
			policy:  AuthPolicy{AllowTempBannedLogin: false},
			allowed: []domain.Role{domain.RoleAuthenticated},
			want: want{
				status:     http.StatusSeeOther,
				location:   "/login?notice=banned",
				destroyed:  true,
				cookieGone: true,
			},
		},
		{
			name:    "active matching role passes through with snapshot",
			userFn:  userWithState(0, 20, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{testRoleModerator},
			want: want{
				status:   http.StatusOK,
				called:   true,
				snapshot: true,
			},
		},
		{
			name:    "active mismatched role returns 403",
			userFn:  userWithState(0, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{testRoleModerator},
			want: want{
				status: http.StatusForbidden,
			},
		},
		{
			name:    "active mismatched role hidden returns 404",
			userFn:  userWithState(0, 0, time.Time{}),
			policy:  AuthPolicy{AllowTempBannedLogin: true},
			allowed: []domain.Role{testRoleModerator},
			hidden:  true,
			want: want{
				status: http.StatusNotFound,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sess := &stubSessionService{
				validateFn: func(context.Context, string) (*domain.Session, error) {
					return &domain.Session{UserID: 42}, nil
				},
			}
			users := &stubUserLookup{getByIDFn: tt.userFn}
			mw := newTierMiddleware(sess, users, tt.hidden, tt.policy, tt.allowed...)

			var called bool
			var gotSnap *AccountSnapshot
			var snapPresent bool
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotSnap, snapPresent = SnapshotFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-raw"})
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.want.status {
				t.Errorf("status = %d, want %d", rr.Code, tt.want.status)
			}
			if tt.want.location != "" {
				if got := rr.Header().Get("Location"); got != tt.want.location {
					t.Errorf("Location = %q, want %q", got, tt.want.location)
				}
			}
			if tt.hidden && rr.Header().Get("Location") != "" {
				t.Errorf("hidden mode must not set Location, got %q", rr.Header().Get("Location"))
			}
			if called != tt.want.called {
				t.Errorf("downstream called = %v, want %v", called, tt.want.called)
			}
			if got := len(sess.destroyCalls) > 0; got != tt.want.destroyed {
				t.Errorf("session destroyed = %v, want %v (destroyCalls=%v)", got, tt.want.destroyed, sess.destroyCalls)
			}
			if tt.want.destroyed && (len(sess.destroyCalls) != 1 || sess.destroyCalls[0] != "session-raw") {
				t.Errorf("destroy called with %v, want exactly [\"session-raw\"]", sess.destroyCalls)
			}
			cookie := findSetCookie(rr, SessionCookieName)
			if tt.want.cookieGone {
				if cookie == nil || cookie.MaxAge >= 0 {
					t.Errorf("expected cookie cleared, got %+v", cookie)
				}
			}
			if tt.want.snapshot {
				if !snapPresent {
					t.Fatalf("expected snapshot in context")
				}
				if gotSnap.UserID != 42 {
					t.Errorf("snapshot.UserID = %d, want 42", gotSnap.UserID)
				}
			}
		})
	}
}
