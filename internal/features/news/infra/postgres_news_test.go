//go:build integration

package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

var _ domain.Repository = (*Repository)(nil)

func setupRepo(t *testing.T) *Repository {
	t.Helper()
	pool := testutil.OpenPostgres(t, "DB_CP_TEST_URL")
	testutil.TruncatePostgres(t, pool, "cp_news")

	return NewRepository(pool)
}

func fixture(category string) domain.News {
	return domain.News{
		Title:    "Test title",
		Body:     "Test body",
		Category: category,
	}
}

func TestRepository_CreateAndGet(t *testing.T) {
	repo := setupRepo(t)

	id, err := repo.Create(context.Background(), fixture("Announcement"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id <= 0 {
		t.Errorf("id = %d, want > 0", id)
	}

	got, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Test title" || got.Category != "Announcement" {
		t.Errorf("got = %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("CreatedAt is zero")
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	repo := setupRepo(t)
	_, err := repo.Get(context.Background(), 99999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepository_Update(t *testing.T) {
	repo := setupRepo(t)
	id, err := repo.Create(context.Background(), fixture("Announcement"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Update(context.Background(), domain.News{
		ID: id, Title: "Updated", Body: "newbody", Category: "Patch",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Updated" || got.Category != "Patch" {
		t.Errorf("got = %+v", got)
	}
}

func TestRepository_Update_NotFound(t *testing.T) {
	repo := setupRepo(t)
	err := repo.Update(context.Background(), domain.News{
		ID: 99999, Title: "X", Body: "Y", Category: "Announcement",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepository_Delete(t *testing.T) {
	repo := setupRepo(t)
	id, err := repo.Create(context.Background(), fixture("Announcement"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(context.Background(), id); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if _, err := repo.Get(context.Background(), id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get after Delete err = %v, want ErrNotFound", err)
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	repo := setupRepo(t)
	if err := repo.Delete(context.Background(), 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRepository_List_OrdersByCreatedAtDesc(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	first, err := repo.Create(ctx, fixture("Announcement"))
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := repo.Create(ctx, fixture("Patch"))
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != second || items[1].ID != first {
		t.Errorf("order = [%d, %d], want [%d, %d] (newest first)", items[0].ID, items[1].ID, second, first)
	}
}

func TestRepository_ListByCategory_Filters(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, fixture("Announcement"))
	patchID, _ := repo.Create(ctx, fixture("Patch"))
	_, _ = repo.Create(ctx, fixture("Announcement"))

	items, err := repo.ListByCategory(ctx, "Patch")
	if err != nil {
		t.Fatalf("ListByCategory: %v", err)
	}
	if len(items) != 1 || items[0].ID != patchID {
		t.Errorf("got %+v, want one Patch item with id %d", items, patchID)
	}
}

func TestRepository_ListByCategory_NoMatches(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()
	_, _ = repo.Create(ctx, fixture("Announcement"))

	items, err := repo.ListByCategory(ctx, "Patch")
	if err != nil {
		t.Fatalf("ListByCategory: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}
