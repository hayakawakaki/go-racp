//go:build integration

package infra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/account/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func randomizeSuffix(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b[:])
}

func cleanupUser(t *testing.T, repo *Repository, id int) {
	t.Helper()
	t.Cleanup(func() { _ = repo.Delete(context.Background(), id) })
}

func TestRepository_CreateAndGet(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	wantBirthdate, err := time.Parse("2006-01-02", "1995-06-15")
	if err != nil {
		t.Fatalf("parse birthdate: %v", err)
	}
	u := &domain.User{
		Username:  "racp_test_" + suf,
		Email:     "racp_test_" + suf + "@example.invalid",
		Password:  "Test1234!",
		Gender:    "M",
		Birthdate: wantBirthdate,
	}
	created, err := repo.Create(ctx, u)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, created.ID)
	if created.ID == 0 {
		t.Errorf("ID not assigned")
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Username != u.Username || got.Email != u.Email {
		t.Errorf("mismatch: %+v", got)
	}
	if got.Birthdate.Format("2006-01-02") != "1995-06-15" {
		t.Errorf("Birthdate = %v; want 1995-06-15", got.Birthdate)
	}

	gotByU, err := repo.GetByUsername(ctx, u.Username)
	if err != nil || gotByU.ID != created.ID {
		t.Errorf("GetByUsername: got %+v, %v", gotByU, err)
	}

	gotByE, err := repo.GetByEmail(ctx, u.Email)
	if err != nil || gotByE.ID != created.ID {
		t.Errorf("GetByEmail: got %+v, %v", gotByE, err)
	}
}

func TestRepository_GetUnknown(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	if _, err := repo.GetByUsername(ctx, "missing_"+suf); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("GetByUsername: got %v, want ErrUserNotFound", err)
	}
	if _, err := repo.GetByEmail(ctx, "missing_"+suf+"@x"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("GetByEmail: got %v, want ErrUserNotFound", err)
	}
}

func TestRepository_Authenticate(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	birthdate, _ := time.Parse("2006-01-02", "1995-06-15")
	u := &domain.User{
		Username:  "racp_test_" + suf,
		Email:     "racp_test_" + suf + "@example.invalid",
		Password:  "secret",
		Gender:    "M",
		Birthdate: birthdate,
	}
	created, err := repo.Create(ctx, u)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, created.ID)

	t.Run("happy", func(t *testing.T) {
		got, err := repo.Authenticate(ctx, u.Username, "secret")
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("ID mismatch")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		_, err := repo.Authenticate(ctx, u.Username, "wrong")
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("got %v, want ErrInvalidCredentials", err)
		}
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := repo.Authenticate(ctx, "ghost_"+suf, "anything")
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("got %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestRepository_Create_SetsStateFiveAndPersistsIt(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	created, err := repo.Create(ctx, &domain.User{
		Username: "racp_test_" + suf,
		Email:    "racp_test_" + suf + "@example.invalid",
		Password: "Test1234!",
		Gender:   "M",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, created.ID)

	if created.State != 1 {
		t.Errorf("returned State = %d, want 1 (unverified)", created.State)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.State != 1 {
		t.Errorf("persisted State = %d, want 1", got.State)
	}
}

func TestRepository_MarkVerified(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("flips state from 1 to 0", func(t *testing.T) {
		suf := randomizeSuffix(t)
		user, err := repo.Create(ctx, &domain.User{
			Username: "racp_test_" + suf,
			Email:    "racp_test_" + suf + "@example.invalid",
			Password: "Test1234!",
			Gender:   "M",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		cleanupUser(t, repo, user.ID)

		if markErr := repo.MarkVerified(ctx, user.ID); markErr != nil {
			t.Fatalf("MarkVerified: %v", markErr)
		}
		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.State != 0 {
			t.Errorf("State after MarkVerified = %d, want 0", got.State)
		}
	})

	t.Run("idempotent on already-verified user", func(t *testing.T) {
		suf := randomizeSuffix(t)
		user, err := repo.Create(ctx, &domain.User{
			Username: "racp_test_" + suf,
			Email:    "racp_test_" + suf + "@example.invalid",
			Password: "Test1234!",
			Gender:   "M",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		cleanupUser(t, repo, user.ID)

		if firstErr := repo.MarkVerified(ctx, user.ID); firstErr != nil {
			t.Fatalf("first MarkVerified: %v", firstErr)
		}
		if err := repo.MarkVerified(ctx, user.ID); err != nil {
			t.Errorf("second MarkVerified on already-verified user: got %v, want nil (idempotent)", err)
		}
	})

	t.Run("unknown account returns ErrUserNotFound", func(t *testing.T) {
		err := repo.MarkVerified(ctx, -1)
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
}

func TestRepository_UpdatePassword(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	u, err := repo.Create(ctx, &domain.User{
		Username: "racp_test_" + suf,
		Email:    "racp_test_" + suf + "@example.invalid",
		Password: "Old1234!",
		Gender:   "M",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, u.ID)

	t.Run("happy", func(t *testing.T) {
		if err := repo.UpdatePassword(ctx, u.ID, "New1234!"); err != nil {
			t.Fatalf("UpdatePassword: %v", err)
		}
		got, authErr := repo.Authenticate(ctx, u.Username, "New1234!")
		if authErr != nil {
			t.Fatalf("Authenticate with new password: %v", authErr)
		}
		if got.ID != u.ID {
			t.Errorf("ID = %d, want %d", got.ID, u.ID)
		}
		if _, oldErr := repo.Authenticate(ctx, u.Username, "Old1234!"); !errors.Is(oldErr, domain.ErrInvalidCredentials) {
			t.Errorf("old password should fail: got %v", oldErr)
		}
	})

	t.Run("missing user returns ErrUserNotFound", func(t *testing.T) {
		if err := repo.UpdatePassword(ctx, -1, "Whatever1!"); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
}

func TestRepository_UpdateEmail(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	u, err := repo.Create(ctx, &domain.User{
		Username: "racp_test_" + suf,
		Email:    "old_" + suf + "@example.invalid",
		Password: "Old1234!",
		Gender:   "M",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, u.ID)

	t.Run("happy", func(t *testing.T) {
		newEmail := "new_" + suf + "@example.invalid"
		if err := repo.UpdateEmail(ctx, u.ID, newEmail); err != nil {
			t.Fatalf("UpdateEmail: %v", err)
		}
		got, getErr := repo.GetByEmail(ctx, newEmail)
		if getErr != nil {
			t.Fatalf("GetByEmail: %v", getErr)
		}
		if got.ID != u.ID {
			t.Errorf("ID = %d, want %d", got.ID, u.ID)
		}
	})

	t.Run("missing user returns ErrUserNotFound", func(t *testing.T) {
		if err := repo.UpdateEmail(ctx, -1, "ghost@example.invalid"); !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
}

func TestRepository_Delete(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	u, err := repo.Create(ctx, &domain.User{
		Username: "racp_test_" + suf,
		Email:    "racp_test_" + suf + "@x",
		Password: "Test1234!",
		Gender:   "M",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, u.ID); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("second Delete: got %v, want ErrUserNotFound", err)
	}
}
