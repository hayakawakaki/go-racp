//go:build integration

package infra

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

// Compile-time interface check.
var _ domain.SessionRepository = (*SessionRepository)(nil)

func newSession(userID int, base time.Time, suffix byte) *domain.Session {
	hash := sha256.Sum256([]byte{suffix})
	return &domain.Session{
		TokenHash:  hash,
		UserID:     userID,
		CreatedAt:  base,
		LastSeenAt: base,
		ExpiresAt:  base.Add(24 * time.Hour),
	}
}

func TestSessionRepository_CreateAndGet(t *testing.T) {
	db := openDB(t)
	truncateSessions(t, db)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	s := newSession(42, base, 0xa1)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByTokenHash(ctx, s.TokenHash)
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if got.UserID != s.UserID {
		t.Errorf("UserID = %d, want %d", got.UserID, s.UserID)
	}
	if !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, s.ExpiresAt)
	}
	if !got.CreatedAt.Equal(s.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, s.CreatedAt)
	}
	if got.TokenHash != s.TokenHash {
		t.Errorf("TokenHash mismatch")
	}
}

func TestSessionRepository_GetByTokenHash_NotFound(t *testing.T) {
	db := openDB(t)
	truncateSessions(t, db)
	repo := NewSessionRepository(db)

	_, err := repo.GetByTokenHash(context.Background(), sha256.Sum256([]byte("nope")))
	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("got %v, want ErrSessionNotFound", err)
	}
}

func TestSessionRepository_Refresh_PreservesUserAndCreated(t *testing.T) {
	db := openDB(t)
	truncateSessions(t, db)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	s := newSession(7, base, 0xb2)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	newLastSeen := base.Add(time.Hour)
	newExp := base.Add(25 * time.Hour)
	if err := repo.Refresh(ctx, s.TokenHash, newLastSeen, newExp); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	got, err := repo.GetByTokenHash(ctx, s.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastSeenAt.Equal(newLastSeen) {
		t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, newLastSeen)
	}
	if !got.ExpiresAt.Equal(newExp) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, newExp)
	}
	if got.UserID != s.UserID {
		t.Errorf("UserID changed: got %d, want %d", got.UserID, s.UserID)
	}
	if !got.CreatedAt.Equal(s.CreatedAt) {
		t.Errorf("CreatedAt changed: got %v, want %v", got.CreatedAt, s.CreatedAt)
	}
}

func TestSessionRepository_Refresh_NotFound(t *testing.T) {
	db := openDB(t)
	truncateSessions(t, db)
	repo := NewSessionRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	err := repo.Refresh(context.Background(), sha256.Sum256([]byte("nope")), now, now.Add(time.Hour))
	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("got %v, want ErrSessionNotFound", err)
	}
}

func TestSessionRepository_Delete(t *testing.T) {
	db := openDB(t)
	truncateSessions(t, db)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	s := newSession(1, base, 0xc3)
	if err := repo.Create(ctx, s); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, s.TokenHash); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByTokenHash(ctx, s.TokenHash); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("row still present after Delete: err=%v", err)
	}

	// Idempotent re-delete returns ErrSessionNotFound (caller decides whether to swallow).
	if err := repo.Delete(ctx, s.TokenHash); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("second Delete: got %v, want ErrSessionNotFound", err)
	}
}
