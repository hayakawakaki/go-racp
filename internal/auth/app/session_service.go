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

const SessionTTL = 24 * time.Hour

type SessionService struct {
	repo domain.SessionRepository
	now  func() time.Time
}

func NewSessionService(repo domain.SessionRepository) *SessionService {
	return &SessionService{repo: repo, now: time.Now}
}

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
		ExpiresAt:  now.Add(SessionTTL),
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
	newExp := now.Add(SessionTTL)
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

func decodeTokenToHash(rawToken string) ([32]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, false
	}
	return sha256.Sum256(raw), true
}
