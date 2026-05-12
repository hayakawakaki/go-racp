package actiontoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

type Manager struct {
	repo Repository
	now  func() time.Time
}

func NewManager(repo Repository) *Manager {
	return &Manager{repo: repo, now: time.Now}
}

func (m *Manager) Issue(ctx context.Context, action Action, accountID int, payload []byte, ttl time.Duration) (string, error) {
	if err := m.repo.DeleteUnconsumed(ctx, accountID, action); err != nil {
		return "", fmt.Errorf("actiontoken.Manager.Issue: %w", err)
	}

	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("actiontoken.Manager.Issue: %w", err)
	}

	hash := sha256.Sum256(raw[:])
	now := m.now()
	token := &ActionToken{
		TokenHash: hash,
		AccountID: accountID,
		Action:    action,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		Payload:   payload,
	}
	if err := m.repo.Insert(ctx, token); err != nil {
		return "", fmt.Errorf("actiontoken.Manager.Issue: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (m *Manager) Consume(ctx context.Context, action Action, rawToken string) (*ActionToken, error) {
	now := m.now()
	token, err := m.lookup(ctx, action, rawToken, now)
	if err != nil {
		return nil, err
	}

	if err := m.repo.MarkConsumed(ctx, token.TokenHash, now); err != nil {
		return nil, fmt.Errorf("actiontoken.Manager.Consume: %w", err)
	}
	token.ConsumedAt.Time = now
	token.ConsumedAt.Valid = true

	return token, nil
}

func (m *Manager) Peek(ctx context.Context, action Action, rawToken string) (*ActionToken, error) {
	return m.lookup(ctx, action, rawToken, m.now())
}

func (m *Manager) MostRecentIssuedAt(ctx context.Context, accountID int, action Action) (time.Time, error) {
	t, err := m.repo.MostRecentIssuedAt(ctx, accountID, action)
	if err != nil {
		return time.Time{}, fmt.Errorf("actiontoken.Manager.MostRecentIssuedAt: %w", err)
	}

	return t, nil
}

func (m *Manager) lookup(ctx context.Context, action Action, rawToken string, now time.Time) (*ActionToken, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	if err != nil || len(decoded) != 32 {
		return nil, ErrTokenInvalid
	}
	hash := sha256.Sum256(decoded)

	token, err := m.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("actiontoken.Manager.lookup: %w", err)
	}

	if token.Action != action {
		return nil, ErrTokenInvalid
	}

	if token.IsConsumed() {
		return nil, ErrTokenAlreadyUsed
	}

	if token.IsExpired(now) {
		return nil, ErrTokenExpired
	}

	return token, nil
}
