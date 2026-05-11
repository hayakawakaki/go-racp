//go:build integration

package infra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
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
	u := &domain.User{
		Username: "racp_test_" + suf,
		Email:    "racp_test_" + suf + "@example.invalid",
		Password: "Test1234!",
		Gender:   "M",
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
	u := &domain.User{
		Username: "racp_test_" + suf,
		Email:    "racp_test_" + suf + "@example.invalid",
		Password: "secret",
		Gender:   "M",
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

func TestRepository_Update(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	u, err := repo.Create(ctx, &domain.User{
		Username: "racp_test_" + suf,
		Email:    "old_" + suf + "@x",
		Password: "old",
		Gender:   "M",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, u.ID)

	t.Run("happy", func(t *testing.T) {
		updated, err := repo.Update(ctx, &domain.User{
			ID:       u.ID,
			Email:    "new_" + suf + "@x",
			Password: "new",
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Email != "new_"+suf+"@x" {
			t.Errorf("email not updated")
		}
		got, _ := repo.Authenticate(ctx, u.Username, "new")
		if got == nil || got.ID != u.ID {
			t.Errorf("password not updated")
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := repo.Update(ctx, &domain.User{ID: 0, Email: "x", Password: "y"})
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("got %v, want ErrUserNotFound", err)
		}
	})
}

func TestRepository_Create_SetsGroupIDFiveAndPersistsIt(t *testing.T) {
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

	if created.GroupID != 5 {
		t.Errorf("returned GroupID = %d, want 5 (unverified)", created.GroupID)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.GroupID != 5 {
		t.Errorf("persisted GroupID = %d, want 5", got.GroupID)
	}
}

func TestRepository_MarkVerified(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("flips group_id from 5 to 0", func(t *testing.T) {
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
		if got.GroupID != 0 {
			t.Errorf("GroupID after MarkVerified = %d, want 0", got.GroupID)
		}
	})

	t.Run("already verified user returns ErrUserNotFound", func(t *testing.T) {
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
		err = repo.MarkVerified(ctx, user.ID)
		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Errorf("second MarkVerified on already-verified user: got %v, want ErrUserNotFound", err)
		}
	})

	t.Run("unknown account returns ErrUserNotFound", func(t *testing.T) {
		err := repo.MarkVerified(ctx, -1)
		if !errors.Is(err, domain.ErrUserNotFound) {
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
