package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type SessionService struct {
	repo domain.SessionRepository
	now  func() time.Time
	ttl  time.Duration
}

func NewSessionService(repo domain.SessionRepository, ttl time.Duration) *SessionService {
	if ttl <= 0 {
		panic("session ttl must be > 0")
	}
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

// Validate looks up the session, slides its expiry forward by the configured TTL, and deletes the row when it has already expired.
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

func (s *SessionService) InvalidateAll(ctx context.Context, userID int) error {
	if err := s.repo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("app.SessionService.InvalidateAll: %w", err)
	}

	return nil
}

func (s *SessionService) InvalidateOthers(ctx context.Context, userID int, rawCurrentToken string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(rawCurrentToken)
	if err != nil || len(decoded) != 32 {
		return domain.ErrInvalidCurrentSessionToken
	}

	hash := sha256.Sum256(decoded)
	if err := s.repo.DeleteByUserIDExcept(ctx, userID, hash); err != nil {
		return fmt.Errorf("app.SessionService.InvalidateOthers: %w", err)
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
