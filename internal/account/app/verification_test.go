package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
)

type fakeActionTokenRepo struct {
	byHash                 map[[32]byte]*actiontoken.ActionToken
	insertHook             func(*actiontoken.ActionToken) error
	getByHashHook          func([32]byte) (*actiontoken.ActionToken, error)
	deleteUnconsumedHook   func(int, actiontoken.Action) error
	markConsumedHook       func([32]byte, time.Time) error
	mostRecentIssuedAtHook func(int, actiontoken.Action) (time.Time, error)
	insertCalls            []actiontoken.ActionToken
	deleteCalls            []struct {
		AccountID int
		Action    actiontoken.Action
	}
	mu sync.Mutex
}

func newFakeActionTokenRepo() *fakeActionTokenRepo {
	return &fakeActionTokenRepo{byHash: map[[32]byte]*actiontoken.ActionToken{}}
}

func (f *fakeActionTokenRepo) Insert(_ context.Context, token *actiontoken.ActionToken) error {
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

func (f *fakeActionTokenRepo) GetByHash(_ context.Context, hash [32]byte) (*actiontoken.ActionToken, error) {
	if f.getByHashHook != nil {
		return f.getByHashHook(hash)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[hash]
	if !ok {
		return nil, actiontoken.ErrTokenInvalid
	}
	cp := *token
	return &cp, nil
}

func (f *fakeActionTokenRepo) DeleteUnconsumed(_ context.Context, accountID int, action actiontoken.Action) error {
	f.mu.Lock()
	f.deleteCalls = append(f.deleteCalls, struct {
		AccountID int
		Action    actiontoken.Action
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

func (f *fakeActionTokenRepo) MarkConsumed(_ context.Context, hash [32]byte, at time.Time) error {
	if f.markConsumedHook != nil {
		return f.markConsumedHook(hash, at)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[hash]
	if !ok {
		return actiontoken.ErrTokenInvalid
	}
	if token.ConsumedAt.Valid {
		return actiontoken.ErrTokenAlreadyUsed
	}
	token.ConsumedAt.Time = at
	token.ConsumedAt.Valid = true
	return nil
}

func (f *fakeActionTokenRepo) MostRecentIssuedAt(_ context.Context, accountID int, action actiontoken.Action) (time.Time, error) {
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

type fakeChangeLog struct {
	byKey          map[changeKey]time.Time
	recordHook     func(int, domain.ChangeType, time.Time) error
	mostRecentHook func(int, domain.ChangeType) (time.Time, error)
	mu             sync.Mutex
}

type changeKey struct {
	AccountID  int
	ChangeType domain.ChangeType
}

func newFakeChangeLog() *fakeChangeLog {
	return &fakeChangeLog{byKey: map[changeKey]time.Time{}}
}

func (f *fakeChangeLog) Record(_ context.Context, accountID int, changeType domain.ChangeType, at time.Time) error {
	if f.recordHook != nil {
		return f.recordHook(accountID, changeType, at)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byKey[changeKey{accountID, changeType}] = at
	return nil
}

func (f *fakeChangeLog) MostRecent(_ context.Context, accountID int, changeType domain.ChangeType) (time.Time, error) {
	if f.mostRecentHook != nil {
		return f.mostRecentHook(accountID, changeType)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byKey[changeKey{accountID, changeType}], nil
}

type fakeMailer struct {
	sent []sentMail
	mu   sync.Mutex
}

type sentMail struct {
	To      string
	Subject string
	Body    string
}

func (m *fakeMailer) SendAsync(to, subject, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, sentMail{To: to, Subject: subject, Body: body})
}

func (m *fakeMailer) Sent() []sentMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]sentMail, len(m.sent))
	copy(out, m.sent)
	return out
}

type fakeSessionInvalidator struct {
	invalidateAllHook        func(int) error
	invalidateExceptCurrHook func(int, string) error
	invalidateAllCalls       []int
	invalidateExceptCalls    []invalidateExceptCall
	mu                       sync.Mutex
}

type invalidateExceptCall struct {
	CurrentRawToken string
	UserID          int
}

func (f *fakeSessionInvalidator) InvalidateAll(_ context.Context, userID int) error {
	f.mu.Lock()
	f.invalidateAllCalls = append(f.invalidateAllCalls, userID)
	f.mu.Unlock()
	if f.invalidateAllHook != nil {
		return f.invalidateAllHook(userID)
	}
	return nil
}

func (f *fakeSessionInvalidator) InvalidateOthers(_ context.Context, userID int, rawToken string) error {
	f.mu.Lock()
	f.invalidateExceptCalls = append(f.invalidateExceptCalls, invalidateExceptCall{CurrentRawToken: rawToken, UserID: userID})
	f.mu.Unlock()
	if f.invalidateExceptCurrHook != nil {
		return f.invalidateExceptCurrHook(userID, rawToken)
	}
	return nil
}

func newVerificationConfig() VerificationConfig {
	return VerificationConfig{
		AppURL:         "https://cp.example/",
		ServerName:     "Test rAthena",
		TokenTTL:       24 * time.Hour,
		ResendCooldown: 60 * time.Second,
	}
}

func newPasswordResetConfig() PasswordResetConfig {
	return PasswordResetConfig{
		AppURL:         "https://cp.example/",
		ServerName:     "Test rAthena",
		TokenTTL:       30 * time.Minute,
		ResendCooldown: 60 * time.Second,
		ChangeCooldown: 24 * time.Hour,
	}
}

func newServiceWithVerification(t *testing.T) (*Service, *fakeUserRepo, *fakeActionTokenRepo, *fakeMailer) {
	t.Helper()
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeActionTokenRepo()
	mailer := &fakeMailer{}
	manager := actiontoken.NewManager(tokenRepo)
	svc := NewService(userRepo, WithVerification(manager, mailer, newVerificationConfig()))
	return svc, userRepo, tokenRepo, mailer
}

type resetFixture struct {
	svc         *Service
	userRepo    *fakeUserRepo
	tokenRepo   *fakeActionTokenRepo
	changeLog   *fakeChangeLog
	mailer      *fakeMailer
	invalidator *fakeSessionInvalidator
}

func newServiceWithReset(t *testing.T) *resetFixture {
	t.Helper()
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeActionTokenRepo()
	changeLog := newFakeChangeLog()
	mailer := &fakeMailer{}
	invalidator := &fakeSessionInvalidator{}
	manager := actiontoken.NewManager(tokenRepo)
	svc := NewService(userRepo,
		WithPasswordReset(manager, mailer, newPasswordResetConfig()),
		WithChangeLog(changeLog),
		WithSessionInvalidator(invalidator),
	)
	return &resetFixture{
		svc:         svc,
		userRepo:    userRepo,
		tokenRepo:   tokenRepo,
		changeLog:   changeLog,
		mailer:      mailer,
		invalidator: invalidator,
	}
}

func extractRawTokenFromBody(t *testing.T, body string) string {
	t.Helper()
	_, rest, found := strings.Cut(body, "token=")
	if !found {
		t.Fatalf("body missing token= marker: %s", body)
	}
	endIdx := strings.IndexAny(rest, "\"' <>&")
	if endIdx < 0 {
		t.Fatalf("body has no terminator after token: %s", rest)
	}
	raw, err := url.QueryUnescape(rest[:endIdx])
	if err != nil {
		t.Fatalf("url.QueryUnescape: %v", err)
	}
	return raw
}

func TestWithVerification_PanicsOnInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  VerificationConfig
	}{
		{name: "zero TTL", cfg: VerificationConfig{TokenTTL: 0, ResendCooldown: time.Second}},
		{name: "negative TTL", cfg: VerificationConfig{TokenTTL: -time.Second, ResendCooldown: time.Second}},
		{name: "zero resend cooldown", cfg: VerificationConfig{TokenTTL: time.Hour, ResendCooldown: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for cfg=%+v", tt.cfg)
				}
			}()
			WithVerification(actiontoken.NewManager(newFakeActionTokenRepo()), &fakeMailer{}, tt.cfg)
		})
	}
}

func TestWithPasswordReset_PanicsOnInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  PasswordResetConfig
	}{
		{name: "zero TTL", cfg: PasswordResetConfig{TokenTTL: 0, ResendCooldown: time.Second, ChangeCooldown: time.Hour}},
		{name: "zero resend cooldown", cfg: PasswordResetConfig{TokenTTL: time.Hour, ResendCooldown: 0, ChangeCooldown: time.Hour}},
		{name: "zero change cooldown", cfg: PasswordResetConfig{TokenTTL: time.Hour, ResendCooldown: time.Second, ChangeCooldown: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for cfg=%+v", tt.cfg)
				}
			}()
			WithPasswordReset(actiontoken.NewManager(newFakeActionTokenRepo()), &fakeMailer{}, tt.cfg)
		})
	}
}

