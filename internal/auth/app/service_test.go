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

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

// fakeUserRepo implements domain.Repository in memory. Hooks override
// individual methods for error-path tests.
type fakeUserRepo struct {
	byID              map[int]*domain.User
	createHook        func(*domain.User) (*domain.User, error)
	getAllHook        func() ([]domain.User, error)
	getByIDHook       func(int) (*domain.User, error)
	getByUsernameHook func(string) (*domain.User, error)
	getByEmailHook    func(string) (*domain.User, error)
	updateHook        func(*domain.User) (*domain.User, error)
	deleteHook        func(int) error
	authenticateHook  func(string, string) (*domain.User, error)
	markVerifiedHook  func(int) error
	mu                sync.Mutex
	nextID            int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byID: map[int]*domain.User{}, nextID: 1}
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	if f.createHook != nil {
		return f.createHook(u)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *u
	cp.ID = f.nextID
	f.nextID++
	f.byID[cp.ID] = &cp
	return &cp, nil
}

func (f *fakeUserRepo) GetAll(_ context.Context) ([]domain.User, error) {
	if f.getAllHook != nil {
		return f.getAllHook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.User, 0, len(f.byID))
	for _, u := range f.byID {
		out = append(out, *u)
	}
	return out, nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id int) (*domain.User, error) {
	if f.getByIDHook != nil {
		return f.getByIDHook(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	if f.getByUsernameHook != nil {
		return f.getByUsernameHook(username)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if f.getByEmailHook != nil {
		return f.getByEmailHook(email)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeUserRepo) Update(_ context.Context, u *domain.User) (*domain.User, error) {
	if f.updateHook != nil {
		return f.updateHook(u)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[u.ID]; !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *u
	f.byID[u.ID] = &cp
	return &cp, nil
}

func (f *fakeUserRepo) Delete(_ context.Context, id int) error {
	if f.deleteHook != nil {
		return f.deleteHook(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeUserRepo) Authenticate(_ context.Context, username, password string) (*domain.User, error) {
	if f.authenticateHook != nil {
		return f.authenticateHook(username, password)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.Username == username && u.Password == password {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrInvalidCredentials
}

func (f *fakeUserRepo) MarkVerified(_ context.Context, accountID int) error {
	if f.markVerifiedHook != nil {
		return f.markVerifiedHook(accountID)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[accountID]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.GroupID = 0
	return nil
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	validCmd := CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.com",
		Gender:          "F",
	}

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		svc := NewService(repo)

		dto, err := svc.Create(context.Background(), validCmd)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if dto.Username != validCmd.Username || dto.Email != validCmd.Email || dto.ID == 0 {
			t.Errorf("DTO mismatch: %+v", dto)
		}
	})

	t.Run("field validation collects errors", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		svc := NewService(repo)

		bad := CreateCommand{
			Username:        "abc",
			Password:        "weak",
			PasswordConfirm: "different",
			Email:           "not-an-email",
			Gender:          "X",
		}
		_, err := svc.Create(context.Background(), bad)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		for _, key := range []string{"username", "password", "password_confirm", "email", "gender"} {
			if ve.Fields[key] == "" {
				t.Errorf("missing field error for %q in %+v", key, ve.Fields)
			}
		}
	})

	t.Run("password confirm mismatch alone", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())
		bad := validCmd
		bad.PasswordConfirm = "Different1!"
		_, err := svc.Create(context.Background(), bad)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Fields["password_confirm"] == "" {
			t.Errorf("missing password_confirm error")
		}
	})

	t.Run("username conflict", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		_, _ = repo.Create(context.Background(), &domain.User{
			Username: "testuser", Email: "other@example.com",
		})
		svc := NewService(repo)

		_, err := svc.Create(context.Background(), validCmd)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Fields["username"] == "" {
			t.Errorf("missing username conflict")
		}
	})

	t.Run("email conflict", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		_, _ = repo.Create(context.Background(), &domain.User{
			Username: "otheruser", Email: "test@example.com",
		})
		svc := NewService(repo)

		_, err := svc.Create(context.Background(), validCmd)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Fields["email"] == "" {
			t.Errorf("missing email conflict")
		}
	})

	t.Run("both conflicts aggregate", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		_, _ = repo.Create(context.Background(), &domain.User{
			Username: "testuser", Email: "other@example.com",
		})
		_, _ = repo.Create(context.Background(), &domain.User{
			Username: "otheruser", Email: "test@example.com",
		})
		svc := NewService(repo)

		_, err := svc.Create(context.Background(), validCmd)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected ValidationError, got %v", err)
		}
		if ve.Fields["username"] == "" {
			t.Errorf("missing username conflict in %+v", ve.Fields)
		}
		if ve.Fields["email"] == "" {
			t.Errorf("missing email conflict in %+v", ve.Fields)
		}
	})

	t.Run("concurrent Create with same username serializes", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		svc := NewService(repo)

		const N = 8
		var wg sync.WaitGroup
		results := make([]error, N)
		wg.Add(N)
		for i := range N {
			go func(idx int) {
				defer wg.Done()
				_, err := svc.Create(context.Background(), validCmd)
				results[idx] = err
			}(i)
		}
		wg.Wait()

		successes, conflicts := 0, 0
		for _, err := range results {
			if err == nil {
				successes++
				continue
			}
			var ve *domain.ValidationError
			if errors.As(err, &ve) && ve.Fields["username"] != "" {
				conflicts++
				continue
			}
			t.Errorf("unexpected error: %v", err)
		}
		if successes != 1 {
			t.Errorf("successes = %d, want 1", successes)
		}
		if conflicts != N-1 {
			t.Errorf("conflicts = %d, want %d", conflicts, N-1)
		}
	})

	t.Run("GetByUsername repo error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		repo.getByUsernameHook = func(string) (*domain.User, error) { return nil, errors.New("boom") }
		svc := NewService(repo)

		_, err := svc.Create(context.Background(), validCmd)
		if err == nil {
			t.Fatalf("expected error")
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			t.Errorf("expected wrapped infra error, got ValidationError: %v", err)
		}
		if !strings.Contains(err.Error(), "app.Service.Create") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("Repo.Create error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		repo.createHook = func(*domain.User) (*domain.User, error) { return nil, errors.New("boom") }
		svc := NewService(repo)

		_, err := svc.Create(context.Background(), validCmd)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Create") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_GetAll(t *testing.T) {
	t.Parallel()

	t.Run("non-empty", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		_, _ = repo.Create(context.Background(), &domain.User{Username: "a", Email: "a@x"})
		_, _ = repo.Create(context.Background(), &domain.User{Username: "b", Email: "b@x"})
		svc := NewService(repo)

		out, err := svc.GetAll(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 2 {
			t.Errorf("len = %d, want 2", len(out))
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())
		out, err := svc.GetAll(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 0 {
			t.Errorf("len = %d, want 0", len(out))
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		repo.getAllHook = func() ([]domain.User, error) { return nil, errors.New("boom") }
		svc := NewService(repo)

		_, err := svc.GetAll(context.Background())
		if err == nil || !strings.Contains(err.Error(), "app.Service.GetAll") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_GetByID(t *testing.T) {
	t.Parallel()

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		u, _ := repo.Create(context.Background(), &domain.User{Username: "a", Email: "a@x"})
		svc := NewService(repo)

		dto, err := svc.GetByID(context.Background(), u.ID)
		if err != nil {
			t.Fatal(err)
		}
		if dto.ID != u.ID || dto.Username != "a" {
			t.Errorf("dto mismatch: %+v", dto)
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())
		_, err := svc.GetByID(context.Background(), 999)
		if err == nil || !strings.Contains(err.Error(), "app.Service.GetByID") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_GetByEmail(t *testing.T) {
	t.Parallel()

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		u, _ := repo.Create(context.Background(), &domain.User{Username: "a", Email: "a@x"})
		svc := NewService(repo)

		dto, err := svc.GetByEmail(context.Background(), u.Email)
		if err != nil {
			t.Fatal(err)
		}
		if dto.Email != u.Email {
			t.Errorf("dto.Email = %q, want %q", dto.Email, u.Email)
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())
		_, err := svc.GetByEmail(context.Background(), "missing@x")
		if err == nil || !strings.Contains(err.Error(), "app.Service.GetByEmail") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		u, _ := repo.Create(context.Background(), &domain.User{Username: "a", Email: "old@x", Password: "old"})
		svc := NewService(repo)

		dto, err := svc.Update(context.Background(), u.ID, UpdateCommand{Email: "new@x", Password: "new"})
		if err != nil {
			t.Fatal(err)
		}
		if dto.Email != "new@x" {
			t.Errorf("dto.Email = %q, want new@x", dto.Email)
		}
	})

	t.Run("GetByID error wraps", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())
		_, err := svc.Update(context.Background(), 999, UpdateCommand{Email: "x", Password: "y"})
		if err == nil || !strings.Contains(err.Error(), "app.Service.Update") {
			t.Errorf("not wrapped: %v", err)
		}
	})

	t.Run("Update error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		u, _ := repo.Create(context.Background(), &domain.User{Username: "a", Email: "a@x"})
		repo.updateHook = func(*domain.User) (*domain.User, error) { return nil, errors.New("boom") }
		svc := NewService(repo)

		_, err := svc.Update(context.Background(), u.ID, UpdateCommand{Email: "new@x", Password: "newpw"})
		if err == nil || !strings.Contains(err.Error(), "app.Service.Update") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		u, _ := repo.Create(context.Background(), &domain.User{Username: "a", Email: "a@x"})
		svc := NewService(repo)

		if err := svc.Delete(context.Background(), u.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.GetByID(context.Background(), u.ID); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("not deleted")
		}
	})

	t.Run("repo error wraps", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())
		err := svc.Delete(context.Background(), 999)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Delete") {
			t.Errorf("not wrapped: %v", err)
		}
	})
}

