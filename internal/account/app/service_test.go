package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/accountchange"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
	authdomain "github.com/hayakawakaki/go-racp/internal/auth/domain"
)

type fakeUserRepo struct {
	byID               map[int]*authdomain.User
	createHook         func(*authdomain.User) (*authdomain.User, error)
	getByIDHook        func(int) (*authdomain.User, error)
	getByEmailHook     func(string) (*authdomain.User, error)
	updatePasswordHook func(int, string) error
	updateEmailHook    func(int, string) error
	mu                 sync.Mutex
	nextID             int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byID: map[int]*authdomain.User{}, nextID: 1}
}

func (f *fakeUserRepo) Create(_ context.Context, u *authdomain.User) (*authdomain.User, error) {
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

func (f *fakeUserRepo) GetAll(_ context.Context) ([]authdomain.User, error) { return nil, nil }

func (f *fakeUserRepo) GetByID(_ context.Context, id int) (*authdomain.User, error) {
	if f.getByIDHook != nil {
		return f.getByIDHook(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, authdomain.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUserRepo) GetByUsername(_ context.Context, username string) (*authdomain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byID {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, authdomain.ErrUserNotFound
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*authdomain.User, error) {
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
	return nil, authdomain.ErrUserNotFound
}

func (f *fakeUserRepo) Delete(_ context.Context, id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return authdomain.ErrUserNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeUserRepo) Authenticate(_ context.Context, _, _ string) (*authdomain.User, error) {
	return nil, authdomain.ErrInvalidCredentials
}

func (f *fakeUserRepo) MarkVerified(_ context.Context, id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return authdomain.ErrUserNotFound
	}
	u.GroupID = 0
	return nil
}

func (f *fakeUserRepo) UpdatePassword(_ context.Context, id int, newPassword string) error {
	if f.updatePasswordHook != nil {
		return f.updatePasswordHook(id, newPassword)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return authdomain.ErrUserNotFound
	}
	u.Password = newPassword
	return nil
}

func (f *fakeUserRepo) UpdateEmail(_ context.Context, id int, newEmail string) error {
	if f.updateEmailHook != nil {
		return f.updateEmailHook(id, newEmail)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return authdomain.ErrUserNotFound
	}
	u.Email = newEmail
	return nil
}

type fakeSessionInvalidator struct {
	hook  func(int, string) error
	calls []invalidateExceptCall
	mu    sync.Mutex
}

type invalidateExceptCall struct {
	RawToken string
	UserID   int
}

func (f *fakeSessionInvalidator) InvalidateAllForUserExceptCurrent(_ context.Context, userID int, rawToken string) error {
	f.mu.Lock()
	f.calls = append(f.calls, invalidateExceptCall{RawToken: rawToken, UserID: userID})
	f.mu.Unlock()
	if f.hook != nil {
		return f.hook(userID, rawToken)
	}
	return nil
}

type fakeChangeLog struct {
	byKey          map[changeKey]time.Time
	recordHook     func(int, accountchange.Type, time.Time) error
	mostRecentHook func(int, accountchange.Type) (time.Time, error)
	mu             sync.Mutex
}

type changeKey struct {
	AccountID  int
	ChangeType accountchange.Type
}

func newFakeChangeLog() *fakeChangeLog {
	return &fakeChangeLog{byKey: map[changeKey]time.Time{}}
}

func (f *fakeChangeLog) Record(_ context.Context, accountID int, changeType accountchange.Type, at time.Time) error {
	if f.recordHook != nil {
		return f.recordHook(accountID, changeType, at)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byKey[changeKey{accountID, changeType}] = at
	return nil
}

func (f *fakeChangeLog) MostRecent(_ context.Context, accountID int, changeType accountchange.Type) (time.Time, error) {
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

type fakeActionTokenRepo struct {
	byHash                 map[[32]byte]*actiontoken.ActionToken
	mostRecentIssuedAtHook func(int, actiontoken.Action) (time.Time, error)
	insertCalls            []actiontoken.ActionToken
	mu                     sync.Mutex
}

func newFakeActionTokenRepo() *fakeActionTokenRepo {
	return &fakeActionTokenRepo{byHash: map[[32]byte]*actiontoken.ActionToken{}}
}

func (f *fakeActionTokenRepo) Insert(_ context.Context, token *actiontoken.ActionToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *token
	f.byHash[token.TokenHash] = &cp
	f.insertCalls = append(f.insertCalls, cp)
	return nil
}

func (f *fakeActionTokenRepo) GetByHash(_ context.Context, hash [32]byte) (*actiontoken.ActionToken, error) {
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
	defer f.mu.Unlock()
	for hash, token := range f.byHash {
		if token.AccountID == accountID && token.Action == action && !token.ConsumedAt.Valid {
			delete(f.byHash, hash)
		}
	}
	return nil
}

func (f *fakeActionTokenRepo) MarkConsumed(_ context.Context, hash [32]byte, at time.Time) error {
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

func newConfig() Config {
	return Config{
		AppURL:                 "https://cp.example/",
		ServerName:             "Test rAthena",
		EmailChangeTokenTTL:    30 * time.Minute,
		EmailChangeRequestCool: 60 * time.Second,
		EmailChangeCool:        24 * time.Hour,
		PasswordChangeCool:     time.Hour,
	}
}

type serviceFixture struct {
	svc         *Service
	userRepo    *fakeUserRepo
	tokenRepo   *fakeActionTokenRepo
	changeLog   *fakeChangeLog
	mailer      *fakeMailer
	invalidator *fakeSessionInvalidator
}

func newFixture(t *testing.T) *serviceFixture {
	t.Helper()
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeActionTokenRepo()
	changeLog := newFakeChangeLog()
	mailer := &fakeMailer{}
	invalidator := &fakeSessionInvalidator{}
	manager := actiontoken.NewManager(tokenRepo)
	mu := &sync.Mutex{}
	svc := NewService(userRepo, invalidator, manager, changeLog, mailer, mu, newConfig())
	return &serviceFixture{
		svc:         svc,
		userRepo:    userRepo,
		tokenRepo:   tokenRepo,
		changeLog:   changeLog,
		mailer:      mailer,
		invalidator: invalidator,
	}
}

func TestNewService_PanicsOnMissingDeps(t *testing.T) {
	t.Parallel()
	manager := actiontoken.NewManager(newFakeActionTokenRepo())
	mu := &sync.Mutex{}

	tests := []struct {
		do   func()
		name string
	}{
		{name: "nil repo", do: func() {
			NewService(nil, &fakeSessionInvalidator{}, manager, newFakeChangeLog(), &fakeMailer{}, mu, newConfig())
		}},
		{name: "nil session invalidator", do: func() {
			NewService(newFakeUserRepo(), nil, manager, newFakeChangeLog(), &fakeMailer{}, mu, newConfig())
		}},
		{name: "nil token manager", do: func() {
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, nil, newFakeChangeLog(), &fakeMailer{}, mu, newConfig())
		}},
		{name: "nil change log", do: func() {
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, nil, &fakeMailer{}, mu, newConfig())
		}},
		{name: "nil mailer", do: func() {
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, newFakeChangeLog(), nil, mu, newConfig())
		}},
		{name: "nil mutex", do: func() {
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, newFakeChangeLog(), &fakeMailer{}, nil, newConfig())
		}},
		{name: "zero EmailChangeTokenTTL", do: func() {
			cfg := newConfig()
			cfg.EmailChangeTokenTTL = 0
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, newFakeChangeLog(), &fakeMailer{}, mu, cfg)
		}},
		{name: "zero EmailChangeRequestCool", do: func() {
			cfg := newConfig()
			cfg.EmailChangeRequestCool = 0
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, newFakeChangeLog(), &fakeMailer{}, mu, cfg)
		}},
		{name: "zero EmailChangeCool", do: func() {
			cfg := newConfig()
			cfg.EmailChangeCool = 0
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, newFakeChangeLog(), &fakeMailer{}, mu, cfg)
		}},
		{name: "zero PasswordChangeCool", do: func() {
			cfg := newConfig()
			cfg.PasswordChangeCool = 0
			NewService(newFakeUserRepo(), &fakeSessionInvalidator{}, manager, newFakeChangeLog(), &fakeMailer{}, mu, cfg)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic")
				}
			}()
			tt.do()
		})
	}
}