func TestNewService_PanicsOnMissingRepo(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for nil repo")
		}
	}()
	NewService(nil)
}

func TestNewService_PasswordResetRequiresChangeLogAndInvalidator(t *testing.T) {
	t.Parallel()
	manager := actiontoken.NewManager(newFakeActionTokenRepo())

	t.Run("missing change log panics", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic when WithChangeLog is missing")
			}
		}()
		NewService(newFakeUserRepo(),
			WithPasswordReset(manager, &fakeMailer{}, newPasswordResetConfig()),
			WithSessionInvalidator(&fakeSessionInvalidator{}),
		)
	})

	t.Run("missing session invalidator panics", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic when WithSessionInvalidator is missing")
			}
		}()
		NewService(newFakeUserRepo(),
			WithPasswordReset(manager, &fakeMailer{}, newPasswordResetConfig()),
			WithChangeLog(newFakeChangeLog()),
		)
	})
}

func TestService_Create_WithVerificationDeps_IssuesToken(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, mailer := newServiceWithVerification(t)

	dto, err := svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.com",
		Gender:          "F",
		Birthdate:       "2000-01-01",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(tokenRepo.insertCalls) != 1 {
		t.Fatalf("token Insert calls = %d, want 1", len(tokenRepo.insertCalls))
	}
	if tokenRepo.insertCalls[0].AccountID != dto.ID {
		t.Errorf("token AccountID = %d, want %d", tokenRepo.insertCalls[0].AccountID, dto.ID)
	}
	if tokenRepo.insertCalls[0].Action != actiontoken.EmailVerification {
		t.Errorf("token Action = %v, want EmailVerification", tokenRepo.insertCalls[0].Action)
	}
	sent := mailer.Sent()
	if len(sent) != 1 {
		t.Fatalf("Sent len = %d, want 1", len(sent))
	}
	if sent[0].To != "test@example.com" {
		t.Errorf("To = %q, want test@example.com", sent[0].To)
	}
	if !strings.Contains(sent[0].Subject, "Test rAthena") {
		t.Errorf("Subject = %q, want server name embedded", sent[0].Subject)
	}
}

