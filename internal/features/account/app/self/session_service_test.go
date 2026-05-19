package self

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

type fakeSessionRepo struct {
	byHash             map[[32]byte]*domain.Session
	createHook         func(*domain.Session) error
	getByTokenHashHook func([32]byte) (*domain.Session, error)
	refreshHook        func([32]byte, time.Time, time.Time) error
	deleteHook         func([32]byte) error
	mu                 sync.Mutex
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{byHash: map[[32]byte]*domain.Session{}}
}

func (f *fakeSessionRepo) Create(_ context.Context, s *domain.Session) error {
	if f.createHook != nil {
		return f.createHook(s)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *s
	f.byHash[s.TokenHash] = &cp
	return nil
}

func (f *fakeSessionRepo) GetByTokenHash(_ context.Context, h [32]byte) (*domain.Session, error) {
	if f.getByTokenHashHook != nil {
		return f.getByTokenHashHook(h)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byHash[h]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeSessionRepo) Refresh(_ context.Context, h [32]byte, lastSeen, expiresAt time.Time) error {
	if f.refreshHook != nil {
		return f.refreshHook(h, lastSeen, expiresAt)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byHash[h]
	if !ok {
		return domain.ErrSessionNotFound
	}
	s.LastSeenAt = lastSeen
	s.ExpiresAt = expiresAt
	return nil
}

func (f *fakeSessionRepo) Delete(_ context.Context, h [32]byte) error {
	if f.deleteHook != nil {
		return f.deleteHook(h)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byHash[h]; !ok {
		return domain.ErrSessionNotFound
	}
	delete(f.byHash, h)
	return nil
}

func (f *fakeSessionRepo) DeleteByUserID(_ context.Context, userID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, sess := range f.byHash {
		if sess.UserID == userID {
			delete(f.byHash, hash)
		}
	}
	return nil
}

func (f *fakeSessionRepo) DeleteByUserIDExcept(_ context.Context, userID int, exceptHash [32]byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, sess := range f.byHash {
		if sess.UserID == userID && hash != exceptHash {
			delete(f.byHash, hash)
		}
	}
	return nil
}

const testSessionTTL = 24 * time.Hour

func newSvc(t *testing.T, c *testutil.Clock) (*SessionService, *fakeSessionRepo) {
	t.Helper()
	repo := newFakeSessionRepo()
	svc := NewSessionService(repo, testSessionTTL)
	svc.now = c.Now
	return svc, repo
}

func TestSessionService_Create_RoundTrip(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))
	svc, repo := newSvc(t, c)

	token, sess, err := svc.Create(context.Background(), 42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.UserID != 42 {
		t.Errorf("UserID = %d, want 42", sess.UserID)
	}
	if !sess.ExpiresAt.Equal(c.Now().Add(testSessionTTL)) {
		t.Errorf("ExpiresAt = %v, want %v", sess.ExpiresAt, c.Now().Add(testSessionTTL))
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token not base64: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("decoded token len = %d, want 32", len(raw))
	}
	wantHash := sha256.Sum256(raw)
	if wantHash != sess.TokenHash {
		t.Errorf("returned session.TokenHash != sha256(rawToken)")
	}

	if _, err := repo.GetByTokenHash(context.Background(), wantHash); err != nil {
		t.Errorf("GetByTokenHash after Create: %v", err)
	}
}

func TestSessionService_Create_ProducesDistinctTokens(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Now())
	svc, _ := newSvc(t, c)

	t1, _, err := svc.Create(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t2, _, err := svc.Create(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if t1 == t2 {
		t.Errorf("two Create calls returned the same token")
	}
}

func TestSessionService_Validate_SlidesExpiry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		advance time.Duration
	}{
		{name: "1 minute in", advance: time.Minute},
		{name: "12 hours in", advance: 12 * time.Hour},
		{name: "23h59m in", advance: 23*time.Hour + 59*time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := testutil.NewClock(time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))
			svc, _ := newSvc(t, c)

			token, _, err := svc.Create(context.Background(), 7)
			if err != nil {
				t.Fatal(err)
			}
			c.Advance(tt.advance)

			sess, err := svc.Validate(context.Background(), token)
			if err != nil {
				t.Fatalf("Validate: %v", err)
			}
			wantExp := c.Now().Add(testSessionTTL)
			if !sess.ExpiresAt.Equal(wantExp) {
				t.Errorf("ExpiresAt = %v, want %v", sess.ExpiresAt, wantExp)
			}
			if !sess.LastSeenAt.Equal(c.Now()) {
				t.Errorf("LastSeenAt = %v, want %v", sess.LastSeenAt, c.Now())
			}
		})
	}
}