func TestService_Authenticate(t *testing.T) {
	t.Parallel()

	validLogin := LoginCommand{Username: "testuser", Password: "Test1234!"}

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		_, _ = repo.Create(context.Background(), &domain.User{
			Username: validLogin.Username,
			Email:    "test@example.com",
			Password: validLogin.Password,
		})
		svc := NewService(repo)

		dto, err := svc.Authenticate(context.Background(), validLogin)
		if err != nil {
			t.Fatal(err)
		}
		if dto.Username != validLogin.Username {
			t.Errorf("dto.Username = %q, want %q", dto.Username, validLogin.Username)
		}
	})

	t.Run("invalid credentials passes through unwrapped", func(t *testing.T) {
		t.Parallel()
		svc := NewService(newFakeUserRepo())

		// Shape passes; repo lookup fails. The Authenticate error must reach
		// the caller as ErrInvalidCredentials without app-layer wrapping.
		_, err := svc.Authenticate(context.Background(), validLogin)
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("got %v, want ErrInvalidCredentials", err)
		}
		if strings.Contains(err.Error(), "app.Service.Authenticate") {
			t.Errorf("ErrInvalidCredentials should pass through unwrapped, got %q", err.Error())
		}
	})

	t.Run("generic error wraps", func(t *testing.T) {
		t.Parallel()
		repo := newFakeUserRepo()
		repo.authenticateHook = func(string, string) (*domain.User, error) {
			return nil, errors.New("boom")
		}
		svc := NewService(repo)

		_, err := svc.Authenticate(context.Background(), validLogin)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Authenticate") {
			t.Errorf("not wrapped: %v", err)
		}
	})

}

