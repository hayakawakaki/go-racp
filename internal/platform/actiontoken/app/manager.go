package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
)

type Manager struct {
	repo domain.Repository
	now  func() time.Time
}

func NewManager(repo domain.Repository) *Manager {
	return &Manager{repo: repo, now: time.Now}
}

// Issue invalidates any prior unconsumed tokens for the account and action, then mints a fresh single-use token.
func (m *Manager) Issue(ctx context.Context, action domain.Action, accountID int, payload []byte, ttl time.Duration) (string, error) {
	if err := m.repo.DeleteUnconsumed(ctx, accountID, action); err != nil {
		return "", fmt.Errorf("app.Manager.Issue: %w", err)
	}

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("app.Manager.Issue: %w", err)
	}

	hash := sha256.Sum256(raw[:])
	now := m.now()
	token := &domain.ActionToken{
		TokenHash: hash,
		AccountID: accountID,
		Action:    action,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		Payload:   payload,
	}
	if err := m.repo.Insert(ctx, token); err != nil {
		return "", fmt.Errorf("app.Manager.Issue: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// Consume validates the token and atomically marks it used so it cannot be replayed.
func (m *Manager) Consume(ctx context.Context, action domain.Action, rawToken string) (*domain.ActionToken, error) {
	now := m.now()
	token, err := m.lookup(ctx, action, rawToken, now)
	if err != nil {
		return nil, err
	}

	if err := m.repo.MarkConsumed(ctx, token.TokenHash, now); err != nil {
		return nil, fmt.Errorf("app.Manager.Consume: %w", err)
	}
	token.ConsumedAt.Time = now
	token.ConsumedAt.Valid = true

	return token, nil
}

// Peek validates the token and returns it without marking it consumed.
func (m *Manager) Peek(ctx context.Context, action domain.Action, rawToken string) (*domain.ActionToken, error) {
	return m.lookup(ctx, action, rawToken, m.now())
}

func (m *Manager) MostRecentIssuedAt(ctx context.Context, accountID int, action domain.Action) (time.Time, error) {
	t, err := m.repo.MostRecentIssuedAt(ctx, accountID, action)
	if err != nil {
		return time.Time{}, fmt.Errorf("app.Manager.MostRecentIssuedAt: %w", err)
	}

	return t, nil
}

func (m *Manager) lookup(ctx context.Context, action domain.Action, rawToken string, now time.Time) (*domain.ActionToken, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return nil, domain.ErrTokenInvalid
	}
	hash := sha256.Sum256(decoded)

	token, err := m.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("app.Manager.lookup: %w", err)
	}

	if token.Action != action {
		return nil, domain.ErrTokenInvalid
	}

	if token.IsConsumed() {
		return nil, domain.ErrTokenAlreadyUsed
	}

	if token.IsExpired(now) {
		return nil, domain.ErrTokenExpired
	}

	return token, nil
}