func TestSessionService_Validate_Expired(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))
	svc, repo := newSvc(t, c)

	token, sess, err := svc.Create(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(testSessionTTL + time.Nanosecond)

	if _, err := svc.Validate(context.Background(), token); !errors.Is(err, domain.ErrSessionExpired) {
		t.Errorf("Validate got %v, want ErrSessionExpired", err)
	}
	if _, err := repo.GetByTokenHash(context.Background(), sess.TokenHash); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("expired session not deleted: GetByTokenHash err = %v", err)
	}
}

func TestSessionService_Validate_BadInput(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Now())
	svc, _ := newSvc(t, c)

	tests := []struct {
		name  string
		token string
	}{
		{name: "malformed base64", token: "!!!not-base64!!!"},
		{name: "empty", token: ""},
		{name: "wrong length: 16 bytes", token: base64.RawURLEncoding.EncodeToString(make([]byte, 16))},
		{name: "wrong length: 64 bytes", token: base64.RawURLEncoding.EncodeToString(make([]byte, 64))},
		{name: "well-formed but unknown", token: base64.RawURLEncoding.EncodeToString(make([]byte, 32))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.Validate(context.Background(), tt.token)
			if !errors.Is(err, domain.ErrSessionNotFound) {
				t.Errorf("token %q: got %v, want ErrSessionNotFound", tt.token, err)
			}
		})
	}
}

func TestSessionService_Destroy(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Now())
	svc, repo := newSvc(t, c)

	token, sess, err := svc.Create(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Destroy(context.Background(), token); err != nil {
		t.Errorf("Destroy: %v", err)
	}
	if _, err := repo.GetByTokenHash(context.Background(), sess.TokenHash); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("row not deleted")
	}

	if err := svc.Destroy(context.Background(), token); err != nil {
		t.Errorf("Destroy on missing: %v", err)
	}
	if err := svc.Destroy(context.Background(), "!!!"); err != nil {
		t.Errorf("Destroy on malformed: %v", err)
	}
}

func TestSessionService_InvalidateAll(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))
	svc, repo := newSvc(t, c)
	ctx := context.Background()

	_, _, err := svc.Create(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Create(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, sessOther, err := svc.Create(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.InvalidateAll(ctx, 1); err != nil {
		t.Fatalf("InvalidateAll: %v", err)
	}
	for h, s := range repo.byHash {
		if s.UserID == 1 {
			t.Errorf("user 1 session %x still present", h)
		}
	}
	if _, err := repo.GetByTokenHash(ctx, sessOther.TokenHash); err != nil {
		t.Errorf("user 2 session deleted: %v", err)
	}
}

func TestSessionService_InvalidateOthers(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))
	svc, repo := newSvc(t, c)
	ctx := context.Background()

	currentToken, currentSess, err := svc.Create(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, otherSess, err := svc.Create(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.InvalidateOthers(ctx, 1, currentToken); err != nil {
		t.Fatalf("InvalidateOthers: %v", err)
	}
	if _, err := repo.GetByTokenHash(ctx, currentSess.TokenHash); err != nil {
		t.Errorf("current session must survive: %v", err)
	}
	if _, err := repo.GetByTokenHash(ctx, otherSess.TokenHash); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("other session not deleted: %v", err)
	}
}

func TestSessionService_InvalidateOthers_RejectsBadToken(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Now())
	svc, _ := newSvc(t, c)

	tests := []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "not base64", token: "***"},
		{name: "wrong length", token: base64.RawURLEncoding.EncodeToString(make([]byte, 16))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := svc.InvalidateOthers(context.Background(), 1, tt.token)
			if !errors.Is(err, domain.ErrInvalidCurrentSessionToken) {
				t.Errorf("got %v, want ErrInvalidCurrentSessionToken", err)
			}
		})
	}
}

func TestSessionService_Validate_WrapsRepoError(t *testing.T) {
	t.Parallel()
	c := testutil.NewClock(time.Now())
	repo := newFakeSessionRepo()
	repo.getByTokenHashHook = func([32]byte) (*domain.Session, error) { return nil, errors.New("boom") }
	svc := NewSessionService(repo, testSessionTTL)
	svc.now = c.Now

	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	_, err := svc.Validate(context.Background(), token)
	if err == nil || errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("Validate got %v, want a wrapped non-domain error", err)
	}
	if !strings.Contains(err.Error(), "app.SessionService.Validate") {
		t.Errorf("error not wrapped with package/method prefix: %v", err)
	}
}
