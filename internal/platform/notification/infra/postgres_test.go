//go:build integration

package infra

import (
	"context"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/platform/notification/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

var _ domain.Repository = (*Repository)(nil)

func setupRepo(t *testing.T) *Repository {
	t.Helper()

	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_notification")

	return NewRepository(pool)
}

func fixture(accountID int, link string) domain.Notification {
	return domain.Notification{
		AccountID: accountID,
		Category:  "test",
		Title:     "Test title",
		Body:      "Test body",
		Link:      link,
	}
}

func TestRepository_CreateAndRecent(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, fixture(7, "/store"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID <= 0 {
		t.Errorf("id = %d, want > 0", created.ID)
	}
	if created.CreatedAt.IsZero() {
		t.Errorf("CreatedAt is zero")
	}
	if created.ReadAt != nil {
		t.Errorf("ReadAt = %v, want nil", created.ReadAt)
	}

	items, err := repo.RecentByAccount(ctx, 7, 20)
	if err != nil {
		t.Fatalf("RecentByAccount: %v", err)
	}
	if len(items) != 1 || items[0].Link != "/store" {
		t.Errorf("items = %+v, want one item linking to /store", items)
	}
}

func TestRepository_RecentByAccount_ScopedAndLimited(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	for range 3 {
		if _, err := repo.Create(ctx, fixture(7, "")); err != nil {
			t.Fatalf("Create owner: %v", err)
		}
	}
	if _, err := repo.Create(ctx, fixture(8, "")); err != nil {
		t.Fatalf("Create other: %v", err)
	}

	items, err := repo.RecentByAccount(ctx, 7, 2)
	if err != nil {
		t.Fatalf("RecentByAccount: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2 (limit honored)", len(items))
	}
	for _, item := range items {
		if item.AccountID != 7 {
			t.Errorf("item.AccountID = %d, want 7 (scoped)", item.AccountID)
		}
	}
}

func TestRepository_RecentByAccount_OrdersNewestFirst(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	first, err := repo.Create(ctx, fixture(7, ""))
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := repo.Create(ctx, fixture(7, ""))
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	items, err := repo.RecentByAccount(ctx, 7, 20)
	if err != nil {
		t.Fatalf("RecentByAccount: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != second.ID || items[1].ID != first.ID {
		t.Errorf("order = [%d, %d], want [%d, %d] (newest first)", items[0].ID, items[1].ID, second.ID, first.ID)
	}
}

func TestRepository_UnreadCount(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	first, _ := repo.Create(ctx, fixture(7, ""))
	_, _ = repo.Create(ctx, fixture(7, ""))

	count, err := repo.UnreadCount(ctx, 7)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	if _, err := repo.MarkRead(ctx, 7, first.ID, time.Now()); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	count, err = repo.UnreadCount(ctx, 7)
	if err != nil {
		t.Fatalf("UnreadCount after read: %v", err)
	}
	if count != 1 {
		t.Errorf("count after one read = %d, want 1", count)
	}
}

func TestRepository_MarkRead_ReturnsLinkAndIsIdempotent(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, fixture(7, "/tickets/9"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	link, err := repo.MarkRead(ctx, 7, created.ID, time.Now())
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if link != "/tickets/9" {
		t.Errorf("link = %q, want /tickets/9", link)
	}

	again, err := repo.MarkRead(ctx, 7, created.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("MarkRead second: %v", err)
	}
	if again != "/tickets/9" {
		t.Errorf("second link = %q, want /tickets/9 (idempotent)", again)
	}
}

func TestRepository_MarkRead_WrongAccountReturnsEmpty(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, fixture(7, "/x"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	link, err := repo.MarkRead(ctx, 8, created.ID, time.Now())
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if link != "" {
		t.Errorf("link = %q, want empty for non-owner", link)
	}

	count, err := repo.UnreadCount(ctx, 7)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 1 {
		t.Errorf("owner unread = %d, want 1 (non-owner read must not apply)", count)
	}
}

func TestRepository_MarkAllRead(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	for range 3 {
		if _, err := repo.Create(ctx, fixture(7, "")); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	rows, err := repo.MarkAllRead(ctx, 7, time.Now())
	if err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if rows != 3 {
		t.Errorf("rows = %d, want 3", rows)
	}

	count, err := repo.UnreadCount(ctx, 7)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 0 {
		t.Errorf("unread after mark all = %d, want 0", count)
	}
}

func TestRepository_PruneOlderThan(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, fixture(7, "")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	kept, err := repo.PruneOlderThan(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("PruneOlderThan past: %v", err)
	}
	if kept != 0 {
		t.Errorf("pruned %d with a past cutoff, want 0", kept)
	}

	removed, err := repo.PruneOlderThan(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("PruneOlderThan future: %v", err)
	}
	if removed != 1 {
		t.Errorf("pruned %d with a future cutoff, want 1", removed)
	}

	count, err := repo.UnreadCount(ctx, 7)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 0 {
		t.Errorf("unread after prune = %d, want 0", count)
	}
}

func TestRepository_ListPage(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	for range 3 {
		if _, err := repo.Create(ctx, fixture(7, "")); err != nil {
			t.Fatalf("Create owner: %v", err)
		}
	}
	read, err := repo.Create(ctx, fixture(7, ""))
	if err != nil {
		t.Fatalf("Create read: %v", err)
	}
	if _, err := repo.Create(ctx, fixture(8, "")); err != nil {
		t.Fatalf("Create other: %v", err)
	}
	if _, err := repo.MarkRead(ctx, 7, read.ID, time.Now()); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}

	all, total, err := repo.ListPage(ctx, 7, false, 2, 0)
	if err != nil {
		t.Fatalf("ListPage all: %v", err)
	}
	if total != 4 {
		t.Errorf("all total = %d, want 4", total)
	}
	if len(all) != 2 {
		t.Errorf("all page len = %d, want 2 (limit)", len(all))
	}

	page2, _, err := repo.ListPage(ctx, 7, false, 2, 2)
	if err != nil {
		t.Fatalf("ListPage all page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}
	if page2[0].ID == all[0].ID {
		t.Errorf("page2 overlaps page1 (offset not applied)")
	}

	unread, unreadTotal, err := repo.ListPage(ctx, 7, true, 50, 0)
	if err != nil {
		t.Fatalf("ListPage unread: %v", err)
	}
	if unreadTotal != 3 {
		t.Errorf("unread total = %d, want 3", unreadTotal)
	}
	for _, item := range unread {
		if item.IsRead() {
			t.Errorf("unread filter returned a read notification: %+v", item)
		}
	}
}
