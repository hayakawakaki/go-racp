//go:build integration

package infra

import (
	"context"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/tickets/domain"
)

var _ domain.ViewRepository = (*ViewRepository)(nil)

func TestViewRepository_UpsertAndUnreadCount(t *testing.T) {
	repo := setupRepo(t)
	views := NewViewRepository(repo.Pool)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	id, err := repo.Create(context.Background(), ticket, message)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err := views.UnreadCountForPlayer(context.Background(), 100)
	if err != nil {
		t.Fatalf("UnreadCountForPlayer initial: %v", err)
	}
	if count != 1 {
		t.Errorf("initial unread = %d, want 1", count)
	}

	if err := views.Upsert(context.Background(), 100, id, now.Add(time.Hour)); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	count, err = views.UnreadCountForPlayer(context.Background(), 100)
	if err != nil {
		t.Fatalf("UnreadCountForPlayer post-upsert: %v", err)
	}
	if count != 0 {
		t.Errorf("after view unread = %d, want 0", count)
	}
}

func TestViewRepository_UnreadCountForStaff(t *testing.T) {
	repo := setupRepo(t)
	views := NewViewRepository(repo.Pool)
	now := time.Now().UTC().Truncate(time.Second)
	ticket, message := openTicketFixture(100, now)
	if _, err := repo.Create(context.Background(), ticket, message); err != nil {
		t.Fatalf("Create: %v", err)
	}

	count, err := views.UnreadCountForStaff(context.Background(), 9, []string{"Other"})
	if err != nil {
		t.Fatalf("UnreadCountForStaff: %v", err)
	}
	if count != 1 {
		t.Errorf("staff unread = %d, want 1", count)
	}

	count, err = views.UnreadCountForStaff(context.Background(), 9, nil)
	if err != nil {
		t.Fatalf("UnreadCountForStaff empty: %v", err)
	}
	if count != 0 {
		t.Errorf("staff unread (no categories) = %d, want 0", count)
	}
}
