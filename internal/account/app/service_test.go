package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/actiontoken"
)

// fakeUserRepo implements domain.Repository in memory. Hooks override
// individual methods for error-path tests.
type fakeUserRepo struct {
	byID               map[int]*domain.User
	createHook         func(*domain.User) (*domain.User, error)
	getAllHook         func() ([]domain.User, error)
	getByIDHook        func(int) (*domain.User, error)
	getByUsernameHook  func(string) (*domain.User, error)
	getByEmailHook     func(string) (*domain.User, error)
	deleteHook         func(int) error
	authenticateHook   func(string, string) (*domain.User, error)
	markVerifiedHook   func(int) error
	updatePasswordHook func(int, string) error
	updateEmailHook    func(int, string) error
	mu                 sync.Mutex
	nextID             int
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
	u.State = 0
	return nil
}

func (f *fakeUserRepo) UpdatePassword(_ context.Context, accountID int, newPassword string) error {
	if f.updatePasswordHook != nil {
		return f.updatePasswordHook(accountID, newPassword)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[accountID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Password = newPassword
	return nil
}

func (f *fakeUserRepo) UpdateEmail(_ context.Context, accountID int, newEmail string) error {
	if f.updateEmailHook != nil {
		return f.updateEmailHook(accountID, newEmail)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[accountID]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.Email = newEmail
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
		Birthdate:       "2000-01-01",
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

func TestService_Create_StoresBirthdate(t *testing.T) {
	t.Parallel()
	repo := newFakeUserRepo()
	svc := NewService(repo)
	svc.now = func() time.Time { return time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC) }

	dto, err := svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.invalid",
		Gender:          "M",
		Birthdate:       "2000-01-15",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	stored, err := repo.GetByID(context.Background(), dto.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	wantDate, _ := time.Parse("2006-01-02", "2000-01-15")
	if !stored.Birthdate.Equal(wantDate) {
		t.Errorf("Birthdate = %v; want %v", stored.Birthdate, wantDate)
	}
}

func TestService_Create_RejectsInvalidBirthdate(t *testing.T) {
	t.Parallel()
	repo := newFakeUserRepo()
	svc := NewService(repo)
	svc.now = func() time.Time { return time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC) }

	_, err := svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.invalid",
		Gender:          "M",
		Birthdate:       "",
	})
	var validationErr *domain.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Create err = %v; want *domain.ValidationError", err)
	}
	if validationErr.Fields["birthdate"] == "" {
		t.Errorf("Fields[\"birthdate\"] not populated: %+v", validationErr.Fields)
	}
}

func TestService_Create_BirthdateRespectsLocation(t *testing.T) {
	t.Parallel()
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("LoadLocation Asia/Tokyo: %v (likely missing tzdata)", err)
	}
	repo := newFakeUserRepo()
	svc := NewService(repo, WithLocation(tokyo))
	svc.now = func() time.Time { return time.Date(2026, 5, 12, 16, 0, 0, 0, time.UTC) }

	_, err = svc.Create(context.Background(), CreateCommand{
		Username:        "testuser",
		Password:        "Test1234!",
		PasswordConfirm: "Test1234!",
		Email:           "test@example.invalid",
		Gender:          "M",
		Birthdate:       "2026-05-13",
	})
	if err != nil {
		t.Fatalf("Create: %v; expected ok because 2026-05-13 is today in Tokyo when UTC is 2026-05-12 16:00", err)
	}
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

		dto, _, err := svc.Authenticate(context.Background(), validLogin)
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

		// Shape passes, repo lookup fails. The Authenticate error must reach
		// the caller as ErrInvalidCredentials without app-layer wrapping.
		_, _, err := svc.Authenticate(context.Background(), validLogin)
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

		_, _, err := svc.Authenticate(context.Background(), validLogin)
		if err == nil || !strings.Contains(err.Error(), "app.Service.Authenticate") {
			t.Errorf("not wrapped: %v", err)
		}

	})

}

func newEmailChangeConfig() EmailChangeConfig {
	return EmailChangeConfig{
		AppURL:           "https://cp.example/",
		ServerName:       "Test rAthena",
		TokenTTL:         30 * time.Minute,
		RequestCooldown:  60 * time.Second,
		ChangeCooldown:   24 * time.Hour,
		PasswordCooldown: time.Hour,
	}
}

type emailChangeFixture struct {
	svc         *Service
	userRepo    *fakeUserRepo
	tokenRepo   *fakeActionTokenRepo
	changeLog   *fakeChangeLog
	mailer      *fakeMailer
	invalidator *fakeSessionInvalidator
}

func newEmailChangeFixture(t *testing.T) *emailChangeFixture {
	t.Helper()
	userRepo := newFakeUserRepo()
	tokenRepo := newFakeActionTokenRepo()
	changeLog := newFakeChangeLog()
	mailer := &fakeMailer{}
	invalidator := &fakeSessionInvalidator{}
	manager := actiontoken.NewManager(tokenRepo)
	svc := NewService(userRepo,
		WithEmailChange(manager, mailer, newEmailChangeConfig()),
		WithChangeLog(changeLog),
		WithSessionInvalidator(invalidator),
	)
	return &emailChangeFixture{
		svc:         svc,
		userRepo:    userRepo,
		tokenRepo:   tokenRepo,
		changeLog:   changeLog,
		mailer:      mailer,
		invalidator: invalidator,
	}
}

