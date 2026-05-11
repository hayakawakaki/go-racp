//go:build integration

package infra

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

var _ domain.TokenRepository = (*TokenRepository)(nil)

func newActionToken(accountID int, action domain.Action, now time.Time, suffix byte) *domain.ActionToken {
	return &domain.ActionToken{
		TokenHash: sha256.Sum256([]byte{suffix}),
		AccountID: accountID,
		Action:    action,
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
}

func TestTokenRepository_InsertAndGetByHash(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	token := newActionToken(101, domain.ActionEmailVerification, now, 0x01)
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
}

func TestTokenRepository_GetByHash_NotFound(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)

	_, err := repo.GetByHash(context.Background(), sha256.Sum256([]byte("nope")))
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestTokenRepository_DeleteUnconsumed(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	tokenA := newActionToken(200, domain.ActionEmailVerification, now, 0xa1)
	tokenB := newActionToken(200, domain.ActionEmailVerification, now, 0xa2)
	tokenC := newActionToken(201, domain.ActionEmailVerification, now, 0xa3)
	for _, token := range []*domain.ActionToken{tokenA, tokenB, tokenC} {
		if err := repo.Insert(ctx, token); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	if err := repo.MarkConsumed(ctx, tokenB.TokenHash, now); err != nil {
		t.Fatalf("MarkConsumed: %v", err)
	}

	if err := repo.DeleteUnconsumed(ctx, 200, domain.ActionEmailVerification); err != nil {
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

func TestTokenRepository_DeleteUnconsumed_NoRows(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)

	err := repo.DeleteUnconsumed(context.Background(), 999, domain.ActionEmailVerification)
	if err != nil {
		t.Errorf("DeleteUnconsumed against empty set: got %v, want nil", err)
	}
}

func TestTokenRepository_MarkConsumed_Happy(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	token := newActionToken(301, domain.ActionEmailVerification, now, 0xb1)
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

func TestTokenRepository_MarkConsumed_AlreadyConsumed(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	token := newActionToken(302, domain.ActionEmailVerification, now, 0xb2)
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

func TestTokenRepository_MarkConsumed_UnknownHash(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)

	err := repo.MarkConsumed(context.Background(), sha256.Sum256([]byte("ghost")), time.Now())
	if !errors.Is(err, domain.ErrTokenInvalid) {
		t.Errorf("got %v, want ErrTokenInvalid", err)
	}
}

func TestTokenRepository_MostRecentIssuedAt_NoRows(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)

	got, err := repo.MostRecentIssuedAt(context.Background(), 999, domain.ActionEmailVerification)
	if err != nil {
		t.Fatalf("MostRecentIssuedAt: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero time for empty result", got)
	}
}

func TestTokenRepository_MostRecentIssuedAt_ReturnsLatest(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	earlier := newActionToken(401, domain.ActionEmailVerification, base.Add(-2*time.Hour), 0xc1)
	latest := newActionToken(401, domain.ActionEmailVerification, base, 0xc2)
	otherAccount := newActionToken(402, domain.ActionEmailVerification, base.Add(time.Hour), 0xc3)
	for _, token := range []*domain.ActionToken{earlier, latest, otherAccount} {
		if err := repo.Insert(ctx, token); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	got, err := repo.MostRecentIssuedAt(ctx, 401, domain.ActionEmailVerification)
	if err != nil {
		t.Fatalf("MostRecentIssuedAt: %v", err)
	}
	if !got.Equal(base) {
		t.Errorf("got %v, want %v (latest for account 401)", got, base)
	}
}

func TestTokenRepository_MostRecentIssuedAt_FiltersByAction(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	testutil.TruncateMariaDB(t, db, "cp_action_tokens")
	repo := NewTokenRepository(db)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	verify := newActionToken(501, domain.ActionEmailVerification, base, 0xd1)
	other := newActionToken(501, domain.ActionUnknown, base.Add(time.Hour), 0xd2)
	for _, token := range []*domain.ActionToken{verify, other} {
		if err := repo.Insert(ctx, token); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	got, err := repo.MostRecentIssuedAt(ctx, 501, domain.ActionEmailVerification)
	if err != nil {
		t.Fatalf("MostRecentIssuedAt: %v", err)
	}
	if !got.Equal(base) {
		t.Errorf("got %v, want %v (action filter ignores ActionUnknown row)", got, base)
	}
}
