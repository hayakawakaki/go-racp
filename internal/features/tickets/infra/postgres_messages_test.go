//go:build integration

package infra

import (
	"context"
	"testing"
	"time"

	domain2 "github.com/hayakawakaki/go-racp/internal/features/tickets/domain"
)

var _ domain2.MessageRepository = (*MessageRepository)(nil)

func TestMessageRepository_ExcludesInternalWhenRequested(t *testing.T) {
	repo := setupRepo(t)
	messages := NewMessageRepository(repo.Pool)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, opening := openTicketFixture(100, now)
	id, err := repo.Create(context.Background(), ticket, opening)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.AppendInternalNote(context.Background(), id, domain2.Message{
		TicketID: id, AuthorID: 9, AuthorRole: domain2.ActorStaff,
		Visibility: domain2.VisibilityInternal, Body: "secret", CreatedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("AppendInternalNote: %v", err)
	}

	visible, err := messages.List(context.Background(), id, false)
	if err != nil {
		t.Fatalf("List public: %v", err)
	}
	if len(visible) != 1 {
		t.Errorf("public list len = %d, want 1", len(visible))
	}

	all, err := messages.List(context.Background(), id, true)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("all list len = %d, want 2", len(all))
	}
}
