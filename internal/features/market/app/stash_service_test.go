package app

import (
	"context"
	"errors"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

var errStashTest = errors.New("app: stash test error")

type fakeStashRepository struct {
	listErr       error
	lockedErr     error
	items         []domain.StashItem
	mergeExisting int
	locked        bool
	mergeFound    bool
}

func (f *fakeStashRepository) ListByAccount(_ context.Context, _ int) ([]domain.StashItem, error) {
	return f.items, f.listErr
}

func (f *fakeStashRepository) IsLocked(_ context.Context, _ int) (bool, error) {
	return f.locked, f.lockedErr
}

func (f *fakeStashRepository) SlotsUsed(_ context.Context, _ int) (int, error) {
	return len(f.items), nil
}

func (f *fakeStashRepository) MergeableAmount(_ context.Context, _ int, _ domain.StashItem) (existingAmount int, found bool, err error) {
	return f.mergeExisting, f.mergeFound, nil
}

func TestStashService_View(t *testing.T) {
	t.Parallel()

	repo := &fakeStashRepository{
		items:  []domain.StashItem{{NameID: 501, Amount: 10}, {NameID: 502, Amount: 3}},
		locked: true,
	}
	service := NewStashService(repo, 600)

	view, err := service.View(context.Background(), 1)
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if len(view.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(view.Items))
	}
	if !view.Locked {
		t.Errorf("Locked = false, want true")
	}
	if view.SlotsUsed != 2 {
		t.Errorf("SlotsUsed = %d, want 2", view.SlotsUsed)
	}
	if view.SlotsTotal != 600 {
		t.Errorf("SlotsTotal = %d, want 600", view.SlotsTotal)
	}
}

func TestStashService_View_LockError(t *testing.T) {
	t.Parallel()

	repo := &fakeStashRepository{lockedErr: errStashTest}
	service := NewStashService(repo, 600)

	_, err := service.View(context.Background(), 1)
	if !errors.Is(err, errStashTest) {
		t.Errorf("err = %v, want errStashTest", err)
	}
}

func TestStashService_View_ListError(t *testing.T) {
	t.Parallel()

	repo := &fakeStashRepository{listErr: errStashTest}
	service := NewStashService(repo, 600)

	_, err := service.View(context.Background(), 1)
	if !errors.Is(err, errStashTest) {
		t.Errorf("err = %v, want errStashTest", err)
	}
}

func TestNewStashService_DefaultSlots(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		slotsTotal int
	}{
		{name: "zero", slotsTotal: 0},
		{name: "negative", slotsTotal: -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := NewStashService(&fakeStashRepository{}, tt.slotsTotal)

			view, err := service.View(context.Background(), 1)
			if err != nil {
				t.Fatalf("View: %v", err)
			}
			if view.SlotsTotal != domain.DefaultMaxStorageSlots {
				t.Errorf("SlotsTotal = %d, want %d", view.SlotsTotal, domain.DefaultMaxStorageSlots)
			}
		})
	}
}
