//go:build integration

package infra

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func TestSessionRepository_DeleteExpired_RemovesExpiredOnly(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_sessions")
	repo := NewSessionRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	expired := &domain.Session{
		TokenHash:  sha256.Sum256([]byte{0xc1}),
		UserID:     1,
		ExpiresAt:  now.Add(-time.Hour),
		LastSeenAt: now,
		CreatedAt:  now,
	}
	active := &domain.Session{
		TokenHash:  sha256.Sum256([]byte{0xc2}),
		UserID:     2,
		ExpiresAt:  now.Add(time.Hour),
		LastSeenAt: now,
		CreatedAt:  now,
	}

	if err := repo.Create(ctx, expired); err != nil {
		t.Fatalf("Create expired: %v", err)
	}
	if err := repo.Create(ctx, active); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	deleted, err := repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	if _, err := repo.GetByTokenHash(ctx, expired.TokenHash); err == nil {
		t.Errorf("expired session still present")
	}
	if _, err := repo.GetByTokenHash(ctx, active.TokenHash); err != nil {
		t.Errorf("active session was deleted: %v", err)
	}
}

func TestSessionRepository_DeleteExpired_EmptyTable(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_sessions")
	repo := NewSessionRepository(pool)

	deleted, err := repo.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}