type fakeTokenRepo struct {
	byHash               map[[32]byte]*domain.ActionToken
	insertHook           func(*domain.ActionToken) error
	getByHashHook        func([32]byte) (*domain.ActionToken, error)
	deleteUnconsumedHook func(int, domain.Action) error
	markConsumedHook     func([32]byte, time.Time) error
	mostRecentHook       func(int, domain.Action) (time.Time, error)
	insertCalls          []domain.ActionToken
	deleteCalls          []struct {
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
	if f.mostRecentHook != nil {
		return f.mostRecentHook(accountID, action)
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

func newVerificationConfig() VerificationConfig {
	return VerificationConfig{
		AppURL:         "https://cp.example/",
		ServerName:     "Test rAthena",
		TokenTTL:       24 * time.Hour,
		ResendCooldown: 60 * time.Second,
	}
}

func newServiceWithVerification(t *testing.T) (*Service, *fakeUserRepo, *fakeTokenRepo, *fakeMailer) {
	t.Helper()
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeTokenRepo()
	mailer := &fakeMailer{}
	svc := NewService(userRepo, WithVerification(tokenRepo, mailer, newVerificationConfig()))
	return svc, userRepo, tokenRepo, mailer
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

func TestWithVerification_PanicsOnNonPositiveTTL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{name: "zero", ttl: 0},
		{name: "negative", ttl: -time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic for TokenTTL=%v", tt.ttl)
				}
			}()
			WithVerification(newFakeTokenRepo(), &fakeMailer{}, VerificationConfig{
				AppURL:     "https://cp.example",
				ServerName: "X",
				TokenTTL:   tt.ttl,
			})
		})
	}
}

