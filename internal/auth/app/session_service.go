package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/auth/domain"
)

type SessionService struct {
	repo domain.SessionRepository
	now  func() time.Time
	ttl  time.Duration
}

func NewSessionService(repo domain.SessionRepository, ttl time.Duration) *SessionService {
	return &SessionService{repo: repo, ttl: ttl, now: time.Now}
}

func (s *SessionService) TTL() time.Duration { return s.ttl }

func (s *SessionService) Create(ctx context.Context, userID int) (string, *domain.Session, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", nil, fmt.Errorf("app.SessionService.Create: %w", err)
	}
	hash := sha256.Sum256(raw[:])
	now := s.now()
	sess := &domain.Session{
		TokenHash:  hash,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.ttl),
	}
	if err := s.repo.Create(ctx, sess); err != nil {
		return "", nil, fmt.Errorf("app.SessionService.Create: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), sess, nil
}

func (s *SessionService) Validate(ctx context.Context, rawToken string) (*domain.Session, error) {
	hash, ok := decodeTokenToHash(rawToken)
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	sess, err := s.repo.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("app.SessionService.Validate: %w", err)
	}
	now := s.now()
	if sess.IsExpired(now) {
		_ = s.repo.Delete(ctx, hash)
		return nil, domain.ErrSessionExpired
	}
	newExp := now.Add(s.ttl)
	if err := s.repo.Refresh(ctx, hash, now, newExp); err != nil {
		return nil, fmt.Errorf("app.SessionService.Validate: %w", err)
	}
	sess.LastSeenAt = now
	sess.ExpiresAt = newExp
	return sess, nil
}

func (s *SessionService) Destroy(ctx context.Context, rawToken string) error {
	hash, ok := decodeTokenToHash(rawToken)
	if !ok {
		return nil
	}
	if err := s.repo.Delete(ctx, hash); err != nil && !errors.Is(err, domain.ErrSessionNotFound) {
		return fmt.Errorf("app.SessionService.Destroy: %w", err)
	}
	return nil
}

// decodeTokenToHash decodes rawToken using base64.RawURLEncoding, verifies the decoded bytes are exactly 32 bytes, and returns the SHA-256 hash of those bytes and true. If decoding fails or the length is not 32 it returns a zero hash and false.
func decodeTokenToHash(rawToken string) ([32]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(raw), true
}