func TestService_Create_WithoutVerificationDeps_NoTokenIssued(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeUserRepo())

	dto, err := svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.com",
		Gender:          "F",
		Birthdate:       "2000-01-01",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if dto.ID == 0 {
		t.Errorf("expected ID assigned")
	}
}

func TestService_Create_VerificationIssueError_Wraps(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	tokenRepo.insertHook = func(*actiontoken.ActionToken) error { return errors.New("token insert boom") }

	_, err := svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.com",
		Gender:          "F",
		Birthdate:       "2000-01-01",
	})
	if err == nil || !strings.Contains(err.Error(), "app.Service.Create") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_IssueVerification_StoresTokenAndSendsMail(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, mailer := newServiceWithVerification(t)

	before := time.Now()
	if err := svc.IssueVerification(context.Background(), 42, "user@example.com", "user"); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	after := time.Now()

	if len(tokenRepo.deleteCalls) != 1 {
		t.Errorf("DeleteUnconsumed calls = %d, want 1", len(tokenRepo.deleteCalls))
	}
	if len(tokenRepo.insertCalls) != 1 {
		t.Fatalf("Insert calls = %d, want 1", len(tokenRepo.insertCalls))
	}
	stored := tokenRepo.insertCalls[0]
	wantTTL := newVerificationConfig().TokenTTL
	gotTTL := stored.ExpiresAt.Sub(stored.CreatedAt)
	if gotTTL != wantTTL {
		t.Errorf("ExpiresAt - CreatedAt = %v, want %v (TTL)", gotTTL, wantTTL)
	}
	if stored.CreatedAt.Before(before) || stored.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, expected within [%v, %v]", stored.CreatedAt, before, after)
	}
	if len(mailer.Sent()) != 1 {
		t.Errorf("expected 1 mail sent")
	}
}

