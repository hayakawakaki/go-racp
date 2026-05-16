package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

type fakeTokenRepo struct {
	byHash                 map[[32]byte]*domain.ActionToken
	insertHook             func(*domain.ActionToken) error
	getByHashHook          func([32]byte) (*domain.ActionToken, error)
	deleteUnconsumedHook   func(int, domain.Action) error
	markConsumedHook       func([32]byte, time.Time) error
	mostRecentIssuedAtHook func(int, domain.Action) (time.Time, error)
	insertCalls            []domain.ActionToken
	deleteCalls            []struct {
		AccountID int
		Action    domain.Action
	}
	mu sync.Mutex
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{byHash: map[[32]byte]*domain.ActionToken{}}
}

func (f *fakeTokenRepo) Insert(_ context.Context, token *domain.ActionToken) error {
	if f.insertHook != nil {
		return f.insertHook(token)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *token
	f.byHash[token.TokenHash] = &cp
	f.insertCalls = append(f.insertCalls, cp)
	return nil
}

func (f *fakeTokenRepo) GetByHash(_ context.Context, hash [32]byte) (*domain.ActionToken, error) {
	if f.getByHashHook != nil {
		return f.getByHashHook(hash)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[hash]
	if !ok {
		return nil, domain.ErrTokenInvalid
	}
	cp := *token
	return &cp, nil
}

func (f *fakeTokenRepo) DeleteUnconsumed(_ context.Context, accountID int, action domain.Action) error {
	f.mu.Lock()
	f.deleteCalls = append(f.deleteCalls, struct {
		AccountID int
		Action    domain.Action
	}{accountID, action})
	f.mu.Unlock()
	if f.deleteUnconsumedHook != nil {
		return f.deleteUnconsumedHook(accountID, action)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, token := range f.byHash {
		if token.AccountID == accountID && token.Action == action && !token.ConsumedAt.Valid {
			delete(f.byHash, hash)
		}
	}
	return nil
}

func (f *fakeTokenRepo) MarkConsumed(_ context.Context, hash [32]byte, at time.Time) error {
	if f.markConsumedHook != nil {
		return f.markConsumedHook(hash, at)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[hash]
	if !ok {
		return domain.ErrTokenInvalid
	}
	if token.ConsumedAt.Valid {
		return domain.ErrTokenAlreadyUsed
	}
	token.ConsumedAt.Time = at
	token.ConsumedAt.Valid = true
	return nil
}

func (f *fakeTokenRepo) MostRecentIssuedAt(_ context.Context, accountID int, action domain.Action) (time.Time, error) {
	if f.mostRecentIssuedAtHook != nil {
		return f.mostRecentIssuedAtHook(accountID, action)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest time.Time
	for _, token := range f.byHash {
		if token.AccountID == accountID && token.Action == action && token.CreatedAt.After(latest) {
			latest = token.CreatedAt
		}
	}
	return latest, nil
}

func newManagerWithClock(t *testing.T, clock *testutil.Clock) (*Manager, *fakeTokenRepo) {
	t.Helper()
	repo := newFakeTokenRepo()
	manager := NewManager(repo)
	manager.now = clock.Now
	return manager, repo
}

func TestActionToken_IsExpired(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		expiresAt time.Time
		now       time.Time
		name      string
		want      bool
	}{
		{name: "future", expiresAt: base.Add(time.Hour), now: base, want: false},
		{name: "exactly at expiry", expiresAt: base, now: base, want: true},
		{name: "past", expiresAt: base, now: base.Add(time.Second), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token := &domain.ActionToken{ExpiresAt: tt.expiresAt}
			if got := token.IsExpired(tt.now); got != tt.want {
				t.Errorf("IsExpired(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestActionToken_IsConsumed(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		consumedAt sql.NullTime
		name       string
		want       bool
	}{
		{name: "not consumed", consumedAt: sql.NullTime{Valid: false}, want: false},
		{name: "consumed", consumedAt: sql.NullTime{Time: now, Valid: true}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token := &domain.ActionToken{ConsumedAt: tt.consumedAt}
			if got := token.IsConsumed(); got != tt.want {
				t.Errorf("IsConsumed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_Issue_HappyPath(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC))
	manager, repo := newManagerWithClock(t, clock)

	rawToken, err := manager.Issue(context.Background(), domain.EmailVerification, 42, []byte("payload"), time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil {
		t.Fatalf("rawToken not base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded len = %d, want 32", len(decoded))
	}

	if len(repo.insertCalls) != 1 {
		t.Fatalf("Insert calls = %d, want 1", len(repo.insertCalls))
	}
	stored := repo.insertCalls[0]
	if stored.AccountID != 42 {
		t.Errorf("AccountID = %d, want 42", stored.AccountID)
	}
	if stored.Action != domain.EmailVerification {
		t.Errorf("Action = %v, want EmailVerification", stored.Action)
	}
	if !stored.CreatedAt.Equal(clock.Now()) {
		t.Errorf("CreatedAt = %v, want %v", stored.CreatedAt, clock.Now())
	}
	if !stored.ExpiresAt.Equal(clock.Now().Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want %v", stored.ExpiresAt, clock.Now().Add(time.Hour))
	}
	if string(stored.Payload) != "payload" {
		t.Errorf("Payload = %q, want %q", string(stored.Payload), "payload")
	}
	if stored.TokenHash != sha256.Sum256(decoded) {
		t.Errorf("stored hash does not match sha256(raw)")
	}
}

func TestManager_Issue_DeletesUnconsumedFirst(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)

	if _, err := manager.Issue(context.Background(), domain.PasswordReset, 7, nil, time.Hour); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if len(repo.deleteCalls) != 1 {
		t.Fatalf("DeleteUnconsumed calls = %d, want 1", len(repo.deleteCalls))
	}
	if repo.deleteCalls[0].AccountID != 7 {
		t.Errorf("DeleteUnconsumed accountID = %d, want 7", repo.deleteCalls[0].AccountID)
	}
	if repo.deleteCalls[0].Action != domain.PasswordReset {
		t.Errorf("DeleteUnconsumed action = %v, want PasswordReset", repo.deleteCalls[0].Action)
	}
}

func TestManager_Issue_ProducesDistinctTokens(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, _ := newManagerWithClock(t, clock)

	first, err := manager.Issue(context.Background(), domain.EmailVerification, 1, nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Issue(context.Background(), domain.EmailVerification, 1, nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Errorf("two Issue calls returned the same token")
	}
}

func TestManager_Issue_DeleteError_Wraps(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	repo.deleteUnconsumedHook = func(int, domain.Action) error { return errors.New("delete boom") }

	_, err := manager.Issue(context.Background(), domain.EmailVerification, 1, nil, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "app.Manager.Issue") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestManager_Issue_InsertError_Wraps(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	repo.insertHook = func(*domain.ActionToken) error { return errors.New("insert boom") }

	_, err := manager.Issue(context.Background(), domain.EmailVerification, 1, nil, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "app.Manager.Issue") {
		t.Errorf("not wrapped: %v", err)
	}
}

func seedToken(repo *fakeTokenRepo, action domain.Action, accountID int, suffix byte, expiresAt, createdAt time.Time, payload []byte) (rawToken string, hash [32]byte) {
	var raw [32]byte
	raw[0] = suffix
	hash = sha256.Sum256(raw[:])
	repo.byHash[hash] = &domain.ActionToken{
		TokenHash: hash,
		AccountID: accountID,
		Action:    action,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
		Payload:   payload,
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), hash
}

func TestManager_Consume_HappyPath(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC))
	manager, repo := newManagerWithClock(t, clock)
	rawToken, hash := seedToken(repo, domain.EmailVerification, 1, 0xa1,
		clock.Now().Add(time.Hour), clock.Now(), []byte("p"))

	token, err := manager.Consume(context.Background(), domain.EmailVerification, rawToken)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if token.AccountID != 1 {
		t.Errorf("AccountID = %d, want 1", token.AccountID)
	}
	if !token.ConsumedAt.Valid || !token.ConsumedAt.Time.Equal(clock.Now()) {
		t.Errorf("returned token ConsumedAt = %+v, want valid at %v", token.ConsumedAt, clock.Now())
	}
	if !repo.byHash[hash].ConsumedAt.Valid {
		t.Errorf("stored token not marked consumed")
	}
}

func TestManager_Consume_InvalidEncoding(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, _ := newManagerWithClock(t, clock)

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "not base64", input: "***not-base64***"},
		{name: "wrong length", input: base64.RawURLEncoding.EncodeToString([]byte("short"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := manager.Consume(context.Background(), domain.EmailVerification, tt.input)
			if !errors.Is(err, domain.ErrTokenInvalid) {
				t.Errorf("got %v, want ErrTokenInvalid", err)
			}
		})
	}
}

func TestManager_Consume_UnknownToken(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, _ := newManagerWithClock(t, clock)
	raw := make([]byte, 32)
	rawToken := base64.RawURLEncoding.EncodeToString(raw)

	_, err := manager.Consume(context.Background(), domain.EmailVerification, rawToken)
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestManager_Consume_WrongAction(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	rawToken, _ := seedToken(repo, domain.PasswordReset, 1, 0x01,
		clock.Now().Add(time.Hour), clock.Now(), nil)

	_, err := manager.Consume(context.Background(), domain.EmailVerification, rawToken)
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid for action mismatch", err)
	}
}

func TestManager_Consume_AlreadyConsumed(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	rawToken, hash := seedToken(repo, domain.EmailVerification, 1, 0x02,
		clock.Now().Add(time.Hour), clock.Now(), nil)
	repo.byHash[hash].ConsumedAt = sql.NullTime{Time: clock.Now(), Valid: true}

	_, err := manager.Consume(context.Background(), domain.EmailVerification, rawToken)
	if !errors.Is(err, domain.ErrTokenAlreadyUsed) {
		t.Errorf("got %v, want ErrTokenAlreadyUsed", err)
	}
}

func TestManager_Consume_Expired(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC))
	manager, repo := newManagerWithClock(t, clock)
	rawToken, _ := seedToken(repo, domain.EmailVerification, 1, 0x03,
		clock.Now().Add(-time.Second), clock.Now().Add(-time.Hour), nil)

	_, err := manager.Consume(context.Background(), domain.EmailVerification, rawToken)
	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Errorf("got %v, want ErrTokenExpired", err)
	}
}

func TestManager_Consume_MarkConsumedError_Wraps(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	rawToken, _ := seedToken(repo, domain.EmailVerification, 1, 0x04,
		clock.Now().Add(time.Hour), clock.Now(), nil)
	repo.markConsumedHook = func([32]byte, time.Time) error { return errors.New("mark boom") }

	_, err := manager.Consume(context.Background(), domain.EmailVerification, rawToken)
	if err == nil || !strings.Contains(err.Error(), "app.Manager.Consume") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestManager_Consume_RepoLookupError_Wraps(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	repo.getByHashHook = func([32]byte) (*domain.ActionToken, error) { return nil, errors.New("lookup boom") }

	raw := make([]byte, 32)
	_, err := manager.Consume(context.Background(), domain.EmailVerification, base64.RawURLEncoding.EncodeToString(raw))
	if err == nil || !strings.Contains(err.Error(), "app.Manager.lookup") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestManager_Peek_DoesNotConsume(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	rawToken, hash := seedToken(repo, domain.PasswordReset, 1, 0x05,
		clock.Now().Add(time.Hour), clock.Now(), []byte("p"))

	token, err := manager.Peek(context.Background(), domain.PasswordReset, rawToken)
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	if token.AccountID != 1 {
		t.Errorf("AccountID = %d, want 1", token.AccountID)
	}
	if repo.byHash[hash].ConsumedAt.Valid {
		t.Errorf("Peek must not mark consumed")
	}
}

func TestManager_Peek_PropagatesLookupErrors(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC))
	manager, repo := newManagerWithClock(t, clock)

	t.Run("invalid encoding", func(t *testing.T) {
		t.Parallel()
		_, err := manager.Peek(context.Background(), domain.PasswordReset, "***")
		if !errors.Is(err, domain.ErrTokenInvalid) {
			t.Errorf("got %v, want ErrTokenInvalid", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		rawToken, _ := seedToken(repo, domain.PasswordReset, 1, 0x06,
			clock.Now().Add(-time.Second), clock.Now().Add(-time.Hour), nil)
		_, err := manager.Peek(context.Background(), domain.PasswordReset, rawToken)
		if !errors.Is(err, domain.ErrTokenExpired) {
			t.Errorf("got %v, want ErrTokenExpired", err)
		}
	})
}

func TestManager_MostRecentIssuedAt_PassesThrough(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	want := clock.Now().Add(-time.Minute)
	repo.mostRecentIssuedAtHook = func(int, domain.Action) (time.Time, error) { return want, nil }

	got, err := manager.MostRecentIssuedAt(context.Background(), 1, domain.EmailVerification)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestManager_MostRecentIssuedAt_WrapsError(t *testing.T) {
	t.Parallel()
	clock := testutil.NewClock(time.Now())
	manager, repo := newManagerWithClock(t, clock)
	repo.mostRecentIssuedAtHook = func(int, domain.Action) (time.Time, error) {
		return time.Time{}, errors.New("most-recent boom")
	}

	_, err := manager.MostRecentIssuedAt(context.Background(), 1, domain.EmailVerification)
	if err == nil || !strings.Contains(err.Error(), "app.Manager.MostRecentIssuedAt") {
		t.Errorf("not wrapped: %v", err)
	}
}