func TestWithVerification_DefaultsResendCooldown(t *testing.T) {
	t.Parallel()
	cfg := VerificationConfig{
		AppURL:     "https://cp.example",
		ServerName: "X",
		TokenTTL:   time.Hour,
	}
	svc := NewService(newFakeUserRepo(), WithVerification(newFakeTokenRepo(), &fakeMailer{}, cfg))
	if svc.cfg.ResendCooldown != 60*time.Second {
		t.Errorf("ResendCooldown = %v, want 60s default", svc.cfg.ResendCooldown)
	}
}

func TestWithVerification_PreservesExplicitResendCooldown(t *testing.T) {
	t.Parallel()
	cfg := VerificationConfig{
		AppURL:         "https://cp.example",
		ServerName:     "X",
		TokenTTL:       time.Hour,
		ResendCooldown: 5 * time.Minute,
	}
	svc := NewService(newFakeUserRepo(), WithVerification(newFakeTokenRepo(), &fakeMailer{}, cfg))
	if svc.cfg.ResendCooldown != 5*time.Minute {
		t.Errorf("ResendCooldown = %v, want 5m (preserved)", svc.cfg.ResendCooldown)
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
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if dto.ID == 0 {
		t.Errorf("expected ID assigned")
	}
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
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(tokenRepo.insertCalls) != 1 {
		t.Errorf("token Insert calls = %d, want 1", len(tokenRepo.insertCalls))
	}
	if tokenRepo.insertCalls[0].AccountID != dto.ID {
		t.Errorf("token AccountID = %d, want %d", tokenRepo.insertCalls[0].AccountID, dto.ID)
	}
	if tokenRepo.insertCalls[0].Action != domain.ActionEmailVerification {
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

func TestService_Create_VerificationInsertError_Wraps(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	tokenRepo.insertHook = func(*domain.ActionToken) error { return errors.New("token insert boom") }

	_, err := svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.com",
		Gender:          "F",
	})
	if err == nil {
		t.Fatal("expected error from IssueVerification")
	}
	if !strings.Contains(err.Error(), "app.Service.Create") {
		t.Errorf("not wrapped with Create: %v", err)
	}
}

func TestService_IssueVerification_DeletesBeforeInsert(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, mailer := newServiceWithVerification(t)

	if err := svc.IssueVerification(context.Background(), 42, "user@example.com", "user"); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	if len(tokenRepo.deleteCalls) != 1 {
		t.Fatalf("DeleteUnconsumed calls = %d, want 1", len(tokenRepo.deleteCalls))
	}
	if tokenRepo.deleteCalls[0].AccountID != 42 {
		t.Errorf("deleted accountID = %d, want 42", tokenRepo.deleteCalls[0].AccountID)
	}
	if tokenRepo.deleteCalls[0].Action != domain.ActionEmailVerification {
		t.Errorf("deleted action = %v, want EmailVerification", tokenRepo.deleteCalls[0].Action)
	}
	if len(mailer.Sent()) != 1 {
		t.Errorf("expected 1 mail sent")
	}
}

func TestService_IssueVerification_TokenExpiresAtTTL(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	if err := svc.IssueVerification(context.Background(), 1, "user@example.com", "user"); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	stored := tokenRepo.insertCalls[0]
	wantExpiry := fixed.Add(24 * time.Hour)
	if !stored.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", stored.ExpiresAt, wantExpiry)
	}
	if !stored.CreatedAt.Equal(fixed) {
		t.Errorf("CreatedAt = %v, want %v", stored.CreatedAt, fixed)
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
	tokenRepo := newFakeTokenRepo()
	mailer := &fakeMailer{}
	cfg := VerificationConfig{
		AppURL:     "https://cp.example///",
		ServerName: "X",
		TokenTTL:   time.Hour,
	}
	svc := NewService(newFakeUserRepo(), WithVerification(tokenRepo, mailer, cfg))

	if err := svc.IssueVerification(context.Background(), 1, "u@x", "u"); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	body := mailer.Sent()[0].Body
	if strings.Contains(body, "////verify") || strings.Contains(body, "///verify") {
		t.Errorf("trailing slashes not trimmed: %s", body)
	}
	if !strings.Contains(body, "https://cp.example/verify?token=") {
		t.Errorf("body missing canonical URL: %s", body)
	}
}

func TestService_IssueVerification_DeleteError_Wraps(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	tokenRepo.deleteUnconsumedHook = func(int, domain.Action) error { return errors.New("delete boom") }

	err := svc.IssueVerification(context.Background(), 1, "u@x", "u")
	if err == nil || !strings.Contains(err.Error(), "app.Service.IssueVerification") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_ConsumeVerification_HappyPath(t *testing.T) {
	t.Parallel()
	svc, userRepo, _, mailer := newServiceWithVerification(t)
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", GroupID: 5,
	})

	if err := svc.IssueVerification(context.Background(), user.ID, user.Email, user.Username); err != nil {
		t.Fatalf("IssueVerification: %v", err)
	}
	rawToken := extractRawTokenFromBody(t, mailer.Sent()[0].Body)

	if err := svc.ConsumeVerification(context.Background(), rawToken); err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}
	got, _ := userRepo.GetByID(context.Background(), user.ID)
	if got.GroupID != 0 {
		t.Errorf("GroupID = %d, want 0 (verified)", got.GroupID)
	}
}