func TestService_IssueVerification_EmailURLContainsRoundTrippableToken(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, mailer := newServiceWithVerification(t)

	if err := svc.IssueVerification(context.Background(), 1, "user@example.com", "user"); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	body := mailer.Sent()[0].Body
	if !strings.Contains(body, "https://cp.example/verify?token=") {
		t.Errorf("body missing verify URL: %s", body)
	}
	rawToken := extractRawTokenFromBody(t, body)
	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil {
		t.Fatalf("token not RawURLEncoded: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("decoded len = %d, want 32", len(decoded))
	}
	if _, ok := tokenRepo.byHash[sha256.Sum256(decoded)]; !ok {
		t.Errorf("stored token hash does not match sha256 of raw")
	}
}

func TestService_IssueVerification_TrimsTrailingSlashInAppURL(t *testing.T) {
	t.Parallel()
	tokenRepo := newFakeActionTokenRepo()
	mailer := &fakeMailer{}
	cfg := VerificationConfig{
		AppURL:         "https://cp.example///",
		ServerName:     "X",
		TokenTTL:       time.Hour,
		ResendCooldown: 60 * time.Second,
	}
	svc := NewService(newFakeUserRepo(),
		WithVerification(actiontoken.NewManager(tokenRepo), mailer, cfg))

	if err := svc.IssueVerification(context.Background(), 1, "u@x", "u"); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	body := mailer.Sent()[0].Body
	if strings.Contains(body, "////verify") || strings.Contains(body, "///verify") {
		t.Errorf("trailing slashes not trimmed: %s", body)
	}
}

func TestService_IssueVerification_TokenError_Wraps(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	tokenRepo.deleteUnconsumedHook = func(int, actiontoken.Action) error { return errors.New("delete boom") }

	err := svc.IssueVerification(context.Background(), 1, "u@x", "u")
	if err == nil || !strings.Contains(err.Error(), "app.Service.IssueVerification") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_ConsumeVerification_HappyPath(t *testing.T) {
	t.Parallel()
	svc, userRepo, _, mailer := newServiceWithVerification(t)
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", State: 1,
	})

	if err := svc.IssueVerification(context.Background(), user.ID, user.Email, user.Username); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	rawToken := extractRawTokenFromBody(t, mailer.Sent()[0].Body)

	if err := svc.ConsumeVerification(context.Background(), rawToken); err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}
	got, _ := userRepo.GetByID(context.Background(), user.ID)
	if got.State != 0 {
		t.Errorf("State = %d, want 0 (verified)", got.State)
	}
}