func TestService_GetAccount_Happy(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", GroupID: 0,
	})

	got, err := fx.svc.GetAccount(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.Username != "u" || got.Email != "u@example.com" || !got.Verified {
		t.Errorf("got = %+v, want verified user u/u@example.com", got)
	}
}

func TestService_GetAccount_UnverifiedFlag(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", GroupID: 5,
	})

	got, err := fx.svc.GetAccount(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.Verified {
		t.Errorf("Verified = true, want false for group_id=5")
	}
}

func TestService_GetAccount_RepoError_Wraps(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)

	_, err := fx.svc.GetAccount(context.Background(), 999)
	if err == nil || !strings.Contains(err.Error(), "app.Service.GetAccount") {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestService_UpdatePassword_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		newPassword string
		confirm     string
		wantField   string
	}{
		{name: "empty new password", newPassword: "", confirm: "", wantField: "new_password"},
		{name: "weak password", newPassword: "weak", confirm: "weak", wantField: "new_password"},
		{name: "mismatch", newPassword: "NewPass1!", confirm: "OtherPass1!", wantField: "new_password_confirm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newFixture(t)
			user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
				Username: "u", Email: "u@x", Password: "Curr1234!",
			})

			err := fx.svc.UpdatePassword(context.Background(), user.ID, "anytoken", "Curr1234!", tt.newPassword, tt.confirm)
			var ve *authdomain.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if ve.Fields[tt.wantField] == "" {
				t.Errorf("missing %q error in %+v", tt.wantField, ve.Fields)
			}
		})
	}
}

