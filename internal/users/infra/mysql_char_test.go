//go:build integration

package infra

import (
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func setupCharRepo(t *testing.T) *CharRepository {
	t.Helper()
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "char")

	return NewCharRepository(db)
}

func TestCharRepository_ListByAccount_Empty(t *testing.T) {
	repo := setupCharRepo(t)
	chars, err := repo.ListByAccount(context.Background(), 12345)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(chars) != 0 {
		t.Errorf("got %d chars, want 0", len(chars))
	}
}

func TestCharRepository_ListByAccount_OrdersBySlot(t *testing.T) {
	repo := setupCharRepo(t)
	_, err := repo.Client.ExecContext(context.Background(),
		"INSERT INTO `char` (account_id, char_num, name, class, base_level, job_level, zeny, last_map, online, last_login) VALUES "+
			"(7, 1, 'Beta',  1, 50, 30, 1000, 'prontera', 0, NOW()), "+
			"(7, 0, 'Alpha', 0, 10, 5,  100,  'prontera', 1, NOW())")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	chars, err := repo.ListByAccount(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(chars) != 2 || chars[0].Name != "Alpha" || chars[1].Name != "Beta" {
		t.Errorf("ordering wrong: %+v", chars)
	}
}