func TestService_ConsumeVerification_InvalidEncoding(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newServiceWithVerification(t)

	tests := []string{"", "***not-base64***", "short"}
	for _, raw := range tests {
		t.Run("input="+raw, func(t *testing.T) {
			t.Parallel()
			err := svc.ConsumeVerification(context.Background(), raw)
			if !errors.Is(err, domain.ErrTokenInvalid) {
				t.Errorf("got %v, want ErrTokenInvalid", err)
			}
		})
	}
}

func TestService_ConsumeVerification_UnknownTokenInRepo(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newServiceWithVerification(t)
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw)

	err := svc.ConsumeVerification(context.Background(), rawToken)
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestService_ConsumeVerification_WrongAction(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	var rawBytes [32]byte
	rawBytes[0] = 7
	hash := sha256.Sum256(rawBytes[:])
	tokenRepo.byHash[hash] = &domain.ActionToken{
		TokenHash: hash,
		AccountID: 1,
		Action:    domain.ActionUnknown,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	err := svc.ConsumeVerification(context.Background(), base64.RawURLEncoding.EncodeToString(rawBytes[:]))
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid for wrong action", err)
	}
}

func TestService_ConsumeVerification_AlreadyConsumed(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	var rawBytes [32]byte
	rawBytes[0] = 8
	hash := sha256.Sum256(rawBytes[:])
	now := time.Now()
	tokenRepo.byHash[hash] = &domain.ActionToken{
		TokenHash:  hash,
		AccountID:  1,
		Action:     domain.ActionEmailVerification,
		ExpiresAt:  now.Add(time.Hour),
		CreatedAt:  now,
		ConsumedAt: sql.NullTime{Time: now, Valid: true},
	}

	err := svc.ConsumeVerification(context.Background(), base64.RawURLEncoding.EncodeToString(rawBytes[:]))
	if !errors.Is(err, domain.ErrTokenAlreadyUsed) {
		t.Errorf("got %v, want ErrTokenAlreadyUsed", err)
	}
}

func TestService_ConsumeVerification_Expired(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }
	var rawBytes [32]byte
	rawBytes[0] = 9
	hash := sha256.Sum256(rawBytes[:])
	tokenRepo.byHash[hash] = &domain.ActionToken{
		TokenHash: hash,
		AccountID: 1,
		Action:    domain.ActionEmailVerification,
		ExpiresAt: fixed.Add(-time.Second),
		CreatedAt: fixed.Add(-time.Hour),
	}

	err := svc.ConsumeVerification(context.Background(), base64.RawURLEncoding.EncodeToString(rawBytes[:]))
	if !errors.Is(err, domain.ErrTokenExpired) {
		t.Errorf("got %v, want ErrTokenExpired", err)
	}
}

