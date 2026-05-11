package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

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
