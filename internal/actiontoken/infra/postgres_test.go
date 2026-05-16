//go:build integration

package infra

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

var _ domain.Repository = (*PostgresRepository)(nil)

func newPersistedToken(accountID int, action domain.Action, now time.Time, suffix byte) *domain.ActionToken {
	return &domain.ActionToken{
		TokenHash: sha256.Sum256([]byte{suffix}),
		AccountID: accountID,
		Action:    action,
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
}

func TestPostgresRepository_InsertAndGetByHash(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	token := newPersistedToken(101, domain.EmailVerification, now, 0x01)
	token.Payload = []byte("payload-bytes")
	if err := repo.Insert(ctx, token); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := repo.GetByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.AccountID != token.AccountID {
		t.Errorf("AccountID = %d, want %d", got.AccountID, token.AccountID)
	}
	if got.Action != token.Action {
		t.Errorf("Action = %v, want %v", got.Action, token.Action)
	}
	if got.TokenHash != token.TokenHash {
		t.Errorf("TokenHash mismatch: got %x, want %x", got.TokenHash, token.TokenHash)
	}
	if !got.ExpiresAt.Equal(token.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, token.ExpiresAt)
	}
	if !got.CreatedAt.Equal(token.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, token.CreatedAt)
	}
	if got.ConsumedAt.Valid {
		t.Errorf("ConsumedAt should not be set on insert; got %+v", got.ConsumedAt)
	}
	if string(got.Payload) != "payload-bytes" {
		t.Errorf("Payload = %q, want payload-bytes", string(got.Payload))
	}
}

func TestPostgresRepository_GetByHash_NotFound(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)

	_, err := repo.GetByHash(context.Background(), sha256.Sum256([]byte("nope")))
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestPostgresRepository_DeleteUnconsumed(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	tokenA := newPersistedToken(200, domain.EmailVerification, now, 0xa1)
	tokenB := newPersistedToken(200, domain.EmailVerification, now, 0xa2)
	tokenC := newPersistedToken(201, domain.EmailVerification, now, 0xa3)
	for _, token := range []*domain.ActionToken{tokenA, tokenB, tokenC} {
		if err := repo.Insert(ctx, token); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	if err := repo.MarkConsumed(ctx, tokenB.TokenHash, now); err != nil {
		t.Fatalf("MarkConsumed: %v", err)
	}

	if err := repo.DeleteUnconsumed(ctx, 200, domain.EmailVerification); err != nil {
		t.Fatalf("DeleteUnconsumed: %v", err)
	}

	if _, err := repo.GetByHash(ctx, tokenA.TokenHash); !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("tokenA: got %v, want ErrTokenInvalid (deleted)", err)
	}
	if _, err := repo.GetByHash(ctx, tokenB.TokenHash); err != nil {
		t.Errorf("tokenB consumed; should still exist after DeleteUnconsumed: %v", err)
	}
	if _, err := repo.GetByHash(ctx, tokenC.TokenHash); err != nil {
		t.Errorf("tokenC belongs to other account; should still exist: %v", err)
	}
}

func TestPostgresRepository_DeleteUnconsumed_NoRows(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)

	if err := repo.DeleteUnconsumed(context.Background(), 999, domain.EmailVerification); err != nil {
		t.Errorf("DeleteUnconsumed against empty set: got %v, want nil", err)
	}
}

func TestPostgresRepository_MarkConsumed_Happy(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	token := newPersistedToken(301, domain.EmailVerification, now, 0xb1)
	if err := repo.Insert(ctx, token); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	consumedAt := now.Add(time.Minute)
	if err := repo.MarkConsumed(ctx, token.TokenHash, consumedAt); err != nil {
		t.Fatalf("MarkConsumed: %v", err)
	}

	got, err := repo.GetByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if !got.ConsumedAt.Valid {
		t.Errorf("ConsumedAt.Valid = false, want true")
	}
	if !got.ConsumedAt.Time.Equal(consumedAt) {
		t.Errorf("ConsumedAt = %v, want %v", got.ConsumedAt.Time, consumedAt)
	}
}

func TestPostgresRepository_MarkConsumed_AlreadyConsumed(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	token := newPersistedToken(302, domain.EmailVerification, now, 0xb2)
	if err := repo.Insert(ctx, token); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := repo.MarkConsumed(ctx, token.TokenHash, now); err != nil {
		t.Fatalf("first MarkConsumed: %v", err)
	}

	err := repo.MarkConsumed(ctx, token.TokenHash, now.Add(time.Hour))
	if !errors.Is(err, domain.ErrTokenAlreadyUsed) {
		t.Errorf("second MarkConsumed: got %v, want ErrTokenAlreadyUsed", err)
	}
}

func TestPostgresRepository_MarkConsumed_UnknownHash(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)

	err := repo.MarkConsumed(context.Background(), sha256.Sum256([]byte("ghost")), time.Now())
	if !errors.Is(err, domain.ErrTokenAlreadyUsed) {
		t.Errorf("got %v, want ErrTokenAlreadyUsed (unknown hash treated as 0 rows)", err)
	}
}

func TestPostgresRepository_MostRecentIssuedAt_NoRows(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)

	got, err := repo.MostRecentIssuedAt(context.Background(), 999, domain.EmailVerification)
	if err != nil {
		t.Fatalf("MostRecentIssuedAt: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero time for empty result", got)
	}
}

func TestPostgresRepository_MostRecentIssuedAt_ReturnsLatest(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	earlier := newPersistedToken(401, domain.EmailVerification, base.Add(-2*time.Hour), 0xc1)
	latest := newPersistedToken(401, domain.EmailVerification, base, 0xc2)
	otherAccount := newPersistedToken(402, domain.EmailVerification, base.Add(time.Hour), 0xc3)
	for _, token := range []*domain.ActionToken{earlier, latest, otherAccount} {
		if err := repo.Insert(ctx, token); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	got, err := repo.MostRecentIssuedAt(ctx, 401, domain.EmailVerification)
	if err != nil {
		t.Fatalf("MostRecentIssuedAt: %v", err)
	}
	if !got.Equal(base) {
		t.Errorf("got %v, want %v (latest for account 401)", got, base)
	}
}

func TestPostgresRepository_MostRecentIssuedAt_FiltersByAction(t *testing.T) {
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_action_tokens")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	verify := newPersistedToken(501, domain.EmailVerification, base, 0xd1)
	other := newPersistedToken(501, domain.PasswordReset, base.Add(time.Hour), 0xd2)
	for _, token := range []*domain.ActionToken{verify, other} {
		if err := repo.Insert(ctx, token); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	got, err := repo.MostRecentIssuedAt(ctx, 501, domain.EmailVerification)
	if err != nil {
		t.Fatalf("MostRecentIssuedAt: %v", err)
	}
	if !got.Equal(base) {
		t.Errorf("got %v, want %v (action filter ignores other-action row)", got, base)
	}
}