func TestService_ConsumeVerification_MarkConsumedError_Wraps(t *testing.T) {
	t.Parallel()
	svc, _, tokenRepo, _ := newServiceWithVerification(t)
	var rawBytes [32]byte
	rawBytes[0] = 10
	hash := sha256.Sum256(rawBytes[:])
	tokenRepo.byHash[hash] = &domain.ActionToken{
		TokenHash: hash, AccountID: 1, Action: domain.ActionEmailVerification,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	tokenRepo.markConsumedHook = func([32]byte, time.Time) error { return errors.New("mark boom") }

	err := svc.ConsumeVerification(context.Background(), base64.RawURLEncoding.EncodeToString(rawBytes[:]))
	if err == nil || !strings.Contains(err.Error(), "app.Service.ConsumeVerification") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_ConsumeVerification_MarkVerifiedError_Wraps(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, _ := newServiceWithVerification(t)
	var rawBytes [32]byte
	rawBytes[0] = 11
	hash := sha256.Sum256(rawBytes[:])
	tokenRepo.byHash[hash] = &domain.ActionToken{
		TokenHash: hash, AccountID: 99, Action: domain.ActionEmailVerification,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	userRepo.markVerifiedHook = func(int) error { return errors.New("mark verified boom") }

	err := svc.ConsumeVerification(context.Background(), base64.RawURLEncoding.EncodeToString(rawBytes[:]))
	if err == nil || !strings.Contains(err.Error(), "app.Service.ConsumeVerification") {
		t.Errorf("not wrapped: %v", err)
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

func TestService_ResendVerification_AlreadyVerifiedSilent(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, mailer := newServiceWithVerification(t)
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", GroupID: 0,
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
		Username: "u", Email: "u@x", GroupID: 5,
	})
	tokenRepo.mostRecentHook = func(int, domain.Action) (time.Time, error) {
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
		Username: "u", Email: "u@x", GroupID: 5,
	})
	tokenRepo.mostRecentHook = func(int, domain.Action) (time.Time, error) {
		return fixed.Add(-2 * time.Minute), nil
	}

	if err := svc.ResendVerification(context.Background(), user.ID); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	if len(mailer.Sent()) != 1 {
		t.Errorf("expected 1 mail sent after cooldown; got %d", len(mailer.Sent()))
	}
}

func TestService_ResendVerification_NoPriorToken_Issues(t *testing.T) {
	t.Parallel()
	svc, userRepo, _, mailer := newServiceWithVerification(t)
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", GroupID: 5,
	})

	if err := svc.ResendVerification(context.Background(), user.ID); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	if len(mailer.Sent()) != 1 {
		t.Errorf("expected 1 mail sent for first-time resend; got %d", len(mailer.Sent()))
	}
}

func TestService_ResendVerification_MostRecentError_Wraps(t *testing.T) {
	t.Parallel()
	svc, userRepo, tokenRepo, _ := newServiceWithVerification(t)
	user, _ := userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", GroupID: 5,
	})
	tokenRepo.mostRecentHook = func(int, domain.Action) (time.Time, error) {
		return time.Time{}, errors.New("most recent boom")
	}

	err := svc.ResendVerification(context.Background(), user.ID)
	if err == nil || !strings.Contains(err.Error(), "app.Service.ResendVerification") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestDecodeActionToken(t *testing.T) {
	t.Parallel()
	var raw [32]byte
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw[:])
	wantHash := sha256.Sum256(raw[:])

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantHas bool
	}{
		{name: "empty", input: "", wantOK: false},
		{name: "invalid base64", input: "***not-base64***", wantOK: false},
		{name: "wrong length", input: base64.RawURLEncoding.EncodeToString([]byte("short")), wantOK: false},
		{name: "valid 32-byte", input: encoded, wantOK: true, wantHas: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := decodeActionToken(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantHas && got != wantHash {
				t.Errorf("hash = %x, want %x", got, wantHash)
			}
		})
	}
}
