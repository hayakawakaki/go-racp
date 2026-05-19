//go:build integration

package infra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
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
	wantBirthdate, err := time.Parse("2006-01-02", "2008-06-05")
	if err != nil {
		t.Fatalf("parse birthdate: %v", err)
	}
	u := &domain.User{
		Username:  "racp_test_" + suf,
		Email:     "racp_test_" + suf + "@example.invalid",
		Gender:    "M",
		Birthdate: wantBirthdate,
	}
	created, err := repo.Create(ctx, u, "Test1234!")
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
	if got.Birthdate.Format("2006-01-02") != "2008-06-05" {
		t.Errorf("Birthdate = %v; want 2008-06-05", got.Birthdate)
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
	birthdate, _ := time.Parse("2006-01-02", "2008-06-05")
	u := &domain.User{
		Username:  "racp_test_" + suf,
		Email:     "racp_test_" + suf + "@example.invalid",
		Gender:    "M",
		Birthdate: birthdate,
	}
	created, err := repo.Create(ctx, u, "secret")
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
		Gender:   "M",
	}, "Test1234!")
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
			Gender:   "M",
		}, "Test1234!")
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
			Gender:   "M",
		}, "Test1234!")
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
		Gender:   "M",
	}, "Old1234!")
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
		Gender:   "M",
	}, "Old1234!")
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
		Gender:   "M",
	}, "Test1234!")
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

func TestRepository_VerifyPassword(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	wantBirthdate, err := time.Parse("2006-01-02", "2008-06-05")
	if err != nil {
		t.Fatalf("parse birthdate: %v", err)
	}
	u := &domain.User{
		Username:  "racp_test_" + suf,
		Email:     "racp_test_" + suf + "@example.invalid",
		Gender:    "M",
		Birthdate: wantBirthdate,
	}
	created, err := repo.Create(ctx, u, "Test1234!")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	cleanupUser(t, repo, created.ID)

	ok, err := repo.VerifyPassword(ctx, created.ID, "Test1234!")
	if err != nil {
		t.Fatalf("VerifyPassword good: %v", err)
	}
	if !ok {
		t.Fatalf("VerifyPassword good: got false, want true")
	}

	ok, err = repo.VerifyPassword(ctx, created.ID, "wrongpass")
	if err != nil {
		t.Fatalf("VerifyPassword bad: %v", err)
	}
	if ok {
		t.Fatalf("VerifyPassword bad: got true, want false")
	}

	_, err = repo.VerifyPassword(ctx, -1, "anything")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("VerifyPassword missing: err = %v, want ErrUserNotFound", err)
	}
}

func createSeedUser(t *testing.T, repo *Repository, username, email string) *domain.User {
	t.Helper()
	bd, err := time.Parse("2006-01-02", "2008-06-05")
	if err != nil {
		t.Fatalf("parse birthdate: %v", err)
	}
	u, err := repo.Create(context.Background(), &domain.User{
		Username:  username,
		Email:     email,
		Gender:    "M",
		Birthdate: bd,
	}, "Test1234!")
	if err != nil {
		t.Fatalf("Create %s: %v", username, err)
	}
	cleanupUser(t, repo, u.ID)

	return u
}

func TestRepository_List_PaginatesAndFiltersByQuery(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	wanted := createSeedUser(t, repo, "kaki_"+suf, "kaki_"+suf+"@example.invalid")
	_ = createSeedUser(t, repo, "other_"+suf, "other_"+suf+"@example.invalid")

	page, err := repo.List(ctx, ListQuery{Page: 1, PerPage: 20, Query: "kaki_" + suf})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 1 || len(page.Users) != 1 {
		t.Fatalf("page total/len = %d/%d, want 1/1", page.Total, len(page.Users))
	}
	if page.Users[0].ID != wanted.ID {
		t.Errorf("returned ID = %d, want %d", page.Users[0].ID, wanted.ID)
	}
}

func TestRepository_List_ExcludesActor(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	actor := createSeedUser(t, repo, "actor_"+suf, "actor_"+suf+"@example.invalid")
	other := createSeedUser(t, repo, "other_"+suf, "other_"+suf+"@example.invalid")

	page, err := repo.List(ctx, ListQuery{Page: 1, PerPage: 20, Query: suf, ExcludeID: actor.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 1 || len(page.Users) != 1 || page.Users[0].ID != other.ID {
		t.Errorf("page = %+v, want exactly other (id=%d)", page, other.ID)
	}
}

func TestRepository_UpdateBan_TempThenClear(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	u := createSeedUser(t, repo, "ban_"+suf, "ban_"+suf+"@example.invalid")

	unbanAt := uint32(time.Now().Add(time.Hour).Unix()) //nolint:gosec // unban_time is uint32 in rAthena
	if err := repo.UpdateBan(ctx, u.ID, 0, unbanAt); err != nil {
		t.Fatalf("UpdateBan temp: %v", err)
	}
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after temp ban: %v", err)
	}
	if got.UnbanTime.IsZero() {
		t.Errorf("UnbanTime should be set after temp ban, got zero")
	}

	if err := repo.UpdateBan(ctx, u.ID, 0, 0); err != nil {
		t.Fatalf("UpdateBan clear: %v", err)
	}
	got, err = repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after clear: %v", err)
	}
	if !got.UnbanTime.IsZero() {
		t.Errorf("UnbanTime should be zero after clear, got %v", got.UnbanTime)
	}
}

func TestRepository_UpdateBan_Missing(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	if err := repo.UpdateBan(context.Background(), -1, 0, 0); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("got %v, want ErrUserNotFound", err)
	}
}

func TestRepository_UpdateGroup(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	ctx := context.Background()

	suf := randomizeSuffix(t)
	u := createSeedUser(t, repo, "grp_"+suf, "grp_"+suf+"@example.invalid")

	if err := repo.UpdateGroup(ctx, u.ID, 20); err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after UpdateGroup: %v", err)
	}
	if got.GroupID != 20 {
		t.Errorf("GroupID = %d, want 20", got.GroupID)
	}
}

func TestRepository_UpdateGroup_Missing(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	if err := repo.UpdateGroup(context.Background(), -1, 20); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("got %v, want ErrUserNotFound", err)
	}
}
