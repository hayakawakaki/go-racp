//go:build integration

package infra

import (
	"context"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func setupAuditRepo(t *testing.T) *ActionRepository {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_user_actions")

	return NewActionRepository(pool)
}

func TestActionRepository_RecordAndList(t *testing.T) {
	repo := setupAuditRepo(t)
	ctx := context.Background()

	if err := repo.Record(ctx, domain.Action{
		ActorUserID:  10,
		TargetUserID: 20,
		Kind:         domain.ActionBan,
		Reason:       "spam",
		BeforeValue:  "0,0",
		AfterValue:   "5,0",
	}); err != nil {
		t.Fatalf("Record ban: %v", err)
	}
	if err := repo.Record(ctx, domain.Action{
		ActorUserID:  10,
		TargetUserID: 20,
		Kind:         domain.ActionSetRole,
		Reason:       "demote",
		BeforeValue:  "20",
		AfterValue:   "0",
	}); err != nil {
		t.Fatalf("Record set_role: %v", err)
	}

	list, err := repo.ListByTarget(ctx, 20, 10)
	if err != nil {
		t.Fatalf("ListByTarget: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Kind != domain.ActionSetRole {
		t.Errorf("first row should be most recent (set_role); got %s", list[0].Kind)
	}
	if list[0].At.IsZero() {
		t.Errorf("At should be set by DB default")
	}
}