func TestService_ConsumeVerification_PropagatesTokenErrors(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	now := time.Now()

	tests := []struct {
		setup   func() string
		wantErr error
		name    string
	}{
		{
			name: "invalid encoding",
			setup: func() string {
				return "***not-base64***"
			},
			wantErr: actiontoken.ErrTokenInvalid,
		},
		{
			name: "unknown token",
			setup: func() string {
				raw := make([]byte, 32)
				return base64.RawURLEncoding.EncodeToString(raw)
			},
			wantErr: actiontoken.ErrTokenInvalid,
		},
		{
			name: "wrong action",
			setup: func() string {
				var raw [32]byte
				raw[0] = 0xa1
				hash := sha256.Sum256(raw[:])
				tokenRepo.byHash[hash] = &actiontoken.ActionToken{
					TokenHash: hash, AccountID: 1, Action: actiontoken.PasswordReset,
					ExpiresAt: now.Add(time.Hour), CreatedAt: now,
				}
				return base64.RawURLEncoding.EncodeToString(raw[:])
			},
			wantErr: actiontoken.ErrTokenInvalid,
		},
		{
			name: "expired",
			setup: func() string {
				var raw [32]byte
				raw[0] = 0xa2
				hash := sha256.Sum256(raw[:])
				tokenRepo.byHash[hash] = &actiontoken.ActionToken{
					TokenHash: hash, AccountID: 1, Action: actiontoken.EmailVerification,
					ExpiresAt: now.Add(-time.Second), CreatedAt: now.Add(-time.Hour),
				}
				return base64.RawURLEncoding.EncodeToString(raw[:])
			},
			wantErr: actiontoken.ErrTokenExpired,
		},
		{
			name: "already consumed",
			setup: func() string {
				var raw [32]byte
				raw[0] = 0xa3
				hash := sha256.Sum256(raw[:])
				tokenRepo.byHash[hash] = &actiontoken.ActionToken{
					TokenHash: hash, AccountID: 1, Action: actiontoken.EmailVerification,
					ExpiresAt:  now.Add(time.Hour),
					CreatedAt:  now,
					ConsumedAt: sql.NullTime{Time: now, Valid: true},
				}
				return base64.RawURLEncoding.EncodeToString(raw[:])
			},
			wantErr: actiontoken.ErrTokenAlreadyUsed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawToken := tt.setup()
			err := svc.ConsumeVerification(context.Background(), rawToken)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_ConsumeVerification_MarkVerifiedError_Wraps(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, _ := newServiceWithVerification(t)
	var raw [32]byte
	raw[0] = 0xb1
	hash := sha256.Sum256(raw[:])
	tokenRepo.byHash[hash] = &actiontoken.ActionToken{
		TokenHash: hash, AccountID: 99, Action: actiontoken.EmailVerification,
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}
	userRepo.markVerifiedHook = func(int) error { return errors.New("mark verified boom") }

	err := svc.ConsumeVerification(context.Background(), base64.RawURLEncoding.EncodeToString(raw[:]))
	if err == nil || !strings.Contains(err.Error(), "app.Service.ConsumeVerification") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_ResendVerification_AlreadyVerifiedSilent(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, mailer := newServiceWithVerification(t)
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", State: 0,
	})

	if err := svc.ResendVerification(context.Background(), user.ID); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	if len(mailer.Sent()) != 0 {
		t.Errorf("verified user should not receive mail; sent = %d", len(mailer.Sent()))
	}
	if len(tokenRepo.insertCalls) != 0 {
		t.Errorf("verified user should not get a new token; inserts = %d", len(tokenRepo.insertCalls))
	}
}

func TestService_ResendVerification_ThrottledSilent(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, mailer := newServiceWithVerification(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", State: 1,
	})
	tokenRepo.mostRecentIssuedAtHook = func(int, actiontoken.Action) (time.Time, error) {
		return fixed.Add(-30 * time.Second), nil
	}

	if err := svc.ResendVerification(context.Background(), user.ID); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	if len(mailer.Sent()) != 0 {
		t.Errorf("throttled resend should not send mail; sent = %d", len(mailer.Sent()))
	}
	if len(tokenRepo.insertCalls) != 0 {
		t.Errorf("throttled resend should not insert token; inserts = %d", len(tokenRepo.insertCalls))
	}
}

func TestService_ResendVerification_CooldownElapsed_Issues(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, mailer := newServiceWithVerification(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", State: 1,
	})
	tokenRepo.mostRecentIssuedAtHook = func(int, actiontoken.Action) (time.Time, error) {
		return fixed.Add(-2 * time.Minute), nil
	}

	if err := svc.ResendVerification(context.Background(), user.ID); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	if len(mailer.Sent()) != 1 {
		t.Errorf("expected 1 mail sent after cooldown; got %d", len(mailer.Sent()))
	}
}

