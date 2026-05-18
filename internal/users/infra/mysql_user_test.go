//go:build integration

package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/testutil"
	"github.com/hayakawakaki/go-racp/internal/users/domain"
)

func setupUserRepo(t *testing.T) *UserRepository {
	t.Helper()
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "login")

	return NewUserRepository(db)
}

func insertLogin(t *testing.T, repo *UserRepository, userid, email string, state, groupID int, unbanSecs uint32) int {
	t.Helper()
	res, err := repo.Client.ExecContext(context.Background(),
		"INSERT INTO login (userid, email, user_pass, sex, birthdate, state, group_id, unban_time, last_ip, lastlogin) VALUES (?, ?, 'x', 'M', '2000-01-01', ?, ?, ?, '127.0.0.1', NOW())",
		userid, email, state, groupID, unbanSecs,
	)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed lastInsertId: %v", err)
	}

	return int(id)
}

func TestUserRepository_GetByID(t *testing.T) {
	repo := setupUserRepo(t)
	id := insertLogin(t, repo, "testuser", "test@example.com", 0, 0, 0)

	user, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if user.Username != "testuser" || user.Email != "test@example.com" {
		t.Errorf("user = %+v", user)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	repo := setupUserRepo(t)
	_, err := repo.GetByID(context.Background(), 9_999_999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUserRepository_List_PaginatesAndFiltersByQuery(t *testing.T) {
	repo := setupUserRepo(t)
	insertLogin(t, repo, "kaki", "kaki@example.com", 0, 0, 0)
	insertLogin(t, repo, "crazyarashi", "crazy@example.com", 0, 0, 0)
	insertLogin(t, repo, "testuser", "tester@example.com", 0, 0, 0)

	page, err := repo.List(context.Background(), ListQuery{Page: 1, PerPage: 20, Query: "kaki"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 1 || len(page.Users) != 1 || page.Users[0].Username != "kaki" {
		t.Errorf("page = %+v", page)
	}
}

func TestUserRepository_UpdateBan_TempThenUnban(t *testing.T) {
	repo := setupUserRepo(t)
	id := insertLogin(t, repo, "testuser", "test@example.com", 0, 0, 0)

	unbanAt := uint32(time.Now().Add(time.Hour).Unix())
	if err := repo.UpdateBan(context.Background(), id, 0, unbanAt); err != nil {
		t.Fatalf("UpdateBan temp: %v", err)
	}
	user, _ := repo.GetByID(context.Background(), id)
	if user.UnbanTime.IsZero() {
		t.Errorf("UnbanTime should be set, got zero")
	}

	if err := repo.UpdateBan(context.Background(), id, 0, 0); err != nil {
		t.Fatalf("UpdateBan clear: %v", err)
	}
	user, _ = repo.GetByID(context.Background(), id)
	if !user.UnbanTime.IsZero() {
		t.Errorf("UnbanTime should be zero, got %v", user.UnbanTime)
	}
}

func TestUserRepository_UpdateGroup(t *testing.T) {
	repo := setupUserRepo(t)
	id := insertLogin(t, repo, "testuser", "test@example.com", 0, 0, 0)

	if err := repo.UpdateGroup(context.Background(), id, 20); err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	user, _ := repo.GetByID(context.Background(), id)
	if user.GroupID != 20 {
		t.Errorf("GroupID = %d, want 20", user.GroupID)
	}
}
