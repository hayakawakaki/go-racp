package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
)

func TestService_Authenticate_ReturnsTier(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	future := fixed.Add(time.Hour)
	past := fixed.Add(-time.Hour)

	tests := []struct {
		unbanTime time.Time
		name      string
		state     int
		want      Tier
	}{
		{name: "active state zero unban", state: 0, unbanTime: time.Time{}, want: TierActive},
		{name: "active state past unban", state: 0, unbanTime: past, want: TierActive},
		{name: "active state future unban is temp banned", state: 0, unbanTime: future, want: TierTempBanned},
		{name: "unverified state", state: 1, unbanTime: time.Time{}, want: TierUnverified},
		{name: "perma banned state", state: 5, unbanTime: time.Time{}, want: TierPermaBanned},
		{name: "unknown state defaults to active", state: 9, unbanTime: time.Time{}, want: TierActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeUserRepo()
			_, _ = repo.Create(context.Background(), &domain.User{
				Username: "testuser", Password: "Test1234!",
				State: tt.state, UnbanTime: tt.unbanTime,
			})
			svc := NewService(repo)
			svc.now = func() time.Time { return fixed }

			_, gotTier, err := svc.Authenticate(context.Background(), LoginCommand{
				Username: "testuser", Password: "Test1234!",
			})
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if gotTier != tt.want {
				t.Errorf("Tier = %v, want %v", gotTier, tt.want)
			}
		})
	}
}

func TestService_AssertUnrestricted(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	future := fixed.Add(time.Hour)

	tests := []struct {
		setup    func(repo *fakeUserRepo) int
		wantErr  error
		name     string
		wantPass bool
	}{
		{
			name: "active passes",
			setup: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{Username: "u", State: 0})
				return u.ID
			},
			wantPass: true,
		},
		{
			name: "perma banned returns ErrAccountPermaBanned",
			setup: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{Username: "u", State: 5})
				return u.ID
			},
			wantErr: domain.ErrAccountPermaBanned,
		},
		{
			name: "temp banned returns ErrAccountTempBanned",
			setup: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{
					Username: "u", State: 0, UnbanTime: future,
				})
				return u.ID
			},
			wantErr: domain.ErrAccountTempBanned,
		},
		{
			name: "user not found returns ErrAccountDeleted",
			setup: func(_ *fakeUserRepo) int {
				return 999
			},
			wantErr: domain.ErrAccountDeleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newFakeUserRepo()
			id := tt.setup(repo)
			svc := NewService(repo)
			svc.now = func() time.Time { return fixed }

			err := svc.assertUnrestricted(context.Background(), id)
			if tt.wantPass {
				if err != nil {
					t.Errorf("assertUnrestricted returned %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("assertUnrestricted err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_AssertUnrestricted_RepoError_Wraps(t *testing.T) {
	t.Parallel()
	repo := newFakeUserRepo()
	repo.getByIDHook = func(int) (*domain.User, error) { return nil, errors.New("db boom") }
	svc := NewService(repo)

	err := svc.assertUnrestricted(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, domain.ErrAccountDeleted) || errors.Is(err, domain.ErrAccountPermaBanned) || errors.Is(err, domain.ErrAccountTempBanned) {
		t.Errorf("repo error should not surface as a tier sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), "app.Service.assertUnrestricted") {
		t.Errorf("not wrapped: %v", err)
	}
}

func seedResetToken(t *testing.T, fx *resetFixture, accountID int, marker byte) string {
	t.Helper()
	var raw [32]byte
	raw[0] = marker
	hash := sha256.Sum256(raw[:])
	now := time.Now()
	fx.tokenRepo.byHash[hash] = &actiontokendomain.ActionToken{
		TokenHash: hash, AccountID: accountID, Action: actiontokendomain.PasswordReset,
		ExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Minute),
	}

	return base64.RawURLEncoding.EncodeToString(raw[:])
}

func TestService_ConsumePasswordReset_TierRejections(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	future := fixed.Add(time.Hour)

	tests := []struct {
		seedUser func(*fakeUserRepo) int
		wantErr  error
		name     string
		marker   byte
	}{
		{
			name:   "perma banned",
			marker: 0xd1,
			seedUser: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{
					Username: "u", Email: "u@x", Password: "Old1234!", State: 5,
				})
				return u.ID
			},
			wantErr: domain.ErrAccountPermaBanned,
		},
		{
			name:   "temp banned",
			marker: 0xd2,
			seedUser: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{
					Username: "u", Email: "u@x", Password: "Old1234!", State: 0, UnbanTime: future,
				})
				return u.ID
			},
			wantErr: domain.ErrAccountTempBanned,
		},
		{
			name:   "deleted before consume",
			marker: 0xd3,
			seedUser: func(_ *fakeUserRepo) int {
				return 4242
			},
			wantErr: domain.ErrAccountDeleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newServiceWithReset(t)
			fx.svc.now = func() time.Time { return fixed }
			accountID := tt.seedUser(fx.userRepo)
			rawToken := seedResetToken(t, fx, accountID, tt.marker)

			err := fx.svc.ConsumePasswordReset(context.Background(), rawToken, "NewPass1!")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}

			user, repoErr := fx.userRepo.GetByID(context.Background(), accountID)
			if repoErr == nil && user.Password != "Old1234!" {
				t.Errorf("password was updated to %q despite tier rejection", user.Password)
			}

			var consumed bool
			for _, stored := range fx.tokenRepo.byHash {
				if stored.AccountID == accountID && stored.Action == actiontokendomain.PasswordReset {
					consumed = stored.ConsumedAt.Valid
				}
			}
			if !consumed {
				t.Errorf("token must be consumed on tier rejection so the link cannot be retried")
			}

			if len(fx.invalidator.invalidateAllCalls) != 0 {
				t.Errorf("InvalidateAll must not run for a rejected reset: %v", fx.invalidator.invalidateAllCalls)
			}
		})
	}
}