func TestService_ResendVerification_UserNotFound_Wraps(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newServiceWithVerification(t)

	err := svc.ResendVerification(context.Background(), 999)
	if err == nil || !strings.Contains(err.Error(), "app.Service.ResendVerification") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_RequestPasswordReset_InvalidEmail_ReturnsValidationError(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)

	err := fx.svc.RequestPasswordReset(context.Background(), "not-an-email")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["email"] == "" {
		t.Errorf("missing email error in %+v", ve.Fields)
	}
	if len(fx.mailer.Sent()) != 0 {
		t.Errorf("expected no mail on validation failure")
	}
}

func TestService_RequestPasswordReset_UnknownUser_SilentSuccess(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)

	if err := fx.svc.RequestPasswordReset(context.Background(), "ghost@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if len(fx.mailer.Sent()) != 0 {
		t.Errorf("unknown user must not receive mail; sent = %d", len(fx.mailer.Sent()))
	}
	if len(fx.tokenRepo.insertCalls) != 0 {
		t.Errorf("unknown user must not get a token; inserts = %d", len(fx.tokenRepo.insertCalls))
	}
}

func TestService_RequestPasswordReset_RecentlyChangedSilent(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com",
	})
	fx.changeLog.byKey[changeKey{user.ID, domain.ChangeTypePassword}] = fixed.Add(-time.Hour)

	if err := fx.svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if len(fx.mailer.Sent()) != 0 {
		t.Errorf("recent password change must suppress mail")
	}
}

func TestService_RequestPasswordReset_TokenCooldownSilent(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com",
	})
	fx.tokenRepo.mostRecentIssuedAtHook = func(int, actiontoken.Action) (time.Time, error) {
		return fixed.Add(-30 * time.Second), nil
	}

	if err := fx.svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if len(fx.mailer.Sent()) != 0 {
		t.Errorf("token cooldown must suppress mail")
	}
}

func TestService_RequestPasswordReset_Success(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com",
	})

	if err := fx.svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if len(fx.mailer.Sent()) != 1 {
		t.Fatalf("expected 1 mail sent, got %d", len(fx.mailer.Sent()))
	}
	if fx.mailer.Sent()[0].To != user.Email {
		t.Errorf("To = %q, want %q", fx.mailer.Sent()[0].To, user.Email)
	}
	if len(fx.tokenRepo.insertCalls) != 1 {
		t.Errorf("expected 1 token insert, got %d", len(fx.tokenRepo.insertCalls))
	}
	if fx.tokenRepo.insertCalls[0].Action != actiontoken.PasswordReset {
		t.Errorf("token Action = %v, want PasswordReset", fx.tokenRepo.insertCalls[0].Action)
	}
	body := fx.mailer.Sent()[0].Body
	if !strings.Contains(body, "https://cp.example/reset-password?token=") {
		t.Errorf("body missing reset URL: %s", body)
	}
}

func TestService_ConsumePasswordReset_InvalidPassword_ReturnsValidationError(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)

	err := fx.svc.ConsumePasswordReset(context.Background(), "anytoken", "weak")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["password"] == "" {
		t.Errorf("missing password error in %+v", ve.Fields)
	}
}