func TestService_UpdatePassword_WrongCurrentPassword(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.UpdatePassword(context.Background(), user.ID, "anytoken", "WrongPass!", "NewPass1!", "NewPass1!")
	var ve *authdomain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["current_password"] == "" {
		t.Errorf("missing current_password error in %+v", ve.Fields)
	}
}

func TestService_UpdatePassword_RecentlyChanged_Blocked(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})
	fx.changeLog.byKey[changeKey{user.ID, accountchange.Password}] = fixed.Add(-time.Minute)

	err := fx.svc.UpdatePassword(context.Background(), user.ID, "anytoken", "Curr1234!", "NewPass1!", "NewPass1!")
	if !errors.Is(err, authdomain.ErrPasswordRecentlyChanged) {
		t.Errorf("got %v, want ErrPasswordRecentlyChanged", err)
	}
}

func TestService_UpdatePassword_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.UpdatePassword(context.Background(), user.ID, "current-token", "Curr1234!", "NewPass1!", "NewPass1!")
	if err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, _ := fx.userRepo.GetByID(context.Background(), user.ID)
	if got.Password != "NewPass1!" {
		t.Errorf("password not updated; got %q", got.Password)
	}
	if at := fx.changeLog.byKey[changeKey{user.ID, accountchange.Password}]; at.IsZero() {
		t.Errorf("change log not recorded")
	}
	if len(fx.invalidator.calls) != 1 || fx.invalidator.calls[0].RawToken != "current-token" {
		t.Errorf("invalidator calls = %+v, want one call with current-token", fx.invalidator.calls)
	}
}

