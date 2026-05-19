package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
)

type fakeRepo struct {
	items  map[int64]domain.News
	nextID int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: map[int64]domain.News{}, nextID: 1}
}

func (r *fakeRepo) Create(_ context.Context, n domain.News) (int64, error) {
	id := r.nextID
	r.nextID++
	n.ID = id
	r.items[id] = n

	return id, nil
}

func (r *fakeRepo) Update(_ context.Context, n domain.News) error {
	existing, ok := r.items[n.ID]
	if !ok {
		return domain.ErrNotFound
	}
	n.CreatedAt = existing.CreatedAt
	r.items[n.ID] = n

	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.items, id)

	return nil
}

func (r *fakeRepo) Get(_ context.Context, id int64) (domain.News, error) {
	n, ok := r.items[id]
	if !ok {
		return domain.News{}, domain.ErrNotFound
	}

	return n, nil
}

func (r *fakeRepo) List(_ context.Context) ([]domain.News, error) {
	out := make([]domain.News, 0, len(r.items))
	for _, n := range r.items {
		out = append(out, n)
	}

	return out, nil
}

func (r *fakeRepo) ListByCategory(_ context.Context, category string) ([]domain.News, error) {
	out := make([]domain.News, 0)
	for _, n := range r.items {
		if n.Category == category {
			out = append(out, n)
		}
	}

	return out, nil
}

func newTestService() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	resolver := domain.NewCategoryResolver([]domain.Category{
		{Key: "Announcement", Display: "Announcement"},
		{Key: "Patch", Display: "Patch Notes"},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewService(repo, resolver, logger), repo
}

func TestNewService_NilRepoPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil repo, got none")
		}
	}()
	NewService(nil, domain.NewCategoryResolver(nil), nil)
}

func TestNewService_NilLoggerUsesDefault(t *testing.T) {
	t.Parallel()
	svc := NewService(newFakeRepo(), domain.NewCategoryResolver(nil), nil)
	if svc.logger == nil {
		t.Errorf("logger is nil after NewService(nil logger)")
	}
}

func TestService_Categories_ReturnsResolver(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()
	if !svc.Categories().Has("Announcement") {
		t.Errorf("Categories().Has(Announcement) = false")
	}
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr  error
		name     string
		title    string
		body     string
		category string
	}{
		{nil, "valid create", "Title", "body", "Announcement"},
		{domain.ErrTitleEmpty, "empty title", "", "body", "Announcement"},
		{domain.ErrBodyEmpty, "empty body", "Title", "", "Announcement"},
		{domain.ErrInvalidCategory, "unknown category", "Title", "body", "Bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, _ := newTestService()
			_, err := svc.Create(context.Background(), tt.title, tt.body, tt.category)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Create err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_Create_SetsCreatedAt(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService()
	fixed := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	id, err := svc.Create(context.Background(), "Title", "body", "Announcement")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := repo.items[id].CreatedAt; !got.Equal(fixed) {
		t.Errorf("CreatedAt = %v, want %v", got, fixed)
	}
}

func TestService_Update(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService()
	id, err := svc.Create(context.Background(), "Title", "body", "Announcement")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	if err := svc.Update(context.Background(), id, "Updated", "newbody", "Patch"); err != nil {
		t.Errorf("Update: %v", err)
	}
	if got := repo.items[id].Title; got != "Updated" {
		t.Errorf("Title after update = %q, want Updated", got)
	}

	if err := svc.Update(context.Background(), 9999, "X", "Y", "Announcement"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update missing err = %v, want ErrNotFound", err)
	}
	if err := svc.Update(context.Background(), id, "", "body", "Announcement"); !errors.Is(err, domain.ErrTitleEmpty) {
		t.Errorf("Update empty title err = %v, want ErrTitleEmpty", err)
	}
}

func TestService_Delete(t *testing.T) {
	t.Parallel()
	svc, repo := newTestService()
	id, _ := svc.Create(context.Background(), "Title", "body", "Announcement")

	if err := svc.Delete(context.Background(), id); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if _, ok := repo.items[id]; ok {
		t.Errorf("item still in repo after delete")
	}

	if err := svc.Delete(context.Background(), 9999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Delete missing err = %v, want ErrNotFound", err)
	}
}

func TestService_GetByID_PopulatesDisplay(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()
	id, _ := svc.Create(context.Background(), "Title", "body", "Patch")

	got, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.CategoryDisplay != "Patch Notes" {
		t.Errorf("CategoryDisplay = %q, want Patch Notes", got.CategoryDisplay)
	}

	if _, err := svc.GetByID(context.Background(), 9999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetByID missing err = %v, want ErrNotFound", err)
	}
}

func TestService_List_PopulatesDisplay(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()
	_, _ = svc.Create(context.Background(), "A", "x", "Announcement")
	_, _ = svc.Create(context.Background(), "B", "y", "Patch")

	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.CategoryDisplay == "" {
			t.Errorf("item.CategoryDisplay empty for category %q", item.Category)
		}
	}
}

func TestService_ListByCategory(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService()
	_, _ = svc.Create(context.Background(), "A", "x", "Announcement")
	_, _ = svc.Create(context.Background(), "B", "y", "Patch")

	items, err := svc.ListByCategory(context.Background(), "Patch")
	if err != nil {
		t.Fatalf("ListByCategory: %v", err)
	}
	if len(items) != 1 || items[0].Category != "Patch" {
		t.Errorf("items = %+v, want one Patch item", items)
	}

	if _, err := svc.ListByCategory(context.Background(), "Bogus"); !errors.Is(err, domain.ErrInvalidCategory) {
		t.Errorf("ListByCategory(Bogus) err = %v, want ErrInvalidCategory", err)
	}
}