func TestService_ConsumePasswordReset_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", Password: "Old1234!", State: 1,
	})

	now := time.Now()
	var raw [32]byte
	raw[0] = 0xc1
	hash := sha256.Sum256(raw[:])
	fx.tokenRepo.byHash[hash] = &actiontoken.ActionToken{
		TokenHash: hash, AccountID: user.ID, Action: actiontoken.PasswordReset,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Minute),
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw[:])

	if err := fx.svc.ConsumePasswordReset(context.Background(), rawToken, "NewPass1!"); err != nil {
		t.Fatalf("ConsumePasswordReset: %v", err)
	}
	got, _ := fx.userRepo.GetByID(context.Background(), user.ID)
	if got.Password != "NewPass1!" {
		t.Errorf("password not updated; got %q", got.Password)
	}
	if got.State != 0 {
		t.Errorf("State after reset = %d, want 0 (verified as side effect)", got.State)
	}
	if at := fx.changeLog.byKey[changeKey{user.ID, domain.ChangeTypePassword}]; at.IsZero() {
		t.Errorf("password change not recorded; want a non-zero timestamp")
	}
	if len(fx.invalidator.invalidateAllCalls) != 1 || fx.invalidator.invalidateAllCalls[0] != user.ID {
		t.Errorf("InvalidateAll calls = %v, want [%d]", fx.invalidator.invalidateAllCalls, user.ID)
	}
}

func TestService_ConsumePasswordReset_TokenError_Wraps(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)

	err := fx.svc.ConsumePasswordReset(context.Background(), "***bad***", "NewPass1!")
	if err == nil || !strings.Contains(err.Error(), "app.Service.ConsumePasswordReset") {
		t.Errorf("not wrapped: %v", err)
	}
	if !errors.Is(err, actiontoken.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid in chain, got %v", err)
	}
}

func TestService_ConsumePasswordReset_InvalidatorError_Wraps(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", Password: "Old1234!",
	})
	var raw [32]byte
	raw[0] = 0xc2
	hash := sha256.Sum256(raw[:])
	fx.tokenRepo.byHash[hash] = &actiontoken.ActionToken{
		TokenHash: hash, AccountID: user.ID, Action: actiontoken.PasswordReset,
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}
	fx.invalidator.invalidateAllHook = func(int) error { return errors.New("invalidate boom") }

	err := fx.svc.ConsumePasswordReset(context.Background(),
		base64.RawURLEncoding.EncodeToString(raw[:]), "NewPass1!")
	if err == nil || !strings.Contains(err.Error(), "app.Service.ConsumePasswordReset") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_PeekPasswordReset_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)
	var raw [32]byte
	raw[0] = 0xc3
	hash := sha256.Sum256(raw[:])
	fx.tokenRepo.byHash[hash] = &actiontoken.ActionToken{
		TokenHash: hash, AccountID: 7, Action: actiontoken.PasswordReset,
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}

	token, err := fx.svc.PeekPasswordReset(context.Background(), base64.RawURLEncoding.EncodeToString(raw[:]))
	if err != nil {
		t.Fatalf("PeekPasswordReset: %v", err)
	}
	if token.AccountID != 7 {
		t.Errorf("AccountID = %d, want 7", token.AccountID)
	}
	if fx.tokenRepo.byHash[hash].ConsumedAt.Valid {
		t.Errorf("Peek must not consume token")
	}
}

func TestService_PeekPasswordReset_WrapsError(t *testing.T) {
	t.Parallel()
	fx := newServiceWithReset(t)

	_, err := fx.svc.PeekPasswordReset(context.Background(), "***bad***")
	if err == nil || !strings.Contains(err.Error(), "app.Service.PeekPasswordReset") {
		t.Errorf("not wrapped: %v", err)
	}
}