func TestService_RequestEmailChange_InvalidEmail(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "not-an-email")
	var ve *authdomain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["new_email"] == "" {
		t.Errorf("missing new_email error in %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_WrongCurrentPassword(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Wrong!", "new@example.com")
	var ve *authdomain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["current_password"] == "" {
		t.Errorf("missing current_password error in %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_SameEmail(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "U@EXAMPLE.COM")
	var ve *authdomain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Fields["new_email"], "same") {
		t.Errorf("expected 'same as current' error; got %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_AlreadyTaken(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	owner, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "owner", Email: "taken@example.com",
	})
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})
	_ = owner

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "taken@example.com")
	var ve *authdomain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Fields["new_email"], "already in use") {
		t.Errorf("expected 'already in use' error; got %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_RecentEmailChange_Blocked(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})
	fx.changeLog.byKey[changeKey{user.ID, accountchange.Email}] = fixed.Add(-time.Hour)

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "new@example.com")
	if !errors.Is(err, authdomain.ErrEmailRecentlyChanged) {
		t.Errorf("got %v, want ErrEmailRecentlyChanged", err)
	}
}

func TestService_RequestEmailChange_TokenCooldown_Blocked(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})
	fx.tokenRepo.mostRecentIssuedAtHook = func(int, actiontoken.Action) (time.Time, error) {
		return fixed.Add(-30 * time.Second), nil
	}

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "new@example.com")
	if !errors.Is(err, ErrEmailChangeCooldown) {
		t.Errorf("got %v, want ErrEmailChangeCooldown", err)
	}
}

func TestService_RequestEmailChange_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "new@example.com")
	if err != nil {
		t.Fatalf("RequestEmailChange: %v", err)
	}
	if len(fx.mailer.Sent()) != 1 {
		t.Fatalf("expected 1 mail sent, got %d", len(fx.mailer.Sent()))
	}
	sent := fx.mailer.Sent()[0]
	if sent.To != "new@example.com" {
		t.Errorf("mail To = %q, want new@example.com (new address, not current)", sent.To)
	}
	if len(fx.tokenRepo.insertCalls) != 1 {
		t.Fatalf("expected 1 token insert, got %d", len(fx.tokenRepo.insertCalls))
	}
	stored := fx.tokenRepo.insertCalls[0]
	if stored.Action != actiontoken.EmailChange {
		t.Errorf("Action = %v, want EmailChange", stored.Action)
	}
	if string(stored.Payload) != "new@example.com" {
		t.Errorf("Payload = %q, want new@example.com", string(stored.Payload))
	}
}

func TestService_ConsumeEmailChange_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "old@example.com",
	})

	rawToken, err := fx.svc.TokenManager.Issue(context.Background(), actiontoken.EmailChange, user.ID, []byte("new@example.com"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	got, err := fx.svc.ConsumeEmailChange(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("ConsumeEmailChange: %v", err)
	}
	if got.Email != "new@example.com" {
		t.Errorf("returned email = %q, want new@example.com", got.Email)
	}
	persisted, _ := fx.userRepo.GetByID(context.Background(), user.ID)
	if persisted.Email != "new@example.com" {
		t.Errorf("persisted email = %q, want new@example.com", persisted.Email)
	}
	if at := fx.changeLog.byKey[changeKey{user.ID, accountchange.Email}]; at.IsZero() {
		t.Errorf("email change not recorded")
	}
}

func TestService_ConsumeEmailChange_InvalidToken(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)

	_, err := fx.svc.ConsumeEmailChange(context.Background(), "***bad***")
	if !errors.Is(err, actiontoken.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestService_ConsumeEmailChange_TakenByAnother_ReturnsErrEmailTaken(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "old@example.com",
	})
	_, _ = fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "other", Email: "new@example.com",
	})

	rawToken, err := fx.svc.TokenManager.Issue(context.Background(), actiontoken.EmailChange, user.ID, []byte("new@example.com"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fx.svc.ConsumeEmailChange(context.Background(), rawToken)
	if !errors.Is(err, authdomain.ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

func TestService_ConsumeEmailChange_PayloadFailsValidation(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &authdomain.User{
		Username: "u", Email: "old@example.com",
	})

	rawToken, err := fx.svc.TokenManager.Issue(context.Background(), actiontoken.EmailChange, user.ID, []byte("not-an-email"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fx.svc.ConsumeEmailChange(context.Background(), rawToken)
	if !errors.Is(err, actiontoken.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid for corrupt payload", err)
	}
}