func TestService_GetAccount_Happy(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", State: 0,
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
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", State: 1,
	})

	got, err := fx.svc.GetAccount(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.Verified {
		t.Errorf("Verified = true, want false for state=1")
	}
}

func TestService_GetAccount_RepoError_Wraps(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)

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
			fx := newEmailChangeFixture(t)
			user, _ := fx.userRepo.Create(context.Background(), &domain.User{
				Username: "u", Email: "u@x", Password: "Curr1234!",
			})

			err := fx.svc.UpdatePassword(context.Background(), user.ID, "anytoken", "Curr1234!", tt.newPassword, tt.confirm)
			var ve *domain.ValidationError
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
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.UpdatePassword(context.Background(), user.ID, "anytoken", "WrongPass!", "NewPass1!", "NewPass1!")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["current_password"] == "" {
		t.Errorf("missing current_password error in %+v", ve.Fields)
	}
}

func TestService_UpdatePassword_RecentlyChanged_Blocked(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})
	fx.changeLog.byKey[changeKey{user.ID, domain.ChangeTypePassword}] = fixed.Add(-time.Minute)

	err := fx.svc.UpdatePassword(context.Background(), user.ID, "anytoken", "Curr1234!", "NewPass1!", "NewPass1!")
	if !errors.Is(err, domain.ErrPasswordRecentlyChanged) {
		t.Errorf("got %v, want ErrPasswordRecentlyChanged", err)
	}
}

func TestService_UpdatePassword_HappyPath(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
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
	if at := fx.changeLog.byKey[changeKey{user.ID, domain.ChangeTypePassword}]; at.IsZero() {
		t.Errorf("change log not recorded")
	}
	if len(fx.invalidator.invalidateExceptCalls) != 1 || fx.invalidator.invalidateExceptCalls[0].CurrentRawToken != "current-token" {
		t.Errorf("invalidator calls = %+v, want one call with current-token", fx.invalidator.invalidateExceptCalls)
	}
}

func TestService_RequestEmailChange_InvalidEmail(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "not-an-email")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["new_email"] == "" {
		t.Errorf("missing new_email error in %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_WrongCurrentPassword(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@x", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Wrong!", "new@example.com")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if ve.Fields["current_password"] == "" {
		t.Errorf("missing current_password error in %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_SameEmail(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "U@EXAMPLE.COM")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Fields["new_email"], "same") {
		t.Errorf("expected 'same as current' error; got %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_AlreadyTaken(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	owner, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "owner", Email: "taken@example.com",
	})
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})
	_ = owner

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "taken@example.com")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if !strings.Contains(ve.Fields["new_email"], "already in use") {
		t.Errorf("expected 'already in use' error; got %+v", ve.Fields)
	}
}

func TestService_RequestEmailChange_RecentEmailChange_Blocked(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "u@example.com", Password: "Curr1234!",
	})
	fx.changeLog.byKey[changeKey{user.ID, domain.ChangeTypeEmail}] = fixed.Add(-time.Hour)

	err := fx.svc.RequestEmailChange(context.Background(), user.ID, "Curr1234!", "new@example.com")
	if !errors.Is(err, domain.ErrEmailRecentlyChanged) {
		t.Errorf("got %v, want ErrEmailRecentlyChanged", err)
	}
}

func TestService_RequestEmailChange_TokenCooldown_Blocked(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	fixed := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	fx.svc.now = func() time.Time { return fixed }
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
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
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
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
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
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
	if at := fx.changeLog.byKey[changeKey{user.ID, domain.ChangeTypeEmail}]; at.IsZero() {
		t.Errorf("email change not recorded")
	}
}

func TestService_ConsumeEmailChange_InvalidToken(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)

	_, err := fx.svc.ConsumeEmailChange(context.Background(), "***bad***")
	if !errors.Is(err, actiontoken.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestService_ConsumeEmailChange_TakenByAnother_ReturnsErrEmailTaken(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
		Username: "u", Email: "old@example.com",
	})
	_, _ = fx.userRepo.Create(context.Background(), &domain.User{
		Username: "other", Email: "new@example.com",
	})

	rawToken, err := fx.svc.TokenManager.Issue(context.Background(), actiontoken.EmailChange, user.ID, []byte("new@example.com"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fx.svc.ConsumeEmailChange(context.Background(), rawToken)
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("got %v, want ErrEmailTaken", err)
	}
}

func TestService_ConsumeEmailChange_PayloadFailsValidation(t *testing.T) {
	t.Parallel()
	fx := newEmailChangeFixture(t)
	user, _ := fx.userRepo.Create(context.Background(), &domain.User{
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