func seedEmailChangeToken(t *testing.T, fx *emailChangeFixture, accountID int, marker byte, newEmail string) string {
	t.Helper()
	var raw [32]byte
	raw[0] = marker
	hash := sha256.Sum256(raw[:])
	now := time.Now()
	fx.tokenRepo.byHash[hash] = &actiontokendomain.ActionToken{
		TokenHash: hash, AccountID: accountID, Action: actiontokendomain.EmailChange,
		Payload:   []byte(newEmail),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now.Add(-time.Minute),
	}

	return base64.RawURLEncoding.EncodeToString(raw[:])
}

func TestService_ConsumeEmailChange_TierRejections(t *testing.T) {
	t.Parallel()

	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	future := fixed.Add(time.Hour)

	tests := []struct {
		seedUser func(*fakeUserRepo) int
		wantErr  error
		name     string
		marker   byte
	}{
		{
			name:   "perma banned",
			marker: 0xe1,
			seedUser: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{
					Username: "u", Email: "old@example.com", State: 5,
				})
				return u.ID
			},
			wantErr: domain.ErrAccountPermaBanned,
		},
		{
			name:   "temp banned",
			marker: 0xe2,
			seedUser: func(repo *fakeUserRepo) int {
				u, _ := repo.Create(context.Background(), &domain.User{
					Username: "u", Email: "old@example.com", State: 0, UnbanTime: future,
				})
				return u.ID
			},
			wantErr: domain.ErrAccountTempBanned,
		},
		{
			name:   "deleted before consume",
			marker: 0xe3,
			seedUser: func(_ *fakeUserRepo) int {
				return 4343
			},
			wantErr: domain.ErrAccountDeleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fx := newEmailChangeFixture(t)
			fx.svc.now = func() time.Time { return fixed }
			accountID := tt.seedUser(fx.userRepo)
			rawToken := seedEmailChangeToken(t, fx, accountID, tt.marker, "new@example.com")

			_, err := fx.svc.ConsumeEmailChange(context.Background(), rawToken)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}

			user, repoErr := fx.userRepo.GetByID(context.Background(), accountID)
			if repoErr == nil && user.Email != "old@example.com" {
				t.Errorf("email was updated to %q despite tier rejection", user.Email)
			}

			var consumed bool
			for _, stored := range fx.tokenRepo.byHash {
				if stored.AccountID == accountID && stored.Action == actiontokendomain.EmailChange {
					consumed = stored.ConsumedAt.Valid
				}
			}
			if !consumed {
				t.Errorf("token must be consumed on tier rejection so the link cannot be retried")
			}
		})
	}
}
